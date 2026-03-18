// Package history manages the accumulated traffic snapshot store.
// Snapshots are persisted in .sync-cache/history.json via GitHub Actions cache,
// giving us a growing time-series that outlasts the API's 14-day rolling window.
package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const cacheFile = ".sync-cache/history.json"

// TapSnapshot records traffic and per-package downloads for one tap at a point in time.
type TapSnapshot struct {
	Uniques   int               `json:"uniques"`
	Count     int               `json:"count"`
	Downloads map[string]int64  `json:"downloads,omitempty"` // package name → 30d installs
}

// DaySnapshot records all tap traffic for a single calendar date.
type DaySnapshot struct {
	Date string                 `json:"date"` // YYYY-MM-DD
	Taps map[string]TapSnapshot `json:"taps"`
}

// Store holds the full history and manages persistence.
type Store struct {
	Snapshots []DaySnapshot `json:"snapshots"`
}

// Load reads the history from .sync-cache/history.json.
// Returns an empty store if the file does not exist.
func Load() (*Store, error) {
	data, err := os.ReadFile(cacheFile)
	if os.IsNotExist(err) {
		return &Store{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading history cache: %w", err)
	}
	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  history.json is corrupt, starting fresh: %v\n", err)
		return &Store{}, nil
	}
	return &s, nil
}

// LoadWithBootstrap reads history from the cache and, if the cache has fewer
// snapshots than the committed stats.json at fallbackPath, merges them.
// This ensures that history seeded into the committed stats.json (e.g. via a
// "seed" commit) is not silently discarded when the CI cache starts fresh.
//
// Merge semantics:
//   - Cache data takes precedence for any date that appears in both sources.
//   - Dates present only in stats.json are added from stats.json.
//   - Result is sorted chronologically.
func LoadWithBootstrap(fallbackPath string) (*Store, error) {
	cache, err := Load()
	if err != nil {
		return nil, err
	}

	// Parse just the history array from the stats.json file.
	statsSnaps, err := readHistoryFromStatsJSON(fallbackPath)
	if err != nil || len(statsSnaps) <= len(cache.Snapshots) {
		// Cache is at least as large, or stats.json is missing/unreadable — use cache as-is.
		return cache, nil
	}

	// Merge: build a map of cache snapshots keyed by date (cache wins on conflicts).
	byDate := make(map[string]DaySnapshot, len(cache.Snapshots))
	for _, s := range cache.Snapshots {
		byDate[s.Date] = s
	}
	// Add stats.json snapshots only for dates not already in cache.
	for _, s := range statsSnaps {
		if _, exists := byDate[s.Date]; !exists {
			byDate[s.Date] = s
		}
	}

	merged := make([]DaySnapshot, 0, len(byDate))
	for _, s := range byDate {
		merged = append(merged, s)
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Date < merged[j].Date
	})

	fmt.Fprintf(os.Stderr, "→ History: bootstrapped %d snapshots from %s (cache had %d)\n",
		len(merged), fallbackPath, len(cache.Snapshots))
	return &Store{Snapshots: merged}, nil
}

// readHistoryFromStatsJSON extracts just the history array from a stats.json file.
func readHistoryFromStatsJSON(path string) ([]DaySnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var statsFile struct {
		History []DaySnapshot `json:"history"`
	}
	if err := json.Unmarshal(data, &statsFile); err != nil {
		return nil, fmt.Errorf("parsing history from %s: %w", path, err)
	}
	return statsFile.History, nil
}

// Append adds a new snapshot for today if one doesn't already exist.
// Idempotent: repeated runs on the same day are no-ops.
func (s *Store) Append(taps map[string]TapSnapshot) {
	today := time.Now().UTC().Format("2006-01-02")
	for _, snap := range s.Snapshots {
		if snap.Date == today {
			fmt.Fprintf(os.Stderr, "→ History: snapshot for %s already exists, skipping\n", today)
			return
		}
	}
	s.Snapshots = append(s.Snapshots, DaySnapshot{
		Date: today,
		Taps: taps,
	})
	// Keep chronological order.
	sort.Slice(s.Snapshots, func(i, j int) bool {
		return s.Snapshots[i].Date < s.Snapshots[j].Date
	})
	fmt.Fprintf(os.Stderr, "→ History: appended snapshot for %s (%d total)\n", today, len(s.Snapshots))
}

// Save writes the store back to .sync-cache/history.json.
func (s *Store) Save() error {
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0o755); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling history: %w", err)
	}
	if err := os.WriteFile(cacheFile, data, 0o644); err != nil {
		return fmt.Errorf("writing history cache: %w", err)
	}
	return nil
}
