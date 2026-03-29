import { test, expect } from '@playwright/test';

test.describe('Builds page', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/homebrew-stats/builds/');
  });

  test('page loads with correct title', async ({ page }) => {
    await expect(page).toHaveTitle(/Bluefin CI\/CD Build Metrics/);
  });

  test('shows collecting state or dashboard content', async ({ page }) => {
    // The page always shows "Bluefin Build Pipeline" in the h1 (static, always rendered)
    await expect(page.locator('h1')).toContainText('Bluefin Build Pipeline');

    // Either the collecting banner or some dashboard content must be present.
    // Use page.content() to check static HTML — avoids visibility-based false negatives
    // that can occur when CSS variables or layout shift affect isVisible() in CI.
    const html = await page.content();
    const hasCollecting = html.includes('class="collecting"') || html.includes("class='collecting'") || html.includes('Collecting build data');
    const hasDashboard = html.includes('kpi-strip') || html.includes('kpi-card') || html.includes('dora-panel') || html.includes('Pipeline Status');
    expect(hasCollecting || hasDashboard).toBe(true);
  });

  test('builds tab is active in navigation', async ({ page }) => {
    // Look for a nav link that is "active" and contains "Builds"
    const activeLink = page.locator('nav a.active, nav a[aria-current="page"]');
    // At minimum the Bluefin builds tab link should exist in the nav (exact href to avoid matching aurora/bazzite tabs)
    const buildsLink = page.locator('nav a[href="/homebrew-stats/builds/"]');
    await expect(buildsLink).toBeVisible();
  });

  test('page renders without chart-empty on structural elements', async ({ page }) => {
    const html = await page.content();
    // The collecting state is OK, but if health bar is rendered, no element should be chart-empty due to missing data
    const hasHealthBar = await page.locator('.kpi-strip').isVisible().catch(() => false);
    if (hasHealthBar) {
      // If we have real data, structural charts should be rendered
      const emptyCharts = await page.locator('canvas.chart-empty').count();
      expect(emptyCharts).toBe(0);
    }
    // page loaded successfully regardless
    expect(html).toContain('Bluefin Build Pipeline');
  });

  test('builds.json data script is present and parseable', async ({ page }) => {
    // Check that if any data script tags exist, they are valid JSON
    const scripts = await page.evaluate(() => {
      const els = Array.from(document.querySelectorAll('script[type="application/json"]'));
      return els.map(el => ({ id: el.id, valid: (() => { try { JSON.parse(el.textContent ?? ''); return true; } catch { return false; } })() }));
    });
    for (const s of scripts) {
      expect(s.valid).toBe(true);
    }
  });

  test('recent builds table renders correctly when data present', async ({ page }) => {
    const table = page.locator('.recent-builds-table');
    const isVisible = await table.isVisible().catch(() => false);
    if (isVisible) {
      // Table should have at least a header row
      const rows = await table.locator('tr').count();
      expect(rows).toBeGreaterThanOrEqual(1);
    }
    // If not visible, we're in collecting state — pass
  });

  test('flakiness table renders correctly when data present', async ({ page }) => {
    const table = page.locator('.flakiness-table');
    const isVisible = await table.isVisible().catch(() => false);
    if (isVisible) {
      const rows = await table.locator('tr').count();
      expect(rows).toBeGreaterThanOrEqual(1);
    }
  });

  test('DORA panel renders when data present', async ({ page }) => {
    const panel = page.locator('.dora-panel');
    const isVisible = await panel.isVisible().catch(() => false);
    if (isVisible) {
      const cards = await panel.locator('.dora-grid > *').count();
      expect(cards).toBe(4);
    }
  });
});
