package countme

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- countmeCSVURL regression test ---
// The CSV lives on the Fedora data-analysis server. The ublue-os/countme GitHub repo
// does NOT contain totals.csv — raw.githubusercontent.com returns 404 for that path.
// This test guards against that regression.
func TestCountmeCSVURL_PointsToFedoraServer(t *testing.T) {
	if !strings.HasPrefix(countmeCSVURL, "https://data-analysis.fedoraproject.org/") {
		t.Errorf("countmeCSVURL must point to data-analysis.fedoraproject.org, got: %s\n"+
			"IMPORTANT: raw.githubusercontent.com/ublue-os/countme/main/totals.csv returns 404 — "+
			"that file does not exist in the GitHub repo.", countmeCSVURL)
	}
	if strings.Contains(countmeCSVURL, "githubusercontent") {
		t.Errorf("countmeCSVURL must NOT use raw.githubusercontent.com — the file does not exist there (404). " +
			"Correct URL: https://data-analysis.fedoraproject.org/csv-reports/countme/totals.csv")
	}
}

// --- parseDistroName tests ---

func TestParseDistroName_ValidExact(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"Bazzite", "bazzite"},
		{"Bluefin", "bluefin"},
		{"Aurora", "aurora"},
		{"secureblue", "secureblue"},
		{"wayblue", "wayblue"},
	}
	for _, c := range cases {
		got, ok := parseDistroName(c.input)
		if !ok {
			t.Errorf("parseDistroName(%q): expected ok=true", c.input)
			continue
		}
		if got != c.expected {
			t.Errorf("parseDistroName(%q): expected %q, got %q", c.input, c.expected, got)
		}
	}
}

func TestParseDistroName_Lowercase(t *testing.T) {
	_, ok := parseDistroName("bazzite")
	if ok {
		t.Fatal("parseDistroName('bazzite'): expected ok=false (case-sensitive)")
	}
}

func TestParseDistroName_Unknown(t *testing.T) {
	_, ok := parseDistroName("Cloudora DX")
	if ok {
		t.Fatal("parseDistroName('Cloudora DX'): expected ok=false")
	}
}

func TestParseDistroName_Empty(t *testing.T) {
	_, ok := parseDistroName("")
	if ok {
		t.Fatal("expected ok=false for empty string")
	}
}

// --- MergeIntoHistory tests ---

func TestMergeIntoHistory_Dedup(t *testing.T) {
	store := &HistoryStore{
		WeekRecords: []WeekRecord{
			{WeekStart: "2024-01-01", WeekEnd: "2024-01-07", Distros: map[string]int{"bazzite": 100}, Total: 100},
		},
	}
	// Same week_start — should overwrite
	newRecs := []WeekRecord{
		{WeekStart: "2024-01-01", WeekEnd: "2024-01-07", Distros: map[string]int{"bazzite": 200}, Total: 200},
	}
	result := MergeIntoHistory(store, newRecs)
	if len(result.WeekRecords) != 1 {
		t.Fatalf("expected 1 week record after dedup, got %d", len(result.WeekRecords))
	}
	if result.WeekRecords[0].Total != 200 {
		t.Errorf("expected Total=200 (last write wins), got %d", result.WeekRecords[0].Total)
	}
}

func TestMergeIntoHistory_NewWeek(t *testing.T) {
	store := &HistoryStore{
		WeekRecords: []WeekRecord{
			{WeekStart: "2024-01-01", WeekEnd: "2024-01-07", Total: 100},
		},
	}
	newRecs := []WeekRecord{
		{WeekStart: "2024-01-08", WeekEnd: "2024-01-14", Total: 150},
	}
	result := MergeIntoHistory(store, newRecs)
	if len(result.WeekRecords) != 2 {
		t.Fatalf("expected 2 week records, got %d", len(result.WeekRecords))
	}
}

func TestMergeIntoHistory_EmptyStore(t *testing.T) {
	store := &HistoryStore{}
	newRecs := []WeekRecord{
		{WeekStart: "2024-01-01", Total: 100},
	}
	result := MergeIntoHistory(store, newRecs)
	if len(result.WeekRecords) != 1 {
		t.Fatalf("expected 1 week record, got %d", len(result.WeekRecords))
	}
}

// --- FetchCSVLast30Days mock test ---

