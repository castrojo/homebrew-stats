package brewfile

import (
	"fmt"

	ghclient "github.com/castrojo/bootc-ecosystem/internal/github"
)

// BrewfileSource describes a Brewfile in a GitHub repo.
type BrewfileSource struct {
	Owner string
	Repo  string
	Path  string
	Label string // human label e.g. "bluefin/cli"
}

// Fetch fetches and parses a Brewfile from GitHub.
// Returns the parsed result and any fetch error.
func Fetch(src BrewfileSource) (ParsedBrewfile, error) {
	content, err := ghclient.GetFileContent(src.Owner, src.Repo, src.Path)
	if err != nil {
		return ParsedBrewfile{}, fmt.Errorf("fetch brewfile %s/%s/%s: %w", src.Owner, src.Repo, src.Path, err)
	}
	return Parse(content), nil
}

// AllSources returns all known Brewfile sources for Bluefin and Bazzite.
// Aurora is excluded (all their Brewfiles are flatpak-only).
func AllSources() []BrewfileSource {
	return []BrewfileSource{
		// Bluefin (projectbluefin/common) — shared Brewfiles live under system_files/shared/
		{Owner: "projectbluefin", Repo: "common", Path: "system_files/shared/usr/share/ublue-os/homebrew/ai-tools.Brewfile", Label: "bluefin/ai-tools"},
		{Owner: "projectbluefin", Repo: "common", Path: "system_files/shared/usr/share/ublue-os/homebrew/cli.Brewfile", Label: "bluefin/cli"},
		{Owner: "projectbluefin", Repo: "common", Path: "system_files/shared/usr/share/ublue-os/homebrew/cncf.Brewfile", Label: "bluefin/cncf"},
		{Owner: "projectbluefin", Repo: "common", Path: "system_files/shared/usr/share/ublue-os/homebrew/ide.Brewfile", Label: "bluefin/ide"},
		{Owner: "projectbluefin", Repo: "common", Path: "system_files/shared/usr/share/ublue-os/homebrew/k8s-tools.Brewfile", Label: "bluefin/k8s-tools"},
		{Owner: "projectbluefin", Repo: "common", Path: "system_files/shared/usr/share/ublue-os/homebrew/artwork.Brewfile", Label: "bluefin/artwork"},
		{Owner: "projectbluefin", Repo: "common", Path: "system_files/shared/usr/share/ublue-os/homebrew/fonts.Brewfile", Label: "bluefin/fonts"},
		{Owner: "projectbluefin", Repo: "common", Path: "system_files/shared/usr/share/ublue-os/homebrew/swift.Brewfile", Label: "bluefin/swift"},
		{Owner: "projectbluefin", Repo: "common", Path: "system_files/shared/usr/share/ublue-os/homebrew/experimental-ide.Brewfile", Label: "bluefin/experimental-ide"},
		// Bazzite (ublue-os/bazzite)
		{Owner: "ublue-os", Repo: "bazzite", Path: "system_files/overrides/usr/share/ublue-os/homebrew/bazzite-cli.Brewfile", Label: "bazzite/bazzite-cli"},
	}
}
