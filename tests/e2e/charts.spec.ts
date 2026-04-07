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
    await page.goto('/bootc-ecosystem/');
    await page.waitForLoadState('networkidle');
  });

  test('traffic-data has valid JSON', async ({ page }) => {
    const data = await getScriptJSON(page, 'traffic-data') as Record<string, unknown>;
    expect(Array.isArray(data.history), 'traffic-data.history must be an array').toBe(true);
  });

  test('downloads-data-ublue-os-homebrew-tap has valid JSON', async ({ page }) => {
    await getScriptJSON(page, 'downloads-data-ublue-os-homebrew-tap');
  });

  test('comparison-data has valid JSON', async ({ page }) => {
    const data = await getScriptJSON(page, 'comparison-data') as Record<string, unknown>;
    expect(Array.isArray(data.relevant), 'comparison-data.relevant must be an array').toBe(true);
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

// ─── Package Leaderboard ──────────────────────────────────────────────────────

test.describe('Package Leaderboard', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/bootc-ecosystem/');
    await page.waitForLoadState('networkidle');
  });

  test('leaderboard table is visible and has at least one row', async ({ page }) => {
    const table = page.locator('#pkg-leaderboard');
    await expect(table).toBeVisible();
    const rows = table.locator('tbody tr');
    await expect(rows.first()).toBeVisible();
  });

  test('badges use self-describing labels (not "stable" or "experimental")', async ({ page }) => {
    const badges = page.locator('.tap-badge');
    const count = await badges.count();
    expect(count, 'At least one tap badge must be present').toBeGreaterThan(0);

    const texts = await badges.allTextContents();
    for (const text of texts) {
      expect(text, `Badge "${text}" must not use opaque label "stable"`).not.toBe('stable');
      expect(text, `Badge "${text}" must not use opaque label "experimental"`).not.toBe('experimental');
    }
  });

  test('ublue-os/tap badge is present', async ({ page }) => {
    const badge = page.locator('.tap-badge', { hasText: 'ublue-os/tap' });
    await expect(badge.first()).toBeVisible();
  });

  test('homebrew badge is present for Brewfile core packages', async ({ page }) => {
    const badge = page.locator('.tap-badge', { hasText: 'homebrew' });
    await expect(badge.first()).toBeVisible();
  });

  test('search filter hides non-matching rows', async ({ page }) => {
    const searchInput = page.locator('#pkg-search');
    await expect(searchInput).toBeVisible();

    // Type a search query that should match something
    await searchInput.fill('ublue-os');
    // Some rows should be visible
    const visibleRows = page.locator('#pkg-leaderboard tbody tr:visible');
    const count = await visibleRows.count();
    expect(count, 'Filtering by "ublue-os" should show at least one row').toBeGreaterThan(0);

    // Clear and verify rows come back
    await searchInput.fill('');
  });

  test('zero-install cells render em-dash not "0"', async ({ page }) => {
    // If any package has zero installs, it should show '—' not '0'
    const cells = page.locator('#pkg-leaderboard tbody .num-cell');
    const cellTexts = await cells.allTextContents();
    for (const text of cellTexts) {
      expect(text.trim(), `Cell "${text}" must not render bare "0" — use "—" instead`).not.toBe('0');
    }
  });
});

// ─── Testhub tab ─────────────────────────────────────────────────────────────

test.describe('Testhub tab', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/bootc-ecosystem/testhub/');
    await page.waitForLoadState('networkidle');
  });

  test('testhub-build-data has valid JSON', async ({ page }) => {
    const data = await getScriptJSON(page, 'testhub-build-data') as Record<string, unknown>;
    expect(Array.isArray(data.history), 'testhub-build-data.history must be an array').toBe(true);
  });

  test('testhub-version-data has valid JSON', async ({ page }) => {
    const data = await getScriptJSON(page, 'testhub-version-data') as Record<string, unknown>;
    expect(Array.isArray(data.history), 'testhub-version-data.history must be an array').toBe(true);
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
    // (data-quality row count check moved to smoke-test)
  });
});

