import { CoreApp, DataSourceInstanceSettings, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import { DEFAULT_QUERY, N8nDataSourceOptions, N8nQuery, ResourceOption, VariableResource } from './types';
import { N8nVariableSupport } from './variables';

// Separator for multi-value template variables; cannot occur in n8n ids or statuses.
const MULTI_SEPARATOR = '\u001f';

export class DataSource extends DataSourceWithBackend<N8nQuery, N8nDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<N8nDataSourceOptions>) {
    super(instanceSettings);
    this.variables = new N8nVariableSupport(this);
  }

  getDefaultQuery(_: CoreApp): Partial<N8nQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: N8nQuery, scopedVars: ScopedVars): N8nQuery {
    return {
      ...query,
      statuses: interpolateList(query.statuses, scopedVars),
      workflowIds: interpolateList(query.workflowIds, scopedVars),
      projectId: query.projectId ? getTemplateSrv().replace(query.projectId, scopedVars) : query.projectId,
    };
  }

  filterQuery(query: N8nQuery): boolean {
    return !query.hide;
  }

  /** Options for the query and variable editors, served by the backend. */
  getOptions(resource: VariableResource): Promise<ResourceOption[]> {
    return this.getResource<ResourceOption[]>(resource);
  }
}

/**
 * Expands template variables in a list. A multi-value variable such as $workflow
 * becomes one entry per selected value.
 */
export function interpolateList(values: string[] | undefined, scopedVars: ScopedVars): string[] | undefined {
  if (!values) {
    return values;
  }
  const templateSrv = getTemplateSrv();
  return values.flatMap((v) =>
    templateSrv
      .replace(v, scopedVars, (value: string | string[]) =>
        Array.isArray(value) ? value.join(MULTI_SEPARATOR) : value
      )
      .split(MULTI_SEPARATOR)
      .filter((s) => s !== '')
  );
}