func TestFetchCSVLast30Days_MockServer(t *testing.T) {
	// Minimal valid CSV with one matching row and one non-matching row.
	// repo_tag must be present; canonical rows use "fedora-41".
	csvData := `week_start,week_end,os_name,sys_age,repo_tag,hits
2024-01-01,2024-01-07,Bazzite,0,fedora-42,500
2024-01-01,2024-01-07,bazzite,0,fedora-42,100
2024-01-01,2024-01-07,Cloudora DX,0,fedora-42,200
2024-01-01,2024-01-07,Bazzite,-1,fedora-42,50
2024-01-01,2024-01-07,Bluefin,0,fedora-42,300
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(csvData))
	}))
	defer srv.Close()

	recs, _, _, err := fetchCSVFromURL(srv.URL, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should aggregate: Bazzite=500 (not 100 lowercase, not 50 sys_age=-1)
	// Bluefin=300. No Cloudora DX.
	if len(recs) != 1 {
		t.Fatalf("expected 1 week record (one week_start), got %d", len(recs))
	}
	rec := recs[0]
	if rec.Distros["bazzite"] != 500 {
		t.Errorf("expected bazzite=500, got %d", rec.Distros["bazzite"])
	}
	if rec.Distros["bluefin"] != 300 {
		t.Errorf("expected bluefin=300, got %d", rec.Distros["bluefin"])
	}
	if _, exists := rec.Distros["cloudora-dx"]; exists {
		t.Error("Cloudora DX should not appear in results")
	}
	// sys_age=-1 rows should be excluded
	if rec.Distros["bazzite"] == 550 {
		t.Error("sys_age=-1 rows should be excluded from aggregate")
	}
}

// --- parseOsVersionDist tests ---

func TestParseOsVersionDist(t *testing.T) {
	rows := []csvRow{
		{osName: "Bazzite", osVersion: "41", sysAge: "0", hits: 100},
		{osName: "Bazzite", osVersion: "40", sysAge: "0", hits: 50},
		{osName: "Bluefin", osVersion: "41", sysAge: "0", hits: 30},
		{osName: "Unknown", osVersion: "41", sysAge: "0", hits: 5},   // not a target distro
		{osName: "Bazzite", osVersion: "41", sysAge: "-1", hits: 99}, // sys_age=-1 excluded
		{osName: "Bazzite", osVersion: "", sysAge: "0", hits: 10},    // empty version excluded
	}

	got := parseOsVersionDist(rows)

	// Unknown should be absent
	if _, ok := got["Unknown"]; ok {
		t.Error("Unknown should not appear in result")
	}
	// Bazzite/41 = 100 (sys_age=-1 row excluded)
	if got["Bazzite"]["41"] != 100 {
		t.Errorf("Bazzite/41: expected 100, got %d", got["Bazzite"]["41"])
	}
	// Bazzite/40 = 50
	if got["Bazzite"]["40"] != 50 {
		t.Errorf("Bazzite/40: expected 50, got %d", got["Bazzite"]["40"])
	}
	// Bluefin/41 = 30
	if got["Bluefin"]["41"] != 30 {
		t.Errorf("Bluefin/41: expected 30, got %d", got["Bluefin"]["41"])
	}
}

func TestMergeOsVersionDist(t *testing.T) {
	existing := map[string]map[string]int{
		"Bazzite": {"41": 100},
	}
	newData := map[string]map[string]int{
		"Bazzite": {"41": 20, "40": 50},
		"Bluefin": {"41": 30},
	}

	result := MergeOsVersionDist(existing, newData)

	// Bazzite/41 should be replaced (not summed) by newData value: 20
	if result["Bazzite"]["41"] != 20 {
		t.Errorf("Bazzite/41: expected 20 (new data wins), got %d", result["Bazzite"]["41"])
	}
	// Bazzite/40 from newData
	if result["Bazzite"]["40"] != 50 {
		t.Errorf("Bazzite/40: expected 50, got %d", result["Bazzite"]["40"])
	}
	// Bluefin from newData
	if result["Bluefin"]["41"] != 30 {
		t.Errorf("Bluefin/41: expected 30, got %d", result["Bluefin"]["41"])
	}
}

func TestMergeOsVersionDist_NilExisting(t *testing.T) {
	newData := map[string]map[string]int{
		"Aurora": {"42": 5},
	}
	result := MergeOsVersionDist(nil, newData)
	if result["Aurora"]["42"] != 5 {
		t.Errorf("Aurora/42: expected 5, got %d", result["Aurora"]["42"])
	}
}

func TestMergeOsVersionDist_PreservesUnaffectedDistros(t *testing.T) {
	existing := map[string]map[string]int{
		"Bazzite": {"41": 100},
		"Bluefin": {"40": 200},
	}
	newData := map[string]map[string]int{
		"Bazzite": {"41": 999},
		// Bluefin not in newData — should be preserved
	}
	result := MergeOsVersionDist(existing, newData)
	if result["Bluefin"]["40"] != 200 {
		t.Errorf("Bluefin/40 should be preserved from existing, got %d", result["Bluefin"]["40"])
	}
}

func TestParseOsVersionDist_WithCsvOsVersionColumn(t *testing.T) {
	// Simulate a CSV that has os_version and repo_tag columns.
	// repo_tag must match ^fedora-\d+$ for rows to pass rowsToWeekRecords,
	// but parseOsVersionDist operates on all rows (repo_tag agnostic).
	// Bluefin LTS is not in validDistros, so it must not appear in dist output.
	csvData := `week_start,week_end,os_name,os_version,sys_age,repo_tag,hits
2024-01-01,2024-01-07,Bazzite,42,0,fedora-42,200
2024-01-01,2024-01-07,Bazzite,41,0,fedora-42,80
2024-01-01,2024-01-07,Bluefin LTS,42,0,fedora-42,15
2024-01-01,2024-01-07,Bazzite,42,-1,fedora-42,999
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(csvData))
	}))
	defer srv.Close()

	_, dist, _, err := fetchCSVFromURL(srv.URL, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dist["Bazzite"]["42"] != 200 {
		t.Errorf("Bazzite/42: expected 200, got %d", dist["Bazzite"]["42"])
	}
	if dist["Bazzite"]["41"] != 80 {
		t.Errorf("Bazzite/41: expected 80, got %d", dist["Bazzite"]["41"])
	}
	// Bluefin LTS is excluded from validDistros — must not appear in dist output.
	if _, exists := dist["Bluefin LTS"]; exists {
		t.Error("Bluefin LTS should not appear in dist output (excluded from validDistros)")
	}
}

