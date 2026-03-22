package main

import (
"encoding/json"
"fmt"
"os"
"path/filepath"
"sort"
"strings"
"time"

ghclient "github.com/castrojo/homebrew-stats/internal/github"
"github.com/castrojo/homebrew-stats/internal/builds"
"github.com/castrojo/homebrew-stats/internal/contributors"
"github.com/castrojo/homebrew-stats/internal/countme"
"github.com/castrojo/homebrew-stats/internal/history"
"github.com/castrojo/homebrew-stats/internal/metrics"
"github.com/castrojo/homebrew-stats/internal/osanalytics"
"github.com/castrojo/homebrew-stats/internal/tap"
"github.com/castrojo/homebrew-stats/internal/tapanalytics"
"github.com/castrojo/homebrew-stats/internal/testhub"
)

func main() {
// Default to fetch-homebrew for backward compatibility with `just sync`.
cmd := "fetch-homebrew"
if len(os.Args) >= 2 {
cmd = os.Args[1]
}
switch cmd {
case "fetch-homebrew":
if err := runFetchHomebrew(); err != nil {
fmt.Fprintln(os.Stderr, "❌", err)
os.Exit(1)
}
case "fetch-testhub":
if err := runFetchTesthub(); err != nil {
fmt.Fprintln(os.Stderr, "❌", err)
os.Exit(1)
}
case "fetch-countme":
if err := runFetchCountme(); err != nil {
fmt.Fprintln(os.Stderr, "❌", err)
os.Exit(1)
}
case "fetch-contributors":
if err := runFetchContributors(); err != nil {
fmt.Fprintln(os.Stderr, "❌", err)
os.Exit(1)
}
case "fetch-builds":
if err := runFetchBuilds(); err != nil {
fmt.Fprintln(os.Stderr, "❌ fetch-builds:", err)
os.Exit(1)
}
default:
fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", cmd)
fmt.Fprintln(os.Stderr, "usage: stats [fetch-homebrew|fetch-testhub|fetch-countme|fetch-contributors|fetch-builds]")
os.Exit(1)
}
}

// ── fetch-homebrew ──────────────────────────────────────────────────────────

// homebrewOutput is the full JSON written to src/data/stats.json.
type homebrewOutput struct {
GeneratedAt string                 `json:"generated_at"`
Summary     metrics.Summary        `json:"summary"`
Taps        []tap.TapStats         `json:"taps"`
TopPackages []metrics.TopPackage   `json:"top_packages"`
History     []history.DaySnapshot  `json:"history"`
OSAnalytics *osanalytics.Analytics `json:"os_analytics,omitempty"`
}

// taps to track, in display order.
var taps = []struct{ owner, repo string }{
{"ublue-os", "homebrew-tap"},
{"ublue-os", "homebrew-experimental-tap"},
}

func runFetchHomebrew() error {
client, err := ghclient.NewClient()
if err != nil {
return err
}

hist, err := history.LoadWithBootstrap("src/data/stats.json")
if err != nil {
fmt.Fprintf(os.Stderr, "⚠️  Could not load history: %v\n", err)
hist = &history.Store{}
}

fmt.Fprintln(os.Stderr, "→ Fetching Homebrew cask-install analytics…")
brewInstalls, err := tapanalytics.Fetch()
if err != nil {
fmt.Fprintf(os.Stderr, "⚠️  Homebrew cask-install analytics: %v\n", err)
brewInstalls = make(map[string]tapanalytics.PkgInstalls)
} else {
fmt.Fprintf(os.Stderr, "  cask-install: %d ublue-os packages found\n", len(brewInstalls))
}

tapStats := make([]tap.TapStats, 0, len(taps))
todayTaps := make(map[string]history.TapSnapshot)
for _, t := range taps {
fmt.Fprintf(os.Stderr, "→ Collecting %s/%s…\n", t.owner, t.repo)
ts, err := tap.Collect(t.owner, t.repo, client, brewInstalls)
if err != nil {
fmt.Fprintf(os.Stderr, "⚠️  %s/%s: %v\n", t.owner, t.repo, err)
continue
}
tapStats = append(tapStats, *ts)
pkgDownloads := make(map[string]int64, len(ts.Packages))
for _, pkg := range ts.Packages {
if pkg.Downloads > 0 {
pkgDownloads[pkg.Name] = pkg.Downloads
}
}
snap := history.TapSnapshot{Downloads: pkgDownloads}
if ts.Traffic != nil {
snap.Uniques = ts.Traffic.Uniques
snap.Count = ts.Traffic.Count
}
if len(pkgDownloads) > 0 || ts.Traffic != nil {
todayTaps[ts.Name] = snap
}
}

if len(todayTaps) > 0 {
hist.Append(todayTaps)
if err := hist.Save(); err != nil {
fmt.Fprintf(os.Stderr, "⚠️  Could not save history: %v\n", err)
}
}

for i := range tapStats {
ts := &tapStats[i]
ts.GrowthPct = metrics.GrowthPct(hist.Snapshots, ts.Name)
for j := range ts.Packages {
pkg := &ts.Packages[j]
pkg.Velocity7d = metrics.Velocity7d(hist.Snapshots, ts.Name, pkg.Name)
}
}
summary := metrics.ComputeSummary(tapStats, hist.Snapshots)
topPkgs := metrics.ComputeTopPackages(tapStats, hist.Snapshots)

var osData *osanalytics.Analytics
osPeriods := make([]osanalytics.PeriodData, 0, 3)
for _, p := range []string{"30d", "90d", "365d"} {
pd, err := osanalytics.Fetch(p)
if err != nil {
fmt.Fprintf(os.Stderr, "⚠️  OS analytics (%s): %v\n", p, err)
continue
}
osPeriods = append(osPeriods, *pd)
}
if len(osPeriods) > 0 {
osData = &osanalytics.Analytics{Periods: osPeriods}
}

out := homebrewOutput{
GeneratedAt: time.Now().UTC().Format(time.RFC3339),
Summary:     summary,
Taps:        tapStats,
TopPackages: topPkgs,
History:     hist.Snapshots,
OSAnalytics: osData,
}
if err := writeJSON("src/data/stats.json", out); err != nil {
return err
}
backupPath := filepath.Join(".sync-cache", "stats-latest.json")
data, _ := json.MarshalIndent(out, "", "  ")
if err := os.WriteFile(backupPath, data, 0o644); err != nil {
fmt.Fprintf(os.Stderr, "⚠️  Could not write stats backup: %v\n", err)
} else {
fmt.Fprintln(os.Stderr, "✓ Backed up stats to", backupPath)
}
fmt.Fprintln(os.Stderr, "✓ Wrote src/data/stats.json")
for _, ts := range tapStats {
if ts.Traffic != nil {
fmt.Fprintf(os.Stderr, "  %s: %d unique tappers, %d packages\n",
ts.Name, ts.Traffic.Uniques, len(ts.Packages))
}
}
fmt.Fprintf(os.Stderr, "  History: %d snapshots\n", len(hist.Snapshots))
return nil
}

