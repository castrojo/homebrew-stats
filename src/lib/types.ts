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
  prs_merged_30d: number;
  issues_opened_30d: number;
  discussion_posts_30d: number;
  repos_active: string[];
  is_bot: boolean;
}

export interface ContributorSummary {
  active_contributors: number;
  new_contributors: number;
  total_commits: number;
  total_prs_merged: number;
  total_issues_opened: number;
  total_issues_closed: number;
  bus_factor: number;
  review_participation_rate: number;
  active_repos: number;
  total_discussions: number;
  discussion_answer_rate: number;
}

export interface RepoStats {
  name: string;
  commits_30d: number;
  unique_human_authors_30d: number;
  prs_merged_30d: number;
  issues_opened_30d: number;
  bus_factor: number;
  bot_commits_30d: number;
  human_commits_30d: number;
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
  total_discussion_comments_30d: number;
  unique_discussion_authors_30d: number;
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
