package contributors

import "time"

// ContributorEntry is one human or bot contributor's activity across all repos.
type ContributorEntry struct {
	Login               string   `json:"login"`
	Name                string   `json:"name,omitempty"`
	AvatarURL           string   `json:"avatar_url,omitempty"`
	Company             string   `json:"company,omitempty"`
	Location            string   `json:"location,omitempty"`
	Commits30d          int      `json:"commits_30d"`
	Commits60d          int      `json:"commits_60d"`
	Commits365d         int      `json:"commits_365d"`
	PRsMerged30d        int      `json:"prs_merged_30d"`
	PRsMerged60d        int      `json:"prs_merged_60d"`
	PRsMerged365d       int      `json:"prs_merged_365d"`
	IssuesOpened30d     int      `json:"issues_opened_30d"`
	IssuesOpened60d     int      `json:"issues_opened_60d"`
	IssuesOpened365d    int      `json:"issues_opened_365d"`
	DiscussionPosts30d  int      `json:"discussion_posts_30d"`
	DiscussionPosts60d  int      `json:"discussion_posts_60d"`
	DiscussionPosts365d int      `json:"discussion_posts_365d"`
	ReposActive         []string `json:"repos_active"`
	IsBot               bool     `json:"is_bot"`
}

// ContributorSummary is the KPI hero row.
type ContributorSummary struct {
	ActiveContributors      int     `json:"active_contributors"`
	ActiveContributors60d   int     `json:"active_contributors_60d"`
	ActiveContributors365d  int     `json:"active_contributors_365d"`
	NewContributors         int     `json:"new_contributors"`
	TotalCommits            int     `json:"total_commits"`
	TotalCommits60d         int     `json:"total_commits_60d"`
	TotalCommits365d        int     `json:"total_commits_365d"`
	TotalPRsMerged          int     `json:"total_prs_merged"`
	TotalPRsMerged60d       int     `json:"total_prs_merged_60d"`
	TotalPRsMerged365d      int     `json:"total_prs_merged_365d"`
	TotalIssuesOpened       int     `json:"total_issues_opened"`
	TotalIssuesClosed       int     `json:"total_issues_closed"`
	BusFactor               int     `json:"bus_factor"`
	BusFactor60d            int     `json:"bus_factor_60d"`
	BusFactor365d           int     `json:"bus_factor_365d"`
	ReviewParticipationRate float64 `json:"review_participation_rate"`
	ActiveRepos             int     `json:"active_repos"`
	TotalDiscussions        int     `json:"total_discussions"`
	DiscussionAnswerRate    float64 `json:"discussion_answer_rate"`
}

// RepoStats is per-repository stats across 30d/60d/365d windows.
type RepoStats struct {
	Name                  string         `json:"name"`
	Commits30d            int            `json:"commits_30d"`
	Commits60d            int            `json:"commits_60d"`
	Commits365d           int            `json:"commits_365d"`
	UniqueHumanAuthors30d int            `json:"unique_human_authors_30d"`
	PRsMerged30d          int            `json:"prs_merged_30d"`
	PRsMerged60d          int            `json:"prs_merged_60d"`
	PRsMerged365d         int            `json:"prs_merged_365d"`
	IssuesOpened30d       int            `json:"issues_opened_30d"`
	IssuesOpened60d       int            `json:"issues_opened_60d"`
	IssuesOpened365d      int            `json:"issues_opened_365d"`
	BusFactor             int            `json:"bus_factor"`
	BusFactor60d          int            `json:"bus_factor_60d"`
	BusFactor365d         int            `json:"bus_factor_365d"`
	BotCommits30d         int            `json:"bot_commits_30d"`
	HumanCommits30d       int            `json:"human_commits_30d"`
	HumanCommits60d       int            `json:"human_commits_60d"`
	HumanCommits365d      int            `json:"human_commits_365d"`
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
	TotalDiscussions30d         int              `json:"total_discussions_30d"`
	TotalDiscussions60d         int              `json:"total_discussions_60d"`
	TotalDiscussions365d        int              `json:"total_discussions_365d"`
	TotalDiscussionComments30d  int              `json:"total_discussion_comments_30d"`
	TotalDiscussionComments60d  int              `json:"total_discussion_comments_60d"`
	TotalDiscussionComments365d int              `json:"total_discussion_comments_365d"`
	UniqueDiscussionAuthors30d  int              `json:"unique_discussion_authors_30d"`
	UniqueDiscussionAuthors60d  int              `json:"unique_discussion_authors_60d"`
	UniqueDiscussionAuthors365d int              `json:"unique_discussion_authors_365d"`
	AnsweredRate                float64          `json:"answered_rate"`
	WeeklyTrend                 []DiscussionWeek `json:"weekly_trend"`
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
