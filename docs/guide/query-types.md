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
