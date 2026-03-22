package builds

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	gh "github.com/google/go-github/v60/github"
)

// CollectorConfig holds configuration for the builds data collector.
type CollectorConfig struct {
	Repos        []RepoConfig
	LookbackDays int
	HistoryPath  string
	OutputPath   string
}

// Collector fetches GitHub Actions data and writes builds.json.
type Collector struct {
	client *gh.Client
	cfg    CollectorConfig
}

// NewCollector creates a new Collector.
func NewCollector(client *gh.Client, cfg CollectorConfig) *Collector {
	return &Collector{client: client, cfg: cfg}
}

// Run executes the full data collection pipeline.
func (c *Collector) Run(ctx context.Context) error {
	// 1. Load existing history
	history, err := c.loadHistory()
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  builds: could not load history (starting fresh): %v\n", err)
		history = &BuildsHistory{}
	}

	// 2. Build known ID set for deduplication
	knownIDs := make(map[int64]bool, len(history.Runs))
	for _, r := range history.Runs {
		knownIDs[r.ID] = true
	}

	// 3. Determine since date
	since := time.Now().UTC().AddDate(0, 0, -c.cfg.LookbackDays)
	if len(history.Runs) > 0 {
		// Parse latest run date and go back 1 day for overlap
		latest, parseErr := time.Parse(time.RFC3339, history.Runs[len(history.Runs)-1].CreatedAt)
		if parseErr == nil {
			since = latest.AddDate(0, 0, -1)
		}
	}

	// 4. Fetch new runs for each repo/workflow
	var newRuns []WorkflowRunRecord
	for _, repo := range c.cfg.Repos {
		for _, wfFile := range repo.WorkflowFiles {
			runs, fetchErr := c.fetchRuns(ctx, repo, wfFile, since, knownIDs)
			if fetchErr != nil {
				fmt.Fprintf(os.Stderr, "⚠️  builds: fetch %s/%s/%s: %v\n",
					repo.Owner, repo.Repo, wfFile, fetchErr)
				continue
			}
			newRuns = append(newRuns, runs...)
		}
	}

	// 5. Append, sort, prune
	history.Runs = append(history.Runs, newRuns...)
	sort.Slice(history.Runs, func(i, j int) bool {
		return history.Runs[i].CreatedAt < history.Runs[j].CreatedAt
	})
	cutoff := time.Now().UTC().AddDate(0, -6, 0).Format(time.RFC3339)
	filtered := history.Runs[:0]
	for _, r := range history.Runs {
		if r.CreatedAt >= cutoff {
			filtered = append(filtered, r)
		}
	}
	history.Runs = filtered

	// 6. Compute all metrics
	output := BuildsOutput{
		GeneratedAt:      time.Now().UTC().Format(time.RFC3339),
		Summary:          ComputeSummary(history.Runs),
		DORAMetrics:      computeDORA(history.Runs),
		TopFlaky:         ComputeTopFlaky(history.Runs, 10),
		RecentBuilds:     buildRecentBuilds(history.Runs, 20),
		DurationTrend:    ComputeDurationTrend(history.Runs, 90),
		FailureBreakdown: ComputeFailureBreakdown(history.Runs),
		TriggerBreakdown: ComputeTriggerBreakdown(history.Runs),
		History:          computeDailySnapshots(history.Runs),
		Repos:            computeAllRepoMetrics(history.Runs, c.cfg.Repos),
	}

	// 7. Write output
	if err := c.writeJSON(c.cfg.OutputPath, output); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	// 8. Save history
	if err := c.writeJSON(c.cfg.HistoryPath, history); err != nil {
		return fmt.Errorf("saving history: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✅ fetch-builds: %d repos, %d new runs, %d total in history\n",
		len(c.cfg.Repos), len(newRuns), len(history.Runs))
	return nil
}

