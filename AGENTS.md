# homebrew-stats — Agent / Contributor Guide

Quick orientation for AI coding agents and new contributors.

---

## Repository layout

```
homebrew-stats/
├── stats-go/              # Go data-collection backend
│   ├── cmd/stats/         # CLI entry point (main.go)
│   └── internal/
│       ├── github/        # GitHub REST API client (traffic data)
│       ├── history/       # Snapshot persistence (.sync-cache/)
│       ├── metrics/       # Pure stats computation (see below)
│       ├── osanalytics/   # OS-breakdown analytics
│       ├── tap/           # Homebrew tap scraping
│       └── tapanalytics/  # Per-tap aggregation
├── src/                   # Astro + Chart.js frontend
│   ├── components/        # Astro components (charts, KPI widgets)
│   ├── lib/               # Pure TS utility functions + unit tests
│   └── data/stats.json    # Generated — do NOT edit by hand
├── .github/workflows/     # CI/CD (see daily-build.yml)
└── public/                # Static assets
```

---

## stats-go/internal/metrics — pure metrics package

All functions are **pure** (no I/O, no global state). Safe to test in isolation.

### Key types

```go
// Summary — aggregate KPIs across all tracked taps
type Summary struct {
    TotalInstalls30d       int64
    TotalUniqueTappers     int64
    TotalPackages          int
    StaleCount             int
    FreshCount             int
    UnknownFreshnessCount  int
    WoWGrowthPct           *float64  // nil when insufficient history
}

// TopPackage — one entry in the top-10 leaderboard
type TopPackage struct {
    TapName   string
    Name      string
    Downloads int64
    History   []HistoryPoint
}
```

### Key functions

| Function | Description |
|---|---|
| `Velocity7d(history, tapName, pkgName)` | Average daily install momentum over the trailing 7 days. Returns 0 when `< 8` qualifying snapshots exist. Negative deltas are clamped to 0. |
| `GrowthPct(history, tapName)` | Week-over-week % change in total tap downloads (`*float64`). Returns `nil` on insufficient history (< 8 snapshots) or zero denominator. |
| `ComputeSummary(taps, history)` | Builds the `Summary` struct from tap stats and snapshot history. |
| `ComputeTopPackages(taps, history, n)` | Returns the top `n` packages ranked by current download count, with full history series attached. |

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

1. **Sync tap data** — builds the Go binary and runs it; falls back to cached `stats-latest.json` on failure
2. **Build Astro site** — `npm run build`
3. **Verify charts have data** — fails if any `chart-empty` marker appears in the built HTML
4. **Verify summary KPIs** — asserts `summary.total_packages > 0`; catches Go binary failures that produce empty output
5. **Deploy to GitHub Pages**

### Running locally

```bash
# Go backend
cd stats-go
go build -o stats ./cmd/stats/
go test ./...

# Frontend
npm ci
npm run build
npx vitest run
```

---

## Testing conventions

- **Go**: table-driven tests in `*_test.go` next to the package under test. Run `go test ./...` from `stats-go/`.
- **TypeScript**: vitest unit tests in `src/lib/*.test.ts` covering pure chart/package utility functions. Run `npx vitest run` from the repo root.
- **Do not add `console.log` debugging** to committed code.
- **Do not push** from automated agents — commit only.
