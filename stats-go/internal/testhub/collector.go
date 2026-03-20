package testhub

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	gh "github.com/google/go-github/v60/github"
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

// ListPackages fetches container packages from the given GitHub org.
// Only packages with the "testhub/" prefix are returned; the prefix is stripped
// from the display name. Version count is determined by paginating
// PackageGetAllVersions rather than using GetVersionCount() which always returns 0
// from the list API.
func ListPackages(ctx context.Context, client *gh.Client, org string) ([]Package, error) {
	opts := &gh.PackageListOptions{
		PackageType: gh.String("container"),
		ListOptions: gh.ListOptions{PerPage: 100},
	}

	var all []Package
	for {
		pkgs, resp, err := client.Organizations.ListPackages(ctx, org, opts)
		if err != nil {
			return nil, fmt.Errorf("listing packages for %s: %w", org, err)
		}

		for _, pkg := range pkgs {
			fullName := pkg.GetName()
			if !isTesthubPackage(fullName) {
				continue
			}
			name := stripTesthubPrefix(fullName)

			p := Package{
				Name:    name,
				HTMLURL: pkg.GetHTMLURL(),
				// PullCount: the GitHub Packages API (go-github v60) does not expose
				// a pull/download count for container packages. The field is reserved
				// for when that data becomes available via the API.
			}
			if t := pkg.GetCreatedAt(); !t.IsZero() {
				p.CreatedAt = t.UTC().Format(time.RFC3339)
			}
			if t := pkg.GetUpdatedAt(); !t.IsZero() {
				p.UpdatedAt = t.UTC().Format(time.RFC3339)
			}

			// Paginate versions to get an accurate count and extract the latest tag.
			var versionCount int64
			vPage := 1
			for {
				vers, vResp, vErr := client.Organizations.PackageGetAllVersions(
					ctx, org, "container", fullName,
					&gh.PackageListOptions{ListOptions: gh.ListOptions{PerPage: 100, Page: vPage}},
				)
				if vErr != nil {
					break
				}
				// Grab the latest tag from the first version on the first page.
				if vPage == 1 && len(vers) > 0 {
					meta := vers[0].GetMetadata()
					if meta != nil && meta.Container != nil && len(meta.Container.Tags) > 0 {
						p.Version = meta.Container.Tags[0]
					}
				}
				versionCount += int64(len(vers))
				if vResp.NextPage == 0 {
					break
				}
				vPage = vResp.NextPage
			}
			p.VersionCount = versionCount

			all = append(all, p)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return all, nil
}

// FetchBuildCounts fetches workflow run job results from projectbluefin/testhub
// for runs with ID > lastRunID. Returns aggregated per-app counts and the new max run ID.
func FetchBuildCounts(ctx context.Context, client *gh.Client, lastRunID int64) ([]AppDayCount, int64, error) {
	opts := &gh.ListWorkflowRunsOptions{
		ListOptions: gh.ListOptions{PerPage: 50},
	}

	counts := make(map[string]*AppDayCount)
	newMaxRunID := lastRunID

	for {
		runs, resp, err := client.Actions.ListWorkflowRunsByFileName(
			ctx, "projectbluefin", "testhub", "build.yml", opts,
		)
		if err != nil {
			return nil, newMaxRunID, fmt.Errorf("listing workflow runs: %w", err)
		}

		allOld := true
		for _, run := range runs.WorkflowRuns {
			runID := run.GetID()
			if runID <= lastRunID {
				continue
			}
			allOld = false

			if runID > newMaxRunID {
				newMaxRunID = runID
			}

			// Fetch jobs for this run.
			jobOpts := &gh.ListWorkflowJobsOptions{
				ListOptions: gh.ListOptions{PerPage: 100},
			}
			for {
				jobs, jResp, jErr := client.Actions.ListWorkflowJobs(ctx, "projectbluefin", "testhub", runID, jobOpts)
				if jErr != nil {
					fmt.Fprintf(os.Stderr, "⚠️  testhub jobs for run %d: %v\n", runID, jErr)
					break
				}
				for _, job := range jobs.Jobs {
					app, ok := parseJobApp(job.GetName())
					if !ok {
						continue
					}
					if _, exists := counts[app]; !exists {
						counts[app] = &AppDayCount{App: app}
					}
					switch job.GetConclusion() {
					case "success":
						counts[app].Passed++
						counts[app].Total++
					case "failure", "cancelled":
						counts[app].Failed++
						counts[app].Total++
					}
				}
				if jResp.NextPage == 0 {
					break
				}
				jobOpts.Page = jResp.NextPage
			}
		}

		if allOld || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
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
