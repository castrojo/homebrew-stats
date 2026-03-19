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
