# Catalog submission – prepared texts

Fields for grafana.com → _My Plugins_ → _Submit plugin_. Fill in URL and SHA1 from the GitHub release.

- **Plugin ID:** `nluecke-n8n-datasource`
- **Archive URL:** `https://github.com/nluecke/grafana-n8n-datasource/releases/download/v1.0.0/nluecke-n8n-datasource-1.0.0.zip`
- **SHA1:** from the release (`nluecke-n8n-datasource-1.0.0.zip.sha1` or `shasum <zip>`)
- **Source code URL:** `https://github.com/nluecke/grafana-n8n-datasource` (tag `v1.0.0`)

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
- `@grafana/plugin-validator` 0.49.5 on the release archive: no errors (only "unsigned plugin" and an optional sponsorship-link recommendation).
- Community plugin, not affiliated with n8n GmbH; the plugin does not use n8n's logo.
