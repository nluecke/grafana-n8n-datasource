package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/nluecke/grafana-n8n-datasource/pkg/n8n"
	"github.com/nluecke/grafana-n8n-datasource/pkg/n8n/n8ntest"
	"github.com/stretchr/testify/require"
)

// newFakeInstance seeds 3,000 executions (one per minute, newest just before now,
// ~50 h of history) for two workflows; every 5th execution fails.
func newFakeInstance(t *testing.T, legacy bool, jsonData map[string]any) (*n8ntest.Server, *Datasource) {
	t.Helper()
	srv := n8ntest.NewServer()
	t.Cleanup(srv.Close)
	srv.LegacyAPI = legacy
	srv.Executions = n8ntest.Generate(n8ntest.GenOptions{
		Count: 3000, Newest: time.Now().Add(-time.Minute).Truncate(time.Minute), Step: time.Minute,
		Workflows: []string{"wf1", "wf2"},
		Status: func(i int) string {
			if i%5 == 0 {
				return "error"
			}
			return "success"
		},
	})
	srv.Workflows = []n8n.Workflow{
		{ID: "wf1", Name: "Alpha", Active: true, Tags: []n8n.Tag{{ID: "t1", Name: "prod"}}},
		{ID: "wf2", Name: "Beta"},
	}
	if jsonData == nil {
		jsonData = map[string]any{}
	}
	ds := newTestDatasource(t, srv.APIURL(), jsonData, map[string]string{"apiKey": n8ntest.Key})
	return srv, ds
}

func runQuery(t *testing.T, ds *Datasource, queryType string, tr backend.TimeRange, model map[string]any) backend.DataResponse {
	t.Helper()
	raw, err := json.Marshal(model)
	require.NoError(t, err)
	resp, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
		Queries: []backend.DataQuery{{RefID: "A", QueryType: queryType, TimeRange: tr, Interval: 10 * time.Minute, JSON: raw}},
	})
	require.NoError(t, err)
	return resp.Responses["A"]
}

func lastHours(h int) backend.TimeRange {
	now := time.Now()
	return backend.TimeRange{From: now.Add(-time.Duration(h) * time.Hour), To: now}
}

func noticeTexts(frames data.Frames) string {
	var parts []string
	for _, f := range frames {
		if f.Meta != nil {
			for _, n := range f.Meta.Notices {
				parts = append(parts, n.Text)
			}
		}
	}
	return strings.Join(parts, " | ")
}

func TestQueryExecutionsTable(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "current", true: "legacy"}[legacy], func(t *testing.T) {
			srv, ds := newFakeInstance(t, legacy, nil)
			res := runQuery(t, ds, QueryTypeExecutions, lastHours(1), map[string]any{})
			require.NoError(t, res.Error)
			require.Len(t, res.Frames, 1)
			f := res.Frames[0]
			require.InDelta(t, 60, f.Rows(), 1)

			names := []string{}
			for _, fld := range f.Fields {
				names = append(names, fld.Name)
			}
			require.Equal(t, []string{"id", "workflowId", "workflow", "status", "mode", "startedAt", "stoppedAt", "durationMs", "retryOf", "url"}, names)

			row := f.RowCopy(0)
			require.Contains(t, []string{"Alpha", "Beta"}, row[2])
			require.Equal(t, 100.0, *row[7].(*float64))
			require.Equal(t, srv.URL+"/workflow/"+row[1].(string)+"/executions/"+row[0].(string), row[9])
			require.Equal(t, "Open in n8n", f.Fields[0].Config.Links[0].Title)

			// Paging requests (limit=250) use the server-side filter only where supported;
			// the one-off capability probe (limit=1) is sent in both cases.
			usedServerFilter := false
			for _, r := range srv.Requests() {
				if strings.Contains(r, "limit=250") && strings.Contains(r, "startedAfter=") {
					usedServerFilter = true
				}
			}
			require.Equal(t, !legacy, usedServerFilter, strings.Join(srv.Requests(), "\n"))
		})
	}
}

func TestQueryExecutionsFilters(t *testing.T) {
	srv, ds := newFakeInstance(t, false, nil)

	// Single values are sent to n8n.
	res := runQuery(t, ds, QueryTypeExecutions, lastHours(2), map[string]any{"statuses": []string{"error"}, "workflowIds": []string{"wf1"}})
	require.NoError(t, res.Error)
	for i := 0; i < res.Frames[0].Rows(); i++ {
		row := res.Frames[0].RowCopy(i)
		require.Equal(t, "wf1", row[1])
		require.Equal(t, "error", row[3])
	}
	require.Contains(t, strings.Join(srv.Requests(), "\n"), "status=error")

	// Several values are filtered in the plugin; template "All" is ignored.
	res = runQuery(t, ds, QueryTypeExecutions, lastHours(2), map[string]any{"statuses": []string{"error", "success"}, "workflowIds": []string{"$__all"}})
	require.NoError(t, res.Error)
	require.InDelta(t, 120, res.Frames[0].Rows(), 1)
}

