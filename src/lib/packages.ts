export interface Package {
  name: string;
  type: string;
  version?: string;
  latest_version?: string;
  is_stale: boolean;
  freshness_known: boolean;
  downloads: number;
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
