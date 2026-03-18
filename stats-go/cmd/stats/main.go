package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	ghclient "github.com/castrojo/homebrew-stats/internal/github"
	"github.com/castrojo/homebrew-stats/internal/history"
	"github.com/castrojo/homebrew-stats/internal/osanalytics"
	"github.com/castrojo/homebrew-stats/internal/tap"
)

// Output is the full JSON written to src/data/stats.json.
type Output struct {
	GeneratedAt string                 `json:"generated_at"`
	Taps        []tap.TapStats         `json:"taps"`
	History     []history.DaySnapshot  `json:"history"`
	OSAnalytics *osanalytics.Analytics `json:"os_analytics,omitempty"`
}

// Taps to track, in display order.
var taps = []struct{ owner, repo string }{
	{"ublue-os", "homebrew-tap"},
	{"ublue-os", "homebrew-experimental-tap"},
}

func main() {
	client, err := ghclient.NewClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "❌ "+err.Error())
		os.Exit(1)
	}

	// Load historical data from cache.
	hist, err := history.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Could not load history: %v\n", err)
		hist = &history.Store{}
	}

	// Collect data for each tap.
	tapStats := make([]tap.TapStats, 0, len(taps))
	todayTaps := make(map[string]history.TapSnapshot)

	for _, t := range taps {
		fmt.Fprintf(os.Stderr, "→ Collecting %s/%s…\n", t.owner, t.repo)
		ts, err := tap.Collect(t.owner, t.repo, client)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  %s/%s: %v\n", t.owner, t.repo, err)
			continue
		}
		tapStats = append(tapStats, *ts)
		if ts.Traffic != nil {
			pkgDownloads := make(map[string]int64, len(ts.Packages))
			for _, pkg := range ts.Packages {
				if pkg.Downloads > 0 {
					pkgDownloads[pkg.Name] = pkg.Downloads
				}
			}
			todayTaps[ts.Name] = history.TapSnapshot{
				Uniques:   ts.Traffic.Uniques,
				Count:     ts.Traffic.Count,
				Downloads: pkgDownloads,
			}
		}
	}

	// Append today's snapshot and save cache.
	if len(todayTaps) > 0 {
		hist.Append(todayTaps)
		if err := hist.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Could not save history: %v\n", err)
		}
	}

	// Fetch OS analytics from Homebrew (public API, no auth required).
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

	// Write src/data/stats.json.
	out := Output{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Taps:        tapStats,
		History:     hist.Snapshots,
		OSAnalytics: osData,
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "❌ JSON marshal:", err)
		os.Exit(1)
	}

	outPath := filepath.Join("src", "data", "stats.json")
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "❌ mkdir:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "❌ write:", err)
		os.Exit(1)
	}

	// Backup stats.json to cache for fallback builds.
	backupPath := filepath.Join(".sync-cache", "stats-latest.json")
	if err := os.WriteFile(backupPath, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Could not write stats backup: %v\n", err)
		// Non-fatal — don't exit
	} else {
		fmt.Fprintln(os.Stderr, "✓ Backed up stats to", backupPath)
	}

	// Summary to stderr.
	fmt.Fprintln(os.Stderr, "✓ Wrote", outPath)
	for _, ts := range tapStats {
		if ts.Traffic != nil {
			fmt.Fprintf(os.Stderr, "  %s: %d unique tappers, %d packages\n",
				ts.Name, ts.Traffic.Uniques, len(ts.Packages))
		}
	}
	fmt.Fprintf(os.Stderr, "  History: %d snapshots\n", len(hist.Snapshots))
}
