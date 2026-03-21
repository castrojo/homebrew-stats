import { test, expect } from "@playwright/test";
import type { Page } from '@playwright/test';

/**
 * Chart rendering regression suite.
 *
 * Guards against:
 *   1. set:text / JSON injection bugs — data is literal template text not JSON
 *   2. Chart.js rendering failures — canvas is blank or zero-sized
 *   3. Key contract regressions — wrong case for distro/os_version keys
 */

// ─── helpers ─────────────────────────────────────────────────────────────────

/** Use page.evaluate to extract and parse JSON from a <script type="application/json"> tag.
 *  Playwright's locator() does not reliably find <script> elements; evaluate() is the correct approach. */
async function getScriptJSON(page: Page, id: string): Promise<unknown> {
  const result = await page.evaluate((scriptId: string) => {
    const el = document.getElementById(scriptId);
    if (!el) return { error: `Element #${scriptId} not found in DOM` };
    const content = el.textContent ?? '';
    try {
      return { data: JSON.parse(content) };
    } catch (e) {
      return { error: `JSON.parse failed: ${String(e)}`, preview: content.slice(0, 200) };
    }
  }, id);

  const r = result as { error?: string; preview?: string; data?: unknown };
  if (r.error) {
    throw new Error(`script#${id}: ${r.error}${r.preview ? `\nContent: ${r.preview}` : ''}`);
  }
  return r.data;
}

async function expectCanvasRendered(page: Page, canvasId: string) {
  const canvas = page.locator(`canvas#${canvasId}`);
  await expect(canvas, `canvas#${canvasId} must exist`).toBeAttached();
  const box = await canvas.boundingBox();
  expect(box, `canvas#${canvasId} must have a bounding box — Chart.js did not render`).not.toBeNull();
  expect(box!.width, `canvas#${canvasId} width must be > 0`).toBeGreaterThan(0);
  expect(box!.height, `canvas#${canvasId} height must be > 0`).toBeGreaterThan(0);
}

// ─── Homebrew tab ─────────────────────────────────────────────────────────────

test.describe('Homebrew tab', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/homebrew-stats/');
    await page.waitForLoadState('networkidle');
  });

  test('traffic-data has valid JSON with non-empty history', async ({ page }) => {
    const data = await getScriptJSON(page, 'traffic-data') as Record<string, unknown>;
    expect(Array.isArray(data.history), 'traffic-data.history must be an array').toBe(true);
    expect((data.history as unknown[]).length, 'history must be non-empty').toBeGreaterThan(0);
  });

  test('downloads-data-ublue-os-homebrew-tap has valid JSON', async ({ page }) => {
    await getScriptJSON(page, 'downloads-data-ublue-os-homebrew-tap');
  });

  test('comparison-data has valid JSON with non-empty history', async ({ page }) => {
    const data = await getScriptJSON(page, 'comparison-data') as Record<string, unknown>;
    expect(Array.isArray(data.relevant), 'comparison-data.relevant must be an array').toBe(true);
    expect((data.relevant as unknown[]).length).toBeGreaterThan(0);
  });

  test('os-data has valid JSON with periods', async ({ page }) => {
    const data = await getScriptJSON(page, 'os-data') as Record<string, unknown>;
    expect(typeof data.periods, 'os-data.periods must be an object').toBe('object');
  });

  test('TrafficChart canvas is rendered by Chart.js', async ({ page }) => {
    await expectCanvasRendered(page, 'traffic-chart');
  });

  test('TapComparisonChart canvas is rendered by Chart.js', async ({ page }) => {
    await expectCanvasRendered(page, 'tap-comparison-chart');
  });
});

// ─── Testhub tab ─────────────────────────────────────────────────────────────

test.describe('Testhub tab', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/homebrew-stats/testhub/');
    await page.waitForLoadState('networkidle');
  });

  test('testhub-build-data has valid JSON with non-empty history', async ({ page }) => {
    const data = await getScriptJSON(page, 'testhub-build-data') as Record<string, unknown>;
    expect(Array.isArray(data.history), 'testhub-build-data.history must be an array').toBe(true);
    expect((data.history as unknown[]).length, 'testhub history must be non-empty').toBeGreaterThan(0);
  });

  test('testhub-version-data has valid JSON with non-empty history', async ({ page }) => {
    const data = await getScriptJSON(page, 'testhub-version-data') as Record<string, unknown>;
    expect(Array.isArray(data.history), 'testhub-version-data.history must be an array').toBe(true);
    expect((data.history as unknown[]).length).toBeGreaterThan(0);
  });

  test('TesthubBuildChart canvas is rendered by Chart.js', async ({ page }) => {
    await expectCanvasRendered(page, 'testhub-build-chart');
  });

  test('TesthubVersionTimeline canvas is rendered by Chart.js', async ({ page }) => {
    await expectCanvasRendered(page, 'testhub-version-chart');
  });

  test('Package table has at least one data row', async ({ page }) => {
    const rows = page.locator('table tbody tr');
    await expect(rows.first()).toBeVisible();
    expect(await rows.count()).toBeGreaterThan(0);
  });
});

