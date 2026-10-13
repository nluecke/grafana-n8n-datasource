import { DataQueryRequest, DataSourceInstanceSettings, ScopedVars } from '@grafana/data';
import { lastValueFrom } from 'rxjs';

import { DataSource, interpolateList } from './datasource';
import { N8nDataSourceOptions, N8nVariableQuery } from './types';

const variables: Record<string, string | string[]> = {
  workflow: ['wf1', 'wf2'],
  status: 'error',
  project: 'p1',
};

jest.mock('@grafana/runtime', () => ({
  ...jest.requireActual('@grafana/runtime'),
  getTemplateSrv: () => ({
    getVariables: () => [],
    replace: (target: string, _scoped?: ScopedVars, format?: (value: string | string[]) => string) =>
      target.replace(/\$(\w+)/g, (match, name) => {
        const value = variables[name];
        if (value === undefined) {
          return match;
        }
        return format ? format(value) : Array.isArray(value) ? value.join(',') : value;
      }),
  }),
}));

const settings = {
  id: 1,
  uid: 'n8n',
  type: 'nluecke-n8n-datasource',
  jsonData: {},
} as unknown as DataSourceInstanceSettings<N8nDataSourceOptions>;

describe('interpolateList', () => {
  it('expands multi-value variables into separate entries', () => {
    expect(interpolateList(['$workflow', 'wf3'], {})).toEqual(['wf1', 'wf2', 'wf3']);
  });

  it('keeps plain values and single-value variables', () => {
    expect(interpolateList(['success', '$status'], {})).toEqual(['success', 'error']);
  });

  it('passes undefined through', () => {
    expect(interpolateList(undefined, {})).toBeUndefined();
  });
});

describe('DataSource', () => {
  const ds = new DataSource(settings);

  it('interpolates workflow, status and project filters', () => {
    const q = ds.applyTemplateVariables(
      { refId: 'A', queryType: 'executions', workflowIds: ['$workflow'], statuses: ['$status'], projectId: '$project' },
      {}
    );
    expect(q.workflowIds).toEqual(['wf1', 'wf2']);
    expect(q.statuses).toEqual(['error']);
    expect(q.projectId).toBe('p1');
  });

  it('skips hidden queries', () => {
    expect(ds.filterQuery({ refId: 'A', hide: true })).toBe(false);
    expect(ds.filterQuery({ refId: 'A' })).toBe(true);
  });

  it('returns workflows as text/value variable values', async () => {
    jest.spyOn(ds, 'getOptions').mockResolvedValue([{ value: 'wf1', label: 'Alpha' }]);
    const request = {
      targets: [{ refId: 'v', resource: 'workflows' }],
    } as unknown as DataQueryRequest<N8nVariableQuery>;
    const res = await lastValueFrom((ds.variables as any).query(request));
    const frame = (res as any).data[0];
    expect(frame.fields.map((f: any) => [f.name, f.values])).toEqual([
      ['text', ['Alpha']],
      ['value', ['wf1']],
    ]);
    expect(ds.getOptions).toHaveBeenCalledWith('workflows');
  });
});
