package countme

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Badge endpoint base URL pattern.
const badgeBaseURL = "https://raw.githubusercontent.com/ublue-os/countme/main/badge-endpoints"

// countmeCSVURL is the source for weekly countme data.
const countmeCSVURL = "https://raw.githubusercontent.com/ublue-os/countme/main/totals.csv"

// validDistros maps exact os_name values (from CSV) to canonical keys used in output JSON.
var validDistros = map[string]string{
	"Bazzite":     "bazzite",
	"Bluefin":     "bluefin",
	"Bluefin LTS": "bluefin-lts",
	"Aurora":      "aurora",
}

// badgeNames lists the distros with badge endpoints (ublue-os hosted only).
var badgeNames = []string{"bazzite", "bluefin", "bluefin-lts", "aurora"}

// parseBadgeValue converts a badge message string like "71k", "3.6k", "1.2M", "64"
// into an integer count.
func parseBadgeValue(s string) (int, error) {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return 0, fmt.Errorf("empty badge value")
	}

	last := s[len(s)-1]
	switch last {
	case 'k', 'K':
		f, err := strconv.ParseFloat(s[:len(s)-1], 64)
		if err != nil {
			return 0, fmt.Errorf("parse badge %q: %w", s, err)
		}
		return int(math.Round(f * 1000)), nil
	case 'M':
		f, err := strconv.ParseFloat(s[:len(s)-1], 64)
		if err != nil {
			return 0, fmt.Errorf("parse badge %q: %w", s, err)
		}
		return int(math.Round(f * 1000000)), nil
	default:
		v, err := strconv.Atoi(s)
		if err != nil {
			return 0, fmt.Errorf("parse badge %q: %w", s, err)
		}
		return v, nil
	}
}

// csvRow holds a single parsed row from the countme CSV.
type csvRow struct {
	weekStart string
	weekEnd   string
	osName    string
	osVersion string
	sysAge    string
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

	required := []string{"week_start", "week_end", "os_name", "sys_age", "hits"}
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
			hits:      hits,
		})
	}
	return rows, nil
}

// rowsToWeekRecords aggregates csvRows into WeekRecords, applying distro and sys_age filters.
func rowsToWeekRecords(rows []csvRow) []WeekRecord {
	type weekKey struct{ start, end string }
	agg := make(map[weekKey]map[string]int)

	for _, row := range rows {
		if row.sysAge == "-1" {
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

// fetchBadgeCountsFromURLs fetches badge counts from custom URLs (for testability).
// Returns a map of distro key → active user count.
func fetchBadgeCountsFromURLs(urls map[string]string) (map[string]int, error) {
	type result struct {
		key   string
		count int
		err   error
	}

	results := make(chan result, len(urls))
	var wg sync.WaitGroup

	for key, url := range urls {
		wg.Add(1)
		go func(k, u string) {
			defer wg.Done()
			resp, err := http.Get(u) //nolint:noctx
			if err != nil {
				results <- result{key: k, err: err}
				return
			}
			defer resp.Body.Close()

			var badge struct {
				Message string `json:"message"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&badge); err != nil {
				results <- result{key: k, err: fmt.Errorf("decode badge %s: %w", k, err)}
				return
			}

			count, err := parseBadgeValue(badge.Message)
			results <- result{key: k, count: count, err: err}
		}(key, url)
	}

	wg.Wait()
	close(results)

	counts := make(map[string]int, len(urls))
	var errs []string
	for r := range results {
		if r.err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", r.key, r.err))
			continue
		}
		counts[r.key] = r.count
	}

	if len(errs) > 0 {
		return counts, fmt.Errorf("badge fetch errors: %s", strings.Join(errs, "; "))
	}
	return counts, nil
}

// FetchBadgeCounts fetches badge counts for all 4 distros concurrently.
func FetchBadgeCounts() (map[string]int, error) {
	urls := make(map[string]string, len(badgeNames))
	for _, name := range badgeNames {
		urls[name] = fmt.Sprintf("%s/%s.json", badgeBaseURL, name)
	}
	return fetchBadgeCountsFromURLs(urls)
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

// AppendDayRecord appends or overwrites today's badge snapshot in the store.
// Deduplicates by date.
func AppendDayRecord(store *HistoryStore, badge map[string]int) *HistoryStore {
	today := time.Now().UTC().Format("2006-01-02")
	total := 0
	for _, v := range badge {
		total += v
	}
	rec := DayRecord{
		Date:    today,
		Distros: badge,
		Total:   total,
	}

	newStore := &HistoryStore{
		WeekRecords: store.WeekRecords,
	}

	replaced := false
	for _, d := range store.DayRecords {
		if d.Date == today {
			newStore.DayRecords = append(newStore.DayRecords, rec)
			replaced = true
		} else {
			newStore.DayRecords = append(newStore.DayRecords, d)
		}
	}
	if !replaced {
		newStore.DayRecords = append(newStore.DayRecords, rec)
	}

	return newStore
}
