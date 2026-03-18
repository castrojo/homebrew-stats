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

// TapSnapshot records traffic for one tap at a point in time.
type TapSnapshot struct {
	Uniques int `json:"uniques"`
	Count   int `json:"count"`
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
