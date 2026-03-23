package metrics_test

import (
"fmt"
"testing"

"github.com/castrojo/homebrew-stats/internal/history"
"github.com/castrojo/homebrew-stats/internal/metrics"
"github.com/castrojo/homebrew-stats/internal/tap"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

const testTap = "ublue-os/homebrew-tap"
const testPkg = "goose-linux"

// makeSnaps builds a slice of DaySnapshot with sequential dates and the given
// download values for testTap/testPkg.
func makeSnaps(downloads ...int64) []history.DaySnapshot {
snaps := make([]history.DaySnapshot, len(downloads))
for i, dl := range downloads {
snaps[i] = history.DaySnapshot{
Date: fmt.Sprintf("2026-01-%02d", i+1),
Taps: map[string]history.TapSnapshot{
testTap: {Downloads: map[string]int64{testPkg: dl}},
},
}
}
return snaps
}

// makeSnapsSummed builds snapshots where the tap has multiple packages,
// allowing sum-based logic to be exercised.
func makeSnapsSummed(pkgDownloads ...map[string]int64) []history.DaySnapshot {
snaps := make([]history.DaySnapshot, len(pkgDownloads))
for i, dl := range pkgDownloads {
snaps[i] = history.DaySnapshot{
Date: fmt.Sprintf("2026-01-%02d", i+1),
Taps: map[string]history.TapSnapshot{
testTap: {Downloads: dl},
},
}
}
return snaps
}

// ---------------------------------------------------------------------------
// Velocity7d
// ---------------------------------------------------------------------------

func TestVelocity7d_NormalCase(t *testing.T) {
// 8 snapshots: downloads grow from 0 to 70 (delta=70 over 7 steps → 10.0/day).
snaps := makeSnaps(0, 10, 20, 30, 40, 50, 60, 70)
got := metrics.Velocity7d(snaps, testTap, testPkg)
if got != 10.0 {
t.Errorf("Velocity7d = %v, want 10.0", got)
}
}

func TestVelocity7d_MoreThanEightSnapshots(t *testing.T) {
// 10 snapshots; velocity uses vals[n] vs vals[n-7].
// vals: 0,10,20,30,40,50,60,70,80,90 → (90-20)/7 = 10.0
snaps := makeSnaps(0, 10, 20, 30, 40, 50, 60, 70, 80, 90)
got := metrics.Velocity7d(snaps, testTap, testPkg)
if got != 10.0 {
t.Errorf("Velocity7d = %v, want 10.0", got)
}
}

func TestVelocity7d_FewerThanEightSnapshots_ReturnsZero(t *testing.T) {
// 7 qualifying snapshots — one short of the required minimum.
snaps := makeSnaps(10, 20, 30, 40, 50, 60, 70)
got := metrics.Velocity7d(snaps, testTap, testPkg)
if got != 0 {
t.Errorf("Velocity7d with 7 snapshots = %v, want 0", got)
}
}

func TestVelocity7d_NegativeDelta_ClampedToZero(t *testing.T) {
// Rolling-window roll-off: 30d total drops when old installs age out.
snaps := makeSnaps(200, 190, 180, 170, 160, 150, 140, 100)
got := metrics.Velocity7d(snaps, testTap, testPkg)
if got != 0 {
t.Errorf("Velocity7d with negative delta = %v, want 0 (clamped)", got)
}
}

func TestVelocity7d_RoundingToOneDecimal(t *testing.T) {
// delta=10 over 7 → 10/7 ≈ 1.428… → rounds to 1.4
snaps := makeSnaps(0, 0, 0, 0, 0, 0, 0, 10)
got := metrics.Velocity7d(snaps, testTap, testPkg)
if got != 1.4 {
t.Errorf("Velocity7d = %v, want 1.4", got)
}
}

func TestVelocity7d_PackageMissingFromSomeSnapshots(t *testing.T) {
// 4 snapshots have no data for the package; 8 qualifying snapshots follow.
// The 4 early snapshots should be skipped, and velocity computed on the 8 good ones.
snaps := []history.DaySnapshot{
// No pkg data in these 4:
{Date: "2026-01-01", Taps: map[string]history.TapSnapshot{testTap: {Downloads: map[string]int64{}}}},
{Date: "2026-01-02", Taps: map[string]history.TapSnapshot{testTap: {Downloads: map[string]int64{}}}},
{Date: "2026-01-03", Taps: map[string]history.TapSnapshot{testTap: {Downloads: map[string]int64{}}}},
{Date: "2026-01-04", Taps: map[string]history.TapSnapshot{testTap: {Downloads: map[string]int64{}}}},
// 8 qualifying snapshots (vals 10..80):
{Date: "2026-01-05", Taps: map[string]history.TapSnapshot{testTap: {Downloads: map[string]int64{testPkg: 10}}}},
{Date: "2026-01-06", Taps: map[string]history.TapSnapshot{testTap: {Downloads: map[string]int64{testPkg: 20}}}},
{Date: "2026-01-07", Taps: map[string]history.TapSnapshot{testTap: {Downloads: map[string]int64{testPkg: 30}}}},
{Date: "2026-01-08", Taps: map[string]history.TapSnapshot{testTap: {Downloads: map[string]int64{testPkg: 40}}}},
{Date: "2026-01-09", Taps: map[string]history.TapSnapshot{testTap: {Downloads: map[string]int64{testPkg: 50}}}},
{Date: "2026-01-10", Taps: map[string]history.TapSnapshot{testTap: {Downloads: map[string]int64{testPkg: 60}}}},
{Date: "2026-01-11", Taps: map[string]history.TapSnapshot{testTap: {Downloads: map[string]int64{testPkg: 70}}}},
{Date: "2026-01-12", Taps: map[string]history.TapSnapshot{testTap: {Downloads: map[string]int64{testPkg: 80}}}},
}
// vals = [10,20,30,40,50,60,70,80]; (80-10)/7 = 10.0
got := metrics.Velocity7d(snaps, testTap, testPkg)
if got != 10.0 {
t.Errorf("Velocity7d = %v, want 10.0", got)
}
}

func TestVelocity7d_UnknownTap_ReturnsZero(t *testing.T) {
snaps := makeSnaps(10, 20, 30, 40, 50, 60, 70, 80)
got := metrics.Velocity7d(snaps, "nonexistent/tap", testPkg)
if got != 0 {
t.Errorf("Velocity7d for unknown tap = %v, want 0", got)
}
}

func TestVelocity7d_NilHistory_ReturnsZero(t *testing.T) {
got := metrics.Velocity7d(nil, testTap, testPkg)
if got != 0 {
t.Errorf("Velocity7d with nil history = %v, want 0", got)
}
}

// ---------------------------------------------------------------------------
// GrowthPct
// ---------------------------------------------------------------------------

func TestGrowthPct_NormalCase(t *testing.T) {
// 8 snapshots; snap[n-7] sum=100, snap[n] sum=200 → 100.0%
snaps := makeSnaps(100, 110, 120, 130, 140, 150, 160, 200)
got := metrics.GrowthPct(snaps, testTap)
if got == nil {
t.Fatal("GrowthPct returned nil, want non-nil")
}
if *got != 100.0 {
t.Errorf("GrowthPct = %v, want 100.0", *got)
}
}

func TestGrowthPct_FewerThanEightSnapshots_ReturnsNil(t *testing.T) {
snaps := makeSnaps(10, 20, 30, 40, 50, 60, 70)
got := metrics.GrowthPct(snaps, testTap)
if got != nil {
t.Errorf("GrowthPct with 7 snapshots = %v, want nil", *got)
}
}

func TestGrowthPct_ZeroDenominator_ReturnsNil(t *testing.T) {
// snap[n-7] sum = 0 → division-by-zero guard → nil
snaps := makeSnaps(0, 0, 0, 0, 0, 0, 0, 50)
got := metrics.GrowthPct(snaps, testTap)
if got != nil {
t.Errorf("GrowthPct with zero baseline = %v, want nil", *got)
}
}

func TestGrowthPct_NegativeGrowth(t *testing.T) {
// downloads decrease from 200 to 100 → -50%
snaps := makeSnaps(200, 190, 180, 170, 160, 150, 140, 100)
got := metrics.GrowthPct(snaps, testTap)
if got == nil {
t.Fatal("GrowthPct returned nil, want non-nil")
}
if *got != -50.0 {
t.Errorf("GrowthPct = %v, want -50.0", *got)
}
}

func TestGrowthPct_UnknownTap_ReturnsNil(t *testing.T) {
snaps := makeSnaps(10, 20, 30, 40, 50, 60, 70, 80)
got := metrics.GrowthPct(snaps, "nonexistent/tap")
if got != nil {
t.Errorf("GrowthPct for unknown tap = %v, want nil", *got)
}
}

func TestGrowthPct_MultiPackageSums(t *testing.T) {
// Two packages per snapshot; sums: 20,30,40,50,60,70,80,100
// snap[n-7]=20, snap[n]=100 → (100-20)/20*100 = 400%
snaps := makeSnapsSummed(
map[string]int64{"a": 10, "b": 10},
map[string]int64{"a": 15, "b": 15},
map[string]int64{"a": 20, "b": 20},
map[string]int64{"a": 25, "b": 25},
map[string]int64{"a": 30, "b": 30},
map[string]int64{"a": 35, "b": 35},
map[string]int64{"a": 40, "b": 40},
map[string]int64{"a": 50, "b": 50},
)
got := metrics.GrowthPct(snaps, testTap)
if got == nil {
t.Fatal("GrowthPct returned nil, want non-nil")
}
if *got != 400.0 {
t.Errorf("GrowthPct = %v, want 400.0", *got)
}
}

// ---------------------------------------------------------------------------
// ComputeSummary
// ---------------------------------------------------------------------------

func TestComputeSummary_BasicAggregation(t *testing.T) {
taps := []tap.TapStats{
{
Name:    testTap,
Traffic: &tap.Traffic{Uniques: 100},
Packages: []tap.Package{
{Name: "pkg-a", Type: "cask", Downloads: 50, Installs90d: 120, Installs365d: 400, FreshnessKnown: true, IsStale: false},
{Name: "pkg-b", Type: "cask", Downloads: 30, Installs90d: 80,  Installs365d: 250, FreshnessKnown: true, IsStale: true},
{Name: "pkg-c", Type: "cask", Downloads: 20, Installs90d: 60,  Installs365d: 200, FreshnessKnown: false},
},
},
{
Name:    "ublue-os/homebrew-experimental-tap",
Traffic: &tap.Traffic{Uniques: 50},
Packages: []tap.Package{
{Name: "pkg-d", Type: "cask", Downloads: 10, Installs90d: 30, Installs365d: 100, FreshnessKnown: true, IsStale: false},
},
},
}
s := metrics.ComputeSummary(taps, nil)

if s.TotalInstalls30d != 110 {
t.Errorf("TotalInstalls30d = %d, want 110", s.TotalInstalls30d)
}
if s.TotalInstalls90d != 290 {
t.Errorf("TotalInstalls90d = %d, want 290", s.TotalInstalls90d)
}
if s.TotalInstalls365d != 950 {
t.Errorf("TotalInstalls365d = %d, want 950", s.TotalInstalls365d)
}
if s.TotalUniqueTappers != 150 {
t.Errorf("TotalUniqueTappers = %d, want 150", s.TotalUniqueTappers)
}
if s.TotalPackages != 4 {
t.Errorf("TotalPackages = %d, want 4", s.TotalPackages)
}
if s.FreshCount != 2 {
t.Errorf("FreshCount = %d, want 2", s.FreshCount)
}
if s.StaleCount != 1 {
t.Errorf("StaleCount = %d, want 1", s.StaleCount)
}
if s.UnknownFreshnessCount != 1 {
t.Errorf("UnknownFreshnessCount = %d, want 1", s.UnknownFreshnessCount)
}
}

func TestComputeSummary_WoWGrowthPct_NilWhenInsufficientHistory(t *testing.T) {
taps := []tap.TapStats{{Name: testTap}}
snaps := makeSnaps(10, 20, 30) // only 3 snapshots
s := metrics.ComputeSummary(taps, snaps)
if s.WoWGrowthPct != nil {
t.Errorf("WoWGrowthPct = %v, want nil for < 8 snapshots", *s.WoWGrowthPct)
}
}

func TestComputeSummary_WoWGrowthPct_ComputedWhenEnoughHistory(t *testing.T) {
taps := []tap.TapStats{{Name: testTap}}
// snap[n-7]=100, snap[n]=200 → 100%
snaps := makeSnaps(100, 110, 120, 130, 140, 150, 160, 200)
s := metrics.ComputeSummary(taps, snaps)
if s.WoWGrowthPct == nil {
t.Fatal("WoWGrowthPct is nil, want non-nil for 8 snapshots")
}
if *s.WoWGrowthPct != 100.0 {
t.Errorf("WoWGrowthPct = %v, want 100.0", *s.WoWGrowthPct)
}
}

func TestComputeSummary_NilTrafficDoesNotPanic(t *testing.T) {
taps := []tap.TapStats{
{Name: testTap, Traffic: nil, Packages: []tap.Package{{Name: "p", Downloads: 5}}},
}
s := metrics.ComputeSummary(taps, nil)
if s.TotalInstalls30d != 5 {
t.Errorf("TotalInstalls30d = %d, want 5", s.TotalInstalls30d)
}
if s.TotalUniqueTappers != 0 {
t.Errorf("TotalUniqueTappers = %d, want 0 when Traffic is nil", s.TotalUniqueTappers)
}
}

// ---------------------------------------------------------------------------
// ComputeTopPackages
// ---------------------------------------------------------------------------

func makeTapStats(tapName string, pkgs ...tap.Package) tap.TapStats {
return tap.TapStats{Name: tapName, Packages: pkgs}
}

func pkg(name string, downloads int64) tap.Package {
return tap.Package{Name: name, Downloads: downloads}
}

func TestComputeTopPackages_SelectsTop10ByDownloads(t *testing.T) {
// 15 packages; only the top 10 should appear.
taps := []tap.TapStats{
makeTapStats(testTap,
pkg("p01", 1000), pkg("p02", 900), pkg("p03", 800), pkg("p04", 700), pkg("p05", 600),
pkg("p06", 500), pkg("p07", 400), pkg("p08", 300), pkg("p09", 200), pkg("p10", 100),
pkg("p11", 90), pkg("p12", 80), pkg("p13", 70), pkg("p14", 60), pkg("p15", 50),
),
}
top := metrics.ComputeTopPackages(taps, nil)
if len(top) != 10 {
t.Fatalf("ComputeTopPackages returned %d entries, want 10", len(top))
}
if top[0].Name != "p01" {
t.Errorf("top[0].Name = %q, want p01", top[0].Name)
}
for _, tp := range top {
if tp.Name == "p11" || tp.Name == "p12" {
t.Errorf("package %q should not be in top 10", tp.Name)
}
}
}

func TestComputeTopPackages_FewerThan10Packages_ReturnsAll(t *testing.T) {
taps := []tap.TapStats{
makeTapStats(testTap, pkg("a", 100), pkg("b", 50), pkg("c", 10)),
}
top := metrics.ComputeTopPackages(taps, nil)
if len(top) != 3 {
t.Fatalf("ComputeTopPackages returned %d entries, want 3", len(top))
}
}

func TestComputeTopPackages_CrossTapSelection(t *testing.T) {
// Top packages can come from multiple taps.
tap2 := "ublue-os/homebrew-experimental-tap"
taps := []tap.TapStats{
makeTapStats(testTap, pkg("alpha", 100), pkg("beta", 50)),
makeTapStats(tap2, pkg("gamma", 75)),
}
top := metrics.ComputeTopPackages(taps, nil)
if len(top) != 3 {
t.Fatalf("want 3 packages, got %d", len(top))
}
// alpha=100 is the global leader.
if top[0].Name != "alpha" || top[0].Tap != testTap {
t.Errorf("top[0] = {%q, %q}, want {alpha, %s}", top[0].Name, top[0].Tap, testTap)
}
}

func TestComputeTopPackages_HistoryAttachedCorrectly(t *testing.T) {
taps := []tap.TapStats{
makeTapStats(testTap, pkg(testPkg, 70)),
}
snaps := makeSnaps(10, 20, 30, 40, 50, 60, 70)
top := metrics.ComputeTopPackages(taps, snaps)
if len(top) != 1 {
t.Fatalf("want 1 package, got %d", len(top))
}
if len(top[0].History) != 7 {
t.Errorf("history length = %d, want 7", len(top[0].History))
}
if top[0].History[0].Downloads != 10 {
t.Errorf("History[0].Downloads = %d, want 10", top[0].History[0].Downloads)
}
if top[0].History[6].Downloads != 70 {
t.Errorf("History[6].Downloads = %d, want 70", top[0].History[6].Downloads)
}
}

func TestComputeTopPackages_EmptyHistory_ProducesEmptyHistorySlice(t *testing.T) {
taps := []tap.TapStats{
makeTapStats(testTap, pkg(testPkg, 100)),
}
top := metrics.ComputeTopPackages(taps, nil)
if len(top) != 1 {
t.Fatalf("want 1 package, got %d", len(top))
}
if len(top[0].History) != 0 {
t.Errorf("expected empty history, got %v", top[0].History)
}
}

func TestComputeTopPackages_HistoryOmitsSnapshotsMissingPackage(t *testing.T) {
// 5 snapshots; the package only appears in 3 of them.
snaps := []history.DaySnapshot{
{Date: "2026-01-01", Taps: map[string]history.TapSnapshot{testTap: {Downloads: map[string]int64{}}}},
{Date: "2026-01-02", Taps: map[string]history.TapSnapshot{testTap: {Downloads: map[string]int64{testPkg: 20}}}},
{Date: "2026-01-03", Taps: map[string]history.TapSnapshot{testTap: {Downloads: map[string]int64{}}}},
{Date: "2026-01-04", Taps: map[string]history.TapSnapshot{testTap: {Downloads: map[string]int64{testPkg: 40}}}},
{Date: "2026-01-05", Taps: map[string]history.TapSnapshot{testTap: {Downloads: map[string]int64{testPkg: 50}}}},
}
taps := []tap.TapStats{makeTapStats(testTap, pkg(testPkg, 50))}
top := metrics.ComputeTopPackages(taps, snaps)
if len(top[0].History) != 3 {
t.Errorf("expected 3 history points, got %d", len(top[0].History))
}
if top[0].History[0].Downloads != 20 {
t.Errorf("first history point = %d, want 20", top[0].History[0].Downloads)
}
}