// ─── Overall tab ─────────────────────────────────────────────────────────────

test.describe('Overall tab', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/homebrew-stats/overall/');
    await page.waitForLoadState('networkidle');
  });

  test('countme-trend-data has valid JSON with non-empty sorted weeks and lowercase distro keys', async ({ page }) => {
    const data = await getScriptJSON(page, 'countme-trend-data') as Record<string, unknown>;
    expect(Array.isArray(data.sorted), 'countme-trend-data.sorted must be an array').toBe(true);
    expect((data.sorted as unknown[]).length, 'sorted weeks must be non-empty').toBeGreaterThan(0);

    // REGRESSION GUARD: distros must use lowercase keys (bazzite vs Bazzite case bug)
    const week = (data.sorted as Array<Record<string, unknown>>)[0];
    const distros = week.distros as Record<string, number>;
    expect(distros.bazzite, 'distros must use lowercase key "bazzite" not "Bazzite"').toBeGreaterThan(0);
    expect(distros['bluefin-lts'], 'distros must have "bluefin-lts" key').toBeDefined();
  });

  test('ecosystem-pie-data has valid JSON with non-zero currentWeek total', async ({ page }) => {
    const data = await getScriptJSON(page, 'ecosystem-pie-data') as Record<string, unknown>;
    const cw = data.currentWeek as Record<string, unknown> | null;
    expect(cw, 'ecosystem-pie-data.currentWeek must not be null').not.toBeNull();
    expect(typeof cw!.total, 'currentWeek.total must be a number').toBe('number');
    expect(cw!.total as number, 'currentWeek.total must be > 0').toBeGreaterThan(0);
  });

  test('fedora-version-data has valid JSON with title-case osVersionDist keys', async ({ page }) => {
    const data = await getScriptJSON(page, 'fedora-version-data') as Record<string, unknown>;
    const ovd = data.osVersionDist as Record<string, unknown>;
    expect(typeof ovd, 'fedora-version-data.osVersionDist must be an object').toBe('object');
    // REGRESSION GUARD: osVersionDist uses title-case keys (different contract from distros lowercase)
    expect(ovd.Bazzite, '"Bazzite" key must exist in osVersionDist (title-case contract)').toBeTruthy();
    expect(ovd.Bluefin, '"Bluefin" key must exist in osVersionDist').toBeTruthy();
  });

  test('CountmeTrendChart canvas is rendered by Chart.js', async ({ page }) => {
    await expectCanvasRendered(page, 'countme-trend-chart');
  });

  test('EcosystemPieChart canvas is rendered by Chart.js', async ({ page }) => {
    await expectCanvasRendered(page, 'ecosystem-pie-chart');
  });

  test('FedoraVersionChart canvas is rendered by Chart.js', async ({ page }) => {
    await expectCanvasRendered(page, 'fedora-version-chart');
  });

  test('Countme explainer callout is visible and mentions countme', async ({ page }) => {
    const callout = page.locator('.explainer');
    await expect(callout).toBeVisible();
    await expect(callout).toContainText('countme');
  });
});

// ─── No-empty-state contract ──────────────────────────────────────────────────
// These tests enforce that no chart renders in the empty/placeholder state.
// An empty chart (class="chart-empty") means data failed to reach the component.
// This is the E2E equivalent of the CI "Verify charts have data" step, and
// catches the case where the CI grep gives a false negative.

test.describe('No empty charts contract', () => {
  async function assertNoEmptyCharts(page: Page, url: string) {
    await page.goto(url);
    const emptyCharts = await page.locator('.chart-empty').all();
    if (emptyCharts.length > 0) {
      // Collect context for easier debugging
      const messages = await Promise.all(
        emptyCharts.map(el => el.textContent())
      );
      throw new Error(
        `${emptyCharts.length} empty chart(s) found on ${url}:\n` +
        messages.map((m, i) => `  [${i + 1}] "${m?.trim()}"`).join('\n')
      );
    }
  }

  test('Homebrew tab has no empty charts', async ({ page }) => {
    await assertNoEmptyCharts(page, '/homebrew-stats/');
  });

  test('Testhub tab has no empty charts', async ({ page }) => {
    await assertNoEmptyCharts(page, '/homebrew-stats/testhub/');
  });

  test('Overall tab has no empty charts', async ({ page }) => {
    await assertNoEmptyCharts(page, '/homebrew-stats/overall/');
  });
});