// ── fetch-testhub ───────────────────────────────────────────────────────────

const testhubCacheFile = ".sync-cache/testhub-history.json"

type testhubOutput struct {
GeneratedAt  string                `json:"generated_at"`
Packages     []testhub.Package     `json:"packages"`
BuildMetrics []testhub.BuildMetrics `json:"build_metrics"`
History      []testhub.DaySnapshot  `json:"history"`
}

func runFetchTesthub() error {
client, err := ghclient.NewClient()
if err != nil {
return err
}

store, err := loadTesthubHistory()
if err != nil {
fmt.Fprintf(os.Stderr, "⚠️  testhub history: %v\n", err)
store = &testhub.HistoryStore{}
}

// Determine last processed run ID to fetch only new runs.
var lastRunID int64
if len(store.Snapshots) > 0 {
lastRunID = store.Snapshots[len(store.Snapshots)-1].LastRunID
}

fmt.Fprintln(os.Stderr, "→ Fetching projectbluefin testhub packages…")
pkgs, err := testhub.ListPackages(client.Context(), client.GitHub(), "projectbluefin")
if err != nil {
fmt.Fprintf(os.Stderr, "⚠️  testhub packages: %v\n", err)
pkgs = nil
} else {
fmt.Fprintf(os.Stderr, "  packages: %d\n", len(pkgs))
}

fmt.Fprintf(os.Stderr, "→ Fetching testhub build counts (since run %d)…\n", lastRunID)
counts, newLastRunID, fetchErr := testhub.FetchBuildCounts(client.Context(), client.GitHub(), lastRunID)
if fetchErr != nil {
fmt.Fprintf(os.Stderr, "⚠️  testhub build counts: %v\n", fetchErr)
counts = nil
newLastRunID = lastRunID
} else {
fmt.Fprintf(os.Stderr, "  build counts: %d apps, new max run_id=%d\n", len(counts), newLastRunID)
}

if fetchErr != nil {
fmt.Fprintf(os.Stderr, "⚠️  skipping testhub history save — fetch failed: %v\n", fetchErr)
} else {
store = testhub.AppendSnapshot(store, pkgs, counts, newLastRunID)
if err := saveTesthubHistory(store); err != nil {
fmt.Fprintf(os.Stderr, "⚠️  failed to save testhub history: %v\n", err)
}
}

// Compute build metrics for 7d and 30d windows; merge PassRate30d into results.
metrics7d := testhub.ComputeBuildMetrics(store.Snapshots, 7)
metrics30d := testhub.ComputeBuildMetrics(store.Snapshots, 30)
// Build a lookup for 30d rates.
rate30d := make(map[string]float64, len(metrics30d))
for _, m := range metrics30d {
rate30d[m.App] = m.PassRate30d
}
// Merge: fill PassRate30d and LastStatus/LastBuildAt.
lastStatusByApp := computeLastStatus(store.Snapshots)
buildMetrics := make([]testhub.BuildMetrics, 0, len(metrics7d))
for _, m := range metrics7d {
	m.PassRate30d = rate30d[m.App]
	if ls, ok := lastStatusByApp[m.App]; ok {
		m.LastStatus = ls.status
		m.LastBuildAt = ls.at
	}
	buildMetrics = append(buildMetrics, m)
}

if len(buildMetrics) == 0 {
	// If computed metrics are empty (e.g. cold start with empty history),
	// fall back to the committed src/data/testhub.json so the site always
	// has build status data instead of all-unknown ⚪ — .
	if fallback := loadFallbackTesthubBuildMetrics(); len(fallback) > 0 {
		buildMetrics = fallback
		fmt.Fprintf(os.Stderr, "  using %d fallback build metrics from committed testhub.json\n", len(buildMetrics))
	} else {
		buildMetrics = []testhub.BuildMetrics{}
	}
}

if pkgs == nil {
	// Package listing failed (e.g. missing read:packages scope on GITHUB_TOKEN).
	// Fall back to the committed src/data/testhub.json so the site always has
	// package data instead of rendering an empty table.
	if fallback := loadFallbackTesthubPackages(); len(fallback) > 0 {
		pkgs = fallback
		fmt.Fprintf(os.Stderr, "  using %d fallback packages from committed testhub.json\n", len(pkgs))
	} else {
		pkgs = []testhub.Package{}
	}
}
if store.Snapshots == nil {
store.Snapshots = []testhub.DaySnapshot{}
}
out := testhubOutput{
GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
Packages:     pkgs,
BuildMetrics: buildMetrics,
History:      store.Snapshots,
}
if err := writeJSON("src/data/testhub.json", out); err != nil {
return err
}
fmt.Fprintln(os.Stderr, "✓ Wrote src/data/testhub.json")
return nil
}


