/**
 * Shared Chart.js theme utilities.
 * Import this in every chart component's <script> — never declare getCSSVar locally.
 */

/** Brand colour palette — consistent ordering across all charts. */
export const BRAND_COLOURS = [
  '#388bfd', // blue
  '#a371f7', // purple
  '#3fb950', // green
  '#d29922', // yellow
  '#f78166', // red-orange
  '#79c0ff', // light blue
  '#d2a8ff', // light purple
  '#56d364', // light green
  '#e3b341', // amber
  '#ff7b72', // red
] as const;

/** Read a CSS custom property value from the document root. */
export function getCSSVar(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
}

/** Returns current theme colours for use in Chart.js options. */
export function getChartColors(): { text: string; muted: string; grid: string } {
  return {
    text: getCSSVar('--text'),
    muted: getCSSVar('--muted'),
    grid: getCSSVar('--border'),
  };
}

/**
 * Returns Chart.js scale/plugin options reflecting the current theme.
 * Use this as a starting point; override specific options as needed.
 */
export function getChartDefaults(colors: ReturnType<typeof getChartColors>) {
  return {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: {
        labels: { color: colors.text, font: { size: 12 } },
      },
    },
    scales: {
      x: {
        ticks: { color: colors.muted, font: { size: 11 } },
        grid: { color: colors.grid },
      },
      y: {
        beginAtZero: true,
        ticks: { color: colors.muted, font: { size: 11 } },
        grid: { color: colors.grid },
      },
    },
  };
}

/**
 * Updates an existing Chart instance with current theme colours without destroying it.
 * Call this inside a window.addEventListener('themechange', ...) handler.
 *
 * This handles the common Line/Bar case. For charts with custom scale options,
 * call chart.update() after manually setting options.
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function applyTheme(chart: { options: any; update: () => void }): void {
  const colors = getChartColors();
  try {
    if (chart.options.plugins?.legend?.labels) {
      chart.options.plugins.legend.labels.color = colors.text;
    }
    if (chart.options.scales?.x) {
      chart.options.scales.x.ticks.color = colors.muted;
      chart.options.scales.x.grid.color = colors.grid;
    }
    if (chart.options.scales?.y) {
      chart.options.scales.y.ticks.color = colors.muted;
      chart.options.scales.y.grid.color = colors.grid;
      if (chart.options.scales.y.title) {
        chart.options.scales.y.title.color = colors.muted;
      }
    }
    chart.update();
  } catch {
    // Theme update is best-effort; never crash the chart
  }
}
