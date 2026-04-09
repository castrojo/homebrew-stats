package contributors

// White-box tests for the pure functions in collector.go.
// Functions that call ghcli.Run() or ghpkg.GraphQL() are excluded because they
// shell out to the `gh` CLI / hit live APIs — those require full integration
// harnesses outside the scope of unit tests.
//
// Covered here:
//   - FilterCommitsAfter
//   - FilterIssuesAfter
//   - FilterPRsAfter
//   - FilterDiscussionsAfter
//   - percentile (private)
//   - round1 (private)

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// FilterCommitsAfter
// ---------------------------------------------------------------------------

func TestFilterCommitsAfter_KeepsAfterCutoff(t *testing.T) {
	cutoff := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)
	commits := []CommitRecord{
		{Login: "alice", Date: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
		{Login: "bob", Date: time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC)}, // before cutoff
		{Login: "carol", Date: time.Date(2024, 1, 11, 0, 0, 0, 0, time.UTC)},
	}
	got := FilterCommitsAfter(commits, cutoff)
	if len(got) != 2 {
		t.Fatalf("expected 2 commits after cutoff, got %d", len(got))
	}
	for _, c := range got {
		if !c.Date.After(cutoff) {
			t.Errorf("commit from %v is not after cutoff %v", c.Date, cutoff)
		}
	}
}

func TestFilterCommitsAfter_ExactCutoffExcluded(t *testing.T) {
	// A commit exactly AT the cutoff is not "After" — must be excluded.
	cutoff := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)
	commits := []CommitRecord{
		{Login: "alice", Date: cutoff},
	}
	got := FilterCommitsAfter(commits, cutoff)
	if len(got) != 0 {
		t.Errorf("commit at exact cutoff should be excluded, got %d entries", len(got))
	}
}

func TestFilterCommitsAfter_EmptyInput(t *testing.T) {
	cutoff := time.Now()
	got := FilterCommitsAfter(nil, cutoff)
	if len(got) != 0 {
		t.Errorf("expected empty result for nil input, got %d", len(got))
	}
}

func TestFilterCommitsAfter_AllPass(t *testing.T) {
	cutoff := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	commits := []CommitRecord{
		{Login: "a", Date: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Login: "b", Date: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)},
	}
	got := FilterCommitsAfter(commits, cutoff)
	if len(got) != 2 {
		t.Errorf("expected all 2 commits to pass, got %d", len(got))
	}
}

func TestFilterCommitsAfter_NonePass(t *testing.T) {
	cutoff := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	commits := []CommitRecord{
		{Login: "a", Date: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
	}
	got := FilterCommitsAfter(commits, cutoff)
	if len(got) != 0 {
		t.Errorf("expected 0 commits after future cutoff, got %d", len(got))
	}
}

// ---------------------------------------------------------------------------
// FilterIssuesAfter
// ---------------------------------------------------------------------------

func TestFilterIssuesAfter_KeepsAfterCutoff(t *testing.T) {
	cutoff := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	issues := []IssueRecord{
		{Login: "alice", CreatedAt: time.Date(2024, 3, 5, 0, 0, 0, 0, time.UTC)},
		{Login: "bob", CreatedAt: time.Date(2024, 2, 28, 0, 0, 0, 0, time.UTC)}, // before
		{Login: "carol", CreatedAt: time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)},
	}
	got := FilterIssuesAfter(issues, cutoff)
	if len(got) != 2 {
		t.Fatalf("expected 2 issues after cutoff, got %d", len(got))
	}
}

func TestFilterIssuesAfter_Empty(t *testing.T) {
	got := FilterIssuesAfter(nil, time.Now())
	if len(got) != 0 {
		t.Errorf("expected empty, got %d", len(got))
	}
}

func TestFilterIssuesAfter_AllFiltered(t *testing.T) {
	cutoff := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	issues := []IssueRecord{
		{Login: "alice", CreatedAt: time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)},
	}
	got := FilterIssuesAfter(issues, cutoff)
	if len(got) != 0 {
		t.Errorf("expected 0, got %d", len(got))
	}
}

// ---------------------------------------------------------------------------
// FilterPRsAfter
// ---------------------------------------------------------------------------

func TestFilterPRsAfter_KeepsAfterCutoff(t *testing.T) {
	cutoff := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	prs := []PRRecord{
		{Login: "alice", MergedAt: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)},
		{Login: "bob", MergedAt: time.Date(2024, 5, 31, 0, 0, 0, 0, time.UTC)}, // before
	}
	got := FilterPRsAfter(prs, cutoff)
	if len(got) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(got))
	}
	if got[0].Login != "alice" {
		t.Errorf("expected alice, got %q", got[0].Login)
	}
}

func TestFilterPRsAfter_Empty(t *testing.T) {
	got := FilterPRsAfter(nil, time.Now())
	if len(got) != 0 {
		t.Errorf("expected empty, got %d", len(got))
	}
}

func TestFilterPRsAfter_HasReviewersPreserved(t *testing.T) {
	cutoff := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	prs := []PRRecord{
		{Login: "alice", MergedAt: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC), HasReviewers: true},
		{Login: "bob", MergedAt: time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC), HasReviewers: false},
	}
	got := FilterPRsAfter(prs, cutoff)
	if len(got) != 2 {
		t.Fatalf("expected 2 PRs, got %d", len(got))
	}
	if !got[0].HasReviewers {
		t.Error("HasReviewers should be true for first PR")
	}
	if got[1].HasReviewers {
		t.Error("HasReviewers should be false for second PR")
	}
}

