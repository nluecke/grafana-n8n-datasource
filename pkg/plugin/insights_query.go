package plugin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/nluecke/grafana-n8n-datasource/pkg/n8n"
)

const (
	InsightsModeSummary    = "summary"
	InsightsModeTimeSeries = "timeseries"

	maxInsightsBuckets     = 400
	insightsConcurrency    = 4
	insightsLicenceHint    = " Community Edition allows ranges of at least 24 hours within the last 7 days; longer history and hourly data need a Business or Enterprise licence."
	hourlyNotAvailableText = "Hourly data is not available"
)

func (d *Datasource) queryInsights(ctx context.Context, q backend.DataQuery, qm queryModel) (data.Frames, error) {
	mode := qm.InsightsMode
	if mode == "" {
		mode = InsightsModeSummary
	}
	switch mode {
	case InsightsModeSummary:
		s, err := d.client.InsightsSummary(ctx, q.TimeRange.From, q.TimeRange.To, qm.ProjectID)
		if err != nil {
			return nil, insightsError(err)
		}
		return data.Frames{insightsSummaryFrame(s)}, nil
	case InsightsModeTimeSeries:
		return d.queryInsightsTimeSeries(ctx, q, qm)
	}
	return nil, userError{fmt.Sprintf("unknown insights mode %q", mode)}
}

func (d *Datasource) queryInsightsTimeSeries(ctx context.Context, q backend.DataQuery, qm queryModel) (data.Frames, error) {
	var notices []data.Notice
	bucket, auto, err := insightsBucket(qm.InsightsBucket, q.TimeRange)
	if err != nil {
		return nil, err
	}
	starts, err := insightsBucketStarts(q.TimeRange, bucket)
	if err != nil {
		return nil, err
	}
	summaries, err := d.fetchInsightsBuckets(ctx, starts, bucket, qm.ProjectID)
	if err != nil && auto && bucket < 24*time.Hour && strings.Contains(err.Error(), hourlyNotAvailableText) {
		bucket = 24 * time.Hour
		if starts, err = insightsBucketStarts(q.TimeRange, bucket); err != nil {
			return nil, err
		}
		summaries, err = d.fetchInsightsBuckets(ctx, starts, bucket, qm.ProjectID)
		notices = append(notices, data.Notice{
			Severity: data.NoticeSeverityInfo,
			Text:     "The n8n licence does not provide hourly insights; showing daily buckets.",
		})
	}
	if err != nil {
		return nil, insightsError(err)
	}

	n := len(starts)
	total, failed, failureRate := make([]float64, n), make([]float64, n), make([]float64, n)
	timeSaved, avgRunTime := make([]float64, n), make([]float64, n)
	for i, s := range summaries {
		total[i], failed[i], failureRate[i] = s.Total.Value, s.Failed.Value, s.FailureRate.Value
		timeSaved[i], avgRunTime[i] = s.TimeSaved.Value, s.AverageRunTime.Value
	}
	frame := data.NewFrame("insights",
		data.NewField("time", nil, starts),
		withUnit(data.NewField("total", nil, total), "short"),
		withUnit(data.NewField("failed", nil, failed), "short"),
		withUnit(data.NewField("failureRate", nil, failureRate), "percentunit"),
		withUnit(data.NewField("averageRunTime", nil, avgRunTime), "ms"),
		withUnit(data.NewField("timeSaved", nil, timeSaved), "m"),
	)
	notices = append(notices, data.Notice{
		Severity: data.NoticeSeverityInfo,
		Text:     "n8n compacts insights hourly, so the most recent hour may be incomplete.",
	})
	frame.Meta = &data.FrameMeta{Type: data.FrameTypeTimeSeriesWide, Notices: notices}
	return data.Frames{frame}, nil
}