// ─── IssueButton ─────────────────────────────────────────────────────────────

test.describe('IssueButton', () => {
  for (const [tab, url] of [
    ['Homebrew', '/homebrew-stats/'],
    ['Testhub', '/homebrew-stats/testhub/'],
    ['Overall', '/homebrew-stats/overall/'],
  ]) {
    test(`${tab} tab has a "File an issue" link`, async ({ page }) => {
      await page.goto(url as string);
      const link = page.locator('a[href*="castrojo/homebrew-stats/issues/new"]');
      await expect(link, `${tab} tab must have a file-an-issue link`).toBeVisible();
    });
  }
});

// ─── Interactive chart controls ───────────────────────────────────────────────
// These tests click the new toggle buttons added in batch-issue session and
// verify the chart canvas survives the interaction (not destroyed, still has
// a non-zero bounding box).

test.describe('OsSection interactive controls', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/homebrew-stats/');
  });

  test('Atomic Focus toggle switches active button and keeps canvas rendered', async ({ page }) => {
    const atomicBtn = page.locator('#os-focus-btns .range-btn[data-focus="atomic"]');
    const allBtn    = page.locator('#os-focus-btns .range-btn[data-focus="all"]');

    await expect(allBtn).toHaveClass(/active/);
    await atomicBtn.click();
    await expect(atomicBtn).toHaveClass(/active/);
    await expect(allBtn).not.toHaveClass(/active/);
    await expectCanvasRendered(page, 'os-bar-chart');
  });

  test('Log scale toggle switches active button and keeps canvas rendered', async ({ page }) => {
    const logBtn    = page.locator('#os-scale-btns .range-btn[data-scale="log"]');
    const linearBtn = page.locator('#os-scale-btns .range-btn[data-scale="linear"]');

    await expect(linearBtn).toHaveClass(/active/);
    await logBtn.click();
    await expect(logBtn).toHaveClass(/active/);
    await expect(linearBtn).not.toHaveClass(/active/);
    await expectCanvasRendered(page, 'os-bar-chart');
  });
});

test.describe('FedoraVersionChart log scale toggle', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/homebrew-stats/overall/');
  });

  test('Log scale toggle switches active button and keeps canvas rendered', async ({ page }) => {
    const logBtn    = page.locator('#fedora-scale-btns .range-btn[data-scale="log"]');
    const linearBtn = page.locator('#fedora-scale-btns .range-btn[data-scale="linear"]');

    await expect(linearBtn).toHaveClass(/active/);
    await logBtn.click();
    await expect(logBtn).toHaveClass(/active/);
    await expect(linearBtn).not.toHaveClass(/active/);
    await expectCanvasRendered(page, 'fedora-version-chart');
  });
});

// ─── Testhub data quality ─────────────────────────────────────────────────────

test.describe('Testhub data quality', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/homebrew-stats/testhub/');
  });

  test('KPI cards have all expected labels', async ({ page }) => {
    const labels = await page.locator('.kpi-label').allTextContents();
    const normalized = labels.map(l => l.trim().toUpperCase());
    expect(normalized).toContain('TOTAL PACKAGES');
    expect(normalized).toContain('LATEST BUILD STATUS');
    expect(normalized).toContain('UPDATED THIS WEEK');
    expect(normalized).toContain('BUILD RUNS (30D)');
    // 'TOTAL PULLS' is conditional — omitted until OCI pull API available (#13)
  });

  test('Package table has all expected column headers', async ({ page }) => {
    const headers = await page.locator('#testhub-table thead th').allTextContents();
    const text = headers.join(' ');
    expect(text).toContain('Package');
    expect(text).toContain('Build Status');
    expect(text).toContain('Version Count');
    expect(text).toContain('Pulls');
  });

  test('At least one package has a known build status', async ({ page }) => {
    // DATA QUALITY: this assertion depends on cache warmth and live API data.
    // Pre-deploy E2E only tests RENDERING; data quality belongs in smoke-test.yml.
    // On a cold cache (e.g. after a cache key bump), build_metrics legitimately
    // starts empty — blocking deploy here causes outages. Warn and skip instead.
    await page.goto('/homebrew-stats/testhub/');
    const statusCells = await page.locator('#testhub-tbody td:nth-child(4)').allTextContents();
    const known = statusCells.filter(s => s.includes('🟢') || s.includes('🔴'));
    if (known.length === 0) {
      console.warn(
        '⚠️  All packages show unknown build status — testhub cache may be cold. ' +
        'This is expected on the first run after a cache key bump. ' +
        'DATA QUALITY checks belong in smoke-test.yml (post-deploy), not here.'
      );
      test.skip();
    }
    expect(known.length).toBeGreaterThan(0);
  });
});