// fetchRuns fetches completed runs for one workflow file since `since`.
func (c *Collector) fetchRuns(ctx context.Context, repo RepoConfig, wfFile string,
	since time.Time, knownIDs map[int64]bool) ([]WorkflowRunRecord, error) {

	opts := &gh.ListWorkflowRunsOptions{
		Status:      "completed",
		Created:     ">=" + since.Format("2006-01-02"),
		ListOptions: gh.ListOptions{PerPage: 100},
	}

	var records []WorkflowRunRecord
	for {
		runs, resp, err := c.client.Actions.ListWorkflowRunsByFileName(
			ctx, repo.Owner, repo.Repo, wfFile, opts)
		if err != nil {
			return nil, err
		}

		for _, run := range runs.WorkflowRuns {
			id := run.GetID()
			if knownIDs[id] {
				continue
			}

			startedAt := run.GetRunStartedAt().Time
			completedAt := run.GetUpdatedAt().Time
			createdAt := run.GetCreatedAt().Time

			durationSec := 0
			if !completedAt.IsZero() && !startedAt.IsZero() {
				durationSec = int(completedAt.Sub(startedAt).Seconds())
			}
			queueSec := 0
			if !startedAt.IsZero() && !createdAt.IsZero() && startedAt.After(createdAt) {
				queueSec = int(startedAt.Sub(createdAt).Seconds())
			}

			record := WorkflowRunRecord{
				ID:           id,
				Repo:         repo.Label,
				WorkflowName: run.GetName(),
				WorkflowFile: wfFile,
				RunNumber:    run.GetRunNumber(),
				Event:        run.GetEvent(),
				Branch:       run.GetHeadBranch(),
				Conclusion:   run.GetConclusion(),
				CreatedAt:    createdAt.Format(time.RFC3339),
				StartedAt:    startedAt.Format(time.RFC3339),
				CompletedAt:  completedAt.Format(time.RFC3339),
				DurationSec:  durationSec,
				QueueTimeSec: queueSec,
			}

			// Fetch jobs for this run
			jobs, jobErr := c.fetchJobs(ctx, repo, id)
			if jobErr != nil {
				fmt.Fprintf(os.Stderr, "⚠️  builds: jobs for run %d: %v\n", id, jobErr)
			} else {
				record.Jobs = jobs
			}

			records = append(records, record)
			knownIDs[id] = true
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return records, nil
}

// fetchJobs fetches all jobs (with steps) for a given run.
func (c *Collector) fetchJobs(ctx context.Context, repo RepoConfig, runID int64) ([]JobRecord, error) {
	opts := &gh.ListWorkflowJobsOptions{
		Filter:      "all",
		ListOptions: gh.ListOptions{PerPage: 100},
	}

	var jobs []JobRecord
	for {
		result, resp, err := c.client.Actions.ListWorkflowJobs(
			ctx, repo.Owner, repo.Repo, runID, opts)
		if err != nil {
			return nil, err
		}

		for _, job := range result.Jobs {
			startedAt := job.GetStartedAt().Time
			completedAt := job.GetCompletedAt().Time
			durSec := 0
			if !startedAt.IsZero() && !completedAt.IsZero() {
				durSec = int(completedAt.Sub(startedAt).Seconds())
			}

			platform, variant, flavor, stream := ParseJobDimensions(job.GetName())

			jr := JobRecord{
				ID:          job.GetID(),
				Name:        job.GetName(),
				Conclusion:  job.GetConclusion(),
				StartedAt:   startedAt.Format(time.RFC3339),
				CompletedAt: completedAt.Format(time.RFC3339),
				DurationSec: durSec,
				RunnerName:  job.GetRunnerName(),
				Platform:    platform,
				Variant:     variant,
				Flavor:      flavor,
				Stream:      stream,
			}

			for _, step := range job.Steps {
				stepStart := step.GetStartedAt().Time
				stepEnd := step.GetCompletedAt().Time
				stepDur := 0
				if !stepStart.IsZero() && !stepEnd.IsZero() {
					stepDur = int(stepEnd.Sub(stepStart).Seconds())
				}
				jr.Steps = append(jr.Steps, StepRecord{
					Name:        step.GetName(),
					Conclusion:  step.GetConclusion(),
					DurationSec: stepDur,
				})
			}

			jobs = append(jobs, jr)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return jobs, nil
}

func (c *Collector) loadHistory() (*BuildsHistory, error) {
	data, err := os.ReadFile(c.cfg.HistoryPath)
	if err != nil {
		return &BuildsHistory{}, nil // file not found is OK
	}
	var h BuildsHistory
	if err := json.Unmarshal(data, &h); err != nil {
		return nil, fmt.Errorf("parsing history: %w", err)
	}
	return &h, nil
}

func (c *Collector) writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// buildRecentBuilds returns the most recent `limit` runs, sorted newest first.
func buildRecentBuilds(runs []WorkflowRunRecord, limit int) []RecentBuild {
	sorted := make([]WorkflowRunRecord, len(runs))
	copy(sorted, runs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt > sorted[j].CreatedAt
	})
	if limit > len(sorted) {
		limit = len(sorted)
	}
	result := make([]RecentBuild, 0, limit)
	for _, r := range sorted[:limit] {
		failedJobs := 0
		for _, j := range r.Jobs {
			if j.Conclusion == "failure" {
				failedJobs++
			}
		}
		result = append(result, RecentBuild{
			RunID:       r.ID,
			Repo:        r.Repo,
			Workflow:    r.WorkflowName,
			Branch:      r.Branch,
			Event:       r.Event,
			Conclusion:  r.Conclusion,
			DurationMin: float64(r.DurationSec) / 60.0,
			StartedAt:   r.StartedAt,
			JobCount:    len(r.Jobs),
			FailedJobs:  failedJobs,
		})
	}
	return result
}

// computeDailySnapshots aggregates runs into per-day snapshots.
func computeDailySnapshots(runs []WorkflowRunRecord) []DailySnapshot {
	byDate := make(map[string]*DailySnapshot)
	for _, r := range runs {
		if len(r.CreatedAt) < 10 {
			continue
		}
		date := r.CreatedAt[:10]
		snap, ok := byDate[date]
		if !ok {
			snap = &DailySnapshot{
				Date:          date,
				RepoBreakdown: make(map[string]RepoDayCount),
			}
			byDate[date] = snap
		}
		snap.TotalRuns++
		rb := snap.RepoBreakdown[r.Repo]
		rb.Runs++
		switch r.Conclusion {
		case "success":
			snap.SuccessCount++
			rb.Successes++
		case "failure":
			snap.FailureCount++
			rb.Failures++
		case "cancelled", "skipped", "action_required":
			snap.CancelledCount++
		}
		snap.RepoBreakdown[r.Repo] = rb
		snap.AvgDurationMin += float64(r.DurationSec) / 60.0
		snap.AvgQueueTimeSec += float64(r.QueueTimeSec)
	}

	snapshots := make([]DailySnapshot, 0, len(byDate))
	for _, s := range byDate {
		if s.TotalRuns > 0 {
			s.AvgDurationMin /= float64(s.TotalRuns)
			s.AvgQueueTimeSec /= float64(s.TotalRuns)
		}
		snapshots = append(snapshots, *s)
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Date < snapshots[j].Date
	})
	return snapshots
}

// computeAllRepoMetrics builds RepoMetrics for each configured repo.
func computeAllRepoMetrics(runs []WorkflowRunRecord, repos []RepoConfig) []RepoMetrics {
	result := make([]RepoMetrics, 0, len(repos))
	for _, repo := range repos {
		result = append(result, ComputeRepoMetrics(runs, repo.Label))
	}
	return result
}
