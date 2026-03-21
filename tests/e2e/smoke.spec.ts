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
    await page.goto('/homebrew-stats/testhub/');
    // tbody is SSR'd — wait for it to be attached (script/hidden elements need state: 'attached')
    await page.waitForSelector('#testhub-tbody', { state: 'attached', timeout: 15_000 });

    const statusCells = await page.locator('#testhub-tbody td:nth-child(4)').allTextContents();
    const known = statusCells.filter(s => s.includes('🟢') || s.includes('🔴'));

    expect(
      known.length,
      `Expected at least one package with a known build status (🟢/🔴), but all ${statusCells.length} show ⚪ unknown. ` +
      'This indicates build_metrics is empty — check testhub cache state and seed file.'
    ).toBeGreaterThan(0);
  });

  test('meta.json: generated_at reflects today', async ({ page }) => {
    const today = new Date().toISOString().slice(0, 10); // YYYY-MM-DD
    const response = await page.goto(`/homebrew-stats/meta.json?cb=${Date.now()}`);
    expect(response?.status()).toBe(200);

    const body = await response?.text();
    const meta = JSON.parse(body ?? '{}') as { generated_at?: string };
    expect(
      meta.generated_at,
      `meta.json generated_at="${meta.generated_at}" but today is ${today}. Site data may be stale.`
    ).toBe(today);
  });

  test('homebrew: traffic history has data', async ({ page }) => {
    await page.goto('/homebrew-stats/');
    // traffic-data is a SSR'd <script type="application/json"> element — use 'attached' not 'visible'
    await page.waitForSelector('#traffic-data', { state: 'attached', timeout: 15_000 });

    const data = await getScriptJSON(page, 'traffic-data') as { history?: unknown[] };
    expect(
      (data.history ?? []).length,
      'traffic-data.history is empty — homebrew data may be missing or sync failed.'
    ).toBeGreaterThan(0);
  });
});
