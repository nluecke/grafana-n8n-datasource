package plugin

import (
	"context"
	"fmt"
	"math"
	"slices"
	"sort"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/nluecke/grafana-n8n-datasource/pkg/n8n"
)

const (
	MetricCount       = "count"
	MetricSuccessRate = "successRate"
	MetricErrorRate   = "errorRate"
	MetricDuration    = "duration"

	BucketAuto  = "auto"
	BucketRange = "range"

	GroupByNone     = "none"
	GroupByStatus   = "status"
	GroupByWorkflow = "workflow"

	maxBuckets = 2000
)

var defaultPercentiles = []float64{50, 90, 99}

// failedStatuses count as failures for success and error rates. Executions that
// are still running or were canceled are not counted.
var failedStatuses = []string{"error", "crashed"}

func isFinishedForRate(status string) bool {
	return status == "success" || slices.Contains(failedStatuses, status)
}

func (d *Datasource) queryExecutionStats(ctx context.Context, q backend.DataQuery, qm queryModel) (data.Frames, error) {
	fetched, err := d.fetchExecutions(ctx, q.TimeRange, qm)
	if err != nil {
		return nil, err
	}
	frames, err := buildStatsFrames(fetched.executions, fetched.workflows, q.TimeRange, q.Interval, qm)
	if err != nil {
		return nil, err
	}
	if len(frames) > 0 && len(fetched.notices) > 0 {
		frames[0].AppendNotices(fetched.notices...)
	}
	return frames, nil
}

// buckets aligns the time range to interval-sized buckets.
type buckets struct {
	start    time.Time
	interval time.Duration
	n        int
}

// rangeBucket covers the whole time range with a single bucket starting at From.
func rangeBucket(tr backend.TimeRange) buckets {
	return buckets{start: tr.From, interval: tr.To.Sub(tr.From) + time.Nanosecond, n: 1}
}

func newBuckets(tr backend.TimeRange, interval time.Duration) buckets {
	if interval < time.Second {
		interval = time.Minute
	}
	for tr.To.Sub(tr.From)/interval > maxBuckets {
		interval *= 2
	}
	start := tr.From.Truncate(interval)
	n := int(tr.To.Sub(start)/interval) + 1
	return buckets{start: start, interval: interval, n: n}
}

func (b buckets) index(t time.Time) int {
	i := int(t.Sub(b.start) / b.interval)
	if i < 0 || i >= b.n {
		return -1
	}
	return i
}

func (b buckets) times() []time.Time {
	ts := make([]time.Time, b.n)
	for i := range ts {
		ts[i] = b.start.Add(time.Duration(i) * b.interval)
	}
	return ts
}

// series accumulates executions per bucket for one group.
type series struct {
	labels data.Labels
	name   string
	execs  [][]n8n.Execution
}

