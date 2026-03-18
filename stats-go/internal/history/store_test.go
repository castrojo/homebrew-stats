package history

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

// today returns the current UTC date in YYYY-MM-DD format, matching Store.Append behaviour.
func today() string {
	return time.Now().UTC().Format("2006-01-02")
}

func TestStoreAppend_AddsSnapshot(t *testing.T) {
	s := &Store{}
	taps := map[string]TapSnapshot{
		"ublue-os/homebrew-tap": {Count: 10, Uniques: 5},
	}
	s.Append(taps)

	if len(s.Snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(s.Snapshots))
	}
	if s.Snapshots[0].Date != today() {
		t.Errorf("Date = %q, want %q", s.Snapshots[0].Date, today())
	}
	tap := s.Snapshots[0].Taps["ublue-os/homebrew-tap"]
	if tap.Count != 10 || tap.Uniques != 5 {
		t.Errorf("Tap snapshot = %+v, want Count=10 Uniques=5", tap)
	}
}

func TestStoreAppend_Idempotent(t *testing.T) {
	s := &Store{}
	taps := map[string]TapSnapshot{
		"ublue-os/homebrew-tap": {Count: 10, Uniques: 5},
	}
	s.Append(taps)
	// Second Append on the same day must be a no-op.
	s.Append(taps)

	if len(s.Snapshots) != 1 {
		t.Fatalf("expected 1 snapshot after double Append on same day, got %d", len(s.Snapshots))
	}
}

func TestStoreAppend_ChronologicalOrder(t *testing.T) {
	// Seed with two past snapshots that are out of order.
	s := &Store{
		Snapshots: []DaySnapshot{
			{Date: "2026-01-15", Taps: map[string]TapSnapshot{}},
			{Date: "2024-12-31", Taps: map[string]TapSnapshot{}}, // older — out of order
		},
	}
	// Append today's snapshot; Append sorts the full slice after adding.
	s.Append(map[string]TapSnapshot{"tap": {Count: 1}})

	for i := 1; i < len(s.Snapshots); i++ {
		if s.Snapshots[i].Date < s.Snapshots[i-1].Date {
			t.Errorf("snapshots not in chronological order at index %d: %s before %s",
				i, s.Snapshots[i-1].Date, s.Snapshots[i].Date)
		}
	}
	// Also verify today is last (it's the most recent date).
	last := s.Snapshots[len(s.Snapshots)-1].Date
	if last != today() {
		t.Errorf("expected today (%s) to be last snapshot, got %s", today(), last)
	}
}

func TestStoreAppend_EmptyTaps(t *testing.T) {
	s := &Store{}
	s.Append(map[string]TapSnapshot{})
	if len(s.Snapshots) != 1 {
		t.Fatalf("expected 1 snapshot even for empty taps, got %d", len(s.Snapshots))
	}
	if s.Snapshots[0].Taps == nil {
		t.Error("Taps map must not be nil")
	}
}

// chdirTemp switches the working directory to a fresh temp dir for the duration of the test.
// Load and Save use a hardcoded relative path (.sync-cache/history.json), so we must chdir.
func chdirTemp(t *testing.T) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

func TestLoad_ReturnsEmptyStoreWhenFileAbsent(t *testing.T) {
	chdirTemp(t)
	s, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error for missing file: %v", err)
	}
	if len(s.Snapshots) != 0 {
		t.Errorf("expected 0 snapshots for fresh store, got %d", len(s.Snapshots))
	}
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	chdirTemp(t)
	original := &Store{
		Snapshots: []DaySnapshot{
			{
				Date: "2026-01-01",
				Taps: map[string]TapSnapshot{
					"ublue-os/homebrew-tap": {Count: 42, Uniques: 7, Downloads: map[string]int64{"pkg": 100}},
				},
			},
		},
	}
	if err := original.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error after Save(): %v", err)
	}
	if len(loaded.Snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(loaded.Snapshots))
	}
	snap := loaded.Snapshots[0]
	if snap.Date != "2026-01-01" {
		t.Errorf("Date = %q, want 2026-01-01", snap.Date)
	}
	tap := snap.Taps["ublue-os/homebrew-tap"]
	if tap.Count != 42 || tap.Uniques != 7 {
		t.Errorf("TapSnapshot = %+v, want Count=42 Uniques=7", tap)
	}
	if tap.Downloads["pkg"] != 100 {
		t.Errorf("Downloads[pkg] = %d, want 100", tap.Downloads["pkg"])
	}
}

