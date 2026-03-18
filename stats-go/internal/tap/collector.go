// Package tap collects health data for a Homebrew tap repository.
package tap

import (
	"fmt"
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
	// SourceOwner/SourceRepo point to the upstream project for freshness checks.
	SourceOwner string `json:"source_owner,omitempty"`
	SourceRepo  string `json:"source_repo,omitempty"`
	// DownloadOwner/DownloadRepo point to the ublue-os tap repo whose release
	// assets host this package's download URL. Empty when the package is not
	// distributed via the tap's own releases.
	DownloadOwner string `json:"download_owner,omitempty"`
	DownloadRepo  string `json:"download_repo,omitempty"`
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

	// Freshness check and download count for each package with a detected GitHub source.
	for i := range ts.Packages {
		p := &ts.Packages[i]

		// Freshness: requires an upstream source and a pinned version.
		if p.SourceOwner != "" && p.SourceRepo != "" && p.Version != "" {
			latest, err := client.GetLatestReleaseTag(p.SourceOwner, p.SourceRepo)
			if err == nil && latest != "" {
				p.LatestVersion = normaliseVersion(latest)
				p.FreshnessKnown = true
				p.IsStale = p.LatestVersion != normaliseVersion(p.Version)
			}
		}

		// Downloads: only count assets served by the ublue-os tap repos themselves.
		// If the package URL does not resolve to a tap release, downloads stay 0.
		if p.DownloadOwner == "" || p.DownloadRepo == "" {
			continue
		}
		downloads, err := client.GetTotalDownloads(p.DownloadOwner, p.DownloadRepo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Downloads for %s/%s: %v\n", p.DownloadOwner, p.DownloadRepo, err)
		} else {
			p.Downloads = downloads
		}
	}

	// Sort packages by downloads descending (within type: casks then formulas).
	sort.Slice(ts.Packages, func(i, j int) bool {
		if ts.Packages[i].Type != ts.Packages[j].Type {
			return ts.Packages[i].Type < ts.Packages[j].Type // "cask" < "formula"
		}
		return ts.Packages[i].Downloads > ts.Packages[j].Downloads
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
	// Scan all GitHub URLs in the file:
	//   - ublue-os tap repos  → DownloadOwner/DownloadRepo (first match wins)
	//   - any other GitHub repo → SourceOwner/SourceRepo   (first match wins)
	tapRepos := map[string]bool{
		"homebrew-tap":              true,
		"homebrew-experimental-tap": true,
	}
	for _, m := range reGHURL.FindAllStringSubmatch(content, -1) {
		if len(m) < 3 {
			continue
		}
		owner := m[1]
		repo := strings.TrimSuffix(m[2], ".git")
		if owner == "ublue-os" && tapRepos[repo] {
			// This URL is hosted by the tap itself — count its downloads.
			if p.DownloadOwner == "" {
				p.DownloadOwner = owner
				p.DownloadRepo = repo
			}
		} else {
			// Non-tap URL — use as upstream source for freshness.
			if p.SourceOwner == "" {
				p.SourceOwner = owner
				p.SourceRepo = repo
			}
		}
	}
	return p
}

func normaliseVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}
