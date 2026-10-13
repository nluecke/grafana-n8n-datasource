package n8n

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsAllowed(t *testing.T) {
	allowed := []struct{ method, path string }{
		{http.MethodGet, "/workflows"},
		{http.MethodGet, "/workflows/AsQijaPaClnVpCUC"},
		{http.MethodGet, "/executions"},
		{http.MethodGet, "/executions/5005"},
		{http.MethodGet, "/tags"},
		{http.MethodGet, "/projects"},
		{http.MethodGet, "/insights/summary"},
		{http.MethodPost, "/audit"},
	}
	for _, c := range allowed {
		require.True(t, IsAllowed(c.method, c.path), "%s %s", c.method, c.path)
	}

	denied := []struct{ method, path string }{
		{http.MethodPost, "/workflows"},
		{http.MethodPut, "/workflows/abc"},
		{http.MethodDelete, "/workflows/abc"},
		{http.MethodPost, "/workflows/abc/activate"},
		{http.MethodPost, "/workflows/abc/deactivate"},
		{http.MethodGet, "/workflows/abc/activate"},
		{http.MethodDelete, "/executions/1"},
		{http.MethodPost, "/executions/1/retry"},
		{http.MethodPost, "/executions/1/stop"},
		{http.MethodGet, "/credentials"},
		{http.MethodGet, "/users"},
		{http.MethodGet, "/variables"},
		{http.MethodPost, "/source-control/pull"},
		{http.MethodGet, "/audit"},
		{http.MethodGet, "/executions/../users"},
		{http.MethodGet, "/workflows/a/../../users"},
		{http.MethodGet, ""},
	}
	for _, c := range denied {
		require.False(t, IsAllowed(c.method, c.path), "%s %s", c.method, c.path)
	}
}

func TestClientRefusesNonAllowlistedRequestWithoutSending(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "secret", srv.Client())
	err := c.do(context.Background(), http.MethodDelete, "/workflows/abc", nil, nil)
	require.ErrorIs(t, err, ErrNotAllowed)
	err = c.Get(context.Background(), "/credentials", nil, nil)
	require.ErrorIs(t, err, ErrNotAllowed)
	require.Zero(t, calls.Load())
}

func TestClientSendsKeyAndEncodesQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "secret", r.Header.Get(HeaderAPIKey))
		require.Equal(t, "/api/v1/executions", r.URL.Path)
		require.Equal(t, "startedAfter=2026-10-13T00%3A00%3A00Z", r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"data":[],"nextCursor":null}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL+"/api/v1/", "secret", srv.Client())
	var out struct {
		Data []any `json:"data"`
	}
	err := c.Get(context.Background(), "/executions", url.Values{"startedAfter": {"2026-10-13T00:00:00Z"}}, &out)
	require.NoError(t, err)
}

func TestClientErrors(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		contentType string
		body        string
		check       func(t *testing.T, err error)
	}{
		{"unauthorized", 401, "application/json", `{"message":"Unauthorized"}`, func(t *testing.T, err error) {
			var apiErr *APIError
			require.ErrorAs(t, err, &apiErr)
			require.Equal(t, 401, apiErr.StatusCode)
			require.Equal(t, "Unauthorized", apiErr.Message)
		}},
		{"html instead of api", 200, "text/html; charset=utf-8", `<!DOCTYPE html>`, func(t *testing.T, err error) {
			require.ErrorIs(t, err, ErrNotJSON)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			err := NewClient(srv.URL, "secret", srv.Client()).Get(context.Background(), "/workflows", nil, nil)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "secret")
			tc.check(t, err)
		})
	}
}

func TestAPIErrorWithoutMessage(t *testing.T) {
	err := error(&APIError{StatusCode: 502})
	require.Equal(t, "n8n API returned 502", err.Error())
	require.False(t, errors.Is(err, ErrNotJSON))
}
