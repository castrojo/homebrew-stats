/**
 * schema.test.ts — Data file schema validation
 *
 * These tests read the committed src/data/*.json files and verify they
 * conform to the contracts expected by Astro components. A failure here
 * means either the seed script, Go backend, or a component was changed
 * without updating the other side.
 *
 * KEY CONTRACT (catches the Bazzite-key-case bug):
 *   distros keys in week_records/day_records MUST be lowercase:
 *     bazzite, bluefin, bluefin-lts, aurora
 *   os_version_dist keys MUST be title case (raw os_name from CSV):
 *     Bazzite, Bluefin, Bluefin LTS, Aurora
 *
 * Why the difference: the Go validDistros map normalises os_name to
 * lowercase badge names for distros, but parseOsVersionDist() keeps
 * the raw os_name (title case) as the outer key.
 */

import { readFileSync } from "fs";
import { resolve } from "path";
import { describe, it, expect } from "vitest";

// ── helpers ──────────────────────────────────────────────────────────────────

function loadJSON(relPath: string): unknown {
  const abs = resolve(process.cwd(), relPath);
  return JSON.parse(readFileSync(abs, "utf8"));
}

const DISTRO_KEYS_LOWERCASE = ["bazzite", "bluefin", "bluefin-lts", "aurora"] as const;
const DISTRO_KEYS_TITLECASE = ["Bazzite", "Bluefin", "Bluefin LTS", "Aurora"] as const;

// ── countme.json ─────────────────────────────────────────────────────────────

describe("src/data/countme.json schema", () => {
  const raw = loadJSON("src/data/countme.json") as Record<string, unknown>;

  it("has required top-level fields", () => {
    expect(raw).toHaveProperty("generated_at");
    expect(raw).toHaveProperty("history");
  });

  it("history.week_records is an array (never null)", () => {
    const history = raw.history as Record<string, unknown>;
    expect(Array.isArray(history.week_records)).toBe(true);
  });

  it("history.day_records is an array (never null)", () => {
    const history = raw.history as Record<string, unknown>;
    expect(Array.isArray(history.day_records)).toBe(true);
  });

  it("week_records distros keys are lowercase — CRITICAL: components use bazzite not Bazzite", () => {
    const history = raw.history as Record<string, unknown>;
    const weeks = history.week_records as Array<{ distros: Record<string, number> }>;
    if (weeks.length === 0) return; // skip if empty (valid on first run)

    for (const week of weeks) {
      const keys = Object.keys(week.distros);
      for (const key of DISTRO_KEYS_LOWERCASE) {
        expect(
          keys,
          `week_records distros must use lowercase "${key}" not "${key[0].toUpperCase() + key.slice(1)}"`
        ).toContain(key);
      }
      // Explicitly assert no title-case keys slipped in
      expect(keys).not.toContain("Bazzite");
      expect(keys).not.toContain("Bluefin");
      expect(keys).not.toContain("Aurora");
    }
  });

  it("day_records distros keys are lowercase", () => {
    const history = raw.history as Record<string, unknown>;
    const days = history.day_records as Array<{ distros: Record<string, number> }>;
    if (days.length === 0) return;

    for (const day of days) {
      const keys = Object.keys(day.distros);
      expect(keys).not.toContain("Bazzite");
      expect(keys).not.toContain("Bluefin");
      expect(keys).not.toContain("Aurora");
    }
  });

  it("week_records have required fields with correct types", () => {
    const history = raw.history as Record<string, unknown>;
    const weeks = history.week_records as Array<Record<string, unknown>>;
    if (weeks.length === 0) return;

    for (const week of weeks) {
      expect(typeof week.week_start).toBe("string");
      expect(typeof week.week_end).toBe("string");
      expect(typeof week.total).toBe("number");
      expect(week.total).toBeGreaterThanOrEqual(0);
      // ISO date format YYYY-MM-DD
      expect(week.week_start).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    }
  });

  it("day_records have required fields with correct types", () => {
    const history = raw.history as Record<string, unknown>;
    const days = history.day_records as Array<Record<string, unknown>>;
    if (days.length === 0) return;

    for (const day of days) {
      expect(typeof day.date).toBe("string");
      expect(day.date).toMatch(/^\d{4}-\d{2}-\d{2}$/);
      expect(typeof day.total).toBe("number");
    }
  });

  it("os_version_dist keys are title case when present — FedoraVersionChart uses Bazzite not bazzite", () => {
    if (!raw.os_version_dist) return; // optional field
    const dist = raw.os_version_dist as Record<string, unknown>;
    const keys = Object.keys(dist);
    if (keys.length === 0) return;

    for (const key of keys) {
      expect(
        DISTRO_KEYS_TITLECASE as readonly string[],
        `os_version_dist key "${key}" must be title-case (Bazzite/Bluefin/Bluefin LTS/Aurora)`
      ).toContain(key);
    }
    // Must NOT use lowercase (that's the badge name, not the CSV os_name)
    expect(keys).not.toContain("bazzite");
    expect(keys).not.toContain("bluefin");
    expect(keys).not.toContain("aurora");
  });

  it("os_version_dist version keys are numeric strings", () => {
    if (!raw.os_version_dist) return;
    const dist = raw.os_version_dist as Record<string, Record<string, number>>;
    for (const [distro, versions] of Object.entries(dist)) {
      for (const [ver, count] of Object.entries(versions)) {
        expect(Number.isFinite(Number(ver)), `${distro} version key "${ver}" must be numeric`).toBe(true);
        expect(count).toBeGreaterThanOrEqual(0);
      }
    }
  });

  it("wow_growth_pct uses lowercase badge keys when present", () => {
    if (!raw.wow_growth_pct) return;
    const wow = raw.wow_growth_pct as Record<string, number>;
    expect(wow).toHaveProperty("bazzite");
    expect(wow).toHaveProperty("bluefin");
    expect(wow).toHaveProperty("aurora");
    expect(wow).toHaveProperty("total");
    // Sanity: growth pct should be a finite number
    for (const val of Object.values(wow)) {
      expect(Number.isFinite(val)).toBe(true);
    }
  });
});

