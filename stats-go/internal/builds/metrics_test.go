package builds

import (
	"fmt"
	"math"
	"testing"
	"time"
)

// timeAgo returns an RFC3339 timestamp for `days` days ago.
func timeAgo(days int) string {
	return time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339)
}

// makeRun is a test helper that builds a WorkflowRunRecord with sensible defaults.
func makeRun(id int64, repo, conclusion, event, branch string, daysAgo int) WorkflowRunRecord {
	return WorkflowRunRecord{
		ID:          id,
		Repo:        repo,
		Conclusion:  conclusion,
		Event:       event,
		Branch:      branch,
		CreatedAt:   timeAgo(daysAgo),
		StartedAt:   timeAgo(daysAgo),
		CompletedAt: timeAgo(daysAgo),
		DurationSec: 120,
	}
}

// ── TestSuccessRate ──────────────────────────────────────────────────────────

func TestSuccessRate(t *testing.T) {
	cases := []struct {
		name    string
		runs    []WorkflowRunRecord
		days    int
		wantMin float64
		wantMax float64
	}{
		{
			name:    "empty",
			runs:    nil,
			days:    7,
			wantMin: 0,
			wantMax: 0,
		},
		{
			name: "all-success",
			runs: []WorkflowRunRecord{
				makeRun(1, "r", "success", "push", "main", 1),
				makeRun(2, "r", "success", "push", "main", 2),
			},
			days:    7,
			wantMin: 100,
			wantMax: 100,
		},
		{
			name: "all-failure",
			runs: []WorkflowRunRecord{
				makeRun(1, "r", "failure", "push", "main", 1),
				makeRun(2, "r", "failure", "push", "main", 2),
			},
			days:    7,
			wantMin: 0,
			wantMax: 0,
		},
		{
			name: "mixed-50pct",
			runs: []WorkflowRunRecord{
				makeRun(1, "r", "success", "push", "main", 1),
				makeRun(2, "r", "failure", "push", "main", 2),
			},
			days:    7,
			wantMin: 50,
			wantMax: 50,
		},
		{
			name: "all-cancelled-excluded",
			runs: []WorkflowRunRecord{
				makeRun(1, "r", "cancelled", "push", "main", 1),
				makeRun(2, "r", "skipped", "push", "main", 2),
				makeRun(3, "r", "action_required", "push", "main", 3),
			},
			days:    7,
			wantMin: 0,
			wantMax: 0,
		},
		{
			name: "window-boundary-excludes-old",
			runs: []WorkflowRunRecord{
				makeRun(1, "r", "success", "push", "main", 1),   // inside 7d window
				makeRun(2, "r", "failure", "push", "main", 8),   // outside 7d window
				makeRun(3, "r", "failure", "push", "main", 100), // outside 7d window
			},
			days:    7,
			wantMin: 100,
			wantMax: 100,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SuccessRate(tc.runs, tc.days)
			if got < tc.wantMin || got > tc.wantMax {
				t.Errorf("SuccessRate() = %.2f, want [%.2f, %.2f]", got, tc.wantMin, tc.wantMax)
			}
		})
	}
}

// ── TestPercentile ───────────────────────────────────────────────────────────

func TestPercentile(t *testing.T) {
	cases := []struct {
		name   string
		values []float64
		p      float64
		want   float64
	}{
		{name: "empty", values: nil, p: 50, want: 0},
		{name: "single", values: []float64{42}, p: 50, want: 42},
		{name: "four-values-p50", values: []float64{1, 2, 3, 4}, p: 50, want: 2},
		{name: "four-values-p95", values: []float64{1, 2, 3, 4}, p: 95, want: 4},
		{name: "four-values-p99", values: []float64{1, 2, 3, 4}, p: 99, want: 4},
		{name: "p0-returns-min", values: []float64{5, 10, 15}, p: 0, want: 5},
		{name: "p100-returns-max", values: []float64{5, 10, 15}, p: 100, want: 15},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Percentile(tc.values, tc.p)
			if got != tc.want {
				t.Errorf("Percentile(%v, %v) = %v, want %v", tc.values, tc.p, got, tc.want)
			}
		})
	}
}

// ── TestFlakinessIndex ───────────────────────────────────────────────────────

