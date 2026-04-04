# bootc-ecosystem

Health dashboard for [ublue-os/homebrew-tap](https://github.com/ublue-os/homebrew-tap) and [ublue-os/homebrew-experimental-tap](https://github.com/ublue-os/homebrew-experimental-tap).

Live site: https://castrojo.github.io/bootc-ecosystem/

## What it tracks

- **Unique tappers** — unique IPs that ran `brew tap ublue-os/tap` in the last 14 days
- **Package health** — version freshness for each cask and formula (current / stale / unknown)
- **Download counts** — total GitHub release asset downloads per package (sourced from the upstream release repo)
- **Historical trends** — unique tappers and download counts over time, accumulated via GitHub Actions cache

## Local development

```bash
# Install npm dependencies
just install

# Fetch latest data from GitHub (requires GITHUB_TOKEN or GITHUB_PAT)
just sync

# Start hot-reload dev server at http://localhost:4324/bootc-ecosystem/
just dev

# Fetch data then start dev server
just sync-dev

# Build static site to dist/ (uses existing synced data)
just build

# Fetch data and build (full local pipeline without container)
just sync-build

# Build container image locally
just container-build

# Build container + run it at http://localhost:8080/bootc-ecosystem/
just serve

# Stop the running container
just stop
```

> **Note:** `just serve` builds and runs a full container image (slow). For UI iteration,
> use `just dev` (fast hot-reload, no container needed).

## Data flow

```
GitHub API
    │  clone traffic   (push access required — see Token requirement)
    │  .rb file contents (public)
    │  latest release tags (public)
    │  release asset download counts (public)
    ▼
stats-go/cmd/stats/main.go
    │  writes
    ▼
src/data/stats.json          ← consumed by Astro at build time
    │
    ▼
Astro static site (src/)
    │  builds to
    ▼
dist/                         → GitHub Pages
    └─────────────────────────→ ghcr.io/castrojo/bootc-ecosystem (Chainguard nginx)
```

## Architecture

```
bootc-ecosystem/
├── stats-go/                   Go CLI — fetches GitHub API data
│   ├── cmd/stats/main.go       Entry point: collect → history → write stats.json
│   ├── internal/github/        GitHub API client (traffic, files, releases, downloads)
│   ├── internal/tap/           Ruby .rb parser, freshness check, download count
│   └── internal/history/       Accumulates daily snapshots in .sync-cache/history.json
│
├── src/                        Astro static site
│   ├── pages/index.astro       Root page
│   ├── components/
│   │   ├── TapSection.astro    Per-tap stat cards + package table
│   │   ├── TrafficChart.astro  Unique tappers over time (Chart.js)
│   │   └── DownloadsChart.astro Per-tap total installs + top-10 package charts
│   └── layouts/Layout.astro   Dark GitHub-style theme, CSS custom properties
│
├── .sync-cache/history.json    Persisted history (via GitHub Actions cache)
├── Containerfile               3-stage Chainguard build (Go → Astro → nginx)
└── .github/workflows/
    ├── daily-build.yml         Sync + Astro build + GitHub Pages deploy (6 AM UTC)
    └── build-container.yml     Sync + container build + push to GHCR (8 AM UTC)
```

## GitHub Actions

### `daily-build.yml` — GitHub Pages

Runs daily at **6 AM UTC** (also on push to main and `workflow_dispatch`):

1. Restore `.sync-cache` from `actions/cache`
2. Build Go binary → run `./stats-go/stats` to fetch live data
3. Save updated `.sync-cache` (90-day retention)
4. `npm ci` + `astro build`
5. Deploy `dist/` to GitHub Pages

### `build-container.yml` — GHCR container

Runs daily at **8 AM UTC** (two hours after Pages, so the cache is warm):

1. Restore `.sync-cache` from `actions/cache`
2. Sync tap data (same as above)
3. Build and push `Containerfile` to `ghcr.io/castrojo/bootc-ecosystem:latest` and `:sha`

### History cache

`actions/cache` persists `.sync-cache/history.json` across daily runs using the key
`tap-history-v1-{run_id}` with restore key `tap-history-v1-`. Each run appends one
`DaySnapshot` (idempotent — duplicate dates are skipped). This builds a time-series
that outlasts the GitHub API's 14-day rolling window.

## Token requirement

Only `GITHUB_TOKEN` is required, and it is automatically provided by GitHub Actions.

| Token | Scope | Purpose |
|---|---|---|
| `GITHUB_TOKEN` (automatic) | repository-scoped | GitHub API access for traffic, package contents, releases, and downloads |

For local development, use your authenticated GitHub CLI token:

```bash
export GITHUB_TOKEN=$(gh auth token)
just sync
```

> **Note:** Clone traffic for `ublue-os/*` taps may be unavailable in some contexts.
> The sync handles this gracefully and continues with remaining data.

## Adding a new tap

Add an entry to the `taps` slice in `stats-go/cmd/stats/main.go`:

```go
var taps = []struct{ owner, repo string }{
    {"ublue-os", "homebrew-tap"},
    {"ublue-os", "homebrew-experimental-tap"},
    {"my-org", "my-new-tap"},  // add here
}
```

The pipeline automatically discovers `Casks/` and `Formula/` directories in the repo.
Freshness checking requires a detectable GitHub URL in the `.rb` file.
