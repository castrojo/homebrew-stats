#!/usr/bin/env python3
"""
seed-history.py — One-shot seed for testhub and countme historical data.

Generates 30 days of realistic simulated history so charts are visible
on day 1. CI will append real data from this baseline onward.

Real baseline numbers (from live badge endpoints, 2026-03-19):
  Bazzite:     71,000 active users
  Bluefin:      3,600
  Bluefin-LTS:     64
  Aurora:       2,600

Real testhub apps (from projectbluefin/testhub flatpaks/):
  firefox-nightly, ghostty, goose, io.github.denysmb.kontainer,
  lmstudio, org.altlinux.tuner, rancher-desktop, thunderbird-nightly,
  virtualbox

Run from repo root:
  python3 scripts/seed-history.py
"""

import json
import math
import os
import random

from datetime import date, timedelta

random.seed(42)  # reproducible

# ── config ────────────────────────────────────────────────────────────────────

TODAY = date(2026, 3, 19)
DAYS = 30

# Real testhub apps with actual package metadata from GitHub API
TESTHUB_APPS = [
    {"name": "firefox-nightly",              "updated_at": "2026-03-18", "version_count": 22, "html_url": "https://github.com/orgs/projectbluefin/packages/container/package/testhub%2Ffirefox-nightly"},
    {"name": "ghostty",                      "updated_at": "2026-03-16", "version_count": 18, "html_url": "https://github.com/orgs/projectbluefin/packages/container/package/testhub%2Fghostty"},
    {"name": "goose",                        "updated_at": "2026-03-17", "version_count": 31, "html_url": "https://github.com/orgs/projectbluefin/packages/container/package/testhub%2Fgoose"},
    {"name": "io.github.denysmb.kontainer",  "updated_at": "2026-03-15", "version_count": 9,  "html_url": "https://github.com/orgs/projectbluefin/packages/container/package/testhub%2Fio.github.denysmb.kontainer"},
    {"name": "lmstudio",                     "updated_at": "2026-03-15", "version_count": 14, "html_url": "https://github.com/orgs/projectbluefin/packages/container/package/testhub%2Flmstudio"},
    {"name": "org.altlinux.tuner",           "updated_at": "2026-03-15", "version_count": 7,  "html_url": "https://github.com/orgs/projectbluefin/packages/container/package/testhub%2Forg.altlinux.tuner"},
    {"name": "rancher-desktop",              "updated_at": "2026-03-17", "version_count": 19, "html_url": "https://github.com/orgs/projectbluefin/packages/container/package/testhub%2Francher-desktop"},
    {"name": "thunderbird-nightly",          "updated_at": "2026-03-17", "version_count": 28, "html_url": "https://github.com/orgs/projectbluefin/packages/container/package/testhub%2Fthunderbird-nightly"},
    {"name": "virtualbox",                   "updated_at": "2026-03-15", "version_count": 11, "html_url": "https://github.com/orgs/projectbluefin/packages/container/package/testhub%2Fvirtualbox"},
]

# Per-app pass rate baseline (0.0–1.0). Some apps are flakier than others.
APP_PASS_RATE = {
    "firefox-nightly":             0.82,
    "ghostty":                     0.97,
    "goose":                       0.91,
    "io.github.denysmb.kontainer": 0.95,
    "lmstudio":                    0.78,
    "org.altlinux.tuner":          0.93,
    "rancher-desktop":             0.88,
    "thunderbird-nightly":         0.84,
    "virtualbox":                  0.75,
}

# Countme baseline (real numbers, current week)
BASELINE = {
    "Bazzite":     71_000,
    "Bluefin":      3_600,
    "Bluefin LTS":     64,
    "Aurora":       2_600,
}

# Fedora version distribution per distro (realistic for March 2026)
OS_VERSION_DIST = {
    "Bazzite":    {"40": 22_000, "41": 45_000, "42": 4_000},
    "Bluefin":    {"40": 1_000,  "41": 2_400,  "42": 200},
    "Bluefin LTS":{"40": 20,     "41": 42,     "42": 2},
    "Aurora":     {"40": 900,    "41": 1_550,  "42": 150},
}