func TestFlakinessIndex(t *testing.T) {
	allTrue := func(n int) []bool {
		b := make([]bool, n)
		for i := range b {
			b[i] = true
		}
		return b
	}
	allFalse := func(n int) []bool { return make([]bool, n) }
	alternating := func(n int) []bool {
		b := make([]bool, n)
		for i := range b {
			b[i] = i%2 == 0
		}
		return b
	}

	cases := []struct {
		name       string
		outcomes   []bool
		windowSize int
		wantMin    float64
		wantMax    float64
	}{
		{name: "all-pass", outcomes: allTrue(20), windowSize: 5, wantMin: 0, wantMax: 0.001},
		{name: "all-fail", outcomes: allFalse(20), windowSize: 5, wantMin: 0, wantMax: 0.001},
		{name: "alternating-high-flakiness", outcomes: alternating(20), windowSize: 5, wantMin: 0.09, wantMax: 0.5},
		{name: "windowSize-1-all-pass", outcomes: allTrue(10), windowSize: 1, wantMin: 0, wantMax: 0.001},
		{name: "too-short", outcomes: []bool{true}, windowSize: 5, wantMin: 0, wantMax: 0},
		{name: "empty", outcomes: nil, windowSize: 5, wantMin: 0, wantMax: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FlakinessIndex(tc.outcomes, tc.windowSize)
			if got < tc.wantMin || got > tc.wantMax {
				t.Errorf("FlakinessIndex() = %.4f, want [%.4f, %.4f]", got, tc.wantMin, tc.wantMax)
			}
		})
	}
}

// ── TestClassifyFailure ──────────────────────────────────────────────────────

func TestClassifyFailure(t *testing.T) {
	step := func(name, conclusion string) StepRecord {
		return StepRecord{Name: name, Conclusion: conclusion}
	}
	job := func(conclusion string, steps ...StepRecord) JobRecord {
		return JobRecord{Conclusion: conclusion, Steps: steps}
	}

	cases := []struct {
		name string
		job  JobRecord
		want string
	}{
		{
			name: "cancelled",
			job:  JobRecord{Conclusion: "cancelled"},
			want: "cancelled",
		},
		{
			name: "build-step",
			job:  job("failure", step("Build image with Buildah", "failure")),
			want: "build",
		},
		{
			name: "push-step",
			job:  job("failure", step("Push image to registry", "failure")),
			want: "push",
		},
		{
			name: "sign-step",
			job:  job("failure", step("Cosign image", "failure")),
			want: "sign",
		},
		{
			name: "sbom-step",
			job:  job("failure", step("Generate SBOM with Syft", "failure")),
			want: "sbom",
		},
		{
			name: "test-step",
			job:  job("failure", step("Run integration tests", "failure")),
			want: "test",
		},
		{
			name: "infra-step",
			job:  job("failure", step("Set up QEMU", "failure")),
			want: "infra",
		},
		{
			name: "unknown-no-steps",
			job:  JobRecord{Conclusion: "failure"},
			want: "unknown",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyFailure(tc.job)
			if got != tc.want {
				t.Errorf("ClassifyFailure() = %q, want %q", got, tc.want)
			}
		})
	}
}

// ── TestParseJobDimensions ───────────────────────────────────────────────────

func TestParseJobDimensions(t *testing.T) {
	cases := []struct {
		name         string
		jobName      string
		wantPlatform string
		wantVariant  string
		wantFlavor   string
		wantStream   string
	}{
		{
			name:        "bluefin-matrix-format",
			jobName:     "build_container (main, bluefin)",
			wantFlavor:  "main",
			wantVariant: "bluefin",
		},
		{
			name:         "arm64-format",
			jobName:      "build / Build and push image (arm64)",
			wantPlatform: "arm64",
		},
		{
			name:         "iso-format",
			jobName:      "Build Stable ISOs / Build ISOs (amd64, nvidia-open, stable)",
			wantPlatform: "amd64",
			wantStream:   "stable",
		},
		{
			name:    "unknown-no-parens",
			jobName: "build image",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotPlatform, gotVariant, gotFlavor, gotStream := ParseJobDimensions(tc.jobName)
			if gotPlatform != tc.wantPlatform {
				t.Errorf("platform = %q, want %q (job: %q)", gotPlatform, tc.wantPlatform, tc.jobName)
			}
			if tc.wantVariant != "" && gotVariant != tc.wantVariant {
				t.Errorf("variant = %q, want %q (job: %q)", gotVariant, tc.wantVariant, tc.jobName)
			}
			if tc.wantFlavor != "" && gotFlavor != tc.wantFlavor {
				t.Errorf("flavor = %q, want %q (job: %q)", gotFlavor, tc.wantFlavor, tc.jobName)
			}
			if gotStream != tc.wantStream {
				t.Errorf("stream = %q, want %q (job: %q)", gotStream, tc.wantStream, tc.jobName)
			}
		})
	}
}

