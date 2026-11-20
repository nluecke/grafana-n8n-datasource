# Limitations

[Overview](../../src/README.md) · [Configuration](configuration.md) · [Query types](query-types.md) · [Troubleshooting](troubleshooting.md) · [Limitations](limitations.md)

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
