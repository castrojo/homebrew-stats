// token_test.go pins the analytics token-linkage logic so that agents or
// refactors cannot silently break it.  The linkage was previously believed to
// be broken (installs always null), but code review confirmed it works
// correctly.  These tests are the regression guard.

package tap

import (
	"testing"

	"github.com/castrojo/homebrew-stats/internal/tapanalytics"
)

// ---------------------------------------------------------------------------
// tapPrefix construction
// ---------------------------------------------------------------------------

func TestBuildTapPrefix_StandardTap(t *testing.T) {
	// "homebrew-tap" → Homebrew strips "homebrew-" → shortname "tap"
	// Full prefix: "ublue-os/tap/"
	got := buildTapPrefix("ublue-os", "homebrew-tap")
	want := "ublue-os/tap/"
	if got != want {
		t.Errorf("buildTapPrefix(%q, %q) = %q, want %q", "ublue-os", "homebrew-tap", got, want)
	}
}

func TestBuildTapPrefix_ExperimentalTap(t *testing.T) {
	// "homebrew-experimental-tap" → shortname "experimental-tap"
	// Full prefix: "ublue-os/experimental-tap/"
	got := buildTapPrefix("ublue-os", "homebrew-experimental-tap")
	want := "ublue-os/experimental-tap/"
	if got != want {
		t.Errorf("buildTapPrefix(%q, %q) = %q, want %q", "ublue-os", "homebrew-experimental-tap", got, want)
	}
}

func TestBuildTapPrefix_NonHomebrewPrefixedRepo(t *testing.T) {
	// Repos that don't start with "homebrew-" are left intact (defensive).
	got := buildTapPrefix("someorg", "my-tap")
	want := "someorg/my-tap/"
	if got != want {
		t.Errorf("buildTapPrefix(%q, %q) = %q, want %q", "someorg", "my-tap", got, want)
	}
}

// ---------------------------------------------------------------------------
// Package lookup-key assembly  (tapPrefix + p.Name)
// ---------------------------------------------------------------------------

func TestLookupKeyAssembly(t *testing.T) {
	// A cask named "goose-linux" in the ublue-os/homebrew-tap must produce the
	// exact key that the Homebrew analytics API uses: "ublue-os/tap/goose-linux".
	prefix := buildTapPrefix("ublue-os", "homebrew-tap")
	pkgName := "goose-linux"
	got := prefix + pkgName
	want := "ublue-os/tap/goose-linux"
	if got != want {
		t.Errorf("lookup key = %q, want %q", got, want)
	}
}

func TestLookupKeyAssembly_ExperimentalTap(t *testing.T) {
	prefix := buildTapPrefix("ublue-os", "homebrew-experimental-tap")
	pkgName := "aurora-dx-gnome"
	got := prefix + pkgName
	want := "ublue-os/experimental-tap/aurora-dx-gnome"
	if got != want {
		t.Errorf("lookup key = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// Integration: applyDownloads uses the right keys
// ---------------------------------------------------------------------------

func TestApplyDownloads_TokenLinkageEndToEnd(t *testing.T) {
	// Simulate the exact flow in Collect():
	//   1. Build the prefix from owner+repo.
	//   2. Call applyDownloads with a map keyed by full token.
	//   3. Confirm the cask receives the expected counts.
	prefix := buildTapPrefix("ublue-os", "homebrew-tap")

	pkgs := []Package{
		{Name: "goose-linux", Type: "cask"},
	}
	installs := map[string]tapanalytics.PkgInstalls{
		"ublue-os/tap/goose-linux": {Installs30d: 42, Installs90d: 110, Installs365d: 400},
	}
	applyDownloads(pkgs, prefix, installs)

	p := pkgs[0]
	if p.Downloads != 42 {
		t.Errorf("Downloads (30d) = %d, want 42", p.Downloads)
	}
	if p.Installs90d != 110 {
		t.Errorf("Installs90d = %d, want 110", p.Installs90d)
	}
	if p.Installs365d != 400 {
		t.Errorf("Installs365d = %d, want 400", p.Installs365d)
	}
}

// ---------------------------------------------------------------------------
// Formula type guard — cask analytics must never be applied to formulas
// ---------------------------------------------------------------------------

func TestApplyDownloads_FormulaTypeGuard(t *testing.T) {
	// Even when the analytics map contains a key that would match a formula's
	// name, the formula must never receive download data.  This prevents stale
	// analytics bleed-over when a cask is converted to a formula.
	prefix := buildTapPrefix("ublue-os", "homebrew-tap")

	pkgs := []Package{
		{Name: "some-tool", Type: "formula"}, // was once a cask, now a formula
	}
	installs := map[string]tapanalytics.PkgInstalls{
		"ublue-os/tap/some-tool": {Installs30d: 999, Installs90d: 999, Installs365d: 999},
	}
	applyDownloads(pkgs, prefix, installs)

	p := pkgs[0]
	if p.Downloads != 0 || p.Installs90d != 0 || p.Installs365d != 0 {
		t.Errorf("formula must receive zero analytics data, got Downloads=%d Installs90d=%d Installs365d=%d",
			p.Downloads, p.Installs90d, p.Installs365d)
	}
}

// ---------------------------------------------------------------------------
// Missing analytics — zero value is the correct sentinel
// ---------------------------------------------------------------------------

func TestApplyDownloads_MissingEntry_ZeroValue(t *testing.T) {
	// A cask with no entry in the analytics map must have all counters at zero.
	// Zero is the defined "not reported" sentinel for this struct — the field
	// is never negative, so zero unambiguously means "no data".
	prefix := buildTapPrefix("ublue-os", "homebrew-tap")

	pkgs := []Package{
		{Name: "obscure-cask", Type: "cask"},
	}
	// Empty analytics map — nothing to match.
	applyDownloads(pkgs, prefix, map[string]tapanalytics.PkgInstalls{})

	p := pkgs[0]
	if p.Downloads != 0 || p.Installs90d != 0 || p.Installs365d != 0 {
		t.Errorf("cask with no analytics entry should have all-zero counts, got %+v", p)
	}
}

func TestApplyDownloads_WrongPrefix_ZeroValue(t *testing.T) {
	// If the analytics data was fetched under a different prefix (e.g., wrong
	// tap name), the lookup must silently produce zero rather than matching
	// an unrelated entry.
	prefix := buildTapPrefix("ublue-os", "homebrew-experimental-tap") // experimental-tap prefix

	pkgs := []Package{
		{Name: "goose-linux", Type: "cask"},
	}
	// Analytics keyed under the standard tap, not the experimental tap.
	installs := map[string]tapanalytics.PkgInstalls{
		"ublue-os/tap/goose-linux": {Installs30d: 500},
	}
	applyDownloads(pkgs, prefix, installs)

	p := pkgs[0]
	if p.Downloads != 0 {
		t.Errorf("wrong prefix must not match: Downloads = %d, want 0", p.Downloads)
	}
}
