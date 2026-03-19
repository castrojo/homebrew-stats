/**
 * Chart.js component registration — import this module for its side effects.
 *
 * Registers only the chart types and components actually used by this site,
 * instead of the kitchen-sink chart.js/auto import.
 *
 * Used chart types: Line, Bar, Doughnut
 * Used scales: Category (x-axis labels), Linear (y-axis numbers)
 * Used elements: Point, Line, Bar, Arc
 * Used plugins: Tooltip, Legend
 *
 * Chart.register() is idempotent — safe to import from multiple modules/pages.
 *
 * NEVER import chart.js/auto in component files. Always import this module instead.
 */
import {
  Chart,
  LineController,
  LineElement,
  PointElement,
  LinearScale,
  CategoryScale,
  BarController,
  BarElement,
  DoughnutController,
  ArcElement,
  Tooltip,
  Legend,
  Filler,
} from 'chart.js';

Chart.register(
  LineController,
  LineElement,
  PointElement,
  LinearScale,
  CategoryScale,
  BarController,
  BarElement,
  DoughnutController,
  ArcElement,
  Tooltip,
  Legend,
  Filler,
);