// loadFallbackTesthubPackages reads the package list from the committed
// src/data/testhub.json. Used when the GitHub API call fails (e.g. missing
// read:packages scope) so the rendered site always has package data.
func loadFallbackTesthubPackages() []testhub.Package {
	data, err := os.ReadFile("src/data/testhub.json")
	if err != nil {
		return nil
	}
	var out testhubOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out.Packages
}

// loadFallbackTesthubBuildMetrics reads the build metrics from the committed
// src/data/testhub.json. Used when the history compute yields no results
// (e.g. CI cold-start with no cached history) so the rendered site
// always has status data instead of all-unknown ⚪ — .
func loadFallbackTesthubBuildMetrics() []testhub.BuildMetrics {
	data, err := os.ReadFile("src/data/testhub.json")
	if err != nil {
		return nil
	}
	var out testhubOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out.BuildMetrics
}

type lastStatus struct {
status string
at     string
}

// computeLastStatus returns the last known build status per app from snapshots.
func computeLastStatus(snapshots []testhub.DaySnapshot) map[string]lastStatus {
// Sort descending by date to find the most recent entry per app.
sorted := make([]testhub.DaySnapshot, len(snapshots))
copy(sorted, snapshots)
sort.Slice(sorted, func(i, j int) bool { return sorted[i].Date > sorted[j].Date })

result := make(map[string]lastStatus)
for _, snap := range sorted {
for _, c := range snap.BuildCounts {
if _, seen := result[c.App]; seen {
continue
}
status := "unknown"
if c.Passed > 0 && c.Failed == 0 {
status = "passing"
} else if c.Failed > 0 {
status = "failing"
}
result[c.App] = lastStatus{status: status, at: snap.Date}
}
}
return result
}

// hasBuildCounts returns true if at least one snapshot has non-empty build data.
// Used to detect caches that exist but were written before build counts were available.
func hasBuildCounts(snapshots []testhub.DaySnapshot) bool {
for _, snap := range snapshots {
if len(snap.BuildCounts) > 0 {
return true
}
}
return false
}

func loadTesthubHistoryFrom(cacheFile, seedFile string) (*testhub.HistoryStore, error) {
data, err := os.ReadFile(cacheFile)
if err == nil {
var store testhub.HistoryStore
if jsonErr := json.Unmarshal(data, &store); jsonErr == nil && hasBuildCounts(store.Snapshots) {
// Cache is valid and has snapshots with build data — use it.
return &store, nil
}
// Cache exists but is empty, malformed, or all snapshots lack build data — fall through to seed.
fmt.Fprintf(os.Stderr, "  cache file empty or missing build data, trying seed file\n")
}
// Try seed file (covers: file-not-found, read error, empty/malformed cache).
if seed, seedErr := os.ReadFile(seedFile); seedErr == nil {
var store testhub.HistoryStore
if json.Unmarshal(seed, &store) == nil && len(store.Snapshots) > 0 {
fmt.Fprintf(os.Stderr, "  loaded %d snapshots from seed file\n", len(store.Snapshots))
return &store, nil
}
}
return &testhub.HistoryStore{}, nil
}

func loadTesthubHistory() (*testhub.HistoryStore, error) {
	return loadTesthubHistoryFrom(testhubCacheFile, "src/data/testhub-seed-history.json")
}

func saveTesthubHistory(store *testhub.HistoryStore) error {
data, err := json.MarshalIndent(store, "", "  ")
if err != nil {
return err
}
if err := os.MkdirAll(".sync-cache", 0o755); err != nil {
return err
}
return os.WriteFile(testhubCacheFile, data, 0o644)
}

// ── fetch-countme ───────────────────────────────────────────────────────────

const countmeCacheFile = ".sync-cache/countme-history.json"

type countmeWoW struct {
Bazzite    float64 `json:"bazzite"`
Bluefin    float64 `json:"bluefin"`
BluefinLTS float64 `json:"bluefin-lts"`
Aurora     float64 `json:"aurora"`
Total      float64 `json:"total"`
}

type countmeOutput struct {
GeneratedAt   string                    `json:"generated_at"`
CurrentWeek   *countme.WeekRecord       `json:"current_week,omitempty"`
PrevWeek      *countme.WeekRecord       `json:"prev_week,omitempty"`
WoWGrowthPct  *countmeWoW               `json:"wow_growth_pct,omitempty"`
History       countme.HistoryStore      `json:"history"`
OsVersionDist map[string]map[string]int `json:"os_version_dist,omitempty"`
}

