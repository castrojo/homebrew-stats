package countme

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- parseBadgeValue tests ---

func TestParseBadgeValue_RawInt(t *testing.T) {
	v, err := parseBadgeValue("64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 64 {
		t.Errorf("expected 64, got %d", v)
	}
}

func TestParseBadgeValue_Kilo(t *testing.T) {
	v, err := parseBadgeValue("71k")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 71000 {
		t.Errorf("expected 71000, got %d", v)
	}
}

func TestParseBadgeValue_KiloDecimal(t *testing.T) {
	v, err := parseBadgeValue("3.6k")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 3600 {
		t.Errorf("expected 3600, got %d", v)
	}
}

func TestParseBadgeValue_Mega(t *testing.T) {
	v, err := parseBadgeValue("1.2M")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 1200000 {
		t.Errorf("expected 1200000, got %d", v)
	}
}

func TestParseBadgeValue_Invalid(t *testing.T) {
	_, err := parseBadgeValue("N/A")
	if err == nil {
		t.Fatal("expected error for 'N/A'")
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
		{"Bluefin LTS", "bluefin-lts"},
		{"Aurora", "aurora"},
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

// --- AppendDayRecord tests ---

func TestAppendDayRecord_NewDay(t *testing.T) {
	store := &HistoryStore{}
	badge := map[string]int{"bazzite": 71000, "bluefin": 3600}
	result := AppendDayRecord(store, badge)
	if len(result.DayRecords) != 1 {
		t.Fatalf("expected 1 day record, got %d", len(result.DayRecords))
	}
	if result.DayRecords[0].Distros["bazzite"] != 71000 {
		t.Errorf("expected bazzite=71000, got %d", result.DayRecords[0].Distros["bazzite"])
	}
}

func TestAppendDayRecord_Idempotent(t *testing.T) {
	today := time.Now().UTC().Format("2006-01-02")
	store := &HistoryStore{
		DayRecords: []DayRecord{
			{Date: today, Distros: map[string]int{"bazzite": 100}, Total: 100},
		},
	}
	badge := map[string]int{"bazzite": 200}
	result := AppendDayRecord(store, badge)
	// Should overwrite today
	if len(result.DayRecords) != 1 {
		t.Fatalf("expected 1 day record after idempotent append, got %d", len(result.DayRecords))
	}
	if result.DayRecords[0].Distros["bazzite"] != 200 {
		t.Errorf("expected updated bazzite=200, got %d", result.DayRecords[0].Distros["bazzite"])
	}
}

// --- FetchBadgeCounts mock test ---

func TestFetchBadgeCountsFromURLs(t *testing.T) {
	// Mock server returns a typical badge endpoint response
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return different values based on path to simulate different distros
		if strings.Contains(r.URL.Path, "bazzite") {
			w.Write([]byte(`{"schemaVersion":1,"label":"active users","message":"71k","color":"blue"}`))
		} else if strings.Contains(r.URL.Path, "bluefin-lts") {
			w.Write([]byte(`{"schemaVersion":1,"label":"active users","message":"64","color":"blue"}`))
		} else {
			w.Write([]byte(`{"schemaVersion":1,"label":"active users","message":"2.6k","color":"blue"}`))
		}
	}))
	defer srv.Close()

	// Call the internal function that accepts custom URLs (for testability)
	urls := map[string]string{
		"bazzite":     srv.URL + "/bazzite.json",
		"bluefin":     srv.URL + "/bluefin.json",
		"bluefin-lts": srv.URL + "/bluefin-lts.json",
		"aurora":      srv.URL + "/aurora.json",
	}
	counts, err := fetchBadgeCountsFromURLs(urls)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if counts["bazzite"] != 71000 {
		t.Errorf("expected bazzite=71000, got %d", counts["bazzite"])
	}
	if counts["bluefin-lts"] != 64 {
		t.Errorf("expected bluefin-lts=64, got %d", counts["bluefin-lts"])
	}
}

// --- CSV parsing test ---

func TestFetchCSVLast30Days_MockServer(t *testing.T) {
	// Minimal valid CSV with one matching row and one non-matching row
	csvData := `week_start,week_end,os_name,sys_age,hits
2024-01-01,2024-01-07,Bazzite,0,500
2024-01-01,2024-01-07,bazzite,0,100
2024-01-01,2024-01-07,Cloudora DX,0,200
2024-01-01,2024-01-07,Bazzite,-1,50
2024-01-01,2024-01-07,Bluefin,0,300
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(csvData))
	}))
	defer srv.Close()

	recs, _, err := fetchCSVFromURL(srv.URL)
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
	// Simulate a CSV that has an os_version column.
	csvData := `week_start,week_end,os_name,os_version,sys_age,hits
2024-01-01,2024-01-07,Bazzite,41,0,200
2024-01-01,2024-01-07,Bazzite,40,0,80
2024-01-01,2024-01-07,Bluefin LTS,42,0,15
2024-01-01,2024-01-07,Bazzite,41,-1,999
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(csvData))
	}))
	defer srv.Close()

	_, dist, err := fetchCSVFromURL(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dist["Bazzite"]["41"] != 200 {
		t.Errorf("Bazzite/41: expected 200, got %d", dist["Bazzite"]["41"])
	}
	if dist["Bazzite"]["40"] != 80 {
		t.Errorf("Bazzite/40: expected 80, got %d", dist["Bazzite"]["40"])
	}
	if dist["Bluefin LTS"]["42"] != 15 {
		t.Errorf("Bluefin LTS/42: expected 15, got %d", dist["Bluefin LTS"]["42"])
	}
}
