// Package metrics computes derived statistics from raw tap and history data.
// All computations are pure functions — they read input, return output, and
// have no side effects, making them trivially testable.
package metrics

import (
	"math"
	"sort"

	"github.com/castrojo/homebrew-stats/internal/history"
	"github.com/castrojo/homebrew-stats/internal/tap"
)

// ---------------------------------------------------------------------------
// velocity7d
// ---------------------------------------------------------------------------

// Velocity7d returns the average daily install momentum for a package over the
// last 7 days, expressed as a 1-decimal float64.
//
// Definition:
//
//	velocity7d = (snap[n].downloads[pkg] - snap[n-7].downloads[pkg]) / 7
//
// Only snapshots that contain download data for the package are considered.
// Returns 0 when:
//   - fewer than 8 qualifying snapshots exist
//   - the delta would be negative (rolling-window roll-off)
func Velocity7d(snapshots []history.DaySnapshot, tapName, pkgName string) float64 {
	vals := packageDownloadSeries(snapshots, tapName, pkgName)
	if len(vals) < 8 {
		return 0
	}
	n := len(vals) - 1
	delta := float64(vals[n] - vals[n-7])
	if delta < 0 {
		delta = 0
	}
	return math.Round(delta/7*10) / 10
}

// packageDownloadSeries returns the chronological sequence of download counts
// for a given package within a tap, skipping snapshots where data is absent.
func packageDownloadSeries(snapshots []history.DaySnapshot, tapName, pkgName string) []int64 {
	out := make([]int64, 0, len(snapshots))
	for _, snap := range snapshots {
		tapSnap, ok := snap.Taps[tapName]
		if !ok {
			continue
		}
		dl, ok := tapSnap.Downloads[pkgName]
		if !ok {
			continue
		}
		out = append(out, dl)
	}
	return out
}

// ---------------------------------------------------------------------------
// growth_pct
// ---------------------------------------------------------------------------

// GrowthPct returns the week-over-week percentage change in total downloads for
// a tap, or nil when insufficient history is available.
//
// Definition:
//
//	growth_pct = (sum[n] - sum[n-7]) / sum[n-7] * 100
//
// where sum[n] is the sum of all package downloads for the tap at snapshot n.
// Only snapshots that contain download data for the tap are considered.
// Returns nil when:
//   - fewer than 8 qualifying snapshots exist
//   - sum[n-7] is 0 (would produce division-by-zero)
func GrowthPct(snapshots []history.DaySnapshot, tapName string) *float64 {
	sums := tapDownloadSumSeries(snapshots, tapName)
	if len(sums) < 8 {
		return nil
	}
	n := len(sums) - 1
	sumN := float64(sums[n])
	sumN7 := float64(sums[n-7])
	if sumN7 == 0 {
		return nil
	}
	pct := math.Round((sumN-sumN7)/sumN7*100*10) / 10
	return &pct
}

// tapDownloadSumSeries returns the chronological sequence of total download
// sums for a tap, one entry per snapshot that has download data for that tap.
func tapDownloadSumSeries(snapshots []history.DaySnapshot, tapName string) []int64 {
	out := make([]int64, 0, len(snapshots))
	for _, snap := range snapshots {
		tapSnap, ok := snap.Taps[tapName]
		if !ok || len(tapSnap.Downloads) == 0 {
			continue
		}
		var sum int64
		for _, dl := range tapSnap.Downloads {
			sum += dl
		}
		out = append(out, sum)
	}
	return out
}

// ---------------------------------------------------------------------------
// Summary
// ---------------------------------------------------------------------------

// Summary holds aggregate KPIs precomputed for hero / dashboard cards.
type Summary struct {
	TotalInstalls30d      int64    `json:"total_installs_30d"`
	TotalInstalls90d      int64    `json:"total_installs_90d"`
	TotalInstalls365d     int64    `json:"total_installs_365d"`
	TotalUniqueTappers    int      `json:"total_unique_tappers"`
	TotalPackages         int      `json:"total_packages"`
	StaleCount            int      `json:"stale_count"`
	FreshCount            int      `json:"fresh_count"`
	UnknownFreshnessCount int      `json:"unknown_freshness_count"`
	WoWGrowthPct          *float64 `json:"wow_growth_pct"`
}

