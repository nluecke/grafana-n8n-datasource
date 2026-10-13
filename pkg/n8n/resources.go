package n8n

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// maxListPages bounds listing of workflows, tags and projects (250 items each).
const maxListPages = 40

func listAll[T any](ctx context.Context, c *Client, path string, q url.Values) ([]T, error) {
	if q == nil {
		q = url.Values{}
	}
	q.Set("limit", strconv.Itoa(MaxPageLimit))
	var all []T
	for i := 0; i < maxListPages; i++ {
		var p page[T]
		if err := c.Get(ctx, path, q, &p); err != nil {
			return nil, err
		}
		all = append(all, p.Data...)
		if p.NextCursor == nil || *p.NextCursor == "" {
			break
		}
		q.Set("cursor", *p.NextCursor)
	}
	return all, nil
}

// ListWorkflows returns all workflows visible to the API key.
func (c *Client) ListWorkflows(ctx context.Context) ([]Workflow, error) {
	return listAll[Workflow](ctx, c, "/workflows", url.Values{"excludePinnedData": {"true"}})
}

// ListTags returns all workflow tags.
func (c *Client) ListTags(ctx context.Context) ([]Tag, error) {
	return listAll[Tag](ctx, c, "/tags", nil)
}

// ListProjects returns all projects. Requires a licence with project support;
// n8n answers 403 otherwise.
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	return listAll[Project](ctx, c, "/projects", nil)
}

// InsightsSummary returns aggregated insights for [from, to). The allowed range
// and granularity depend on the n8n licence.
func (c *Client) InsightsSummary(ctx context.Context, from, to time.Time, projectID string) (InsightsSummary, error) {
	q := url.Values{
		"startDate": {from.UTC().Format(time.RFC3339)},
		"endDate":   {to.UTC().Format(time.RFC3339)},
	}
	if projectID != "" {
		q.Set("projectId", projectID)
	}
	var s InsightsSummary
	err := c.Get(ctx, "/insights/summary", q, &s)
	return s, err
}

// Audit generates the n8n security audit. Not cached. n8n answers with an
// empty array instead of an object when no risk was found.
func (c *Client) Audit(ctx context.Context) (map[string]AuditReport, error) {
	var raw json.RawMessage
	if err := c.post(ctx, "/audit", &raw); err != nil {
		return nil, err
	}
	out := map[string]AuditReport{}
	if trimmed := bytes.TrimSpace(raw); len(trimmed) == 0 || trimmed[0] == '[' {
		return out, nil
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode audit report: %w", err)
	}
	return out, nil
}