func buildStatsFrames(execs []n8n.Execution, workflows map[string]n8n.Workflow, tr backend.TimeRange, interval time.Duration, qm queryModel) (data.Frames, error) {
	metric := qm.Metric
	if metric == "" {
		metric = MetricCount
	}
	groupBy := qm.GroupBy
	if groupBy == "" {
		groupBy = GroupByNone
		if metric == MetricCount {
			groupBy = GroupByStatus
		}
	}
	switch metric {
	case MetricCount, MetricSuccessRate, MetricErrorRate, MetricDuration:
	default:
		return nil, userError{fmt.Sprintf("unknown metric %q", metric)}
	}
	switch groupBy {
	case GroupByNone, GroupByWorkflow:
	case GroupByStatus:
		if metric != MetricCount {
			return nil, userError{"grouping by status is only supported for the count metric"}
		}
	default:
		return nil, userError{fmt.Sprintf("unknown groupBy %q", groupBy)}
	}

	var b buckets
	switch qm.Bucket {
	case "", BucketAuto:
		b = newBuckets(tr, interval)
	case BucketRange:
		b = rangeBucket(tr)
	default:
		return nil, userError{fmt.Sprintf("unknown bucket %q", qm.Bucket)}
	}
	groups := map[string]*series{}
	groupFor := func(e n8n.Execution) *series {
		key, labels, name := "", data.Labels{}, ""
		switch groupBy {
		case GroupByStatus:
			key, name = e.Status, e.Status
			labels["status"] = e.Status
		case GroupByWorkflow:
			key, name = e.WorkflowID, workflowName(workflows, e.WorkflowID)
			labels["workflowId"] = e.WorkflowID
			labels["workflow"] = name
		}
		s, ok := groups[key]
		if !ok {
			s = &series{labels: labels, name: name, execs: make([][]n8n.Execution, b.n)}
			groups[key] = s
		}
		return s
	}
	// Always emit the main series, even without executions, so stat panels and
	// alert rules see 0 instead of "no data".
	switch groupBy {
	case GroupByNone:
		groupFor(n8n.Execution{})
	case GroupByStatus:
		groupFor(n8n.Execution{Status: "success"})
		groupFor(n8n.Execution{Status: "error"})
	}
	for _, e := range execs {
		if e.StartedAt == nil {
			continue
		}
		i := b.index(*e.StartedAt)
		if i < 0 {
			continue
		}
		s := groupFor(e)
		s.execs[i] = append(s.execs[i], e)
	}

	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	times := b.times()
	var frames data.Frames
	for _, k := range keys {
		s := groups[k]
		switch metric {
		case MetricCount:
			frames = append(frames, valueFrame(times, s, "executions", s.name, "short", countValues(s.execs)))
		case MetricSuccessRate:
			frames = append(frames, valueFrame(times, s, "successRate", s.name, "percentunit", rateValues(s.execs, false)))
		case MetricErrorRate:
			frames = append(frames, valueFrame(times, s, "errorRate", s.name, "percentunit", rateValues(s.execs, true)))
		case MetricDuration:
			pcts := qm.Percentiles
			if len(pcts) == 0 {
				pcts = defaultPercentiles
			}
			for _, p := range pcts {
				if p <= 0 || p > 100 {
					return nil, userError{fmt.Sprintf("percentile %v out of range (0, 100]", p)}
				}
				ps := &series{labels: s.labels.Copy(), name: s.name}
				ps.labels["percentile"] = fmt.Sprintf("p%g", p)
				display := fmt.Sprintf("p%g", p)
				if s.name != "" {
					display = s.name + " " + display
				}
				frames = append(frames, valueFrame(times, ps, "durationMs", display, "ms", percentileValues(s.execs, p)))
			}
		}
	}
	return frames, nil
}

func valueFrame(times []time.Time, s *series, field, display, unit string, values []*float64) *data.Frame {
	v := data.NewField(field, s.labels, values)
	v.Config = &data.FieldConfig{Unit: unit}
	if display != "" {
		v.Config.DisplayNameFromDS = display
	}
	f := data.NewFrame(field, data.NewField("time", nil, times), v)
	f.Meta = &data.FrameMeta{Type: data.FrameTypeTimeSeriesMulti}
	return f
}

func countValues(perBucket [][]n8n.Execution) []*float64 {
	out := make([]*float64, len(perBucket))
	for i, execs := range perBucket {
		v := float64(len(execs))
		out[i] = &v
	}
	return out
}

// rateValues returns the success (or error) share of finished executions per bucket,
// null where no execution finished.
func rateValues(perBucket [][]n8n.Execution, errors bool) []*float64 {
	out := make([]*float64, len(perBucket))
	for i, execs := range perBucket {
		var total, failed int
		for _, e := range execs {
			if !isFinishedForRate(e.Status) {
				continue
			}
			total++
			if slices.Contains(failedStatuses, e.Status) {
				failed++
			}
		}
		if total == 0 {
			continue
		}
		v := float64(total-failed) / float64(total)
		if errors {
			v = float64(failed) / float64(total)
		}
		out[i] = &v
	}
	return out
}

func percentileValues(perBucket [][]n8n.Execution, p float64) []*float64 {
	out := make([]*float64, len(perBucket))
	for i, execs := range perBucket {
		durations := make([]float64, 0, len(execs))
		for _, e := range execs {
			if dur, ok := e.Duration(); ok {
				durations = append(durations, float64(dur.Milliseconds()))
			}
		}
		if v, ok := percentile(durations, p); ok {
			out[i] = &v
		}
	}
	return out
}

// percentile uses linear interpolation between closest ranks.
func percentile(values []float64, p float64) (float64, bool) {
	if len(values) == 0 {
		return 0, false
	}
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	if len(sorted) == 1 {
		return sorted[0], true
	}
	rank := p / 100 * float64(len(sorted)-1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))
	frac := rank - float64(lo)
	return sorted[lo] + (sorted[hi]-sorted[lo])*frac, true
}
