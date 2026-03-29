/**
 * Shared data types for the homebrew-stats data pipeline.
 *
 * These types mirror the JSON structures written by stats-go.
 * DO NOT edit these without also updating the corresponding Go types.
 */

// ── Homebrew ──────────────────────────────────────────────────────────────────

export interface TapSnapshot {
  uniques: number;
  count: number;
  downloads?: Record<string, number>;
}

export interface DaySnapshot {
  date: string;
  taps: Record<string, TapSnapshot>;
}

// ── Testhub ──────────────────────────────────────────────────────────────────

export interface AppDayCount {
  app: string;
  passed: number;
  failed: number;
  total: number;
}

export interface Package {
  name: string;
  version?: string;
  html_url?: string;
  version_count: number;
  pull_count?: number;
  created_at?: string;
  updated_at?: string;
}

export interface TesthubDaySnapshot {
  date: string;
  packages: Package[];
  build_counts: AppDayCount[];
  last_run_id: number;
}

// ── Countme ──────────────────────────────────────────────────────────────────

export interface WeekRecord {
  week_start: string;
  week_end: string;
  distros: Record<string, number>;
  total: number;
}

// ── Contributors ─────────────────────────────────────────────────────────────

export interface ContributorEntry {
  login: string;
  name?: string;
  avatar_url?: string;
  company?: string;
  location?: string;
  commits_30d: number;
  commits_90d?: number;
  commits_60d?: number;
  commits_365d?: number;
  prs_merged_30d: number;
  prs_merged_90d?: number;
  prs_merged_60d?: number;
  prs_merged_365d?: number;
  issues_opened_30d: number;
  issues_opened_90d?: number;
  issues_opened_60d?: number;
  issues_opened_365d?: number;
  discussion_posts_30d: number;
  discussion_posts_90d?: number;
  discussion_posts_60d?: number;
  discussion_posts_365d?: number;
  repos_active: string[];
  is_bot: boolean;
}

export interface ContributorSummary {
  active_contributors: number;
  active_contributors_90d?: number;
  active_contributors_60d?: number;
  active_contributors_365d?: number;
  new_contributors: number;
  total_commits: number;
  total_commits_90d?: number;
  total_commits_60d?: number;
  total_commits_365d?: number;
  total_prs_merged: number;
  total_prs_merged_90d?: number;
  total_prs_merged_60d?: number;
  total_prs_merged_365d?: number;
  total_issues_opened: number;
  total_issues_closed: number;
  bus_factor: number;
  bus_factor_90d?: number;
  bus_factor_60d?: number;
  bus_factor_365d?: number;
  review_participation_rate: number;
  active_repos: number;
  total_discussions: number;
  discussion_answer_rate: number;
}

export interface RepoStats {
  name: string;
  commits_30d: number;
  commits_90d?: number;
  commits_60d?: number;
  commits_365d?: number;
  unique_human_authors_30d: number;
  prs_merged_30d: number;
  prs_merged_90d?: number;
  prs_merged_60d?: number;
  prs_merged_365d?: number;
  issues_opened_30d: number;
  issues_opened_90d?: number;
  issues_opened_60d?: number;
  issues_opened_365d?: number;
  bus_factor: number;
  bus_factor_90d?: number;
  bus_factor_60d?: number;
  bus_factor_365d?: number;
  bot_commits_30d: number;
  human_commits_30d: number;
  human_commits_90d?: number;
  human_commits_60d?: number;
  human_commits_365d?: number;
  active_weeks_streak: number;
  weekly_commits_52w: number[];
  commits_by_day_of_week: Record<string, number>;
  contribution_heatmap: number[][];
  issue_label_distribution: Record<string, number>;
}

export interface DiscussionWeek {
  week: string;
  discussions: number;
  comments: number;
}

