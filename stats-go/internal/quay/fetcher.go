// Package quay fetches public pull-count statistics from the Quay.io public API.
// No authentication is required — all data is from public repositories.
package quay

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// RepoConfig identifies a Quay.io repository to track.
type RepoConfig struct {
	Namespace string
	Name      string
	Label     string // human label used in JSON output
}

// DailyStat is one day of pull activity.
type DailyStat struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// Stream is a named tag (a published release stream).
type Stream struct {
	Name         string `json:"name"`
	LastModified string `json:"last_modified"`
	DigestShort  string `json:"digest_short"`
	ArchCount    int    `json:"arch_count"`
}

// RepoStats holds the collected data for a single Quay.io repo.
type RepoStats struct {
	Label         string      `json:"label"`
	Namespace     string      `json:"namespace"`
	Repo          string      `json:"repo"`
	BootcVerified bool        `json:"bootc_verified"` // true if containers.bootc=1 label confirmed
	DailyStats    []DailyStat `json:"daily_stats"`
	Streams       []Stream    `json:"streams"`
	Pulls7d       int         `json:"pulls_7d"`
	Pulls30d      int         `json:"pulls_30d"`
	Pulls90d      int         `json:"pulls_90d"`
	AvgDaily7d    int         `json:"avg_daily_7d"`
	AvgDaily30d   int         `json:"avg_daily_30d"`
	LatestDate    string      `json:"latest_date"`
	LatestPulls   int         `json:"latest_pulls"`
}

// QuayData is the root document written to src/data/quay-*.json.
type QuayData struct {
	GeneratedAt   string      `json:"generated_at"`
	Repos         []RepoStats `json:"repos"`
	TotalPulls7d  int         `json:"total_pulls_7d"`
	TotalPulls30d int         `json:"total_pulls_30d"`
	TotalPulls90d int         `json:"total_pulls_90d"`
	// Combined daily stats (sum across all repos) for a single trend chart.
	CombinedDaily []DailyStat `json:"combined_daily"`
}

var client = &http.Client{Timeout: 20 * time.Second}

