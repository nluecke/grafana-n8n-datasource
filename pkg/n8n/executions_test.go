package n8n_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nluecke/grafana-n8n-datasource/pkg/n8n"
	"github.com/nluecke/grafana-n8n-datasource/pkg/n8n/n8ntest"
	"github.com/stretchr/testify/require"
)

var now = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

// 3,000 executions, one per minute, newest at `now` → ~50 hours of history.
func newSeededServer(t *testing.T, legacy bool) *n8ntest.Server {
	t.Helper()
	srv := n8ntest.NewServer()
	t.Cleanup(srv.Close)
	srv.LegacyAPI = legacy
	srv.Executions = n8ntest.Generate(n8ntest.GenOptions{Count: 3000, Newest: now, Step: time.Minute})
	return srv
}

func countRequests(srv *n8ntest.Server, prefix string) int {
	n := 0
	for _, r := range srv.Requests() {
		if strings.HasPrefix(r, prefix) {
			n++
		}
	}
	return n
}

func TestListExecutionsServerTimeFilter(t *testing.T) {
	srv := newSeededServer(t, false)
	c := n8n.NewClient(srv.APIURL(), n8ntest.APIKey, srv.Client())

	res, err := c.ListExecutions(context.Background(), n8n.ExecutionFilter{
		From: now.Add(-6 * time.Hour), To: now, ServerTimeFilter: true, MaxPages: 20,
	})
	require.NoError(t, err)
	require.Len(t, res.Executions, 361) // inclusive bounds, one per minute
	require.Equal(t, 2, res.Pages)
	require.False(t, res.Truncated)
	require.Contains(t, srv.Requests()[0], "startedAfter=2026-10-13T06%3A00%3A00Z")
}

func TestListExecutionsClientCutoff(t *testing.T) {
	srv := newSeededServer(t, true)
	c := n8n.NewClient(srv.APIURL(), n8ntest.APIKey, srv.Client())

	res, err := c.ListExecutions(context.Background(), n8n.ExecutionFilter{
		From: now.Add(-6 * time.Hour), To: now, MaxPages: 20,
	})
	require.NoError(t, err)
	require.Len(t, res.Executions, 361)
	// Page 1: 250 in range, page 2: 111 in range + older rows, page 3: entirely
	// older → stop. Paging only stops on a page completely before the range,
	// because ids and start times are not strictly ordered (queued executions).
	require.Equal(t, 3, res.Pages)
	require.False(t, res.Truncated)
	require.NotContains(t, strings.Join(srv.Requests(), " "), "startedAfter")
	require.True(t, res.Oldest.Before(now.Add(-6*time.Hour)))
}

func TestListExecutionsStopsOnlyWhenWholePageIsBeforeRange(t *testing.T) {
	srv := newSeededServer(t, true)
	c := n8n.NewClient(srv.APIURL(), n8ntest.APIKey, srv.Client())

	// Range ends 10 hours ago: the first pages are newer than the range and must be skipped, not stop paging.
	res, err := c.ListExecutions(context.Background(), n8n.ExecutionFilter{
		From: now.Add(-12 * time.Hour), To: now.Add(-10 * time.Hour), MaxPages: 20,
	})
	require.NoError(t, err)
	require.Len(t, res.Executions, 121)
	for _, e := range res.Executions {
		require.False(t, e.StartedAt.After(now.Add(-10*time.Hour)))
		require.False(t, e.StartedAt.Before(now.Add(-12*time.Hour)))
	}
}

func TestListExecutionsMaxPages(t *testing.T) {
	srv := newSeededServer(t, true)
	c := n8n.NewClient(srv.APIURL(), n8ntest.APIKey, srv.Client())

	res, err := c.ListExecutions(context.Background(), n8n.ExecutionFilter{
		From: now.Add(-48 * time.Hour), To: now, MaxPages: 3,
	})
	require.NoError(t, err)
	require.True(t, res.Truncated)
	require.Equal(t, 3, res.Pages)
	require.Len(t, res.Executions, 750)
	require.Equal(t, 3, countRequests(srv, "GET /api/v1/executions"))
}

