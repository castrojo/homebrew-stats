package builds

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

// SuccessRate returns the success percentage over the trailing `days` days.
// Excludes cancelled, skipped, and action_required runs from denominator.
// Returns 0.0 if no qualifying runs exist.
func SuccessRate(runs []WorkflowRunRecord, days int) float64 {
	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339)
	var total, successes int
	for _, r := range runs {
		if r.CreatedAt < cutoff {
			continue
		}
		switch r.Conclusion {
		case "cancelled", "skipped", "action_required":
			continue
		}
		total++
		if r.Conclusion == "success" {
			successes++
		}
	}
	if total == 0 {
		return 0.0
	}
	return float64(successes) / float64(total) * 100.0
}

// Percentile computes the Pth percentile of a float64 slice using nearest-rank.
// Returns 0.0 for empty slice.
func Percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0.0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}

	// Nearest-rank method
	rank := math.Ceil(p / 100.0 * float64(len(sorted)))
	idx := int(rank) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// MTTR returns mean time to recovery in minutes.
// For each failure on a branch, find the next success on that branch.
// Averages the gap durations. Returns 0.0 if no failure→success pairs.
func MTTR(runs []WorkflowRunRecord) float64 {
	// Group by branch, sorted by CreatedAt ascending
	byBranch := make(map[string][]WorkflowRunRecord)
	for _, r := range runs {
		byBranch[r.Branch] = append(byBranch[r.Branch], r)
	}
	for branch := range byBranch {
		sort.Slice(byBranch[branch], func(i, j int) bool {
			return byBranch[branch][i].CreatedAt < byBranch[branch][j].CreatedAt
		})
	}

	var totalMinutes float64
	var pairs int

	for _, branchRuns := range byBranch {
		for i, r := range branchRuns {
			if r.Conclusion != "failure" {
				continue
			}
			failTime, err := time.Parse(time.RFC3339, r.CreatedAt)
			if err != nil {
				continue
			}
			// Find next success on this branch
			for j := i + 1; j < len(branchRuns); j++ {
				if branchRuns[j].Conclusion == "success" {
					recoveryTime, err := time.Parse(time.RFC3339, branchRuns[j].CreatedAt)
					if err != nil {
						break
					}
					totalMinutes += recoveryTime.Sub(failTime).Minutes()
					pairs++
					break
				}
			}
		}
	}

	if pairs == 0 {
		return 0.0
	}
	return totalMinutes / float64(pairs)
}

// MTBF returns mean time between failures in hours.
// Returns 0.0 if fewer than 2 failures.
func MTBF(runs []WorkflowRunRecord) float64 {
	var failures []time.Time
	for _, r := range runs {
		if r.Conclusion == "failure" {
			t, err := time.Parse(time.RFC3339, r.CreatedAt)
			if err == nil {
				failures = append(failures, t)
			}
		}
	}
	if len(failures) < 2 {
		return 0.0
	}
	sort.Slice(failures, func(i, j int) bool {
		return failures[i].Before(failures[j])
	})

	var totalHours float64
	for i := 1; i < len(failures); i++ {
		totalHours += failures[i].Sub(failures[i-1]).Hours()
	}
	return totalHours / float64(len(failures)-1)
}

// FlakinessIndex computes the standard deviation of success rates in
// sliding windows of size windowSize over the run outcomes.
// Returns 0.0 for consistent results (all-pass or all-fail).
// Range is 0.0 (consistent) to 0.5 (maximally flaky, alternating).
func FlakinessIndex(outcomes []bool, windowSize int) float64 {
	if windowSize < 1 {
		windowSize = 1
	}
	if len(outcomes) < windowSize {
		return 0.0
	}

	var rates []float64
	for i := 0; i <= len(outcomes)-windowSize; i++ {
		window := outcomes[i : i+windowSize]
		successes := 0
		for _, ok := range window {
			if ok {
				successes++
			}
		}
		rates = append(rates, float64(successes)/float64(windowSize))
	}

	if len(rates) == 0 {
		return 0.0
	}

	// Compute mean
	var mean float64
	for _, r := range rates {
		mean += r
	}
	mean /= float64(len(rates))

	// Compute standard deviation
	var variance float64
	for _, r := range rates {
		d := r - mean
		variance += d * d
	}
	variance /= float64(len(rates))
	return math.Sqrt(variance)
}

