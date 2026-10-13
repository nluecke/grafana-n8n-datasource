import React from 'react';
import { QueryEditorProps } from '@grafana/data';
import { InlineField, RadioButtonGroup } from '@grafana/ui';

import type { DataSource } from '../datasource';
import { N8nDataSourceOptions, N8nQuery, N8nVariableQuery, VariableResource } from '../types';

type Props = QueryEditorProps<DataSource, N8nQuery, N8nDataSourceOptions, N8nVariableQuery>;

const RESOURCES: Array<{ label: string; value: VariableResource; description: string }> = [
  { label: 'Workflows', value: 'workflows', description: 'Value: workflow id, text: workflow name' },
  { label: 'Projects', value: 'projects', description: 'Requires an n8n licence with projects' },
  { label: 'Tags', value: 'tags', description: 'Value: tag id, text: tag name' },
];

export function VariableQueryEditor({ query, onChange }: Props) {
  const resource = query.resource ?? 'workflows';
  return (
    <InlineField label="Values" labelWidth={14} tooltip="Use the variable in the Workflow or Project filter of a query">
      <RadioButtonGroup<VariableResource>
        options={RESOURCES}
        value={resource}
        onChange={(value) => onChange({ ...query, refId: query.refId ?? 'variable', resource: value })}
      />
    </InlineField>
  );
}
