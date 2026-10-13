# n8n data source for Grafana – development

Grafana data source plugin (`nluecke-n8n-datasource`, Go backend) for n8n Cloud and self-hosted n8n via
the n8n public API. User documentation: [src/README.md](src/README.md) (shown in the plugin catalog).

## Repository layout

| Path                              | Content                                                                                                               |
| --------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| `pkg/n8n`                         | Read-only n8n API client: allowlist, cursor paging, retries, cache; `n8ntest` fake server                             |
| `pkg/plugin`                      | Query types, statistics, health check, resource endpoints                                                             |
| `pkg/models`, `pkg/schema`        | Settings model and its [dsconfig](https://github.com/grafana/dsconfig) schema (`dsconfig.json`) with conformance test |
| `src`                             | Config editor, query editor, variable support, bundled dashboard                                                      |
| `provisioning`                    | Datasources and dashboards for the dev stack                                                                          |
| `scripts/dev`                     | Bootstrap of a fresh n8n, dashboard generator                                                                         |
| `scripts/probe`, `seed/workflows` | API probes and seed workflows from the feasibility spike                                                              |
| `docs`                            | PoC findings, dsconfig feedback, publishing checklist, n8n OpenAPI specs                                              |

## Prerequisites

Go (version from `go.mod`), Node.js (see `.nvmrc`), Docker with Compose v2.24+, Python 3.

## Dev stack

```bash
cp .env.example .env    # optional: set N8N_CLOUD_URL / N8N_CLOUD_API_KEY for a second instance
make dev
```

`make dev` builds frontend and backend, starts n8n + Postgres, and on a fresh n8n

1. creates the owner (`N8N_OWNER_EMAIL` / `N8N_OWNER_PASSWORD`),
2. creates a read-only API key for Grafana (`N8N_API_KEY`) and a key for seeding (`N8N_SEED_API_KEY`), written to `.env`,
3. imports the seed workflows (schedule every minute, one failing randomly, one slow, one webhook),

then starts Grafana OSS with the plugin and two provisioned datasources (`n8n`, `n8n remote`) and the
bundled dashboard.

| Service                   | URL                                    |
| ------------------------- | -------------------------------------- |
| Grafana (anonymous admin) | http://localhost:3000 (`GRAFANA_PORT`) |
| n8n                       | http://localhost:5678 (`N8N_PORT`)     |

Steps 1–2 use n8n's internal REST API (`/rest/owner/setup`, `/rest/login`, `/rest/api-keys`), which may
change between n8n versions. Generate load with `N=5000 TARGET=local scripts/probe/webhook-load.sh`.

Other targets: `make down` (keep data), `make clean` (delete data), `make logs`, `make dashboard`
(regenerate `src/dashboards/n8n-workflow-executions.json`).

## Tests

```bash
make test               # go test, typecheck, lint, jest
make e2e                # Playwright against the running dev stack
make test-integration   # all query types against the instances in .env
```

Release archive and validation:

```bash
mage -v && npm run build
cp -r dist nluecke-n8n-datasource && zip -qr nluecke-n8n-datasource.zip nluecke-n8n-datasource
npx @grafana/plugin-validator@latest -sourceCodeUri file://$PWD nluecke-n8n-datasource.zip
```

## Further reading

- [docs/poc-findings.md](docs/poc-findings.md) – measurements and API behaviour per n8n version and licence
- [docs/dsconfig-feedback.md](docs/dsconfig-feedback.md) – issue candidates for grafana/dsconfig
- [docs/publishing.md](docs/publishing.md) – signing and catalog submission checklist