// ClassifyFailure categorizes a failure by examining step names.
// Categories: "build", "push", "sign", "sbom", "test", "infra", "cancelled", "unknown"
func ClassifyFailure(job JobRecord) string {
	if job.Conclusion == "cancelled" {
		return "cancelled"
	}

	for _, step := range job.Steps {
		if step.Conclusion != "failure" {
			continue
		}
		name := strings.ToLower(step.Name)
		switch {
		case containsAny(name, "push", "upload", "publish"):
			return "push"
		case containsAny(name, "sign", "cosign", "sigstore"):
			return "sign"
		case containsAny(name, "sbom", "syft", "attestation"):
			return "sbom"
		case containsAny(name, "test", "check", "verify", "validate"):
			return "test"
		case containsAny(name, "build", "compile", "docker", "podman", "buildah"):
			return "build"
		case containsAny(name, "runner", "infra", "setup", "checkout", "init", "install dependency", "set up"):
			return "infra"
		}
	}

	// Fallback: classify by job name
	name := strings.ToLower(job.Name)
	switch {
	case containsAny(name, "push", "upload", "publish"):
		return "push"
	case containsAny(name, "sign", "cosign"):
		return "sign"
	case containsAny(name, "sbom"):
		return "sbom"
	case containsAny(name, "test"):
		return "test"
	case containsAny(name, "build", "compile"):
		return "build"
	}

	return "unknown"
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// reJobMatrix matches patterns like:
//
//	"build_container (main, bluefin)"
//	"build / Build and push image (arm64)"
//	"Build Stable ISOs / Build ISOs (amd64, nvidia-open, stable)"
var (
	reParens       = regexp.MustCompile(`\(([^)]+)\)`)
	reArch         = regexp.MustCompile(`\b(amd64|arm64|x86_64|aarch64)\b`)
	reStreamSuffix = regexp.MustCompile(`\b(stable|latest|beta|lts|testing)\b`)
)

// ParseJobDimensions extracts platform, variant, flavor, and stream from a job name.
func ParseJobDimensions(jobName string) (platform, variant, flavor, stream string) {
	// Extract the content inside the last set of parentheses.
	matches := reParens.FindAllStringSubmatch(jobName, -1)
	if len(matches) == 0 {
		return
	}
	inner := matches[len(matches)-1][1]
	parts := strings.Split(inner, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}

	// Identify each part
	for _, part := range parts {
		lower := strings.ToLower(part)
		if reArch.MatchString(lower) {
			platform = part
			continue
		}
		if reStreamSuffix.MatchString(lower) {
			stream = lower
			continue
		}
		// First non-arch, non-stream part → flavor; second → variant.
		if flavor == "" {
			flavor = lower
		} else if variant == "" {
			variant = lower
		}
	}

	return
}

// ComputeDORALevel classifies pipeline performance into elite/high/medium/low
// based on published DORA benchmarks.
func ComputeDORALevel(d DORAMetrics) string {
	// DORA 2023 benchmarks (simplified):
	// Elite:  deploy > 1/day, lead time < 1h, CFR < 5%, MTTR < 1h
	// High:   deploy >= 1/week, lead time < 1d, CFR < 10%, MTTR < 1d
	// Medium: deploy >= 1/month, lead time < 1wk, CFR < 15%, MTTR < 1wk
	// Low:    everything else

	freqPerDay := d.DeployFreqPerWeek / 7.0
	leadTimeHours := d.LeadTimeMinutes / 60.0
	mttrHours := d.MTTRMinutes / 60.0
	cfr := d.ChangeFailureRatePct

	eliteScore := 0
	if freqPerDay >= 1 {
		eliteScore++
	}
	if leadTimeHours < 1 {
		eliteScore++
	}
	if cfr < 5 {
		eliteScore++
	}
	if mttrHours < 1 {
		eliteScore++
	}
	if eliteScore >= 3 {
		return "elite"
	}

	highScore := 0
	if d.DeployFreqPerWeek >= 1 {
		highScore++
	}
	if leadTimeHours < 24 {
		highScore++
	}
	if cfr < 10 {
		highScore++
	}
	if mttrHours < 24 {
		highScore++
	}
	if highScore >= 3 {
		return "high"
	}

	medScore := 0
	if d.DeployFreqPerWeek >= 0.25 { // ~once per month
		medScore++
	}
	if leadTimeHours < 168 { // < 1 week
		medScore++
	}
	if cfr < 15 {
		medScore++
	}
	if mttrHours < 168 {
		medScore++
	}
	if medScore >= 3 {
		return "medium"
	}

	return "low"
}

// HealthStatus returns "healthy", "degraded", or "failing" based on thresholds.
// healthy: rate7d > 95 AND mttrMin < 60
// degraded: rate7d > 80 (or mttrMin < 240)
// failing: otherwise
func HealthStatus(rate7d float64, mttrMin float64) string {
	if rate7d > 95 && mttrMin < 60 {
		return "healthy"
	}
	if rate7d > 80 || (mttrMin > 0 && mttrMin < 240) {
		return "degraded"
	}
	return "failing"
}

// ComputeSummary builds PipelineSummary from all runs.
func ComputeSummary(runs []WorkflowRunRecord) PipelineSummary {
	rate7d := SuccessRate(runs, 7)
	rate30d := SuccessRate(runs, 30)

	cutoff7d := time.Now().UTC().AddDate(0, 0, -7).Format(time.RFC3339)
	cutoff30d := time.Now().UTC().AddDate(0, 0, -30).Format(time.RFC3339)

	var total7d, total30d int
	var durations []float64
	var totalQueueSec float64
	var queueCount int

	streams := make(map[string]bool)

	for _, r := range runs {
		switch r.Conclusion {
		case "cancelled", "skipped", "action_required":
			// skip from totals
		default:
			if r.CreatedAt >= cutoff7d {
				total7d++
			}
			if r.CreatedAt >= cutoff30d {
				total30d++
			}
		}

		if r.DurationSec > 0 {
			durations = append(durations, float64(r.DurationSec)/60.0)
		}
		if r.QueueTimeSec > 0 {
			totalQueueSec += float64(r.QueueTimeSec)
			queueCount++
		}

		// Count distinct workflow files as "streams"
		streams[r.WorkflowFile] = true
	}

	var avgDur, p50, p95, p99, avgQueue float64
	if len(durations) > 0 {
		var sum float64
		for _, d := range durations {
			sum += d
		}
		avgDur = sum / float64(len(durations))
		p50 = Percentile(durations, 50)
		p95 = Percentile(durations, 95)
		p99 = Percentile(durations, 99)
	}
	if queueCount > 0 {
		avgQueue = totalQueueSec / float64(queueCount)
	}

	mttr := MTTR(runs)

	return PipelineSummary{
		OverallSuccessRate7d:  rate7d,
		OverallSuccessRate30d: rate30d,
		TotalBuilds7d:         total7d,
		TotalBuilds30d:        total30d,
		AvgDurationMin:        avgDur,
		P50DurationMin:        p50,
		P95DurationMin:        p95,
		P99DurationMin:        p99,
		AvgQueueTimeSec:       avgQueue,
		ActiveStreams:         len(streams),
		HealthStatus:          HealthStatus(rate7d, mttr),
	}
}

// ComputeRepoMetrics builds RepoMetrics for a single repo label.
func ComputeRepoMetrics(runs []WorkflowRunRecord, repo string) RepoMetrics {
	var repoRuns []WorkflowRunRecord
	for _, r := range runs {
		if r.Repo == repo {
			repoRuns = append(repoRuns, r)
		}
	}

	rate7d := SuccessRate(repoRuns, 7)
	rate30d := SuccessRate(repoRuns, 30)

	cutoff7d := time.Now().UTC().AddDate(0, 0, -7).Format(time.RFC3339)
	cutoff30d := time.Now().UTC().AddDate(0, 0, -30).Format(time.RFC3339)

	var total7d, total30d int
	var totalDurSec int
	var durCount int

	for _, r := range repoRuns {
		switch r.Conclusion {
		case "cancelled", "skipped", "action_required":
		default:
			if r.CreatedAt >= cutoff7d {
				total7d++
			}
			if r.CreatedAt >= cutoff30d {
				total30d++
			}
		}
		if r.DurationSec > 0 {
			totalDurSec += r.DurationSec
			durCount++
		}
	}

	var avgDur float64
	if durCount > 0 {
		avgDur = float64(totalDurSec) / float64(durCount) / 60.0
	}

	// Compute per-stream metrics (keyed by workflow file)
	streamMap := make(map[string][]WorkflowRunRecord)
	for _, r := range repoRuns {
		streamMap[r.WorkflowFile] = append(streamMap[r.WorkflowFile], r)
	}
	var streams []StreamMetrics
	for wfFile, wfRuns := range streamMap {
		sm := computeStreamMetrics(wfFile, wfRuns)
		streams = append(streams, sm)
	}
	sort.Slice(streams, func(i, j int) bool {
		return streams[i].Name < streams[j].Name
	})

	// Compute per-architecture metrics from job records
	archMap := make(map[string][]JobRecord)
	for _, r := range repoRuns {
		if r.CreatedAt < cutoff7d {
			continue
		}
		for _, j := range r.Jobs {
			if j.Platform != "" {
				archMap[j.Platform] = append(archMap[j.Platform], j)
			}
		}
	}
	var archs []ArchMetrics
	for platform, jobs := range archMap {
		am := computeArchMetrics(platform, jobs)
		archs = append(archs, am)
	}
	sort.Slice(archs, func(i, j int) bool {
		return archs[i].Platform < archs[j].Platform
	})

	return RepoMetrics{
		Repo:           repo,
		SuccessRate7d:  rate7d,
		SuccessRate30d: rate30d,
		TotalRuns7d:    total7d,
		TotalRuns30d:   total30d,
		AvgDurationMin: avgDur,
		Streams:        streams,
		Architectures:  archs,
	}
}

func computeStreamMetrics(wfFile string, runs []WorkflowRunRecord) StreamMetrics {
	// Use the workflow file base name as the stream name
	name := wfFile
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	name = strings.TrimSuffix(name, ".yml")

	rate7d := SuccessRate(runs, 7)
	rate30d := SuccessRate(runs, 30)

	cutoff7d := time.Now().UTC().AddDate(0, 0, -7).Format(time.RFC3339)
	var total7d int
	var totalDurSec, durCount int
	var lastRunAt, lastConclusion string

	for _, r := range runs {
		switch r.Conclusion {
		case "cancelled", "skipped", "action_required":
		default:
			if r.CreatedAt >= cutoff7d {
				total7d++
			}
		}
		if r.DurationSec > 0 {
			totalDurSec += r.DurationSec
			durCount++
		}
		if r.CreatedAt > lastRunAt {
			lastRunAt = r.CreatedAt
			lastConclusion = r.Conclusion
		}
	}

	var avgDur float64
	if durCount > 0 {
		avgDur = float64(totalDurSec) / float64(durCount) / 60.0
	}

	return StreamMetrics{
		Name:           name,
		SuccessRate7d:  rate7d,
		SuccessRate30d: rate30d,
		TotalRuns7d:    total7d,
		AvgDurationMin: avgDur,
		LastRunAt:      lastRunAt,
		LastConclusion: lastConclusion,
	}
}

func computeArchMetrics(platform string, jobs []JobRecord) ArchMetrics {
	var successes, totalDurSec, durCount int
	for _, j := range jobs {
		if j.Conclusion == "success" {
			successes++
		}
		if j.DurationSec > 0 {
			totalDurSec += j.DurationSec
			durCount++
		}
	}
	var rate float64
	if len(jobs) > 0 {
		rate = float64(successes) / float64(len(jobs)) * 100.0
	}
	var avgDur float64
	if durCount > 0 {
		avgDur = float64(totalDurSec) / float64(durCount) / 60.0
	}
	return ArchMetrics{
		Platform:       platform,
		SuccessRate7d:  rate,
		AvgDurationMin: avgDur,
		TotalJobs7d:    len(jobs),
	}
}

// ComputeTopFlaky returns the top `limit` flakiest jobs ranked by FlakinessIndex.
func ComputeTopFlaky(runs []WorkflowRunRecord, limit int) []FlakyJob {
	type jobKey struct {
		repo string
		name string
	}
	type jobAgg struct {
		outcomes    []bool
		lastFailure string
		totalRuns   int
		failures    int
		// track step failure frequencies
		stepFailCounts map[string]int
	}

	agg := make(map[jobKey]*jobAgg)

	for _, r := range runs {
		for _, j := range r.Jobs {
			key := jobKey{repo: r.Repo, name: j.Name}
			a, ok := agg[key]
			if !ok {
				a = &jobAgg{stepFailCounts: make(map[string]int)}
				agg[key] = a
			}
			a.totalRuns++
			outcome := j.Conclusion == "success"
			a.outcomes = append(a.outcomes, outcome)
			if !outcome {
				a.failures++
				if j.StartedAt > a.lastFailure {
					a.lastFailure = j.StartedAt
				}
				for _, s := range j.Steps {
					if s.Conclusion == "failure" {
						a.stepFailCounts[s.Name]++
					}
				}
			}
		}
	}

	var result []FlakyJob
	for key, a := range agg {
		if a.totalRuns < 2 {
			continue
		}
		fi := FlakinessIndex(a.outcomes, 5)
		var failRate float64
		if a.totalRuns > 0 {
			failRate = float64(a.failures) / float64(a.totalRuns) * 100.0
		}

		// Find top failing step
		topStep := ""
		topCount := 0
		for step, cnt := range a.stepFailCounts {
			if cnt > topCount {
				topCount = cnt
				topStep = step
			}
		}

		result = append(result, FlakyJob{
			Repo:           key.repo,
			JobName:        key.name,
			TotalRuns:      a.totalRuns,
			Failures:       a.failures,
			FailureRate:    failRate,
			FlakinessIndex: fi,
			LastFailure:    a.lastFailure,
			TopFailStep:    topStep,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].FlakinessIndex != result[j].FlakinessIndex {
			return result[i].FlakinessIndex > result[j].FlakinessIndex
		}
		return result[i].FailureRate > result[j].FailureRate
	})

	if limit > len(result) {
		limit = len(result)
	}
	return result[:limit]
}

// ComputeDurationTrend returns daily P50/P95/P99 buckets over `days` days.
func ComputeDurationTrend(runs []WorkflowRunRecord, days int) []DurationBucket {
	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339)
	byDate := make(map[string][]float64)

	for _, r := range runs {
		if r.CreatedAt < cutoff || r.DurationSec <= 0 {
			continue
		}
		if len(r.CreatedAt) < 10 {
			continue
		}
		date := r.CreatedAt[:10]
		byDate[date] = append(byDate[date], float64(r.DurationSec)/60.0)
	}

	buckets := make([]DurationBucket, 0, len(byDate))
	for date, durs := range byDate {
		var sum, minD, maxD float64
		minD = durs[0]
		maxD = durs[0]
		for _, d := range durs {
			sum += d
			if d < minD {
				minD = d
			}
			if d > maxD {
				maxD = d
			}
		}
		buckets = append(buckets, DurationBucket{
			Date: date,
			P50:  Percentile(durs, 50),
			P95:  Percentile(durs, 95),
			P99:  Percentile(durs, 99),
			Avg:  sum / float64(len(durs)),
			Min:  minD,
			Max:  maxD,
		})
	}

	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].Date < buckets[j].Date
	})
	return buckets
}

