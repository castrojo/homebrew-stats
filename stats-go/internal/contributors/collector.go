package contributors

import (
"encoding/json"
"fmt"
"os"
"strings"
"time"

ghcli "github.com/castrojo/homebrew-stats/internal/ghcli"
ghpkg "github.com/castrojo/homebrew-stats/internal/github"
)

// Repos to always include (hardcoded for predictability).
var TrackedRepos = []string{
"ublue-os/bluefin",
"ublue-os/bluefin-lts",
"projectbluefin/dakota",
"projectbluefin/common",
"projectbluefin/finpilot",
"projectbluefin/documentation",
"projectbluefin/bluefin-mcp",
"projectbluefin/testhub",
"projectbluefin/iso",
"projectbluefin/website",
}

const discussionsQuery = `
query RepoDiscussions($owner: String!, $name: String!, $after: String) {
  repository(owner: $owner, name: $name) {
    discussions(first: 50, after: $after, orderBy: {field: CREATED_AT, direction: DESC}) {
      pageInfo { hasNextPage endCursor }
      nodes {
        id
        title
        createdAt
        author { login avatarUrl }
        comments(first: 20) {
          totalCount
          nodes {
            createdAt
            author { login avatarUrl }
          }
        }
      }
    }
  }
}
`

// FilterCommitsAfter returns only commits whose Date is after cutoff.
func FilterCommitsAfter(commits []CommitRecord, cutoff time.Time) []CommitRecord {
var out []CommitRecord
for _, c := range commits {
if c.Date.After(cutoff) {
out = append(out, c)
}
}
return out
}

// FilterIssuesAfter returns only issues created after cutoff.
func FilterIssuesAfter(issues []IssueRecord, cutoff time.Time) []IssueRecord {
var out []IssueRecord
for _, iss := range issues {
if iss.CreatedAt.After(cutoff) {
out = append(out, iss)
}
}
return out
}

// FilterPRsAfter returns only PRs merged after cutoff.
func FilterPRsAfter(prs []PRRecord, cutoff time.Time) []PRRecord {
var out []PRRecord
for _, pr := range prs {
if pr.MergedAt.After(cutoff) {
out = append(out, pr)
}
}
return out
}

// FilterDiscussionsAfter returns only discussions created after cutoff.
func FilterDiscussionsAfter(discussions []DiscussionRecord, cutoff time.Time) []DiscussionRecord {
var out []DiscussionRecord
for _, d := range discussions {
if d.CreatedAt.After(cutoff) {
out = append(out, d)
}
}
return out
}

// CommitRecord is a single commit with author and date.
type CommitRecord struct {
Login string
Date  time.Time
SHA   string
}

// IssueRecord is a single issue.
type IssueRecord struct {
Login     string
State     string
CreatedAt time.Time
ClosedAt  *time.Time
Labels    []labelRecord
}

// labelRecord is a minimal label representation.
type labelRecord struct {
Name string `json:"name"`
}

// PRRecord is a single merged PR.
type PRRecord struct {
Login        string
MergedAt     time.Time
HasReviewers bool
}

// DiscussionRecord is a single discussion with its comments.
type DiscussionRecord struct {
AuthorLogin   string
CreatedAt     time.Time
CommentCount  int
CommentLogins []string
}

// rawCommit is the minimal shape we need from the commits API.
type rawCommit struct {
SHA    string `json:"sha"`
Author *struct {
Login string `json:"login"`
} `json:"author"`
Commit struct {
Author struct {
Name string `json:"name"`
Date string `json:"date"`
} `json:"author"`
} `json:"commit"`
}

// FetchRepoCommits fetches all commits in [since, until] for the given repo.
func FetchRepoCommits(owner, repo string, since, until time.Time) ([]CommitRecord, error) {
out, err := ghcli.Run("api",
fmt.Sprintf("repos/%s/%s/commits?since=%s&until=%s&per_page=100",
owner, repo,
since.Format(time.RFC3339), until.Format(time.RFC3339)),
"--paginate")
if err != nil {
return nil, fmt.Errorf("list commits %s/%s: %w", owner, repo, err)
}

// --paginate concatenates arrays; decode them all.
var all []CommitRecord
dec := json.NewDecoder(strings.NewReader(string(out)))
for dec.More() {
var page []rawCommit
if err := dec.Decode(&page); err != nil {
break
}
for _, c := range page {
login := ""
if c.Author != nil {
login = c.Author.Login
}
if login == "" {
login = c.Commit.Author.Name
}
var date time.Time
if c.Commit.Author.Date != "" {
date, _ = time.Parse(time.RFC3339, c.Commit.Author.Date)
}
all = append(all, CommitRecord{
Login: login,
Date:  date,
SHA:   c.SHA,
})
}
}
return all, nil
}

