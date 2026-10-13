# n8n data source for Grafana

Workflow and execution observability for **n8n Cloud** and **self-hosted n8n**, based on the
[n8n public REST API](https://docs.n8n.io/api/). Configure a URL and an API key; no Prometheus
scraping and no database access are needed.

- Execution tables with links into the n8n editor
- Execution statistics as time series: counts by status, success and error rate, duration percentiles,
  per workflow or in total
- Long-term aggregates from n8n Insights, independent of execution pruning
- Workflow inventory and the n8n security audit
- Template variables for workflows, projects and tags
- Alerting, for example on the error rate of a workflow
- Bundled dashboard **n8n Workflow Executions (API)**

## When to use this plugin

| Setup                                             | Recommended                                                                                                                                                                                                                        |
| ------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| n8n Cloud                                         | This plugin. n8n Cloud offers neither the `/metrics` endpoint nor database access, so the public API is the only source.                                                                                                           |
| Self-hosted n8n, no extra infrastructure wanted   | This plugin: URL and API key are enough.                                                                                                                                                                                           |
| Self-hosted n8n with Prometheus / Postgres access | Combine with the community dashboards [n8n System Health Overview](https://grafana.com/grafana/dashboards/24474) (Prometheus) and [n8n Workflow & Execution Analytics](https://grafana.com/grafana/dashboards/24475) (PostgreSQL). |

## Requirements

- Grafana 13.2 or later
- n8n with the public API enabled
  - **n8n Cloud:** Starter, Pro or Enterprise plan. The API is **not available in the free trial**.
  - **Self-hosted:** all editions. The API is on by default and can be disabled with `N8N_PUBLIC_API_DISABLED=true`.
- Grafana must reach n8n over the network. Grafana Cloud cannot reach n8n on a private network directly;
  use a public URL or [Private Data Source Connect](https://grafana.com/docs/grafana-cloud/connect-externally-hosted/private-data-source-connect/).

## Create an API key

In n8n open **Settings → n8n API → Create an API key**. Use a dedicated n8n user for Grafana and give
the key only read scopes:

`workflow:list`, `workflow:read`, `execution:list`, `execution:read`, `tag:list`, `project:list`, `insights:read`, `securityAudit:generate`

Recent n8n versions enforce these scopes on every edition. The plugin itself only sends read requests
(enforced by an allowlist in the backend), but a key with write scopes would allow more than the plugin needs.

## Configuration

| Field                       | Description                                                                                                                                                                                       |
| --------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| URL                         | Base URL of the public API **including `/api/v1`**. n8n Cloud: `https://<instance>.app.n8n.cloud/api/v1`, self-hosted: `http://<host>:5678/api/v1` (add `N8N_PATH` if n8n runs under a sub path). |
| API key                     | The key created above. Stored encrypted, only used by the Grafana backend.                                                                                                                        |
| Skip TLS verify             | Do not verify the server certificate. For testing only.                                                                                                                                           |
| With CA cert / CA cert      | Verify against a custom CA, e.g. an internal PKI.                                                                                                                                                 |
| Max pages                   | Upper limit of API pages (250 executions each) per query. Default 20, range 1–100.                                                                                                                |
| Cache TTL                   | Seconds API responses are cached in the backend. Default 60, 0 disables caching.                                                                                                                  |
| Private data source connect | Grafana Cloud: reach a self-hosted n8n on a private network through a PDC agent. Shown under _Additional settings_ when PDC is enabled for the stack.                                             |

**Save & test** checks the connection and reports whether the n8n version supports the server-side time
filter. Typical messages:

- _401_: wrong or expired key. n8n answers the same way when the public API is disabled.
- _403_: free trial of n8n Cloud, or the key lacks the `workflow:list` scope.
- _404 / web page instead of API_: the URL does not end with `/api/v1`.
- _TLS error_: enable **With CA cert** for internal certificate authorities.

Provisioning example:

```yaml
apiVersion: 1
datasources:
  - name: n8n
    type: nluecke-n8n-datasource
    access: proxy
    url: https://example.app.n8n.cloud/api/v1
    jsonData:
      maxPages: 20
      cacheTtlSeconds: 60
    secureJsonData:
      apiKey: ${N8N_API_KEY}
```

## Query types

| Query type      | Result                                                                                     | Options                                                                                              |
| --------------- | ------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------- |
| Execution stats | Time series per panel interval or one value for the whole range                            | Metric (count, success rate, error rate, duration percentiles), group by (status, workflow), filters |
| Executions      | Table: execution, workflow, status, mode, start, stop, duration, retry of, link            | Workflows, statuses, project                                                                         |
| Workflows       | Table: name, active, archived, tags, project, trigger count, "skips successful executions" | Workflows, project                                                                                   |
| Insights        | Summary (with change vs. previous period) or time series per hour/day/week                 | Mode, bucket, project                                                                                |
| Audit           | n8n security audit report                                                                  | –                                                                                                    |

Success rate = success / (success + error + crashed). Running, waiting and canceled executions are not counted.

## Limitations

- **Execution retention limits the time range.** Statistics and tables are computed from the execution list,
  which n8n prunes: self-hosted by default after 14 days or 10,000 executions (`EXECUTIONS_DATA_MAX_AGE`,
  `EXECUTIONS_DATA_PRUNE_MAX_COUNT`), on n8n Cloud depending on the plan (e.g. Starter 7 days / 2,500,
  Pro 30 days / 25,000). Panels show a notice when the time range starts before the oldest stored execution.
  Use the Insights query for longer ranges.
- **Workflows that do not save successful executions** (workflow setting, or `EXECUTIONS_DATA_SAVE_ON_SUCCESS=none`
  for the whole instance) make counts and success rates too low. The plugin warns for affected workflows;
  the instance-wide setting cannot be detected through the API.
- **Data volume:** every query loads the executions of its time range, 250 per request, up to _Max pages_
  (default 20 = 5,000 executions). Selected statuses and workflows are requested separately and filtered by n8n,
  so filtered panels (e.g. failed executions) stay complete on busy instances. If the limit is reached, the panel
  shows a warning icon with **"Incomplete data"**: how many executions were loaded, which part of the time range
  they cover, and how to fix it. Clicking the icon opens the data source settings, where you raise _Max pages_
  (up to 100). Alternatively narrow the time range or filters, or use the Insights query for totals.
- **Older n8n versions** have no server-side time filter; the plugin then pages from the newest execution
  backwards until the start of the time range, which needs more requests for ranges in the past.
- **Insights** cover all workflows of the instance (or a project), are updated hourly, and their allowed
  ranges depend on the licence: Community Edition allows ranges of at least 24 hours within the last 7 days;
  hourly data and longer history need Business or Enterprise (n8n Cloud: by plan).
- **Caching:** results can be up to _Cache TTL_ seconds old.
- Projects are only available with an n8n licence that includes them; otherwise the project filter is hidden.
