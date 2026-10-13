# PoC Findings – n8n Datasource for Grafana

Format: date – finding – consequence.

## Phase 1 – Spike

### Market check

- 2026-10-13 – grafana.com plugin API (`/api/plugins`, 370 plugins, plus `query=n8n` incl. enterprise) returns **no** n8n plugin. GitHub repository search `grafana n8n datasource` returns 0 repositories. – No maintained plugin exists; continue.

### Environments

- 2026-10-13 – Local n8n OSS: `n8nio/n8n:latest` = **2.42.7**, Postgres 16, `docker-compose.yaml` (initially `docker-compose.dev.yml`). Public API answers 401 without key → enabled by default.
- 2026-10-13 – Second instance ("remote"): an internal **self-hosted** n8n of an older release with a licence that includes Insights beyond Community Edition, **not `*.app.n8n.cloud`**. It serves a markedly older public API spec (fewer paths, no `startedAfter`). `/projects` → 403. – This instance does **not** cover the n8n Cloud path; a real `*.app.n8n.cloud` account is still needed for that.
- 2026-10-13 – Remote TLS certificate is signed by an internal CA not in the system trust store (`unable to get local issuer certificate`). – Plugin needs `tlsSkipVerify` and ideally a custom CA option (`tlsCACert`), like Grafana's HTTP datasources.

### API surface (spec in `docs/n8n-openapi.yml` = local 2.42.7, `docs/n8n-openapi-older.yml` = remote (older release))

- 2026-10-13 – Spec served unauthenticated at `<base>/api/v1/openapi.yml`. Version field is `1.1.1` on both, although the path sets differ heavily. – `info.version` is useless for feature detection; detect features by probing or by spec content.
- 2026-10-13 – `GET /executions` filters: `status` (single value enum: canceled, crashed, error, new, running, success, unknown, waiting), `workflowId`, `projectId`, `includeData`, `limit` (default 100, **max 250**), `cursor`. **2.42.7 additionally has `startedAfter` / `startedBefore`** (server-side time filter); the older remote version does not. – Backend: use server-side filter when available, else page newest-first with client-side cutoff on `startedAt`.
- 2026-10-13 – `GET /insights/summary` (`insights:read`) exists in both versions. Params: `startDate`, `endDate` (default last 7 days), `projectId`. Returns only **aggregate values** `total`, `failed`, `failureRate`, `timeSaved`, `averageRunTime` each with `value`, `deviation` (vs. previous period), `unit`. **No time series, no per-workflow breakdown.** – A time series would require one call per bucket; per-workflow insights are not available through the public API.
- 2026-10-13 – Remote: `/insights/summary` returned instance-wide numbers while the same key saw **0 workflows and 0 executions** via `/workflows` and `/executions`; `/users` was readable, `POST /audit` and `GET /projects` → 403. – Later explained: the key only sees its owner's personal project, and it is a scoped key (see Phase 5).
- 2026-10-13 – Auth: valid key 200, invalid key 401 (both instances).

- 2026-10-13 – `GET /executions` item fields (without `includeData`): `id`, `finished`, `mode`, `retryOf`, `retrySuccessId`, `status`, `startedAt`, `stoppedAt`, `workflowId`, `waitTill`. **No workflow name.** – Duration = `stoppedAt - startedAt`; workflow names must be resolved via a cached `/workflows` list.
- 2026-10-13 – Pagination: `nextCursor` is base64 JSON `{"lastId":"<execId>","limit":N}` → keyset pagination by id, newest first; stable under concurrent inserts.
- 2026-10-13 – `status` accepts **one** value only (`error,success` → 400). – Multi-status filter = several requests or client-side filtering.
- 2026-10-13 – Version differences 2.42.7 (local) vs remote (older):
  | Behaviour                                                                                                                                                              | 2.42.7                  | remote (older)                               |
  | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------- | -------------------------------------------- |
  | `startedAfter`/`startedBefore`                                                                                                                                         | yes                     | 400 `Unknown query parameter 'startedAfter'` |
  | `limit` > 250                                                                                                                                                          | silently clamped to 250 | 400 `limit must be <= 250`                   |
  | unencoded `:` in query values                                                                                                                                          | accepted                | 400 `must be url encoded`                    |
  | – Client must always URL-encode, cap `limit` at 250, and can detect the time filter by probing once (400 "Unknown query parameter" → fall back to client-side cutoff). |
