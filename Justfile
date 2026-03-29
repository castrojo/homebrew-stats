set shell := ["bash", "-euo", "pipefail", "-c"]

default:
    just --list

# Fetch CI/CD build metrics for all images from GitHub Actions API
sync-builds:
    cd stats-go && go build -o stats ./cmd/stats/
    GITHUB_TOKEN="${GITHUB_TOKEN:-$GITHUB_PAT}" ./stats-go/stats fetch-builds-bluefin
    GITHUB_TOKEN="${GITHUB_TOKEN:-$GITHUB_PAT}" ./stats-go/stats fetch-builds-aurora
    GITHUB_TOKEN="${GITHUB_TOKEN:-$GITHUB_PAT}" ./stats-go/stats fetch-builds-bazzite
    GITHUB_TOKEN="${GITHUB_TOKEN:-$GITHUB_PAT}" ./stats-go/stats fetch-builds-universal-blue
    GITHUB_TOKEN="${GITHUB_TOKEN:-$GITHUB_PAT}" ./stats-go/stats fetch-builds-ucore
    GITHUB_TOKEN="${GITHUB_TOKEN:-$GITHUB_PAT}" ./stats-go/stats fetch-builds-zirconium
    GITHUB_TOKEN="${GITHUB_TOKEN:-$GITHUB_PAT}" ./stats-go/stats fetch-builds-bootcrew

# Fetch OpenSSF Scorecard scores for tracked repos
sync-scorecard:
    cd stats-go && go run ./cmd/stats fetch-scorecard

# Fetch latest data from GitHub API (requires GITHUB_TOKEN)
sync:
    cd stats-go && go build -o stats ./cmd/stats/
    GITHUB_TOKEN="${GITHUB_TOKEN:-$GITHUB_PAT}" ./stats-go/stats
    just sync-builds

# Astro hot-reload dev server (uses existing synced data)
dev:
    npm run dev

# Fetch data then start dev server
sync-dev:
    just sync
    just dev

# Build the static site (uses existing synced data)
build:
    npm run build

# Fetch data and build
sync-build:
    just sync
    just build

# Build container and serve locally
serve:
    just container-build
    podman rm -f homebrew-stats 2>/dev/null || true
    podman run -d --name homebrew-stats -p 8080:8080 ghcr.io/castrojo/homebrew-stats:local
    xdg-open http://localhost:8080/homebrew-stats/ || true
    echo "Running at http://localhost:8080/homebrew-stats/ — use 'just stop' to kill"

# Build the container image locally
container-build:
    podman build -t ghcr.io/castrojo/homebrew-stats:local -f Containerfile --build-arg SKIP_GO_SYNC=true .

# Stop the running container
stop:
    podman rm -f homebrew-stats 2>/dev/null || true

# Run unit tests (Go backend + TypeScript frontend)
test:
    cd stats-go && go test -v ./...
    npm run test

# Install npm dependencies
install:
    npm install

# Run Playwright E2E browser tests (requires built site)
test-e2e: build
    npm run test:e2e

# Run all tests: unit + E2E
test-all: test test-e2e

# Verify the live GitHub Pages site is healthy
verify-live:
    #!/usr/bin/env bash
    set -euo pipefail
    BASE="https://castrojo.github.io/homebrew-stats"
    echo "=== Verifying live site: $BASE ==="

    for path in "/" "/testhub/" "/overall/" "/contributors/" "/builds/" "/aurora-builds/" "/bazzite-builds/" "/universal-blue/" "/ucore/" "/zirconium/" "/bootcrew/"; do
      code=$(curl -sf -o /dev/null -w "%{http_code}" "$BASE$path" || echo "000")
      if [ "$code" = "200" ]; then
        echo "✅ $BASE$path → HTTP $code"
      else
        echo "❌ $BASE$path → HTTP $code"
        exit 1
      fi
    done

    echo "--- Checking canvas IDs ---"
    html=$(curl -sf "$BASE/")
    for id in traffic-chart tap-comparison-chart os-bar-chart; do
      if echo "$html" | grep -q "id=\"$id\""; then
        echo "✅ canvas#$id found"
      else
        echo "❌ canvas#$id MISSING"
        exit 1
      fi
    done

    echo "--- Checking no chart-empty ---"
    if echo "$html" | grep -q 'class="chart-empty"'; then
      echo "❌ chart-empty element found on homepage"
      exit 1
    fi
    echo "✅ No chart-empty on homepage"

    echo "--- Checking freshness (meta.json) ---"
    today=$(date -u +%Y-%m-%d)
    gen=$(curl -sf "$BASE/meta.json" | python3 -c "import sys,json; print(json.load(sys.stdin).get('generated_at',''))" 2>/dev/null || echo "")
    if [ "$gen" = "$today" ]; then
      echo "✅ meta.json generated_at=$gen (fresh)"
    else
      echo "⚠️  meta.json generated_at=${gen:-<missing>} (expected $today) — site may be stale or meta.json not yet deployed"
    fi

    echo "--- Checking contributors data ---"
    contrib_html=$(curl -sf "$BASE/contributors/" 2>/dev/null || echo "")
    # contributors.json is not a static asset served by the site — skip remote check
    # Instead just verify the contributors page loaded correctly
    if echo "$contrib_html" | grep -q 'commit-activity-chart'; then
      echo "✅ Contributors page has commit-activity-chart canvas"
    else
      echo "⚠️  Contributors page missing commit-activity-chart canvas (may be empty state)"
    fi

    echo "=== Live site verification PASSED ==="