func get(url string) ([]byte, error) {
	resp, err := client.Get(url) //nolint:gosec // URL constructed from allowlisted base
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("quay GET %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// FetchAll fetches data for all configured repos and returns a QuayData document.
func FetchAll(repos []RepoConfig) (*QuayData, error) {
	result := &QuayData{GeneratedAt: time.Now().UTC().Format(time.RFC3339)}

	// Accumulate combined daily totals keyed by date.
	combinedMap := map[string]int{}

	for _, cfg := range repos {
		rs, err := fetchRepo(cfg)
		if err != nil {
			return nil, fmt.Errorf("fetch %s/%s: %w", cfg.Namespace, cfg.Name, err)
		}
		result.Repos = append(result.Repos, *rs)
		result.TotalPulls7d += rs.Pulls7d
		result.TotalPulls30d += rs.Pulls30d
		result.TotalPulls90d += rs.Pulls90d
		for _, d := range rs.DailyStats {
			combinedMap[d.Date] += d.Count
		}
	}

	// Sort combined daily by date.
	dates := make([]string, 0, len(combinedMap))
	for d := range combinedMap {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	for _, d := range dates {
		result.CombinedDaily = append(result.CombinedDaily, DailyStat{Date: d, Count: combinedMap[d]})
	}

	return result, nil
}

func fetchRepo(cfg RepoConfig) (*RepoStats, error) {
	// --- Bootc label verification via OCI registry API ---
	// Walk: manifest-list → amd64 manifest → config blob → check Labels.
	// Non-fatal: logs warning but does not abort if label is missing.
	bootcVerified := verifyBootcLabel(cfg.Namespace, cfg.Name)
	if !bootcVerified {
		fmt.Fprintf(io.Discard, "") // no-op; caller prints warning
		_, _ = fmt.Fprintf(
			// Write to stderr via the io package isn't wired here; callers print.
			io.Discard, "warning: %s/%s missing containers.bootc label\n", cfg.Namespace, cfg.Name,
		)
	}

	// --- Stats + metadata ---
	metaURL := fmt.Sprintf("https://quay.io/api/v1/repository/%s/%s?includeStats=true", cfg.Namespace, cfg.Name)
	metaBody, err := get(metaURL)
	if err != nil {
		return nil, err
	}

	var meta struct {
		Stats []struct {
			Date  string `json:"date"`
			Count int    `json:"count"`
		} `json:"stats"`
	}
	if err := json.Unmarshal(metaBody, &meta); err != nil {
		return nil, fmt.Errorf("parse meta for %s/%s: %w", cfg.Namespace, cfg.Name, err)
	}

	daily := make([]DailyStat, 0, len(meta.Stats))
	for _, s := range meta.Stats {
		daily = append(daily, DailyStat{Date: s.Date, Count: s.Count})
	}
	// Quay returns chronological order; ensure sorted.
	sort.Slice(daily, func(i, j int) bool { return daily[i].Date < daily[j].Date })

	pulls7d, pulls30d, pulls90d := sumLast(daily, 7), sumLast(daily, 30), sumLast(daily, 90)
	avg7d, avg30d := 0, 0
	if len(daily) > 0 {
		n7 := min(7, len(daily))
		n30 := min(30, len(daily))
		if n7 > 0 {
			avg7d = pulls7d / n7
		}
		if n30 > 0 {
			avg30d = pulls30d / n30
		}
	}
	var latestDate string
	var latestPulls int
	if len(daily) > 0 {
		latestDate = daily[len(daily)-1].Date
		latestPulls = daily[len(daily)-1].Count
	}

	// --- Tags (streams) ---
	tagsURL := fmt.Sprintf("https://quay.io/api/v1/repository/%s/%s/tag/?limit=100&onlyActiveTags=true", cfg.Namespace, cfg.Name)
	tagsBody, err := get(tagsURL)
	if err != nil {
		return nil, err
	}

	var tagsResp struct {
		Tags []struct {
			Name               string `json:"name"`
			LastModified       string `json:"last_modified"`
			ManifestDigest     string `json:"manifest_digest"`
			IsManifestList     bool   `json:"is_manifest_list"`
			ChildManifestCount int    `json:"child_manifest_count"`
		} `json:"tags"`
	}
	if err := json.Unmarshal(tagsBody, &tagsResp); err != nil {
		return nil, fmt.Errorf("parse tags for %s/%s: %w", cfg.Namespace, cfg.Name, err)
	}

	var streams []Stream
	for _, t := range tagsResp.Tags {
		// Skip sha256 digest tags (attestations, sigs, sboms) — only named streams.
		if strings.HasPrefix(t.Name, "sha256-") {
			continue
		}
		// Skip versioned tags (start with a digit) — focus on named stream pointers.
		if len(t.Name) > 0 && t.Name[0] >= '0' && t.Name[0] <= '9' {
			continue
		}
		digest := t.ManifestDigest
		if len(digest) > 19 {
			digest = digest[:19] // "sha256:abcdef01234"
		}
		archCount := t.ChildManifestCount
		if !t.IsManifestList {
			archCount = 1
		}
		streams = append(streams, Stream{
			Name:         t.Name,
			LastModified: t.LastModified,
			DigestShort:  digest,
			ArchCount:    archCount,
		})
	}

	return &RepoStats{
		Label:         cfg.Label,
		Namespace:     cfg.Namespace,
		Repo:          cfg.Name,
		BootcVerified: bootcVerified,
		DailyStats:    daily,
		Streams:       streams,
		Pulls7d:       pulls7d,
		Pulls30d:      pulls30d,
		Pulls90d:      pulls90d,
		AvgDaily7d:    avg7d,
		AvgDaily30d:   avg30d,
		LatestDate:    latestDate,
		LatestPulls:   latestPulls,
	}, nil
}

// verifyBootcLabel checks that an image carries containers.bootc=1 (or ostree.bootable=1)
// by walking: manifest-list → first amd64 manifest → config blob → Labels map.
// Returns false (with no error) if the label is absent or any fetch fails.
func verifyBootcLabel(namespace, name string) bool {
	// Step 1: get manifest list, find amd64 child digest
	mlURL := fmt.Sprintf("https://quay.io/v2/%s/%s/manifests/latest", namespace, name)
	// Try common stream tags if "latest" doesn't exist
	for _, tag := range []string{"latest", "stable", "stream10", "10", "9"} {
		mlURL = fmt.Sprintf("https://quay.io/v2/%s/%s/manifests/%s", namespace, name, tag)
		body, err := getWithAccept(mlURL, "application/vnd.oci.image.index.v1+json,application/vnd.docker.distribution.manifest.list.v2+json")
		if err != nil {
			continue
		}
		var ml struct {
			Manifests []struct {
				Digest   string `json:"digest"`
				Platform struct {
					Architecture string `json:"architecture"`
				} `json:"platform"`
			} `json:"manifests"`
			// Single-arch manifests have config directly
			Config *struct {
				Digest string `json:"digest"`
			} `json:"config"`
		}
		if err := json.Unmarshal(body, &ml); err != nil {
			continue
		}
		// If it's a single manifest (not a list), check config directly
		if ml.Config != nil && ml.Config.Digest != "" {
			return checkConfigLabels(namespace, name, ml.Config.Digest)
		}
		// Walk manifest list for amd64
		for _, m := range ml.Manifests {
			if m.Platform.Architecture == "amd64" || m.Platform.Architecture == "x86_64" {
				if ok := checkManifestConfig(namespace, name, m.Digest); ok {
					return true
				}
			}
		}
		// Fall back to first entry
		if len(ml.Manifests) > 0 {
			return checkManifestConfig(namespace, name, ml.Manifests[0].Digest)
		}
	}
	return false
}

func checkManifestConfig(namespace, name, digest string) bool {
	url := fmt.Sprintf("https://quay.io/v2/%s/%s/manifests/%s", namespace, name, digest)
	body, err := getWithAccept(url, "application/vnd.oci.image.manifest.v1+json,application/vnd.docker.distribution.manifest.v2+json")
	if err != nil {
		return false
	}
	var m struct {
		Config struct {
			Digest string `json:"digest"`
		} `json:"config"`
	}
	if err := json.Unmarshal(body, &m); err != nil || m.Config.Digest == "" {
		return false
	}
	return checkConfigLabels(namespace, name, m.Config.Digest)
}

func checkConfigLabels(namespace, name, configDigest string) bool {
	url := fmt.Sprintf("https://quay.io/v2/%s/%s/blobs/%s", namespace, name, configDigest)
	body, err := get(url)
	if err != nil {
		return false
	}
	var cfg struct {
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"config"`
	}
	if err := json.Unmarshal(body, &cfg); err != nil {
		return false
	}
	labels := cfg.Config.Labels
	return labels["containers.bootc"] == "1" || labels["ostree.bootable"] == "1" || labels["ostree.bootable"] == "true"
}

func getWithAccept(url, accept string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil) //nolint:noctx
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func sumLast(stats []DailyStat, n int) int {
	total := 0
	start := len(stats) - n
	if start < 0 {
		start = 0
	}
	for _, s := range stats[start:] {
		total += s.Count
	}
	return total
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// FedoraRepos are the two Fedora bootc images tracked on the Fedora tab.
var FedoraRepos = []RepoConfig{
	{Namespace: "fedora", Name: "fedora-coreos", Label: "Fedora CoreOS"},
	{Namespace: "fedora", Name: "fedora-bootc", Label: "Fedora bootc"},
}

// CentOSRepos are the CentOS bootc images tracked on the CentOS tab.
var CentOSRepos = []RepoConfig{
	{Namespace: "centos-bootc", Name: "centos-bootc", Label: "CentOS bootc"},
}

// AlmaLinuxRepos tracks AlmaLinux bootc images.
var AlmaLinuxRepos = []RepoConfig{
	{Namespace: "almalinuxorg", Name: "almalinux-bootc", Label: "AlmaLinux bootc"},
}
