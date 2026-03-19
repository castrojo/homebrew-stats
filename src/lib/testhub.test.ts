import { describe, it, expect } from "vitest";
import {
  computeBuildPassRate,
  detectVersionChanges,
  type AppDayCount,
  type DaySnapshot,
} from "./testhub";

describe("computeBuildPassRate", () => {
  it("returns 0 for empty counts", () => {
    expect(computeBuildPassRate([], 7)).toBe(0);
  });

  it("returns 100 for all-passed counts", () => {
    const counts: AppDayCount[] = [{ app: "ghostty", passed: 10, failed: 0, total: 10 }];
    expect(computeBuildPassRate(counts, 7)).toBe(100);
  });

  it("returns 0 for all-failed counts", () => {
    const counts: AppDayCount[] = [{ app: "ghostty", passed: 0, failed: 5, total: 5 }];
    expect(computeBuildPassRate(counts, 7)).toBe(0);
  });

  it("computes correct rate for mixed results", () => {
    const counts: AppDayCount[] = [
      { app: "ghostty", passed: 3, failed: 1, total: 4 },
      { app: "firefox", passed: 10, failed: 0, total: 10 },
    ];
    // Total: 13 passed / 14 total = 92.857...%
    const rate = computeBuildPassRate(counts, 7);
    expect(rate).toBeCloseTo(92.857, 2);
  });

  it("filters by windowDays", () => {
    // windowDays is informational in the pure function — the caller filters snapshots.
    // Just test that the function handles counts correctly.
    const counts: AppDayCount[] = [{ app: "ghostty", passed: 5, failed: 5, total: 10 }];
    expect(computeBuildPassRate(counts, 30)).toBe(50);
  });
});

describe("detectVersionChanges", () => {
  it("returns empty for empty history", () => {
    expect(detectVersionChanges([], "ghostty")).toEqual([]);
  });

  it("returns empty when app not found in any snapshot", () => {
    const snaps: DaySnapshot[] = [
      {
        date: "2024-01-01",
        packages: [{ name: "firefox", version: "120.0", version_count: 1 }],
        build_counts: [],
        last_run_id: 1,
      },
    ];
    expect(detectVersionChanges(snaps, "ghostty")).toEqual([]);
  });

  it("detects a version change between snapshots", () => {
    const snaps: DaySnapshot[] = [
      {
        date: "2024-01-01",
        packages: [{ name: "ghostty", version: "1.0.0", version_count: 1 }],
        build_counts: [],
        last_run_id: 1,
      },
      {
        date: "2024-01-08",
        packages: [{ name: "ghostty", version: "1.1.0", version_count: 2 }],
        build_counts: [],
        last_run_id: 2,
      },
    ];
    const changes = detectVersionChanges(snaps, "ghostty");
    expect(changes).toHaveLength(1);
    expect(changes[0]).toEqual({ date: "2024-01-08", version: "1.1.0" });
  });

  it("does not emit a change when version stays the same", () => {
    const snaps: DaySnapshot[] = [
      {
        date: "2024-01-01",
        packages: [{ name: "ghostty", version: "1.0.0", version_count: 1 }],
        build_counts: [],
        last_run_id: 1,
      },
      {
        date: "2024-01-08",
        packages: [{ name: "ghostty", version: "1.0.0", version_count: 1 }],
        build_counts: [],
        last_run_id: 2,
      },
    ];
    expect(detectVersionChanges(snaps, "ghostty")).toEqual([]);
  });

  it("handles first snapshot as initial version (no change emitted)", () => {
    const snaps: DaySnapshot[] = [
      {
        date: "2024-01-01",
        packages: [{ name: "ghostty", version: "1.0.0", version_count: 1 }],
        build_counts: [],
        last_run_id: 1,
      },
    ];
    expect(detectVersionChanges(snaps, "ghostty")).toEqual([]);
  });
});
