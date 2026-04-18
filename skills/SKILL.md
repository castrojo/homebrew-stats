# bootc-ecosystem — Operational Knowledge

## When to Use
Load this skill for any work in `castrojo/bootc-ecosystem` — Go backend, Astro frontend, Chart.js components, CI pipeline, data schemas.

## When NOT to Use
For generic Go, Astro, or Chart.js patterns → their upstream docs. For homebrew tap content → load `~/src/skills/homebrew-taps/SKILL.md`.

---

## Project Decisions (Build Pages)

Use these decisions as defaults for future agents unless superseded by a newer issue.

- Builds navigation/page model:
  - Existing `Builds` UX maps to **Bluefin**.
  - Add dedicated **Aurora** and **Bazzite** build pages instead of combining all images into one page.
- Data pipeline model:
  - Maintain separate build data outputs/history per image to avoid cross-image coupling and cache interference.
  - Do not aggregate mirrored sources that can duplicate runs.
- Aurora source mapping (confirmed):
  - Include `ublue-os/aurora` (image repo), `get-aurora-dev/common`, `get-aurora-dev/iso`.
  - Exclude `get-aurora-dev/aurora-test` to reduce duplicate mirrored-run risk.
- Bazzite source mapping (current):
  - Start with `ublue-os/bazzite`.
  - Add Bazzite-org infrastructure repos only after explicit repo/workflow confirmation.
- Terminology:
  - In UI copy, refer to Aurora/Bazzite/Bluefin as **images**, not distros.
- Issue closure in this repo:
  - Follow `~/src/skills/github-issues/SKILL.md` closure protocol before closing any issue.
  - Do not use "implemented in working tree" as closure evidence.

Tracking issues for this plan:
- Epic: `https://github.com/castrojo/bootc-ecosystem/issues/30`
- Tasks: `https://github.com/castrojo/bootc-ecosystem/issues/31` to `https://github.com/castrojo/bootc-ecosystem/issues/36`

---

## Repository Layout

```
bootc-ecosystem/
├── stats-go/              # Go data-collection backend
│   ├── cmd/stats/         # CLI entry point (main.go) — subcommand dispatch
│   └── internal/
│       ├── brewfile/      # Brewfile fetcher (parses GitHub-hosted Brewfiles)
│       ├── builds/        # GitHub Actions build run collector (per-image)
│       ├── contributors/  # GitHub commits, PRs, issues, discussions APIs
│       ├── countme/       # Universal Blue countme badge + CSV fetcher
│       ├── ghcli/         # gh CLI wrapper (reads GITHUB_TOKEN automatically)
│       ├── github/        # GitHub REST API client (traffic, packages, Actions data)
│       ├── history/       # Snapshot persistence (.sync-cache/)
│       ├── metrics/       # Pure stats computation (no I/O)
│       ├── osanalytics/   # OS-breakdown analytics
│       ├── quay/          # Quay.io public API client (no auth required)
│       ├── scorecard/     # OpenSSF Scorecard public API client (no auth required)
│       ├── supplychain/   # OCI image supply-chain inspection (anonymous pull)
│       ├── tap/           # Homebrew tap scraping
│       ├── tapanalytics/  # Per-tap aggregation
│       └── testhub/       # projectbluefin/testhub container package stats
├── src/                   # Astro + Chart.js frontend (3 tabs)
│   ├── components/        # Astro components (charts, KPI widgets, TabNav)
│   ├── layouts/           # Layout.astro — shared layout with TabNav
│   ├── lib/               # Pure TS utility functions + unit tests
│   ├── pages/
│   │   ├── index.astro    # Homebrew tab
│   │   ├── testhub/       # Testhub tab
│   │   └── overall/       # Overall tab
│   └── data/
│       ├── stats.json     # Homebrew tap data — do NOT edit by hand
│       ├── testhub.json   # Testhub package/build data — do NOT edit by hand
│       └── countme.json   # Universal Blue active user data — do NOT edit by hand
├── .sync-cache/           # Persistent GitHub Actions cache (history stores)
│   ├── history.json           # Homebrew tap history
│   ├── testhub-history.json   # Testhub snapshot history
│   └── countme-history.json   # Countme week/day record history
├── .github/workflows/     # CI/CD (daily-build.yml, smoke-test.yml)
└── public/                # Static assets
```

