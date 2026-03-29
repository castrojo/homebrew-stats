package builds

// WorkflowRunRecord represents one completed GitHub Actions workflow run.
type WorkflowRunRecord struct {
	ID           int64       `json:"id"`
	Repo         string      `json:"repo"`
	WorkflowName string      `json:"workflow_name"`
	WorkflowFile string      `json:"workflow_file"`
	RunNumber    int         `json:"run_number"`
	Event        string      `json:"event"`
	Branch       string      `json:"branch"`
	Conclusion   string      `json:"conclusion"`
	CreatedAt    string      `json:"created_at"`
	StartedAt    string      `json:"started_at"`
	CompletedAt  string      `json:"completed_at"`
	DurationSec  int         `json:"duration_sec"`
	QueueTimeSec int         `json:"queue_time_sec"`
	Jobs         []JobRecord `json:"jobs,omitempty"`
}

// JobRecord represents one job within a workflow run.
type JobRecord struct {
	ID          int64        `json:"id"`
	Name        string       `json:"name"`
	Conclusion  string       `json:"conclusion"`
	StartedAt   string       `json:"started_at"`
	CompletedAt string       `json:"completed_at"`
	DurationSec int          `json:"duration_sec"`
	RunnerName  string       `json:"runner_name,omitempty"`
	Platform    string       `json:"platform,omitempty"`
	Variant     string       `json:"variant,omitempty"`
	Flavor      string       `json:"flavor,omitempty"`
	Stream      string       `json:"stream,omitempty"`
	Steps       []StepRecord `json:"steps,omitempty"`
}

// StepRecord represents one step within a job.
type StepRecord struct {
	Name        string `json:"name"`
	Conclusion  string `json:"conclusion"`
	DurationSec int    `json:"duration_sec"`
}

// MonthlySnapshot holds aggregated monthly history for trend charts.
type MonthlySnapshot struct {
	Month           string             `json:"month"` // "2025-01"
	TotalRuns       int                `json:"total_runs"`
	SuccessCount    int                `json:"success_count"`
	FailureCount    int                `json:"failure_count"`
	CancelledCount  int                `json:"cancelled_count"`
	SuccessRate     float64            `json:"success_rate"` // 0.0–100.0
	AvgDurationMin  float64            `json:"avg_duration_min"`
	P95DurationMin  float64            `json:"p95_duration_min"`
	RepoSuccessRate map[string]float64 `json:"repo_success_rate"`
	DORALevel       string             `json:"dora_level"`
}

// BuildsOutput is the top-level JSON written to src/data/builds.json.
type BuildsOutput struct {
	GeneratedAt      string            `json:"generated_at"`
	Repos            []RepoMetrics     `json:"repos"`
	Summary          PipelineSummary   `json:"summary"`
	DORAMetrics      DORAMetrics       `json:"dora_metrics"`
	TopFlaky         []FlakyJob        `json:"top_flaky"`
	RecentBuilds     []RecentBuild     `json:"recent_builds"`
	DurationTrend    []DurationBucket  `json:"duration_trend"`
	FailureBreakdown []FailureCategory `json:"failure_breakdown"`
	TriggerBreakdown TriggerBreakdown  `json:"trigger_breakdown"`
	History          []DailySnapshot   `json:"history"`
	MonthlyHistory   []MonthlySnapshot `json:"monthly_history"`
}

// PipelineSummary holds top-level KPIs.
type PipelineSummary struct {
	OverallSuccessRate7d  float64 `json:"overall_success_rate_7d"`
	OverallSuccessRate30d float64 `json:"overall_success_rate_30d"`
	TotalBuilds7d         int     `json:"total_builds_7d"`
	TotalBuilds30d        int     `json:"total_builds_30d"`
	AvgDurationMin        float64 `json:"avg_duration_min"`
	P50DurationMin        float64 `json:"p50_duration_min"`
	P95DurationMin        float64 `json:"p95_duration_min"`
	P99DurationMin        float64 `json:"p99_duration_min"`
	AvgQueueTimeSec       float64 `json:"avg_queue_time_sec"`
	ActiveStreams         int     `json:"active_streams"`
	HealthStatus          string  `json:"health_status"`
}

