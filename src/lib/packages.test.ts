import { describe, it, expect } from "vitest";
import { sortPackages, filterByType, countStale, countUpToDate, type Package } from "./packages";

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

describe("filterByType", () => {
  const pkgs: Package[] = [
    { name: "c1", type: "cask", is_stale: false, freshness_known: true, downloads: 1 },
    { name: "f1", type: "formula", is_stale: false, freshness_known: true, downloads: 2 },
    { name: "c2", type: "cask", is_stale: true, freshness_known: true, downloads: 3 },
  ];

  it("returns only casks", () => {
    expect(filterByType(pkgs, "cask").map((p: Package) => p.name)).toEqual(["c1", "c2"]);
  });

  it("returns only formulas", () => {
    expect(filterByType(pkgs, "formula").map((p: Package) => p.name)).toEqual(["f1"]);
  });

  it("returns empty array when no match", () => {
    expect(filterByType(pkgs, "other")).toHaveLength(0);
  });
});

describe("countStale", () => {
  it("counts packages where is_stale is true", () => {
    const pkgs: Package[] = [
      { name: "a", type: "cask", is_stale: true, freshness_known: true, downloads: 0 },
      { name: "b", type: "cask", is_stale: false, freshness_known: true, downloads: 0 },
      { name: "c", type: "cask", is_stale: true, freshness_known: true, downloads: 0 },
    ];
    expect(countStale(pkgs)).toBe(2);
  });

  it("returns 0 when nothing is stale", () => {
    const pkgs: Package[] = [
      { name: "a", type: "cask", is_stale: false, freshness_known: true, downloads: 0 },
    ];
    expect(countStale(pkgs)).toBe(0);
  });
});

describe("countUpToDate", () => {
  it("counts packages where freshness_known and not stale", () => {
    const pkgs: Package[] = [
      { name: "a", type: "cask", is_stale: false, freshness_known: true, downloads: 0 },   // up to date
      { name: "b", type: "cask", is_stale: true, freshness_known: true, downloads: 0 },    // stale
      { name: "c", type: "cask", is_stale: false, freshness_known: false, downloads: 0 },  // unknown
    ];
    expect(countUpToDate(pkgs)).toBe(1);
  });
});
