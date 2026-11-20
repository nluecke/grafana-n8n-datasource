# Query types

[Overview](../../src/README.md) · [Configuration](configuration.md) · [Query types](query-types.md) · [Troubleshooting](troubleshooting.md) · [Limitations](limitations.md)

| Query type      | Result                                                                                     | Options                                                                                              |
| --------------- | ------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------- |
| Execution stats | Time series per panel interval or one value for the whole range                            | Metric (count, success rate, error rate, duration percentiles), group by (status, workflow), filters |
| Executions      | Table: execution, workflow, status, mode, start, stop, duration, retry of, link            | Workflows, statuses, project                                                                         |
| Workflows       | Table: name, active, archived, tags, project, trigger count, "skips successful executions" | Workflows, project                                                                                   |
| Insights        | Summary (with change vs. previous period) or time series per hour/day/week                 | Mode, bucket, project                                                                                |
| Audit           | n8n security audit report                                                                  | –                                                                                                    |

Success rate = success / (success + error + crashed). Running, waiting and canceled executions are not counted.

Execution stats time series use the panel interval, but at most 150 buckets per query: finer intervals are rounded
up to 1m, 2m, 5m, 10m, 15m, 30m, 1h, 2h, 3h, 6h, 12h, 1d or multiples of a day (e.g. 12 h → 5 min, 24 h → 10 min,
7 days → 2 h). Executions are individual events, so finer buckets would mostly hold zero or one execution.
Series grouped by status have fixed colours (success green, error red, crashed dark red, canceled grey, running blue,
waiting yellow, new purple); panel overrides still take precedence.