// ── TestHealthStatus ─────────────────────────────────────────────────────────

func TestHealthStatus(t *testing.T) {
	cases := []struct {
		name    string
		rate7d  float64
		mttrMin float64
		want    string
	}{
		{name: "healthy", rate7d: 98, mttrMin: 30, want: "healthy"},
		{name: "degraded-low-rate", rate7d: 85, mttrMin: 30, want: "degraded"},
		{name: "degraded-high-mttr", rate7d: 96, mttrMin: 120, want: "degraded"},
		{name: "failing", rate7d: 70, mttrMin: 300, want: "failing"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := HealthStatus(tc.rate7d, tc.mttrMin)
			if got != tc.want {
				t.Errorf("HealthStatus(%.0f, %.0f) = %q, want %q", tc.rate7d, tc.mttrMin, got, tc.want)
			}
		})
	}
}

// ── TestComputeDORALevel ─────────────────────────────────────────────────────

func TestComputeDORALevel(t *testing.T) {
	cases := []struct {
		name string
		dora DORAMetrics
		want string
	}{
		{
			name: "elite",
			dora: DORAMetrics{
				DeployFreqPerWeek:    14, // 2/day
				LeadTimeMinutes:      30, // 30 min
				ChangeFailureRatePct: 2,  // 2%
				MTTRMinutes:          45, // 45 min
			},
			want: "elite",
		},
		{
			name: "high",
			dora: DORAMetrics{
				DeployFreqPerWeek:    3,   // 3/week
				LeadTimeMinutes:      240, // 4 hours
				ChangeFailureRatePct: 8,   // 8%
				MTTRMinutes:          120, // 2 hours
			},
			want: "high",
		},
		{
			name: "medium",
			dora: DORAMetrics{
				DeployFreqPerWeek:    0.5,  // ~2/month
				LeadTimeMinutes:      2880, // 48h
				ChangeFailureRatePct: 12,
				MTTRMinutes:          480,
			},
			want: "medium",
		},
		{
			name: "low",
			dora: DORAMetrics{
				DeployFreqPerWeek:    0.1,
				LeadTimeMinutes:      20000,
				ChangeFailureRatePct: 40,
				MTTRMinutes:          10000,
			},
			want: "low",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeDORALevel(tc.dora)
			if got != tc.want {
				t.Errorf("ComputeDORALevel() = %q, want %q (dora: %+v)", got, tc.want, tc.dora)
			}
		})
	}
}

// ── TestMTTR ────────────────────────────────────────────────────────────────

func TestMTTR(t *testing.T) {
	t.Run("no-runs", func(t *testing.T) {
		got := MTTR(nil)
		if got != 0 {
			t.Errorf("MTTR(nil) = %.2f, want 0", got)
		}
	})

	t.Run("failure-then-success", func(t *testing.T) {
		base := time.Now().UTC().Truncate(time.Hour)
		runs := []WorkflowRunRecord{
			{Conclusion: "failure", Branch: "main", CreatedAt: base.Add(-2 * time.Hour).Format(time.RFC3339)},
			{Conclusion: "success", Branch: "main", CreatedAt: base.Format(time.RFC3339)},
		}
		got := MTTR(runs)
		// Expect approximately 120 minutes
		if got < 110 || got > 130 {
			t.Errorf("MTTR() = %.2f min, want ~120 min", got)
		}
	})

	t.Run("no-recovery", func(t *testing.T) {
		runs := []WorkflowRunRecord{
			{Conclusion: "failure", Branch: "main", CreatedAt: timeAgo(2)},
			{Conclusion: "failure", Branch: "main", CreatedAt: timeAgo(1)},
		}
		got := MTTR(runs)
		if got != 0 {
			t.Errorf("MTTR() = %.2f, want 0 (no recovery)", got)
		}
	})
}

