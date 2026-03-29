/**
 * Shared utility functions for builds dashboard components.
 * Single source of truth for color thresholds and status icons.
 */

/**
 * Returns a CSS color variable based on a success rate percentage.
 * Thresholds: >=95 = green, >=85 = yellow, else = red.
 */
export function rateColor(rate: number): string {
  if (rate >= 95) return 'var(--green)';
  if (rate >= 85) return 'var(--yellow)';
  return 'var(--red)';
}

/**
 * Returns a colored dot emoji for a CI conclusion string.
 */
export function conclusionDot(conclusion: string): string {
  switch (conclusion) {
    case 'success':   return '🟢';
    case 'failure':   return '🔴';
    case 'cancelled': return '⚪';
    default:          return '🟡';
  }
}

/**
 * Returns a CSS color variable for a DORA performance level string.
 */
export function doraLevelColor(level: string): string {
  switch (level) {
    case 'elite':  return 'var(--blue)';
    case 'high':   return 'var(--green)';
    case 'medium': return 'var(--yellow)';
    case 'low':    return 'var(--red)';
    default:       return 'var(--muted)';
  }
}

/**
 * Maps a DORA level string to a numeric ordinal for chart y-axis.
 * elite=4, high=3, medium=2, low=1, unknown=0.
 */
export function doraLevelOrdinal(level: string): number {
  switch (level) {
    case 'elite':  return 4;
    case 'high':   return 3;
    case 'medium': return 2;
    case 'low':    return 1;
    default:       return 0;
  }
}
