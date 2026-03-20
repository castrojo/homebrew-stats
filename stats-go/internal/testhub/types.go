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

// AppDayCount stores raw pass/fail counts per app per day
type AppDayCount struct {
	App    string `json:"app"`
	Passed int    `json:"passed"`
	Failed int    `json:"failed"`
	Total  int    `json:"total"`
}

// BuildMetrics is computed from raw counts — never stored in history
type BuildMetrics struct {
	App         string  `json:"app"`
	PassRate7d  float64 `json:"pass_rate_7d"`
	PassRate30d float64 `json:"pass_rate_30d"`
	LastStatus  string  `json:"last_status"`
	LastBuildAt string  `json:"last_build_at"`
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