# ── helpers ───────────────────────────────────────────────────────────────────

def monday_of(d: date) -> date:
    return d - timedelta(days=d.weekday())


def week_end_of(d: date) -> date:
    return monday_of(d) + timedelta(days=6)


def simulate_build_counts(app: str, day_idx: int) -> dict:
    """Simulate build job results for an app on a given day."""
    base = APP_PASS_RATE[app]
    # Add a small random walk for realism
    noise = random.gauss(0, 0.05)
    rate = max(0.0, min(1.0, base + noise))
    total = random.randint(1, 4)
    passed = round(rate * total)
    failed = total - passed
    return {"app": app, "passed": passed, "failed": failed, "total": total}


def grow_toward(baseline: int, days_ago: int, total_days: int) -> int:
    """Return a value that grows from ~85% of baseline toward baseline over total_days."""
    fraction = days_ago / total_days  # 1.0 at start, 0.0 at end (today)
    factor = 0.85 + 0.15 * (1.0 - fraction)
    noise = random.gauss(0, 0.015)
    return max(1, round(baseline * (factor + noise)))


# ── generate testhub history ──────────────────────────────────────────────────

def gen_testhub_history() -> list:
    snapshots = []
    for i in range(DAYS - 1, -1, -1):  # oldest first
        day = TODAY - timedelta(days=i)
        build_counts = [simulate_build_counts(a["name"], i) for a in TESTHUB_APPS]
        snap = {
            "date": day.isoformat(),
            "packages": [
                {
                    "name": a["name"],
                    "version": f"latest",
                    "html_url": a["html_url"],
                    "version_count": a["version_count"],
                    "updated_at": a["updated_at"] + "T00:00:00Z",
                    "created_at": "",
                }
                for a in TESTHUB_APPS
            ],
            "build_counts": build_counts,
            "last_run_id": 1000000 + (DAYS - i) * 100,
        }
        snapshots.append(snap)
    return snapshots


def compute_build_metrics(snapshots: list, window: int) -> list:
    """Compute build metrics from snapshots, matching Go logic."""
    if not snapshots:
        return []
    latest_date = max(s["date"] for s in snapshots)
    from datetime import datetime
    latest = datetime.fromisoformat(latest_date).date()
    cutoff = (latest - timedelta(days=window)).isoformat()

    agg = {}
    for snap in snapshots:
        if snap["date"] < cutoff:
            continue
        for bc in snap["build_counts"]:
            app = bc["app"]
            if app not in agg:
                agg[app] = {"passed": 0, "failed": 0, "total": 0}
            agg[app]["passed"] += bc["passed"]
            agg[app]["failed"] += bc["failed"]
            agg[app]["total"] += bc["total"]

    metrics = []
    for app, a in agg.items():
        rate = (a["passed"] / a["total"] * 100.0) if a["total"] > 0 else 0.0
        metrics.append({
            "app": app,
            "pass_rate_7d": round(rate, 1),
            "pass_rate_30d": 0.0,
            "last_status": "passing" if rate >= 70 else "failing",
            "last_build_at": TODAY.isoformat() + "T00:00:00Z",
        })

    # Backfill pass_rate_30d
    agg30 = {}
    cutoff30 = (latest - timedelta(days=30)).isoformat()
    for snap in snapshots:
        if snap["date"] < cutoff30:
            continue
        for bc in snap["build_counts"]:
            app = bc["app"]
            if app not in agg30:
                agg30[app] = {"passed": 0, "total": 0}
            agg30[app]["passed"] += bc["passed"]
            agg30[app]["total"] += bc["total"]
    for m in metrics:
        a30 = agg30.get(m["app"], {})
        total30 = a30.get("total", 0)
        m["pass_rate_30d"] = round(a30.get("passed", 0) / total30 * 100.0, 1) if total30 > 0 else 0.0

    return metrics