- 2026-10-13 – `GET /workflows` returns full node definitions (~2 KB per simple workflow). Filters: `active`, `tags`, `name`, `projectId`, `excludePinnedData`. – Cache workflow list; consider `excludePinnedData=true`.
- 2026-10-13 – `POST /audit` works on local Community Edition (returns risk reports); 403 on remote with this key. `GET /projects` → 403 on Community (`Your license does not allow for feat:projectRole:admin`). – `audit` query type optional; project filter/dropdown only where licensed, must degrade gracefully.

### Measurements (local, n8n 2.42.7 + Postgres, M-series Mac, 3 runs)

- 2026-10-13 – Load: 5,000 webhook executions in 69 s (P=10), all 200; ~10 % error by design.
  | Window                                                                                                                                                                                                                                                                                    | Pages × limit           | Total time |
  | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------- | ---------- |
  | 100 executions                                                                                                                                                                                                                                                                            | 1 × 100                 | 14–48 ms   |
  | 1,000 executions                                                                                                                                                                                                                                                                          | 4 × 250                 | 55–59 ms   |
  | 5,006 executions                                                                                                                                                                                                                                                                          | 21 × 250                | 234–270 ms |
  | 5,006 executions                                                                                                                                                                                                                                                                          | 51 × 100                | 470–600 ms |
  | 2,197 of 5,006 (partial window)                                                                                                                                                                                                                                                           | 9 × 250, client cutoff  | 201 ms     |
  | same                                                                                                                                                                                                                                                                                      | 9 × 250, `startedAfter` | 112 ms     |
  | (Correction 2026-10-13: the probe compared timestamps as strings — `"…45.007Z" < "…45Z"` — so window row counts are off by a few dozen rows at the boundary; timings are unaffected. Re-check with parsed times: `startedAfter` returns no older rows; `startedBefore` is **inclusive**.) |
  | – Far below the 5 s Go threshold even without cache. Remote/WAN latency (~130 ms per request against the remote instance) will dominate: 5,000 executions ≈ 21 requests ≈ 3 s → cache + `maxPages` needed.                                                                                |
- 2026-10-13 – Rate limit: 1,500 requests in 10 s (20 parallel) on `/executions?limit=1` → all 200, no rate-limit headers. – No public-API rate limit on self-hosted by default; keep retry/backoff for reverse proxies and Cloud (unverified).

### Pruning & retention

- 2026-10-13 – Defaults read from n8n 2.42.7 source: `EXECUTIONS_DATA_PRUNE=true`, `EXECUTIONS_DATA_MAX_AGE=336` h, `EXECUTIONS_DATA_PRUNE_MAX_COUNT=10000`, `saveDataOnSuccess/OnError=all`. With 5,000 executions/hour the local history only reaches back ~2 h. – Dashboard window is bounded by retention; README + frame notice.
- 2026-10-13 – n8n Cloud docs (manage-cloud/cloud-data-management): Start/Starter max 2,500 executions / 7 days; Pro 25,000 / 30 days; Enterprise 50,000 / unlimited time.

### Insights

- 2026-10-13 – Community Edition: summary allowed only for ranges ≤ 7 days (8 days → 400 `The selected date range exceeds the maximum history allowed by your license`); sub-day ranges → 400 `Hourly data is not available with your current license`.
- 2026-10-13 – Remote (licensed): 1 h … 365 d all accepted, hourly allowed. – A coarse time series is possible via one call per bucket (licensed: hourly buckets; CE: daily buckets, ≤ 7 days).
- 2026-10-13 – Insights lag: raw events flushed every 30 s, but `/insights/summary` reads compacted data; compaction runs every 60 min (`compactionIntervalMinutes=60`). Locally 5,010 raw rows existed while summary still returned 0. – Insights are not near-real-time; panels must say so.
- 2026-10-13 – Insights are retained independently of execution pruning (default 365 d, hourly → daily after 90 d, → weekly after 180 d).

### Disabled API / health check

