export interface WeekRecord {
  week_start: string;
  week_end: string;
  distros: Record<string, number>;
  total: number;
}

/**
 * Compute week-over-week percentage growth.
 * Returns 0 if prev is 0 (avoids division by zero).
 */
export function computeWoWGrowth(current: number, prev: number): number {
  if (prev === 0) return 0;
  return ((current - prev) / prev) * 100;
}

/**
 * Filter week records to those with week_start within the last `days` days.
 * Does not sort — preserves input order.
 */
export function filterWeekRecordsByDays(records: WeekRecord[], days: number): WeekRecord[] {
  const cutoff = new Date();
  cutoff.setDate(cutoff.getDate() - days);
  const cutoffStr = cutoff.toISOString().slice(0, 10);
  return records.filter(r => r.week_start >= cutoffStr);
}
