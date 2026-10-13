import { test, expect } from '@grafana/plugin-e2e';

// The provisioned datasource (provisioning/datasources/datasources.yml) points to the
// n8n of the dev stack and gets its API key from .env, see `make dev`.
const PROVISIONED_UID = 'n8n-local';

test('smoke: should render config editor', async ({ createDataSourceConfigPage, readProvisionedDataSource, page }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await createDataSourceConfigPage({ type: ds.type });
  await expect(page.getByRole('textbox', { name: 'URL' })).toBeVisible();
  await expect(page.getByPlaceholder('n8n API key')).toBeVisible();
});

test('"Save & test" should be successful for the provisioned n8n', async ({ gotoDataSourceConfigPage }) => {
  const configPage = await gotoDataSourceConfigPage(PROVISIONED_UID);
  await expect(configPage.saveAndTest()).toBeOK();
  await expect(configPage).toHaveAlert('success', { hasText: 'Connected to the n8n API' });
});

test('"Save & test" should fail when the API key is missing', async ({
  createDataSourceConfigPage,
  readProvisionedDataSource,
  page,
}) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  const configPage = await createDataSourceConfigPage({ type: ds.type });
  await page.getByRole('textbox', { name: 'URL' }).fill('http://n8n:5678/api/v1');
  await expect(configPage.saveAndTest()).not.toBeOK();
  await expect(configPage).toHaveAlert('error', { hasText: 'API key is missing' });
});

test('"Save & test" should explain a URL without /api/v1', async ({
  createDataSourceConfigPage,
  readProvisionedDataSource,
  page,
}) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  const configPage = await createDataSourceConfigPage({ type: ds.type });
  await page.getByRole('textbox', { name: 'URL' }).fill('http://n8n:5678');
  await page.getByPlaceholder('n8n API key').fill('any-key');
  await expect(configPage.saveAndTest()).not.toBeOK();
  await expect(configPage).toHaveAlert('error', { hasText: 'Try http://n8n:5678/api/v1' });
});
