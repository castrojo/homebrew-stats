package brewfile

import (
	"bufio"
	"strings"
)

// ParsedBrewfile holds the taps, brew formulas, and casks from a Brewfile.
type ParsedBrewfile struct {
	Taps  []string // e.g. "anomalyco/tap"
	Brews []string // e.g. "atuin", "anomalyco/tap/opencode"
	Casks []string // e.g. "font-jetbrains-mono-nerd-font", "ublue-os/tap/visual-studio-code-linux"
}

// Parse parses a Brewfile text and returns the taps, brews, and casks.
// Flatpak lines are skipped. Comment lines (# ...) are skipped.
// Token format: tap/brew/cask "value" (with optional trailing comma/options — just take the first quoted arg).
func Parse(content string) ParsedBrewfile {
	var result ParsedBrewfile
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip blank lines, comments, and flatpak lines.
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "flatpak") {
			continue
		}

		token, value, ok := parseDirective(line)
		if !ok {
			continue
		}

		switch token {
		case "tap":
			result.Taps = append(result.Taps, value)
		case "brew":
			result.Brews = append(result.Brews, value)
		case "cask":
			result.Casks = append(result.Casks, value)
		}
	}
	return result
}

// parseDirective extracts the directive keyword and first quoted argument from a Brewfile line.
// Returns (keyword, value, true) on success, ("", "", false) if the line can't be parsed.
func parseDirective(line string) (keyword, value string, ok bool) {
	// Split on whitespace to get the first token (tap, brew, cask, etc.)
	idx := strings.IndexByte(line, ' ')
	if idx < 0 {
		return "", "", false
	}
	keyword = line[:idx]

	// Extract the first double-quoted argument on the rest of the line.
	rest := line[idx:]
	first := strings.IndexByte(rest, '"')
	if first < 0 {
		return "", "", false
	}
	second := strings.IndexByte(rest[first+1:], '"')
	if second < 0 {
		return "", "", false
	}
	value = rest[first+1 : first+1+second]
	return keyword, value, true
}
