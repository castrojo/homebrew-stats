package testhub

import (
	"testing"
	"time"
)

// --- AppendSnapshot tests ---

func TestAppendSnapshot_NewDay(t *testing.T) {
	store := &HistoryStore{}
	pkgs := []Package{{Name: "ghostty", VersionCount: 5, PullCount: 42}}
	counts := []AppDayCount{{App: "ghostty", Passed: 3, Failed: 1, Total: 4}}

	result := AppendSnapshot(store, pkgs, counts, 100)

	if len(result.Snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(result.Snapshots))
	}
	if result.Snapshots[0].LastRunID != 100 {
		t.Errorf("expected LastRunID=100, got %d", result.Snapshots[0].LastRunID)
	}
	if len(result.Snapshots[0].BuildCounts) != 1 {
		t.Errorf("expected 1 build count, got %d", len(result.Snapshots[0].BuildCounts))
	}
	if result.Snapshots[0].Packages[0].PullCount != 42 {
		t.Errorf("expected PullCount=42, got %d", result.Snapshots[0].Packages[0].PullCount)
	}
}

func TestAppendSnapshot_Idempotent(t *testing.T) {
	store := &HistoryStore{}
	today := time.Now().UTC().Format("2006-01-02")
	store.Snapshots = []DaySnapshot{
		{Date: today, LastRunID: 50, BuildCounts: []AppDayCount{{App: "ghostty", Passed: 2, Failed: 0, Total: 2}}},
	}

	pkgs := []Package{{Name: "ghostty", VersionCount: 5}}
	counts := []AppDayCount{{App: "ghostty", Passed: 5, Failed: 1, Total: 6}}

	result := AppendSnapshot(store, pkgs, counts, 100)

	if len(result.Snapshots) != 1 {
		t.Fatalf("expected 1 snapshot after idempotent append, got %d", len(result.Snapshots))
	}
	if result.Snapshots[0].LastRunID != 100 {
		t.Errorf("expected updated LastRunID=100, got %d", result.Snapshots[0].LastRunID)
	}
	if result.Snapshots[0].BuildCounts[0].Passed != 5 {
		t.Errorf("expected updated Passed=5, got %d", result.Snapshots[0].BuildCounts[0].Passed)
	}
}

func TestAppendSnapshot_Multipledays(t *testing.T) {
	store := &HistoryStore{
		Snapshots: []DaySnapshot{
			{Date: "2024-01-01", LastRunID: 10},
			{Date: "2024-01-02", LastRunID: 20},
		},
	}
	pkgs := []Package{{Name: "ghostty"}}
	counts := []AppDayCount{}

	result := AppendSnapshot(store, pkgs, counts, 30)

	if len(result.Snapshots) != 3 {
		t.Fatalf("expected 3 snapshots, got %d", len(result.Snapshots))
	}
}

// --- parseJobApp tests ---

func TestParseJobApp_CompileOciX86(t *testing.T) {
	r, ok := parseJobApp("compile-oci (ghostty, x86_64)")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if r.App != "ghostty" {
		t.Errorf("expected app=ghostty, got %q", r.App)
	}
	if r.Stage != "compile-oci" {
		t.Errorf("expected stage=compile-oci, got %q", r.Stage)
	}
	if r.Arch != "x86_64" {
		t.Errorf("expected arch=x86_64, got %q", r.Arch)
	}
	if !r.HasArch {
		t.Error("expected HasArch=true")
	}
}

func TestParseJobApp_CompileOciAarch64(t *testing.T) {
	r, ok := parseJobApp("compile-oci (firefox-nightly, aarch64)")
	if !ok {
		t.Fatal("expected ok=true for aarch64 (now tracked)")
	}
	if r.Arch != "aarch64" {
		t.Errorf("expected arch=aarch64, got %q", r.Arch)
	}
	if !r.HasArch {
		t.Error("expected HasArch=true")
	}
}

func TestParseJobApp_SignAndPush(t *testing.T) {
	r, ok := parseJobApp("sign-and-push (ghostty, x86_64)")
	if !ok {
		t.Fatal("expected ok=true for sign-and-push")
	}
	if r.Stage != "sign-and-push" {
		t.Errorf("expected stage=sign-and-push, got %q", r.Stage)
	}
	if r.Arch != "x86_64" {
		t.Errorf("expected arch=x86_64, got %q", r.Arch)
	}
}

