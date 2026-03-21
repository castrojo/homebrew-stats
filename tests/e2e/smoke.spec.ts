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

import { test, expect } from '@playwright/test';

const isLiveSite = !!process.env.BASE_URL;

test.describe('Smoke — data quality (live site only)', () => {
  test.skip(!isLiveSite, 'Smoke tests only run against the live deployed site (BASE_URL must be set)');

  test('testhub: at least one package has a known build status', async ({ page }) => {
    await page.goto('/homebrew-stats/testhub/');
    await page.waitForLoadState('networkidle');

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

  test('homebrew: KPI total packages is non-zero', async ({ page }) => {
    await page.goto('/homebrew-stats/');
    await page.waitForLoadState('networkidle');

    // The stats-data script is always present; parse it to check summary.
    const raw = await page.locator('#stats-data').textContent();
    const stats = JSON.parse(raw ?? '{}') as { summary?: { total_packages?: number } };
    const totalPackages = stats.summary?.total_packages ?? 0;

    expect(
      totalPackages,
      'summary.total_packages is 0 — homebrew data may be missing or sync failed.'
    ).toBeGreaterThan(0);
  });
});
