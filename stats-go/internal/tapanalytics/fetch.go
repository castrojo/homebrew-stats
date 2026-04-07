// Package tapanalytics fetches Homebrew cask-install analytics for ublue-os taps.
package tapanalytics

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const caskInstallBaseURL = "https://formulae.brew.sh/api/analytics/cask-install"
const formulaInstallBaseURL = "https://formulae.brew.sh/api/analytics/install"

// PkgInstalls holds Homebrew cask install counts across periods.
type PkgInstalls struct {
	Installs30d  int64 `json:"installs_30d"`
	Installs90d  int64 `json:"installs_90d"`
	Installs365d int64 `json:"installs_365d"`
}

// Fetch returns install counts for all ublue-os tap packages across 30d/90d/365d.
// Map key is the full cask token e.g. "ublue-os/tap/jetbrains-toolbox-linux".
func Fetch() (map[string]PkgInstalls, error) {
	result := make(map[string]PkgInstalls)

	periods := []struct {
		name string
		set  func(p *PkgInstalls, v int64)
	}{
		{"30d", func(p *PkgInstalls, v int64) { p.Installs30d = v }},
		{"90d", func(p *PkgInstalls, v int64) { p.Installs90d = v }},
		{"365d", func(p *PkgInstalls, v int64) { p.Installs365d = v }},
	}

	for _, period := range periods {
		url := fmt.Sprintf("%s/%s.json", caskInstallBaseURL, period.name)
		resp, err := http.Get(url) //nolint:gosec // URL is constructed from allowlisted base
		if err != nil {
			return nil, fmt.Errorf("fetching cask-install analytics (%s): %w", period.name, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("cask-install analytics (%s): HTTP %d", period.name, resp.StatusCode)
		}

		var payload struct {
			Items []struct {
				Cask  string `json:"cask"`
				Count string `json:"count"`
			} `json:"items"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return nil, fmt.Errorf("decoding cask-install analytics (%s): %w", period.name, err)
		}

		for _, item := range payload.Items {
			if !strings.HasPrefix(item.Cask, "ublue-os/") {
				continue
			}
			countStr := strings.ReplaceAll(item.Count, ",", "")
			count, _ := strconv.ParseInt(countStr, 10, 64)
			entry := result[item.Cask]
			period.set(&entry, count)
			result[item.Cask] = entry
		}
	}

	return result, nil
}

// fetchCaskPeriods fetches all cask-install analytics from baseURL across 30d/90d/365d.
// No prefix filter — returns every entry from the API.
func fetchCaskPeriods(baseURL string) (map[string]PkgInstalls, error) {
	result := make(map[string]PkgInstalls)

	periods := []struct {
		name string
		set  func(p *PkgInstalls, v int64)
	}{
		{"30d", func(p *PkgInstalls, v int64) { p.Installs30d = v }},
		{"90d", func(p *PkgInstalls, v int64) { p.Installs90d = v }},
		{"365d", func(p *PkgInstalls, v int64) { p.Installs365d = v }},
	}

	for _, period := range periods {
		url := fmt.Sprintf("%s/%s.json", baseURL, period.name)
		resp, err := http.Get(url) //nolint:gosec // URL is constructed from allowlisted base
		if err != nil {
			return nil, fmt.Errorf("fetching cask-install analytics (%s): %w", period.name, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("cask-install analytics (%s): HTTP %d", period.name, resp.StatusCode)
		}

		var payload struct {
			Items []struct {
				Cask  string `json:"cask"`
				Count string `json:"count"`
			} `json:"items"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return nil, fmt.Errorf("decoding cask-install analytics (%s): %w", period.name, err)
		}

		for _, item := range payload.Items {
			countStr := strings.ReplaceAll(item.Count, ",", "")
			count, _ := strconv.ParseInt(countStr, 10, 64)
			entry := result[item.Cask]
			period.set(&entry, count)
			result[item.Cask] = entry
		}
	}

	return result, nil
}

// fetchFormulaPeriods fetches all formula-install analytics from baseURL across 30d/90d/365d.
// No prefix filter — returns every entry from the API.
func fetchFormulaPeriods(baseURL string) (map[string]PkgInstalls, error) {
	result := make(map[string]PkgInstalls)

	periods := []struct {
		name string
		set  func(p *PkgInstalls, v int64)
	}{
		{"30d", func(p *PkgInstalls, v int64) { p.Installs30d = v }},
		{"90d", func(p *PkgInstalls, v int64) { p.Installs90d = v }},
		{"365d", func(p *PkgInstalls, v int64) { p.Installs365d = v }},
	}

	for _, period := range periods {
		url := fmt.Sprintf("%s/%s.json", baseURL, period.name)
		resp, err := http.Get(url) //nolint:gosec // URL is constructed from allowlisted base
		if err != nil {
			return nil, fmt.Errorf("fetching formula-install analytics (%s): %w", period.name, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("formula-install analytics (%s): HTTP %d", period.name, resp.StatusCode)
		}

		var payload struct {
			Items []struct {
				Formula string `json:"formula"`
				Count   string `json:"count"`
			} `json:"items"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return nil, fmt.Errorf("decoding formula-install analytics (%s): %w", period.name, err)
		}

		for _, item := range payload.Items {
			countStr := strings.ReplaceAll(item.Count, ",", "")
			count, _ := strconv.ParseInt(countStr, 10, 64)
			entry := result[item.Formula]
			period.set(&entry, count)
			result[item.Formula] = entry
		}
	}

	return result, nil
}

// FetchAllCasks returns install counts for ALL cask packages across 30d/90d/365d.
// No prefix filtering — the full Homebrew cask analytics dataset is returned.
func FetchAllCasks() (map[string]PkgInstalls, error) {
	return fetchCaskPeriods(caskInstallBaseURL)
}

// FetchFormulas returns install counts for ALL formula packages across 30d/90d/365d.
// No prefix filtering — the full Homebrew formula analytics dataset is returned.
func FetchFormulas() (map[string]PkgInstalls, error) {
	return fetchFormulaPeriods(formulaInstallBaseURL)
}

// fetchAliasMap fetches the Homebrew formula list and returns a map of
// alias → canonical formula name (e.g. "nvim" → "neovim").
func fetchAliasMap() (map[string]string, error) {
	resp, err := http.Get("https://formulae.brew.sh/api/formula.json") //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("fetching formula list for aliases: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("formula list: HTTP %d", resp.StatusCode)
	}

	var formulas []struct {
		Name    string   `json:"name"`
		Aliases []string `json:"aliases"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&formulas); err != nil {
		return nil, fmt.Errorf("decoding formula list: %w", err)
	}

	aliases := make(map[string]string, len(formulas))
	for _, f := range formulas {
		for _, a := range f.Aliases {
			aliases[a] = f.Name
		}
	}
	return aliases, nil
}

// FetchFormulasWithAliases returns install counts for ALL formula packages across
// 30d/90d/365d, with alias names also indexed (e.g. "nvim" → same counts as "neovim").
// This allows Brewfile tokens that use aliases to resolve correctly.
func FetchFormulasWithAliases() (map[string]PkgInstalls, error) {
	installs, err := fetchFormulaPeriods(formulaInstallBaseURL)
	if err != nil {
		return nil, err
	}

	aliases, err := fetchAliasMap()
	if err != nil {
		// Non-fatal: return canonical-only map
		return installs, nil
	}

	for alias, canonical := range aliases {
		if data, ok := installs[canonical]; ok {
			installs[alias] = data
		}
	}
	return installs, nil
}