- 2026-10-13 – `N8N_PUBLIC_API_DISABLED=true`: `/api/v1/workflows` returns **401 `{"message":"Unauthorized"}` — identical to a wrong key**; `/api/v1/openapi.yml` is still served (200). – Health check cannot distinguish "API disabled" from "wrong key"; the 401 message must mention both.
- 2026-10-13 – Wrong base path (e.g. missing `/api/v1`, or `/api/v2`) returns **200 `text/html`** (n8n UI SPA). – Health check must verify `Content-Type: application/json` and hint "URL must end with /api/v1".

### n8n Cloud vs self-hosted (docs research, not yet verified against a Cloud instance)

- Cloud: API on paid plans only (Starter, Pro, Enterprise), not in the free trial; base URL `https://<name>.app.n8n.cloud/api/v1`; no API playground on Cloud.
- Scopes: Enterprise only; all other keys have full access.
- Self-hosted: API on all editions; can be disabled via `N8N_PUBLIC_API_DISABLED`, path changed via `N8N_PUBLIC_API_ENDPOINT` (default `api`), Swagger UI via `N8N_PUBLIC_API_SWAGGERUI_DISABLED`.
- No official statement on public-API rate limits found (only unverified third-party claims of 1,000 req / 5 min per IP).
- Same API code base; Cloud runs a recent n8n version, so feature differences come from **version and license**, not from Cloud vs self-hosted as such.

## Go/No-Go (2026-10-13, preliminary)

**Go for self-hosted.** All Go criteria are met: time window via server-side filter (new versions) or keyset pagination with cutoff (old versions); status and duration derivable; 5,000 executions in < 0.3 s locally; auth identical on both instances. Insights add long-range aggregates (licence-dependent, ≤ 7 days on CE, 60 min lag).

Open before the Cloud part is confirmed:

- Real `*.app.n8n.cloud` account for verification (version, rate limits, Insights ranges per plan).
- Remote key sees 0 workflows/executions → role/scopes of the key owner unclear; seed data on the remote instance pending approval.

## Phase 2 – Scaffold & configuration

