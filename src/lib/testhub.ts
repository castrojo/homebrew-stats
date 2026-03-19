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

export interface DaySnapshot {
  date: string;
  packages: Package[];
  build_counts: AppDayCount[];
  last_run_id: number;
}

/**
 * Compute overall pass rate across all app counts.
 * windowDays is accepted for API consistency but callers are responsible
 * for pre-filtering snapshots to the desired window.
 */
export function computeBuildPassRate(counts: AppDayCount[], _windowDays: number): number {
  const totalPassed = counts.reduce((sum, c) => sum + c.passed, 0);
  const totalRuns = counts.reduce((sum, c) => sum + c.total, 0);
  if (totalRuns === 0) return 0;
  return (totalPassed / totalRuns) * 100;
}

/**
 * Detect version changes for a specific app across snapshots.
 * Returns only entries where the version differs from the previous snapshot.
 * The first snapshot is treated as baseline (no change emitted).
 */
export function detectVersionChanges(
  snapshots: DaySnapshot[],
  app: string,
): { date: string; version: string }[] {
  const changes: { date: string; version: string }[] = [];
  let prevVersion: string | undefined;

  for (const snap of snapshots) {
    const pkg = snap.packages.find(p => p.name === app);
    if (!pkg) continue;

    const ver = pkg.version ?? "";
    if (prevVersion !== undefined && ver !== prevVersion) {
      changes.push({ date: snap.date, version: ver });
    }
    prevVersion = ver;
  }

  return changes;
}
