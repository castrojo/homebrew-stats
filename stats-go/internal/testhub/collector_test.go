package testhub

import (
	"testing"
	"time"
)

// --- AppendSnapshot tests ---

func TestAppendSnapshot_NewDay(t *testing.T) {
	store := &HistoryStore{}
	pkgs := []Package{{Name: "ghostty", VersionCount: 5}}
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
}

func TestAppendSnapshot_Idempotent(t *testing.T) {
	store := &HistoryStore{}
	today := time.Now().UTC().Format("2006-01-02")
	// Pre-populate with today's snapshot
	store.Snapshots = []DaySnapshot{
		{Date: today, LastRunID: 50, BuildCounts: []AppDayCount{{App: "ghostty", Passed: 2, Failed: 0, Total: 2}}},
	}

	pkgs := []Package{{Name: "ghostty", VersionCount: 5}}
	counts := []AppDayCount{{App: "ghostty", Passed: 5, Failed: 1, Total: 6}}

	result := AppendSnapshot(store, pkgs, counts, 100)

	// Should still have exactly 1 snapshot (overwrites today's)
	if len(result.Snapshots) != 1 {
		t.Fatalf("expected 1 snapshot after idempotent append, got %d", len(result.Snapshots))
	}
	// Should be updated to latest data
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

	// Today is a new day — should have 3 snapshots
	if len(result.Snapshots) != 3 {
		t.Fatalf("expected 3 snapshots, got %d", len(result.Snapshots))
	}
}

// --- parseJobApp tests ---

func TestParseJobApp_ValidX86(t *testing.T) {
	app, ok := parseJobApp("compile-oci (ghostty, x86_64)")
	if !ok {
		t.Fatal("expected ok=true for valid x86_64 job")
	}
	if app != "ghostty" {
		t.Errorf("expected app=ghostty, got %q", app)
	}
}

func TestParseJobApp_SkipAarch64(t *testing.T) {
	_, ok := parseJobApp("compile-oci (firefox-nightly, aarch64)")
	if ok {
		t.Fatal("expected ok=false for aarch64 job (should be skipped)")
	}
}

func TestParseJobApp_NotCompileOci(t *testing.T) {
	_, ok := parseJobApp("Build summary")
	if ok {
		t.Fatal("expected ok=false for non-compile-oci job")
	}
}

func TestParseJobApp_HyphenatedApp(t *testing.T) {
	app, ok := parseJobApp("compile-oci (firefox-nightly, x86_64)")
	if !ok {
		t.Fatal("expected ok=true for hyphenated app name")
	}
	if app != "firefox-nightly" {
		t.Errorf("expected app=firefox-nightly, got %q", app)
	}
}

// --- ComputeBuildMetrics tests ---

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
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	if metrics[0].PassRate7d != 100.0 {
		t.Errorf("expected PassRate7d=100.0, got %f", metrics[0].PassRate7d)
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
	if metrics[0].PassRate7d != 0.0 {
		t.Errorf("expected PassRate7d=0.0 for all failures, got %f", metrics[0].PassRate7d)
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
		t.Fatalf("expected 2 metrics, got %d", len(metrics))
	}
	// Find ghostty
	for _, m := range metrics {
		if m.App == "ghostty" {
			// 3/4 = 75%
			if m.PassRate7d != 75.0 {
				t.Errorf("ghostty: expected PassRate7d=75.0, got %f", m.PassRate7d)
			}
		}
	}
}

func TestComputeBuildMetrics_ZeroTotal(t *testing.T) {
	// Edge case: Total=0 must not panic (division by zero)
	snapshots := []DaySnapshot{
		{
			Date:        "2024-01-01",
			BuildCounts: []AppDayCount{{App: "ghostty", Passed: 0, Failed: 0, Total: 0}},
		},
	}
	// Should not panic
	metrics := ComputeBuildMetrics(snapshots, 7)
	if len(metrics) == 1 && metrics[0].PassRate7d != 0.0 {
		t.Errorf("expected PassRate7d=0.0 for zero total, got %f", metrics[0].PassRate7d)
	}
}

func TestComputeBuildMetrics_WindowFiltering(t *testing.T) {
	// Only snapshots within the window should count for PassRate7d
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
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	// 7-day window: old snapshot is 10 days ago, outside window
	// Only recent snapshot counts → PassRate7d = 100%
	if metrics[0].PassRate7d != 100.0 {
		t.Errorf("expected PassRate7d=100.0 (old snapshot excluded by window), got %f", metrics[0].PassRate7d)
	}
}
