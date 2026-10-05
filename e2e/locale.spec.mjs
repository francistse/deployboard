import { test, expect } from '@playwright/test';

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    try {
      localStorage.removeItem('deployboard:locale');
      localStorage.removeItem('launch-pilot:locale');
    } catch { /* private browsing */ }
  });
  await page.goto('/');
  await page.waitForSelector('.job-table, .job-table__empty', { timeout: 15_000 });
});

test('header language switch updates UI copy and persists across reload', async ({ page }) => {
  const headerSwitch = page.locator('.lang-switch--header');
  await expect(headerSwitch).toBeVisible();

  await expect(page.locator('header p')).toHaveText(/macOS launchd inventory/i);

  await headerSwitch.getByRole('radio', { name: '日本語' }).click();
  await expect(page.locator('html')).toHaveAttribute('lang', 'ja');
  await expect(page.locator('header p')).toContainText('macOS launchd');
  await expect(page.locator('header p')).not.toHaveText(/your deployments only/i);
  await expect(page.getByRole('button', { name: /設定/ }).first()).toBeVisible();

  const stored = await page.evaluate(() => localStorage.getItem('deployboard:locale'));
  expect(stored).toBe('ja');

  await page.reload();
  await page.waitForSelector('.job-table, .job-table__empty', { timeout: 15_000 });
  await expect(page.locator('html')).toHaveAttribute('lang', 'ja');
  await expect(headerSwitch.getByRole('radio', { name: '日本語' })).toHaveAttribute('aria-checked', 'true');
  await expect(page.getByRole('button', { name: /設定/ }).first()).toBeVisible();
});

test('settings panel language switch keeps pace with the header control', async ({ page }) => {
  await page.getByRole('button', { name: /Settings|設定|设置|設定/ }).first().click();
  const panel = page.locator('.settings-panel');
  await expect(panel).toBeVisible();

  await panel.getByRole('radio', { name: '简体中文' }).click();
  await expect(page.locator('html')).toHaveAttribute('lang', 'zh-Hans');
  await expect(panel.locator('h2')).toHaveText('设置');
  await expect(page.locator('.lang-switch--header').getByRole('radio', { name: '简体中文' }))
    .toHaveAttribute('aria-checked', 'true');
});

test('browser language maps Traditional Chinese tags to zh-Hant before first paint', async ({ browser }) => {
  const context = await browser.newContext({ locale: 'zh-TW' });
  const page = await context.newPage();
  await page.addInitScript(() => {
    try {
      localStorage.removeItem('deployboard:locale');
      localStorage.removeItem('launch-pilot:locale');
    } catch { /* private browsing */ }
  });
  await page.goto('/');
  await page.waitForSelector('.job-table, .job-table__empty', { timeout: 15_000 });
  await expect(page.locator('html')).toHaveAttribute('lang', 'zh-Hant');
  await expect(page.locator('.lang-switch--header').getByRole('radio', { name: '繁體中文' }))
    .toHaveAttribute('aria-checked', 'true');
  await context.close();
});
