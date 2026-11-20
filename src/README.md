# n8n data source for Grafana

Workflow and execution observability for **n8n Cloud** and **self-hosted n8n**, built on the
[n8n public API](https://docs.n8n.io/api/). A URL and an API key are all it needs: no Prometheus scraping,
no database access.

- **Executions** as tables with links into the n8n editor
- **Execution statistics** as time series: counts by status, success and error rate, duration percentiles,
  per workflow or in total
- **Long-term trends** from n8n Insights, independent of execution pruning
- **Workflow inventory** and the n8n security audit
- **Template variables** for workflows, projects and tags
- **Alerting**, for example on the error rate of a workflow
- **Bundled dashboard** _n8n Workflow Executions (API)_

## When to use it

| Setup                                             | Recommendation                                                                                                                                                                                                              |
| ------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| n8n Cloud                                         | This plugin. n8n Cloud has no `/metrics` endpoint and no database access, so the public API is the only source.                                                                                                             |
| Self-hosted n8n, no extra infrastructure wanted   | This plugin.                                                                                                                                                                                                                |
| Self-hosted n8n with Prometheus / Postgres access | Combine it with the dashboards [n8n System Health Overview](https://grafana.com/grafana/dashboards/24474) (Prometheus) and [n8n Workflow & Execution Analytics](https://grafana.com/grafana/dashboards/24475) (PostgreSQL). |

## Requirements

- Grafana 13.2 or later
- n8n with the public API: **n8n Cloud** Starter, Pro or Enterprise (not available in the free trial);
  **self-hosted** all editions (on by default)
- Network access from Grafana to n8n. Grafana Cloud reaches n8n on a private network through
  [Private Data Source Connect](https://grafana.com/docs/grafana-cloud/connect-externally-hosted/private-data-source-connect/).

## Get started

1. In n8n, open **Settings → n8n API → Create an API key**. Use a dedicated user and only these read scopes:
   `workflow:list`, `workflow:read`, `execution:list`, `execution:read`, `tag:list`, `project:list`,
   `insights:read`, `securityAudit:generate`
2. In Grafana, add an **n8n** data source. Enter the API URL **including `/api/v1`**, e.g.
   `https://<instance>.app.n8n.cloud/api/v1` or `http://<host>:5678/api/v1`, and the API key. Click **Save & test**.
3. On the data source's **Dashboards** tab, import **n8n Workflow Executions (API)**.

## Good to know

- **History is limited by execution pruning.** Statistics are computed from the stored executions
  (self-hosted default: 14 days or 10,000 executions; n8n Cloud: depends on the plan). Use the Insights query for longer ranges.
- **Workflows that do not save successful executions** make counts and success rates too low. The plugin
  warns about affected workflows.
- **Busy instances:** each query loads up to _Max pages_ × 250 executions. Panels show **Incomplete data** when
  the limit is reached; raise _Max pages_ in the data source settings.
- The plugin only sends read requests to n8n, enforced by an allowlist in the backend.

## Documentation

- [Configuration and provisioning](https://github.com/nluecke/grafana-n8n-datasource/blob/main/docs/guide/configuration.md)
- [Query types](https://github.com/nluecke/grafana-n8n-datasource/blob/main/docs/guide/query-types.md)
- [Troubleshooting](https://github.com/nluecke/grafana-n8n-datasource/blob/main/docs/guide/troubleshooting.md)
- [Limitations in detail](https://github.com/nluecke/grafana-n8n-datasource/blob/main/docs/guide/limitations.md)

---

Community plugin, not affiliated with, endorsed by or supported by n8n GmbH. "n8n" is a trademark of n8n GmbH.
Report issues on [GitHub](https://github.com/nluecke/grafana-n8n-datasource/issues).
