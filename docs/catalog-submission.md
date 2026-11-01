# Catalog submission – prepared texts

Fields for grafana.com → _My Plugins_ → _Submit plugin_, filled in for the version currently under review.

- **Plugin ID:** `nluecke-n8n-datasource`
- **Archive URL:** `https://github.com/nluecke/grafana-n8n-datasource/releases/download/v1.0.3/nluecke-n8n-datasource-1.0.3.zip`
- **SHA1:** `5476d895e3f1f66a30ba6b4791502a731d762b9d` (also in `nluecke-n8n-datasource-1.0.3.zip.sha1` of the release)
- **Source code URL:** `https://github.com/nluecke/grafana-n8n-datasource` (tag `v1.0.3`, commit `8336eb1`)
- **Provenance:** build attestation for the archive (`gh attestation verify nluecke-n8n-datasource-1.0.3.zip --repo nluecke/grafana-n8n-datasource`)

## Testing guidance for reviewers

The plugin needs an n8n instance with the public API. A complete local environment starts with one command:

```bash
git clone https://github.com/nluecke/grafana-n8n-datasource && cd grafana-n8n-datasource
make dev   # requires Docker, Go, Node.js, Python 3
```

`make dev` builds the plugin, starts n8n and Postgres, creates an n8n owner, a read-only API key and four
seed workflows that execute every minute (one fails randomly, one takes 20 s, one is a webhook), and starts
Grafana on http://localhost:3000 with the data source and the bundled dashboard
"n8n Workflow Executions (API)" provisioned. After a few minutes the dashboard shows data.

Without the dev stack: create a free self-hosted n8n (`docker run -p 5678:5678 n8nio/n8n`), create an API key
under Settings → n8n API, and configure the data source with URL `http://<host>:5678/api/v1` and the key.

## Notes for the review

- Backend plugin (Go). The n8n client only sends allowlisted read requests (GET on workflows, executions, tags,
  projects, insights; POST /audit, which generates a report without changing data), enforced and tested in
  `pkg/n8n/client.go`.
- Private Data Source Connect is supported through the SDK HTTP client and was tested locally with a SOCKS5
  proxy (`make pdc`).
- `@grafana/plugin-validator` 0.52.1 on the v1.0.3 archive with gosec and govulncheck: exit 0, warnings only. All binaries built with Go 1.26.10; govulncheck binary scan clean.
  - gosec G101 ×2: false positives, the header name `X-N8N-API-KEY` (`pkg/n8n/client.go`) and the dummy key of the test fake (`pkg/n8n/n8ntest/server.go`). Both carry `#nosec` comments, which the validator ignores.
  - govulncheck source scan ("update Go toolchain"): runs with the validator environment's own Go toolchain, not the one the plugin is built with.
- Community plugin, not affiliated with n8n GmbH; the plugin does not use n8n's logo.
