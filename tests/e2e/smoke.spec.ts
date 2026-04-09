/**
 * smoke.spec.ts — Post-deploy data quality checks.
 *
 * These tests run ONLY against the live deployed site (when BASE_URL is set).
 * They are skipped during pre-deploy CI (no BASE_URL → local preview).
 *
 * Triggered by smoke-test.yml via `workflow_run` after a successful deploy.
 * Failures here do NOT block the deploy — they fire as a post-deploy alert.
 *
 * Rule: data quality tests belong here, NOT in charts.spec.ts.
 * charts.spec.ts tests RENDERING only (structure, canvas, JSON parseable).
 */

import { test, expect, type Page } from '@playwright/test';

const isLiveSite = !!process.env.BASE_URL;

/** Extract and parse JSON from a SSR'd <script type="application/json"> element. */
async function getScriptJSON(page: Page, id: string): Promise<unknown> {
  const result = await page.evaluate((scriptId: string) => {
    const el = document.getElementById(scriptId);
    if (!el) return { error: `Element #${scriptId} not found` };
    try {
      return { data: JSON.parse(el.textContent ?? '') };
    } catch (e) {
      return { error: `JSON.parse failed: ${String(e)}` };
    }
  }, id);
  const r = result as { error?: string; data?: unknown };
  if (r.error) throw new Error(`script#${id}: ${r.error}`);
  return r.data;
}

test.describe('Smoke — data quality (live site only)', () => {
  test.skip(!isLiveSite, 'Smoke tests only run against the live deployed site (BASE_URL must be set)');

  test('testhub: at least one package has a known build status', async ({ page }) => {
    await page.goto('/bootc-ecosystem/testhub/');
    // tbody is SSR'd — wait for it to be attached (script/hidden elements need state: 'attached')
    await page.waitForSelector('#testhub-tbody', { state: 'attached', timeout: 15_000 });

    // Build Status is the 2nd column: Package | Build Status | Arch | Last Published.
    // TesthubPackageTable renders "✅ Passing" and "❌ Failing" (not 🟢/🔴).
    const statusCells = await page.locator('#testhub-tbody td:nth-child(2)').allTextContents();
    const known = statusCells.filter(s => s.includes('✅') || s.includes('❌'));

    expect(
      known.length,
      `Expected at least one package with a known build status (✅ Passing / ❌ Failing), ` +
      `but all ${statusCells.length} show ⏳ Pending or Unknown. ` +
      'This indicates build_metrics is empty — check testhub cache state and seed file.'
    ).toBeGreaterThan(0);
  });

  test('meta.json: generated_at reflects today', async ({ page }) => {
    const today = new Date().toISOString().slice(0, 10); // YYYY-MM-DD
    const response = await page.goto(`/bootc-ecosystem/meta.json?cb=${Date.now()}`);
    expect(response?.status()).toBe(200);

    const body = await response?.text();
    const meta = JSON.parse(body ?? '{}') as { generated_at?: string };
    expect(
      meta.generated_at,
      `meta.json generated_at="${meta.generated_at}" but today is ${today}. Site data may be stale.`
    ).toBe(today);
  });

  test('homebrew: traffic history has data', async ({ page }) => {
    await page.goto('/bootc-ecosystem/');
    // traffic-data is a SSR'd <script type="application/json"> element — use 'attached' not 'visible'
    await page.waitForSelector('#traffic-data', { state: 'attached', timeout: 15_000 });

    const data = await getScriptJSON(page, 'traffic-data') as { history?: unknown[] };
    expect(
      (data.history ?? []).length,
      'traffic-data.history is empty — homebrew data may be missing or sync failed.'
    ).toBeGreaterThan(0);
  });

  test('testhub: build and version history have data', async ({ page }) => {
    await page.goto('/bootc-ecosystem/testhub/');
    await page.waitForSelector('#testhub-build-data', { state: 'attached', timeout: 15_000 });

    const buildData = await getScriptJSON(page, 'testhub-build-data') as { history?: unknown[] };
    expect((buildData.history ?? []).length, 'testhub-build-data.history is empty').toBeGreaterThan(0);

    const versionData = await getScriptJSON(page, 'testhub-version-data') as { history?: unknown[] };
    expect((versionData.history ?? []).length, 'testhub-version-data.history is empty').toBeGreaterThan(0);
  });

  test('testhub: package table has data rows', async ({ page }) => {
    await page.goto('/bootc-ecosystem/testhub/');
    const rows = page.locator('#testhub-tbody tr');
    expect(await rows.count(), 'Testhub package table is empty').toBeGreaterThan(0);
  });

  test('overall: countme trend and ecosystem data have non-zero values', async ({ page }) => {
    await page.goto('/bootc-ecosystem/overall/');
    await page.waitForSelector('#countme-trend-data', { state: 'attached', timeout: 15_000 });

    const trendData = await getScriptJSON(page, 'countme-trend-data') as { monthly?: Array<{ distros: Record<string, number> }> };
    expect((trendData.monthly ?? []).length, 'countme-trend-data.monthly is empty').toBeGreaterThan(0);
    const latestWeek = trendData.monthly![0];
    expect(latestWeek.distros.bazzite, 'Bazzite countme value is 0').toBeGreaterThan(0);

    const pieData = await getScriptJSON(page, 'ecosystem-pie-data') as { currentWeek?: { total: number } };
    expect(pieData.currentWeek?.total, 'Ecosystem pie total is 0').toBeGreaterThan(0);
  });

  test('contributors: all data sources have non-empty data', async ({ page }) => {
    await page.goto('/bootc-ecosystem/contributors/');
    await page.waitForSelector('#commit-activity-data', { state: 'attached', timeout: 15_000 });

    const commitData = await getScriptJSON(page, 'commit-activity-data') as { repos?: unknown[] };
    expect((commitData.repos ?? []).length, 'commit-activity-data.repos is empty').toBeGreaterThan(0);

    const leaderboardData = await getScriptJSON(page, 'contributor-leaderboard-data') as { topContributors?: unknown[] };
    expect((leaderboardData.topContributors ?? []).length, 'contributor-leaderboard-data is empty').toBeGreaterThan(0);

    const busFactorData = await getScriptJSON(page, 'bus-factor-data') as { summary?: { bus_factor: number } };
    expect(busFactorData.summary?.bus_factor, 'Bus factor is 0').toBeGreaterThan(0);
  });
});
