package osanalytics

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const baseURL = "https://formulae.brew.sh/api/analytics/os-version"

// OSEntry is a single OS with aggregated counts.
type OSEntry struct {
	OS      string  `json:"os"`
	Count   int64   `json:"count"`
	Percent float64 `json:"percent"`
}

// PeriodData holds the top-10 Linux OS entries for one time period.
type PeriodData struct {
	Period string    `json:"period"` // "30d", "90d", "365d"
	Items  []OSEntry `json:"items"`
}

// Analytics holds OS analytics for all periods.
type Analytics struct {
	Periods []PeriodData `json:"periods"`
}

// reVersionSuffix strips trailing version info to get the base OS name.
// Examples:
//
//	"Ubuntu 24.04 LTS" → "Ubuntu"
//	"Fedora Linux 40"  → "Fedora Linux"
//	"macOS Sequoia (15)" → "macOS Sequoia" (but macOS is filtered out)
var reVersionSuffix = regexp.MustCompile(`\s+\d[\d.]*(\s+LTS)?(\s+\(\d+\))?$`)
var reParenVersion = regexp.MustCompile(`\s*\(\d+\)$`)

func baseName(osVersion string) string {
	s := reVersionSuffix.ReplaceAllString(strings.TrimSpace(osVersion), "")
	s = reParenVersion.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func isMacOS(osVersion string) bool {
	return strings.Contains(osVersion, "macOS") || strings.Contains(osVersion, "OS X")
}

// Fetch retrieves and processes OS analytics for a given period ("30d", "90d", "365d").
// Returns top-10 Linux OSes with versions combined (e.g., Ubuntu 22 + Ubuntu 24 = Ubuntu).
func Fetch(period string) (*PeriodData, error) {
	url := fmt.Sprintf("%s/%s.json", baseURL, period)
	resp, err := http.Get(url) //nolint:gosec // URL is constructed from allowlisted base
	if err != nil {
		return nil, fmt.Errorf("fetching OS analytics (%s): %w", period, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OS analytics (%s): HTTP %d", period, resp.StatusCode)
	}

	var payload struct {
		Items []struct {
			OSVersion string `json:"os_version"`
			Count     string `json:"count"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decoding OS analytics (%s): %w", period, err)
	}

	// Aggregate by base OS name, Linux only.
	aggregated := make(map[string]int64)
	for _, item := range payload.Items {
		if isMacOS(item.OSVersion) {
			continue
		}
		name := baseName(item.OSVersion)
		countStr := strings.ReplaceAll(item.Count, ",", "")
		count, _ := strconv.ParseInt(countStr, 10, 64)
		aggregated[name] += count
	}

	// Sort by count descending.
	type kv struct {
		name  string
		count int64
	}
	var sorted []kv
	var total int64
	for name, count := range aggregated {
		sorted = append(sorted, kv{name, count})
		total += count
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].count > sorted[j].count })

	if len(sorted) > 10 {
		sorted = sorted[:10]
	}

	items := make([]OSEntry, 0, len(sorted))
	for _, entry := range sorted {
		pct := 0.0
		if total > 0 {
			pct = math.Round(float64(entry.count)/float64(total)*100*100) / 100
		}
		items = append(items, OSEntry{
			OS:      entry.name,
			Count:   entry.count,
			Percent: pct,
		})
	}

	return &PeriodData{Period: period, Items: items}, nil
}
