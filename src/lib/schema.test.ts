/**
 * schema.test.ts — Data file schema validation
 *
 * These tests read the committed src/data/*.json files and verify they
 * conform to the contracts expected by Astro components. A failure here
 * means either the seed script, Go backend, or a component was changed
 * without updating the other side.
 *
 * KEY CONTRACT (catches the Bazzite-key-case bug):
 *   distros keys in week_records/day_records MUST be lowercase when present:
 *     bazzite, bluefin, aurora, secureblue, wayblue, origami
 *   bluefin-lts uses CentOS/EPEL repos so it never appears in countme data.
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

// All valid lowercase distro keys — not all must be present in every record
// (e.g. bluefin-lts uses CentOS repos so it never appears; secureblue/wayblue/origami are tracked too)
const DISTRO_KEYS_LOWERCASE = ["bazzite", "bluefin", "aurora", "secureblue", "wayblue", "origami"] as const;
const DISTRO_KEYS_TITLECASE = ["Bazzite", "Bluefin", "Aurora"] as const;

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
      // Every key that IS present must be a known lowercase key.
      // Not all keys need to be present (e.g. bluefin-lts uses CentOS repos, never appears).
      for (const key of keys) {
        expect(
          DISTRO_KEYS_LOWERCASE as readonly string[],
          `week_records distros key "${key}" must be lowercase and in the allowed set`
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
      // pull_count is optional (GitHub Packages API may not expose it yet); when present must be a non-negative number
      if (pkg.pull_count !== undefined && pkg.pull_count !== null) {
        expect(typeof pkg.pull_count).toBe("number");
        expect(pkg.pull_count as number).toBeGreaterThanOrEqual(0);
      }
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
    const valid = new Set(["passing", "failing", "unknown", ""]);
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

// ── contributors.json ─────────────────────────────────────────────────────────

describe("src/data/contributors.json schema", () => {
  const raw = loadJSON("src/data/contributors.json") as Record<string, unknown>;

  it("has required top-level fields", () => {
    expect(raw).toHaveProperty("generated_at");
    expect(raw).toHaveProperty("period");
    expect(raw).toHaveProperty("summary");
    expect(raw).toHaveProperty("top_contributors");
    expect(raw).toHaveProperty("repos");
    expect(raw).toHaveProperty("discussions_summary");
  });

  it("generated_at is a non-empty string", () => {
    expect(typeof raw.generated_at).toBe("string");
    expect((raw.generated_at as string).length).toBeGreaterThan(0);
  });

  it("period has start and end date strings", () => {
    const period = raw.period as Record<string, unknown>;
    expect(typeof period.start).toBe("string");
    expect(typeof period.end).toBe("string");
  });

  it("summary fields are non-negative numbers", () => {
    const s = raw.summary as Record<string, unknown>;
    const numFields = [
      "active_contributors", "new_contributors", "total_commits",
      "total_prs_merged", "total_issues_opened", "total_issues_closed",
      "bus_factor", "active_repos", "total_discussions",
    ];
    for (const f of numFields) {
      expect(typeof s[f], `summary.${f} must be a number`).toBe("number");
      expect(s[f] as number, `summary.${f} must be >= 0`).toBeGreaterThanOrEqual(0);
    }
  });

  it("summary.bus_factor is an integer >= 1 when contributors exist", () => {
    const s = raw.summary as Record<string, number>;
    const contribs = raw.top_contributors as unknown[];
    if (contribs.length > 0) {
      expect(s.bus_factor).toBeGreaterThanOrEqual(1);
      expect(Number.isInteger(s.bus_factor)).toBe(true);
    }
  });

  it("summary.review_participation_rate is in [0, 1]", () => {
    const s = raw.summary as Record<string, number>;
    expect(s.review_participation_rate).toBeGreaterThanOrEqual(0);
    expect(s.review_participation_rate).toBeLessThanOrEqual(1);
  });

  it("summary.discussion_answer_rate is in [0, 1]", () => {
    const s = raw.summary as Record<string, number>;
    expect(s.discussion_answer_rate).toBeGreaterThanOrEqual(0);
    expect(s.discussion_answer_rate).toBeLessThanOrEqual(1);
  });

  it("top_contributors is an array (never null)", () => {
    expect(Array.isArray(raw.top_contributors)).toBe(true);
  });

  it("top_contributors entries have required fields with correct types", () => {
    const contribs = raw.top_contributors as Array<Record<string, unknown>>;
    if (contribs.length === 0) return;
    for (const c of contribs) {
      expect(typeof c.login, `login must be string`).toBe("string");
      expect((c.login as string).length, `login must be non-empty`).toBeGreaterThan(0);
      expect(typeof c.commits_30d, `commits_30d must be number`).toBe("number");
      expect(c.commits_30d as number).toBeGreaterThanOrEqual(0);
      expect(typeof c.prs_merged_30d).toBe("number");
      expect(typeof c.issues_opened_30d).toBe("number");
      expect(typeof c.is_bot).toBe("boolean");
      expect(Array.isArray(c.repos_active)).toBe(true);
    }
  });

  it("top_contributors bot flag: logins ending in [bot] must have is_bot=true", () => {
    const contribs = raw.top_contributors as Array<{ login: string; is_bot: boolean }>;
    for (const c of contribs) {
      if (c.login.endsWith("[bot]")) {
        expect(c.is_bot, `${c.login} must have is_bot=true`).toBe(true);
      }
    }
  });

  it("repos is an array (never null)", () => {
    expect(Array.isArray(raw.repos)).toBe(true);
  });

  it("repos entries have required fields", () => {
    const repos = raw.repos as Array<Record<string, unknown>>;
    if (repos.length === 0) return;
    for (const r of repos) {
      expect(typeof r.name).toBe("string");
      expect((r.name as string).length).toBeGreaterThan(0);
      expect(typeof r.commits_30d).toBe("number");
      expect(typeof r.bus_factor).toBe("number");
      expect(Array.isArray(r.weekly_commits_52w)).toBe(true);
    }
  });

  it("repos weekly_commits_52w has at most 52 entries", () => {
    const repos = raw.repos as Array<{ weekly_commits_52w: number[] }>;
    for (const r of repos) {
      expect(r.weekly_commits_52w.length).toBeLessThanOrEqual(52);
      for (const count of r.weekly_commits_52w) {
        expect(typeof count).toBe("number");
        expect(count).toBeGreaterThanOrEqual(0);
      }
    }
  });

  it("repos human_commits_30d + bot_commits_30d <= commits_30d", () => {
    const repos = raw.repos as Array<{
      commits_30d: number;
      human_commits_30d: number;
      bot_commits_30d: number;
    }>;
    for (const r of repos) {
      // Allow a small tolerance for unlinked commits not counted in either bucket
      expect(r.human_commits_30d + r.bot_commits_30d).toBeLessThanOrEqual(r.commits_30d);
    }
  });

  it("discussions_summary has required fields", () => {
    const ds = raw.discussions_summary as Record<string, unknown>;
    expect(typeof ds.total_discussions_30d).toBe("number");
    expect(typeof ds.total_discussion_comments_30d).toBe("number");
    expect(Array.isArray(ds.weekly_trend)).toBe(true);
  });

  it("discussions_summary.answered_rate is in [0, 1]", () => {
    const ds = raw.discussions_summary as Record<string, number>;
    expect(ds.answered_rate).toBeGreaterThanOrEqual(0);
    expect(ds.answered_rate).toBeLessThanOrEqual(1);
  });

  it("discussions_summary.weekly_trend entries have week + counts", () => {
    const ds = raw.discussions_summary as { weekly_trend: Array<Record<string, unknown>> };
    for (const entry of ds.weekly_trend) {
      expect(typeof entry.week).toBe("string");
      expect(typeof entry.discussions).toBe("number");
      expect(typeof entry.comments).toBe("number");
    }
  });

  it("top_contributors have _60d and _365d commit fields", () => {
    const data = raw as Record<string, unknown>;
    const contribs = data.top_contributors as Array<Record<string, unknown>>;
    if (contribs.length === 0) return;
    for (const c of contribs.slice(0, 3)) {
      expect(typeof c.commits_60d, `${c.login}: commits_60d must be a number`).toBe("number");
      expect(typeof c.commits_365d, `${c.login}: commits_365d must be a number`).toBe("number");
      expect(c.commits_60d as number, `commits_60d >= commits_30d`).toBeGreaterThanOrEqual(c.commits_30d as number);
      expect(c.commits_365d as number, `commits_365d >= commits_60d`).toBeGreaterThanOrEqual(c.commits_60d as number);
      if (c.commits_90d !== undefined) {
        expect(typeof c.commits_90d, `${c.login}: commits_90d must be a number when present`).toBe("number");
        expect(c.commits_90d as number, `commits_90d >= commits_30d`).toBeGreaterThanOrEqual(c.commits_30d as number);
        expect(c.commits_365d as number, `commits_365d >= commits_90d`).toBeGreaterThanOrEqual(c.commits_90d as number);
      }
    }
  });

  it("top_contributors have _60d and _365d prs_merged fields", () => {
    const data = raw as Record<string, unknown>;
    const contribs = data.top_contributors as Array<Record<string, unknown>>;
    if (contribs.length === 0) return;
    for (const c of contribs.slice(0, 3)) {
      expect(typeof c.prs_merged_60d, `${c.login}: prs_merged_60d must be a number`).toBe("number");
      expect(typeof c.prs_merged_365d, `${c.login}: prs_merged_365d must be a number`).toBe("number");
      if (c.prs_merged_90d !== undefined) {
        expect(typeof c.prs_merged_90d, `${c.login}: prs_merged_90d must be a number when present`).toBe("number");
      }
    }
  });

  it("repos have _60d and _365d commit fields", () => {
    const data = raw as Record<string, unknown>;
    const repos = data.repos as Array<Record<string, unknown>>;
    if (repos.length === 0) return;
    const r = repos[0];
    expect(typeof r.commits_60d, `repos[0].commits_60d must be a number`).toBe("number");
    expect(typeof r.commits_365d, `repos[0].commits_365d must be a number`).toBe("number");
    expect(r.commits_60d as number).toBeGreaterThanOrEqual(0);
    expect(r.commits_365d as number).toBeGreaterThanOrEqual(r.commits_60d as number);
    if (r.commits_90d !== undefined) {
      expect(typeof r.commits_90d, `repos[0].commits_90d must be a number when present`).toBe("number");
      expect(r.commits_90d as number).toBeGreaterThanOrEqual(r.commits_30d as number);
      expect(r.commits_365d as number).toBeGreaterThanOrEqual(r.commits_90d as number);
    }
  });

  it("repos have _60d and _365d bus_factor fields", () => {
    const data = raw as Record<string, unknown>;
    const repos = data.repos as Array<Record<string, unknown>>;
    if (repos.length === 0) return;
    for (const r of repos) {
      expect(typeof r.bus_factor_60d, `${r.name}: bus_factor_60d must be a number`).toBe("number");
      expect(typeof r.bus_factor_365d, `${r.name}: bus_factor_365d must be a number`).toBe("number");
      expect(r.bus_factor_60d as number).toBeGreaterThanOrEqual(1);
      expect(r.bus_factor_365d as number).toBeGreaterThanOrEqual(1);
      if (r.bus_factor_90d !== undefined) {
        expect(typeof r.bus_factor_90d, `${r.name}: bus_factor_90d must be a number when present`).toBe("number");
        expect(r.bus_factor_90d as number).toBeGreaterThanOrEqual(1);
      }
    }
  });

  it("summary has _60d and _365d fields", () => {
    const s = raw.summary as Record<string, unknown>;
    const fields60 = ["active_contributors_60d", "total_commits_60d", "total_prs_merged_60d", "bus_factor_60d"];
    const fields365 = ["active_contributors_365d", "total_commits_365d", "total_prs_merged_365d", "bus_factor_365d"];
    for (const f of [...fields60, ...fields365]) {
      expect(typeof s[f], `summary.${f} must be a number`).toBe("number");
      expect(s[f] as number, `summary.${f} must be >= 0`).toBeGreaterThanOrEqual(0);
    }
    // 365d >= 60d >= 30d for additive metrics
    expect(s.total_commits_60d as number).toBeGreaterThanOrEqual(s.total_commits as number);
    expect(s.total_commits_365d as number).toBeGreaterThanOrEqual(s.total_commits_60d as number);
    expect(s.total_prs_merged_60d as number).toBeGreaterThanOrEqual(s.total_prs_merged as number);
    expect(s.total_prs_merged_365d as number).toBeGreaterThanOrEqual(s.total_prs_merged_60d as number);
    if (s.total_commits_90d !== undefined) {
      expect(typeof s.total_commits_90d, `summary.total_commits_90d must be a number when present`).toBe("number");
      expect(s.total_commits_90d as number).toBeGreaterThanOrEqual(s.total_commits as number);
      expect(s.total_commits_365d as number).toBeGreaterThanOrEqual(s.total_commits_90d as number);
    }
    if (s.total_prs_merged_90d !== undefined) {
      expect(typeof s.total_prs_merged_90d, `summary.total_prs_merged_90d must be a number when present`).toBe("number");
      expect(s.total_prs_merged_90d as number).toBeGreaterThanOrEqual(s.total_prs_merged as number);
      expect(s.total_prs_merged_365d as number).toBeGreaterThanOrEqual(s.total_prs_merged_90d as number);
    }
    if (s.active_contributors_90d !== undefined) {
      expect(typeof s.active_contributors_90d, `summary.active_contributors_90d must be a number when present`).toBe("number");
      expect(s.active_contributors_90d as number).toBeGreaterThanOrEqual(s.active_contributors as number);
      expect(s.active_contributors_365d as number).toBeGreaterThanOrEqual(s.active_contributors_90d as number);
    }
    if (s.bus_factor_90d !== undefined) {
      expect(typeof s.bus_factor_90d, `summary.bus_factor_90d must be a number when present`).toBe("number");
      expect(s.bus_factor_90d as number).toBeGreaterThanOrEqual(1);
    }
  });

  it("discussions_summary has _60d and _365d fields", () => {
    const ds = raw.discussions_summary as Record<string, unknown>;
    expect(typeof ds.total_discussions_60d).toBe("number");
    expect(typeof ds.total_discussions_365d).toBe("number");
    expect(typeof ds.total_discussion_comments_60d).toBe("number");
    expect(typeof ds.total_discussion_comments_365d).toBe("number");
    expect(typeof ds.unique_discussion_authors_60d).toBe("number");
    expect(typeof ds.unique_discussion_authors_365d).toBe("number");
    // monotonic: 365d >= 60d >= 30d
    expect(ds.total_discussions_60d as number).toBeGreaterThanOrEqual(ds.total_discussions_30d as number);
    expect(ds.total_discussions_365d as number).toBeGreaterThanOrEqual(ds.total_discussions_60d as number);
    if (ds.total_discussions_90d !== undefined) {
      expect(typeof ds.total_discussions_90d).toBe("number");
      expect(ds.total_discussions_90d as number).toBeGreaterThanOrEqual(ds.total_discussions_30d as number);
      expect(ds.total_discussions_365d as number).toBeGreaterThanOrEqual(ds.total_discussions_90d as number);
    }
    if (ds.total_discussion_comments_90d !== undefined) {
      expect(typeof ds.total_discussion_comments_90d).toBe("number");
      expect(ds.total_discussion_comments_90d as number).toBeGreaterThanOrEqual(ds.total_discussion_comments_30d as number);
      expect(ds.total_discussion_comments_365d as number).toBeGreaterThanOrEqual(ds.total_discussion_comments_90d as number);
    }
    if (ds.unique_discussion_authors_90d !== undefined) {
      expect(typeof ds.unique_discussion_authors_90d).toBe("number");
      expect(ds.unique_discussion_authors_90d as number).toBeGreaterThanOrEqual(ds.unique_discussion_authors_30d as number);
      expect(ds.unique_discussion_authors_365d as number).toBeGreaterThanOrEqual(ds.unique_discussion_authors_90d as number);
    }
  });
});

import buildsData from '../data/builds-bluefin.json';

describe('builds-bluefin.json schema', () => {
  const data = buildsData as Record<string, unknown>;

  it('has required top-level keys', () => {
    expect(buildsData).toHaveProperty('generated_at');
    expect(buildsData).toHaveProperty('summary');
    expect(buildsData).toHaveProperty('dora_metrics');
    expect(buildsData).toHaveProperty('repos');
    expect(buildsData).toHaveProperty('top_flaky');
    expect(buildsData).toHaveProperty('recent_builds');
    expect(buildsData).toHaveProperty('duration_trend');
    expect(buildsData).toHaveProperty('failure_breakdown');
    expect(buildsData).toHaveProperty('trigger_breakdown');
    expect(buildsData).toHaveProperty('history');
  });

  it('summary has required fields with correct types', () => {
    const s = data.summary as Record<string, unknown>;
    expect(typeof s.overall_success_rate_7d).toBe('number');
    expect(typeof s.overall_success_rate_30d).toBe('number');
    expect(typeof s.total_builds_7d).toBe('number');
    expect(typeof s.avg_duration_min).toBe('number');
    expect(typeof s.health_status).toBe('string');
  });

  it('dora_metrics has required fields with correct types', () => {
    const d = data.dora_metrics as Record<string, unknown>;
    expect(typeof d.deploy_freq_per_week).toBe('number');
    expect(typeof d.lead_time_minutes).toBe('number');
    expect(typeof d.change_failure_rate_pct).toBe('number');
    expect(typeof d.mttr_minutes).toBe('number');
    expect(typeof d.mtbf_hours).toBe('number');
    expect(typeof d.dora_level).toBe('string');
  });

  it('repos is an array', () => {
    expect(Array.isArray(data.repos)).toBe(true);
  });

  it('recent_builds is an array', () => {
    expect(Array.isArray(data.recent_builds)).toBe(true);
  });

  it('history is an array', () => {
    expect(Array.isArray(data.history)).toBe(true);
  });

  it('top_flaky is an array', () => {
    expect(Array.isArray(data.top_flaky)).toBe(true);
  });

  it('trigger_breakdown has correct shape', () => {
    const t = data.trigger_breakdown as Record<string, unknown>;
    expect(typeof t.scheduled).toBe('number');
    expect(typeof t.push).toBe('number');
    expect(typeof t.pull_request).toBe('number');
    expect(typeof t.workflow_dispatch).toBe('number');
    expect(typeof t.other).toBe('number');
  });
});