// ── TestMTBF ────────────────────────────────────────────────────────────────

func TestMTBF(t *testing.T) {
	t.Run("no-failures", func(t *testing.T) {
		runs := []WorkflowRunRecord{
			makeRun(1, "r", "success", "push", "main", 1),
		}
		got := MTBF(runs)
		if got != 0 {
			t.Errorf("MTBF() = %.2f, want 0 (no failures)", got)
		}
	})

	t.Run("single-failure", func(t *testing.T) {
		runs := []WorkflowRunRecord{
			makeRun(1, "r", "failure", "push", "main", 1),
		}
		got := MTBF(runs)
		if got != 0 {
			t.Errorf("MTBF() = %.2f, want 0 (fewer than 2 failures)", got)
		}
	})

	t.Run("two-failures-24h-apart", func(t *testing.T) {
		base := time.Now().UTC().Truncate(time.Hour)
		runs := []WorkflowRunRecord{
			{Conclusion: "failure", Branch: "main", CreatedAt: base.Add(-24 * time.Hour).Format(time.RFC3339)},
			{Conclusion: "failure", Branch: "main", CreatedAt: base.Format(time.RFC3339)},
		}
		got := MTBF(runs)
		// Expect approximately 24 hours
		if math.Abs(got-24) > 1 {
			t.Errorf("MTBF() = %.2f h, want ~24 h", got)
		}
	})
}

// ── TestComputeSummary ───────────────────────────────────────────────────────

func TestComputeSummary(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		s := ComputeSummary(nil)
		if s.TotalBuilds7d != 0 {
			t.Errorf("TotalBuilds7d = %d, want 0", s.TotalBuilds7d)
		}
	})

	t.Run("basic-metrics", func(t *testing.T) {
		runs := []WorkflowRunRecord{
			makeRun(1, "r", "success", "push", "main", 1),
			makeRun(2, "r", "success", "push", "main", 2),
			makeRun(3, "r", "failure", "push", "main", 3),
		}
		s := ComputeSummary(runs)
		if s.TotalBuilds7d != 3 {
			t.Errorf("TotalBuilds7d = %d, want 3", s.TotalBuilds7d)
		}
		// ~66.7% success rate
		if s.OverallSuccessRate7d < 60 || s.OverallSuccessRate7d > 70 {
			t.Errorf("OverallSuccessRate7d = %.2f, want ~66.7", s.OverallSuccessRate7d)
		}
		if s.HealthStatus == "" {
			t.Error("HealthStatus should not be empty")
		}
	})
}

// ── TestComputeTriggerBreakdown ──────────────────────────────────────────────

func TestComputeTriggerBreakdown(t *testing.T) {
	runs := []WorkflowRunRecord{
		makeRun(1, "r", "success", "schedule", "main", 1),
		makeRun(2, "r", "success", "push", "main", 1),
		makeRun(3, "r", "success", "pull_request", "main", 1),
		makeRun(4, "r", "success", "workflow_dispatch", "main", 1),
		makeRun(5, "r", "success", "repository_dispatch", "main", 1),
	}
	td := ComputeTriggerBreakdown(runs)
	if td.Scheduled != 1 {
		t.Errorf("Scheduled = %d, want 1", td.Scheduled)
	}
	if td.Push != 1 {
		t.Errorf("Push = %d, want 1", td.Push)
	}
	if td.PR != 1 {
		t.Errorf("PR = %d, want 1", td.PR)
	}
	if td.Manual != 1 {
		t.Errorf("Manual = %d, want 1", td.Manual)
	}
	if td.Other != 1 {
		t.Errorf("Other = %d, want 1", td.Other)
	}
}

// ── TestComputeFailureBreakdown ──────────────────────────────────────────────

