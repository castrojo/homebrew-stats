import { describe, it, expect } from "vitest";
import {
  computeWoWGrowth,
  filterWeekRecordsByDays,
  type WeekRecord,
} from "./countme";

describe("computeWoWGrowth", () => {
  it("returns 0 when prev is 0", () => {
    expect(computeWoWGrowth(1000, 0)).toBe(0);
  });

  it("returns positive percentage for growth", () => {
    expect(computeWoWGrowth(110, 100)).toBeCloseTo(10.0, 5);
  });

  it("returns negative percentage for decline", () => {
    expect(computeWoWGrowth(90, 100)).toBeCloseTo(-10.0, 5);
  });

  it("returns 0 when values are equal", () => {
    expect(computeWoWGrowth(100, 100)).toBe(0);
  });

  it("handles large numbers", () => {
    expect(computeWoWGrowth(71000, 70000)).toBeCloseTo(1.4286, 3);
  });
});

describe("filterWeekRecordsByDays", () => {
  const makeRecord = (weekStart: string, total: number): WeekRecord => ({
    week_start: weekStart,
    week_end: weekStart, // not used in filtering
    distros: {},
    total,
  });

  it("returns empty for empty input", () => {
    expect(filterWeekRecordsByDays([], 30)).toEqual([]);
  });

  it("returns all records when all are within window", () => {
    const today = new Date();
    const yesterday = new Date(today);
    yesterday.setDate(today.getDate() - 1);
    const rec = makeRecord(yesterday.toISOString().slice(0, 10), 100);
    expect(filterWeekRecordsByDays([rec], 30)).toHaveLength(1);
  });

  it("excludes records older than window", () => {
    const old = makeRecord("2020-01-01", 100);
    expect(filterWeekRecordsByDays([old], 30)).toHaveLength(0);
  });

  it("preserves order (does not sort)", () => {
    const today = new Date();
    const d1 = new Date(today); d1.setDate(today.getDate() - 5);
    const d2 = new Date(today); d2.setDate(today.getDate() - 2);
    const r1 = makeRecord(d1.toISOString().slice(0, 10), 100);
    const r2 = makeRecord(d2.toISOString().slice(0, 10), 200);
    const result = filterWeekRecordsByDays([r1, r2], 30);
    expect(result).toHaveLength(2);
    expect(result[0].total).toBe(100);
    expect(result[1].total).toBe(200);
  });
});
