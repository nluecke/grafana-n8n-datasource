package plugin

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/require"
)

const testAPIKey = "test-key-do-not-log"

// fakeN8n mimics the public API behaviour observed against n8n 2.42.7 and an
// older self-hosted release (see docs/poc-findings.md).
type fakeN8n struct {
	status         int
	contentType    string
	body           string
	noStartedAfter bool // older n8n: 400 "Unknown query parameter 'startedAfter'"
}

func (f fakeN8n) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if f.status != 0 {
		w.Header().Set("Content-Type", f.contentType)
		w.WriteHeader(f.status)
		_, _ = w.Write([]byte(f.body))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Header.Get("X-N8N-API-KEY") != testAPIKey {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Unauthorized"}`))
		return
	}
	if r.URL.Path == "/api/v1/executions" && r.URL.Query().Has("startedAfter") && f.noStartedAfter {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"Unknown query parameter 'startedAfter'"}`))
		return
	}
	_, _ = w.Write([]byte(`{"data":[],"nextCursor":null}`))
}

func newTestDatasource(t *testing.T, url string, jsonData map[string]any, secure map[string]string) *Datasource {
	t.Helper()
	raw, err := json.Marshal(jsonData)
	require.NoError(t, err)
	inst, err := NewDatasource(context.Background(), backend.DataSourceInstanceSettings{
		Type:                    "nluecke-n8n-datasource",
		URL:                     url,
		JSONData:                raw,
		DecryptedSecureJSONData: secure,
	})
	require.NoError(t, err)
	return inst.(*Datasource)
}

func checkHealth(t *testing.T, ds *Datasource) *backend.CheckHealthResult {
	t.Helper()
	res, err := ds.CheckHealth(context.Background(), &backend.CheckHealthRequest{})
	require.NoError(t, err)
	require.NotContains(t, res.Message, testAPIKey)
	return res
}

func TestCheckHealth(t *testing.T) {
	cases := []struct {
		name       string
		server     fakeN8n
		path       string
		apiKey     string
		wantStatus backend.HealthStatus
		wantMsg    string
	}{
		{"ok with time filter", fakeN8n{}, "/api/v1", testAPIKey, backend.HealthStatusOk, "Server-side time filter available"},
		{"ok old version", fakeN8n{noStartedAfter: true}, "/api/v1", testAPIKey, backend.HealthStatusOk, "no server-side time filter"},
		{"wrong key or api disabled", fakeN8n{}, "/api/v1", "wrong", backend.HealthStatusError, "N8N_PUBLIC_API_DISABLED"},
		{"forbidden", fakeN8n{status: 403, contentType: "application/json", body: `{"message":"Forbidden"}`}, "/api/v1", testAPIKey, backend.HealthStatusError, "free trial"},
		{"not found", fakeN8n{status: 404, contentType: "application/json", body: `{"message":"not found"}`}, "/api/v1", testAPIKey, backend.HealthStatusError, "end with /api/v1"},
		{"not found without api path", fakeN8n{status: 404, contentType: "application/json", body: `{"message":"not found"}`}, "", testAPIKey, backend.HealthStatusError, "/api/v1."},
		{"ui instead of api", fakeN8n{status: 200, contentType: "text/html; charset=utf-8", body: "<!DOCTYPE html>"}, "", testAPIKey, backend.HealthStatusError, "/api/v1."},
		{"rate limited", fakeN8n{status: 429, contentType: "application/json", body: `{}`}, "/api/v1", testAPIKey, backend.HealthStatusError, "429"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.server)
			defer srv.Close()
			ds := newTestDatasource(t, srv.URL+tc.path, nil, map[string]string{"apiKey": tc.apiKey})
			res := checkHealth(t, ds)
			require.Equal(t, tc.wantStatus, res.Status, res.Message)
			require.Contains(t, res.Message, tc.wantMsg)
		})
	}
}

func TestCheckHealthSettingsErrors(t *testing.T) {
	cases := []struct {
		name     string
		url      string
		jsonData map[string]any
		secure   map[string]string
		wantMsg  string
	}{
		{"missing url", "", nil, map[string]string{"apiKey": testAPIKey}, "URL is missing"},
		{"bad scheme", "ftp://n8n", nil, map[string]string{"apiKey": testAPIKey}, "http:// or https://"},
		{"missing key", "http://n8n:5678/api/v1", nil, nil, "API key is missing"},
		{"missing ca", "https://n8n/api/v1", map[string]any{"tlsAuthWithCACert": true}, map[string]string{"apiKey": testAPIKey}, "CA certificate is missing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := checkHealth(t, newTestDatasource(t, tc.url, tc.jsonData, tc.secure))
			require.Equal(t, backend.HealthStatusError, res.Status)
			require.Contains(t, res.Message, tc.wantMsg)
		})
	}
}

func TestCheckHealthTLS(t *testing.T) {
	srv := httptest.NewTLSServer(fakeN8n{})
	defer srv.Close()
	secure := map[string]string{"apiKey": testAPIKey}

	t.Run("unknown CA", func(t *testing.T) {
		res := checkHealth(t, newTestDatasource(t, srv.URL+"/api/v1", nil, secure))
		require.Equal(t, backend.HealthStatusError, res.Status)
		require.Contains(t, res.Message, "With CA cert")
	})

	t.Run("skip verify", func(t *testing.T) {
		res := checkHealth(t, newTestDatasource(t, srv.URL+"/api/v1", map[string]any{"tlsSkipVerify": true}, secure))
		require.Equal(t, backend.HealthStatusOk, res.Status, res.Message)
	})

	t.Run("custom CA", func(t *testing.T) {
		caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
		withCA := map[string]string{"apiKey": testAPIKey, "tlsCACert": string(caPEM)}
		res := checkHealth(t, newTestDatasource(t, srv.URL+"/api/v1", map[string]any{"tlsAuthWithCACert": true}, withCA))
		require.Equal(t, backend.HealthStatusOk, res.Status, res.Message)
	})
}

func TestCheckHealthUnreachable(t *testing.T) {
	srv := httptest.NewServer(fakeN8n{})
	url := srv.URL
	srv.Close()
	res := checkHealth(t, newTestDatasource(t, url+"/api/v1", nil, map[string]string{"apiKey": testAPIKey}))
	require.Equal(t, backend.HealthStatusError, res.Status)
	require.Contains(t, res.Message, "Cannot reach n8n")
}
