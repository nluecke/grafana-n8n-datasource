import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type QueryType = 'executions' | 'executionStats' | 'workflows' | 'insights' | 'audit';
export type StatsMetric = 'count' | 'successRate' | 'errorRate' | 'duration';
export type StatsGroupBy = 'none' | 'status' | 'workflow';
export type StatsBucket = 'auto' | 'range';
export type InsightsMode = 'summary' | 'timeseries';
export type InsightsBucket = 'auto' | '1h' | '1d' | '1w';

/**
 * Query model. Must stay in sync with queryModel in pkg/plugin/query.go.
 */
export interface N8nQuery extends DataQuery {
  queryType?: QueryType;
  statuses?: string[];
  workflowIds?: string[];
  projectId?: string;

  metric?: StatsMetric;
  groupBy?: StatsGroupBy;
  percentiles?: number[];
  bucket?: StatsBucket;

  insightsMode?: InsightsMode;
  insightsBucket?: InsightsBucket;
}

export const DEFAULT_QUERY: Partial<N8nQuery> = {
  queryType: 'executionStats',
  metric: 'count',
  groupBy: 'status',
};

export const EXECUTION_STATUSES = ['success', 'error', 'crashed', 'canceled', 'running', 'waiting', 'new', 'unknown'];
export const DEFAULT_PERCENTILES = [50, 90, 99];

export type VariableResource = 'workflows' | 'projects' | 'tags';

export interface N8nVariableQuery extends DataQuery {
  resource: VariableResource;
}

/** Value/label pair returned by the backend resource endpoints. */
export interface ResourceOption {
  value: string;
  label: string;
}

/**
 * Options stored in jsonData. Must stay in sync with pkg/schema/dsconfig.json.
 */
export interface N8nDataSourceOptions extends DataSourceJsonData {
  maxPages?: number;
  cacheTtlSeconds?: number;
  tlsSkipVerify?: boolean;
  tlsAuthWithCACert?: boolean;
  enableSecureSocksProxy?: boolean;
}

/**
 * Values stored in secureJsonData, only readable by the backend.
 */
export interface N8nSecureJsonData {
  apiKey?: string;
  tlsCACert?: string;
}

export const DEFAULT_MAX_PAGES = 20;
export const DEFAULT_CACHE_TTL_SECONDS = 60;