// DORAMetrics holds the four DORA key metrics.
type DORAMetrics struct {
	DeploymentFrequency  string  `json:"deployment_frequency"`
	DeployFreqPerWeek    float64 `json:"deploy_freq_per_week"`
	LeadTimeMinutes      float64 `json:"lead_time_minutes"`
	ChangeFailureRatePct float64 `json:"change_failure_rate_pct"`
	MTTRMinutes          float64 `json:"mttr_minutes"`
	MTBFHours            float64 `json:"mtbf_hours"`
	DORALevel            string  `json:"dora_level"`
}

// FlakyJob is one entry in the flakiness leaderboard.
type FlakyJob struct {
	Repo           string  `json:"repo"`
	JobName        string  `json:"job_name"`
	TotalRuns      int     `json:"total_runs"`
	Failures       int     `json:"failures"`
	FailureRate    float64 `json:"failure_rate"`
	FlakinessIndex float64 `json:"flakiness_index"`
	LastFailure    string  `json:"last_failure"`
	TopFailStep    string  `json:"top_fail_step"`
}

// RecentBuild is one entry in the recent builds feed.
type RecentBuild struct {
	RunID       int64   `json:"run_id"`
	Repo        string  `json:"repo"`
	Workflow    string  `json:"workflow"`
	Branch      string  `json:"branch"`
	Event       string  `json:"event"`
	Conclusion  string  `json:"conclusion"`
	DurationMin float64 `json:"duration_min"`
	StartedAt   string  `json:"started_at"`
	HTMLURL     string  `json:"html_url"`
	JobCount    int     `json:"job_count"`
	FailedJobs  int     `json:"failed_jobs"`
}

// DurationBucket holds daily P50/P95/P99 percentile data.
type DurationBucket struct {
	Date string  `json:"date"`
	P50  float64 `json:"p50"`
	P95  float64 `json:"p95"`
	P99  float64 `json:"p99"`
	Avg  float64 `json:"avg"`
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
}

// FailureCategory is one segment of the failure breakdown.
type FailureCategory struct {
	Category string  `json:"category"`
	Count    int     `json:"count"`
	Pct      float64 `json:"pct"`
}

// TriggerBreakdown counts runs by trigger event.
type TriggerBreakdown struct {
	Scheduled int `json:"scheduled"`
	Push      int `json:"push"`
	PR        int `json:"pull_request"`
	Manual    int `json:"workflow_dispatch"`
	Other     int `json:"other"`
}

// DailySnapshot holds aggregated daily history.
type DailySnapshot struct {
	Date            string                  `json:"date"`
	TotalRuns       int                     `json:"total_runs"`
	SuccessCount    int                     `json:"success_count"`
	FailureCount    int                     `json:"failure_count"`
	CancelledCount  int                     `json:"cancelled_count"`
	AvgDurationMin  float64                 `json:"avg_duration_min"`
	P95DurationMin  float64                 `json:"p95_duration_min"`
	AvgQueueTimeSec float64                 `json:"avg_queue_time_sec"`
	RepoBreakdown   map[string]RepoDayCount `json:"repo_breakdown"`
}

// RepoDayCount is per-repo counts within a DailySnapshot.
type RepoDayCount struct {
	Runs      int `json:"runs"`
	Successes int `json:"successes"`
	Failures  int `json:"failures"`
}

// RepoMetrics holds per-repository rollup.
type RepoMetrics struct {
	Repo                   string          `json:"repo"`
	SuccessRate7d          float64         `json:"success_rate_7d"`
	SuccessRate30d         float64         `json:"success_rate_30d"`
	TotalRuns7d            int             `json:"total_runs_7d"`
	TotalRuns30d           int             `json:"total_runs_30d"`
	AvgDurationMin         float64         `json:"avg_duration_min"`
	Streams                []StreamMetrics `json:"streams"`
	Architectures          []ArchMetrics   `json:"architectures,omitempty"`
	SignStepSuccessRate30d float64         `json:"sign_step_success_rate_30d"`
	SBOMStepSuccessRate30d float64         `json:"sbom_step_success_rate_30d"`
}