---

## stats-go Subcommands

The `stats` binary dispatches on `os.Args[1]`. No-arg default = `fetch-homebrew` (backward compat for `just sync`).

| Subcommand | Writes | Source | Caveats |
|---|---|---|---|
| `stats fetch-homebrew` | `src/data/stats.json` + `.sync-cache/history.json` + `.sync-cache/stats-latest.json` | GitHub API (tap traffic, Homebrew analytics) | Requires `GITHUB_TOKEN`; traffic API needs push access — falls back to cached values in CI |
| `stats fetch-brewfile-taps` | `src/data/brewfile-stats.json` | Parses GitHub-hosted Brewfiles (bluefin-common, bazzite) + Homebrew analytics | Requires `GITHUB_TOKEN`; skips ublue-os taps (already tracked by fetch-homebrew) |
| `stats fetch-testhub` | `src/data/testhub.json` + `.sync-cache/testhub-history.json` | GitHub Packages API + Actions API (projectbluefin/testhub) | Requires `GITHUB_TOKEN` with `packages: read` scope; falls back to committed testhub.json on cold start |
| `stats fetch-countme` | `src/data/countme.json` + `.sync-cache/countme-history.json` | Fedora countme CSV (data-analysis.fedoraproject.org) | No GITHUB_TOKEN needed; CSV updates weekly (Sundays); skips re-fetch when current week already cached |
| `stats fetch-releases` | `src/data/releases.json` | GitHub Releases API (Bluefin, Aurora, Bazzite, uCore) | Requires `GITHUB_TOKEN`; tracks 4 repos: `ublue-os/{bluefin,aurora,bazzite,ucore}` |
| `stats fetch-contributors` | `src/data/contributors.json` + `.sync-cache/contributors-history.json` + `.sync-cache/contributor-profiles.json` | GitHub commits, PRs, issues, discussions, participation, punch card APIs | Requires `GITHUB_TOKEN`; fetches 365d data and filters in-memory; profiles are cached to avoid repeated API calls |
| `stats fetch-scorecard` | `src/data/scorecard.json` | OpenSSF Scorecard public API (api.securityscorecards.dev) | No GITHUB_TOKEN needed; some repos not yet indexed → marked as `indexed: false` in output |
| `stats fetch-supply-chain` | `src/data/supply-chain.json` | OCI image manifests + GitHub workflow YAML inspection | OCI metadata pulled **anonymously** (`authn.Anonymous`) — no registry credentials needed; detects cosign, SBOM, Sigstore |
| `stats fetch-builds-bluefin` | `src/data/builds-bluefin.json` + `.sync-cache/builds-bluefin-history.json` | GitHub Actions API (`ublue-os/bluefin` + related repos) | Requires `GITHUB_TOKEN`; 14-day lookback, 30 runs/workflow max |
| `stats fetch-builds-aurora` | `src/data/builds-aurora.json` + `.sync-cache/builds-aurora-history.json` | GitHub Actions API (`ublue-os/aurora`, `get-aurora-dev/{common,iso}`) | Requires `GITHUB_TOKEN`; excludes `get-aurora-dev/aurora-test` to avoid mirrored-run duplication |
| `stats fetch-builds-bazzite` | `src/data/builds-bazzite.json` + `.sync-cache/builds-bazzite-history.json` | GitHub Actions API (`ublue-os/bazzite`) | Requires `GITHUB_TOKEN` |
| `stats fetch-builds-universal-blue` | `src/data/builds-universal-blue.json` + `.sync-cache/builds-universal-blue-history.json` | GitHub Actions API (`ublue-os/main` + related repos) | Requires `GITHUB_TOKEN` |
| `stats fetch-builds-ucore` | `src/data/builds-ucore.json` + `.sync-cache/builds-ucore-history.json` | GitHub Actions API (`ublue-os/ucore`) | Requires `GITHUB_TOKEN` |
| `stats fetch-builds-zirconium` | `src/data/builds-zirconium.json` + `.sync-cache/builds-zirconium-history.json` | GitHub Actions API (`zirconium-dev/zirconium`) | Requires `GITHUB_TOKEN` |
| `stats fetch-builds-bootcrew` | `src/data/builds-bootcrew.json` + `.sync-cache/builds-bootcrew-history.json` | GitHub Actions API (`bootcrew/mono`) | Requires `GITHUB_TOKEN` |
| `stats fetch-builds-blue-build` | `src/data/builds-blue-build.json` + `.sync-cache/builds-blue-build-history.json` | GitHub Actions API (BlueBuild repos) | Requires `GITHUB_TOKEN` |
| `stats fetch-quay-fedora` | `src/data/quay-fedora.json` | Quay.io public API (fedora base images) | No GITHUB_TOKEN needed; Quay.io API is public for tracked repos |
| `stats fetch-quay-centos` | `src/data/quay-centos.json` | Quay.io public API (centos base images) | No GITHUB_TOKEN needed |
| `stats fetch-quay-almalinux` | `src/data/quay-almalinux.json` | Quay.io public API (almalinux base images) | No GITHUB_TOKEN needed |

