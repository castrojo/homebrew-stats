/**
 * Pure chart-data computation functions, extracted from DownloadsChart.astro
 * so they can be unit-tested independently of the browser/Chart.js runtime.
 */

export interface TapSnapshot {
  uniques: number;
  count: number;
  downloads?: Record<string, number>;
}

export interface DaySnapshot {
  date: string;
  taps: Record<string, TapSnapshot>;
}

/**
 * Computes a true cumulative install count from daily snapshots of 30-day
 * rolling window totals (as returned by the Homebrew analytics API).
 *
 * Strategy:
 *   - Day 0: use the 30d total as the starting baseline.
 *   - Day N (N > 0): add max(0, today_total - yesterday_total).
 *     Clamping at 0 prevents old installs aging out of the 30d window from
 *     ever reducing the cumulative count.
 */
export function computeTotalData(snapshots: DaySnapshot[], tapName: string): number[] {
  let running = 0;
  return snapshots.map((d, i) => {
    const curr = d.taps[tapName]?.downloads ?? {};
    const currTotal = Object.values(curr).reduce((s, n) => s + n, 0);
    if (i === 0) {
      running = currTotal;
    } else {
      const prev = snapshots[i - 1].taps[tapName]?.downloads ?? {};
      const prevTotal = Object.values(prev).reduce((s, n) => s + n, 0);
      running += Math.max(0, currTotal - prevTotal);
    }
    return running;
  });
}

/**
 * Computes a true cumulative install count for a single package from daily
 * snapshots of 30-day rolling window totals.
 *
 * Same delta strategy as computeTotalData() but scoped to one package:
 *   - Day 0: use the 30d total as the starting baseline.
 *   - Day N (N > 0): add max(0, today - yesterday).
 */
export function computePackageCumulative(
  snapshots: DaySnapshot[],
  tapName: string,
  pkgName: string,
): number {
  if (snapshots.length === 0) return 0;
  let running = 0;
  for (let i = 0; i < snapshots.length; i++) {
    const curr = snapshots[i].taps[tapName]?.downloads?.[pkgName] ?? 0;
    if (i === 0) {
      running = curr;
    } else {
      const prev = snapshots[i - 1].taps[tapName]?.downloads?.[pkgName] ?? 0;
      running += Math.max(0, curr - prev);
    }
  }
  return running;
}

/**
 * Returns the top N package names ranked by their download count in the
 * most recent snapshot, and the remaining package names.
 */
export function computeTopPkgs(
  snapshots: DaySnapshot[],
  tapName: string,
  topN: number,
): { top: string[]; rest: string[] } {
  if (snapshots.length === 0) return { top: [], rest: [] };
  const latest = snapshots[snapshots.length - 1].taps[tapName]?.downloads ?? {};
  const ranked = Object.entries(latest)
    .sort((a, b) => b[1] - a[1])
    .map(([name]) => name);
  return { top: ranked.slice(0, topN), rest: ranked.slice(topN) };
}
