package countme

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// countmeCSVURL is the source for weekly countme data.
// IMPORTANT: The CSV lives on the Fedora data-analysis server, NOT in the ublue-os/countme
// GitHub repo (which does not contain totals.csv — that path returns 404).
// Confirmed correct URL via: curl -sI https://data-analysis.fedoraproject.org/csv-reports/countme/totals.csv
const countmeCSVURL = "https://data-analysis.fedoraproject.org/csv-reports/countme/totals.csv"

// validDistros maps exact os_name values (from CSV) to canonical keys used in output JSON.
// These are filtered to fedora-NN canonical repo_tags only (see isCanonicalRepoTag).
//
// Bluefin LTS is intentionally excluded: it uses CentOS/EPEL repos (epel-10, epel-debug-10, etc.)
// rather than fedora-NN repos, so the fedora-NN filter produces zero hits for it.
// The ublue-os/countme repo notes "centos countme data is broken" and has disabled LTS from
// its badge generation. We follow the same decision — LTS countme data is publicly marked broken.
//
// BlueBuildOS is excluded: its countme participation collapsed 99.7% in the week of 2026-02-08
// (from ~484 to 1–2 hits/week), consistent with a reporting infrastructure change rather than
// user loss. Re-add when the project restores countme reporting.
var validDistros = map[string]string{
	"Bazzite":    "bazzite",
	"Bluefin":    "bluefin",
	"Aurora":     "aurora",
	"secureblue": "secureblue",
	"wayblue":    "wayblue",
}

// canonicalRepoTagRe matches Fedora canonical repo tags of the form "fedora-NN" (e.g. fedora-41,
// fedora-42). This is the methodology used by ublue-os/countme to count unique devices: each
// machine checks in once per enabled repo per week, and all machines have the canonical fedora-NN
// repo. Summing all repo_tags (updates-released-fNN, fedora-cisco-openh264-NN, etc.) inflates
// counts ~2x. Filtering to fedora-NN only gives the closest approximation to unique device count.
var canonicalRepoTagRe = regexp.MustCompile(`^fedora-\d+$`)

// anomalousWeekEnds lists week_end dates to exclude from aggregation.
// These are known data quality issues identified by ublue-os/countme:
//   - 2024-12-29: partial year-end week (incomplete data)
//   - 2025-07-06: Fedora infrastructure migration caused a ~40% artificial drop
var anomalousWeekEnds = map[string]bool{
	"2024-12-29": true,
	"2025-07-06": true,
}

// isCanonicalRepoTag returns true if the repo_tag is a canonical Fedora repo (fedora-NN).
func isCanonicalRepoTag(repoTag string) bool {
	return canonicalRepoTagRe.MatchString(repoTag)
}

// csvRow holds a single parsed row from the countme CSV.
type csvRow struct {
	weekStart string
	weekEnd   string
	osName    string
	osVersion string
	sysAge    string
	repoTag   string
	hits      int
}

// parseOsVersionDist extracts os_name → os_version → count from CSV rows.
// Only includes the 4 target distros; skips rows with sys_age == "-1" or empty os_version.
func parseOsVersionDist(rows []csvRow) map[string]map[string]int {
	result := make(map[string]map[string]int)
	for _, row := range rows {
		if row.sysAge == "-1" {
			continue
		}
		if _, ok := validDistros[row.osName]; !ok {
			continue
		}
		if row.osVersion == "" {
			continue
		}
		if result[row.osName] == nil {
			result[row.osName] = make(map[string]int)
		}
		result[row.osName][row.osVersion] += row.hits
	}
	return result
}

// MergeOsVersionDist replaces per-distro version data with new data (not additive).
// New data wins for each distro present in newData — this is a current-snapshot view.
func MergeOsVersionDist(existing, newData map[string]map[string]int) map[string]map[string]int {
	result := make(map[string]map[string]int, len(existing))
	for distro, versions := range existing {
		cp := make(map[string]int, len(versions))
		for ver, cnt := range versions {
			cp[ver] = cnt
		}
		result[distro] = cp
	}
	// New data overwrites entire distro entries.
	for distro, versions := range newData {
		cp := make(map[string]int, len(versions))
		for ver, cnt := range versions {
			cp[ver] = cnt
		}
		result[distro] = cp
	}
	return result
}