func TestComputeFailureBreakdown(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		got := ComputeFailureBreakdown(nil)
		if len(got) != 0 {
			t.Errorf("expected empty breakdown, got %d entries", len(got))
		}
	})

	t.Run("all-success-no-breakdown", func(t *testing.T) {
		runs := []WorkflowRunRecord{
			makeRun(1, "r", "success", "push", "main", 1),
		}
		got := ComputeFailureBreakdown(runs)
		if len(got) != 0 {
			t.Errorf("expected empty breakdown for all-success, got %d entries", len(got))
		}
	})

	t.Run("failure-classified", func(t *testing.T) {
		run := makeRun(1, "r", "failure", "push", "main", 1)
		run.Jobs = []JobRecord{
			{
				Conclusion: "failure",
				Steps: []StepRecord{
					{Name: "Build image", Conclusion: "failure"},
				},
			},
		}
		got := ComputeFailureBreakdown([]WorkflowRunRecord{run})
		if len(got) == 0 {
			t.Fatal("expected at least one failure category")
		}
		if got[0].Category != "build" {
			t.Errorf("category = %q, want %q", got[0].Category, "build")
		}
		if math.Abs(got[0].Pct-100) > 0.01 {
			t.Errorf("pct = %.2f, want 100", got[0].Pct)
		}
	})
}

// ── TestComputeDurationTrend ─────────────────────────────────────────────────

func TestComputeDurationTrend(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		got := ComputeDurationTrend(nil, 30)
		if len(got) != 0 {
			t.Errorf("expected empty, got %d buckets", len(got))
		}
	})

	t.Run("single-run", func(t *testing.T) {
		run := makeRun(1, "r", "success", "push", "main", 1)
		run.DurationSec = 600 // 10 min
		got := ComputeDurationTrend([]WorkflowRunRecord{run}, 30)
		if len(got) != 1 {
			t.Fatalf("expected 1 bucket, got %d", len(got))
		}
		if math.Abs(got[0].P50-10) > 0.1 {
			t.Errorf("P50 = %.2f, want 10 min", got[0].P50)
		}
	})
}

// ── TestComputeRepoMetrics ───────────────────────────────────────────────────

func TestComputeRepoMetrics(t *testing.T) {
	runs := []WorkflowRunRecord{
		makeRun(1, "bluefin", "success", "push", "main", 1),
		makeRun(2, "bluefin", "failure", "push", "main", 2),
		makeRun(3, "common", "success", "push", "main", 1),
	}

	rm := ComputeRepoMetrics(runs, "bluefin")
	if rm.Repo != "bluefin" {
		t.Errorf("Repo = %q, want %q", rm.Repo, "bluefin")
	}
	if rm.TotalRuns7d != 2 {
		t.Errorf("TotalRuns7d = %d, want 2", rm.TotalRuns7d)
	}
	if rm.SuccessRate7d != 50 {
		t.Errorf("SuccessRate7d = %.1f, want 50", rm.SuccessRate7d)
	}
}

// ── makeRunAt helper ─────────────────────────────────────────────────────────

// makeRunAt builds a WorkflowRunRecord with a specific timestamp.
// durationMin is converted to DurationSec for storage.
func makeRunAt(repo, conclusion string, durationMin float64, t time.Time) WorkflowRunRecord {
	return WorkflowRunRecord{
		Repo:        repo,
		Conclusion:  conclusion,
		DurationSec: int(durationMin * 60),
		CreatedAt:   t.Format(time.RFC3339),
	}
}

// ── TestComputeMonthlySnapshots ──────────────────────────────────────────────

func TestComputeMonthlySnapshots_ExcludesCurrentMonth_Issue40(t *testing.T) {
	now := time.Now()
	// run in current month — must be excluded
	r1 := makeRunAt("org/repo-a", "success", 5.0, now)
	// run in prior month — must be included
	prior := now.AddDate(0, -1, 0)
	r2 := makeRunAt("org/repo-a", "success", 5.0, prior)

	result := computeMonthlySnapshots([]WorkflowRunRecord{r1, r2})

	if len(result) != 1 {
		t.Fatalf("expected 1 month, got %d", len(result))
	}
	wantMonth := prior.Format("2006-01")
	if result[0].Month != wantMonth {
		t.Errorf("expected month %s, got %s", wantMonth, result[0].Month)
	}
}

