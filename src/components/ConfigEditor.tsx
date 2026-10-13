import React, { ChangeEvent } from 'react';
import {
  Collapse,
  Field,
  FieldSet,
  Input,
  SecretInput,
  SecretTextArea,
  SecureSocksProxySettings,
  Switch,
} from '@grafana/ui';
import { config } from '@grafana/runtime';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import { DEFAULT_CACHE_TTL_SECONDS, DEFAULT_MAX_PAGES, N8nDataSourceOptions, N8nSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<N8nDataSourceOptions, N8nSecureJsonData> {}

const URL_PATTERN = /^https?:\/\//;

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;
  const [advancedOpen, setAdvancedOpen] = React.useState(false);

  const urlInvalid = !!options.url && !URL_PATTERN.test(options.url);

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
      <FieldSet label="Connection">
        <Field
          label="URL"
          description={
            <>
              Base URL of the n8n public API including <code>/api/v1</code>. n8n Cloud:{' '}
              <code>https://&lt;instance&gt;.app.n8n.cloud/api/v1</code>, self-hosted:{' '}
              <code>http://&lt;host&gt;:5678/api/v1</code>
            </>
          }
          required
          invalid={urlInvalid}
          error={urlInvalid ? 'Must start with http:// or https://' : undefined}
        >
          <Input
            id="config-editor-url"
            value={options.url}
            placeholder="https://<instance>.app.n8n.cloud/api/v1"
            width={60}
            onChange={(e: ChangeEvent<HTMLInputElement>) => onOptionsChange({ ...options, url: e.target.value })}
          />
        </Field>
        <Field
          label="API key"
          description="Create a key in n8n under Settings > n8n API. Use a dedicated user for Grafana; on Enterprise restrict the key to read scopes."
          required
        >
          <SecretInput
            id="config-editor-api-key"
            isConfigured={!!secureJsonFields.apiKey}
            value={secureJsonData?.apiKey ?? ''}
            placeholder="n8n API key"
            width={60}
            onReset={() => resetSecret('apiKey')}
            onChange={(e: ChangeEvent<HTMLInputElement>) => updateSecret('apiKey', e.target.value)}
          />
        </Field>
      </FieldSet>

      <FieldSet label="TLS">
        <Field label="Skip TLS verify" description="Do not verify the server certificate. Use only for testing.">
          <Switch
            id="config-editor-tls-skip-verify"
            value={!!jsonData.tlsSkipVerify}
            onChange={(e) => updateJsonData('tlsSkipVerify', e.currentTarget.checked)}
          />
        </Field>
        <Field
          label="With CA cert"
          description="Verify the server certificate against a custom CA, e.g. an internal PKI."
        >
          <Switch
            id="config-editor-tls-with-ca"
            value={!!jsonData.tlsAuthWithCACert}
            onChange={(e) => updateJsonData('tlsAuthWithCACert', e.currentTarget.checked)}
          />
        </Field>
        {jsonData.tlsAuthWithCACert && (
          <Field label="CA cert" required>
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
      </FieldSet>

      <Collapse label="Advanced" isOpen={advancedOpen} onToggle={() => setAdvancedOpen(!advancedOpen)}>
        <Field
          label="Max pages"
          description="Upper limit of API pages (250 executions each) fetched per query (1-100)."
        >
          <Input
            id="config-editor-max-pages"
            type="number"
            min={1}
            max={100}
            width={20}
            placeholder={String(DEFAULT_MAX_PAGES)}
            value={jsonData.maxPages ?? ''}
            onChange={onNumberChange('maxPages')}
          />
        </Field>
        <Field
          label="Cache TTL (seconds)"
          description="How long API responses are cached in the backend. 0 disables the cache."
        >
          <Input
            id="config-editor-cache-ttl"
            type="number"
            min={0}
            max={3600}
            width={20}
            placeholder={String(DEFAULT_CACHE_TTL_SECONDS)}
            value={jsonData.cacheTtlSeconds ?? ''}
            onChange={onNumberChange('cacheTtlSeconds')}
          />
        </Field>
        {config.secureSocksDSProxyEnabled && (
          <SecureSocksProxySettings options={options} onOptionsChange={onOptionsChange} />
        )}
      </Collapse>
    </>
  );
}
