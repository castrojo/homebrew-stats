// Package tap collects health data for a Homebrew tap repository.
package tap

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	ghclient "github.com/castrojo/homebrew-stats/internal/github"
)

// Traffic holds 14-day clone traffic for a tap.
type Traffic struct {
	Count   int    `json:"count"`
	Uniques int    `json:"uniques"`
	Window  string `json:"window"`
}

// Package represents a single cask or formula in the tap.
type Package struct {
	Name           string `json:"name"`
	Type           string `json:"type"` // "cask" or "formula"
	Version        string `json:"version,omitempty"`
	LatestVersion  string `json:"latest_version,omitempty"`
	IsStale        bool   `json:"is_stale"`
	FreshnessKnown bool   `json:"freshness_known"`
	Downloads      int64  `json:"downloads"`
	Description    string `json:"description,omitempty"`
	Homepage       string `json:"homepage,omitempty"`
	SourceOwner    string `json:"source_owner,omitempty"`
	SourceRepo     string `json:"source_repo,omitempty"`
}

// StatusString returns "current", "stale", or "unknown".
func (p *Package) StatusString() string {
	if !p.FreshnessKnown {
		return "unknown"
	}
	if p.IsStale {
		return "stale"
	}
	return "current"
}

// TapStats holds all collected data for one tap.
type TapStats struct {
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Traffic   *Traffic  `json:"traffic,omitempty"`
	Packages  []Package `json:"packages"`
	UpdatedAt string    `json:"updated_at"`
}

var (
	reVersion  = regexp.MustCompile(`(?m)^\s*version\s+"([^"]+)"`)
	reDesc     = regexp.MustCompile(`(?m)^\s*desc\s+"([^"]+)"`)
	reHomepage = regexp.MustCompile(`(?m)^\s*homepage\s+"([^"]+)"`)
	reGHURL    = regexp.MustCompile(`github\.com/([^/]+)/([^/\s"#]+)`)
)

// Collect fetches traffic and package data for the given owner/repo tap.
func Collect(owner, repo string, client *ghclient.Client) (*TapStats, error) {
	name := owner + "/" + repo
	ts := &TapStats{
		Name:      name,
		URL:       "https://github.com/" + name,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// Traffic.
	count, uniques, err := client.GetTrafficClones(owner, repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Traffic for %s: %v\n", name, err)
	} else {
		ts.Traffic = &Traffic{Count: count, Uniques: uniques, Window: "14 days"}
	}

	// Packages — Casks first, then Formula.
	dirs := []struct {
		path    string
		pkgType string
	}{
		{"Casks", "cask"},
		{"Formula", "formula"},
	}

	for _, d := range dirs {
		files, err := client.ListDirectory(owner, repo, d.path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Listing %s/%s: %v\n", name, d.path, err)
			continue
		}
		for _, filename := range files {
			pkgName := strings.TrimSuffix(filename, ".rb")
			content, err := client.GetFileContent(owner, repo, d.path+"/"+filename)
			if err != nil {
				ts.Packages = append(ts.Packages, Package{Name: pkgName, Type: d.pkgType})
				continue
			}
			pkg := parseRuby(pkgName, d.pkgType, content)
			ts.Packages = append(ts.Packages, pkg)
		}
	}

	// Fetch Homebrew Analytics install counts for this tap's package type.
	caskAnalytics, err := fetchBrewAnalytics("cask")
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Brew cask analytics: %v\n", err)
	}
	formulaAnalytics, err := fetchBrewAnalytics("formula")
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Brew formula analytics: %v\n", err)
	}

	// Freshness check and download lookup for each package.
	for i := range ts.Packages {
		p := &ts.Packages[i]

		// Homebrew Analytics 30-day install count.
		if p.Type == "cask" {
			p.Downloads = caskAnalytics[p.Name]
		} else {
			p.Downloads = formulaAnalytics[p.Name]
		}

		// Freshness check — only for packages with a detected GitHub source.
		if p.SourceOwner == "" || p.SourceRepo == "" || p.Version == "" {
			continue
		}
		latest, err := client.GetLatestReleaseTag(p.SourceOwner, p.SourceRepo)
		if err != nil || latest == "" {
			continue
		}
		p.LatestVersion = normaliseVersion(latest)
		p.FreshnessKnown = true
		p.IsStale = p.LatestVersion != normaliseVersion(p.Version)
	}

	// Sort packages alphabetically within type (casks then formulas).
	sort.Slice(ts.Packages, func(i, j int) bool {
		if ts.Packages[i].Type != ts.Packages[j].Type {
			return ts.Packages[i].Type < ts.Packages[j].Type // "cask" < "formula"
		}
		return ts.Packages[i].Name < ts.Packages[j].Name
	})

	return ts, nil
}

func parseRuby(name, pkgType, content string) Package {
	p := Package{Name: name, Type: pkgType}
	if m := reVersion.FindStringSubmatch(content); len(m) > 1 {
		p.Version = m[1]
	}
	if m := reDesc.FindStringSubmatch(content); len(m) > 1 {
		p.Description = m[1]
	}
	if m := reHomepage.FindStringSubmatch(content); len(m) > 1 {
		p.Homepage = m[1]
	}
	// Detect GitHub source from url or homepage.
	for _, m := range reGHURL.FindAllStringSubmatch(content, -1) {
		if len(m) >= 3 {
			owner := m[1]
			repo := strings.TrimSuffix(m[2], ".git")
			// Skip the tap repo itself and common non-source hosts.
			if owner == "ublue-os" && (repo == "homebrew-tap" || repo == "homebrew-experimental-tap") {
				continue
			}
			p.SourceOwner = owner
			p.SourceRepo = repo
			break
		}
	}
	return p
}

func normaliseVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

// fetchBrewAnalytics fetches 30-day install counts from the Homebrew Analytics API.
// pkgType must be "cask" or "formula". Returns a map of package name → install count.
func fetchBrewAnalytics(pkgType string) (map[string]int64, error) {
	endpoint := "https://formulae.brew.sh/api/analytics/install/30d.json"
	if pkgType == "cask" {
		endpoint = "https://formulae.brew.sh/api/analytics/cask-install/30d.json"
	}

	resp, err := http.Get(endpoint) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("fetching brew analytics: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Items []struct {
			Cask    string `json:"cask"`
			Formula string `json:"formula"`
			Count   string `json:"count"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding brew analytics: %w", err)
	}

	counts := make(map[string]int64, len(result.Items))
	for _, item := range result.Items {
		name := item.Cask
		if name == "" {
			name = item.Formula
		}
		// Count is formatted with commas (e.g. "159,172") — strip them.
		cleaned := strings.ReplaceAll(item.Count, ",", "")
		var n int64
		fmt.Sscan(cleaned, &n)
		counts[name] = n
	}
	return counts, nil
}
