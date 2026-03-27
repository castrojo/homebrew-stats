export interface WeekRecord {
  week_start: string;
  week_end: string;
  distros: Record<string, number>;
  total: number;
}

/**
 * Collapse weekly records into one record per month using month-end semantics.
 * For each month (YYYY-MM), keeps the record with the latest week_end date.
 * Returns records sorted ascending by week_end.
 */
export function aggregateWeekRecordsToMonthEnd(records: WeekRecord[]): WeekRecord[] {
  const monthEndByMonth = new Map<string, WeekRecord>();

  for (const record of records) {
    const month = record.week_end.slice(0, 7);
    const existing = monthEndByMonth.get(month);

    if (
      !existing ||
      record.week_end > existing.week_end ||
      (record.week_end === existing.week_end && record.week_start > existing.week_start)
    ) {
      monthEndByMonth.set(month, record);
    }
  }

  return [...monthEndByMonth.values()].sort((a, b) => a.week_end.localeCompare(b.week_end));
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
 * Filter week records to those with week_end within the last `days` days.
 * Does not sort — preserves input order.
 */
export function filterWeekRecordsByDays(records: WeekRecord[], days: number): WeekRecord[] {
  const cutoff = new Date();
  cutoff.setDate(cutoff.getDate() - days);
  const cutoffStr = cutoff.toISOString().slice(0, 10);
  return records.filter(r => r.week_end >= cutoffStr);
}