- 2026-10-13 – Repository: [github.com/nluecke/grafana-n8n-datasource](https://github.com/nluecke/grafana-n8n-datasource), open source (Apache-2.0).
- 2026-10-13 – Scaffold `@grafana/create-plugin` 7.12.2, Go SDK v0.297.0, Grafana dependency `>=13.2.0`.
- 2026-10-13 – dsconfig schema lives in `pkg/schema/dsconfig.json` (location from `PUBLISH-SCHEMA.md`, not `src/` as initially planned). Conformance suite green; drift detection verified.
- 2026-10-13 – TLS: setting keys follow SDK conventions, so `HTTPClientOptions` provides skip-verify, custom CA and PDC (secure socks proxy) without custom code. Added `tlsAuthWithCACert` + `tlsCACert` because the remote instance uses an internal CA.
- 2026-10-13 – Health check verified against both real instances: local → "Server-side time filter available", remote → "no server-side time filter" (detected via 400 on `startedAfter`); invalid key → 401 message mentioning `N8N_PUBLIC_API_DISABLED`.

## Phase 3 – Backend queries

- 2026-10-13 – `startedAfter`/`startedBefore` re-verified with parsed timestamps: exact, `startedBefore` inclusive. Executions must be compared as `time.Time`, never as strings.
- 2026-10-13 – Paging without server filter stops only when a page lies **completely** before the range (one extra page compared to "oldest item before range"), because ids and start times are not strictly ordered (queued executions in queue mode).
- 2026-10-13 – `POST /audit` returns `[]` instead of an object when no risk is found (n8n source: `if (reports.length === 0) return []`).
- 2026-10-13 – **Insights licence errors are 403**, not 400 (Phase 1 only looked at the message). Community Edition rules measured on 2.42.7:
  | Range                                                                                                                                   | Result                                                                            |
  | --------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------- |
  | < 24 h (e.g. 23 h, or today 00:00 → now)                                                                                                | 403 `Hourly data is not available with your current license`                      |
  | ≥ 24 h within the last ~7 days (also `endDate` in the future)                                                                           | 200                                                                               |
  | starting 8 days ago                                                                                                                     | 403 `The selected date range exceeds the maximum history allowed by your license` |
  | – Time series buckets always span full days (current bucket ends tomorrow 00:00); `auto` falls back from hourly to daily with a notice. |
- 2026-10-13 – Insights were still 0 locally ~45 min after 5,000 executions (raw rows present, compaction every 60 min) → confirms the lag.
- 2026-10-13 – Real-instance run (`go test -tags integration`, cache off):
  | Query                       | local 2.42.7 (CE)                                      | remote (older, licensed)                   |
  | --------------------------- | ------------------------------------------------------ | ------------------------------------------ |
  | executions 24 h             | 349 ms, 5,000 rows, truncated at 20 pages (load spike) | 287 ms, history notice (only ~2 h of data) |
  | count by status 24 h        | 193 ms                                                 | 120 ms                                     |
  | duration by workflow 24 h   | 200 ms                                                 | 119 ms                                     |
  | workflows                   | 8 ms                                                   | 59 ms                                      |
  | insights summary 7 d / 30 d | 4 ms / licence error with hint                         | 51 ms / 68 ms                              |
  | insights series 24 h auto   | 82 ms, daily fallback                                  | 458 ms, 25 hourly buckets                  |
  | audit                       | 150 ms                                                 | 403 (key)                                  |
- 2026-10-13 – Backend binary builds with `mage build:linuxARM64` (26 MB, static).

## Phase 4 – Frontend

- 2026-10-13 – Grafana 13.2 marks `Select`/`MultiSelect` as deprecated; the query editor uses `Combobox`/`MultiCombobox`.
- 2026-10-13 – Multi-value template variables are expanded into list entries in `applyTemplateVariables` (custom format function with a separator that cannot occur in n8n ids); the backend additionally drops `$__all`.
- 2026-10-13 – Variables (workflows, projects, tags) via `CustomVariableSupport` on top of the backend resource endpoints.
- 2026-10-13 – Jest prints "worker process has failed to exit gracefully": open `MESSAGEPORT` handle created by `@grafana/ui` (`TruncatedText` imports `react-dom/server`), not by plugin code. Tests pass.

## Phase 5 – Local stack

- 2026-10-13 – **API key scopes are enforced on Community Edition (2.42.7)**, contrary to the docs ("Non-Enterprise API keys have full access") and the original assumption. A key created with `workflow:list, workflow:read, execution:list, execution:read` gets 200 on `GET /workflows`, 403 on `POST /workflows` and 403 on `GET /tags`. The UI/REST `POST /rest/api-keys` requires an explicit `scopes` array (107 scopes available). – Recommend a read-only key for every edition: `workflow:list, workflow:read, execution:list, execution:read, tag:list, project:list, insights:read, securityAudit:generate`. This also explains the remote key's 403 on `/projects` and `/audit` (scoped key).
- 2026-10-13 – `/healthz` answers 200 while n8n still serves "n8n is starting up"; readiness must be checked via `/healthz/readiness` or a JSON answer from `/rest/settings`.
- 2026-10-13 – Fresh-instance bootstrap via internal REST (not public API): `POST /rest/owner/setup`, `POST /rest/login` (field `emailOrLdapLoginId`), `POST /rest/api-keys` with `label`, `expiresAt: null`, `scopes`. Internal API, may change between n8n versions; only used by the dev stack.
- 2026-10-13 – `make dev` verified on a fresh, isolated n8n (separate compose project): owner + read-only Grafana key + seed key created, seeds imported, second run idempotent. The read-only key gets 200 on workflows/tags/insights/audit and 403 on `POST /workflows`.
- 2026-10-13 – Insights compaction timer restarts with every n8n container start (default 60 min), so a frequently restarted dev instance shows no Insights. Dev stack sets `N8N_INSIGHTS_COMPACTION_INTERVAL_MINUTES=2`. After compaction, local Insights (5,189 executions, 512 failed, 9.9 %) matched the execution-list statistics (≈5k, 490 failed, 90.2 % success; difference = executions outside the truncated list).
- 2026-10-13 – Bundled dashboard "n8n Workflow Executions (API)" verified in Grafana OSS 13.2.2 against local n8n 2.42.7 (CE) and the remote older instance with the same JSON (datasource variable). Stat tiles use the new `bucket: range` option (one value for the whole time range) instead of averaging per-interval rates.
- 2026-10-13 – Ports 3000/2345 were taken by another local Grafana plugin stack; `GRAFANA_PORT` / `N8N_PORT` are configurable, the delve port is not published.

## Phase 6 – Quality

- 2026-10-13 – `go test ./pkg/...` (incl. `-race`), `go vet`, `npm run typecheck`, `npm run lint`, `npm run test:ci` (11 tests) green. E2E with `@grafana/plugin-e2e` against the dev stack: 7/7 (config editor smoke, Save & test OK for the provisioned n8n, missing key, URL without `/api/v1`, query editor, workflows query returns seed data).
- 2026-10-13 – With `Accept: application/json` (as the plugin sends) n8n answers a wrong path with **404 JSON**, not with the HTML UI seen with plain curl in Phase 1. Health message for 404 now suggests `<url>/api/v1`.
- 2026-10-13 – `@grafana/plugin-validator` (strict, with source) on the archive with all six platform binaries:
  | Finding                                                                      | Resolution                                                                                                                                                                                                |
  | ---------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
  | osv-scanner HIGH `basic-ftp` (via `@grafana/sign-plugin` → `proxy-agent`)    | fixed: npm override `basic-ftp@^6.2.3`; `sign-plugin` still works                                                                                                                                         |
  | osv-scanner HIGH `braces` 3.0.3 (via `eslint-webpack-plugin` → `micromatch`) | **justified**: newest version, no fix published; dev dependency of the build-time linter, not contained in the archive. Blocks the validator's code-diff and automated review steps until fixed upstream. |
  | plugin.json without screenshots                                              | fixed: `img/screenshot-dashboard.png`                                                                                                                                                                     |
  | LICENSE with template placeholders                                           | fixed                                                                                                                                                                                                     |
  | unsigned plugin                                                              | expected before publishing (see `docs/publishing.md`)                                                                                                                                                     |

## Phase 7 – Cloud matrix

|                                | n8n OSS (local, 2.42.7 CE)                                                         | n8n remote (self-hosted, older, licensed; stand-in for Cloud)                                              | n8n Cloud                           |
| ------------------------------ | ---------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- | ----------------------------------- |
| **Grafana OSS 13.2.2 (local)** | ✅ health, all query types, dashboard, e2e 7/7                                     | ✅ health (legacy paging detected), all query types except audit (key scope), dashboard with the same JSON | ⏳ no `*.app.n8n.cloud` account yet |
| **Grafana Cloud**              | ⏳ needs signed plugin; n8n reachable publicly or via PDC (toggle added, untested) | ⏳ needs signed plugin; internal host → PDC                                                                | ⏳ needs signed plugin              |

- 2026-10-13 – Private Data Source Connect: the SDK HTTP client reads `jsonData.enableSecureSocksProxy`; added to `dsconfig.json`, settings struct and config editor (`SecureSocksProxySettings`, shown when Grafana has PDC enabled). Untested without Grafana Cloud.
- 2026-10-13 – n8n Cloud differences are expected to come from version and licence/plan only (same code base): Cloud runs current n8n (server-side time filter, scope enforcement), Insights ranges and execution retention by plan. To verify once an account exists: `make test-integration` with `N8N_CLOUD_URL` / `N8N_CLOUD_API_KEY`, rate limits (429) under dashboard load.

## Conclusion (2026-10-13)

**Go.** The public API is sufficient for a useful datasource: time-bounded execution queries (server-side filter on current n8n, paging fallback on older versions), status and duration, Insights for long ranges, identical auth on all instances. A working prototype exists: Go backend with read-only allowlist, five query types, alerting, variables, bundled dashboard, one-command dev stack, unit/integration/e2e tests.

Main risks and limits: execution retention (dashboard range), truncation on high-volume instances (maxPages), Insights licence limits and hourly lag, n8n Cloud not yet verified, Grafana Cloud only after signing/publishing (`docs/publishing.md`).

- 2026-10-13 – Plugin ID `nluecke-n8n-datasource` (grafana.com organisation `nluecke`, Grafana Cloud stack `nluecke.grafana.net`). Next step: private signature for `https://nluecke.grafana.net` (see `docs/publishing.md`); whether Grafana Cloud can install privately signed plugins is still to be clarified.

## Pre-release fixes (2026-10-13)

- 2026-10-13 – **PDC tested locally** (`make pdc`: SOCKS5 proxy + Grafana with `secure_socks_datasource_proxy`): data source with PDC enabled is healthy and returns data through the proxy; with the proxy stopped only this data source fails (`socks connect … socks-proxy:1080->n8n:5678`), the direct one keeps working.
- 2026-10-13 – Bug found by the PDC test: the health check answered from the response cache and stayed green after the connection broke. Health checks now bypass the cache (`n8n.WithoutCache`). Health message with PDC enabled now points to the PDC agent.
- 2026-10-13 – Bug: a locally signed `dist/` (private signature for `https://nluecke.grafana.net/`) made local Grafana reject the plugin ("Could not load plugin"): a root-URL mismatch is an _invalid_ signature, not an unsigned one. `make build` now removes `dist/MANIFEST.txt`.
- 2026-10-13 – "Incomplete data" (maxPages) reworked: one server-side filtered request per selected status/workflow (≤ 20 combinations) instead of loading everything and filtering in the plugin. Real data: "failed executions, 12 h" went from truncated to 531 complete rows. Notice now states loaded executions, covered share of the range, the concrete fix (raise Max pages, current value, maximum), and links to the data source settings. The page estimate is labelled "at an even execution rate", because n8n offers no execution count and bursts (e.g. the 5,000-execution load test) make extrapolation unreliable.
- 2026-10-13 – Config editor rebuilt with `@grafana/plugin-ui` (`DataSourceDescription`, `ConfigSection`, collapsible TLS / additional settings): fixed overlapping URL hints, read scopes listed explicitly, Max pages shows its effect ("20 pages = up to 5,000 executions"), PDC switch in the same style.

## Release preparation (2026-10-13)

- 2026-10-13 – The `grafana/plugin-actions/build-plugin` release action runs `@grafana/plugin-validator` itself and failed on `braces` (GHSA-vfj7-8cjw-p6xm, published 2026-09-18, no fixed version). The validator ignores `osv-scanner.toml` and only skips findings pulled in by `@grafana/*` packages or on its internal whitelist (currently only `playwright@1.55.0`). The current `@grafana/create-plugin` 7.12.3 template still ships Jest 29 and `eslint-webpack-plugin`, so every newly scaffolded plugin hits this.
- 2026-10-13 – Fix: Jest 29 → 30 (no `micromatch` dependency any more) and `eslint-webpack-plugin` removed from `.config/webpack/webpack.config.ts` (project change to the scaffold config; linting runs via `npm run lint` in CI). `braces`/`micromatch` are gone from the lockfile; validator 0.49.5: exit 0, no errors, code diff and automated review run.
- 2026-10-13 – Release action defaults to Go 1.25 / Node 20, but `go.mod` requires Go ≥ 1.26.5 → `go-version: '1.26'`, `node-version: '22'` in `release.yml`.
- 2026-10-13 – Lockfiles must be generated so that `npm ci` works with npm 10 (CI, Node 22) and npm 11 (local); after dependency changes run `npx npm@10 install --package-lock-only` and test `npm ci` with both.
- 2026-10-13 – Branch protection: ruleset "main: PR + green CI" (no force push/deletion, PR required, required check `CI result`, an aggregate job because e2e job names change with the Grafana matrix).

## Catalog submission feedback (2026-10-13)

- 2026-10-13 – Automated submission checks on v1.0.0: warnings only ("can be reviewed"): gosec G101 ×2, govulncheck source (11) and binary (13) findings, unsigned plugin.
- gosec G101 were false positives (`HeaderAPIKey = "X-N8N-API-KEY"` is a header name, `n8ntest.APIKey` a test dummy) → `#nosec G101` with justification.
- govulncheck binary: the release was built with a Go 1.26 patch below 1.26.9 → `go.mod` now requires Go 1.26.9 (the release job runs with `GOTOOLCHAIN=local`, so an older Go fails loudly). `golang.org/x/net` v0.59.0 → v0.60.0 (GO-2026-6611, GO-2026-6617).
- govulncheck source: the validator runs it with `GOTOOLCHAIN=local`, i.e. with the reviewer environment's own Go; its standard-library findings ("update to 1.27.2") depend on that environment, not on the plugin. Locally the source scan is clean with Go 1.26.9 and 1.27.2.
- Local toolchain note: Homebrew's golangci-lint pulled in Go 1.27.2; gosec cannot read Go 1.27 export data yet → run gosec with `GOTOOLCHAIN=go1.26.9`.
- CI: new non-required job "Security scan" (gosec + govulncheck), `make security` locally. Released as v1.0.1.