func runFetchCountme() error {
store, err := loadCountmeHistory()
if err != nil {
fmt.Fprintf(os.Stderr, "⚠️  countme history: %v\n", err)
store = &countme.HistoryStore{}
}

fmt.Fprintln(os.Stderr, "→ Fetching Universal Blue badge counts…")
badge, err := countme.FetchBadgeCounts()
if err != nil {
fmt.Fprintf(os.Stderr, "⚠️  countme badges: %v\n", err)
badge = nil
} else {
fmt.Fprintf(os.Stderr, "  badges: %v\n", badge)
}

fmt.Fprintln(os.Stderr, "→ Fetching countme CSV (last 30d)…")
csvRecs, osVersionDist, err := countme.FetchCSVLast30Days()
if err != nil {
fmt.Fprintf(os.Stderr, "⚠️  countme CSV: %v\n", err)
csvRecs = nil
osVersionDist = nil
} else {
fmt.Fprintf(os.Stderr, "  CSV: %d week records\n", len(csvRecs))
}

if csvRecs != nil {
store = countme.MergeIntoHistory(store, csvRecs)
}
if osVersionDist != nil {
store.OsVersionDist = countme.MergeOsVersionDist(store.OsVersionDist, osVersionDist)
}
if badge != nil {
store = countme.AppendDayRecord(store, badge)
}
if err := saveCountmeHistory(store); err != nil {
fmt.Fprintf(os.Stderr, "⚠️  countme history save: %v\n", err)
}

// If we still have no week history (e.g. cold-start CI with empty cache + failed CSV),
// fall back to the committed src/data/countme.json so the site renders real data.
if len(store.WeekRecords) == 0 {
	if fb := loadFallbackCountmeHistory(); fb != nil {
		fmt.Fprintf(os.Stderr, "  using %d fallback week records from committed countme.json\n", len(fb.WeekRecords))
		store = fb
	}
}

out := buildCountmeOutput(store)
if err := writeJSON("src/data/countme.json", out); err != nil {
return err
}
fmt.Fprintln(os.Stderr, "✓ Wrote src/data/countme.json")
return nil
}

func buildCountmeOutput(store *countme.HistoryStore) countmeOutput {
// Ensure nil slices marshal as [] not null in JSON.
if store.WeekRecords == nil {
store.WeekRecords = []countme.WeekRecord{}
}
if store.DayRecords == nil {
store.DayRecords = []countme.DayRecord{}
}
out := countmeOutput{
GeneratedAt: time.Now().UTC().Format(time.RFC3339),
History:     *store,
}

// Sort week records descending by week_start to find current and prev.
weeks := make([]countme.WeekRecord, len(store.WeekRecords))
copy(weeks, store.WeekRecords)
sort.Slice(weeks, func(i, j int) bool { return weeks[i].WeekStart > weeks[j].WeekStart })

if len(weeks) >= 1 {
w := weeks[0]
out.CurrentWeek = &w
}
if len(weeks) >= 2 {
w := weeks[1]
out.PrevWeek = &w
}
if out.CurrentWeek != nil && out.PrevWeek != nil {
out.WoWGrowthPct = computeWoW(out.CurrentWeek, out.PrevWeek)
}
out.OsVersionDist = store.OsVersionDist
return out
}

func computeWoW(current, prev *countme.WeekRecord) *countmeWoW {
growth := func(cur, prv int) float64 {
if prv == 0 {
return 0
}
return float64(cur-prv) / float64(prv) * 100.0
}
return &countmeWoW{
Bazzite:    growth(current.Distros["bazzite"], prev.Distros["bazzite"]),
Bluefin:    growth(current.Distros["bluefin"], prev.Distros["bluefin"]),
BluefinLTS: growth(current.Distros["bluefin-lts"], prev.Distros["bluefin-lts"]),
Aurora:     growth(current.Distros["aurora"], prev.Distros["aurora"]),
Total:      growth(current.Total, prev.Total),
}
}

func loadCountmeHistory() (*countme.HistoryStore, error) {
data, err := os.ReadFile(countmeCacheFile)
if os.IsNotExist(err) {
return &countme.HistoryStore{}, nil
}
if err != nil {
return nil, err
}
var store countme.HistoryStore
if err := json.Unmarshal(data, &store); err != nil {
return nil, err
}
return &store, nil
}

func saveCountmeHistory(store *countme.HistoryStore) error {
data, err := json.MarshalIndent(store, "", "  ")
if err != nil {
return err
}
if err := os.MkdirAll(".sync-cache", 0o755); err != nil {
return err
}
return os.WriteFile(countmeCacheFile, data, 0o644)
}

// ── shared helpers ──────────────────────────────────────────────────────────

// writeJSON marshals v to JSON and writes it to path (creating parent dirs as needed).
func writeJSON(path string, v any) error {
data, err := json.MarshalIndent(v, "", "  ")
if err != nil {
return fmt.Errorf("json marshal: %w", err)
}
if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
return fmt.Errorf("mkdir: %w", err)
}
if err := os.WriteFile(path, data, 0o644); err != nil {
return fmt.Errorf("write %s: %w", path, err)
}
return nil
}

// loadFallbackCountmeHistory reads week_records from the committed src/data/countme.json
// and reconstructs a HistoryStore from it. Used when both the cache and the live CSV fetch
// are unavailable (e.g. CI cold-start with no read:packages scope or rate-limited endpoint).
func loadFallbackCountmeHistory() *countme.HistoryStore {
data, err := os.ReadFile("src/data/countme.json")
if err != nil {
return nil
}
var out countmeOutput
if err := json.Unmarshal(data, &out); err != nil {
return nil
}
if len(out.History.WeekRecords) == 0 {
return nil
}
return &countme.HistoryStore{
WeekRecords:  out.History.WeekRecords,
OsVersionDist: out.OsVersionDist,
}
}

// ── fetch-contributors ──────────────────────────────────────────────────────

const contributorsCacheFile   = ".sync-cache/contributors-history.json"
const contributorProfilesFile = ".sync-cache/contributor-profiles.json"

