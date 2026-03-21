package contributors

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	gh "github.com/google/go-github/v60/github"

	ghclient "github.com/castrojo/homebrew-stats/internal/github"
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
	Labels    []*gh.Label
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

// FetchRepoCommits fetches all commits in [since, until] for the given repo.
// Paginates automatically. Filters using GitHub's Since/Until params.
// Returns a slice of (authorLogin, commitDate) pairs.
func FetchRepoCommits(ctx context.Context, client *gh.Client, owner, repo string, since, until time.Time) ([]CommitRecord, error) {
	opts := &gh.CommitsListOptions{
		Since: since,
		Until: until,
		ListOptions: gh.ListOptions{PerPage: 100},
	}
	var all []CommitRecord
	for {
		commits, resp, err := client.Repositories.ListCommits(ctx, owner, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("list commits %s/%s: %w", owner, repo, err)
		}
		for _, c := range commits {
			login := ""
			if c.Author != nil {
				login = c.Author.GetLogin()
			}
			if login == "" && c.Commit != nil && c.Commit.Author != nil {
				// Fallback to git author name for unlinked commits.
				login = c.Commit.Author.GetName()
			}
			var date time.Time
			if c.Commit != nil && c.Commit.Author != nil {
				date = c.Commit.Author.GetDate().Time
			}
			all = append(all, CommitRecord{
				Login: login,
				Date:  date,
				SHA:   c.GetSHA(),
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
		sleepIfRateLimited(resp)
	}
	return all, nil
}

// FetchRepoIssues fetches issues (not PRs) created in the last 30d.
// IMPORTANT: Issues.Since filters updated_at, NOT created_at.
// We paginate sorted by created desc and stop when created_at < since.
func FetchRepoIssues(ctx context.Context, client *gh.Client, owner, repo string, since time.Time) ([]IssueRecord, error) {
	opts := &gh.IssueListByRepoOptions{
		State:     "all",
		Sort:      "created",
		Direction: "desc",
		ListOptions: gh.ListOptions{PerPage: 100},
	}
	var all []IssueRecord
	for {
		issues, resp, err := client.Issues.ListByRepo(ctx, owner, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("list issues %s/%s: %w", owner, repo, err)
		}
		done := false
		for _, issue := range issues {
			if issue.CreatedAt != nil && issue.CreatedAt.Before(since) {
				done = true
				break
			}
			// Skip pull requests (issues API returns both).
			if issue.PullRequestLinks != nil {
				continue
			}
			all = append(all, IssueRecord{
				Login:     issue.User.GetLogin(),
				State:     issue.GetState(),
				CreatedAt: issue.GetCreatedAt().Time,
				ClosedAt:  issue.ClosedAt.GetTime(),
				Labels:    issue.Labels,
			})
		}
		if done || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
		sleepIfRateLimited(resp)
	}
	return all, nil
}

// FetchRepoPRs fetches merged PRs updated since the cutoff.
// IMPORTANT: No merged-at filter on API — paginate closed PRs, check MergedAt client-side.
func FetchRepoPRs(ctx context.Context, client *gh.Client, owner, repo string, since time.Time) ([]PRRecord, error) {
	opts := &gh.PullRequestListOptions{
		State:     "closed",
		Sort:      "updated",
		Direction: "desc",
		ListOptions: gh.ListOptions{PerPage: 100},
	}
	var all []PRRecord
	for {
		prs, resp, err := client.PullRequests.List(ctx, owner, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("list PRs %s/%s: %w", owner, repo, err)
		}
		done := false
		for _, pr := range prs {
			if pr.UpdatedAt != nil && pr.UpdatedAt.Before(since) {
				done = true
				break
			}
			if pr.MergedAt == nil || pr.MergedAt.IsZero() {
				continue
			}
			if pr.MergedAt.Before(since) {
				continue
			}
			all = append(all, PRRecord{
				Login:        pr.User.GetLogin(),
				MergedAt:     pr.MergedAt.Time,
				HasReviewers: len(pr.RequestedReviewers) > 0 || pr.GetReviewComments() > 0,
			})
		}
		if done || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
		sleepIfRateLimited(resp)
	}
	return all, nil
}

// FetchParticipation fetches the 52-week participation stats for a repo.
// May return HTTP 202 on first call — retries once after 10s.
func FetchParticipation(ctx context.Context, client *gh.Client, owner, repo string) ([]int, error) {
	stats, resp, err := client.Repositories.ListParticipation(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("participation %s/%s: %w", owner, repo, err)
	}
	if resp.StatusCode == 202 {
		fmt.Fprintf(os.Stderr, "  ℹ️  participation computing (202) for %s/%s — retrying in 10s\n", owner, repo)
		time.Sleep(10 * time.Second)
		stats, _, err = client.Repositories.ListParticipation(ctx, owner, repo)
		if err != nil {
			return nil, fmt.Errorf("participation retry %s/%s: %w", owner, repo, err)
		}
	}
	if stats == nil {
		return []int{}, nil
	}
	return stats.All, nil
}

// FetchPunchCard fetches the commit heatmap (day, hour, count) for a repo.
func FetchPunchCard(ctx context.Context, client *gh.Client, owner, repo string) ([][]int, error) {
	cards, _, err := client.Repositories.ListPunchCard(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("punch card %s/%s: %w", owner, repo, err)
	}
	result := make([][]int, 0, len(cards))
	for _, s := range cards {
		result = append(result, []int{s.GetDay(), s.GetHour(), s.GetCommits()})
	}
	return result, nil
}

// FetchDiscussions fetches discussions created since the cutoff using GraphQL.
// Returns empty slice if the repo has discussions disabled (GraphQL will return an empty list).
func FetchDiscussions(client *ghclient.Client, owner, repo string, since time.Time) ([]DiscussionRecord, error) {
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
		if err := client.GraphQL(discussionsQuery, vars, &data); err != nil {
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

// FetchUserProfile fetches a GitHub user's profile, using the TTL cache to avoid
// redundant API calls. Updates the cache in-place.
func FetchUserProfile(ctx context.Context, client *gh.Client, login string, cache *ContributorProfileCache) (*CachedProfile, error) {
	if cache.Profiles == nil {
		cache.Profiles = make(map[string]*CachedProfile)
	}
	if p, ok := cache.Profiles[strings.ToLower(login)]; ok {
		if time.Since(p.CachedAt) < ProfileCacheTTL {
			return p, nil
		}
	}
	user, _, err := client.Users.Get(ctx, login)
	if err != nil {
		return nil, fmt.Errorf("get user %s: %w", login, err)
	}
	p := &CachedProfile{
		Login:           user.GetLogin(),
		Name:            user.GetName(),
		AvatarURL:       user.GetAvatarURL(),
		Company:         user.GetCompany(),
		Bio:             user.GetBio(),
		Location:        user.GetLocation(),
		Blog:            user.GetBlog(),
		TwitterUsername: user.GetTwitterUsername(),
		PublicRepos:     user.GetPublicRepos(),
		Followers:       user.GetFollowers(),
		CachedAt:        time.Now().UTC(),
	}
	cache.Profiles[strings.ToLower(login)] = p
	return p, nil
}

// sleepIfRateLimited checks the remaining rate limit and sleeps proactively
// if it's dangerously low. This prevents hitting the 5000/hr cap.
func sleepIfRateLimited(resp *gh.Response) {
	if resp == nil {
		return
	}
	if resp.Rate.Remaining < 100 {
		sleep := time.Until(resp.Rate.Reset.Time) + 10*time.Second
		if sleep > 0 {
			fmt.Fprintf(os.Stderr, "⏳ rate limit low (%d remaining) — sleeping %v\n",
				resp.Rate.Remaining, sleep)
			time.Sleep(sleep)
		}
	}
}
