package main

import (
"encoding/json"
"fmt"
"os"
"path/filepath"
"sort"
"time"

ghclient "github.com/castrojo/homebrew-stats/internal/github"
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
default:
fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", cmd)
fmt.Fprintln(os.Stderr, "usage: stats [fetch-homebrew|fetch-testhub|fetch-countme]")
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
counts, newLastRunID, err := testhub.FetchBuildCounts(client.Context(), client.GitHub(), lastRunID)
if err != nil {
fmt.Fprintf(os.Stderr, "⚠️  testhub build counts: %v\n", err)
counts = nil
newLastRunID = lastRunID
} else {
fmt.Fprintf(os.Stderr, "  build counts: %d apps, new max run_id=%d\n", len(counts), newLastRunID)
}

store = testhub.AppendSnapshot(store, pkgs, counts, newLastRunID)
if err := saveTesthubHistory(store); err != nil {
fmt.Fprintf(os.Stderr, "⚠️  testhub history save: %v\n", err)
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

if pkgs == nil {
pkgs = []testhub.Package{}
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

func loadTesthubHistory() (*testhub.HistoryStore, error) {
data, err := os.ReadFile(testhubCacheFile)
if os.IsNotExist(err) {
return &testhub.HistoryStore{}, nil
}
if err != nil {
return nil, err
}
var store testhub.HistoryStore
if err := json.Unmarshal(data, &store); err != nil {
return nil, err
}
return &store, nil
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
GeneratedAt string              `json:"generated_at"`
CurrentWeek *countme.WeekRecord `json:"current_week,omitempty"`
PrevWeek    *countme.WeekRecord `json:"prev_week,omitempty"`
WoWGrowthPct *countmeWoW        `json:"wow_growth_pct,omitempty"`
History     countme.HistoryStore `json:"history"`
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
csvRecs, err := countme.FetchCSVLast30Days()
if err != nil {
fmt.Fprintf(os.Stderr, "⚠️  countme CSV: %v\n", err)
csvRecs = nil
} else {
fmt.Fprintf(os.Stderr, "  CSV: %d week records\n", len(csvRecs))
}

if csvRecs != nil {
store = countme.MergeIntoHistory(store, csvRecs)
}
if badge != nil {
store = countme.AppendDayRecord(store, badge)
}
if err := saveCountmeHistory(store); err != nil {
fmt.Fprintf(os.Stderr, "⚠️  countme history save: %v\n", err)
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
