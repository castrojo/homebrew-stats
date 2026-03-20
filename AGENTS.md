# homebrew-stats — Agent / Contributor Guide

Quick orientation for AI coding agents and new contributors.

---

## Repository layout

```
homebrew-stats/
├── stats-go/              # Go data-collection backend
│   ├── cmd/stats/         # CLI entry point (main.go) — subcommand dispatch
│   └── internal/
│       ├── countme/       # Universal Blue countme badge + CSV fetcher
│       ├── github/        # GitHub REST API client (traffic data)
│       ├── history/       # Snapshot persistence (.sync-cache/)
│       ├── metrics/       # Pure stats computation (see below)
│       ├── osanalytics/   # OS-breakdown analytics
│       ├── tap/           # Homebrew tap scraping
│       ├── tapanalytics/  # Per-tap aggregation
│       └── testhub/       # projectbluefin/testhub container package stats
├── src/                   # Astro + Chart.js frontend (3 tabs)
│   ├── components/        # Astro components (charts, KPI widgets, TabNav)
│   ├── layouts/           # Layout.astro — shared layout with TabNav
│   ├── lib/               # Pure TS utility functions + unit tests
│   ├── pages/
│   │   ├── index.astro    # 🍺 Homebrew tab
│   │   ├── testhub/       # 🧪 Testhub tab
│   │   └── overall/       # 🌐 Overall tab
│   └── data/
│       ├── stats.json     # Homebrew tap data — do NOT edit by hand
│       ├── testhub.json   # Testhub package/build data — do NOT edit by hand
│       └── countme.json   # Universal Blue active user data — do NOT edit by hand
├── .sync-cache/           # Persistent GitHub Actions cache (history stores)
│   ├── history.json           # Homebrew tap history
│   ├── testhub-history.json   # Testhub snapshot history
│   └── countme-history.json   # Countme week/day record history
├── .github/workflows/     # CI/CD (see daily-build.yml)
└── public/                # Static assets
```

---

## stats-go subcommands

The `stats` binary is built from `stats-go/cmd/stats/`. It dispatches on `os.Args[1]`:

| Subcommand | Writes | Source |
|---|---|---|
| `stats fetch-homebrew` | `src/data/stats.json` + `.sync-cache/history.json` | GitHub API (tap traffic) |
| `stats fetch-testhub` | `src/data/testhub.json` + `.sync-cache/testhub-history.json` | GitHub Packages + Actions API |
| `stats fetch-countme` | `src/data/countme.json` + `.sync-cache/countme-history.json` | ublue-os/countme CSV + badges |

No-arg default = `fetch-homebrew` (backward compat for `just sync`).

---

## stats-go/internal/metrics — pure metrics package

All functions are **pure** (no I/O, no global state). Safe to test in isolation.

### Key types

```go
// Summary — aggregate KPIs across all tracked taps
type Summary struct {
    TotalInstalls30d       int64
    TotalUniqueTappers     int      // sum of traffic.Uniques per tap
    TotalPackages          int
    StaleCount             int
    FreshCount             int
    UnknownFreshnessCount  int
    WoWGrowthPct           *float64 // nil when insufficient history
}

// TopPackage — one entry in the top-10 leaderboard
type TopPackage struct {
    Name    string
    Tap     string
    History []PackageHistoryPoint
}

type PackageHistoryPoint struct {
    Date      string
    Downloads int64
}
```

### Key functions

| Function | Description |
|---|---|
| `Velocity7d(history, tapName, pkgName)` | Average daily install momentum over the trailing 7 days. Returns 0 when `< 8` qualifying snapshots exist. Negative deltas are clamped to 0. |
| `GrowthPct(history, tapName)` | Week-over-week % change in total tap downloads (`*float64`). Returns `nil` on insufficient history (< 8 snapshots) or zero denominator. |
| `ComputeSummary(taps, history)` | Builds the `Summary` struct from tap stats and snapshot history. |
| `ComputeTopPackages(taps, history)` | Returns the top 10 packages ranked by current download count, with full history series attached. |

### Edge-case contracts (enforced by tests)

