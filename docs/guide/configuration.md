# Configuration

[Overview](../../src/README.md) · [Configuration](configuration.md) · [Query types](query-types.md) · [Troubleshooting](troubleshooting.md) · [Limitations](limitations.md)

## API key

In n8n open **Settings → n8n API → Create an API key**. Use a dedicated n8n user for Grafana and give
the key only read scopes:

`workflow:list`, `workflow:read`, `execution:list`, `execution:read`, `tag:list`, `project:list`, `insights:read`, `securityAudit:generate`

Recent n8n versions enforce these scopes on every edition. The plugin itself only sends read requests
(enforced by an allowlist in the backend), but a key with write scopes would allow more than the plugin needs.

## Data source settings

Add a data source of type **n8n** under _Connections → Data sources_.

| Field                       | Description                                                                                                                                                                                       |
| --------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| URL                         | Base URL of the public API **including `/api/v1`**. n8n Cloud: `https://<instance>.app.n8n.cloud/api/v1`, self-hosted: `http://<host>:5678/api/v1` (add `N8N_PATH` if n8n runs under a sub path). |
| API key                     | The key created above. Stored encrypted, only used by the Grafana backend.                                                                                                                        |
| Skip TLS verify             | Do not verify the server certificate. For testing only.                                                                                                                                           |
| With CA cert / CA cert      | Verify against a custom CA, e.g. an internal PKI.                                                                                                                                                 |
| Max pages                   | Upper limit of API pages (250 executions each) per query. Default 20, range 1–100.                                                                                                                |
| Cache TTL                   | Seconds API responses are cached in the backend. Default 60, 0 disables caching.                                                                                                                  |
| Private data source connect | Grafana Cloud: reach a self-hosted n8n on a private network through a PDC agent. Shown under _Additional settings_ when PDC is enabled for the stack.                                             |

The URL must not contain a user name or password (`https://user:password@host/...`): it is stored unencrypted
and used for the links in query results. Authenticate with the API key.

## Provisioning

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
