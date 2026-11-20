# Changelog

## 1.0.7

- Security: built with Go 1.26.11 (GO-2026-6859, net/http) and klauspost/compress v1.20.2 (GO-2026-6865)
- Execution stats: automatic time buckets are capped at 150 and rounded up to standard intervals (1m, 5m, 15m, 1h, …), so charts in Explore and in panels without _Max data points_ no longer turn into thin stripes. Panels with _Whole range_ buckets and the bundled dashboard are unchanged
- Execution stats: series grouped by status have fixed colours (success green, error red, crashed dark red, canceled grey, …), also without panel overrides
- Bundled dashboard _n8n Workflow Executions (API)_ redesigned: value-coloured stat tiles, ranked bar gauges for top failing workflows and error rate per workflow, smooth duration percentiles on a logarithmic axis, readable table columns. Existing installations: re-import it on the data source's **Dashboards** tab
- Catalog page: shorter overview; configuration, query types, troubleshooting and limitations moved to [docs/guide](https://github.com/nluecke/grafana-n8n-datasource/tree/main/docs/guide); new screenshots including the query editor in Explore
- Dependencies: grafana-plugin-sdk-go 0.299.0, Grafana packages 13.3.1, @grafana/plugin-ui 0.18.0; release built with build-plugin 2.0.1

## 1.0.6

- First signed release (community signature); no code changes

## 1.0.5

- Health check: fixed messages per failure type (TLS, network, PDC, unexpected response, unexpected error) instead of the underlying error text, which could reveal host names, addresses and ports; details go to the server log
- Query editor: pressing Enter in Metric, Group by, Bucket or Insights bucket without a matching option no longer crashes the editor on Grafana 13.3
- Reject data source URLs that contain a user name or password

## 1.0.4

- Rename the API key header constant and the test fake's dummy key so gosec G101 (hardcoded credentials) no longer reports false positives; the plugin validator ignores `#nosec` comments

## 1.0.3

- Build with Go 1.26.10 (GO-2026-6809: distrust compromised intermediate certificates in crypto/x509) and golang.org/x/net v0.62.0
- Frontend built against Grafana 13.3 packages (`@grafana/data`, `ui`, `runtime`, `schema`, `i18n` 13.3.0, `@grafana/plugin-ui` 0.17.9); still compatible with Grafana 13.2

## 1.0.2

- Release build pinned to Go 1.26.9. The 1.0.1 tag was not released: its release build resolved Go 1.26.8 and stopped at the go.mod minimum.

## 1.0.1

- Build with Go 1.26.9 or later and golang.org/x/net v0.60.0 (Go vulnerability fixes reported by govulncheck)
- Mark two gosec G101 false positives (HTTP header name, test fixture)
- CI: security scan job with gosec and govulncheck

## 1.0.0

Initial release.

- Query types: executions, execution statistics (count, success and error rate, duration percentiles; per
  interval or whole range; grouped by status or workflow), workflows, n8n Insights (summary and time series),
  security audit
- Server-side time, status and workflow filters where n8n supports them, cursor paging with a configurable
  page limit and clear "Incomplete data" notices
- Health check with actionable messages for wrong keys, disabled API, wrong URL, TLS and network errors
- TLS options (skip verify, custom CA) and Private Data Source Connect
- Template variables for workflows, projects and tags; alerting support
- Bundled dashboard "n8n Workflow Executions (API)"