func TestLoad_HandlesCorruptJSON(t *testing.T) {
	chdirTemp(t)
	// Write deliberately corrupt JSON.
	if err := os.MkdirAll(".sync-cache", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(".sync-cache/history.json", []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	s, err := Load()
	if err != nil {
		t.Fatalf("Load() should not return an error for corrupt JSON, got: %v", err)
	}
	if len(s.Snapshots) != 0 {
		t.Errorf("expected empty store after corrupt JSON, got %d snapshots", len(s.Snapshots))
	}
}

func TestSave_WritesValidJSON(t *testing.T) {
	chdirTemp(t)
	s := &Store{}
	s.Append(map[string]TapSnapshot{"tap": {Count: 1, Uniques: 1}})
	if err := s.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	data, err := os.ReadFile(".sync-cache/history.json")
	if err != nil {
		t.Fatalf("ReadFile after Save(): %v", err)
	}
	var decoded Store
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("saved JSON is not valid: %v", err)
	}
	if len(decoded.Snapshots) != 1 {
		t.Errorf("expected 1 snapshot in saved JSON, got %d", len(decoded.Snapshots))
	}
}

// writeStatsJSON writes a minimal stats.json file with the given history to path.
func writeStatsJSON(t *testing.T, path string, snapshots []DaySnapshot) {
	t.Helper()
	type statsFile struct {
		History []DaySnapshot `json:"history"`
	}
	data, err := json.Marshal(statsFile{History: snapshots})
	if err != nil {
		t.Fatalf("marshal stats JSON: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write stats JSON: %v", err)
	}
}

func TestLoadWithBootstrap_UsesStatsWhenCacheEmpty(t *testing.T) {
	chdirTemp(t)
	// Write stats.json with 3 historical snapshots; no cache file.
	snaps := []DaySnapshot{
		{Date: "2026-01-01", Taps: map[string]TapSnapshot{"tap": {Uniques: 1, Downloads: map[string]int64{"pkg": 100}}}},
		{Date: "2026-01-02", Taps: map[string]TapSnapshot{"tap": {Uniques: 2, Downloads: map[string]int64{"pkg": 110}}}},
		{Date: "2026-01-03", Taps: map[string]TapSnapshot{"tap": {Uniques: 3, Downloads: map[string]int64{"pkg": 120}}}},
	}
	writeStatsJSON(t, "stats.json", snaps)

	s, err := LoadWithBootstrap("stats.json")
	if err != nil {
		t.Fatalf("LoadWithBootstrap error: %v", err)
	}
	if len(s.Snapshots) != 3 {
		t.Fatalf("expected 3 snapshots from bootstrap, got %d", len(s.Snapshots))
	}
	if s.Snapshots[0].Date != "2026-01-01" {
		t.Errorf("first snapshot date = %q, want 2026-01-01", s.Snapshots[0].Date)
	}
	// Verify downloads are preserved through bootstrap.
	dl := s.Snapshots[2].Taps["tap"].Downloads["pkg"]
	if dl != 120 {
		t.Errorf("Downloads[pkg] at day 3 = %d, want 120", dl)
	}
}

func TestLoadWithBootstrap_PrefersCacheWhenLarger(t *testing.T) {
	chdirTemp(t)
	// Cache has 5 snapshots; stats.json has only 3. Cache wins.
	cacheStore := &Store{
		Snapshots: []DaySnapshot{
			{Date: "2026-01-01", Taps: map[string]TapSnapshot{}},
			{Date: "2026-01-02", Taps: map[string]TapSnapshot{}},
			{Date: "2026-01-03", Taps: map[string]TapSnapshot{}},
			{Date: "2026-01-04", Taps: map[string]TapSnapshot{}},
			{Date: "2026-01-05", Taps: map[string]TapSnapshot{}},
		},
	}
	if err := cacheStore.Save(); err != nil {
		t.Fatalf("Save cache: %v", err)
	}
	statsSnaps := []DaySnapshot{
		{Date: "2026-01-01", Taps: map[string]TapSnapshot{}},
		{Date: "2026-01-02", Taps: map[string]TapSnapshot{}},
		{Date: "2026-01-03", Taps: map[string]TapSnapshot{}},
	}
	writeStatsJSON(t, "stats.json", statsSnaps)

	s, err := LoadWithBootstrap("stats.json")
	if err != nil {
		t.Fatalf("LoadWithBootstrap error: %v", err)
	}
	if len(s.Snapshots) != 5 {
		t.Errorf("expected 5 snapshots (from cache), got %d", len(s.Snapshots))
	}
}

func TestLoadWithBootstrap_MergesWhenStatsHasMoreHistory(t *testing.T) {
	chdirTemp(t)
	// Cache has 1 snapshot; stats.json has 3. Merged result has 3.
	cacheStore := &Store{
		Snapshots: []DaySnapshot{
			{Date: "2026-01-03", Taps: map[string]TapSnapshot{"tap": {Uniques: 99}}},
		},
	}
	if err := cacheStore.Save(); err != nil {
		t.Fatalf("Save cache: %v", err)
	}
	statsSnaps := []DaySnapshot{
		{Date: "2026-01-01", Taps: map[string]TapSnapshot{"tap": {Downloads: map[string]int64{"pkg": 50}}}},
		{Date: "2026-01-02", Taps: map[string]TapSnapshot{"tap": {Downloads: map[string]int64{"pkg": 60}}}},
		{Date: "2026-01-03", Taps: map[string]TapSnapshot{"tap": {Downloads: map[string]int64{"pkg": 70}}}},
	}
	writeStatsJSON(t, "stats.json", statsSnaps)

	s, err := LoadWithBootstrap("stats.json")
	if err != nil {
		t.Fatalf("LoadWithBootstrap error: %v", err)
	}
	if len(s.Snapshots) != 3 {
		t.Errorf("expected 3 snapshots (merged), got %d", len(s.Snapshots))
	}
	// Cache data for 2026-01-03 (Uniques=99) takes precedence over stats data.
	tap := s.Snapshots[2].Taps["tap"]
	if tap.Uniques != 99 {
		t.Errorf("Uniques for 2026-01-03 = %d, want 99 (cache wins for duplicate dates)", tap.Uniques)
	}
}

func TestLoadWithBootstrap_MissingStatsFileIsNotAnError(t *testing.T) {
	chdirTemp(t)
	// Neither cache nor stats.json exists — should return empty store, no error.
	s, err := LoadWithBootstrap("nonexistent-stats.json")
	if err != nil {
		t.Fatalf("LoadWithBootstrap should not error when stats file missing, got: %v", err)
	}
	if len(s.Snapshots) != 0 {
		t.Errorf("expected 0 snapshots, got %d", len(s.Snapshots))
	}
}
