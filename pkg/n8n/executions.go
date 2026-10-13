package n8n

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

// ExecutionFilter selects executions. Status and WorkflowID are passed to n8n,
// which accepts a single value for each.
type ExecutionFilter struct {
	Status     string
	WorkflowID string
	ProjectID  string
	From, To   time.Time
	// ServerTimeFilter sends startedAfter/startedBefore. Without it pages are
	// fetched newest first until a page lies completely before From.
	ServerTimeFilter bool
	MaxPages         int
}

// ExecutionResult holds the executions started within [From, To].
type ExecutionResult struct {
	Executions []Execution
	Pages      int
	// Truncated is set when MaxPages was reached before the time range was covered.
	Truncated bool
	// Oldest is the earliest startedAt seen on any page, used to detect pruned history.
	Oldest *time.Time
}

// ListExecutions pages through GET /executions (newest first) for the filter.
func (c *Client) ListExecutions(ctx context.Context, f ExecutionFilter) (ExecutionResult, error) {
	var res ExecutionResult
	q := url.Values{"limit": {strconv.Itoa(MaxPageLimit)}}
	if f.Status != "" {
		q.Set("status", f.Status)
	}
	if f.WorkflowID != "" {
		q.Set("workflowId", f.WorkflowID)
	}
	if f.ProjectID != "" {
		q.Set("projectId", f.ProjectID)
	}
	if f.ServerTimeFilter {
		q.Set("startedAfter", f.From.UTC().Format(time.RFC3339Nano))
		q.Set("startedBefore", f.To.UTC().Format(time.RFC3339Nano))
	}
	maxPages := f.MaxPages
	if maxPages <= 0 {
		maxPages = 1
	}

	for {
		var p page[Execution]
		if err := c.Get(ctx, "/executions", q, &p); err != nil {
			return res, err
		}
		res.Pages++

		pageBeforeRange := len(p.Data) > 0
		for _, e := range p.Data {
			if e.StartedAt == nil {
				// Not started yet (new/waiting in queue); belongs to "now".
				pageBeforeRange = false
				if !f.To.Before(time.Now()) {
					res.Executions = append(res.Executions, e)
				}
				continue
			}
			if res.Oldest == nil || e.StartedAt.Before(*res.Oldest) {
				t := *e.StartedAt
				res.Oldest = &t
			}
			if !e.StartedAt.Before(f.From) {
				pageBeforeRange = false
			}
			if !e.StartedAt.Before(f.From) && !e.StartedAt.After(f.To) {
				res.Executions = append(res.Executions, e)
			}
		}

		if p.NextCursor == nil || *p.NextCursor == "" || pageBeforeRange {
			return res, nil
		}
		if res.Pages >= maxPages {
			res.Truncated = true
			return res, nil
		}
		q.Set("cursor", *p.NextCursor)
	}
}