func TestQueryExecutionsTruncationNotice(t *testing.T) {
	_, ds := newFakeInstance(t, true, map[string]any{"maxPages": 2})
	res := runQuery(t, ds, QueryTypeExecutions, lastHours(24), map[string]any{})
	require.NoError(t, res.Error)
	require.Equal(t, 500, res.Frames[0].Rows())
	notices := res.Frames[0].Meta.Notices
	require.Len(t, notices, 1)
	// 500 of 1,440 executions per day loaded (one per minute) → ~35 % → about 6 pages needed.
	require.Contains(t, notices[0].Text, "only the newest 500 executions were loaded")
	require.Contains(t, notices[0].Text, "35% of the time range")
	require.Contains(t, notices[0].Text, "raise \"Max pages\" in the data source settings (now 2; each page adds 250 executions, maximum 100)")
	require.Contains(t, notices[0].Text, "At an even execution rate about 6 pages would cover the full range")
	require.Equal(t, "/connections/datasources/edit/test-uid", notices[0].Link)

	// Beyond the 100-page maximum the notice points to narrower ranges and Insights.
	res = runQuery(t, ds, QueryTypeExecutions, lastHours(24*365), map[string]any{})
	require.Contains(t, noticeTexts(res.Frames), "more than the maximum: use a shorter range, or the Insights query")
}

// Filtering on the server keeps failed executions complete although the
// unfiltered list would be truncated.
func TestQueryExecutionsFanOutKeepsFilteredResultsComplete(t *testing.T) {
	// 3 pages = 750 executions: enough for the ~576 failures, not for all ~2,880 executions.
	srv, ds := newFakeInstance(t, false, map[string]any{"maxPages": 3})
	unfiltered := runQuery(t, ds, QueryTypeExecutions, lastHours(48), map[string]any{})
	require.Contains(t, noticeTexts(unfiltered.Frames), "Incomplete data")

	res := runQuery(t, ds, QueryTypeExecutions, lastHours(48), map[string]any{"statuses": []string{"error", "crashed"}})
	require.NoError(t, res.Error)
	// 3,000 executions, every 5th failed → 600 errors in the last ~50 h; 48 h window → 576.
	require.InDelta(t, 576, res.Frames[0].Rows(), 2)
	require.NotContains(t, noticeTexts(res.Frames), "Incomplete data")

	var statusReqs []string
	for _, r := range srv.Requests() {
		if strings.Contains(r, "/executions?") && strings.Contains(r, "status=") {
			statusReqs = append(statusReqs, r)
		}
	}
	require.GreaterOrEqual(t, len(statusReqs), 2, "one request per status")

	// Merged result is ordered newest first.
	f := res.Frames[0]
	for i := 1; i < f.Rows(); i++ {
		prev, cur := f.Fields[5].At(i-1).(*time.Time), f.Fields[5].At(i).(*time.Time)
		require.False(t, cur.After(*prev))
	}
}

func TestQueryExecutionsHistoryNotice(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		_, ds := newFakeInstance(t, legacy, nil)
		res := runQuery(t, ds, QueryTypeExecutions, lastHours(24*7), map[string]any{})
		require.NoError(t, res.Error)
		require.Contains(t, noticeTexts(res.Frames), "holds no executions before", "legacy=%v", legacy)

		res = runQuery(t, ds, QueryTypeExecutions, lastHours(6), map[string]any{})
		require.NotContains(t, noticeTexts(res.Frames), "holds no executions before", "legacy=%v", legacy)
	}
}

func TestQueryExecutionsSkippedSuccessNotice(t *testing.T) {
	srv, ds := newFakeInstance(t, false, nil)
	srv.Workflows[1].Settings.SaveDataSuccessExecution = "none"
	res := runQuery(t, ds, QueryTypeExecutionStats, lastHours(1), map[string]any{"metric": "successRate"})
	require.NoError(t, res.Error)
	require.Contains(t, noticeTexts(res.Frames), "do not save successful executions, so counts and success rates are too low: Beta")
}