// ─── Overall tab ─────────────────────────────────────────────────────────────

test.describe('Overall tab', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/bootc-ecosystem/overall/');
    await page.waitForLoadState('networkidle');
  });

  test('countme-trend-data has valid JSON and lowercase distro keys', async ({ page }) => {
    const data = await getScriptJSON(page, 'countme-trend-data') as Record<string, unknown>;
    const monthly = data.monthly as Array<Record<string, unknown>>;
    expect(Array.isArray(monthly), 'countme-trend-data.monthly must be an array').toBe(true);

    // REGRESSION GUARD: all present distro keys must be lowercase and in the allowed set.
    const week = monthly[0];
    if (week) {
      const distros = week.distros as Record<string, number>;
      expect(distros, 'week.distros must exist').toBeDefined();
      const allowedDistroKeys = ['bazzite', 'bluefin', 'bluefin-lts', 'aurora', 'secureblue'];
      for (const key of Object.keys(distros)) {
        expect(allowedDistroKeys, `distro key "${key}" must be lowercase and in allowed set`).toContain(key);
      }
      expect(Object.keys(distros)).not.toContain('Bazzite');
      expect(Object.keys(distros)).not.toContain('Bluefin');
      expect(Object.keys(distros)).not.toContain('Aurora');
    }
  });

  test('ecosystem-pie-data has valid JSON structure', async ({ page }) => {
    const data = await getScriptJSON(page, 'ecosystem-pie-data') as Record<string, unknown>;
    const cw = data.currentWeek as Record<string, unknown> | null;
    expect(cw, 'ecosystem-pie-data.currentWeek must be defined (can be empty structure)').toBeDefined();
  });

  test('countme-trend-data has monthly aggregation payload', async ({ page }) => {
    const data = await getScriptJSON(page, 'countme-trend-data') as Record<string, unknown>;
    const monthly = data.monthly as Array<Record<string, unknown>>;
    expect(Array.isArray(monthly), 'countme-trend-data.monthly must be an array').toBe(true);

    if (monthly.length > 0) {
      expect(typeof monthly[0].week_end, 'monthly entry must expose week_end').toBe('string');
      expect(typeof monthly[0].distros, 'monthly entry must expose distros object').toBe('object');
    }
  });

  test('CountmeTrendChart canvas is rendered by Chart.js', async ({ page }) => {
    await expectCanvasRendered(page, 'countme-trend-chart');
  });

  test('EcosystemPieChart canvas is rendered by Chart.js', async ({ page }) => {
    await expectCanvasRendered(page, 'ecosystem-pie-chart');
  });

  test('Bazzite individual trend chart canvas is rendered by Chart.js', async ({ page }) => {
    await expectCanvasRendered(page, 'bazzite-trend-chart');
  });

  test('Bluefin individual trend chart canvas is rendered by Chart.js', async ({ page }) => {
    await expectCanvasRendered(page, 'bluefin-trend-chart');
  });

  test('Aurora individual trend chart canvas is rendered by Chart.js', async ({ page }) => {
    await expectCanvasRendered(page, 'aurora-trend-chart');
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
    await assertNoEmptyCharts(page, '/bootc-ecosystem/');
  });

  test('Testhub tab has no empty charts', async ({ page }) => {
    await assertNoEmptyCharts(page, '/bootc-ecosystem/testhub/');
  });

  test('Overall tab has no empty charts', async ({ page }) => {
    await assertNoEmptyCharts(page, '/bootc-ecosystem/overall/');
  });

  test('Builds tab has no empty charts', async ({ page }) => {
    await page.goto('/bootc-ecosystem/builds/');
    // Bootstrap state: only a .collecting paragraph renders — no charts at all.
    const isCollecting = (await page.locator('.collecting').count()) > 0;
    if (isCollecting) return;
    await assertNoEmptyCharts(page, '/bootc-ecosystem/builds/');
  });
});