---

## stats-go/internal/metrics — Pure Metrics Package

All functions are **pure** (no I/O, no global state). Safe to test in isolation.

### Key types

```go
type Summary struct {
    TotalInstalls30d       int64
    TotalUniqueTappers     int      // sum of traffic.Uniques per tap
    TotalPackages          int
    StaleCount             int
    FreshCount             int
    UnknownFreshnessCount  int
    WoWGrowthPct           *float64 // nil when insufficient history
}

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

## stats.json Schema

```jsonc
{
  "summary": {
    "total_installs_30d": 123456,
    "total_unique_tappers": 7890,
    "total_packages": 42,
    "stale_count": 3,
    "fresh_count": 39,
    "unknown_freshness_count": 0,
    "wow_growth_pct": 4.2          // null when unavailable
  },
  "top_packages": [
    {
      "tap_name": "ublue-os/homebrew-tap",
      "name": "bluefin-cli",
      "downloads": 9876,
      "history": [{ "date": "2026-03-01", "downloads": 9500 }]
    }
  ],
  "taps": {
    "ublue-os/homebrew-tap": {
      "growth_pct": 4.2,           // week-over-week % change, null when unavailable
      "packages": [
        { "name": "bluefin-cli", "velocity7d": 12.5 }
      ]
    }
  }
}
```

---

## CI — daily-build.yml

Runs daily at 06:00 UTC:

1. **Build stats binary** — `go build ./cmd/stats/`
2. **`stats fetch-homebrew`** — writes `stats.json`; falls back to cached `stats-latest.json` on failure
3. **`stats fetch-brewfile-taps`** — writes `brewfile-stats.json`; `continue-on-error: true`
4. **`stats fetch-testhub`** — writes `testhub.json`; `continue-on-error: true`
5. **`stats fetch-countme`** — writes `countme.json`; `continue-on-error: true`
6. **`stats fetch-builds-*`** — one step per image (bluefin/aurora/bazzite/universal-blue/ucore/zirconium/bootcrew/blue-build); `continue-on-error: true`
7. **`stats fetch-quay-*`** — fedora/centos/almalinux; `continue-on-error: true`
8. **`stats fetch-scorecard`** — writes `scorecard.json`; `continue-on-error: true`
9. **`stats fetch-supply-chain`** — writes `supply-chain.json`; `continue-on-error: true`
10. **`stats fetch-releases`** — writes `releases.json`; `continue-on-error: true`
11. **Build Astro site** — `npm run build`
12. **Verify charts have data** — fails if `class="chart-empty"` appears in output pages
13. **Verify summary KPIs** — asserts `summary.total_packages > 0`
14. **Run Playwright E2E chart tests** — `npm run test:e2e`
15. **Deploy to GitHub Pages**

> **Note:** `fetch-contributors` does NOT run in `daily-build.yml`. It runs in the separate `.github/workflows/contributor-sync.yml` workflow.

---

## Cache Key Strategy

The `.sync-cache/` directory persists incremental history between daily CI runs via `actions/cache`.

### Current cache key
`tap-history-v2-{run_id}` (restore key: `tap-history-v2-`)

### When to bump the version (v2 → v3, etc.)
Bump in BOTH `key` and `restore-keys` in `daily-build.yml` when:
- Testhub packages show all `⚪ —` (unknown build status) on the live site
- Any structural change to the JSON schema of a history file
- A corrupted cache entry is suspected

### How to detect a stale/corrupted cache
In CI logs, look for:
```
→ Fetching testhub build counts (since run XXXXXXX)…
  build counts: 0 apps, new max run_id=XXXXXXX   ← same ID = no new runs (may be normal)