- `Velocity7d`: fewer than 8 snapshots → `0`; negative rolling delta → clamped to `0`; missing package in a snapshot → that snapshot is skipped
- `GrowthPct`: zero denominator → `nil`; < 8 snapshots → `nil`; negative growth is valid (returns negative float)
- `ComputeTopPackages`: cross-tap ranking; packages missing from some snapshots get `0` for those days

---

## stats.json schema additions (as of 2025)

The Go binary writes `src/data/stats.json`. New top-level fields:

```jsonc
{
  // NEW: aggregate KPIs
  "summary": {
    "total_installs_30d": 123456,
    "total_unique_tappers": 7890,
    "total_packages": 42,
    "stale_count": 3,
    "fresh_count": 39,
    "unknown_freshness_count": 0,
    "wow_growth_pct": 4.2          // null when unavailable
  },

  // NEW: top-10 packages across all taps by current download count
  "top_packages": [
    {
      "tap_name": "ublue-os/homebrew-tap",
      "name": "bluefin-cli",
      "downloads": 9876,
      "history": [{ "date": "2026-03-01", "downloads": 9500 }, ...]
    }
  ],

  // Per-tap data (existing shape, new fields added per package)
  "taps": {
    "ublue-os/homebrew-tap": {
      "growth_pct": 4.2,           // NEW: week-over-week % change, null when unavailable
      "packages": [
        {
          "name": "bluefin-cli",
          "velocity7d": 12.5,      // NEW: avg daily installs over trailing 7 days
          ...
        }
      ]
    }
  }
}
```

---

## CI — daily-build.yml

The build runs daily at 06:00 UTC:

1. **Build stats binary** — `go build ./cmd/stats/`
2. **`stats fetch-homebrew`** — writes `stats.json`; falls back to cached `stats-latest.json` on failure
3. **`stats fetch-testhub`** — writes `testhub.json`; `continue-on-error: true` (stub data used on failure)
4. **`stats fetch-countme`** — writes `countme.json`; `continue-on-error: true` (stub data used on failure)
5. **Build Astro site** — `npm run build` (3 pages: `/`, `/testhub/`, `/overall/`)
6. **Verify charts have data** — fails if `chart-empty` appears in `dist/index.html`
7. **Verify summary KPIs** — asserts `summary.total_packages > 0`
8. **Verify testhub/countme data** — warn-only (new data sources may be empty initially)
9. **Deploy to GitHub Pages**

### Running locally

```bash
# Go backend
cd stats-go
go build -o stats ./cmd/stats/
go test ./...

# Sync each data source
GITHUB_TOKEN=... ./stats-go/stats fetch-homebrew
GITHUB_TOKEN=... ./stats-go/stats fetch-testhub
GITHUB_TOKEN=... ./stats-go/stats fetch-countme

# Frontend
npm ci
npm run build
npm test
```

---

## Testing conventions

- **Go**: table-driven tests in `*_test.go` next to the package under test. Run `go test ./...` from `stats-go/`.
- **TypeScript**: vitest unit tests in `src/lib/*.test.ts` covering pure chart/package utility functions. Run `npx vitest run` from the repo root.
- **Do not add `console.log` debugging** to committed code.
- **Do not push** from automated agents — commit only.

---

## MANDATORY PATTERNS — Chart Components

**Read this before touching any `.astro` file with a `<script>` block.**

### Data injection into scripts

```astro
---
import { safeJson } from '../lib/inject.ts';
---
<script type="application/json" id="my-data" set:html={safeJson(data)}></script>
```

| Rule | Why |
|---|---|
| **Always `set:html`** (never `set:text`) | `set:text` HTML-encodes `"` → `&quot;`; browsers don't decode entities in `<script>` raw text |
| **Always `safeJson()`** (never raw `JSON.stringify`) | `JSON.stringify` doesn't escape `<`, so `</script>` in a data value breaks HTML parsing |
| **Never `define:vars`** for chart data | Only works with `is:inline` scripts; chart components use ES module imports |

Read data in the `<script>` block:
```ts
import { readChartData } from '../lib/inject.js';
const data = readChartData<MyType>('my-data');
```

### Chart.js imports

