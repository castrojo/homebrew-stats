package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

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
	store := testhub.HistoryStore{Snapshots: []testhub.DaySnapshot{{Date: "2026-01-01"}}}
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

// TestLoadTesthubHistoryFrom_EmptyCacheFallsToSeed verifies that when the cache
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
