package contributors

import "strings"

// IsBot returns true if the GitHub login belongs to a bot account.
// GitHub bots use the "[bot]" suffix (e.g., "renovate[bot]", "ubot-7274[bot]").
func IsBot(login string) bool {
	return strings.HasSuffix(login, "[bot]")
}

// ComputeBusFactor returns how many top contributors hold >= threshold (e.g., 0.8)
// of total commits. authorCommits maps login → commit count.
// Returns 1 if empty or all-zero to avoid divide-by-zero.
func ComputeBusFactor(authorCommits map[string]int, threshold float64) int {
	if len(authorCommits) == 0 {
		return 1
	}
	total := 0
	for _, c := range authorCommits {
		total += c
	}
	if total == 0 {
		return 1
	}

	// Build sorted slice (descending by commits), excluding bots.
	type pair struct {
		login string
		count int
	}
	pairs := make([]pair, 0, len(authorCommits))
	for login, count := range authorCommits {
		if !IsBot(login) {
			pairs = append(pairs, pair{login, count})
		}
	}
	// Simple insertion sort (small N)
	for i := 1; i < len(pairs); i++ {
		for j := i; j > 0 && pairs[j].count > pairs[j-1].count; j-- {
			pairs[j], pairs[j-1] = pairs[j-1], pairs[j]
		}
	}

	cumulative := 0
	for i, p := range pairs {
		cumulative += p.count
		if float64(cumulative)/float64(total) >= threshold {
			return i + 1
		}
	}
	return len(pairs)
}

// ComputeNewContributors returns logins that appear in current but NOT in historical.
// historical is the set of logins seen in prior periods.
func ComputeNewContributors(current []string, historical map[string]bool) []string {
	var newOnes []string
	for _, login := range current {
		if !historical[login] && !IsBot(login) {
			newOnes = append(newOnes, login)
		}
	}
	return newOnes
}

// ComputeReviewParticipationRate returns the fraction of merged PRs that had
// at least one reviewer (requested or review_comments > 0).
// Returns 0 if totalMerged == 0.
func ComputeReviewParticipationRate(mergedWithReview, totalMerged int) float64 {
	if totalMerged == 0 {
		return 0
	}
	rate := float64(mergedWithReview) / float64(totalMerged)
	if rate > 1.0 {
		rate = 1.0
	}
	return rate
}

// ComputeCrossRepoContributors counts logins that appear as authors in >= minRepos repos.
// repoAuthors maps repo name → set of author logins.
func ComputeCrossRepoContributors(repoAuthors map[string]map[string]bool, minRepos int) int {
	loginRepoCount := make(map[string]int)
	for _, authors := range repoAuthors {
		for login := range authors {
			if !IsBot(login) {
				loginRepoCount[login]++
			}
		}
	}
	count := 0
	for _, repoCount := range loginRepoCount {
		if repoCount >= minRepos {
			count++
		}
	}
	return count
}

// ComputeActiveWeeksStreak returns the number of consecutive non-zero weeks
// from the most recent week backward, given 52 weekly commit counts (oldest first).
func ComputeActiveWeeksStreak(weeklyCommits []int) int {
	streak := 0
	for i := len(weeklyCommits) - 1; i >= 0; i-- {
		if weeklyCommits[i] > 0 {
			streak++
		} else {
			break
		}
	}
	return streak
}
