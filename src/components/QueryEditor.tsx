import React, { useEffect, useMemo, useState } from 'react';
import { QueryEditorProps } from '@grafana/data';
import { getTemplateSrv } from '@grafana/runtime';
import { Alert, Combobox, ComboboxOption, InlineField, MultiCombobox, RadioButtonGroup, Stack } from '@grafana/ui';

import type { DataSource } from '../datasource';
import {
  DEFAULT_PERCENTILES,
  EXECUTION_STATUSES,
  InsightsBucket,
  InsightsMode,
  N8nDataSourceOptions,
  N8nQuery,
  QueryType,
  ResourceOption,
  StatsBucket,
  StatsGroupBy,
  StatsMetric,
} from '../types';

type Props = QueryEditorProps<DataSource, N8nQuery, N8nDataSourceOptions>;

const LABEL_WIDTH = 14;

const QUERY_TYPES: Array<{ label: string; value: QueryType; description: string }> = [
  { label: 'Execution stats', value: 'executionStats', description: 'Time series computed from the execution list' },
  { label: 'Executions', value: 'executions', description: 'Table of individual executions' },
  { label: 'Workflows', value: 'workflows', description: 'Table of workflows' },
  { label: 'Insights', value: 'insights', description: 'Aggregates from n8n Insights, beyond execution retention' },
  { label: 'Audit', value: 'audit', description: 'n8n security audit report' },
];

const METRICS: Array<ComboboxOption<StatsMetric>> = [
  { label: 'Count', value: 'count', description: 'Executions per interval' },
  { label: 'Success rate', value: 'successRate', description: 'success / (success + error + crashed)' },
  { label: 'Error rate', value: 'errorRate', description: '(error + crashed) / (success + error + crashed)' },
  { label: 'Duration', value: 'duration', description: 'Percentiles of stoppedAt - startedAt' },
];

const BUCKETS: Array<ComboboxOption<StatsBucket>> = [
  { label: 'Panel interval', value: 'auto', description: 'One value per query interval (time series)' },
  { label: 'Whole range', value: 'range', description: 'One value for the whole time range (stat panels, alerts)' },
];

const PERCENTILES: Array<ComboboxOption<number>> = [50, 75, 90, 95, 99].map((p) => ({ label: `p${p}`, value: p }));

const INSIGHTS_MODES: Array<{ label: string; value: InsightsMode }> = [
  { label: 'Summary', value: 'summary' },
  { label: 'Time series', value: 'timeseries' },
];

const INSIGHTS_BUCKETS: Array<ComboboxOption<InsightsBucket>> = [
  { label: 'Auto', value: 'auto', description: 'Hourly up to 3 days, then daily, then weekly' },
  { label: 'Hour', value: '1h', description: 'Requires a Business or Enterprise licence' },
  { label: 'Day', value: '1d' },
  { label: 'Week', value: '1w' },
];

function useResourceOptions(datasource: DataSource, resource: 'workflows' | 'projects'): ResourceOption[] {
  const [options, setOptions] = useState<ResourceOption[]>([]);
  useEffect(() => {
    let cancelled = false;
    datasource
      .getOptions(resource)
      .then((opts) => !cancelled && setOptions(opts))
      .catch(() => !cancelled && setOptions([]));
    return () => {
      cancelled = true;
    };
  }, [datasource, resource]);
  return options;
}