// rawIssue is the minimal shape from the issues API.
type rawIssue struct {
User struct {
Login string `json:"login"`
} `json:"user"`
State       string       `json:"state"`
CreatedAt   string       `json:"created_at"`
ClosedAt    *string      `json:"closed_at"`
Labels      []labelRecord `json:"labels"`
PullRequest *struct{}    `json:"pull_request"`
}

// FetchRepoIssues fetches issues (not PRs) created in the last 30d.
func FetchRepoIssues(owner, repo string, since time.Time) ([]IssueRecord, error) {
out, err := ghcli.Run("api",
fmt.Sprintf("repos/%s/%s/issues?state=all&sort=created&direction=desc&per_page=100", owner, repo),
"--paginate")
if err != nil {
return nil, fmt.Errorf("list issues %s/%s: %w", owner, repo, err)
}

var all []IssueRecord
dec := json.NewDecoder(strings.NewReader(string(out)))
for dec.More() {
var page []rawIssue
if err := dec.Decode(&page); err != nil {
break
}
done := false
for _, issue := range page {
createdAt, _ := time.Parse(time.RFC3339, issue.CreatedAt)
if createdAt.Before(since) {
done = true
break
}
if issue.PullRequest != nil {
continue // skip PRs
}
rec := IssueRecord{
Login:     issue.User.Login,
State:     issue.State,
CreatedAt: createdAt,
Labels:    issue.Labels,
}
if issue.ClosedAt != nil && *issue.ClosedAt != "" {
t, _ := time.Parse(time.RFC3339, *issue.ClosedAt)
rec.ClosedAt = &t
}
all = append(all, rec)
}
if done {
break
}
}
return all, nil
}

// rawPR is the minimal shape from the pulls API.
type rawPR struct {
User struct {
Login string `json:"login"`
} `json:"user"`
MergedAt           *string `json:"merged_at"`
UpdatedAt          string  `json:"updated_at"`
RequestedReviewers []struct{} `json:"requested_reviewers"`
ReviewComments     int     `json:"review_comments"`
}

// FetchRepoPRs fetches merged PRs updated since the cutoff.
func FetchRepoPRs(owner, repo string, since time.Time) ([]PRRecord, error) {
out, err := ghcli.Run("api",
fmt.Sprintf("repos/%s/%s/pulls?state=closed&sort=updated&direction=desc&per_page=100", owner, repo),
"--paginate")
if err != nil {
return nil, fmt.Errorf("list PRs %s/%s: %w", owner, repo, err)
}

var all []PRRecord
dec := json.NewDecoder(strings.NewReader(string(out)))
for dec.More() {
var page []rawPR
if err := dec.Decode(&page); err != nil {
break
}
done := false
for _, pr := range page {
updatedAt, _ := time.Parse(time.RFC3339, pr.UpdatedAt)
if updatedAt.Before(since) {
done = true
break
}
if pr.MergedAt == nil || *pr.MergedAt == "" {
continue
}
mergedAt, _ := time.Parse(time.RFC3339, *pr.MergedAt)
if mergedAt.Before(since) {
continue
}
all = append(all, PRRecord{
Login:        pr.User.Login,
MergedAt:     mergedAt,
HasReviewers: len(pr.RequestedReviewers) > 0 || pr.ReviewComments > 0,
})
}
if done {
break
}
}
return all, nil
}

// FetchParticipation fetches the 52-week participation stats for a repo.
func FetchParticipation(owner, repo string) ([]int, error) {
out, err := ghcli.Run("api",
fmt.Sprintf("repos/%s/%s/stats/participation", owner, repo),
"--jq", ".all")
if err != nil {
return nil, fmt.Errorf("participation %s/%s: %w", owner, repo, err)
}
var all []int
if err := json.Unmarshal(out, &all); err != nil {
return nil, fmt.Errorf("participation parse %s/%s: %w", owner, repo, err)
}
return all, nil
}

