# homebrew-stats

Health dashboard for [ublue-os/homebrew-tap](https://github.com/ublue-os/homebrew-tap) and [ublue-os/homebrew-experimental-tap](https://github.com/ublue-os/homebrew-experimental-tap).

Live site: https://castrojo.github.io/homebrew-stats/

## What it tracks

- **Unique tappers** — unique IPs that ran `brew tap ublue-os/tap` in the last 14 days
- **Package health** — version freshness for each cask and formula
- **Historical trend** — unique tappers over time (accumulated via GitHub Actions cache)

## Local development

```bash
# Install dependencies
just install

# Fetch latest data from GitHub (requires GITHUB_TOKEN or GITHUB_PAT)
just sync

# Start dev server (hot reload, uses existing data)
just dev

# Fetch data then start dev server
just sync-dev

# Build static site
just build

# Build + run as container
just container-run
```

## Architecture

- **`stats-go/`** — Go CLI that fetches tap traffic and package data from GitHub API
- **`src/data/stats.json`** — generated JSON consumed by Astro at build time
- **`.sync-cache/history.json`** — accumulated daily traffic snapshots (persisted via GitHub Actions cache)
- **`src/`** — Astro static site (dark theme, Chart.js trend chart)
- **`Containerfile`** — 3-stage Chainguard build (Go → Astro → nginx)

## GitHub Actions cache

The workflow uses `actions/cache` to persist `.sync-cache/history.json` across daily runs.
This builds up a historical traffic trend that goes beyond the GitHub API's 14-day rolling window.

## Token requirement

The GitHub traffic API requires push access. Set `GH_TRAFFIC_TOKEN` as a repository secret
(a PAT with `repo` scope that has push access to `ublue-os/homebrew-tap` and
`ublue-os/homebrew-experimental-tap`).