function variableOptions(): Array<ComboboxOption<string>> {
  return getTemplateSrv()
    .getVariables()
    .map((v) => ({ label: `$${v.name}`, value: `$${v.name}`, group: 'Variables' }));
}

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const workflows = useResourceOptions(datasource, 'workflows');
  const projects = useResourceOptions(datasource, 'projects');
  const variables = useMemo(() => variableOptions(), []);

  const queryType = query.queryType ?? 'executionStats';
  const metric = query.metric ?? 'count';

  const update = (patch: Partial<N8nQuery>) => {
    onChange({ ...query, ...patch });
    onRunQuery();
  };

  const workflowOptions: Array<ComboboxOption<string>> = [
    ...variables,
    ...workflows.map((w) => ({ label: w.label, value: w.value, description: w.value })),
  ];
  const projectOptions: Array<ComboboxOption<string>> = [
    ...variables,
    ...projects.map((p) => ({ label: p.label, value: p.value })),
  ];
  const statusOptions: Array<ComboboxOption<string>> = [
    ...variables,
    ...EXECUTION_STATUSES.map((s) => ({ label: s, value: s })),
  ];
  const groupByOptions: Array<ComboboxOption<StatsGroupBy>> = [
    { label: 'None', value: 'none' },
    ...(metric === 'count' ? [{ label: 'Status', value: 'status' as const }] : []),
    { label: 'Workflow', value: 'workflow' },
  ];

  const usesExecutions = queryType === 'executions' || queryType === 'executionStats';
  const showProject = projects.length > 0 && queryType !== 'audit';

  return (
    <Stack direction="column" gap={1}>
      <InlineField label="Query type" labelWidth={LABEL_WIDTH}>
        <RadioButtonGroup<QueryType>
          options={QUERY_TYPES}
          value={queryType}
          onChange={(value) => update({ queryType: value })}
        />
      </InlineField>

      {(usesExecutions || queryType === 'workflows') && (
        <InlineField label="Workflows" labelWidth={LABEL_WIDTH} tooltip="Empty means all workflows">
          <MultiCombobox<string>
            aria-label="Workflows"
            width={60}
            options={workflowOptions}
            value={query.workflowIds ?? []}
            placeholder="All workflows"
            isClearable
            createCustomValue
            onChange={(opts) => update({ workflowIds: opts.map((o) => o.value) })}
          />
        </InlineField>
      )}

      {usesExecutions && (
        <InlineField label="Status" labelWidth={LABEL_WIDTH} tooltip="Empty means all statuses">
          <MultiCombobox<string>
            aria-label="Status"
            width={60}
            options={statusOptions}
            value={query.statuses ?? []}
            placeholder="All statuses"
            isClearable
            onChange={(opts) => update({ statuses: opts.map((o) => o.value) })}
          />
        </InlineField>
      )}

      {showProject && (
        <InlineField label="Project" labelWidth={LABEL_WIDTH}>
          <Combobox<string>
            aria-label="Project"
            width={60}
            options={projectOptions}
            value={query.projectId ?? null}
            placeholder="All projects"
            isClearable
            onChange={(opt) => update({ projectId: opt?.value })}
          />
        </InlineField>
      )}

      {queryType === 'executionStats' && (
        <Stack direction="row" gap={0}>
          <InlineField label="Metric" labelWidth={LABEL_WIDTH}>
            <Combobox<StatsMetric>
              aria-label="Metric"
              width={25}
              options={METRICS}
              value={metric}
              onChange={(opt) => {
                const groupBy = opt.value !== 'count' && query.groupBy === 'status' ? 'none' : query.groupBy;
                update({ metric: opt.value, groupBy });
              }}
            />
          </InlineField>
          <InlineField label="Group by" labelWidth={10}>
            <Combobox<StatsGroupBy>
              aria-label="Group by"
              width={20}
              options={groupByOptions}
              value={query.groupBy ?? (metric === 'count' ? 'status' : 'none')}
              onChange={(opt) => update({ groupBy: opt.value })}
            />
          </InlineField>
          <InlineField label="Bucket" labelWidth={10}>
            <Combobox<StatsBucket>
              aria-label="Stats bucket"
              width={20}
              options={BUCKETS}
              value={query.bucket ?? 'auto'}
              onChange={(opt) => update({ bucket: opt.value })}
            />
          </InlineField>
          {metric === 'duration' && (
            <InlineField label="Percentiles" labelWidth={12}>
              <MultiCombobox<number>
                aria-label="Percentiles"
                width={30}
                options={PERCENTILES}
                value={query.percentiles ?? DEFAULT_PERCENTILES}
                createCustomValue
                onChange={(opts) =>
                  update({ percentiles: opts.map((o) => Number(o.value)).filter((p) => p > 0 && p <= 100) })
                }
              />
            </InlineField>
          )}
        </Stack>
      )}

      {queryType === 'insights' && (
        <Stack direction="row" gap={0}>
          <InlineField label="Mode" labelWidth={LABEL_WIDTH}>
            <RadioButtonGroup<InsightsMode>
              options={INSIGHTS_MODES}
              value={query.insightsMode ?? 'summary'}
              onChange={(value) => update({ insightsMode: value })}
            />
          </InlineField>
          {query.insightsMode === 'timeseries' && (
            <InlineField label="Bucket" labelWidth={10}>
              <Combobox<InsightsBucket>
                aria-label="Bucket"
                width={20}
                options={INSIGHTS_BUCKETS}
                value={query.insightsBucket ?? 'auto'}
                onChange={(opt) => update({ insightsBucket: opt.value })}
              />
            </InlineField>
          )}
        </Stack>
      )}

      {usesExecutions && (
        <Alert severity="info" title="" bottomSpacing={0}>
          Computed from the execution list, so the time range is limited by n8n&apos;s execution retention. Use Insights
          for longer ranges.
        </Alert>
      )}
      {queryType === 'insights' && (
        <Alert severity="info" title="" bottomSpacing={0}>
          Insights cover all workflows of the instance (or project) and are updated hourly. Community Edition allows
          ranges of at least 24 hours within the last 7 days.
        </Alert>
      )}
    </Stack>
  );
}