```
If `new max run_id` never advances across multiple days AND testhub shows all-unknown → bump the cache key.

### ⚠️ After bumping — mandatory warm-up procedure

A cache key bump forces a cold start. **NEVER add new data-quality tests in the same batch as a cache key bump.** That combination caused the 2026-03-21 production outage.

Procedure:
1. Push the bump commit **alone** (no other changes)
2. Run `gh workflow run .github/workflows/daily-build.yml` to warm the cache
3. Confirm CI is green and testhub shows real statuses on the live site
4. **THEN** add new tests that depend on non-empty testhub data

---

## Pre-deploy vs. Post-deploy Test Classification

**Pre-deploy E2E (`charts.spec.ts`)** — tests RENDERING only. Must pass even on cold cache:
- Canvas elements rendered (non-zero bounding box)
- JSON data scripts present and parseable
- Component structure (KPI cards exist, table has correct headers)
- No `class="chart-empty"` on structural charts

**Post-deploy smoke-test (`smoke-test.yml`)** — tests DATA QUALITY. May warn on cold cache:
- At least one testhub package has a known (non-⚪) build status
- KPI numeric values are non-zero
- `meta.json` freshness reflects today's date

**The rule:** If a test can fail due to API data being unavailable or cache being cold, it belongs in `smoke-test.yml`, NOT in the pre-deploy E2E suite.

### Smoke test architecture

The smoke test is a **separate workflow** triggered by `workflow_run`, NOT a job inside `daily-build.yml`. This is intentional — `daily-build.yml` has `concurrency: { group: pages, cancel-in-progress: true }`, so any job inside that workflow would be cancelled by a new push.

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
// ✅ CORRECT — tree-shaken
import '../lib/chart-registry.js';
import { Chart } from 'chart.js';

// ❌ NEVER — 200KB kitchen-sink bundle
import Chart from 'chart.js/auto';
```

### Shared utilities (never re-declare locally)

```ts
import { BRAND_COLOURS, getCSSVar, getChartColors, getChartDefaults, applyTheme } from '../lib/chart-theme.js';
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

### ESLint guard

`npm run lint` will error if `set:text=` appears on any `<script>` element. Do not disable this rule.

---

## Testing

```bash
just test-all    # 79 unit tests (Vitest) + 18 E2E tests (Playwright)
just test        # unit tests only
just test-e2e    # E2E tests only (builds site first, then launches preview server)
npm run typecheck  # TypeScript type check (no emit)
npm run lint       # ESLint (includes .astro files via eslint-plugin-astro)
cd stats-go && go test ./...   # Go unit tests
```

- **Go**: table-driven tests in `*_test.go` next to the package under test.
- **TypeScript**: vitest unit tests in `src/lib/*.test.ts` covering pure chart/package utility functions.
- Do not add `console.log` debugging to committed code.
- Do not push from automated agents — commit only.

---

## Definition of Done — Non-Negotiable

A task is complete only when ALL three layers pass:

### Layer 1 — Local tests
- `just test-all` passes (unit tests + E2E against local preview)
- `npm run lint` passes (no TS errors, no `set:text` violations)
- `actionlint` passes on all `.github/workflows/*.yml` files
- `cd stats-go && go test ./...` passes

### Layer 2 — CI green
- Push to `main` and confirm `gh run list --limit 5` shows green for:
  - "Build and Deploy to GitHub Pages" workflow
  - "Smoke Test — Live Site" workflow (triggers ~30–60s after deploy)

### Layer 3 — Live site verified
- Run `just verify-live` after CI is green
- Checks `https://castrojo.github.io/bootc-ecosystem/`: HTTP 200 on all 3 pages, canvas elements present, no `class="chart-empty"`, `public/meta.json` reflects today's date

**"CI green" is not done. `just verify-live` passing is done.**