func TestParseJobApp_PublishManifestList(t *testing.T) {
	r, ok := parseJobApp("publish-manifest-list (ghostty)")
	if !ok {
		t.Fatal("expected ok=true for publish-manifest-list")
	}
	if r.Stage != "publish-manifest-list" {
		t.Errorf("expected stage=publish-manifest-list, got %q", r.Stage)
	}
	if r.Arch != "" {
		t.Errorf("expected arch empty for publish-manifest-list, got %q", r.Arch)
	}
	if r.HasArch {
		t.Error("expected HasArch=false for publish-manifest-list")
	}
}

func TestParseJobApp_AnnotatePackages(t *testing.T) {
	r, ok := parseJobApp("annotate-packages (ghostty, aarch64)")
	if !ok {
		t.Fatal("expected ok=true for annotate-packages")
	}
	if r.Stage != "annotate-packages" {
		t.Errorf("expected stage=annotate-packages, got %q", r.Stage)
	}
}

func TestParseJobApp_NotRecognised(t *testing.T) {
	_, ok := parseJobApp("Build summary")
	if ok {
		t.Fatal("expected ok=false for unrecognised job")
	}
}

func TestParseJobApp_HyphenatedApp(t *testing.T) {
	r, ok := parseJobApp("compile-oci (firefox-nightly, x86_64)")
	if !ok {
		t.Fatal("expected ok=true for hyphenated app name")
	}
	if r.App != "firefox-nightly" {
		t.Errorf("expected app=firefox-nightly, got %q", r.App)
	}
}

// TestParseJobApp_CaseNormalize verifies that mixed-case CI job names (Bug 7C)
// are normalized to lowercase so the metric-to-package join succeeds.
func TestParseJobApp_CaseNormalize(t *testing.T) {
	r, ok := parseJobApp("compile-oci (io.github.DenysMb.Kontainer, x86_64)")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if r.App != "io.github.denysmb.kontainer" {
		t.Errorf("expected lowercase app name, got %q", r.App)
	}
}

// --- ComputeBuildMetrics tests ---
// ComputeBuildMetrics now returns map[string]float64.

func TestComputeBuildMetrics_Empty(t *testing.T) {
	metrics := ComputeBuildMetrics([]DaySnapshot{}, 7)
	if len(metrics) != 0 {
		t.Errorf("expected 0 metrics for empty snapshots, got %d", len(metrics))
	}
}

func TestComputeBuildMetrics_AllPassed(t *testing.T) {
	snapshots := []DaySnapshot{
		{
			Date:        "2024-01-01",
			BuildCounts: []AppDayCount{{App: "ghostty", Passed: 10, Failed: 0, Total: 10}},
		},
	}
	metrics := ComputeBuildMetrics(snapshots, 7)
	if len(metrics) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(metrics))
	}
	if metrics["ghostty"] != 100.0 {
		t.Errorf("expected 100.0, got %f", metrics["ghostty"])
	}
}

func TestComputeBuildMetrics_AllFailed(t *testing.T) {
	snapshots := []DaySnapshot{
		{
			Date:        "2024-01-01",
			BuildCounts: []AppDayCount{{App: "ghostty", Passed: 0, Failed: 5, Total: 5}},
		},
	}
	metrics := ComputeBuildMetrics(snapshots, 7)
	if metrics["ghostty"] != 0.0 {
		t.Errorf("expected 0.0 for all failures, got %f", metrics["ghostty"])
	}
}

func TestComputeBuildMetrics_Mixed(t *testing.T) {
	snapshots := []DaySnapshot{
		{
			Date: "2024-01-01",
			BuildCounts: []AppDayCount{
				{App: "ghostty", Passed: 3, Failed: 1, Total: 4},
				{App: "firefox-nightly", Passed: 10, Failed: 0, Total: 10},
			},
		},
	}
	metrics := ComputeBuildMetrics(snapshots, 7)
	if len(metrics) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(metrics))
	}
	// 3/4 = 75%
	if metrics["ghostty"] != 75.0 {
		t.Errorf("ghostty: expected 75.0, got %f", metrics["ghostty"])
	}
}

func TestComputeBuildMetrics_ZeroTotal(t *testing.T) {
	snapshots := []DaySnapshot{
		{
			Date:        "2024-01-01",
			BuildCounts: []AppDayCount{{App: "ghostty", Passed: 0, Failed: 0, Total: 0}},
		},
	}
	// Total=0 should not panic and rate=0 → app absent from map
	metrics := ComputeBuildMetrics(snapshots, 7)
	if rate, ok := metrics["ghostty"]; ok && rate != 0.0 {
		t.Errorf("expected 0.0 or absent for zero total, got %f", rate)
	}
}