type contributorsOutput struct {
	GeneratedAt string `json:"generated_at"`
	Period      struct {
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"period"`
	Summary            contributors.ContributorSummary `json:"summary"`
	TopContributors    []contributors.ContributorEntry `json:"top_contributors"`
	Repos              []contributors.RepoStats        `json:"repos"`
	DiscussionsSummary contributors.DiscussionSummary  `json:"discussions_summary"`
}

func loadContributorsHistory() (*contributors.ContribHistoryStore, error) {
	data, err := os.ReadFile(contributorsCacheFile)
	if os.IsNotExist(err) {
		return &contributors.ContribHistoryStore{}, nil
	}
	if err != nil {
		return nil, err
	}
	var store contributors.ContribHistoryStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, err
	}
	return &store, nil
}

func saveContributorsHistory(store *contributors.ContribHistoryStore) error {
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(".sync-cache", 0o755); err != nil {
		return err
	}
	return os.WriteFile(contributorsCacheFile, data, 0o644)
}

func loadContributorProfiles() (*contributors.ContributorProfileCache, error) {
	data, err := os.ReadFile(contributorProfilesFile)
	if os.IsNotExist(err) {
		return &contributors.ContributorProfileCache{Profiles: make(map[string]*contributors.CachedProfile)}, nil
	}
	if err != nil {
		return nil, err
	}
	var cache contributors.ContributorProfileCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	if cache.Profiles == nil {
		cache.Profiles = make(map[string]*contributors.CachedProfile)
	}
	return &cache, nil
}

func saveContributorProfiles(cache *contributors.ContributorProfileCache) error {
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(contributorProfilesFile, data, 0o644)
}

func runFetchContributors() error {
	client, err := ghclient.NewClient()
	if err != nil {
		return err
	}
	ghClient := client.GitHub()
	ctx := client.Context()

	since365 := time.Now().UTC().AddDate(0, 0, -365)
	since60  := time.Now().UTC().AddDate(0, 0, -60)
	since30  := time.Now().UTC().AddDate(0, 0, -30)
	until    := time.Now().UTC()

	hist, err := loadContributorsHistory()
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  contributors history: %v\n", err)
		hist = &contributors.ContribHistoryStore{}
	}

	profileCache, err := loadContributorProfiles()
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  contributor profiles: %v\n", err)
		profileCache = &contributors.ContributorProfileCache{Profiles: make(map[string]*contributors.CachedProfile)}
	}

	// Per-repo accumulators.
	var repoStats []contributors.RepoStats

	// Cross-repo accumulators — one map per time window.
	allAuthorCommits30d  := make(map[string]int) // login → commits in 30d
	allAuthorCommits60d  := make(map[string]int) // login → commits in 60d
	allAuthorCommits365d := make(map[string]int) // login → commits in 365d
	allAuthorPRs30d      := make(map[string]int)
	allAuthorPRs60d      := make(map[string]int)
	allAuthorPRs365d     := make(map[string]int)
	allAuthorIssues30d   := make(map[string]int)
	allAuthorIssues60d   := make(map[string]int)
	allAuthorIssues365d  := make(map[string]int)
	allAuthorDiscussions30d  := make(map[string]int)
	allAuthorDiscussions60d  := make(map[string]int)
	allAuthorDiscussions365d := make(map[string]int)
	authorRepos := make(map[string]map[string]bool)    // login → set of repos active in (30d)
	repoAuthorSets := make(map[string]map[string]bool) // repo → set of human author logins (30d)

	// Discussion accumulators.
	var allDiscussions []contributors.DiscussionRecord
	totalIssuesOpened30d  := 0
	totalIssuesClosed30d  := 0
	totalPRsMerged30d     := 0
	totalPRsWithReview30d := 0
	totalPRsMerged60d     := 0
	totalPRsMerged365d    := 0
	activeRepoCount := 0

	for _, fullName := range contributors.TrackedRepos {
		parts := strings.SplitN(fullName, "/", 2)
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "⚠️  skipping malformed repo name: %s\n", fullName)
			continue
		}
		owner, repoName := parts[0], parts[1]
		fmt.Fprintf(os.Stderr, "→ Processing %s/%s…\n", owner, repoName)

		// ── Commits ──────────────────────────────────────────────────────
		// Fetch 365 days once; slice in-memory for 30d and 60d windows.
		commits, err := contributors.FetchRepoCommits(ctx, ghClient, owner, repoName, since365, until)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠️  commits: %v\n", err)
			commits = nil
		}
		commits60 := contributors.FilterCommitsAfter(commits, since60)
		commits30 := contributors.FilterCommitsAfter(commits, since30)

		// Per-window author commit maps (used for bus factor and contributor entries).
		repoAuthorCommits30d  := make(map[string]int)
		repoAuthorCommits60d  := make(map[string]int)
		repoAuthorCommits365d := make(map[string]int)

		humanAuthors30d := make(map[string]bool)
		humanAuthors60d := make(map[string]bool)
		humanAuthors365d := make(map[string]bool)
		botCommits30d, humanCommits30d := 0, 0
		humanCommits60d, humanCommits365d := 0, 0

		// 365d pass — populates full-year maps; also sets up authorRepos (cross-repo).
		for _, c := range commits {
			if c.Login == "" {
				continue
			}
			repoAuthorCommits365d[c.Login]++
			allAuthorCommits365d[c.Login]++
			if !contributors.IsBot(c.Login) {
				humanAuthors365d[c.Login] = true
				humanCommits365d++
				if authorRepos[c.Login] == nil {
					authorRepos[c.Login] = make(map[string]bool)
				}
				authorRepos[c.Login][fullName] = true
			}
		}
		// 60d pass.
		for _, c := range commits60 {
			if c.Login == "" {
				continue
			}
			repoAuthorCommits60d[c.Login]++
			allAuthorCommits60d[c.Login]++
			if !contributors.IsBot(c.Login) {
				humanAuthors60d[c.Login] = true
				humanCommits60d++
			}
		}
		// 30d pass.
		for _, c := range commits30 {
			if c.Login == "" {
				continue
			}
			repoAuthorCommits30d[c.Login]++
			allAuthorCommits30d[c.Login]++
			if contributors.IsBot(c.Login) {
				botCommits30d++
			} else {
				humanAuthors30d[c.Login] = true
				humanCommits30d++
			}
		}
		repoAuthorSets[fullName] = humanAuthors30d

		// ── Issues ───────────────────────────────────────────────────────
		// Fetch 365d; filter in-memory for 30d/60d windows.
		issues, err := contributors.FetchRepoIssues(ctx, ghClient, owner, repoName, since365)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠️  issues: %v\n", err)
			issues = nil
		}
		issues60 := contributors.FilterIssuesAfter(issues, since60)
		issues30 := contributors.FilterIssuesAfter(issues, since30)

		issuesOpened30d := 0
		issuesClosed30d := 0
		issuesOpened60d := 0
		issuesOpened365d := 0
		issueLabelDist := make(map[string]int)

		for _, iss := range issues {
			issuesOpened365d++
			allAuthorIssues365d[iss.Login]++
		}
		for _, iss := range issues60 {
			issuesOpened60d++
			allAuthorIssues60d[iss.Login]++
		}
		for _, iss := range issues30 {
			issuesOpened30d++
			totalIssuesOpened30d++
			allAuthorIssues30d[iss.Login]++
			if iss.State == "closed" {
				issuesClosed30d++
				totalIssuesClosed30d++
			}
			for _, l := range iss.Labels {
				issueLabelDist[l.GetName()]++
			}
		}

		// ── PRs ──────────────────────────────────────────────────────────
		// Fetch 365d; filter in-memory for 30d/60d windows.
		prs, err := contributors.FetchRepoPRs(ctx, ghClient, owner, repoName, since365)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠️  PRs: %v\n", err)
			prs = nil
		}
		prs60 := contributors.FilterPRsAfter(prs, since60)
		prs30 := contributors.FilterPRsAfter(prs, since30)

		prsMerged30d := 0
		prsMerged60d := 0
		prsMerged365d := 0

		for _, pr := range prs {
			prsMerged365d++
			allAuthorPRs365d[pr.Login]++
		}
		for _, pr := range prs60 {
			prsMerged60d++
			totalPRsMerged60d++
			allAuthorPRs60d[pr.Login]++
		}
		for _, pr := range prs30 {
			prsMerged30d++
			totalPRsMerged30d++
			allAuthorPRs30d[pr.Login]++
			if pr.HasReviewers {
				totalPRsWithReview30d++
			}
		}
		totalPRsMerged365d += prsMerged365d

		// ── Discussions ───────────────────────────────────────────────────
		// Fetch 365d; all windows sliced in-memory from this set.
		discs, err := contributors.FetchDiscussions(client, owner, repoName, since365)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠️  discussions: %v\n", err)
			discs = nil
		}
		for _, d := range discs {
			allDiscussions = append(allDiscussions, d)
			if d.AuthorLogin != "" && !contributors.IsBot(d.AuthorLogin) {
				allAuthorDiscussions365d[d.AuthorLogin]++
			}
		}
		for _, d := range contributors.FilterDiscussionsAfter(discs, since60) {
			if d.AuthorLogin != "" && !contributors.IsBot(d.AuthorLogin) {
				allAuthorDiscussions60d[d.AuthorLogin]++
			}
		}
		for _, d := range contributors.FilterDiscussionsAfter(discs, since30) {
			if d.AuthorLogin != "" && !contributors.IsBot(d.AuthorLogin) {
				allAuthorDiscussions30d[d.AuthorLogin]++
			}
		}

		// ── Participation (52w weekly) ────────────────────────────────────
		weekly, err := contributors.FetchParticipation(ctx, ghClient, owner, repoName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠️  participation: %v\n", err)
			weekly = []int{}
		}

		// ── Punch card (heatmap) ─────────────────────────────────────────
		heatmap, err := contributors.FetchPunchCard(ctx, ghClient, owner, repoName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠️  punch card: %v\n", err)
			heatmap = [][]int{}
		}

		// Compute day-of-week breakdown from punch card.
		dayNames := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
		dayOfWeek := make(map[string]int, 7)
		for _, row := range heatmap {
			if len(row) == 3 && row[0] >= 0 && row[0] < 7 {
				dayOfWeek[dayNames[row[0]]] += row[2]
			}
		}

		busFactor30d  := contributors.ComputeBusFactor(repoAuthorCommits30d, 0.8)
		busFactor60d  := contributors.ComputeBusFactor(repoAuthorCommits60d, 0.8)
		busFactor365d := contributors.ComputeBusFactor(repoAuthorCommits365d, 0.8)
		streak := contributors.ComputeActiveWeeksStreak(weekly)

		if len(commits30) > 0 || issuesOpened30d > 0 || prsMerged30d > 0 {
			activeRepoCount++
		}

		rs := contributors.RepoStats{
			Name:                  fullName,
			Commits30d:            len(commits30),
			Commits60d:            len(commits60),
			Commits365d:           len(commits),
			UniqueHumanAuthors30d: len(humanAuthors30d),
			PRsMerged30d:          prsMerged30d,
			PRsMerged60d:          prsMerged60d,
			PRsMerged365d:         prsMerged365d,
			IssuesOpened30d:       issuesOpened30d,
			IssuesOpened60d:       issuesOpened60d,
			IssuesOpened365d:      issuesOpened365d,
			BusFactor:             busFactor30d,
			BusFactor60d:          busFactor60d,
			BusFactor365d:         busFactor365d,
			BotCommits30d:         botCommits30d,
			HumanCommits30d:       humanCommits30d,
			HumanCommits60d:       humanCommits60d,
			HumanCommits365d:      humanCommits365d,
			ActiveWeeksStreak:     streak,
			WeeklyCommits52w:      weekly,
			CommitsByDayOfWeek:    dayOfWeek,
			ContributionHeatmap:   heatmap,
			IssueLabelDist:        issueLabelDist,
		}
		repoStats = append(repoStats, rs)
	}

	// ── Compute summary ───────────────────────────────────────────────────────

	// Gather unique human logins active per window.
	activeLogins30d := make([]string, 0)
	for login := range allAuthorCommits30d {
		if !contributors.IsBot(login) {
			activeLogins30d = append(activeLogins30d, login)
		}
	}
	// Also include humans who only opened issues or discussions in 30d.
	for login := range allAuthorIssues30d {
		if !contributors.IsBot(login) {
			allAuthorCommits30d[login] = allAuthorCommits30d[login] // ensure present
		}
	}

	activeLogins60d := make([]string, 0)
	for login := range allAuthorCommits60d {
		if !contributors.IsBot(login) {
			activeLogins60d = append(activeLogins60d, login)
		}
	}
	activeLogins365d := make([]string, 0)
	for login := range allAuthorCommits365d {
		if !contributors.IsBot(login) {
			activeLogins365d = append(activeLogins365d, login)
		}
	}

	// Build historical login set from prior snapshots (for new contributor detection).
	historicalLogins := make(map[string]bool)
	for _, snap := range hist.Snapshots {
		for _, l := range snap.TopContributors {
			historicalLogins[l] = true
		}
	}

	newContribs := contributors.ComputeNewContributors(activeLogins30d, historicalLogins)
	reviewRate := contributors.ComputeReviewParticipationRate(totalPRsWithReview30d, totalPRsMerged30d)

	// Global bus factor across all repos, per window.
	globalBusFactor30d  := contributors.ComputeBusFactor(allAuthorCommits30d, 0.8)
	globalBusFactor60d  := contributors.ComputeBusFactor(allAuthorCommits60d, 0.8)
	globalBusFactor365d := contributors.ComputeBusFactor(allAuthorCommits365d, 0.8)

	// Total commits (human + bot) per window.
	totalCommits30d := 0
	for _, c := range allAuthorCommits30d {
		totalCommits30d += c
	}
	totalCommits60d := 0
	for _, c := range allAuthorCommits60d {
		totalCommits60d += c
	}
	totalCommits365d := 0
	for _, c := range allAuthorCommits365d {
		totalCommits365d += c
	}

	// ── Discussion summary ────────────────────────────────────────────────────
	// allDiscussions holds 365d of data; filter in-memory for each window.
	discs30d := contributors.FilterDiscussionsAfter(allDiscussions, since30)
	discs60d := contributors.FilterDiscussionsAfter(allDiscussions, since60)

	discAuthors30d := make(map[string]bool)
	discAuthors60d := make(map[string]bool)
	discAuthors365d := make(map[string]bool)
	totalDiscComments30d := 0
	totalDiscComments60d := 0
	totalDiscComments365d := 0

	for _, d := range allDiscussions {
		if d.AuthorLogin != "" && !contributors.IsBot(d.AuthorLogin) {
			discAuthors365d[d.AuthorLogin] = true
		}
		totalDiscComments365d += d.CommentCount
	}
	for _, d := range discs60d {
		if d.AuthorLogin != "" && !contributors.IsBot(d.AuthorLogin) {
			discAuthors60d[d.AuthorLogin] = true
		}
		totalDiscComments60d += d.CommentCount
	}
	for _, d := range discs30d {
		if d.AuthorLogin != "" && !contributors.IsBot(d.AuthorLogin) {
			discAuthors30d[d.AuthorLogin] = true
		}
		totalDiscComments30d += d.CommentCount
	}

	discSummary := contributors.DiscussionSummary{
		TotalDiscussions30d:         len(discs30d),
		TotalDiscussions60d:         len(discs60d),
		TotalDiscussions365d:        len(allDiscussions),
		TotalDiscussionComments30d:  totalDiscComments30d,
		TotalDiscussionComments60d:  totalDiscComments60d,
		TotalDiscussionComments365d: totalDiscComments365d,
		UniqueDiscussionAuthors30d:  len(discAuthors30d),
		UniqueDiscussionAuthors60d:  len(discAuthors60d),
		UniqueDiscussionAuthors365d: len(discAuthors365d),
		WeeklyTrend:                 []contributors.DiscussionWeek{},
	}

	// Build weekly trend: bucket discussions by Monday of their creation week.
	if len(allDiscussions) > 0 {
		weekMap := make(map[string]*contributors.DiscussionWeek)
		for _, d := range discs30d {
			// Truncate to Monday of that week.
			wd := int(d.CreatedAt.Weekday())
			if wd == 0 {
				wd = 7 // Sunday → 7 so Monday offset = wd-1
			}
			monday := d.CreatedAt.AddDate(0, 0, -(wd - 1))
			key := monday.Format("2006-01-02")
			if weekMap[key] == nil {
				weekMap[key] = &contributors.DiscussionWeek{Week: key}
			}
			weekMap[key].Discussions++
			weekMap[key].Comments += d.CommentCount
		}
		// Sort weeks ascending.
		weeks := make([]contributors.DiscussionWeek, 0, len(weekMap))
		for _, w := range weekMap {
			weeks = append(weeks, *w)
		}
		sort.Slice(weeks, func(i, j int) bool { return weeks[i].Week < weeks[j].Week })
		discSummary.WeeklyTrend = weeks
	}

	summary := contributors.ContributorSummary{
		ActiveContributors:      len(activeLogins30d),
		ActiveContributors60d:   len(activeLogins60d),
		ActiveContributors365d:  len(activeLogins365d),
		NewContributors:         len(newContribs),
		TotalCommits:            totalCommits30d,
		TotalCommits60d:         totalCommits60d,
		TotalCommits365d:        totalCommits365d,
		TotalPRsMerged:          totalPRsMerged30d,
		TotalPRsMerged60d:       totalPRsMerged60d,
		TotalPRsMerged365d:      totalPRsMerged365d,
		TotalIssuesOpened:       totalIssuesOpened30d,
		TotalIssuesClosed:       totalIssuesClosed30d,
		BusFactor:               globalBusFactor30d,
		BusFactor60d:            globalBusFactor60d,
		BusFactor365d:           globalBusFactor365d,
		ReviewParticipationRate: reviewRate,
		ActiveRepos:             activeRepoCount,
		TotalDiscussions:        len(discs30d),
		DiscussionAnswerRate:    discSummary.AnsweredRate,
	}

	// ── Top contributors (fetch profiles, build entries) ──────────────────────

	// Sort active logins by commit count descending (30d window drives ranking).
	type loginCount struct{ login string; count int }
	ranked := make([]loginCount, 0, len(activeLogins30d))
	for _, login := range activeLogins30d {
		ranked = append(ranked, loginCount{login, allAuthorCommits30d[login]})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].count > ranked[j].count })

	const maxTop = 25
	if len(ranked) > maxTop {
		ranked = ranked[:maxTop]
	}

	topContribs := make([]contributors.ContributorEntry, 0, len(ranked))
	topLogins := make([]string, 0, len(ranked))
	for _, rc := range ranked {
		topLogins = append(topLogins, rc.login)

		// Fetch profile (uses cache).
		profile, err := contributors.FetchUserProfile(ctx, ghClient, rc.login, profileCache)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠️  profile %s: %v\n", rc.login, err)
		}

		entry := contributors.ContributorEntry{
			Login:               rc.login,
			Commits30d:          rc.count,
			Commits60d:          allAuthorCommits60d[rc.login],
			Commits365d:         allAuthorCommits365d[rc.login],
			PRsMerged30d:        allAuthorPRs30d[rc.login],
			PRsMerged60d:        allAuthorPRs60d[rc.login],
			PRsMerged365d:       allAuthorPRs365d[rc.login],
			IssuesOpened30d:     allAuthorIssues30d[rc.login],
			IssuesOpened60d:     allAuthorIssues60d[rc.login],
			IssuesOpened365d:    allAuthorIssues365d[rc.login],
			DiscussionPosts30d:  allAuthorDiscussions30d[rc.login],
			DiscussionPosts60d:  allAuthorDiscussions60d[rc.login],
			DiscussionPosts365d: allAuthorDiscussions365d[rc.login],
			IsBot:               false,
		}

		// Collect repos this login was active in.
		if repos, ok := authorRepos[rc.login]; ok {
			for r := range repos {
				entry.ReposActive = append(entry.ReposActive, r)
			}
			sort.Strings(entry.ReposActive)
		}

		if profile != nil {
			entry.Name = profile.Name
			entry.AvatarURL = profile.AvatarURL
			entry.Company = profile.Company
			entry.Location = profile.Location
		}
		topContribs = append(topContribs, entry)
	}

	// ── Persist history snapshot ──────────────────────────────────────────────
	snap := contributors.ContribDaySnapshot{
		Date:            time.Now().UTC().Format("2006-01-02"),
		ActiveContribs:  len(activeLogins30d),
		TotalCommits:    totalCommits30d,
		TopContributors: topLogins,
	}
	hist.Snapshots = append(hist.Snapshots, snap)
	hist.LastFetchedAt = time.Now().UTC().Format(time.RFC3339)

	if err := saveContributorsHistory(hist); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  save contributors history: %v\n", err)
	}
	if err := saveContributorProfiles(profileCache); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  save contributor profiles: %v\n", err)
	}

	// ── Assemble and write output ─────────────────────────────────────────────
	if repoStats == nil {
		repoStats = []contributors.RepoStats{}
	}
	if topContribs == nil {
		topContribs = []contributors.ContributorEntry{}
	}
	if discSummary.WeeklyTrend == nil {
		discSummary.WeeklyTrend = []contributors.DiscussionWeek{}
	}

	out := contributorsOutput{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Summary:     summary,
		TopContributors: topContribs,
		Repos:       repoStats,
		DiscussionsSummary: discSummary,
	}
	out.Period.Start = since30.Format("2006-01-02")
	out.Period.End = until.Format("2006-01-02")

	if err := writeJSON("src/data/contributors.json", out); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "✓ Wrote src/data/contributors.json")
	fmt.Fprintf(os.Stderr, "  active contributors: %d, repos: %d, commits: %d\n",
		len(activeLogins30d), activeRepoCount, totalCommits30d)
	return nil
}

// ── fetch-builds ─────────────────────────────────────────────────────────────

func runFetchBuilds() error {
	client, err := ghclient.NewClient()
	if err != nil {
		return err
	}

	cfg := builds.CollectorConfig{
		Repos:        builds.DefaultRepos,
		LookbackDays: 14,  // cold-start bootstrap; warm runs use latest-1d automatically
		MaxRunsPerWf: 30,  // cap per workflow file to bound cold-start API calls
		HistoryPath:  ".sync-cache/builds-history.json",
		OutputPath:   "src/data/builds.json",
	}

	collector := builds.NewCollector(client.GitHub(), cfg)
	return collector.Run(client.Context())
}
