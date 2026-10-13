import { test, expect } from '@grafana/plugin-e2e';

test('smoke: should render query editor', async ({ panelEditPage, readProvisionedDataSource }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await panelEditPage.datasource.set(ds.name);
  const row = panelEditPage.getQueryEditorRow('A');
  await expect(row.getByRole('radio', { name: 'Execution stats' })).toBeChecked();
  await expect(row.getByRole('combobox', { name: 'Metric' })).toBeVisible();
});

test('executions query returns rows from n8n', async ({ panelEditPage, readProvisionedDataSource }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await panelEditPage.datasource.set(ds.name);
  await panelEditPage.setVisualization('Table');
  const queryReq = panelEditPage.waitForQueryDataRequest();
  await panelEditPage.getQueryEditorRow('A').getByRole('radio', { name: 'Workflows' }).click();
  await expect(await queryReq).toBeTruthy();
  await expect(panelEditPage.refreshPanel()).toBeOK();
  await expect(panelEditPage.panel.data).toContainText(['seed: heartbeat']);
});
