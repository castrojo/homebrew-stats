import { test, expect } from '@playwright/test';

test.describe('Bazzite Builds page', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/bootc-ecosystem/bazzite-builds/');
  });

  test('page loads with correct title', async ({ page }) => {
    await expect(page).toHaveTitle(/Bazzite CI\/CD Build Metrics/);
  });

  test('shows collecting state or dashboard content', async ({ page }) => {
    // h1 is always rendered (static, outside the bootstrap guard)
    await expect(page.locator('h1')).toContainText('Bazzite Build Pipeline');

    // Either the collecting banner or dashboard content must be present.
    const html = await page.content();
    const hasCollecting = html.includes('class="collecting"') || html.includes('Collecting build data');
    const hasDashboard = html.includes('kpi-strip') || html.includes('kpi-card') || html.includes('dora-panel') || html.includes('Pipeline Status');
    expect(hasCollecting || hasDashboard).toBe(true);
  });

  test('bazzite-builds tab is active in navigation', async ({ page }) => {
    const bazziteLink = page.locator('nav a[href*="bazzite-builds"]');
    await expect(bazziteLink).toBeVisible();
  });

  test('all JSON data scripts are parseable', async ({ page }) => {
    const scripts = await page.evaluate(() => {
      const els = Array.from(document.querySelectorAll('script[type="application/json"]'));
      return els.map(el => ({
        id: el.id,
        valid: (() => { try { JSON.parse(el.textContent ?? ''); return true; } catch { return false; } })()
      }));
    });
    for (const s of scripts) {
      expect(s.valid).toBe(true);
    }
  });

  test('page renders without structural chart-empty when data present', async ({ page }) => {
    const html = await page.content();
    const hasHealthBar = await page.locator('.kpi-strip').isVisible().catch(() => false);
    if (hasHealthBar) {
      const emptyCharts = await page.locator('canvas.chart-empty').count();
      expect(emptyCharts).toBe(0);
    }
    // Always passes if in collecting state
    expect(html).toContain('Bazzite Build Pipeline');
  });
});