func TestListExecutionsPassesSingleFilters(t *testing.T) {
	srv := newSeededServer(t, false)
	c := n8n.NewClient(srv.APIURL(), n8ntest.APIKey, srv.Client())
	_, err := c.ListExecutions(context.Background(), n8n.ExecutionFilter{
		Status: "error", WorkflowID: "wf1", ProjectID: "p1", From: now.Add(-time.Hour), To: now, ServerTimeFilter: true,
	})
	require.NoError(t, err)
	req := srv.Requests()[0]
	for _, want := range []string{"status=error", "workflowId=wf1", "projectId=p1", "limit=250"} {
		require.Contains(t, req, want)
	}
}

func TestSupportsStartedAfter(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		srv := newSeededServer(t, legacy)
		ok, err := n8n.NewClient(srv.APIURL(), n8ntest.APIKey, srv.Client()).SupportsStartedAfter(context.Background())
		require.NoError(t, err)
		require.Equal(t, !legacy, ok)
	}
}

func TestRetryOn429(t *testing.T) {
	srv := newSeededServer(t, false)
	srv.FailNext, srv.FailStatus, srv.RetryAfter = 2, http.StatusTooManyRequests, "1"
	c := n8n.NewClient(srv.APIURL(), n8ntest.APIKey, srv.Client(), n8n.Options{MaxRetries: 3})
	n8n.SetSleepForTest(c, func(context.Context, time.Duration) error { return nil })

	_, err := c.ListWorkflows(context.Background())
	require.NoError(t, err)
	require.Equal(t, 3, countRequests(srv, "GET /api/v1/workflows"))
}

func TestRetryGivesUp(t *testing.T) {
	srv := newSeededServer(t, false)
	srv.FailNext, srv.FailStatus = 10, http.StatusServiceUnavailable
	c := n8n.NewClient(srv.APIURL(), n8ntest.APIKey, srv.Client(), n8n.Options{MaxRetries: 2})
	var waits []time.Duration
	n8n.SetSleepForTest(c, func(_ context.Context, d time.Duration) error { waits = append(waits, d); return nil })

	_, err := c.ListWorkflows(context.Background())
	var apiErr *n8n.APIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusServiceUnavailable, apiErr.StatusCode)
	require.Equal(t, 3, countRequests(srv, "GET /api/v1/workflows"))
	require.Equal(t, []time.Duration{300 * time.Millisecond, 600 * time.Millisecond}, waits)
}

func TestNoRetryOnClientErrors(t *testing.T) {
	srv := newSeededServer(t, false)
	srv.FailNext, srv.FailStatus = 1, http.StatusForbidden
	c := n8n.NewClient(srv.APIURL(), n8ntest.APIKey, srv.Client())
	_, err := c.ListWorkflows(context.Background())
	require.Error(t, err)
	require.Equal(t, 1, countRequests(srv, "GET /api/v1/workflows"))
}

func TestCache(t *testing.T) {
	srv := newSeededServer(t, false)
	c := n8n.NewClient(srv.APIURL(), n8ntest.APIKey, srv.Client(), n8n.Options{CacheTTL: time.Minute})
	clock := now
	n8n.SetClockForTest(c, func() time.Time { return clock })

	for range 3 {
		_, err := c.ListWorkflows(context.Background())
		require.NoError(t, err)
	}
	require.Equal(t, 1, countRequests(srv, "GET /api/v1/workflows"))

	clock = clock.Add(61 * time.Second)
	_, err := c.ListWorkflows(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, countRequests(srv, "GET /api/v1/workflows"))

	// Errors are not cached.
	srv.FailNext, srv.FailStatus = 1, http.StatusForbidden
	_, err = c.ListTags(context.Background())
	require.Error(t, err)
	_, err = c.ListTags(context.Background())
	require.NoError(t, err)

	// POST /audit is never cached.
	_, _ = c.Audit(context.Background())
	_, _ = c.Audit(context.Background())
	require.Equal(t, 2, countRequests(srv, "POST /api/v1/audit"))
}

func TestAuditEmptyArray(t *testing.T) {
	srv := newSeededServer(t, false)
	reports, err := n8n.NewClient(srv.APIURL(), n8ntest.APIKey, srv.Client()).Audit(context.Background())
	require.NoError(t, err)
	require.Empty(t, reports)
}
