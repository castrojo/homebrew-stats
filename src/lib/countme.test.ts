import { describe, it, expect } from "vitest";
import {
  aggregateWeekRecordsToMonthEnd,
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

  it("filters by week_end, not week_start", () => {
    const today = new Date();
    const weekEnd = today.toISOString().slice(0, 10);
    const oldWeekStart = new Date(today);
    oldWeekStart.setDate(today.getDate() - 35);
    const rec: WeekRecord = {
      week_start: oldWeekStart.toISOString().slice(0, 10),
      week_end: weekEnd,
      distros: {},
      total: 123,
    };

    expect(filterWeekRecordsByDays([rec], 30)).toHaveLength(1);
  });
});

describe("aggregateWeekRecordsToMonthEnd", () => {
  const makeRecord = (weekStart: string, weekEnd: string, total: number): WeekRecord => ({
    week_start: weekStart,
    week_end: weekEnd,
    distros: {
      bazzite: total,
      bluefin: Math.floor(total / 2),
    },
    total,
  });

  it("returns empty for empty input", () => {
    expect(aggregateWeekRecordsToMonthEnd([])).toEqual([]);
  });

  it("keeps latest week_end record per month", () => {
    const records = [
      makeRecord("2026-01-01", "2026-01-07", 100),
      makeRecord("2026-01-15", "2026-01-21", 200),
      makeRecord("2026-01-22", "2026-01-28", 300),
      makeRecord("2026-02-01", "2026-02-07", 400),
      makeRecord("2026-02-08", "2026-02-14", 500),
    ];

    const monthly = aggregateWeekRecordsToMonthEnd(records);
    expect(monthly).toHaveLength(2);
    expect(monthly[0].week_end).toBe("2026-01-28");
    expect(monthly[0].total).toBe(300);
    expect(monthly[1].week_end).toBe("2026-02-14");
    expect(monthly[1].total).toBe(500);
  });

  it("sorts output ascending by week_end", () => {
    const records = [
      makeRecord("2026-03-01", "2026-03-07", 700),
      makeRecord("2026-01-01", "2026-01-28", 300),
      makeRecord("2026-02-01", "2026-02-14", 500),
    ];

    const monthly = aggregateWeekRecordsToMonthEnd(records);
    expect(monthly.map(r => r.week_end)).toEqual(["2026-01-28", "2026-02-14", "2026-03-07"]);
  });

  it("breaks same week_end ties by later week_start", () => {
    const records = [
      makeRecord("2026-01-01", "2026-01-31", 100),
      makeRecord("2026-01-08", "2026-01-31", 200),
    ];

    const monthly = aggregateWeekRecordsToMonthEnd(records);
    expect(monthly).toHaveLength(1);
    expect(monthly[0].week_start).toBe("2026-01-08");
    expect(monthly[0].total).toBe(200);
  });
});