// FetchPunchCard fetches the commit heatmap (day, hour, count) for a repo.
func FetchPunchCard(owner, repo string) ([][]int, error) {
out, err := ghcli.Run("api",
fmt.Sprintf("repos/%s/%s/stats/punch_card", owner, repo))
if err != nil {
return nil, fmt.Errorf("punch card %s/%s: %w", owner, repo, err)
}
var cards [][]int
if err := json.Unmarshal(out, &cards); err != nil {
return nil, fmt.Errorf("punch card parse %s/%s: %w", owner, repo, err)
}
return cards, nil
}

// FetchDiscussions fetches discussions created since the cutoff using GraphQL.
func FetchDiscussions(owner, repo string, since time.Time) ([]DiscussionRecord, error) {
type Author struct {
Login string `json:"login"`
}
type CommentNode struct {
CreatedAt string `json:"createdAt"`
Author    Author `json:"author"`
}
type Comments struct {
TotalCount int           `json:"totalCount"`
Nodes      []CommentNode `json:"nodes"`
}
type DiscNode struct {
Title     string   `json:"title"`
CreatedAt string   `json:"createdAt"`
Author    Author   `json:"author"`
Comments  Comments `json:"comments"`
}
type PageInfo struct {
HasNextPage bool   `json:"hasNextPage"`
EndCursor   string `json:"endCursor"`
}
type Discussions struct {
PageInfo PageInfo   `json:"pageInfo"`
Nodes    []DiscNode `json:"nodes"`
}
type Repository struct {
Discussions Discussions `json:"discussions"`
}
type Data struct {
Repository Repository `json:"repository"`
}

var all []DiscussionRecord
var cursor *string
for {
vars := map[string]any{
"owner": owner,
"name":  repo,
"after": cursor,
}
var data Data
if err := ghpkg.GraphQL(discussionsQuery, vars, &data); err != nil {
// Discussions may be disabled — treat as empty, not an error.
fmt.Fprintf(os.Stderr, "  ⚠️  discussions %s/%s: %v\n", owner, repo, err)
return all, nil
}
done := false
for _, node := range data.Repository.Discussions.Nodes {
t, err := time.Parse(time.RFC3339, node.CreatedAt)
if err != nil {
continue
}
if t.Before(since) {
done = true
break
}
rec := DiscussionRecord{
AuthorLogin:  node.Author.Login,
CreatedAt:    t,
CommentCount: node.Comments.TotalCount,
}
for _, c := range node.Comments.Nodes {
if c.Author.Login != "" {
rec.CommentLogins = append(rec.CommentLogins, c.Author.Login)
}
}
all = append(all, rec)
}
if done || !data.Repository.Discussions.PageInfo.HasNextPage {
break
}
c := data.Repository.Discussions.PageInfo.EndCursor
cursor = &c
}
return all, nil
}

// rawUser is the minimal shape from the users API.
type rawUser struct {
Login           string `json:"login"`
Name            string `json:"name"`
AvatarURL       string `json:"avatar_url"`
Company         string `json:"company"`
Bio             string `json:"bio"`
Location        string `json:"location"`
Blog            string `json:"blog"`
TwitterUsername string `json:"twitter_username"`
PublicRepos     int    `json:"public_repos"`
Followers       int    `json:"followers"`
}

// FetchUserProfile fetches a GitHub user's profile, using the TTL cache to avoid
// redundant API calls. Updates the cache in-place.
func FetchUserProfile(login string, cache *ContributorProfileCache) (*CachedProfile, error) {
if cache.Profiles == nil {
cache.Profiles = make(map[string]*CachedProfile)
}
if p, ok := cache.Profiles[strings.ToLower(login)]; ok {
if time.Since(p.CachedAt) < ProfileCacheTTL {
return p, nil
}
}
out, err := ghcli.Run("api", fmt.Sprintf("users/%s", login))
if err != nil {
return nil, fmt.Errorf("get user %s: %w", login, err)
}
var user rawUser
if err := json.Unmarshal(out, &user); err != nil {
return nil, fmt.Errorf("parse user %s: %w", login, err)
}
p := &CachedProfile{
Login:           user.Login,
Name:            user.Name,
AvatarURL:       user.AvatarURL,
Company:         user.Company,
Bio:             user.Bio,
Location:        user.Location,
Blog:            user.Blog,
TwitterUsername: user.TwitterUsername,
PublicRepos:     user.PublicRepos,
Followers:       user.Followers,
CachedAt:        time.Now().UTC(),
}
cache.Profiles[strings.ToLower(login)] = p
return p, nil
}