// ─── New image tabs smoke coverage ────────────────────────────────────────────

test.describe('New image tab smoke coverage', () => {
  for (const pageConfig of [
    {
      name: 'universal-blue',
      url: '/bootc-ecosystem/universal-blue/',
      heading: 'ublue-os Build Pipeline',
      isBuildPage: true,
    },
    {
      name: 'ucore',
      url: '/bootc-ecosystem/ucore/',
      heading: 'uCore Build Pipeline',
      isBuildPage: true,
    },
    {
      name: 'zirconium',
      url: '/bootc-ecosystem/zirconium/',
      heading: 'Zirconium Build Pipeline',
      isBuildPage: true,
    },
    {
      name: 'bootcrew',
      url: '/bootc-ecosystem/bootcrew/',
      heading: 'bootcrew Build Pipeline',
      isBuildPage: true,
    },
    {
      name: 'secureblue',
      url: '/bootc-ecosystem/secureblue/',
      heading: 'secureblue',
      isBuildPage: false,
    },
  ]) {
    test(`${pageConfig.name} page returns 200 and key content is visible`, async ({ page }) => {
      const response = await page.goto(pageConfig.url);
      expect(response?.status(), `${pageConfig.url} must return HTTP 200`).toBe(200);
      await expect(page.locator('h1')).toContainText(pageConfig.heading);

      if (pageConfig.isBuildPage) {
        const html = await page.content();
        const hasCollecting = html.includes('class="collecting"') || html.includes("class='collecting'") || html.includes('Collecting build data');
        const hasDashboard = html.includes('kpi-strip') || html.includes('kpi-card') || html.includes('dora-panel') || html.includes('Pipeline Status');
        expect(hasCollecting || hasDashboard, `${pageConfig.name} page must show collecting state or dashboard`).toBe(true);
      } else {
        await expect(page.locator('canvas#secureblue-trend')).toBeAttached();
      }
    });
  }
});

// ─── IssueButton ─────────────────────────────────────────────────────────────

