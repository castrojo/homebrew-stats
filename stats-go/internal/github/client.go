package github

import (
	"context"
	"fmt"
	"os"
	"strings"

	gh "github.com/google/go-github/v60/github"
	"golang.org/x/oauth2"
)

// Client wraps the GitHub API.
type Client struct {
	gh  *gh.Client
	ctx context.Context
}

// NewClient creates an authenticated GitHub client from GITHUB_TOKEN env var.
func NewClient() (*Client, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN environment variable not set")
	}
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	return &Client{gh: gh.NewClient(tc), ctx: ctx}, nil
}

// GetTrafficClones fetches 14-day clone traffic for owner/repo.
// Requires push access to the repository.
func (c *Client) GetTrafficClones(owner, repo string) (count, uniques int, err error) {
	clones, _, err := c.gh.Repositories.ListTrafficClones(c.ctx, owner, repo, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("traffic clones for %s/%s: %w", owner, repo, err)
	}
	return clones.GetCount(), clones.GetUniques(), nil
}

// ListDirectory returns the names of files in a repo directory.
func (c *Client) ListDirectory(owner, repo, path string) ([]string, error) {
	_, entries, _, err := c.gh.Repositories.GetContents(c.ctx, owner, repo, path, nil)
	if err != nil {
		return nil, fmt.Errorf("listing %s/%s/%s: %w", owner, repo, path, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.GetType() == "file" && strings.HasSuffix(e.GetName(), ".rb") {
			names = append(names, e.GetName())
		}
	}
	return names, nil
}

// GetFileContent fetches the raw content of a file in a repo.
func (c *Client) GetFileContent(owner, repo, path string) (string, error) {
	fc, _, _, err := c.gh.Repositories.GetContents(c.ctx, owner, repo, path, nil)
	if err != nil {
		return "", fmt.Errorf("get file %s/%s/%s: %w", owner, repo, path, err)
	}
	content, err := fc.GetContent()
	if err != nil {
		return "", fmt.Errorf("decode %s/%s/%s: %w", owner, repo, path, err)
	}
	return content, nil
}

// GetLatestReleaseTag returns the latest release tag for owner/repo.
// Returns empty string without error if no releases exist.
func (c *Client) GetLatestReleaseTag(owner, repo string) (string, error) {
	release, _, err := c.gh.Repositories.GetLatestRelease(c.ctx, owner, repo)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return "", nil
		}
		return "", fmt.Errorf("latest release for %s/%s: %w", owner, repo, err)
	}
	return release.GetTagName(), nil
}

// GetTotalDownloads sums asset download counts across all releases for owner/repo.
// Returns 0 without error if the repo has no releases.
func (c *Client) GetTotalDownloads(owner, repo string) (int64, error) {
	opt := &gh.ListOptions{PerPage: 100}
	var total int64
	for {
		releases, resp, err := c.gh.Repositories.ListReleases(c.ctx, owner, repo, opt)
		if err != nil {
			if strings.Contains(err.Error(), "404") {
				return 0, nil
			}
			return 0, fmt.Errorf("list releases for %s/%s: %w", owner, repo, err)
		}
		for _, r := range releases {
			for _, a := range r.Assets {
				total += int64(a.GetDownloadCount())
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return total, nil
}