def build_testhub_json(snapshots: list) -> dict:
    metrics = compute_build_metrics(snapshots, 7)
    return {
        "generated_at": TODAY.isoformat() + "T00:00:00Z",
        "packages": [
            {
                "name": a["name"],
                "version": "latest",
                "html_url": a["html_url"],
                "version_count": a["version_count"],
                "updated_at": a["updated_at"] + "T00:00:00Z",
                "created_at": "",
            }
            for a in TESTHUB_APPS
        ],
        "build_metrics": metrics,
        "history": snapshots,
    }


# ── generate countme history ──────────────────────────────────────────────────

def gen_countme_history() -> dict:
    week_records = []
    day_records = []

    seen_weeks = {}

    for i in range(DAYS - 1, -1, -1):  # oldest first
        day = TODAY - timedelta(days=i)
        week_start = monday_of(day)
        week_end = week_end_of(day)
        ws = week_start.isoformat()
        we = week_end.isoformat()

        distros = {
            name: grow_toward(val, i, DAYS)
            for name, val in BASELINE.items()
        }
        total = sum(distros.values())

        day_records.append({
            "date": day.isoformat(),
            "distros": distros,
            "total": total,
            "week_start": ws,
            "week_end": we,
        })

        # Week record: last day of a week (or last day overall) defines the week
        if ws not in seen_weeks or day >= seen_weeks[ws]["_day"]:
            seen_weeks[ws] = {
                "_day": day,
                "week_start": ws,
                "week_end": we,
                "distros": distros,
                "total": total,
            }

    for ws, wr in sorted(seen_weeks.items()):
        week_records.append({k: v for k, v in wr.items() if k != "_day"})

    return {
        "week_records": week_records,
        "day_records": day_records,
    }


def build_countme_json(history: dict) -> dict:
    week_records = history["week_records"]
    sorted_weeks = sorted(week_records, key=lambda w: w["week_start"], reverse=True)

    current_week = sorted_weeks[0] if sorted_weeks else None
    prev_week = sorted_weeks[1] if len(sorted_weeks) >= 2 else None

    wow = None
    if current_week and prev_week:
        def pct_change(new, old):
            if old == 0:
                return 0.0
            return round((new - old) / old * 100.0, 1)

        wow = {
            "bazzite":     pct_change(current_week["distros"].get("Bazzite", 0),     prev_week["distros"].get("Bazzite", 0)),
            "bluefin":     pct_change(current_week["distros"].get("Bluefin", 0),     prev_week["distros"].get("Bluefin", 0)),
            "bluefin-lts": pct_change(current_week["distros"].get("Bluefin LTS", 0), prev_week["distros"].get("Bluefin LTS", 0)),
            "aurora":      pct_change(current_week["distros"].get("Aurora", 0),       prev_week["distros"].get("Aurora", 0)),
            "total":       pct_change(current_week["total"],                          prev_week["total"]),
        }

    return {
        "generated_at": TODAY.isoformat() + "T00:00:00Z",
        "current_week": current_week,
        "prev_week": prev_week,
        "wow_growth_pct": wow,
        "history": history,
        "os_version_dist": OS_VERSION_DIST,
    }


# ── write files ───────────────────────────────────────────────────────────────

def write_json(path: str, data: dict) -> None:
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w") as f:
        json.dump(data, f, indent=2)
    print(f"✓ wrote {path}")


def main():
    print("Generating testhub history…")
    testhub_snapshots = gen_testhub_history()
    testhub_out = build_testhub_json(testhub_snapshots)

    print("Generating countme history…")
    countme_history = gen_countme_history()
    countme_out = build_countme_json(countme_history)

    testhub_cache = {
        "snapshots": testhub_snapshots,
    }
    countme_cache = countme_history
    countme_cache["os_version_dist"] = OS_VERSION_DIST

    write_json("src/data/testhub.json", testhub_out)
    write_json("src/data/countme.json", countme_out)
    write_json(".sync-cache/testhub-history.json", testhub_cache)
    write_json(".sync-cache/countme-history.json", countme_cache)

    print(f"\nSummary:")
    print(f"  testhub: {len(testhub_snapshots)} snapshots, {len(TESTHUB_APPS)} packages")
    print(f"  countme: {len(countme_history['week_records'])} week records, {len(countme_history['day_records'])} day records")


if __name__ == "__main__":
    main()
