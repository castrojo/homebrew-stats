import { describe, it, expect } from "vitest";
import { sortPackages } from "./packages";

interface Package {
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

describe("sortPackages", () => {
  it("sorts casks before formulas", () => {
    const pkgs: Package[] = [
      { name: "mypkg", type: "formula", is_stale: false, freshness_known: true, downloads: 100 },
      { name: "mycask", type: "cask", is_stale: false, freshness_known: true, downloads: 50 },
    ];
    const result = sortPackages(pkgs);
    expect(result[0].type).toBe("cask");
    expect(result[1].type).toBe("formula");
  });

  it("sorts by downloads descending within same type", () => {
    const pkgs: Package[] = [
      { name: "low", type: "cask", is_stale: false, freshness_known: true, downloads: 10 },
      { name: "high", type: "cask", is_stale: false, freshness_known: true, downloads: 999 },
      { name: "mid", type: "cask", is_stale: false, freshness_known: true, downloads: 500 },
    ];
    const result = sortPackages(pkgs);
    expect(result.map((p: Package) => p.name)).toEqual(["high", "mid", "low"]);
  });

  it("does not mutate the original array", () => {
    const pkgs: Package[] = [
      { name: "a", type: "formula", is_stale: false, freshness_known: true, downloads: 1 },
    ];
    const original = [...pkgs];
    sortPackages(pkgs);
    expect(pkgs).toEqual(original);
  });

  it("treats missing downloads as zero", () => {
    const pkgs: Package[] = [
      { name: "withDownloads", type: "formula", is_stale: false, freshness_known: true, downloads: 50 },
      { name: "noDownloads", type: "formula", is_stale: false, freshness_known: true, downloads: 0 },
    ];
    const result = sortPackages(pkgs);
    expect(result[0].name).toBe("withDownloads");
  });
});