func TestQueryExecutionStats(t *testing.T) {
	_, ds := newFakeInstance(t, false, nil)

	res := runQuery(t, ds, QueryTypeExecutionStats, lastHours(2), map[string]any{"metric": "count", "groupBy": "status"})
	require.NoError(t, res.Error)
	total := 0.0
	for _, f := range res.Frames {
		require.Equal(t, data.FrameTypeTimeSeriesMulti, f.Meta.Type)
		total += sum(floatValues(t, f))
	}
	require.InDelta(t, 120, total, 1)

	// Alert-style query: one numeric series of the error rate.
	res = runQuery(t, ds, QueryTypeExecutionStats, lastHours(1), map[string]any{"metric": "errorRate", "groupBy": "none"})
	require.NoError(t, res.Error)
	require.Len(t, res.Frames, 1)
	var rates []float64
	for _, v := range floatValues(t, res.Frames[0]) {
		if v >= 0 {
			rates = append(rates, v)
		}
	}
	require.NotEmpty(t, rates)
	require.InDelta(t, 0.2, sum(rates)/float64(len(rates)), 0.05)
}

func TestQueryWorkflows(t *testing.T) {
	srv, ds := newFakeInstance(t, false, nil)
	res := runQuery(t, ds, QueryTypeWorkflows, lastHours(1), map[string]any{})
	require.NoError(t, res.Error)
	f := res.Frames[0]
	require.Equal(t, 2, f.Rows())
	row := f.RowCopy(0)
	require.Equal(t, "Alpha", row[1])
	require.Equal(t, true, row[2])
	require.Equal(t, "prod", row[4])
	require.Equal(t, srv.URL+"/workflow/wf1", row[10])
	require.Contains(t, srv.Requests()[len(srv.Requests())-1], "excludePinnedData=true")
}

func TestQueryInsightsSummary(t *testing.T) {
	srv, ds := newFakeInstance(t, false, nil)
	dev := -2.0
	srv.Insights = func(start, end time.Time) (int, any) {
		return 200, n8n.InsightsSummary{
			Total: n8n.InsightsValue{Value: 1200, Unit: "count"}, Failed: n8n.InsightsValue{Value: 60, Deviation: &dev, Unit: "count"},
			FailureRate: n8n.InsightsValue{Value: 0.05, Unit: "ratio"}, AverageRunTime: n8n.InsightsValue{Value: 1500, Unit: "millisecond"},
		}
	}
	res := runQuery(t, ds, QueryTypeInsights, lastHours(24*7), map[string]any{"insightsMode": "summary"})
	require.NoError(t, res.Error)
	f := res.Frames[0]
	v, _ := f.FieldByName("total")
	require.Equal(t, 1200.0, v.At(0))
	v, _ = f.FieldByName("failedDeviation")
	require.Equal(t, -2.0, *v.At(0).(*float64))
	v, _ = f.FieldByName("failureRate")
	require.Equal(t, "percentunit", v.Config.Unit)
}

func TestQueryInsightsTimeSeries(t *testing.T) {
	srv, ds := newFakeInstance(t, false, nil)
	var calls []string
	srv.Insights = func(start, end time.Time) (int, any) {
		calls = append(calls, start.Format(time.RFC3339)+"/"+end.Format(time.RFC3339))
		return 200, n8n.InsightsSummary{Total: n8n.InsightsValue{Value: float64(end.Sub(start) / time.Hour)}}
	}
	tr := backend.TimeRange{From: time.Date(2026, 10, 10, 13, 30, 0, 0, time.UTC), To: time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)}
	res := runQuery(t, ds, QueryTypeInsights, tr, map[string]any{"insightsMode": "timeseries", "insightsBucket": "1d"})
	require.NoError(t, res.Error)
	f := res.Frames[0]
	require.Equal(t, 4, f.Rows()) // Oct 10, 11, 12, 13 (UTC days)
	require.Equal(t, time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), f.Fields[0].At(0))
	require.Equal(t, 24.0, f.Fields[1].At(3), "the current bucket spans a full day")
	require.Len(t, calls, 4)
}

func TestQueryInsightsHourlyFallback(t *testing.T) {
	srv, ds := newFakeInstance(t, false, nil)
	srv.Insights = func(start, end time.Time) (int, any) {
		if end.Sub(start) < 24*time.Hour && start.Hour() != 0 {
			return 403, map[string]string{"message": "Hourly data is not available with your current license"}
		}
		return 200, n8n.InsightsSummary{}
	}
	tr := backend.TimeRange{From: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)}
	res := runQuery(t, ds, QueryTypeInsights, tr, map[string]any{"insightsMode": "timeseries"})
	require.NoError(t, res.Error)
	require.Equal(t, 1, res.Frames[0].Rows())
	require.Contains(t, noticeTexts(res.Frames), "showing daily buckets")
}

func TestQueryInsightsLicenceError(t *testing.T) {
	srv, ds := newFakeInstance(t, false, nil)
	srv.Insights = func(start, end time.Time) (int, any) {
		return 403, map[string]string{"message": "The selected date range exceeds the maximum history allowed by your license"}
	}
	res := runQuery(t, ds, QueryTypeInsights, lastHours(24*30), map[string]any{})
	require.Error(t, res.Error)
	require.Equal(t, backend.StatusBadRequest, res.Status)
	require.Contains(t, res.Error.Error(), "maximum history allowed by your license")
	require.Contains(t, res.Error.Error(), "Community Edition allows ranges of at least 24 hours")
}