// parseCSVRows parses raw CSV bytes into a slice of csvRow structs.
// It tolerates a partial first line (from an HTTP Range request) by locating the header row.
func parseCSVRows(body []byte) ([]csvRow, error) {
	lines := strings.Split(string(body), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("CSV too short")
	}

	// Find the header line (starts with "week_start").
	headerIdx := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "week_start") {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		return nil, fmt.Errorf("CSV header not found")
	}

	csvContent := strings.Join(lines[headerIdx:], "\n")
	r := csv.NewReader(strings.NewReader(csvContent))
	headers, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read CSV header: %w", err)
	}

	// Build column index map.
	colIdx := make(map[string]int, len(headers))
	for i, h := range headers {
		colIdx[strings.TrimSpace(h)] = i
	}

	required := []string{"week_start", "week_end", "os_name", "sys_age", "hits", "repo_tag"}
	for _, col := range required {
		if _, ok := colIdx[col]; !ok {
			return nil, fmt.Errorf("missing CSV column: %s", col)
		}
	}

	// os_version is optional; track whether it exists.
	osVersionIdx, hasOsVersion := colIdx["os_version"]

	var rows []csvRow
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // skip malformed rows
		}

		osVersion := ""
		if hasOsVersion && osVersionIdx < len(row) {
			osVersion = strings.TrimSpace(row[osVersionIdx])
		}

		hitsStr := strings.TrimSpace(row[colIdx["hits"]])
		hits, err := strconv.Atoi(hitsStr)
		if err != nil {
			continue
		}

		rows = append(rows, csvRow{
			weekStart: strings.TrimSpace(row[colIdx["week_start"]]),
			weekEnd:   strings.TrimSpace(row[colIdx["week_end"]]),
			osName:    row[colIdx["os_name"]],
			osVersion: osVersion,
			sysAge:    strings.TrimSpace(row[colIdx["sys_age"]]),
			repoTag:   strings.TrimSpace(row[colIdx["repo_tag"]]),
			hits:      hits,
		})
	}
	return rows, nil
}

// rowsToWeekRecords aggregates csvRows into WeekRecords using the ublue-os/countme methodology:
//   - Only counts rows where repo_tag matches ^fedora-\d+$ (canonical Fedora repo).
//     Each device checks in once per enabled repo per week; filtering to the canonical
//     fedora-NN repo gives one hit per device per week (vs ~2x if all repo_tags are summed).
//   - Excludes rows where sys_age == "-1" (new/reconfigured systems, not steady-state users).
//   - Excludes anomalous weeks listed in anomalousWeekEnds (infrastructure events).
func rowsToWeekRecords(rows []csvRow) []WeekRecord {
	type weekKey struct{ start, end string }
	agg := make(map[weekKey]map[string]int)

	for _, row := range rows {
		if row.sysAge == "-1" {
			continue
		}
		if anomalousWeekEnds[row.weekEnd] {
			continue
		}
		if !isCanonicalRepoTag(row.repoTag) {
			continue
		}
		distroKey, ok := parseDistroName(row.osName)
		if !ok {
			continue
		}
		wk := weekKey{row.weekStart, row.weekEnd}
		if agg[wk] == nil {
			agg[wk] = make(map[string]int)
		}
		agg[wk][distroKey] += row.hits
	}

	records := make([]WeekRecord, 0, len(agg))
	for wk, distros := range agg {
		total := 0
		for _, v := range distros {
			total += v
		}
		records = append(records, WeekRecord{
			WeekStart: wk.start,
			WeekEnd:   wk.end,
			Distros:   distros,
			Total:     total,
		})
	}
	return records
}

// parseDistroName does an exact-match lookup of os_name against the allowed distros.
// Returns the canonical key and true if matched.
func parseDistroName(osName string) (string, bool) {
	key, ok := validDistros[osName]
	return key, ok
}

// fetchCSVFromURL fetches and parses the countme CSV from the given URL.
// Returns week records aggregated by (week_start, week_end) and an os_version
// distribution map (os_name → os_version → hits).
func fetchCSVFromURL(url string) ([]WeekRecord, map[string]map[string]int, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}
	// Request only the last ~10 MB to avoid downloading the full CSV.
	req.Header.Set("Range", "bytes=-10000000")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("GET countme CSV: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read CSV body: %w", err)
	}

	rows, err := parseCSVRows(body)
	if err != nil {
		return nil, nil, err
	}

	weekRecords := rowsToWeekRecords(rows)
	osVersionDist := parseOsVersionDist(rows)
	return weekRecords, osVersionDist, nil
}

// FetchCSVLast30Days fetches and parses the countme CSV using the default URL.
// Returns week records and the os_version distribution (os_name → os_version → hits).
func FetchCSVLast30Days() ([]WeekRecord, map[string]map[string]int, error) {
	return fetchCSVFromURL(countmeCSVURL)
}

// MergeIntoHistory merges new week records into the store, deduplicating by week_start.
// Last write wins on conflict.
func MergeIntoHistory(store *HistoryStore, csvRecs []WeekRecord) *HistoryStore {
	// Build map of existing records by week_start
	byWeek := make(map[string]WeekRecord, len(store.WeekRecords))
	for _, rec := range store.WeekRecords {
		byWeek[rec.WeekStart] = rec
	}
	// Merge: new records overwrite existing
	for _, rec := range csvRecs {
		byWeek[rec.WeekStart] = rec
	}
	// Convert back to slice
	result := &HistoryStore{
		WeekRecords: make([]WeekRecord, 0, len(byWeek)),
		DayRecords:  store.DayRecords,
	}
	for _, rec := range byWeek {
		result.WeekRecords = append(result.WeekRecords, rec)
	}
	return result
}