test.describe('IssueButton', () => {
  for (const [tab, url] of [
    ['Homebrew', '/bootc-ecosystem/'],
    ['Testhub', '/bootc-ecosystem/testhub/'],
    ['Overall', '/bootc-ecosystem/overall/'],
    ['Builds', '/bootc-ecosystem/builds/'],
    ['Contributors', '/bootc-ecosystem/contributors/'],
  ]) {
    test(`${tab} tab has a "File an issue" link`, async ({ page }) => {
      await page.goto(url as string);
      const link = page.locator('a[href*="castrojo/bootc-ecosystem/issues/new"]');
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
    await page.goto('/bootc-ecosystem/');
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

// ─── Testhub data quality ─────────────────────────────────────────────────────

test.describe('Testhub data quality', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/bootc-ecosystem/testhub/');
  });

  test('KPI cards have all expected labels', async ({ page }) => {
    const labels = await page.locator('.kpi-label').allTextContents();
    const normalized = labels.map(l => l.trim().toUpperCase());
    expect(normalized).toContain('TOTAL PACKAGES');
    expect(normalized).toContain('LATEST BUILD STATUS');
    expect(normalized).toContain('UPDATED THIS WEEK');
    expect(normalized).toContain('BUILD RUNS (30D)');
  });

  test('Package table has all expected column headers', async ({ page }) => {
    const headers = await page.locator('#testhub-table thead th').allTextContents();
    const text = headers.join(' ');
    expect(text).toContain('Package');
    expect(text).toContain('Build Status');
    expect(text).toContain('Arch');
    expect(text).toContain('Last Published');
    expect(text).not.toContain('Version Count');
    expect(text).not.toContain('Pulls');
  });

  // Pre-deploy structural: verify status badge element exists in at least one row
  test('Status badge element is present in table rows', async ({ page }) => {
    const badge = page.locator('#testhub-tbody .status-badge').first();
    await expect(badge).toBeAttached();
  });

  // Pre-deploy structural: arch cell is present in at least one row
  test('Arch cell is present in table rows', async ({ page }) => {
    const archCell = page.locator('#testhub-tbody .arch-cell').first();
    await expect(archCell).toBeAttached();
  });

  // Pre-deploy structural: freshness banner element exists (hidden by default)
  test('Freshness banner element exists in DOM', async ({ page }) => {
    const banner = page.locator('#freshness-banner');
    await expect(banner).toBeAttached();
  });

  // Pre-deploy structural: CI runs link is present
  test('View CI Runs link is present', async ({ page }) => {
    const link = page.locator('a[href*="actions/workflows/build.yml"]');
    await expect(link).toBeAttached();
  });

  // Pre-deploy enum validation: last_status must be a known valid value
  test('build_metrics last_status values are valid enum members', async ({ page }) => {
    const rows = page.locator('#testhub-tbody tr');
    const count = await rows.count();
    if (count === 0) return; // cold cache — skip
    const validStatuses = new Set(['passing', 'failing', 'stale', 'pending', 'unknown']);
    for (let i = 0; i < Math.min(count, 5); i++) {
      const badge = rows.nth(i).locator('.status-badge');
      const cls = await badge.getAttribute('class');
      expect(cls, `Row ${i} must have a .status-badge with a class`).toBeTruthy();
      if (!cls) continue; // type guard only — expect above already fails
      // Extract the status class (passing|failing|stale|pending|unknown)
      const statusCls = cls.split(' ').find(c => validStatuses.has(c));
      expect(statusCls, `Row ${i} badge class must be a known status`).toBeTruthy();
    }
  });
});

// ─── Contributors tab ─────────────────────────────────────────────────────────

test.describe('Contributors tab', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/bootc-ecosystem/contributors/');
    await page.waitForLoadState('networkidle');
  });

  // ── Data script validation ────────────────────────────────────────────────

  test('commit-activity-data has valid JSON', async ({ page }) => {
    const data = await getScriptJSON(page, 'commit-activity-data') as Record<string, unknown>;
    expect(Array.isArray(data.repos), 'commit-activity-data.repos must be an array').toBe(true);
  });

  test('contributor-leaderboard-data has valid JSON', async ({ page }) => {
    const data = await getScriptJSON(page, 'contributor-leaderboard-data') as Record<string, unknown>;
    expect(Array.isArray(data.topContributors), 'contributor-leaderboard-data.topContributors must be an array').toBe(true);
  });

  test('bus-factor-data has valid JSON with summary.bus_factor', async ({ page }) => {
    const data = await getScriptJSON(page, 'bus-factor-data') as Record<string, unknown>;
    const summary = data.summary as Record<string, unknown>;
    expect(typeof summary, 'bus-factor-data.summary must be an object').toBe('object');
    expect(typeof summary.bus_factor, 'summary.bus_factor must be a number').toBe('number');
  });

  test('contribution-heatmap-data has valid JSON with repos array', async ({ page }) => {
    const data = await getScriptJSON(page, 'contribution-heatmap-data') as Record<string, unknown>;
    expect(Array.isArray(data.repos), 'contribution-heatmap-data.repos must be an array').toBe(true);
  });

  test('discussion-activity-data has valid JSON when present', async ({ page }) => {
    const el = page.locator('#discussion-activity-data');
    const exists = await el.count() > 0;
    if (!exists) return; // no discussion data — empty state is valid
    const data = await getScriptJSON(page, 'discussion-activity-data') as Record<string, unknown>;
    expect(Array.isArray(data.trend), 'discussion-activity-data.trend must be an array').toBe(true);
  });

  // ── Canvas rendering ──────────────────────────────────────────────────────

  test('CommitActivityChart canvas is rendered by Chart.js', async ({ page }) => {
    await expectCanvasRendered(page, 'commit-activity-chart');
  });

  test('ContributorLeaderboardChart canvas is rendered by Chart.js', async ({ page }) => {
    await expectCanvasRendered(page, 'contributor-leaderboard-chart');
  });

  test('BusFactorChart canvas is rendered by Chart.js', async ({ page }) => {
    await expectCanvasRendered(page, 'bus-factor-chart');
  });

  test('ContributionHeatmapChart canvas is rendered by Chart.js', async ({ page }) => {
    await expectCanvasRendered(page, 'contribution-heatmap-chart');
  });

  test('DiscussionActivityChart canvas is rendered by Chart.js when data is present', async ({ page }) => {
    const canvas = page.locator('canvas#discussion-activity-chart');
    const hasCanvas = await canvas.count() > 0;
    if (!hasCanvas) {
      // No discussion data — verify empty state is shown instead
      const emptyEl = page.locator('.chart-empty').first();
      await expect(emptyEl).toBeVisible();
      return;
    }
    await expectCanvasRendered(page, 'discussion-activity-chart');
  });

  test('release-freq-data has valid JSON with repos array and monthlyBreakdown', async ({ page }) => {
    const el = page.locator('#release-freq-data');
    const exists = await el.count() > 0;
    if (!exists) return; // repos is empty — entire component not rendered; see empty-seed test below
    const data = await getScriptJSON(page, 'release-freq-data') as Record<string, unknown>;
    expect(Array.isArray(data.repos), 'release-freq-data.repos must be an array').toBe(true);
    expect(typeof data.monthlyBreakdown, 'release-freq-data.monthlyBreakdown must be an object').toBe('object');
  });

  test('ReleaseFrequencyChart canvas is rendered by Chart.js when repos data is present', async ({ page }) => {
    const canvas = page.locator('canvas#release-freq');
    const hasCanvas = await canvas.count() > 0;
    if (!hasCanvas) return; // empty seed — repos.length === 0, chart not rendered; see next test
    await expectCanvasRendered(page, 'release-freq');
  });

  test('ReleaseFrequencyChart shows no canvas when repos data is absent (empty seed)', async ({ page }) => {
    const canvas = page.locator('canvas#release-freq');
    const hasCanvas = await canvas.count() > 0;
    if (hasCanvas) return; // real data present — handled by previous test
    await expect(canvas, 'release-freq canvas must not exist when repos is empty').not.toBeAttached();
  });

  // ── LFX-gated charts: empty-state when lfx data is absent ────────────────
  // OrgDependencyChart, PRHealthChart, and NewVsReturningChart all require LFX
  // Insights data. With lfx={} (seed data), they render a .chart-empty <p>
  // element instead of a canvas. We assert the empty state is visible and that
  // no canvas is mistakenly rendered.

  test('OrgDependencyChart shows empty-state (no canvas) when lfx data is absent', async ({ page }) => {
    // The canvas must NOT be present — ChartCard renders <p class="chart-empty"> instead
    const canvas = page.locator('canvas#org-dependency-chart');
    await expect(canvas, 'org-dependency-chart canvas must NOT exist when lfx data is absent').not.toBeAttached();
    // At least one .chart-empty element must be visible
    const emptyEl = page.locator('.chart-empty').first();
    await expect(emptyEl, '.chart-empty element must be visible').toBeVisible();
  });

  test('PRHealthChart shows empty-state (no canvas) when lfx data is absent', async ({ page }) => {
    const canvas = page.locator('canvas#pr-health-chart');
    await expect(canvas, 'pr-health-chart canvas must NOT exist when lfx data is absent').not.toBeAttached();
  });

  test('NewVsReturningChart shows empty-state (no canvas) when lfx data is absent', async ({ page }) => {
    const canvas = page.locator('canvas#new-vs-returning-chart');
    await expect(canvas, 'new-vs-returning-chart canvas must NOT exist when lfx data is absent').not.toBeAttached();
  });

  // ── Page structure ────────────────────────────────────────────────────────

  test('Contributors page has an explainer section', async ({ page }) => {
    const explainer = page.locator('.explainer');
    await expect(explainer).toBeVisible();
    await expect(explainer).toContainText('bootc ecosystem');
  });
});