// fetchInsightsBuckets requests one summary per bucket. Buckets always span their
// full length, also for the current one: n8n accepts an endDate in the future,
// while ranges shorter than 24 h count as hourly data and are rejected without
// the matching licence.
func (d *Datasource) fetchInsightsBuckets(ctx context.Context, starts []time.Time, bucket time.Duration, projectID string) ([]n8n.InsightsSummary, error) {
	out := make([]n8n.InsightsSummary, len(starts))
	errs := make([]error, len(starts))
	sem := make(chan struct{}, insightsConcurrency)
	var wg sync.WaitGroup
	for i, start := range starts {
		end := start.Add(bucket)
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			out[i], errs[i] = d.client.InsightsSummary(ctx, start, end, projectID)
		}()
	}
	wg.Wait()
	return out, errors.Join(errs...)
}

// insightsBucket returns the bucket size and whether it was chosen automatically.
func insightsBucket(setting string, tr backend.TimeRange) (time.Duration, bool, error) {
	switch setting {
	case "1h":
		return time.Hour, false, nil
	case "1d":
		return 24 * time.Hour, false, nil
	case "1w":
		return 7 * 24 * time.Hour, false, nil
	case "", "auto":
		switch r := tr.To.Sub(tr.From); {
		case r <= 3*24*time.Hour:
			return time.Hour, true, nil
		case r <= 120*24*time.Hour:
			return 24 * time.Hour, true, nil
		default:
			return 7 * 24 * time.Hour, true, nil
		}
	}
	return 0, false, userError{fmt.Sprintf("unknown insights bucket %q", setting)}
}

// insightsBucketStarts aligns buckets to UTC hours, days or ISO weeks (Monday).
func insightsBucketStarts(tr backend.TimeRange, bucket time.Duration) ([]time.Time, error) {
	from := tr.From.UTC()
	var start time.Time
	switch bucket {
	case time.Hour:
		start = from.Truncate(time.Hour)
	case 24 * time.Hour:
		start = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	default:
		day := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
		start = day.AddDate(0, 0, -((int(day.Weekday()) + 6) % 7))
	}
	var starts []time.Time
	for t := start; t.Before(tr.To); t = t.Add(bucket) {
		starts = append(starts, t)
		if len(starts) > maxInsightsBuckets {
			return nil, userError{fmt.Sprintf("too many insights buckets (> %d); choose a larger bucket or a shorter time range", maxInsightsBuckets)}
		}
	}
	return starts, nil
}

func insightsSummaryFrame(s n8n.InsightsSummary) *data.Frame {
	metric := func(name, unit string, v n8n.InsightsValue) []*data.Field {
		return []*data.Field{
			withUnit(data.NewField(name, nil, []float64{v.Value}), unit),
			withUnit(data.NewField(name+"Deviation", nil, []*float64{v.Deviation}), unit),
		}
	}
	var fields []*data.Field
	fields = append(fields, metric("total", "short", s.Total)...)
	fields = append(fields, metric("failed", "short", s.Failed)...)
	fields = append(fields, metric("failureRate", "percentunit", s.FailureRate)...)
	fields = append(fields, metric("averageRunTime", "ms", s.AverageRunTime)...)
	fields = append(fields, metric("timeSaved", "m", s.TimeSaved)...)
	frame := data.NewFrame("insights", fields...)
	frame.Meta = &data.FrameMeta{
		Type: data.FrameTypeTable,
		Notices: []data.Notice{{
			Severity: data.NoticeSeverityInfo,
			Text:     "Deviation fields compare with the previous period of the same length.",
		}},
	}
	return frame
}

// insightsError explains licence restrictions, which n8n reports with 403
// ("Hourly data is not available…", "…exceeds the maximum history allowed…").
func insightsError(err error) error {
	var apiErr *n8n.APIError
	if errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusForbidden || apiErr.StatusCode == http.StatusBadRequest) &&
		strings.Contains(strings.ToLower(apiErr.Message), "licen") {
		return userError{"n8n Insights: " + apiErr.Message + "." + insightsLicenceHint}
	}
	return err
}

func withUnit(f *data.Field, unit string) *data.Field {
	f.Config = &data.FieldConfig{Unit: unit}
	return f
}