// StreamMetrics holds per-stream (stable/latest/beta/variant) metrics.
type StreamMetrics struct {
	Name           string  `json:"name"`
	SuccessRate7d  float64 `json:"success_rate_7d"`
	SuccessRate30d float64 `json:"success_rate_30d"`
	TotalRuns7d    int     `json:"total_runs_7d"`
	AvgDurationMin float64 `json:"avg_duration_min"`
	LastRunAt      string  `json:"last_run_at"`
	LastConclusion string  `json:"last_conclusion"`
}

// ArchMetrics holds per-architecture breakdown.
type ArchMetrics struct {
	Platform       string  `json:"platform"`
	SuccessRate7d  float64 `json:"success_rate_7d"`
	AvgDurationMin float64 `json:"avg_duration_min"`
	TotalJobs7d    int     `json:"total_jobs_7d"`
}

// BuildsHistory is stored in .sync-cache/builds-history.json.
type BuildsHistory struct {
	Runs []WorkflowRunRecord `json:"runs"`
}

// RepoConfig defines which workflow files to track for a repository.
type RepoConfig struct {
	Owner         string
	Repo          string
	Label         string
	WorkflowFiles []string
}

// DefaultRepos is the canonical list of all 7 tracked Bluefin build repositories.
// Kept as-is for the legacy fetch-builds subcommand (backward compat).
var DefaultRepos = []RepoConfig{
	{
		Owner: "ublue-os", Repo: "bluefin", Label: "bluefin",
		WorkflowFiles: []string{
			"build-image-stable.yml",
			"build-image-latest-main.yml",
			"build-image-beta.yml",
		},
	},
	{
		Owner: "ublue-os", Repo: "bluefin-lts", Label: "bluefin-lts",
		WorkflowFiles: []string{
			"build-regular.yml",
			"build-dx.yml",
			"build-regular-hwe.yml",
			"build-dx-hwe.yml",
			"build-gdx.yml",
			"build-gnome50.yml",
			"build-testing.yml",
		},
	},
	{Owner: "projectbluefin", Repo: "common", Label: "common", WorkflowFiles: []string{"build.yml"}},
	{Owner: "projectbluefin", Repo: "dakota", Label: "dakota", WorkflowFiles: []string{"build.yml"}},
	{
		Owner: "projectbluefin", Repo: "iso", Label: "iso",
		WorkflowFiles: []string{
			"build-iso-all.yml",
			"build-iso-stable.yml",
			"build-iso-lts-hwe.yml",
		},
	},
	{Owner: "projectbluefin", Repo: "finpilot", Label: "finpilot", WorkflowFiles: []string{"build.yml"}},
	{Owner: "projectbluefin", Repo: "testhub", Label: "testhub", WorkflowFiles: []string{"build.yml"}},
}

// BluefinRepos is the per-image repo set for the Bluefin builds page.
// Identical to DefaultRepos; defined separately so future changes are explicit.
var BluefinRepos = DefaultRepos

// AuroraRepos is the per-image repo set for the Aurora builds page.
// aurora-test is excluded to avoid duplicate mirrored-run noise.
var AuroraRepos = []RepoConfig{
	{
		Owner: "ublue-os", Repo: "aurora", Label: "aurora",
		WorkflowFiles: []string{
			"build-image-stable.yml",
			"build-image-latest-main.yml",
			"build-image-beta.yml",
		},
	},
	{Owner: "get-aurora-dev", Repo: "common", Label: "common", WorkflowFiles: []string{"build.yml"}},
	{Owner: "get-aurora-dev", Repo: "iso", Label: "iso", WorkflowFiles: []string{
		"build-iso-stable.yml",
		"build-iso-latest.yml",
	}},
}

// BazziteRepos is the per-image repo set for the Bazzite builds page.
// Includes the main image build, ISO pipeline, and the DX variant.
// bazzite-arch is excluded (separate Arch-based ecosystem).
// bazzite-gdx is excluded (PR-only triggers, infrequent activity).
var BazziteRepos = []RepoConfig{
	{
		Owner: "ublue-os", Repo: "bazzite", Label: "bazzite",
		WorkflowFiles: []string{
			"build.yml",
			"build_iso.yml",
		},
	},
	{Owner: "ublue-os", Repo: "bazzite-dx", Label: "bazzite-dx", WorkflowFiles: []string{"build.yml"}},
}