func TestComputeMonthlySnapshots_AggregatesSuccessCount_Issue40(t *testing.T) {
	base := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	runs := []WorkflowRunRecord{
		makeRunAt("org/repo-a", "success", 3.0, base),
		makeRunAt("org/repo-a", "failure", 2.0, base),
		makeRunAt("org/repo-a", "cancelled", 1.0, base),
		makeRunAt("org/repo-a", "success", 4.0, base),
	}
	result := computeMonthlySnapshots(runs)
	if len(result) != 1 {
		t.Fatalf("expected 1 month, got %d", len(result))
	}
	m := result[0]
	if m.SuccessCount != 2 {
		t.Errorf("SuccessCount: want 2, got %d", m.SuccessCount)
	}
	if m.FailureCount != 1 {
		t.Errorf("FailureCount: want 1, got %d", m.FailureCount)
	}
	if m.CancelledCount != 1 {
		t.Errorf("CancelledCount: want 1, got %d", m.CancelledCount)
	}
	if m.TotalRuns != 4 {
		t.Errorf("TotalRuns: want 4, got %d", m.TotalRuns)
	}
}

func TestComputeMonthlySnapshots_ComputesAvgDuration_Issue40(t *testing.T) {
	base := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	runs := []WorkflowRunRecord{
		makeRunAt("org/repo-a", "success", 2.0, base),
		makeRunAt("org/repo-a", "success", 4.0, base),
		makeRunAt("org/repo-a", "success", 6.0, base),
	}
	result := computeMonthlySnapshots(runs)
	if len(result) != 1 {
		t.Fatalf("expected 1 month, got %d", len(result))
	}
	want := 4.0 // (2+4+6)/3
	if result[0].AvgDurationMin != want {
		t.Errorf("AvgDurationMin: want %.1f, got %.1f", want, result[0].AvgDurationMin)
	}
}

func TestComputeMonthlySnapshots_ComputesP95Duration_Issue40(t *testing.T) {
	base := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	// 20 runs with durations 1..20 min. P95 index = ceil(0.95*20)-1 = 19-1=18 → value 19.0
	var runs []WorkflowRunRecord
	for i := 1; i <= 20; i++ {
		runs = append(runs, makeRunAt("org/repo-a", "success", float64(i), base))
	}
	result := computeMonthlySnapshots(runs)
	if len(result) != 1 {
		t.Fatalf("expected 1 month, got %d", len(result))
	}
	want := 19.0
	if result[0].P95DurationMin != want {
		t.Errorf("P95DurationMin: want %.1f, got %.1f", want, result[0].P95DurationMin)
	}
}

func TestComputeMonthlySnapshots_PerRepoSuccessRate_Issue40(t *testing.T) {
	base := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	runs := []WorkflowRunRecord{
		makeRunAt("org/repo-a", "success", 1.0, base),
		makeRunAt("org/repo-a", "success", 1.0, base),
		makeRunAt("org/repo-a", "failure", 1.0, base),
		makeRunAt("org/repo-b", "success", 1.0, base),
		makeRunAt("org/repo-b", "failure", 1.0, base),
	}
	result := computeMonthlySnapshots(runs)
	if len(result) != 1 {
		t.Fatalf("expected 1 month, got %d", len(result))
	}
	rsr := result[0].RepoSuccessRate
	wantA := 100.0 * 2.0 / 3.0
	wantB := 50.0
	if rsr["org/repo-a"] != wantA {
		t.Errorf("repo-a: want %.2f, got %.2f", wantA, rsr["org/repo-a"])
	}
	if rsr["org/repo-b"] != wantB {
		t.Errorf("repo-b: want %.2f, got %.2f", wantB, rsr["org/repo-b"])
	}
}

func TestComputeMonthlySnapshots_EmptyInput_Issue40(t *testing.T) {
	result := computeMonthlySnapshots(nil)
	if result == nil {
		t.Fatal("expected non-nil slice for empty input, got nil")
	}
	if len(result) != 0 {
		t.Fatalf("expected empty slice, got %d elements", len(result))
	}
}

// MT1: non-nil empty slice returns non-nil empty slice (not nil which would JSON-marshal as null).
func TestComputeMonthlySnapshots_NonNilEmptySlice_MT1(t *testing.T) {
	result := computeMonthlySnapshots([]WorkflowRunRecord{})
	if result == nil {
		t.Fatal("expected non-nil slice for empty non-nil input, got nil")
	}
	if len(result) != 0 {
		t.Fatalf("expected empty slice, got %d elements", len(result))
	}
}

