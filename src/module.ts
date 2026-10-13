import { DataSourcePlugin } from '@grafana/data';
import { DataSource } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { QueryEditor } from './components/QueryEditor';
import { N8nQuery, N8nDataSourceOptions, N8nSecureJsonData } from './types';

export const plugin = new DataSourcePlugin<DataSource, N8nQuery, N8nDataSourceOptions, N8nSecureJsonData>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);
