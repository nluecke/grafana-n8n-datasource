// Package n8n implements a read-only client for the n8n public REST API.
package n8n

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// HeaderAPIKey is the header n8n reads the API key from.
const HeaderAPIKey = "X-N8N-API-KEY"

// MaxPageLimit is the largest page size the n8n API accepts.
const MaxPageLimit = 250

const (
	defaultMaxRetries = 3
	baseBackoff       = 300 * time.Millisecond
	maxBackoff        = 10 * time.Second
	maxResponseBytes  = 64 << 20
)

type route struct {
	method string
	path   *regexp.Regexp
}

// allowedRoutes lists every request the plugin may send. A non-Enterprise API key
// has full write access to the instance, so anything not listed here is refused
// before it leaves the plugin.
var allowedRoutes = []route{
	{http.MethodGet, regexp.MustCompile(`^/workflows$`)},
	{http.MethodGet, regexp.MustCompile(`^/workflows/[A-Za-z0-9_-]+$`)},
	{http.MethodGet, regexp.MustCompile(`^/executions$`)},
	{http.MethodGet, regexp.MustCompile(`^/executions/[0-9]+$`)},
	{http.MethodGet, regexp.MustCompile(`^/tags$`)},
	{http.MethodGet, regexp.MustCompile(`^/projects$`)},
	{http.MethodGet, regexp.MustCompile(`^/insights/summary$`)},
	// Generates a security audit report; does not modify the instance.
	{http.MethodPost, regexp.MustCompile(`^/audit$`)},
}

// ErrNotAllowed is returned for requests outside of allowedRoutes.
var ErrNotAllowed = errors.New("request not allowed: the n8n client is read-only")

// IsAllowed reports whether method and path (relative to the API base URL) are
// on the read-only allowlist.
func IsAllowed(method, path string) bool {
	for _, r := range allowedRoutes {
		if r.method == method && r.path.MatchString(path) {
			return true
		}
	}
	return false
}

// APIError describes a non-2xx response from n8n.
type APIError struct {
	StatusCode  int
	Message     string
	ContentType string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("n8n API returned %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("n8n API returned %d", e.StatusCode)
}

// ErrNotJSON is returned when n8n answers 2xx with a non-JSON body, which happens
// when the base URL points at the n8n UI instead of the API.
var ErrNotJSON = errors.New("response is not JSON")

// Options tune caching and retries.
type Options struct {
	// CacheTTL caches GET responses per URL. Zero disables the cache.
	CacheTTL time.Duration
	// MaxRetries for 429, 502, 503 and 504 responses. Negative disables retries.
	MaxRetries int
}

// Client talks to one n8n instance.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	cache      *responseCache
	maxRetries int
	sleep      func(context.Context, time.Duration) error
}

// NewClient creates a client. baseURL must include the API path, e.g.
// https://example.app.n8n.cloud/api/v1.
func NewClient(baseURL, apiKey string, httpClient *http.Client, opts ...Options) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	o := Options{MaxRetries: defaultMaxRetries}
	if len(opts) > 0 {
		o = opts[0]
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: httpClient,
		cache:      newResponseCache(o.CacheTTL),
		maxRetries: max(o.MaxRetries, 0),
		sleep:      sleepContext,
	}
}

type noCacheKey struct{}

// WithoutCache returns a context for which GET requests bypass the response cache,
// e.g. for health checks that must test the live connection.
func WithoutCache(ctx context.Context) context.Context {
	return context.WithValue(ctx, noCacheKey{}, true)
}

func cacheDisabled(ctx context.Context) bool {
	v, _ := ctx.Value(noCacheKey{}).(bool)
	return v
}

// Get performs an allowlisted, cached GET request and decodes the JSON body into out.
func (c *Client) Get(ctx context.Context, path string, query url.Values, out any) error {
	return c.do(ctx, http.MethodGet, path, query, out)
}

func (c *Client) post(ctx context.Context, path string, out any) error {
	return c.do(ctx, http.MethodPost, path, nil, out)
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, out any) error {
	if !IsAllowed(method, path) {
		return fmt.Errorf("%w: %s %s", ErrNotAllowed, method, path)
	}

	u := c.baseURL + path
	if len(query) > 0 {
		// url.Values.Encode escapes reserved characters such as ':' which older
		// n8n versions reject when sent unencoded.
		u += "?" + query.Encode()
	}

	cacheable := method == http.MethodGet && !cacheDisabled(ctx)
	if cacheable {
		if body, ok := c.cache.get(u); ok {
			return decode(body, out)
		}
	}

	body, err := c.send(ctx, method, u)
	if err != nil {
		return err
	}
	if cacheable {
		c.cache.set(u, body)
	}
	return decode(body, out)
}

func (c *Client) send(ctx context.Context, method, u string) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		body, retryAfter, err := c.sendOnce(ctx, method, u)
		if err == nil {
			return body, nil
		}
		var apiErr *APIError
		if attempt >= c.maxRetries || !errors.As(err, &apiErr) || !retryable(apiErr.StatusCode) {
			return nil, err
		}
		wait := min(baseBackoff<<attempt, maxBackoff)
		if retryAfter > 0 {
			wait = min(retryAfter, maxBackoff)
		}
		if err := c.sleep(ctx, wait); err != nil {
			return nil, err
		}
	}
}

func (c *Client) sendOnce(ctx context.Context, method, u string) ([]byte, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set(HeaderAPIKey, c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, 0, err
	}

	contentType := resp.Header.Get("Content-Type")
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, parseRetryAfter(resp.Header.Get("Retry-After")),
			&APIError{StatusCode: resp.StatusCode, Message: errorMessage(body), ContentType: contentType}
	}
	if mt, _, _ := mime.ParseMediaType(contentType); mt != "application/json" {
		return nil, 0, fmt.Errorf("%w (content type %q)", ErrNotJSON, contentType)
	}
	return body, 0, nil
}

func retryable(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if s, err := strconv.Atoi(v); err == nil && s > 0 {
		return time.Duration(s) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		return time.Until(t)
	}
	return 0
}

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func decode(body []byte, out any) error {
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode n8n response: %w", err)
	}
	return nil
}

func errorMessage(body []byte) string {
	var e struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &e) == nil {
		return e.Message
	}
	return ""
}
