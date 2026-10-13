import React, { ChangeEvent } from 'react';
import { css } from '@emotion/css';
import { DataSourcePluginOptionsEditorProps, GrafanaTheme2 } from '@grafana/data';
import { config } from '@grafana/runtime';
import { ConfigSection, ConfigSubSection, DataSourceDescription } from '@grafana/plugin-ui';
import {
  Divider,
  Field,
  Input,
  SecretInput,
  SecretTextArea,
  Stack,
  Switch,
  Text,
  TextLink,
  useStyles2,
} from '@grafana/ui';

import { DEFAULT_CACHE_TTL_SECONDS, DEFAULT_MAX_PAGES, N8nDataSourceOptions, N8nSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<N8nDataSourceOptions, N8nSecureJsonData> {}

const URL_PATTERN = /^https?:\/\//;
const PAGE_SIZE = 250;
const MAX_PAGES = 100;
const DOCS_URL = 'https://github.com/nluecke/grafana-n8n-datasource/blob/main/src/README.md';
const READ_SCOPES = [
  'workflow:list',
  'workflow:read',
  'execution:list',
  'execution:read',
  'tag:list',
  'project:list',
  'insights:read',
  'securityAudit:generate',
];

const getStyles = (theme: GrafanaTheme2) => ({
  code: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
});

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;
  const styles = useStyles2(getStyles);

  const urlInvalid = !!options.url && !URL_PATTERN.test(options.url);
  const urlMissingApiPath = !!options.url && !urlInvalid && !/\/api\/v1\/?$/.test(options.url);
  const maxPages = jsonData.maxPages ?? DEFAULT_MAX_PAGES;
  const maxPagesInvalid = maxPages < 1 || maxPages > MAX_PAGES;
  const tlsConfigured = !!jsonData.tlsSkipVerify || !!jsonData.tlsAuthWithCACert;
  const additionalConfigured =
    jsonData.maxPages !== undefined || jsonData.cacheTtlSeconds !== undefined || !!jsonData.enableSecureSocksProxy;

  const updateJsonData = <K extends keyof N8nDataSourceOptions>(key: K, value: N8nDataSourceOptions[K]) => {
    onOptionsChange({ ...options, jsonData: { ...jsonData, [key]: value } });
  };

  const updateSecret = (key: keyof N8nSecureJsonData, value: string) => {
    onOptionsChange({ ...options, secureJsonData: { ...secureJsonData, [key]: value } });
  };

  const resetSecret = (key: keyof N8nSecureJsonData) => {
    onOptionsChange({
      ...options,
      secureJsonFields: { ...secureJsonFields, [key]: false },
      secureJsonData: { ...secureJsonData, [key]: '' },
    });
  };

  const onNumberChange = (key: 'maxPages' | 'cacheTtlSeconds') => (event: ChangeEvent<HTMLInputElement>) => {
    const raw = event.target.value;
    updateJsonData(key, raw === '' ? undefined : Number(raw));
  };

  return (
    <>
      <DataSourceDescription dataSourceName="n8n" docsLink={DOCS_URL} hasRequiredFields />

      <Divider spacing={4} />

      <ConfigSection
        title="Connection"
        description="The n8n public API. Grafana's backend calls it; the browser never talks to n8n directly."
      >
        <Field
          label="URL"
          required
          description="Base URL of the n8n public API, ending with /api/v1."
          invalid={urlInvalid}
          error={urlInvalid ? 'Must start with http:// or https://' : undefined}
        >
          <Input
            id="config-editor-url"
            aria-label="URL"
            value={options.url}
            placeholder="https://<instance>.app.n8n.cloud/api/v1"
            width={60}
            onChange={(e: ChangeEvent<HTMLInputElement>) => onOptionsChange({ ...options, url: e.target.value })}
          />
        </Field>
        <Stack direction="column" gap={0.5}>
          <Text variant="bodySmall" color="secondary">
            n8n Cloud: <span className={styles.code}>{'https://<instance>.app.n8n.cloud/api/v1'}</span>
          </Text>
          <Text variant="bodySmall" color="secondary">
            Self-hosted: <span className={styles.code}>{'http://<host>:5678/api/v1'}</span> (add N8N_PATH if n8n runs
            under a sub path)
          </Text>
          {urlMissingApiPath && (
            <Text variant="bodySmall" color="warning">
              The URL does not end with /api/v1. n8n answers other paths with its web UI or 404.
            </Text>
          )}
        </Stack>
      </ConfigSection>

      <Divider spacing={4} />

      <ConfigSection title="Authentication">
        <Field
          label="API key"
          required
          description="Create a key in n8n under Settings > n8n API, ideally for a dedicated user. Sent as X-N8N-API-KEY header."
        >
          <SecretInput
            id="config-editor-api-key"
            aria-label="API key"
            isConfigured={!!secureJsonFields.apiKey}
            value={secureJsonData?.apiKey ?? ''}
            placeholder="n8n API key"
            width={60}
            onReset={() => resetSecret('apiKey')}
            onChange={(e: ChangeEvent<HTMLInputElement>) => updateSecret('apiKey', e.target.value)}
          />
        </Field>
        <Text variant="bodySmall" color="secondary">
          The plugin only reads. Give the key read scopes only:{' '}
          <span className={styles.code}>{READ_SCOPES.join(', ')}</span>.
        </Text>
      </ConfigSection>

      <Divider spacing={4} />

      <ConfigSection
        title="TLS"
        description="Only needed for self-hosted n8n with a certificate from a private CA."
        isCollapsible
        isInitiallyOpen={tlsConfigured}
      >
        <Field label="Skip TLS verify" description="Do not verify the server certificate. Use only for testing.">
          <Switch
            id="config-editor-tls-skip-verify"
            value={!!jsonData.tlsSkipVerify}
            onChange={(e) => updateJsonData('tlsSkipVerify', e.currentTarget.checked)}
          />
        </Field>
        <Field label="With CA cert" description="Verify the server certificate against a custom CA certificate.">
          <Switch
            id="config-editor-tls-with-ca"
            value={!!jsonData.tlsAuthWithCACert}
            onChange={(e) => updateJsonData('tlsAuthWithCACert', e.currentTarget.checked)}
          />
        </Field>
        {jsonData.tlsAuthWithCACert && (
          <Field label="CA cert" required description="PEM encoded CA certificate.">
            <SecretTextArea
              id="config-editor-tls-ca-cert"
              isConfigured={!!secureJsonFields.tlsCACert}
              placeholder="-----BEGIN CERTIFICATE-----"
              rows={6}
              cols={60}
              onReset={() => resetSecret('tlsCACert')}
              onChange={(e: ChangeEvent<HTMLTextAreaElement>) => updateSecret('tlsCACert', e.target.value)}
            />
          </Field>
        )}
      </ConfigSection>

      <Divider spacing={4} />

      <ConfigSection
        title="Additional settings"
        description="Query limits, caching and network options."
        isCollapsible
        isInitiallyOpen={additionalConfigured}
      >
        <ConfigSubSection
          title="Query limits"
          description={
            <>
              Executions are loaded from n8n in pages of {PAGE_SIZE}, newest first. If a time range holds more
              executions than one query may load, panels show &quot;Incomplete data&quot; with the value needed here.
              See{' '}
              <TextLink href={`${DOCS_URL}#limitations`} external>
                limitations
              </TextLink>
              .
            </>
          }
        >
          <Field
            label="Max pages"
            description={`Pages per query (1-${MAX_PAGES}). ${maxPages} pages = up to ${(maxPages * PAGE_SIZE).toLocaleString('en-US')} executions per query and status/workflow filter. Higher values cover busier instances but make dashboards slower.`}
            invalid={maxPagesInvalid}
            error={maxPagesInvalid ? `Must be between 1 and ${MAX_PAGES}` : undefined}
          >
            <Input
              id="config-editor-max-pages"
              aria-label="Max pages"
              type="number"
              min={1}
              max={MAX_PAGES}
              width={20}
              placeholder={String(DEFAULT_MAX_PAGES)}
              value={jsonData.maxPages ?? ''}
              onChange={onNumberChange('maxPages')}
            />
          </Field>
          <Field
            label="Cache TTL (seconds)"
            description="How long API responses are cached in the Grafana backend. 0 disables the cache."
          >
            <Input
              id="config-editor-cache-ttl"
              aria-label="Cache TTL"
              type="number"
              min={0}
              max={3600}
              width={20}
              placeholder={String(DEFAULT_CACHE_TTL_SECONDS)}
              value={jsonData.cacheTtlSeconds ?? ''}
              onChange={onNumberChange('cacheTtlSeconds')}
            />
          </Field>
        </ConfigSubSection>

        {config.secureSocksDSProxyEnabled && (
          <ConfigSubSection
            title="Private data source connect"
            description="Reach a self-hosted n8n in a private network through a PDC agent (Grafana Cloud)."
          >
            <Field
              label="Enabled"
              description="Route requests to n8n through the PDC agent of this Grafana stack. The agent must reach the URL above."
            >
              <Switch
                id="config-editor-pdc"
                value={!!jsonData.enableSecureSocksProxy}
                onChange={(e) => updateJsonData('enableSecureSocksProxy', e.currentTarget.checked)}
              />
            </Field>
          </ConfigSubSection>
        )}
      </ConfigSection>
    </>
  );
}
