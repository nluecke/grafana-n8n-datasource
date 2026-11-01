# Changelog

## 1.0.3

- Build with Go 1.26.10 (GO-2026-6809: distrust compromised intermediate certificates in crypto/x509) and golang.org/x/net v0.62.0

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