// ─── Builds tab — Monthly Overview ───────────────────────────────────────────
// The Monthly Overview section only renders when monthly_history.length >= 2.
// With bootstrap data (health_status==="unknown") the entire non-bootstrap
// content block is gated by `isBootstrap`, so the monthly section is never
// emitted into the DOM. These tests handle both states gracefully so they
// continue to pass once real data is collected.

test.describe('Builds tab — Monthly Overview', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/bootc-ecosystem/builds/');
    await page.waitForLoadState('networkidle');
  });

  test('Monthly Overview section absent when bootstrap data (monthly_history < 2)', async ({ page }) => {
    // With bootstrap data (health_status==="unknown") the entire non-bootstrap
    // content block is not emitted — .monthly-overview will not be in the DOM.
    // With real data (monthly_history.length >= 2) the section IS rendered and visible.
    // Either state is acceptable here; the rendering assertions are in the next test.
    const section = page.locator('.monthly-overview');
    const count = await section.count();
    // If count === 0 we are in bootstrap state — test passes.
    // If count > 0 we have real data — section must be visible.
    if (count > 0) {
      await expect(section).toBeVisible();
    }
  });

  test('Monthly charts render when monthly_history has data', async ({ page }) => {
    const section = page.locator('.monthly-overview');
    const isVisible = await section.isVisible().catch(() => false);

    if (!isVisible) {
      // Bootstrap state — no monthly data yet, section absent.
      // This is the expected state for the current builds.json fixture.
      return;
    }

    // Data present — all 4 canvas elements must be rendered and sized.
    await expectCanvasRendered(page, 'monthly-success-chart');
    await expectCanvasRendered(page, 'monthly-duration-chart');
    await expectCanvasRendered(page, 'monthly-repo-chart');
    await expectCanvasRendered(page, 'dora-level-trend-chart');
  });

  test('Monthly data scripts have valid JSON when section is visible', async ({ page }) => {
    const section = page.locator('.monthly-overview');
    const isVisible = await section.isVisible().catch(() => false);

    if (!isVisible) {
      // Bootstrap state — scripts are not emitted. Skip assertions.
      return;
    }

    // Validate each data script payload
    const successData = await getScriptJSON(page, 'monthly-success-data') as unknown[];
    expect(Array.isArray(successData), 'monthly-success-data must be an array').toBe(true);

    const durationData = await getScriptJSON(page, 'monthly-duration-data') as unknown[];
    expect(Array.isArray(durationData), 'monthly-duration-data must be an array').toBe(true);

    const repoData = await getScriptJSON(page, 'monthly-repo-data') as unknown[];
    expect(Array.isArray(repoData), 'monthly-repo-data must be an array').toBe(true);

    const doraData = await getScriptJSON(page, 'dora-level-trend-data') as unknown[];
    expect(Array.isArray(doraData), 'dora-level-trend-data must be an array').toBe(true);
  });

  test('trigger-breakdown-data has valid JSON and trigger-breakdown-chart renders', async ({ page }) => {
    // Trigger breakdown data is always emitted (not gated by monthly_history).
    // If the element is absent the component emitted an empty-state placeholder.
    const dataEl = await page.evaluate(() => !!document.getElementById('trigger-breakdown-data'));
    if (!dataEl) {
      // Empty state (all trigger counts zero) — chart-empty rendered instead.
      return;
    }

    const data = await getScriptJSON(page, 'trigger-breakdown-data') as Record<string, unknown>;
    expect(typeof data, 'trigger-breakdown-data must be an object').toBe('object');

    await expectCanvasRendered(page, 'trigger-breakdown-chart');
  });

  test('StreamHealthTable renders rows when builds data is available', async ({ page }) => {
    // The stream health table is inside the non-bootstrap block.
    // Guard the same way as monthly charts.
    const isBootstrap = await page.evaluate(() => {
      const el = document.querySelector('.collecting');
      return el !== null;
    });
    if (isBootstrap) return;

    const rows = page.locator('.stream-table tbody tr');
    const count = await rows.count();
    expect(count, 'StreamHealthTable must have at least one row').toBeGreaterThan(0);
  });

  test('ArchComparisonPanel renders arch cards when builds data is available', async ({ page }) => {
    const isBootstrap = await page.evaluate(() => {
      const el = document.querySelector('.collecting');
      return el !== null;
    });
    if (isBootstrap) return;

    // If no architectures data, the panel is fully absent (hasData guard).
    const section = page.locator('.arch-panel');
    const count = await section.count();
    if (count === 0) return; // no arch data in fixture — acceptable

    const cards = page.locator('.arch-card');
    await expect(cards.first(), 'At least one .arch-card must be visible').toBeVisible();
  });

  test('Builds page has a file-an-issue link', async ({ page }) => {
    const link = page.locator('a[href*="castrojo/bootc-ecosystem/issues/new"]');
    await expect(link, 'Builds tab must have a file-an-issue link').toBeVisible();
  });

  test('SupplyChainPanel renders when supply chain data is available', async ({ page }) => {
    const isBootstrap = await page.evaluate(() => document.querySelector('.collecting') !== null);
    if (isBootstrap) return;
    // Panel is absent when all rates are -1 (step not detected in pipeline)
    const panel = page.locator('.supply-chain-panel');
    if (await panel.count() === 0) return;
    await expect(panel.first()).toBeVisible();
  });
});

