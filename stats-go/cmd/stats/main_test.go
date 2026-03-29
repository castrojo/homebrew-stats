package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/castrojo/homebrew-stats/internal/builds"
	"github.com/castrojo/homebrew-stats/internal/testhub"
)

func writeTempFile(t *testing.T, dir, name string, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadTesthubHistoryFrom_CacheHit(t *testing.T) {
	dir := t.TempDir()
	// Cache must have at least one snapshot with build_counts for it to be used directly.
	store := testhub.HistoryStore{Snapshots: []testhub.DaySnapshot{
		{Date: "2026-01-01", BuildCounts: []testhub.AppDayCount{{App: "test-app", Passed: 5, Failed: 0, Total: 5}}},
	}}
	cacheFile := writeTempFile(t, dir, "cache.json", store)
	seedFile := filepath.Join(dir, "seed.json") // does not exist

	got, err := loadTesthubHistoryFrom(cacheFile, seedFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Snapshots) != 1 {
		t.Errorf("want 1 snapshot, got %d", len(got.Snapshots))
	}
}

func TestLoadTesthubHistoryFrom_SeedFallback(t *testing.T) {
	dir := t.TempDir()
	seed := testhub.HistoryStore{Snapshots: []testhub.DaySnapshot{{Date: "2026-02-01"}, {Date: "2026-02-02"}}}
	seedFile := writeTempFile(t, dir, "seed.json", seed)
	cacheFile := filepath.Join(dir, "cache.json") // does not exist

	got, err := loadTesthubHistoryFrom(cacheFile, seedFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Snapshots) != 2 {
		t.Errorf("want 2 snapshots from seed, got %d", len(got.Snapshots))
	}
}

func TestLoadTesthubHistoryFrom_BothMissing(t *testing.T) {
	dir := t.TempDir()
	got, err := loadTesthubHistoryFrom(
		filepath.Join(dir, "cache.json"),
		filepath.Join(dir, "seed.json"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Snapshots) != 0 {
		t.Errorf("want empty store, got %d snapshots", len(got.Snapshots))
	}
}

func TestLoadTesthubHistoryFrom_CorruptSeed(t *testing.T) {
	dir := t.TempDir()
	cacheFile := filepath.Join(dir, "cache.json") // missing
	seedFile := filepath.Join(dir, "seed.json")
	if err := os.WriteFile(seedFile, []byte("not valid json {{{"), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := loadTesthubHistoryFrom(cacheFile, seedFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Snapshots) != 0 {
		t.Errorf("want empty store on corrupt seed, got %d snapshots", len(got.Snapshots))
	}
}

// TestLoadTesthubHistoryFrom_CacheNoBuildCountsFallsToSeed verifies that when the
// cache file exists and has snapshots but ALL have empty build_counts (e.g. written
// during an outage when FetchBuildCounts returned nothing), the seed is used instead.
func TestLoadTesthubHistoryFrom_CacheNoBuildCountsFallsToSeed(t *testing.T) {
	dir := t.TempDir()
	// Cache has snapshots but no build_counts — simulates post-outage cache state.
	emptyBuildsStore := testhub.HistoryStore{Snapshots: []testhub.DaySnapshot{
		{Date: "2026-03-20"},
		{Date: "2026-03-21"},
	}}
	cacheFile := writeTempFile(t, dir, "cache.json", emptyBuildsStore)
	seed := testhub.HistoryStore{Snapshots: []testhub.DaySnapshot{
		{Date: "2026-02-01", BuildCounts: []testhub.AppDayCount{{App: "a", Passed: 3, Failed: 0, Total: 3}}},
		{Date: "2026-02-02", BuildCounts: []testhub.AppDayCount{{App: "a", Passed: 2, Failed: 1, Total: 3}}},
	}}
	seedFile := writeTempFile(t, dir, "seed.json", seed)

	got, err := loadTesthubHistoryFrom(cacheFile, seedFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Snapshots) != 2 {
		t.Errorf("want 2 snapshots from seed (cache had no build data), got %d", len(got.Snapshots))
	}
	if got.Snapshots[0].Date != "2026-02-01" {
		t.Errorf("expected seed data, got date %s", got.Snapshots[0].Date)
	}
}

// file exists but contains a zero-snapshot store (e.g. after a cache key bump
// creates an empty cache entry), the seed file is used instead.
func TestLoadTesthubHistoryFrom_EmptyCacheFallsToSeed(t *testing.T) {
	dir := t.TempDir()
	emptyStore := testhub.HistoryStore{Snapshots: []testhub.DaySnapshot{}}
	cacheFile := writeTempFile(t, dir, "cache.json", emptyStore)
	seed := testhub.HistoryStore{Snapshots: []testhub.DaySnapshot{{Date: "2026-03-01"}, {Date: "2026-03-02"}}}
	seedFile := writeTempFile(t, dir, "seed.json", seed)

	got, err := loadTesthubHistoryFrom(cacheFile, seedFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Snapshots) != 2 {
		t.Errorf("want 2 snapshots from seed (cache was empty), got %d", len(got.Snapshots))
	}
}

// TestLoadTesthubHistoryFrom_CorruptCacheFallsToSeed verifies that when the cache
// file exists but contains invalid JSON (e.g. zero-byte after a failed write),
// the seed file is used instead.
func TestLoadTesthubHistoryFrom_CorruptCacheFallsToSeed(t *testing.T) {
	dir := t.TempDir()
	cacheFile := filepath.Join(dir, "cache.json")
	if err := os.WriteFile(cacheFile, []byte(""), 0600); err != nil { // zero-byte file
		t.Fatal(err)
	}
	seed := testhub.HistoryStore{Snapshots: []testhub.DaySnapshot{{Date: "2026-03-01"}}}
	seedFile := writeTempFile(t, dir, "seed.json", seed)

	got, err := loadTesthubHistoryFrom(cacheFile, seedFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Snapshots) != 1 {
		t.Errorf("want 1 snapshot from seed (cache was corrupt), got %d", len(got.Snapshots))
	}
}

// ── Per-image repo set tests ─────────────────────────────────────────────────

// TestBluefinReposMatchDefault ensures BluefinRepos is kept in sync with DefaultRepos.
func TestBluefinReposMatchDefault(t *testing.T) {
	if len(builds.BluefinRepos) != len(builds.DefaultRepos) {
		t.Errorf("BluefinRepos has %d repos, DefaultRepos has %d — they should match",
			len(builds.BluefinRepos), len(builds.DefaultRepos))
	}
	for i, r := range builds.BluefinRepos {
		d := builds.DefaultRepos[i]
		if r.Owner != d.Owner || r.Repo != d.Repo {
			t.Errorf("BluefinRepos[%d] = %s/%s, want %s/%s", i, r.Owner, r.Repo, d.Owner, d.Repo)
		}
	}
}

// TestAuroraReposExcludeAuroraTest ensures aurora-test is not in AuroraRepos
// (it mirrors runs and causes duplicate noise).
func TestAuroraReposExcludeAuroraTest(t *testing.T) {
	for _, r := range builds.AuroraRepos {
		if r.Owner == "get-aurora-dev" && r.Repo == "aurora-test" {
			t.Error("AuroraRepos must not include get-aurora-dev/aurora-test (mirrors duplicate runs)")
		}
	}
}

// TestAuroraReposHasExpectedOrgs confirms get-aurora-dev repos are present.
func TestAuroraReposHasExpectedOrgs(t *testing.T) {
	hasUblue := false
	hasAuroraDev := false
	for _, r := range builds.AuroraRepos {
		if r.Owner == "ublue-os" {
			hasUblue = true
		}
		if r.Owner == "get-aurora-dev" {
			hasAuroraDev = true
		}
	}
	if !hasUblue {
		t.Error("AuroraRepos must include at least one ublue-os repo (aurora)")
	}
	if !hasAuroraDev {
		t.Error("AuroraRepos must include at least one get-aurora-dev repo")
	}
}

// TestBazziteReposHasBazziteRepo ensures the core bazzite repo is present.
func TestBazziteReposHasBazziteRepo(t *testing.T) {
	found := false
	for _, r := range builds.BazziteRepos {
		if r.Owner == "ublue-os" && r.Repo == "bazzite" {
			found = true
			break
		}
	}
	if !found {
		t.Error("BazziteRepos must include ublue-os/bazzite")
	}
}

// TestRepoSetsAreNonEmpty ensures none of the image repo sets are empty.
func TestRepoSetsAreNonEmpty(t *testing.T) {
	cases := []struct {
		name  string
		repos []builds.RepoConfig
	}{
		{"BluefinRepos", builds.BluefinRepos},
		{"AuroraRepos", builds.AuroraRepos},
		{"BazziteRepos", builds.BazziteRepos},
	}
	for _, tc := range cases {
		if len(tc.repos) == 0 {
			t.Errorf("%s must not be empty", tc.name)
		}
	}
}

func TestBuildActiveHumanLogins_IncludesIssueAndDiscussionOnly(t *testing.T) {
	commits := map[string]int{
		"alice":         3,
		"renovate[bot]": 2,
	}
	issues := map[string]int{
		"bob": 1,
	}
	discussions := map[string]int{
		"carol":               4,
		"github-actions[bot]": 1,
	}

	got := buildActiveHumanLogins(commits, issues, discussions)
	sort.Strings(got)
	want := []string{"alice", "bob", "carol"}

	if len(got) != len(want) {
		t.Fatalf("want %d logins, got %d (%v)", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("want %v, got %v", want, got)
		}
	}
}

func TestBuildActiveHumanLogins_EmptyInputReturnsEmptySlice(t *testing.T) {
	got := buildActiveHumanLogins(nil, nil, nil)
	if got == nil {
		t.Fatal("want non-nil slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("want 0 logins, got %d", len(got))
	}
}