// MT2: runs in two distinct months produce two separate snapshot buckets.
func TestComputeMonthlySnapshots_TwoMonths_MT2(t *testing.T) {
	jan := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	feb := time.Date(2025, 2, 15, 12, 0, 0, 0, time.UTC)
	runs := []WorkflowRunRecord{
		makeRunAt("org/repo-a", "success", 5.0, jan),
		makeRunAt("org/repo-a", "success", 5.0, feb),
	}
	result := computeMonthlySnapshots(runs)
	if len(result) != 2 {
		t.Fatalf("expected 2 monthly buckets, got %d", len(result))
	}
	months := []string{result[0].Month, result[1].Month}
	if !containsString(months, "2025-01") || !containsString(months, "2025-02") {
		t.Errorf("expected months [2025-01 2025-02], got %v", months)
	}
}

// MT3: a repo appearing only in one month has its key in that month's RepoSuccessRate,
// not in the other month's map. This mirrors the sparse-repo scenario that the
// MonthlyRepoBreakdown Astro fix (issue #46) must handle on the frontend.
func TestComputeMonthlySnapshots_SparseRepos_MT3(t *testing.T) {
	jan := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	feb := time.Date(2025, 2, 15, 12, 0, 0, 0, time.UTC)
	runs := []WorkflowRunRecord{
		makeRunAt("org/repo-a", "success", 5.0, jan), // only in Jan
		makeRunAt("org/repo-b", "success", 5.0, feb), // only in Feb
	}
	result := computeMonthlySnapshots(runs)
	if len(result) != 2 {
		t.Fatalf("expected 2 months, got %d", len(result))
	}
	// Find Jan and Feb snapshots by month string
	var janSnap, febSnap *MonthlySnapshot
	for i := range result {
		switch result[i].Month {
		case "2025-01":
			janSnap = &result[i]
		case "2025-02":
			febSnap = &result[i]
		}
	}
	if janSnap == nil || febSnap == nil {
		t.Fatal("could not find both month snapshots")
	}
	if _, ok := janSnap.RepoSuccessRate["org/repo-a"]; !ok {
		t.Error("Jan snapshot missing org/repo-a key")
	}
	if _, ok := janSnap.RepoSuccessRate["org/repo-b"]; ok {
		t.Error("Jan snapshot must NOT contain org/repo-b (only appears in Feb)")
	}
	if _, ok := febSnap.RepoSuccessRate["org/repo-b"]; !ok {
		t.Error("Feb snapshot missing org/repo-b key")
	}
	if _, ok := febSnap.RepoSuccessRate["org/repo-a"]; ok {
		t.Error("Feb snapshot must NOT contain org/repo-a (only appears in Jan)")
	}
}

// MT4: union of repo keys across all months equals all repos that ever appeared.
// This is the Go-level proof of the contract the Astro fix relies on: the frontend
// must union keys across all snapshots (not just history[0]) to build correct datasets.
func TestComputeMonthlySnapshots_RepoKeyUnion_MT4(t *testing.T) {
	jan := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	feb := time.Date(2025, 2, 15, 12, 0, 0, 0, time.UTC)
	mar := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)
	runs := []WorkflowRunRecord{
		makeRunAt("org/repo-a", "success", 1.0, jan),
		makeRunAt("org/repo-b", "failure", 1.0, feb),
		makeRunAt("org/repo-c", "success", 1.0, mar),
	}
	result := computeMonthlySnapshots(runs)
	if len(result) != 3 {
		t.Fatalf("expected 3 monthly buckets, got %d", len(result))
	}
	// Build union of all repo keys across all snapshots
	union := map[string]bool{}
	for _, snap := range result {
		for k := range snap.RepoSuccessRate {
			union[k] = true
		}
	}
	for _, want := range []string{"org/repo-a", "org/repo-b", "org/repo-c"} {
		if !union[want] {
			t.Errorf("repo %q missing from union of all monthly RepoSuccessRate maps", want)
		}
	}
}

// containsString is a small helper used by MT2.
func containsString(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// ── Integration smoke: fmt is used ──────────────────────────────────────────

func TestPackageSmoke(t *testing.T) {
	// Ensure the package compiles and basic types are correct.
	_ = fmt.Sprintf("smoke: %d repos configured", len(DefaultRepos))
	if len(DefaultRepos) != 7 {
		t.Errorf("DefaultRepos = %d, want 7", len(DefaultRepos))
	}
}