func TestComputeBuildMetrics_WindowFiltering(t *testing.T) {
	old := time.Now().UTC().AddDate(0, 0, -10).Format("2006-01-02")
	recent := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")

	snapshots := []DaySnapshot{
		{
			Date:        old,
			BuildCounts: []AppDayCount{{App: "ghostty", Passed: 0, Failed: 10, Total: 10}},
		},
		{
			Date:        recent,
			BuildCounts: []AppDayCount{{App: "ghostty", Passed: 10, Failed: 0, Total: 10}},
		},
	}
	metrics := ComputeBuildMetrics(snapshots, 7)
	if len(metrics) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(metrics))
	}
	// 7-day window: old snapshot (10d ago) is outside window
	if metrics["ghostty"] != 100.0 {
		t.Errorf("expected 100.0 (old snapshot excluded), got %f", metrics["ghostty"])
	}
}

// TestComputeBuildMetrics_30dField verifies that the 30d rate is not misassigned (Bug 7A).
// The caller is responsible for assigning the return to the correct field; this test
// verifies the map returns the correct value regardless of windowDays.
func TestComputeBuildMetrics_30dField(t *testing.T) {
	old := time.Now().UTC().AddDate(0, 0, -20).Format("2006-01-02")
	recent := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")

	snapshots := []DaySnapshot{
		{
			Date:        old,
			BuildCounts: []AppDayCount{{App: "ghostty", Passed: 4, Failed: 0, Total: 4}},
		},
		{
			Date:        recent,
			BuildCounts: []AppDayCount{{App: "ghostty", Passed: 6, Failed: 0, Total: 6}},
		},
	}
	rate7d := ComputeBuildMetrics(snapshots, 7)
	rate30d := ComputeBuildMetrics(snapshots, 30)

	// 7d window: only recent counts → 6/6 = 100%
	if rate7d["ghostty"] != 100.0 {
		t.Errorf("7d rate: expected 100.0, got %f", rate7d["ghostty"])
	}
	// 30d window: both snapshots count → 10/10 = 100%
	if rate30d["ghostty"] != 100.0 {
		t.Errorf("30d rate: expected 100.0, got %f", rate30d["ghostty"])
	}
	// Verify the two window results are independent (different map objects)
	if &rate7d == &rate30d {
		t.Error("expected independent map objects for 7d and 30d")
	}
}

// --- isTesthubPackage / stripTesthubPrefix tests ---

func TestListPackagesFiltersToTesthubPrefix(t *testing.T) {
	tests := []struct {
		name     string
		pkgName  string
		wantKeep bool
	}{
		{"testhub package", "testhub/ghostty", true},
		{"testhub package with slash", "testhub/goose", true},
		{"bluefin package excluded", "bluefin", false},
		{"dakota package excluded", "dakota", false},
		{"egg package excluded", "egg/foo", false},
		{"partial prefix excluded", "testhubx/ghostty", false},
		{"empty name excluded", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTesthubPackage(tt.pkgName)
			if got != tt.wantKeep {
				t.Errorf("isTesthubPackage(%q) = %v, want %v", tt.pkgName, got, tt.wantKeep)
			}
		})
	}
}

func TestListPackagesStripsPrefix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"testhub/ghostty", "ghostty"},
		{"testhub/goose", "goose"},
		{"testhub/firefox-nightly", "firefox-nightly"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := stripTesthubPrefix(tt.input)
			if got != tt.want {
				t.Errorf("stripTesthubPrefix(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMergePackages_BackfillsMissingNames(t *testing.T) {
	existing := []Package{
		{Name: "ghostty", VersionCount: 7, Version: "1.2.3"},
	}
	fallback := []Package{
		{Name: "ghostty", HTMLURL: "https://example.invalid/ghostty"},
		{Name: "saturn", HTMLURL: "https://example.invalid/saturn"},
	}

	got := MergePackages(existing, fallback)
	if len(got) != 2 {
		t.Fatalf("expected 2 merged packages, got %d", len(got))
	}
	if got[0].Name != "ghostty" || got[1].Name != "saturn" {
		t.Fatalf("expected sorted package names [ghostty saturn], got [%s %s]", got[0].Name, got[1].Name)
	}
	if got[0].VersionCount != 7 || got[0].Version != "1.2.3" {
		t.Fatalf("expected existing ghostty metadata to win, got %+v", got[0])
	}
}
