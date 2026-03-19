package countme

type WeekRecord struct {
	WeekStart string         `json:"week_start"`
	WeekEnd   string         `json:"week_end"`
	Distros   map[string]int `json:"distros"`
	Total     int            `json:"total"`
}

type DayRecord struct {
	Date      string         `json:"date"`
	Distros   map[string]int `json:"distros"`
	Total     int            `json:"total"`
	WeekStart string         `json:"week_start"`
	WeekEnd   string         `json:"week_end"`
}

type HistoryStore struct {
	WeekRecords []WeekRecord `json:"week_records"`
	DayRecords  []DayRecord  `json:"day_records"`
	// OsVersionDist maps os_name → os_version → total active user count
	// accumulated from CSV data over all fetches.
	OsVersionDist map[string]map[string]int `json:"os_version_dist,omitempty"`
}
