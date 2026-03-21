package contributors

import (
	"reflect"
	"sort"
	"testing"
)

// ── IsBot ────────────────────────────────────────────────────────────────────

func TestIsBot(t *testing.T) {
	tests := []struct {
		login string
		want  bool
	}{
		{"renovate[bot]", true},
		{"github-actions[bot]", true},
		{"ubot-7274[bot]", true},
		{"[bot]", true},
		{"castrojo", false},
		{"jorge", false},
		{"bot", false},       // no bracket suffix
		{"[bot]user", false}, // suffix not at end
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.login, func(t *testing.T) {
			if got := IsBot(tt.login); got != tt.want {
				t.Errorf("IsBot(%q) = %v, want %v", tt.login, got, tt.want)
			}
		})
	}
}

// ── ComputeBusFactor ─────────────────────────────────────────────────────────

func TestComputeBusFactor(t *testing.T) {
	tests := []struct {
		name      string
		commits   map[string]int
		threshold float64
		want      int
	}{
		{
			name:      "empty map returns 1",
			commits:   map[string]int{},
			threshold: 0.8,
			want:      1,
		},
		{
			name:      "all-zero returns 1",
			commits:   map[string]int{"alice": 0, "bob": 0},
			threshold: 0.8,
			want:      1,
		},
		{
			name:      "single contributor owns everything",
			commits:   map[string]int{"alice": 100},
			threshold: 0.8,
			want:      1,
		},
		{
			name:      "two contributors threshold exactly met by first",
			commits:   map[string]int{"alice": 80, "bob": 20},
			threshold: 0.8,
			want:      1,
		},
		{
			name:      "threshold requires two contributors",
			commits:   map[string]int{"alice": 50, "bob": 40, "carol": 10},
			threshold: 0.8,
			want:      2,
		},
		{
			name:      "bots excluded from count and total ratio",
			commits:   map[string]int{"alice": 60, "renovate[bot]": 100, "bob": 40},
			threshold: 0.8,
			// bots excluded: human total = 100, alice=60 (60%), alice+bob=100 (100%) → need 2
			want: 2,
		},
		{
			name:      "all bots returns 0 (no human pairs)",
			commits:   map[string]int{"renovate[bot]": 50, "github-actions[bot]": 50},
			threshold: 0.8,
			want:      0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ComputeBusFactor(tt.commits, tt.threshold); got != tt.want {
				t.Errorf("ComputeBusFactor() = %d, want %d", got, tt.want)
			}
		})
	}
}

// ── ComputeNewContributors ───────────────────────────────────────────────────

func TestComputeNewContributors(t *testing.T) {
	tests := []struct {
		name       string
		current    []string
		historical map[string]bool
		want       []string
	}{
		{
			name:       "empty historical — all current are new (non-bot)",
			current:    []string{"alice", "bob", "renovate[bot]"},
			historical: map[string]bool{},
			want:       []string{"alice", "bob"},
		},
		{
			name:       "already seen — excluded",
			current:    []string{"alice", "bob"},
			historical: map[string]bool{"alice": true},
			want:       []string{"bob"},
		},
		{
			name:       "bots excluded even if not in historical",
			current:    []string{"renovate[bot]", "github-actions[bot]"},
			historical: map[string]bool{},
			want:       nil,
		},
		{
			name:       "all seen — returns nil",
			current:    []string{"alice", "bob"},
			historical: map[string]bool{"alice": true, "bob": true},
			want:       nil,
		},
		{
			name:       "empty current — returns nil",
			current:    []string{},
			historical: map[string]bool{"alice": true},
			want:       nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeNewContributors(tt.current, tt.historical)
			// Sort both for deterministic comparison.
			sort.Strings(got)
			want := tt.want
			sort.Strings(want)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("ComputeNewContributors() = %v, want %v", got, want)
			}
		})
	}
}

// ── ComputeReviewParticipationRate ───────────────────────────────────────────

func TestComputeReviewParticipationRate(t *testing.T) {
	tests := []struct {
		name             string
		mergedWithReview int
		totalMerged      int
		want             float64
	}{
		{"zero total returns 0", 0, 0, 0},
		{"all reviewed", 10, 10, 1.0},
		{"half reviewed", 5, 10, 0.5},
		{"none reviewed", 0, 10, 0.0},
		{"cap at 1.0 when over", 11, 10, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeReviewParticipationRate(tt.mergedWithReview, tt.totalMerged)
			if got != tt.want {
				t.Errorf("ComputeReviewParticipationRate(%d, %d) = %v, want %v",
					tt.mergedWithReview, tt.totalMerged, got, tt.want)
			}
		})
	}
}

// ── ComputeCrossRepoContributors ─────────────────────────────────────────────

func TestComputeCrossRepoContributors(t *testing.T) {
	tests := []struct {
		name        string
		repoAuthors map[string]map[string]bool
		minRepos    int
		want        int
	}{
		{
			name: "minRepos=2 — alice appears in 2 repos, bob in 1",
			repoAuthors: map[string]map[string]bool{
				"repo-a": {"alice": true, "bob": true},
				"repo-b": {"alice": true},
			},
			minRepos: 2,
			want:     1, // only alice
		},
		{
			name: "bots excluded",
			repoAuthors: map[string]map[string]bool{
				"repo-a": {"renovate[bot]": true, "alice": true},
				"repo-b": {"renovate[bot]": true, "alice": true},
			},
			minRepos: 2,
			want:     1, // only alice; renovate[bot] excluded
		},
		{
			name: "empty repos — returns 0",
			repoAuthors: map[string]map[string]bool{},
			minRepos: 1,
			want:     0,
		},
		{
			name: "minRepos=1 — all unique human authors count",
			repoAuthors: map[string]map[string]bool{
				"repo-a": {"alice": true},
				"repo-b": {"bob": true},
			},
			minRepos: 1,
			want:     2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeCrossRepoContributors(tt.repoAuthors, tt.minRepos)
			if got != tt.want {
				t.Errorf("ComputeCrossRepoContributors() = %d, want %d", got, tt.want)
			}
		})
	}
}

// ── ComputeActiveWeeksStreak ─────────────────────────────────────────────────

func TestComputeActiveWeeksStreak(t *testing.T) {
	tests := []struct {
		name   string
		weekly []int
		want   int
	}{
		{"all zeros — streak 0", []int{0, 0, 0, 0}, 0},
		{"all non-zero — full length", []int{1, 2, 3, 4}, 4},
		{"trailing zeros break streak", []int{1, 2, 0, 3, 4}, 2},
		{"single trailing zero", []int{5, 3, 1, 0}, 0},
		{"single non-zero at end", []int{0, 0, 0, 1}, 1},
		{"empty slice — streak 0", []int{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeActiveWeeksStreak(tt.weekly)
			if got != tt.want {
				t.Errorf("ComputeActiveWeeksStreak(%v) = %d, want %d", tt.weekly, got, tt.want)
			}
		})
	}
}
