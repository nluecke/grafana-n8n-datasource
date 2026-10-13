package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/nluecke/grafana-n8n-datasource/pkg/n8n"
)

// Query types, sent by the frontend in DataQuery.queryType.
const (
	QueryTypeExecutions     = "executions"
	QueryTypeExecutionStats = "executionStats"
	QueryTypeWorkflows      = "workflows"
	QueryTypeInsights       = "insights"
	QueryTypeAudit          = "audit"
)

// queryModel is the JSON model of a query. Fields not relevant for a query type are ignored.
type queryModel struct {
	Statuses    []string `json:"statuses"`
	WorkflowIDs []string `json:"workflowIds"`
	ProjectID   string   `json:"projectId"`

	// executionStats
	Metric      string    `json:"metric"`  // count | successRate | errorRate | duration
	GroupBy     string    `json:"groupBy"` // none | status | workflow
	Percentiles []float64 `json:"percentiles"`
	Bucket      string    `json:"bucket"` // auto (query interval) | range (one value for the whole time range)

	// insights
	InsightsMode   string `json:"insightsMode"`   // summary | timeseries
	InsightsBucket string `json:"insightsBucket"` // auto | 1h | 1d | 1w
}

func parseQuery(q backend.DataQuery) (queryModel, error) {
	var qm queryModel
	if len(q.JSON) > 0 {
		if err := json.Unmarshal(q.JSON, &qm); err != nil {
			return qm, fmt.Errorf("invalid query: %w", err)
		}
	}
	qm.Statuses = cleanList(qm.Statuses)
	qm.WorkflowIDs = cleanList(qm.WorkflowIDs)
	if qm.ProjectID == "$__all" || qm.ProjectID == "All" {
		qm.ProjectID = ""
	}
	return qm, nil
}

// cleanList drops empty values and the "all" selection of template variables.
func cleanList(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || v == "$__all" || v == "All" {
			continue
		}
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

func (d *Datasource) query(ctx context.Context, q backend.DataQuery) backend.DataResponse {
	if d.settingsErr != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, d.settingsErr.Error())
	}
	qm, err := parseQuery(q)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
	}

	var frames data.Frames
	switch q.QueryType {
	case QueryTypeExecutions, "":
		frames, err = d.queryExecutions(ctx, q, qm)
	case QueryTypeExecutionStats:
		frames, err = d.queryExecutionStats(ctx, q, qm)
	case QueryTypeWorkflows:
		frames, err = d.queryWorkflows(ctx, qm)
	case QueryTypeInsights:
		frames, err = d.queryInsights(ctx, q, qm)
	case QueryTypeAudit:
		frames, err = d.queryAudit(ctx)
	default:
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("unknown query type %q", q.QueryType))
	}
	if err != nil {
		return errorResponse(err)
	}
	return backend.DataResponse{Frames: frames}
}

// errorResponse maps client errors to a DataResponse with a readable message.
func errorResponse(err error) backend.DataResponse {
	var apiErr *n8n.APIError
	if errors.As(err, &apiErr) {
		status := backend.Status(apiErr.StatusCode)
		switch apiErr.StatusCode {
		case http.StatusUnauthorized:
			return backend.ErrDataResponse(status, "n8n rejected the API key (401) or the public API is disabled")
		case http.StatusForbidden:
			return backend.ErrDataResponse(status, "n8n denied access (403): "+apiErr.Message)
		}
		return backend.ErrDataResponse(status, apiErr.Error())
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return backend.ErrDataResponse(backend.StatusTimeout, "timed out waiting for n8n")
	}
	var userErr userError
	if errors.As(err, &userErr) {
		return backend.ErrDataResponse(backend.StatusBadRequest, userErr.Error())
	}
	return backend.ErrDataResponse(backend.StatusInternal, err.Error())
}

// userError marks errors caused by the query itself.
type userError struct{ msg string }

func (e userError) Error() string { return e.msg }

// uiBaseURL derives the n8n editor URL from the API URL for deep links.
func (d *Datasource) uiBaseURL() string {
	return strings.TrimSuffix(d.settings.URL, "/api/v1")
}

func (d *Datasource) cacheTTL() time.Duration {
	if d.settings.CacheTTLSeconds == nil {
		return 0
	}
	return time.Duration(*d.settings.CacheTTLSeconds) * time.Second
}
