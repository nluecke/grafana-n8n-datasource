import { CustomVariableSupport, DataQueryRequest, DataQueryResponse, FieldType, createDataFrame } from '@grafana/data';
import { Observable, from, map } from 'rxjs';

import type { DataSource } from './datasource';
import { N8nVariableQuery } from './types';
import { VariableQueryEditor } from './components/VariableQueryEditor';

/** Template variables for workflows, projects and tags (value = id, text = name). */
export class N8nVariableSupport extends CustomVariableSupport<DataSource, N8nVariableQuery> {
  constructor(private readonly datasource: DataSource) {
    super();
  }

  editor = VariableQueryEditor;

  query(request: DataQueryRequest<N8nVariableQuery>): Observable<DataQueryResponse> {
    const resource = request.targets[0]?.resource ?? 'workflows';
    return from(this.datasource.getOptions(resource)).pipe(
      map((options) => ({
        data: [
          createDataFrame({
            fields: [
              { name: 'text', type: FieldType.string, values: options.map((o) => o.label) },
              { name: 'value', type: FieldType.string, values: options.map((o) => o.value) },
            ],
          }),
        ],
      }))
    );
  }
}