```ts
// ✅ CORRECT — tree-shaken, registers Line/Bar/Doughnut/Category/Linear/Tooltip/Legend
import '../lib/chart-registry.js';
import { Chart } from 'chart.js';

// ❌ NEVER — imports 200KB kitchen-sink bundle
import Chart from 'chart.js/auto';
```

### Shared utilities (never re-declare locally)

```ts
import { BRAND_COLOURS, getCSSVar, getChartColors, getChartDefaults, applyTheme } from '../lib/chart-theme.js';
```

| Export | Purpose |
|---|---|
| `BRAND_COLOURS` | 10-colour brand palette (consistent ordering across all charts) |
| `getCSSVar(name)` | Read a CSS custom property from the document root |
| `getChartColors()` | Returns `{ text, muted, grid }` from current CSS vars |
| `getChartDefaults(colors)` | Chart.js scale/plugin defaults object |
| `applyTheme(chart)` | Update chart colors on `themechange` without destroying |

### Card wrapper

```astro
import ChartCard from './ChartCard.astro';

<ChartCard
  title="My Chart"
  canvasId="my-canvas"
  height={260}
  rangeId="my-range-btns"
  ranges={[{ label: '30d', value: '30' }, { label: 'All', value: 'all' }]}
  defaultRange="30"
/>
```

`ChartCard` provides: card shell, `.chart-card`/`.chart-wrap` CSS, range buttons, `role="img"` + `aria-label` on the canvas, and an empty state (`isEmpty` + `emptyMsg` props). **Do NOT duplicate this CSS in component `<style>` blocks.**

### Shared types

```ts
import type { DaySnapshot, TapSnapshot, WeekRecord, AppDayCount, Package } from '../lib/types.ts';
```

### themechange handler (use `chart.update()`, never `chart.destroy()`)

```ts
window.addEventListener('themechange', () => {
  if (!chart) return;
  const c = getChartColors();
  (chart.options.plugins!.legend!.labels as { color: string }).color = c.text;
  chart.options.scales!.x!.ticks!.color = c.muted;
  (chart.options.scales!.x!.grid as { color: string }).color = c.grid;
  chart.options.scales!.y!.ticks!.color = c.muted;
  (chart.options.scales!.y!.grid as { color: string }).color = c.grid;
  chart.update();  // ← chart.update() NOT chart.destroy() + new Chart()
});
```

### Error handling — wrap Chart.js init

```ts
let chart: Chart | undefined;
try {
  chart = new Chart(canvas, { ... });
} catch (e) {
  console.error('[MyChart] Chart.js init failed:', e);
}
```

### ESLint guard

`npm run lint` will error if `set:text=` appears on any `<script>` element (enforced via `eslint-plugin-astro` + `no-restricted-syntax`). Do not disable this rule.

---

## Testing (full suite)

```bash
just test-all    # 79 unit tests (Vitest) + 18 E2E tests (Playwright)
just test        # unit tests only
just test-e2e    # E2E tests only (builds site first, then launches preview server)
npm run typecheck  # TypeScript type check (no emit)
npm run lint       # ESLint (includes .astro files via eslint-plugin-astro)
```

All must pass before merging. CI runs: typecheck → build → verify data → E2E → deploy.

---

## DEFINITION OF DONE — Non-Negotiable

A task is complete only when ALL three layers pass:

### Layer 1 — Local tests
- `just test-all` passes (unit tests + E2E against local preview)
- `npm run lint` passes (no TS errors, no `set:text` violations)
- `actionlint` passes on all `.github/workflows/*.yml` files
- Go: `cd stats-go && go test ./...` passes

### Layer 2 — CI green
- Push to `main` and confirm `gh run list --limit 5` shows ✅ green for:
  - "Build and Deploy to GitHub Pages" workflow
  - "Smoke Test — Live Site" workflow (triggers ~30–60s after deploy)

### Layer 3 — Live site verified
- Run `just verify-live` after CI is green
- This checks the live URL `https://castrojo.github.io/homebrew-stats/`:
  - HTTP 200 on all 3 pages
  - Canvas elements present
  - No `class="chart-empty"` elements
  - `public/meta.json` reflects today's date (freshness check)

**"CI green" is not done. "It should work" is not done. `just verify-live` passing is done.**