// --- isCanonicalRepoTag tests ---

func TestIsCanonicalRepoTag_Valid(t *testing.T) {
	cases := []string{"fedora-42", "fedora-41", "fedora-40", "fedora-9", "fedora-100"}
	for _, tag := range cases {
		if !isCanonicalRepoTag(tag) {
			t.Errorf("isCanonicalRepoTag(%q): expected true", tag)
		}
	}
}

func TestIsCanonicalRepoTag_Invalid(t *testing.T) {
	cases := []string{
		"updates-released-f42",
		"fedora-cisco-openh264-42",
		"epel-10",
		"epel-debug-10",
		"fedora-",         // no digit
		"fedora",          // no dash-digit
		"fedora-42-extra", // trailing junk
		"",
	}
	for _, tag := range cases {
		if isCanonicalRepoTag(tag) {
			t.Errorf("isCanonicalRepoTag(%q): expected false", tag)
		}
	}
}

// --- rowsToWeekRecords filter tests ---

func TestRowsToWeekRecords_RepoTagFilter(t *testing.T) {
	rows := []csvRow{
		// Canonical tag — should be counted
		{weekStart: "2024-01-01", weekEnd: "2024-01-07", osName: "Bazzite", sysAge: "0", repoTag: "fedora-42", hits: 1000},
		// Non-canonical tags — should be excluded (would double-count otherwise)
		{weekStart: "2024-01-01", weekEnd: "2024-01-07", osName: "Bazzite", sysAge: "0", repoTag: "updates-released-f42", hits: 900},
		{weekStart: "2024-01-01", weekEnd: "2024-01-07", osName: "Bazzite", sysAge: "0", repoTag: "fedora-cisco-openh264-42", hits: 800},
	}
	recs := rowsToWeekRecords(rows)
	if len(recs) != 1 {
		t.Fatalf("expected 1 week record, got %d", len(recs))
	}
	if recs[0].Distros["bazzite"] != 1000 {
		t.Errorf("expected bazzite=1000 (fedora-42 only), got %d", recs[0].Distros["bazzite"])
	}
}

func TestRowsToWeekRecords_AnomalousWeekExcluded(t *testing.T) {
	rows := []csvRow{
		// Normal week — should be counted
		{weekStart: "2025-06-29", weekEnd: "2025-07-05", osName: "Bazzite", sysAge: "0", repoTag: "fedora-42", hits: 500},
		// Anomalous week — infrastructure migration, must be excluded
		{weekStart: "2025-06-30", weekEnd: "2025-07-06", osName: "Bazzite", sysAge: "0", repoTag: "fedora-42", hits: 300},
		// Year-end partial week — must be excluded
		{weekStart: "2024-12-23", weekEnd: "2024-12-29", osName: "Bazzite", sysAge: "0", repoTag: "fedora-42", hits: 200},
	}
	recs := rowsToWeekRecords(rows)
	if len(recs) != 1 {
		t.Fatalf("expected 1 week record (anomalous weeks excluded), got %d", len(recs))
	}
	if recs[0].WeekEnd != "2025-07-05" {
		t.Errorf("expected surviving week_end=2025-07-05, got %s", recs[0].WeekEnd)
	}
}

func TestRowsToWeekRecords_SysAgeMinusOneExcluded(t *testing.T) {
	rows := []csvRow{
		{weekStart: "2024-01-01", weekEnd: "2024-01-07", osName: "Aurora", sysAge: "0", repoTag: "fedora-42", hits: 400},
		{weekStart: "2024-01-01", weekEnd: "2024-01-07", osName: "Aurora", sysAge: "-1", repoTag: "fedora-42", hits: 999},
	}
	recs := rowsToWeekRecords(rows)
	if len(recs) != 1 {
		t.Fatalf("expected 1 week record, got %d", len(recs))
	}
	if recs[0].Distros["aurora"] != 400 {
		t.Errorf("expected aurora=400 (sys_age=-1 excluded), got %d", recs[0].Distros["aurora"])
	}
}