// ComputeSummary builds the Summary from the collected tap data and history.
func ComputeSummary(taps []tap.TapStats, snapshots []history.DaySnapshot) Summary {
	var s Summary
	for _, t := range taps {
		s.TotalPackages += len(t.Packages)
		for _, pkg := range t.Packages {
			s.TotalInstalls30d += pkg.Downloads
			s.TotalInstalls90d += pkg.Installs90d
			s.TotalInstalls365d += pkg.Installs365d
			switch pkg.StatusString() {
			case "stale":
				s.StaleCount++
			case "current":
				s.FreshCount++
			default:
				s.UnknownFreshnessCount++
			}
		}
		if t.Traffic != nil {
			s.TotalUniqueTappers += t.Traffic.Uniques
		}
	}
	s.WoWGrowthPct = aggregateGrowthPct(snapshots)
	return s
}

// aggregateGrowthPct computes WoW % change across ALL taps combined.
// Per snapshot, it sums downloads from every tap present in that snapshot.
// Returns nil when fewer than 8 qualifying snapshots exist or when the
// baseline sum is zero.
func aggregateGrowthPct(snapshots []history.DaySnapshot) *float64 {
	var sums []int64
	for _, snap := range snapshots {
		var sum int64
		hasData := false
		for _, tapSnap := range snap.Taps {
			if len(tapSnap.Downloads) > 0 {
				hasData = true
			}
			for _, dl := range tapSnap.Downloads {
				sum += dl
			}
		}
		if hasData {
			sums = append(sums, sum)
		}
	}
	if len(sums) < 8 {
		return nil
	}
	n := len(sums) - 1
	sumN := float64(sums[n])
	sumN7 := float64(sums[n-7])
	if sumN7 == 0 {
		return nil
	}
	pct := math.Round((sumN-sumN7)/sumN7*100*10) / 10
	return &pct
}

// ---------------------------------------------------------------------------
// top_packages
// ---------------------------------------------------------------------------

// PackageHistoryPoint is a single date → downloads entry in a package's history.
type PackageHistoryPoint struct {
	Date      string `json:"date"`
	Downloads int64  `json:"downloads"`
}

// TopPackage holds the history series for one of the top-10 packages.
type TopPackage struct {
	Name    string                `json:"name"`
	Tap     string                `json:"tap"`
	History []PackageHistoryPoint `json:"history"`
}

// ComputeTopPackages returns the top 10 packages by current (latest) downloads
// across all taps, each with their full history series.
func ComputeTopPackages(taps []tap.TapStats, snapshots []history.DaySnapshot) []TopPackage {
	type entry struct {
		name      string
		tapName   string
		downloads int64
	}

	all := make([]entry, 0, 64)
	for _, t := range taps {
		for _, pkg := range t.Packages {
			all = append(all, entry{pkg.Name, t.Name, pkg.Downloads})
		}
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].downloads > all[j].downloads
	})
	if len(all) > 10 {
		all = all[:10]
	}

	result := make([]TopPackage, 0, len(all))
	for _, e := range all {
		hist := buildPackageHistory(snapshots, e.tapName, e.name)
		result = append(result, TopPackage{
			Name:    e.name,
			Tap:     e.tapName,
			History: hist,
		})
	}
	return result
}

// buildPackageHistory assembles the chronological download series for a single
// package, omitting snapshots where the package has no recorded data.
func buildPackageHistory(snapshots []history.DaySnapshot, tapName, pkgName string) []PackageHistoryPoint {
	out := make([]PackageHistoryPoint, 0, len(snapshots))
	for _, snap := range snapshots {
		tapSnap, ok := snap.Taps[tapName]
		if !ok {
			continue
		}
		dl, ok := tapSnap.Downloads[pkgName]
		if !ok {
			continue
		}
		out = append(out, PackageHistoryPoint{Date: snap.Date, Downloads: dl})
	}
	return out
}