func TestQueryAudit(t *testing.T) {
	srv, ds := newFakeInstance(t, false, nil)
	srv.Audit = map[string]n8n.AuditReport{
		"Nodes Risk Report": {Risk: "nodes", Sections: []n8n.AuditSection{{Title: "Official risky nodes", Location: []map[string]any{{"kind": "node"}, {"kind": "node"}}}}},
	}
	res := runQuery(t, ds, QueryTypeAudit, lastHours(1), map[string]any{})
	require.NoError(t, res.Error)
	row := res.Frames[0].RowCopy(0)
	require.Equal(t, []any{"Nodes Risk Report", "nodes", "Official risky nodes", "", "", int64(2)}, row)
}

func TestQueryErrors(t *testing.T) {
	_, ds := newFakeInstance(t, false, nil)
	res := runQuery(t, ds, "nope", lastHours(1), map[string]any{})
	require.Equal(t, backend.StatusBadRequest, res.Status)

	bad := newTestDatasource(t, ds.settings.URL, nil, map[string]string{"apiKey": "wrong"})
	res = runQuery(t, bad, QueryTypeExecutions, lastHours(1), map[string]any{})
	require.Equal(t, backend.StatusUnauthorized, res.Status)
	require.NotContains(t, res.Error.Error(), "wrong")

	unconfigured := newTestDatasource(t, "", nil, nil)
	res = runQuery(t, unconfigured, QueryTypeExecutions, lastHours(1), map[string]any{})
	require.Contains(t, res.Error.Error(), "URL is missing")
}

// Every request the plugin sends while serving all query types must be on the read-only allowlist.
func TestAllRequestsAreAllowlisted(t *testing.T) {
	srv, ds := newFakeInstance(t, true, nil)
	for _, qt := range []string{QueryTypeExecutions, QueryTypeExecutionStats, QueryTypeWorkflows, QueryTypeInsights, QueryTypeAudit} {
		runQuery(t, ds, qt, lastHours(24*7), map[string]any{"insightsMode": "timeseries"})
	}
	for _, path := range []string{"/workflows", "/tags", "/projects"} {
		callResource(t, ds, path)
	}
	_, _ = ds.CheckHealth(context.Background(), &backend.CheckHealthRequest{})

	require.NotEmpty(t, srv.Requests())
	for _, r := range srv.Requests() {
		method, rest, _ := strings.Cut(r, " ")
		u, err := url.Parse(rest)
		require.NoError(t, err)
		require.True(t, n8n.IsAllowed(method, strings.TrimPrefix(u.Path, "/api/v1")), r)
	}
}

func callResource(t *testing.T, ds *Datasource, path string) *backend.CallResourceResponse {
	t.Helper()
	var resp *backend.CallResourceResponse
	err := ds.CallResource(context.Background(), &backend.CallResourceRequest{Method: http.MethodGet, Path: strings.TrimPrefix(path, "/"), URL: path},
		backend.CallResourceResponseSenderFunc(func(r *backend.CallResourceResponse) error { resp = r; return nil }))
	require.NoError(t, err)
	require.NotNil(t, resp)
	return resp
}

func TestResources(t *testing.T) {
	srv, ds := newFakeInstance(t, false, nil)
	srv.Tags = []n8n.Tag{{ID: "t1", Name: "prod"}}

	resp := callResource(t, ds, "/workflows")
	require.Equal(t, http.StatusOK, resp.Status)
	require.JSONEq(t, `[{"value":"wf1","label":"Alpha"},{"value":"wf2","label":"Beta"}]`, string(resp.Body))

	resp = callResource(t, ds, "/tags")
	require.JSONEq(t, `[{"value":"t1","label":"prod"}]`, string(resp.Body))

	resp = callResource(t, ds, "/projects")
	require.JSONEq(t, `[{"value":"p1","label":"Default"}]`, string(resp.Body))

	srv.ProjectsForbidden = true
	ds2 := newTestDatasource(t, srv.APIURL(), map[string]any{"cacheTtlSeconds": 0}, map[string]string{"apiKey": n8ntest.Key})
	resp = callResource(t, ds2, "/projects")
	require.Equal(t, http.StatusOK, resp.Status)
	require.JSONEq(t, `[]`, string(resp.Body))
}

func TestThousands(t *testing.T) {
	for in, want := range map[int]string{0: "0", 250: "250", 5000: "5,000", 25000: "25,000", 1234567: "1,234,567"} {
		require.Equal(t, want, thousands(in))
	}
}
