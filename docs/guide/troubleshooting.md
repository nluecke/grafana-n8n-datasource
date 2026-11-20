# Troubleshooting

[Overview](../../src/README.md) · [Configuration](configuration.md) · [Query types](query-types.md) · [Troubleshooting](troubleshooting.md) · [Limitations](limitations.md)

## Save & test messages

**Save & test** checks the connection and reports whether the n8n version supports the server-side time
filter. Typical messages:

- _401_: wrong or expired key. n8n answers the same way when the public API is disabled.
- _403_: free trial of n8n Cloud, or the key lacks the `workflow:list` scope.
- _404 / web page instead of API_: the URL does not end with `/api/v1`.
- _TLS error_: enable **With CA cert** for internal certificate authorities.
- _Cannot reach n8n_: the host is not reachable from the Grafana server (or the PDC agent).

The messages do not include the underlying error, so they do not reveal host names, addresses or ports. The
Grafana server log has the details (`Health check failed`).

## Panel messages

- _Incomplete data_ (warning icon): the query hit _Max pages_. Click the icon to open the data source settings and
  raise _Max pages_ (up to 100), or narrow the time range or filters. See [Limitations](limitations.md).
- _n8n holds no executions before …_ (info): older executions were pruned or never ran. Use the Insights query
  for longer ranges.
- _n8n Insights: …_ error: the n8n licence or plan does not allow this range or bucket. Community Edition
  allows ranges of at least 24 hours within the last 7 days.
- _Project filter missing_: the n8n licence does not include projects.
