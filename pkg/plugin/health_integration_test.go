//go:build integration

package plugin

import (
	"os"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/require"
)

// Run with: set -a; . ./.env; set +a; go test -tags integration ./pkg/plugin -run Integration -v
func TestIntegrationCheckHealth(t *testing.T) {
	targets := []struct {
		name, urlEnv, keyEnv string
		jsonData             map[string]any
	}{
		{"local", "N8N_URL", "N8N_API_KEY", nil},
		{"remote", "N8N_CLOUD_URL", "N8N_CLOUD_API_KEY", map[string]any{"tlsSkipVerify": true}},
	}
	for _, tg := range targets {
		t.Run(tg.name, func(t *testing.T) {
			url, key := os.Getenv(tg.urlEnv), os.Getenv(tg.keyEnv)
			if url == "" || key == "" {
				t.Skipf("%s / %s not set", tg.urlEnv, tg.keyEnv)
			}
			ds := newTestDatasource(t, url, tg.jsonData, map[string]string{"apiKey": key})
			res, err := ds.CheckHealth(t.Context(), &backend.CheckHealthRequest{})
			require.NoError(t, err)
			require.NotContains(t, res.Message, key)
			t.Logf("%s: %s", res.Status, res.Message)
			require.Equal(t, backend.HealthStatusOk, res.Status)

			bad := newTestDatasource(t, url, tg.jsonData, map[string]string{"apiKey": "invalid"})
			res, err = bad.CheckHealth(t.Context(), &backend.CheckHealthRequest{})
			require.NoError(t, err)
			t.Logf("invalid key: %s", res.Message)
			require.Equal(t, backend.HealthStatusError, res.Status)
		})
	}
}
