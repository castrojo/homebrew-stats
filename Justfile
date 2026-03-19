set shell := ["bash", "-euo", "pipefail", "-c"]

default:
    just --list

# Fetch latest data from GitHub API (requires GITHUB_TOKEN)
sync:
    cd stats-go && go build -o stats ./cmd/stats/
    GITHUB_TOKEN="${GITHUB_TOKEN:-$GITHUB_PAT}" ./stats-go/stats

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
