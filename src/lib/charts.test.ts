import { describe, it, expect } from "vitest";
import { computeTotalData, computeTopPkgs, computePackageCumulative, type DaySnapshot } from "./charts";

const snap = (date: string, downloads: Record<string, number>): DaySnapshot => ({
  date,
  taps: { "ublue-os/homebrew-tap": { uniques: 0, count: 0, downloads } },
});

const TAP = "ublue-os/homebrew-tap";

describe("computeTotalData — delta-based cumulative", () => {
  it("day 0 equals the 30d baseline total", () => {
    const history = [snap("2026-03-01", { foo: 100, bar: 200 })];
    expect(computeTotalData(history, TAP)).toEqual([300]);
  });

  it("subsequent days add positive deltas to the running total", () => {
    // day 1: 300 total (baseline)
    // day 2: 320 total → delta = +20 → cumulative = 320
    // day 3: 340 total → delta = +20 → cumulative = 340
    const history = [
      snap("2026-03-01", { foo: 100, bar: 200 }),
      snap("2026-03-02", { foo: 110, bar: 210 }),
      snap("2026-03-03", { foo: 120, bar: 220 }),
    ];
    expect(computeTotalData(history, TAP)).toEqual([300, 320, 340]);
  });

  it("clamps negative deltas to zero — old installs aging out do not reduce cumulative", () => {
    // day 1: 300 (baseline)
    // day 2: 250 (30d window shrank) → delta = -50, clamped to 0 → cumulative stays 300
    const history = [
      snap("2026-03-01", { foo: 300 }),
      snap("2026-03-02", { foo: 250 }),
    ];
    expect(computeTotalData(history, TAP)).toEqual([300, 300]);
  });

  it("regression guard: does NOT sum snapshots naively (the original bug)", () => {
    // The old buggy code did: running += snapshot_total
    // day 1: 300, day 2: 320 → buggy result = [300, 620]
    // Correct delta-based result = [300, 320]
    const history = [
      snap("2026-03-01", { foo: 300 }),
      snap("2026-03-02", { foo: 320 }),
    ];
    const result = computeTotalData(history, TAP);
    expect(result[1]).toBe(320);
    expect(result[1]).not.toBe(620); // must NOT be 300 + 320
  });

  it("returns zeros for snapshots with no download data for the tap", () => {
    const history: DaySnapshot[] = [
      { date: "2026-03-01", taps: {} },
      snap("2026-03-02", { foo: 50 }),
    ];
    // day 0: 0 (no data); day 1: delta = max(0, 50-0) = 50 → cumulative = 50
    expect(computeTotalData(history, TAP)).toEqual([0, 50]);
  });

  it("returns empty array for empty history", () => {
    expect(computeTotalData([], TAP)).toEqual([]);
  });

  it("handles a new package appearing mid-history", () => {
    // day 1: only foo=100
    // day 2: foo=110, bar=200 (bar is new) → delta = (110+200) - 100 = 210 → cumulative = 310
    const history = [
      snap("2026-03-01", { foo: 100 }),
      snap("2026-03-02", { foo: 110, bar: 200 }),
    ];
    expect(computeTotalData(history, TAP)).toEqual([100, 310]);
  });
});

describe("computePackageCumulative — delta-based per-package cumulative", () => {
  it("day 0 equals the 30d baseline for the package", () => {
    const history = [snap("2026-03-01", { foo: 100, bar: 200 })];
    expect(computePackageCumulative(history, TAP, "foo")).toBe(100);
    expect(computePackageCumulative(history, TAP, "bar")).toBe(200);
  });

  it("accumulates positive deltas across days", () => {
    const history = [
      snap("2026-03-01", { foo: 100 }),
      snap("2026-03-02", { foo: 120 }),
      snap("2026-03-03", { foo: 150 }),
    ];
    // delta: 20 + 30 = 50; cumulative = 100 + 50 = 150
    expect(computePackageCumulative(history, TAP, "foo")).toBe(150);
  });

  it("clamps negative deltas to zero — rolling window drop never reduces cumulative", () => {
    const history = [
      snap("2026-03-01", { foo: 300 }),
      snap("2026-03-02", { foo: 250 }), // 30d window shrank
      snap("2026-03-03", { foo: 280 }), // back up by 30
    ];
    // day0: 300, day1: clamped 0, day2: +30 → 330
    expect(computePackageCumulative(history, TAP, "foo")).toBe(330);
  });

  it("returns 0 for a package not present in any snapshot", () => {
    const history = [snap("2026-03-01", { bar: 500 })];
    expect(computePackageCumulative(history, TAP, "missing")).toBe(0);
  });

  it("returns 0 for empty history", () => {
    expect(computePackageCumulative([], TAP, "foo")).toBe(0);
  });

  it("handles package appearing mid-history (treated as baseline on first appearance)", () => {
    const history = [
      snap("2026-03-01", { foo: 100 }),
      snap("2026-03-02", { foo: 110, bar: 200 }), // bar appears: delta = 200-0 = 200
    ];
    expect(computePackageCumulative(history, TAP, "bar")).toBe(200);
  });

  it("regression guard: does NOT use raw 30d value on latest snapshot only", () => {
    // If you just returned snapshots[last].downloads[pkg] you'd get 320.
    // Correct cumulative = 300 (baseline) + 20 (delta day2) = 320 — same here,
    // but this case makes the distinction clear when rolling drops then rises.
    const history = [
      snap("2026-03-01", { foo: 300 }),
      snap("2026-03-02", { foo: 100 }), // window dropped by 200 (clamped)
      snap("2026-03-03", { foo: 320 }), // +220 from prev
    ];
    // cumulative = 300 + 0 + 220 = 520, NOT 320 (raw latest)
    expect(computePackageCumulative(history, TAP, "foo")).toBe(520);
    expect(computePackageCumulative(history, TAP, "foo")).not.toBe(320);
  });
});

describe("computeTopPkgs", () => {
  it("ranks packages by most recent snapshot descending", () => {
    const history = [
      snap("2026-03-01", { a: 1000, b: 500, c: 100 }),
      snap("2026-03-02", { a: 200, b: 800, c: 150 }),
    ];
    const { top } = computeTopPkgs(history, TAP, 2);
    // Most recent: b=800, a=200, c=150 → top 2 = ["b", "a"]
    expect(top).toEqual(["b", "a"]);
  });

  it("splits top N and rest correctly", () => {
    const history = [snap("2026-03-01", { a: 300, b: 200, c: 100, d: 50 })];
    const { top, rest } = computeTopPkgs(history, TAP, 2);
    expect(top).toEqual(["a", "b"]);
    expect(rest).toEqual(["c", "d"]);
  });

  it("returns empty arrays for empty history", () => {
    const { top, rest } = computeTopPkgs([], TAP, 5);
    expect(top).toEqual([]);
    expect(rest).toEqual([]);
  });

  it("when N >= all packages, rest is empty", () => {
    const history = [snap("2026-03-01", { a: 1, b: 2 })];
    const { top, rest } = computeTopPkgs(history, TAP, 10);
    expect(top).toHaveLength(2);
    expect(rest).toHaveLength(0);
  });
});