// ── testhub.json ─────────────────────────────────────────────────────────────

describe("src/data/testhub.json schema", () => {
  const raw = loadJSON("src/data/testhub.json") as Record<string, unknown>;

  it("has required top-level fields", () => {
    expect(raw).toHaveProperty("generated_at");
    expect(raw).toHaveProperty("packages");
    expect(raw).toHaveProperty("build_metrics");
    expect(raw).toHaveProperty("history");
  });

  it("packages is an array (never null)", () => {
    expect(Array.isArray(raw.packages)).toBe(true);
  });

  it("build_metrics is an array (never null)", () => {
    expect(Array.isArray(raw.build_metrics)).toBe(true);
  });

  it("history is an array (never null)", () => {
    expect(Array.isArray(raw.history)).toBe(true);
  });

  it("package names do NOT have testhub/ prefix — must be stripped by ListPackages", () => {
    const pkgs = raw.packages as Array<{ name: string }>;
    for (const pkg of pkgs) {
      expect(
        pkg.name,
        `Package "${pkg.name}" must not start with "testhub/" (prefix should be stripped)`
      ).not.toMatch(/^testhub\//);
    }
  });

  it("packages have required fields with correct types", () => {
    const pkgs = raw.packages as Array<Record<string, unknown>>;
    if (pkgs.length === 0) return;

    for (const pkg of pkgs) {
      expect(typeof pkg.name).toBe("string");
      expect((pkg.name as string).length).toBeGreaterThan(0);
      expect(typeof pkg.version_count).toBe("number");
      expect(pkg.version_count as number).toBeGreaterThanOrEqual(0);
    }
  });

  it("build_metrics pass rates are in 0–100 range", () => {
    const metrics = raw.build_metrics as Array<{
      app: string;
      pass_rate_7d: number;
      pass_rate_30d: number;
    }>;
    for (const m of metrics) {
      expect(m.pass_rate_7d, `${m.app} pass_rate_7d out of range`).toBeGreaterThanOrEqual(0);
      expect(m.pass_rate_7d, `${m.app} pass_rate_7d out of range`).toBeLessThanOrEqual(100);
      expect(m.pass_rate_30d, `${m.app} pass_rate_30d out of range`).toBeGreaterThanOrEqual(0);
      expect(m.pass_rate_30d, `${m.app} pass_rate_30d out of range`).toBeLessThanOrEqual(100);
    }
  });

  it("build_metrics last_status is a valid value", () => {
    const metrics = raw.build_metrics as Array<{ last_status: string }>;
    const valid = new Set(["passing", "failing", ""]);
    for (const m of metrics) {
      if (m.last_status !== undefined) {
        expect(valid, `Invalid last_status: "${m.last_status}"`).toContain(m.last_status);
      }
    }
  });

  it("history snapshots have required fields", () => {
    const history = raw.history as Array<Record<string, unknown>>;
    if (history.length === 0) return;

    for (const snap of history) {
      expect(typeof snap.date).toBe("string");
      expect(snap.date).toMatch(/^\d{4}-\d{2}-\d{2}$/);
      expect(Array.isArray(snap.build_counts)).toBe(true);
      expect(typeof snap.last_run_id).toBe("number");
    }
  });

  it("history build_counts app names do not have testhub/ prefix", () => {
    const history = raw.history as Array<{
      build_counts: Array<{ app: string }>;
    }>;
    for (const snap of history) {
      for (const bc of snap.build_counts) {
        expect(bc.app).not.toMatch(/^testhub\//);
      }
    }
  });

  it("history build_counts passed <= total", () => {
    const history = raw.history as Array<{
      build_counts: Array<{ app: string; passed: number; total: number }>;
    }>;
    for (const snap of history) {
      for (const bc of snap.build_counts) {
        expect(bc.passed, `${bc.app}: passed (${bc.passed}) > total (${bc.total})`).toBeLessThanOrEqual(bc.total);
        expect(bc.total).toBeGreaterThanOrEqual(0);
        expect(bc.passed).toBeGreaterThanOrEqual(0);
      }
    }
  });
});
