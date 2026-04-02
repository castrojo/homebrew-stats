# homebrew-stats — Agent Context

## gh-first Policy (Hard Rule)

**All GitHub API interactions MUST use the `gh` CLI. No exceptions.**

- ✅ `gh api repos/OWNER/REPO/...` — REST API
- ✅ `gh api graphql -f query=...` — GraphQL
- ✅ `gh run list / gh run view` — Actions runs
- ✅ `gh attestation verify IMAGE --owner ORG` — SLSA provenance
- ❌ No `go-github` library
- ❌ No `golang.org/x/oauth2`
- ❌ No raw `net/http` calls to `api.github.com` or `ghcr.io`
- ❌ No PATs — `GITHUB_TOKEN` only (set automatically in CI, `gh` reads it)

In Go, exec `gh` via `internal/ghcli.Run(args...)`.
In shell/GHA, call `gh` directly.

## External APIs

Non-GitHub APIs (Homebrew analytics, Fedora countme, securityscorecards.dev) use plain `net/http` — that's fine since `gh` is GitHub-specific.

## Local Dev

```bash
export GITHUB_TOKEN=$(gh auth token)
cd stats-go && go run ./cmd/stats fetch-homebrew
```

## CI

All workflows use `secrets.GITHUB_TOKEN`. The Go binary inherits it via the environment.