// ---------------------------------------------------------------------------
// FilterDiscussionsAfter
// ---------------------------------------------------------------------------

func TestFilterDiscussionsAfter_KeepsAfterCutoff(t *testing.T) {
	cutoff := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
	discussions := []DiscussionRecord{
		{AuthorLogin: "alice", CreatedAt: time.Date(2024, 4, 5, 0, 0, 0, 0, time.UTC)},
		{AuthorLogin: "bob", CreatedAt: time.Date(2024, 3, 31, 0, 0, 0, 0, time.UTC)}, // before
		{AuthorLogin: "carol", CreatedAt: time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)},
	}
	got := FilterDiscussionsAfter(discussions, cutoff)
	if len(got) != 2 {
		t.Fatalf("expected 2 discussions, got %d", len(got))
	}
}

func TestFilterDiscussionsAfter_Empty(t *testing.T) {
	got := FilterDiscussionsAfter(nil, time.Now())
	if len(got) != 0 {
		t.Errorf("expected empty, got %d", len(got))
	}
}

func TestFilterDiscussionsAfter_CommentLoginsPreserved(t *testing.T) {
	cutoff := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	discussions := []DiscussionRecord{
		{
			AuthorLogin:   "alice",
			CreatedAt:     time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
			CommentLogins: []string{"bob", "carol"},
		},
	}
	got := FilterDiscussionsAfter(discussions, cutoff)
	if len(got) != 1 {
		t.Fatalf("expected 1 discussion, got %d", len(got))
	}
	if len(got[0].CommentLogins) != 2 {
		t.Errorf("expected 2 comment logins, got %d", len(got[0].CommentLogins))
	}
}

// ---------------------------------------------------------------------------
// percentile (private)
// ---------------------------------------------------------------------------

func TestPercentile_Empty(t *testing.T) {
	if got := percentile(nil, 50); got != 0 {
		t.Errorf("percentile(nil, 50) = %f, want 0", got)
	}
}

func TestPercentile_SingleValue(t *testing.T) {
	if got := percentile([]float64{42.0}, 50); got != 42.0 {
		t.Errorf("percentile([42], 50) = %f, want 42", got)
	}
}

func TestPercentile_P50(t *testing.T) {
	values := []float64{10, 20, 30, 40, 50}
	got := percentile(values, 50)
	// P50 of [10,20,30,40,50] = 30
	if got != 30.0 {
		t.Errorf("percentile P50 = %f, want 30.0", got)
	}
}

func TestPercentile_P100(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5}
	got := percentile(values, 100)
	if got != 5.0 {
		t.Errorf("percentile P100 = %f, want 5.0", got)
	}
}

func TestPercentile_P0(t *testing.T) {
	values := []float64{5, 10, 15}
	got := percentile(values, 0)
	if got != 5.0 {
		t.Errorf("percentile P0 = %f, want 5.0", got)
	}
}

func TestPercentile_Interpolation(t *testing.T) {
	// 2 values: [0, 100], P50 → midpoint = 50
	values := []float64{0, 100}
	got := percentile(values, 50)
	if got != 50.0 {
		t.Errorf("percentile interpolation = %f, want 50.0", got)
	}
}

// ---------------------------------------------------------------------------
// round1 (private)
// ---------------------------------------------------------------------------

func TestRound1_BasicCases(t *testing.T) {
	cases := []struct {
		input float64
		want  float64
	}{
		{1.25, 1.3},
		{1.24, 1.2},
		{0.0, 0.0},
		{10.05, 10.1},
		{-1.25, -1.3}, // math.Round rounds half away from zero: -12.5 → -13 → -1.3
		{100.0, 100.0},
	}
	for _, c := range cases {
		got := round1(c.input)
		if got != c.want {
			t.Errorf("round1(%f) = %f, want %f", c.input, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// TrackedRepos — structural validation
// ---------------------------------------------------------------------------

func TestTrackedRepos_NotEmpty(t *testing.T) {
	if len(TrackedRepos) == 0 {
		t.Fatal("TrackedRepos must not be empty")
	}
}

func TestTrackedRepos_AllHaveSlash(t *testing.T) {
	for _, repo := range TrackedRepos {
		hasSlash := false
		for _, r := range repo {
			if r == '/' {
				hasSlash = true
				break
			}
		}
		if !hasSlash {
			t.Errorf("TrackedRepos entry %q does not look like owner/repo (missing '/')", repo)
		}
	}
}

func TestTrackedRepos_ContainsExpected(t *testing.T) {
	required := []string{
		"ublue-os/bluefin",
		"ublue-os/bazzite",
		"ublue-os/aurora",
	}
	repoSet := make(map[string]bool, len(TrackedRepos))
	for _, r := range TrackedRepos {
		repoSet[r] = true
	}
	for _, r := range required {
		if !repoSet[r] {
			t.Errorf("TrackedRepos missing required entry %q", r)
		}
	}
}

// ---------------------------------------------------------------------------
// ProfileCacheTTL
// ---------------------------------------------------------------------------

func TestProfileCacheTTL_IsPositive(t *testing.T) {
	if ProfileCacheTTL <= 0 {
		t.Errorf("ProfileCacheTTL must be positive, got %v", ProfileCacheTTL)
	}
}

func TestProfileCacheTTL_IsAtLeastOneDay(t *testing.T) {
	oneDayish := 24 * time.Hour
	if ProfileCacheTTL < oneDayish {
		t.Errorf("ProfileCacheTTL = %v, expected at least 24h", ProfileCacheTTL)
	}
}
