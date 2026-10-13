//go:build integration

package plugin

import (
	"os"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// Run with: set -a; . ./.env; set +a; go test -tags integration ./pkg/plugin -run IntegrationQueries -v
func TestIntegrationQueries(t *testing.T) {
	targets := []struct {
		name, urlEnv, keyEnv string
		jsonData             map[string]any
	}{
		{"local", "N8N_URL", "N8N_API_KEY", map[string]any{"cacheTtlSeconds": 0}},
		{"remote", "N8N_CLOUD_URL", "N8N_CLOUD_API_KEY", map[string]any{"tlsSkipVerify": true, "cacheTtlSeconds": 0}},
	}
	queries := []struct {
		name, queryType string
		hours           int
		model           map[string]any
	}{
		{"executions 24h", QueryTypeExecutions, 24, map[string]any{}},
		{"count by status 24h", QueryTypeExecutionStats, 24, map[string]any{"metric": "count"}},
		{"error rate 24h", QueryTypeExecutionStats, 24, map[string]any{"metric": "errorRate", "groupBy": "none"}},
		{"duration by workflow 24h", QueryTypeExecutionStats, 24, map[string]any{"metric": "duration", "groupBy": "workflow"}},
		{"executions 7d", QueryTypeExecutions, 24 * 7, map[string]any{}},
		{"workflows", QueryTypeWorkflows, 1, map[string]any{}},
		{"insights summary 7d", QueryTypeInsights, 24 * 7, map[string]any{"insightsMode": "summary"}},
		{"insights summary 30d", QueryTypeInsights, 24 * 30, map[string]any{"insightsMode": "summary"}},
		{"insights series 24h auto", QueryTypeInsights, 24, map[string]any{"insightsMode": "timeseries"}},
		{"insights series 7d daily", QueryTypeInsights, 24 * 7, map[string]any{"insightsMode": "timeseries", "insightsBucket": "1d"}},
		{"audit", QueryTypeAudit, 1, map[string]any{}},
	}
	for _, tg := range targets {
		t.Run(tg.name, func(t *testing.T) {
			url, key := os.Getenv(tg.urlEnv), os.Getenv(tg.keyEnv)
			if url == "" || key == "" {
				t.Skipf("%s / %s not set", tg.urlEnv, tg.keyEnv)
			}
			ds := newTestDatasource(t, url, tg.jsonData, map[string]string{"apiKey": key})
			for _, q := range queries {
				now := time.Now()
				tr := backend.TimeRange{From: now.Add(-time.Duration(q.hours) * time.Hour), To: now}
				start := time.Now()
				res := runQuery(t, ds, q.queryType, tr, q.model)
				took := time.Since(start).Round(time.Millisecond)
				if res.Error != nil {
					t.Logf("%-26s %6s  ERROR %d: %v", q.name, took, res.Status, res.Error)
					continue
				}
				rows := 0
				for _, f := range res.Frames {
					rows += f.Rows()
				}
				t.Logf("%-26s %6s  frames=%d rows=%d notices=[%s]", q.name, took, len(res.Frames), rows, noticeTexts(res.Frames))
			}
		})
	}
}
