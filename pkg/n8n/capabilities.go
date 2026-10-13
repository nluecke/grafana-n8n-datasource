package n8n

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SupportsStartedAfter reports whether GET /executions accepts the startedAfter
// filter. Older n8n versions reject unknown query parameters with 400.
func (c *Client) SupportsStartedAfter(ctx context.Context) (bool, error) {
	q := url.Values{
		"limit":        {"1"},
		"startedAfter": {time.Now().UTC().Format(time.RFC3339)},
	}
	err := c.Get(ctx, "/executions", q, nil)
	if err == nil {
		return true, nil
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusBadRequest &&
		strings.Contains(apiErr.Message, "startedAfter") {
		return false, nil
	}
	return false, err
}