export interface DiscussionSummary {
  total_discussions_30d: number;
  total_discussions_90d?: number;
  total_discussions_60d?: number;
  total_discussions_365d?: number;
  total_discussion_comments_30d: number;
  total_discussion_comments_90d?: number;
  total_discussion_comments_60d?: number;
  total_discussion_comments_365d?: number;
  unique_discussion_authors_30d: number;
  unique_discussion_authors_90d?: number;
  unique_discussion_authors_60d?: number;
  unique_discussion_authors_365d?: number;
  answered_rate: number;
  weekly_trend: DiscussionWeek[];
}

export interface ContributorsData {
  generated_at: string;
  period: { start: string; end: string };
  summary: ContributorSummary;
  top_contributors: ContributorEntry[];
  repos: RepoStats[];
  discussions_summary: DiscussionSummary;
}

// ── Builds tab types ──

export interface BuildsData {
  generated_at: string;
  repos: BuildRepoMetrics[];
  summary: PipelineSummary;
  dora_metrics: DORAMetrics;
  top_flaky: FlakyJob[];
  recent_builds: RecentBuild[];
  duration_trend: DurationBucket[];
  failure_breakdown: FailureCategory[];
  trigger_breakdown: TriggerBreakdown;
  history: BuildDailySnapshot[];
  monthly_history?: BuildMonthlySnapshot[];
}

export interface PipelineSummary {
  overall_success_rate_7d: number;
  overall_success_rate_30d: number;
  total_builds_7d: number;
  total_builds_30d: number;
  avg_duration_min: number;
  p50_duration_min: number;
  p95_duration_min: number;
  p99_duration_min: number;
  avg_queue_time_sec: number;
  active_streams: number;
  health_status: string;
}

export interface DORAMetrics {
  deployment_frequency: string;
  deploy_freq_per_week: number;
  lead_time_minutes: number;
  change_failure_rate_pct: number;
  mttr_minutes: number;
  mtbf_hours: number;
  dora_level: string;
}

export interface FlakyJob {
  repo: string;
  job_name: string;
  total_runs: number;
  failures: number;
  failure_rate: number;
  flakiness_index: number;
  last_failure: string;
  top_fail_step: string;
}

export interface RecentBuild {
  run_id: number;
  repo: string;
  workflow: string;
  branch: string;
  event: string;
  conclusion: string;
  duration_min: number;
  started_at: string;
  html_url: string;
  job_count: number;
  failed_jobs: number;
}

export interface DurationBucket {
  date: string;
  p50: number;
  p95: number;
  p99: number;
  avg: number;
  min: number;
  max: number;
}

export interface FailureCategory {
  category: string;
  count: number;
  pct: number;
}

export interface TriggerBreakdown {
  scheduled: number;
  push: number;
  pull_request: number;
  workflow_dispatch: number;
  other: number;
}

export interface BuildDailySnapshot {
  date: string;
  total_runs: number;
  success_count: number;
  failure_count: number;
  cancelled_count: number;
  avg_duration_min: number;
  p95_duration_min: number;
  avg_queue_time_sec: number;
  repo_breakdown: Record<string, { runs: number; successes: number; failures: number }>;
}

export interface BuildRepoMetrics {
  repo: string;
  success_rate_7d: number;
  success_rate_30d: number;
  total_runs_7d: number;
  total_runs_30d: number;
  avg_duration_min: number;
  streams: BuildStreamMetrics[];
  architectures?: BuildArchMetrics[];
  sign_step_success_rate_30d?: number;  // -1 = no sign steps detected
  sbom_step_success_rate_30d?: number;  // -1 = no sbom steps detected
}

export interface BuildStreamMetrics {
  name: string;
  success_rate_7d: number;
  success_rate_30d: number;
  total_runs_7d: number;
  avg_duration_min: number;
  last_run_at: string;
  last_conclusion: string;
}

export interface BuildArchMetrics {
  platform: string;
  success_rate_7d: number;
  avg_duration_min: number;
  total_jobs_7d: number;
}

export interface BuildMonthlySnapshot {
  month: string;
  total_runs: number;
  success_count: number;
  failure_count: number;
  cancelled_count: number;
  success_rate: number;
  avg_duration_min: number;
  p95_duration_min: number;
  repo_success_rate: Record<string, number>;
  dora_level: string;
}
