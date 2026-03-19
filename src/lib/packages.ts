export interface Package {
  name: string;
  type: string;
  version?: string;
  latest_version?: string;
  is_stale: boolean;
  freshness_known: boolean;
  downloads: number;
  /** Average daily install momentum over the last 7-day delta (0 = insufficient history). */
  velocity7d?: number;
  description?: string;
  homepage?: string;
}

/**
 * Sort packages: casks before formulas, then descending by download count.
 * Returns a new array — does not mutate the input.
 */
export function sortPackages(packages: Package[]): Package[] {
  return [...packages].sort((a, b) => {
    if (a.type !== b.type) return a.type < b.type ? -1 : 1; // cask before formula
    return (b.downloads ?? 0) - (a.downloads ?? 0);
  });
}

/** Filter packages by type (e.g. "cask" or "formula"). */
export function filterByType(packages: Package[], type: string): Package[] {
  return packages.filter(p => p.type === type);
}

/** Count packages that are stale. */
export function countStale(packages: Package[]): number {
  return packages.filter(p => p.is_stale).length;
}

/** Count packages that are known-fresh (freshness_known && !is_stale). */
export function countUpToDate(packages: Package[]): number {
  return packages.filter(p => p.freshness_known && !p.is_stale).length;
}
