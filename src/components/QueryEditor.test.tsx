import React from 'react';
import { act, render, screen, waitFor } from '@testing-library/react';

import { QueryEditor } from './QueryEditor';
import type { DataSource } from '../datasource';
import { N8nQuery } from '../types';

jest.mock('@grafana/runtime', () => ({
  ...jest.requireActual('@grafana/runtime'),
  getTemplateSrv: () => ({ getVariables: () => [], replace: (s: string) => s }),
}));

async function setup(query: Partial<N8nQuery>, projects: Array<{ value: string; label: string }> = []) {
  const datasource = {
    getOptions: jest.fn((resource: string) =>
      Promise.resolve(resource === 'workflows' ? [{ value: 'wf1', label: 'Alpha' }] : projects)
    ),
  } as unknown as DataSource;
  const onChange = jest.fn();
  const onRunQuery = jest.fn();
  render(
    <QueryEditor query={{ refId: 'A', ...query }} datasource={datasource} onChange={onChange} onRunQuery={onRunQuery} />
  );
  // Let the workflow and project option loaders settle.
  await act(async () => {});
  return { datasource, onChange, onRunQuery };
}

const combobox = (name: string) => screen.queryByRole('combobox', { name });

describe('QueryEditor', () => {
  it('shows metric and group by for execution stats', async () => {
    const { datasource } = await setup({ queryType: 'executionStats' });
    expect(combobox('Workflows')).toBeInTheDocument();
    expect(combobox('Status')).toBeInTheDocument();
    expect(combobox('Metric')).toBeInTheDocument();
    expect(combobox('Group by')).toBeInTheDocument();
    expect(combobox('Percentiles')).not.toBeInTheDocument();
    await waitFor(() => expect(datasource.getOptions).toHaveBeenCalledWith('workflows'));
  });

  it('shows percentiles for the duration metric', async () => {
    await setup({ queryType: 'executionStats', metric: 'duration' });
    expect(combobox('Percentiles')).toBeInTheDocument();
  });

  it('hides execution filters for insights and shows the bucket in time series mode', async () => {
    await setup({ queryType: 'insights', insightsMode: 'timeseries' });
    expect(combobox('Status')).not.toBeInTheDocument();
    expect(combobox('Bucket')).toBeInTheDocument();
  });

  it('shows the project filter only when projects exist', async () => {
    await setup({ queryType: 'executions' }, [{ value: 'p1', label: 'Default' }]);
    expect(await screen.findByRole('combobox', { name: 'Project' })).toBeInTheDocument();
  });

  it('does not show the project filter without projects', async () => {
    const { datasource } = await setup({ queryType: 'executions' });
    await waitFor(() => expect(datasource.getOptions).toHaveBeenCalledWith('projects'));
    expect(combobox('Project')).not.toBeInTheDocument();
  });
});
