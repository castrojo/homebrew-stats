package testhub

type Package struct {
	Name         string `json:"name"`
	Version      string `json:"version,omitempty"`
	HTMLURL      string `json:"html_url,omitempty"`
	VersionCount int64  `json:"version_count"`
	PullCount    int64  `json:"pull_count"`
	CreatedAt    string `json:"created_at,omitempty"`
	UpdatedAt    string `json:"updated_at,omitempty"`
}

// JobParseResult is the typed result of parsing a CI job name.
// HasArch is false only for publish-manifest-list (which has no arch suffix).
type JobParseResult struct {
	App     string // lowercased, trimmed
	Stage   string // "compile-oci" | "sign-and-push" | "publish-manifest-list" | "annotate-packages"
	Arch    string // "x86_64" | "aarch64" | "" (publish-manifest-list only)
	HasArch bool
}

// AppDayCount stores raw pass/fail counts per app per day.
// Compile-oci cancelled runs count as failures (cascade origin).
// Non-compile stages only count explicit "failure" conclusions (not cancelled).
type AppDayCount struct {
	App            string `json:"app"`
	Passed         int    `json:"passed"`          // compile-oci x86_64 success
	Failed         int    `json:"failed"`           // compile-oci x86_64 failure or cancelled
	PassedAarch64  int    `json:"passed_aarch64"`   // compile-oci aarch64 success
	FailedAarch64  int    `json:"failed_aarch64"`   // compile-oci aarch64 failure or cancelled
	SignFailed     int    `json:"sign_failed"`       // sign-and-push failure (not cancelled)
	PublishFailed  int    `json:"publish_failed"`    // publish-manifest-list failure (not cancelled)
	AnnotateFailed int    `json:"annotate_failed"`   // annotate-packages failure (not cancelled)
	Total          int    `json:"total"`
}

// BuildMetrics is computed from raw counts — never stored in history.
// LastStatus values: "passing" | "failing" | "stale" | "pending" | "unknown"
//   "stale"   — has 30d history but no 7d activity (builds went silent)
//   "pending" — zero build history (new package or never triggered)
type BuildMetrics struct {
	App           string  `json:"app"`
	PassRate7d    float64 `json:"pass_rate_7d"`
	PassRate30d   float64 `json:"pass_rate_30d"`
	LastStatus    string  `json:"last_status"`
	LastBuildAt   string  `json:"last_build_at"`
	Arch86Status  string  `json:"arch_x86_status"`  // "passing" | "failing" | "unknown"
	ArchArmStatus string  `json:"arch_arm_status"`  // "passing" | "failing" | "unknown"
}

type DaySnapshot struct {
	Date        string        `json:"date"`
	Packages    []Package     `json:"packages"`
	BuildCounts []AppDayCount `json:"build_counts"`
	LastRunID   int64         `json:"last_run_id"`
}

type HistoryStore struct {
	Snapshots []DaySnapshot `json:"snapshots"`
}