// ComputeFailureBreakdown categorizes all failures in the dataset.
func ComputeFailureBreakdown(runs []WorkflowRunRecord) []FailureCategory {
	counts := make(map[string]int)
	var total int

	for _, r := range runs {
		if r.Conclusion != "failure" && r.Conclusion != "cancelled" {
			continue
		}
		total++
		if len(r.Jobs) == 0 {
			// No job data — classify from run-level conclusion
			if r.Conclusion == "cancelled" {
				counts["cancelled"]++
			} else {
				counts["unknown"]++
			}
			continue
		}
		// Find first failed/cancelled job
		categorized := false
		for _, j := range r.Jobs {
			if j.Conclusion == "failure" || j.Conclusion == "cancelled" {
				cat := ClassifyFailure(j)
				counts[cat]++
				categorized = true
				break
			}
		}
		if !categorized {
			counts["unknown"]++
		}
	}

	result := make([]FailureCategory, 0, len(counts))
	for cat, cnt := range counts {
		var pct float64
		if total > 0 {
			pct = float64(cnt) / float64(total) * 100.0
		}
		result = append(result, FailureCategory{
			Category: cat,
			Count:    cnt,
			Pct:      pct,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})
	return result
}

// ComputeTriggerBreakdown counts runs by trigger event.
func ComputeTriggerBreakdown(runs []WorkflowRunRecord) TriggerBreakdown {
	var td TriggerBreakdown
	for _, r := range runs {
		switch r.Event {
		case "schedule":
			td.Scheduled++
		case "push":
			td.Push++
		case "pull_request", "pull_request_target":
			td.PR++
		case "workflow_dispatch":
			td.Manual++
		default:
			td.Other++
		}
	}
	return td
}

// computeMonthlySnapshots aggregates runs into per-month snapshots.
// The current (incomplete) month is excluded so only full months appear.
func computeMonthlySnapshots(runs []WorkflowRunRecord) []MonthlySnapshot {
	if len(runs) == 0 {
		return []MonthlySnapshot{}
	}

	currentMonth := time.Now().UTC().Format("2006-01")

	type monthData struct {
		success, failure, cancelled int
		durations                   []float64
		repoSuccess                 map[string]int
		repoTotal                   map[string]int
	}
	months := map[string]*monthData{}
	monthOrder := []string{}

	for _, r := range runs {
		t, err := time.Parse(time.RFC3339, r.CreatedAt)
		if err != nil {
			continue
		}
		month := t.Format("2006-01")
		if month == currentMonth {
			continue
		}
		if _, ok := months[month]; !ok {
			months[month] = &monthData{
				repoSuccess: map[string]int{},
				repoTotal:   map[string]int{},
			}
			monthOrder = append(monthOrder, month)
		}
		d := months[month]
		switch r.Conclusion {
		case "success":
			d.success++
		case "failure":
			d.failure++
		case "cancelled":
			d.cancelled++
		}
		durationMin := float64(r.DurationSec) / 60.0
		d.durations = append(d.durations, durationMin)
		d.repoTotal[r.Repo]++
		if r.Conclusion == "success" {
			d.repoSuccess[r.Repo]++
		}
	}

	sort.Strings(monthOrder)

	result := make([]MonthlySnapshot, 0, len(monthOrder))
	for _, month := range monthOrder {
		d := months[month]
		total := d.success + d.failure + d.cancelled
		// SuccessRate excludes cancelled from denominator (matches SuccessRate() semantics)
		denominator := d.success + d.failure
		var successRate float64
		if denominator > 0 {
			successRate = 100.0 * float64(d.success) / float64(denominator)
		}

		var avgDur float64
		if len(d.durations) > 0 {
			sum := 0.0
			for _, v := range d.durations {
				sum += v
			}
			avgDur = sum / float64(len(d.durations))
		}

		var p95Dur float64
		if len(d.durations) > 0 {
			p95Dur = Percentile(d.durations, 95)
		}

		repoRate := map[string]float64{}
		for repo, tot := range d.repoTotal {
			if tot > 0 {
				repoRate[repo] = 100.0 * float64(d.repoSuccess[repo]) / float64(tot)
			}
		}

		doraLevel := "low"
		if successRate >= 99 {
			doraLevel = "elite"
		} else if successRate >= 95 {
			doraLevel = "high"
		} else if successRate >= 85 {
			doraLevel = "medium"
		}

		result = append(result, MonthlySnapshot{
			Month:           month,
			TotalRuns:       total,
			SuccessCount:    d.success,
			FailureCount:    d.failure,
			CancelledCount:  d.cancelled,
			SuccessRate:     successRate,
			AvgDurationMin:  avgDur,
			P95DurationMin:  p95Dur,
			RepoSuccessRate: repoRate,
			DORALevel:       doraLevel,
		})
	}
	return result
}

// computeDORA builds DORAMetrics from all runs.
func computeDORA(runs []WorkflowRunRecord) DORAMetrics {
	if len(runs) == 0 {
		return DORAMetrics{DORALevel: "low"}
	}

	// Deployment frequency: count successful push/schedule runs in last 30 days
	cutoff30d := time.Now().UTC().AddDate(0, 0, -30).Format(time.RFC3339)
	var deployCount int
	var leadTimes []float64

	for _, r := range runs {
		if r.CreatedAt < cutoff30d {
			continue
		}
		if r.Conclusion == "success" &&
			(r.Event == "push" || r.Event == "schedule" || r.Event == "workflow_dispatch") {
			deployCount++
		}
		// Lead time: time from CreatedAt to CompletedAt for successful runs
		if r.Conclusion == "success" && r.CreatedAt != "" && r.CompletedAt != "" {
			created, err1 := time.Parse(time.RFC3339, r.CreatedAt)
			completed, err2 := time.Parse(time.RFC3339, r.CompletedAt)
			if err1 == nil && err2 == nil {
				leadTimes = append(leadTimes, completed.Sub(created).Minutes())
			}
		}
	}

	freqPerWeek := float64(deployCount) / 4.33 // 30 days ≈ 4.33 weeks
	var deployFreqLabel string
	switch {
	case freqPerWeek >= 7:
		deployFreqLabel = "multiple per day"
	case freqPerWeek >= 1:
		deployFreqLabel = "multiple per week"
	case freqPerWeek >= 0.25:
		deployFreqLabel = "multiple per month"
	default:
		deployFreqLabel = "less than monthly"
	}

	var avgLeadTime float64
	if len(leadTimes) > 0 {
		var sum float64
		for _, lt := range leadTimes {
			sum += lt
		}
		avgLeadTime = sum / float64(len(leadTimes))
	}

	// Change failure rate: % of runs that resulted in failure
	cutoff := time.Now().UTC().AddDate(0, 0, -30).Format(time.RFC3339)
	var totalDeploys, failedDeploys int
	for _, r := range runs {
		if r.CreatedAt < cutoff {
			continue
		}
		if r.Event == "push" || r.Event == "schedule" || r.Event == "workflow_dispatch" {
			totalDeploys++
			if r.Conclusion == "failure" {
				failedDeploys++
			}
		}
	}
	var cfr float64
	if totalDeploys > 0 {
		cfr = float64(failedDeploys) / float64(totalDeploys) * 100.0
	}

	mttr := MTTR(runs)
	mtbf := MTBF(runs)

	d := DORAMetrics{
		DeploymentFrequency:  deployFreqLabel,
		DeployFreqPerWeek:    freqPerWeek,
		LeadTimeMinutes:      avgLeadTime,
		ChangeFailureRatePct: cfr,
		MTTRMinutes:          mttr,
		MTBFHours:            mtbf,
	}
	d.DORALevel = ComputeDORALevel(d)
	return d
}