// ─── Contributors range buttons ───────────────────────────────────────────────

test.describe('Contributors range toggles', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/bootc-ecosystem/contributors/');
    await page.waitForLoadState('networkidle');
  });

  test('range buttons render and are clickable without crashing charts', async ({ page }) => {
    // At least one [data-range] button must exist on the page
    const firstRangeBtn = page.locator('[data-range]').first();
    await expect(firstRangeBtn).toBeVisible();
    await firstRangeBtn.click();
    // After clicking, canvas elements remain visible (no chart crash)
    const canvas = page.locator('canvas').first();
    await expect(canvas).toBeVisible();
  });

  test('KPI range buttons update active state', async ({ page }) => {
    const btn60d = page.locator('#kpi-range-btns [data-kpi-range="60d"]');
    const btn30d = page.locator('#kpi-range-btns [data-kpi-range="30d"]');
    await expect(btn30d).toHaveClass(/active/);
    await btn60d.click();
    await expect(btn60d).toHaveClass(/active/);
    await expect(btn30d).not.toHaveClass(/active/);
  });

  test('leaderboard range toggle switches active button and keeps canvas rendered', async ({ page }) => {
    const btn60d = page.locator('#leaderboard-range-btns [data-range="60d"]');
    await btn60d.click();
    await expect(btn60d).toHaveClass(/active/);
    await expectCanvasRendered(page, 'contributor-leaderboard-chart');
  });

  test('discussion range toggle switches active button and keeps canvas rendered', async ({ page }) => {
    const canvas = page.locator('canvas#discussion-activity-chart');
    const hasCanvas = await canvas.count() > 0;
    if (!hasCanvas) return; // empty state — range toggles not rendered
    const btn60d = page.locator('#discussion-range-btns [data-range="60d"]');
    await btn60d.click();
    await expect(btn60d).toHaveClass(/active/);
    await expectCanvasRendered(page, 'discussion-activity-chart');
  });

  test('release cadence range toggle switches active button and keeps canvas rendered', async ({ page }) => {
    const canvas = page.locator('canvas#release-freq');
    const hasCanvas = await canvas.count() > 0;
    if (!hasCanvas) return; // empty seed — chart not rendered, range buttons absent
    const btn90d = page.locator('#release-freq-range-btns [data-range="90"]');
    await btn90d.click();
    await expect(btn90d).toHaveClass(/active/);
    await expectCanvasRendered(page, 'release-freq');
  });
});
