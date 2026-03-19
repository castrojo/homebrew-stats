package github

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	gh "github.com/google/go-github/v60/github"
	"golang.org/x/oauth2"
)

// TesthubPackage represents a Flatpak package from projectbluefin testhub.
type TesthubPackage struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	HTMLURL string `json:"html_url,omitempty"`
}

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
// Returns empty string without error if no releases exist (404).
func (c *Client) GetLatestReleaseTag(owner, repo string) (string, error) {
	release, _, err := c.gh.Repositories.GetLatestRelease(c.ctx, owner, repo)
	if err != nil {
		var ghErr *gh.ErrorResponse
		if errors.As(err, &ghErr) && ghErr.Response != nil && ghErr.Response.StatusCode == 404 {
			return "", nil
		}
		return "", fmt.Errorf("latest release for %s/%s: %w", owner, repo, err)
	}
	return release.GetTagName(), nil
}

// ListTesthubPackages returns container packages from the given GitHub org via the Packages API.
func (c *Client) ListTesthubPackages(org string) ([]TesthubPackage, error) {
	opts := &gh.PackageListOptions{
		PackageType: gh.String("container"),
		ListOptions: gh.ListOptions{PerPage: 100},
	}

	var all []TesthubPackage
	for {
		pkgs, resp, err := c.gh.Organizations.ListPackages(c.ctx, org, opts)
		if err != nil {
			return nil, fmt.Errorf("listing packages for %s: %w", org, err)
		}

		for _, pkg := range pkgs {
			name := pkg.GetName()
			tp := TesthubPackage{
				Name:    name,
				HTMLURL: pkg.GetHTMLURL(),
			}

			versions, _, verErr := c.gh.Organizations.PackageGetAllVersions(
				c.ctx, org, "container", name,
				&gh.PackageListOptions{ListOptions: gh.ListOptions{PerPage: 1}},
			)
			if verErr == nil && len(versions) > 0 {
				meta := versions[0].GetMetadata()
				if meta != nil && meta.Container != nil && len(meta.Container.Tags) > 0 {
					tp.Version = meta.Container.Tags[0]
				}
			}

			all = append(all, tp)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return all, nil
}

// GitHub returns the underlying go-github client for direct API access in subpackages.
func (c *Client) GitHub() *gh.Client { return c.gh }

// Context returns the client's context.
func (c *Client) Context() context.Context { return c.ctx }

// GetTotalDownloads was removed. Install counts now come from Homebrew analytics.
