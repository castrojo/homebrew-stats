package contributors

import "time"

// ContributorEntry is one human or bot contributor's 30d activity across all repos.
type ContributorEntry struct {
	Login              string   `json:"login"`
	Name               string   `json:"name,omitempty"`
	AvatarURL          string   `json:"avatar_url,omitempty"`
	Company            string   `json:"company,omitempty"`
	Location           string   `json:"location,omitempty"`
	Commits30d         int      `json:"commits_30d"`
	PRsMerged30d       int      `json:"prs_merged_30d"`
	IssuesOpened30d    int      `json:"issues_opened_30d"`
	DiscussionPosts30d int      `json:"discussion_posts_30d"`
	ReposActive        []string `json:"repos_active"`
	IsBot              bool     `json:"is_bot"`
}

// ContributorSummary is the KPI hero row.
type ContributorSummary struct {
	ActiveContributors      int     `json:"active_contributors"`
	NewContributors         int     `json:"new_contributors"`
	TotalCommits            int     `json:"total_commits"`
	TotalPRsMerged          int     `json:"total_prs_merged"`
	TotalIssuesOpened       int     `json:"total_issues_opened"`
	TotalIssuesClosed       int     `json:"total_issues_closed"`
	BusFactor               int     `json:"bus_factor"`
	ReviewParticipationRate float64 `json:"review_participation_rate"`
	ActiveRepos             int     `json:"active_repos"`
	TotalDiscussions        int     `json:"total_discussions"`
	DiscussionAnswerRate    float64 `json:"discussion_answer_rate"`
}

// RepoStats is per-repository 30d stats.
type RepoStats struct {
	Name                  string         `json:"name"`
	Commits30d            int            `json:"commits_30d"`
	UniqueHumanAuthors30d int            `json:"unique_human_authors_30d"`
	PRsMerged30d          int            `json:"prs_merged_30d"`
	IssuesOpened30d       int            `json:"issues_opened_30d"`
	BusFactor             int            `json:"bus_factor"`
	BotCommits30d         int            `json:"bot_commits_30d"`
	HumanCommits30d       int            `json:"human_commits_30d"`
	ActiveWeeksStreak     int            `json:"active_weeks_streak"`
	WeeklyCommits52w      []int          `json:"weekly_commits_52w"`
	CommitsByDayOfWeek    map[string]int `json:"commits_by_day_of_week"`
	ContributionHeatmap   [][]int        `json:"contribution_heatmap"`
	IssueLabelDist        map[string]int `json:"issue_label_distribution"`
}

// DiscussionWeek is one week of discussion activity.
type DiscussionWeek struct {
	Week        string `json:"week"`
	Discussions int    `json:"discussions"`
	Comments    int    `json:"comments"`
}

// DiscussionSummary is aggregate discussion stats.
type DiscussionSummary struct {
	TotalDiscussions30d        int              `json:"total_discussions_30d"`
	TotalDiscussionComments30d int              `json:"total_discussion_comments_30d"`
	UniqueDiscussionAuthors30d int              `json:"unique_discussion_authors_30d"`
	AnsweredRate               float64          `json:"answered_rate"`
	WeeklyTrend                []DiscussionWeek `json:"weekly_trend"`
}

// ContribDaySnapshot is one day's snapshot stored in the history cache.
type ContribDaySnapshot struct {
	Date            string   `json:"date"`
	ActiveContribs  int      `json:"active_contributors"`
	TotalCommits    int      `json:"total_commits"`
	TopContributors []string `json:"top_contributor_logins"` // just logins to keep size small
}

// ContribHistoryStore is the on-disk cache structure (.sync-cache/contributors-history.json).
type ContribHistoryStore struct {
	Snapshots     []ContribDaySnapshot `json:"snapshots"`
	LastFetchedAt string               `json:"last_fetched_at,omitempty"`
}

// CachedProfile is a GitHub user profile with a cache timestamp.
type CachedProfile struct {
	Login           string    `json:"login"`
	Name            string    `json:"name,omitempty"`
	AvatarURL       string    `json:"avatar_url,omitempty"`
	Company         string    `json:"company,omitempty"`
	Bio             string    `json:"bio,omitempty"`
	Location        string    `json:"location,omitempty"`
	Blog            string    `json:"blog,omitempty"`
	TwitterUsername string    `json:"twitter_username,omitempty"`
	PublicRepos     int       `json:"public_repos"`
	Followers       int       `json:"followers"`
	CachedAt        time.Time `json:"cached_at"`
}

// ContributorProfileCache is the on-disk profile cache.
type ContributorProfileCache struct {
	Profiles map[string]*CachedProfile `json:"profiles"` // keyed by login (lowercase)
}

const ProfileCacheTTL = 7 * 24 * time.Hour
