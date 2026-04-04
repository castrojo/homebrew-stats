package testhub

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	ghcli "github.com/castrojo/bootc-ecosystem/internal/ghcli"
)

// parseJobApp extracts the app name from a job name like "compile-oci (ghostty, x86_64)".
// Returns ("", false) if the job is aarch64 or doesn't match the pattern.
func parseJobApp(jobName string) (string, bool) {
	re := regexp.MustCompile(`^compile-oci \((.+),\s*(x86_64|aarch64)\)$`)
	m := re.FindStringSubmatch(jobName)
	if m == nil {
		return "", false
	}
	arch := m[2]
	if arch != "x86_64" {
		return "", false
	}
	return strings.TrimSpace(m[1]), true
}

// isTesthubPackage reports whether the package name belongs to the testhub namespace.
func isTesthubPackage(name string) bool {
	return strings.HasPrefix(name, "testhub/")
}

// stripTesthubPrefix removes the "testhub/" prefix from a package name.
func stripTesthubPrefix(name string) string {
	return strings.TrimPrefix(name, "testhub/")
}

// rawPkg is the minimal JSON shape from the packages list API.
type rawPkg struct {
	Name      string `json:"name"`
	HTMLURL   string `json:"html_url"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// rawVersion is the minimal JSON shape from the package versions API.
type rawVersion struct {
	Metadata struct {
		Container struct {
			Tags []string `json:"tags"`
		} `json:"container"`
	} `json:"metadata"`
}

type rawContentEntry struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	HTMLURL string `json:"html_url"`
}

// ListPackages fetches container packages from the given GitHub org.
// Only packages with the "testhub/" prefix are returned.
func ListPackages(org string) ([]Package, error) {
	out, err := ghcli.Run("api",
		fmt.Sprintf("orgs/%s/packages?package_type=container&per_page=100", org),
		"--paginate")
	if err != nil {
		return nil, fmt.Errorf("listing packages for %s: %w", org, err)
	}

	var rawPkgs []rawPkg
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for dec.More() {
		var page []rawPkg
		if err := dec.Decode(&page); err != nil {
			break
		}
		rawPkgs = append(rawPkgs, page...)
	}

	var all []Package
	for _, pkg := range rawPkgs {
		fullName := pkg.Name
		if !isTesthubPackage(fullName) {
			continue
		}
		name := stripTesthubPrefix(fullName)

		p := Package{
			Name:    name,
			HTMLURL: pkg.HTMLURL,
		}
		if pkg.CreatedAt != "" {
			if t, err := time.Parse(time.RFC3339, pkg.CreatedAt); err == nil {
				p.CreatedAt = t.UTC().Format(time.RFC3339)
			}
		}
		if pkg.UpdatedAt != "" {
			if t, err := time.Parse(time.RFC3339, pkg.UpdatedAt); err == nil {
				p.UpdatedAt = t.UTC().Format(time.RFC3339)
			}
		}

		// Paginate versions to get accurate count and latest tag.
		var versionCount int64
		page := 1
		escapedName := url.PathEscape(fullName)
		for {
			vout, verr := ghcli.Run("api",
				fmt.Sprintf("orgs/%s/packages/container/%s/versions?per_page=100&page=%d", org, escapedName, page))
			if verr != nil {
				break
			}
			var vers []rawVersion
			if err := json.Unmarshal(vout, &vers); err != nil {
				break
			}
			// Grab latest tag from first version on first page.
			if page == 1 && len(vers) > 0 && len(vers[0].Metadata.Container.Tags) > 0 {
				p.Version = vers[0].Metadata.Container.Tags[0]
			}
			versionCount += int64(len(vers))
			if len(vers) < 100 {
				break
			}
			page++
		}
		p.VersionCount = versionCount
		all = append(all, p)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].Name < all[j].Name
	})
	return all, nil
}

// ListFlatpakPackages returns package names from the testhub flatpaks directory.
// This endpoint is publicly readable and acts as a robust package inventory fallback
// when the organization Packages API is unavailable.
func ListFlatpakPackages(owner, repo string) ([]Package, error) {
	out, err := ghcli.Run("api", fmt.Sprintf("repos/%s/%s/contents/flatpaks", owner, repo))
	if err != nil {
		return nil, fmt.Errorf("listing flatpaks for %s/%s: %w", owner, repo, err)
	}
	var entries []rawContentEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		return nil, fmt.Errorf("parse flatpaks directory: %w", err)
	}

	pkgs := make([]Package, 0, len(entries))
	for _, e := range entries {
		if e.Type != "dir" {
			continue
		}
		if strings.EqualFold(e.Name, "TEMPLATE") {
			continue
		}
		pkgs = append(pkgs, Package{
			Name:    strings.ToLower(strings.TrimSpace(e.Name)),
			HTMLURL: e.HTMLURL,
		})
	}
	sort.Slice(pkgs, func(i, j int) bool {
		return pkgs[i].Name < pkgs[j].Name
	})
	return pkgs, nil
}

// MergePackages merges package metadata by name. Existing package records win;
// missing package names are backfilled from fallback.
func MergePackages(existing, fallback []Package) []Package {
	if len(existing) == 0 {
		out := append([]Package(nil), fallback...)
		sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
		return out
	}
	byName := make(map[string]Package, len(existing)+len(fallback))
	for _, p := range fallback {
		byName[p.Name] = p
	}
	for _, p := range existing {
		byName[p.Name] = p
	}

	out := make([]Package, 0, len(byName))
	for _, p := range byName {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ghRunRecord is the minimal shape from gh run list --json.
type ghRunRecord struct {
	DatabaseID int64  `json:"databaseId"`
	Status     string `json:"status"`
}

// ghJobRecord is the minimal shape from the jobs API.
type ghJobRecord struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Conclusion string `json:"conclusion"`
}

// FetchBuildCounts fetches workflow run job results from projectbluefin/testhub
// for runs with ID > lastRunID. Returns aggregated per-app counts and the new max run ID.
func FetchBuildCounts(lastRunID int64) ([]AppDayCount, int64, error) {
	out, err := ghcli.Run("run", "list",
		"--repo", "projectbluefin/testhub",
		"--workflow", "build.yml",
		"--json", "databaseId,status",
		"--limit", "50")
	if err != nil {
		return nil, lastRunID, fmt.Errorf("listing workflow runs: %w", err)
	}

	var runs []ghRunRecord
	if err := json.Unmarshal(out, &runs); err != nil {
		return nil, lastRunID, fmt.Errorf("parse runs: %w", err)
	}

	counts := make(map[string]*AppDayCount)
	newMaxRunID := lastRunID

	for _, run := range runs {
		runID := run.DatabaseID
		if runID <= lastRunID {
			continue
		}
		if runID > newMaxRunID {
			newMaxRunID = runID
		}

		// Fetch jobs for this run.
		jout, jerr := ghcli.Run("api",
			fmt.Sprintf("repos/projectbluefin/testhub/actions/runs/%d/jobs?per_page=100", runID),
			"--jq", ".jobs")
		if jerr != nil {
			fmt.Fprintf(os.Stderr, "⚠️  testhub jobs for run %d: %v\n", runID, jerr)
			continue
		}
		var jobs []ghJobRecord
		if err := json.Unmarshal(jout, &jobs); err != nil {
			continue
		}
		for _, job := range jobs {
			app, ok := parseJobApp(job.Name)
			if !ok {
				continue
			}
			if _, exists := counts[app]; !exists {
				counts[app] = &AppDayCount{App: app}
			}
			switch job.Conclusion {
			case "success":
				counts[app].Passed++
				counts[app].Total++
			case "failure", "cancelled":
				counts[app].Failed++
				counts[app].Total++
			}
		}
	}

	result := make([]AppDayCount, 0, len(counts))
	for _, c := range counts {
		result = append(result, *c)
	}
	return result, newMaxRunID, nil
}

// AppendSnapshot appends or overwrites today's snapshot in the store.
// Returns the updated store (does not mutate the input).
func AppendSnapshot(store *HistoryStore, pkgs []Package, counts []AppDayCount, lastRunID int64) *HistoryStore {
	today := time.Now().UTC().Format("2006-01-02")
	snap := DaySnapshot{
		Date:        today,
		Packages:    pkgs,
		BuildCounts: counts,
		LastRunID:   lastRunID,
	}

	newStore := &HistoryStore{}

	replaced := false
	for _, s := range store.Snapshots {
		if s.Date == today {
			newStore.Snapshots = append(newStore.Snapshots, snap)
			replaced = true
		} else {
			newStore.Snapshots = append(newStore.Snapshots, s)
		}
	}
	if !replaced {
		newStore.Snapshots = append(newStore.Snapshots, snap)
	}

	return newStore
}

// ComputeBuildMetrics computes pass rates from raw snapshot counts over a rolling window.
// The window is relative to the most recent snapshot date (not time.Now()), so that
// historical test data with fixed dates is handled consistently.
// This is a pure function — no I/O.
func ComputeBuildMetrics(snapshots []DaySnapshot, windowDays int) []BuildMetrics {
	if len(snapshots) == 0 {
		return nil
	}

	// Find the most recent snapshot date to anchor the rolling window.
	latestDate := ""
	for _, snap := range snapshots {
		if snap.Date > latestDate {
			latestDate = snap.Date
		}
	}
	latest, err := time.Parse("2006-01-02", latestDate)
	if err != nil {
		// Fallback to now if date is unparseable.
		latest = time.Now().UTC()
	}
	cutoff := latest.AddDate(0, 0, -windowDays)
	cutoffStr := cutoff.Format("2006-01-02")

	// Aggregate per app within window
	type agg struct {
		passed int
		failed int
		total  int
	}
	appData := make(map[string]*agg)

	for _, snap := range snapshots {
		if snap.Date < cutoffStr {
			continue
		}
		for _, c := range snap.BuildCounts {
			if _, ok := appData[c.App]; !ok {
				appData[c.App] = &agg{}
			}
			appData[c.App].passed += c.Passed
			appData[c.App].failed += c.Failed
			appData[c.App].total += c.Total
		}
	}

	result := make([]BuildMetrics, 0, len(appData))
	for app, a := range appData {
		var rate float64
		if a.total > 0 {
			rate = float64(a.passed) / float64(a.total) * 100.0
		}
		bm := BuildMetrics{
			App:        app,
			PassRate7d: rate,
		}
		result = append(result, bm)
	}

	return result
}
