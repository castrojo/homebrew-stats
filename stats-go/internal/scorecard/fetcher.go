// Package scorecard fetches OpenSSF Scorecard scores for tracked repos
// via the public securityscorecards.dev REST API (no auth required).
package scorecard

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// RepoResult holds the scorecard result for a single repository.
// Score and Date are pointer types so they marshal as JSON null (not 0.0/"")
// when the repo is not indexed.
type RepoResult struct {
	Repo    string   `json:"repo"`
	Score   *float64 `json:"score"` // null when not indexed
	Date    *string  `json:"date"`  // null when not indexed
	Indexed bool     `json:"indexed"`
}

// Output is the top-level structure written to src/data/scorecard.json.
type Output struct {
	GeneratedAt string       `json:"generated_at"`
	Results     []RepoResult `json:"results"`
}

// apiResponse mirrors the fields we care about from the Scorecard API JSON body.
type apiResponse struct {
	Score float64 `json:"score"`
	Date  string  `json:"date"`
}

// FetchAll fetches Scorecard data for each repo in the list and returns an Output.
// Each repo string must be in "org/repo" form (e.g. "ublue-os/bluefin").
// Individual repo failures (404, 429, network errors) are logged to stderr and
// result in a not-indexed entry; they do NOT abort the whole run.
func FetchAll(repos []string) (Output, error) {
	// MUST be initialized as a non-nil slice — Go marshals nil as JSON null,
	// which crashes Astro's map() calls.
	results := []RepoResult{}

	client := &http.Client{Timeout: 15 * time.Second}

	for _, repo := range repos {
		parts := strings.SplitN(repo, "/", 2)
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "⚠️  scorecard: malformed repo %q — expected org/repo, skipping\n", repo)
			results = append(results, RepoResult{Repo: repo, Indexed: false})
			continue
		}
		org, name := parts[0], parts[1]

		// CRITICAL: org and name must be separate path segments.
		// Joining them as a single string would URL-encode the "/" → 404.
		url := fmt.Sprintf("https://api.securityscorecards.dev/projects/github.com/%s/%s", org, name)

		result := fetchOne(client, repo, url)
		results = append(results, result)
	}

	return Output{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Results:     results,
	}, nil
}

// fetchOne performs a single GET and returns the RepoResult.
func fetchOne(client *http.Client, repo, url string) RepoResult {
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  scorecard: %s: request failed: %v\n", repo, err)
		return RepoResult{Repo: repo, Indexed: false}
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// fall through to parse
	case http.StatusNotFound:
		fmt.Fprintf(os.Stderr, "⚠️  scorecard: %s: not indexed (404)\n", repo)
		return RepoResult{Repo: repo, Indexed: false}
	case http.StatusTooManyRequests:
		fmt.Fprintf(os.Stderr, "⚠️  scorecard: %s: rate limited (429), skipping\n", repo)
		return RepoResult{Repo: repo, Indexed: false}
	default:
		fmt.Fprintf(os.Stderr, "⚠️  scorecard: %s: unexpected status %d\n", repo, resp.StatusCode)
		return RepoResult{Repo: repo, Indexed: false}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  scorecard: %s: failed to read body: %v\n", repo, err)
		return RepoResult{Repo: repo, Indexed: false}
	}

	var apiResp apiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  scorecard: %s: malformed JSON: %v\n", repo, err)
		return RepoResult{Repo: repo, Indexed: false}
	}

	score := apiResp.Score
	date := apiResp.Date
	return RepoResult{
		Repo:    repo,
		Score:   &score,
		Date:    &date,
		Indexed: true,
	}
}
