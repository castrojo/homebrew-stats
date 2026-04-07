package brewfile

import (
	"testing"
)

func TestParse_TapLine(t *testing.T) {
	got := Parse(`tap "anomalyco/tap"`)
	if len(got.Taps) != 1 || got.Taps[0] != "anomalyco/tap" {
		t.Errorf("Taps = %v, want [\"anomalyco/tap\"]", got.Taps)
	}
	if len(got.Brews) != 0 {
		t.Errorf("Brews should be empty, got %v", got.Brews)
	}
	if len(got.Casks) != 0 {
		t.Errorf("Casks should be empty, got %v", got.Casks)
	}
}

func TestParse_BrewSimple(t *testing.T) {
	got := Parse(`brew "atuin"`)
	if len(got.Brews) != 1 || got.Brews[0] != "atuin" {
		t.Errorf("Brews = %v, want [\"atuin\"]", got.Brews)
	}
}

func TestParse_BrewTapPrefixed(t *testing.T) {
	got := Parse(`brew "anomalyco/tap/opencode"`)
	if len(got.Brews) != 1 || got.Brews[0] != "anomalyco/tap/opencode" {
		t.Errorf("Brews = %v, want [\"anomalyco/tap/opencode\"]", got.Brews)
	}
}

func TestParse_CaskSimple(t *testing.T) {
	got := Parse(`cask "font-jetbrains-mono-nerd-font"`)
	if len(got.Casks) != 1 || got.Casks[0] != "font-jetbrains-mono-nerd-font" {
		t.Errorf("Casks = %v, want [\"font-jetbrains-mono-nerd-font\"]", got.Casks)
	}
}

func TestParse_FlatpakSkipped(t *testing.T) {
	got := Parse(`flatpak "org.foo.Bar"`)
	if len(got.Taps) != 0 || len(got.Brews) != 0 || len(got.Casks) != 0 {
		t.Errorf("flatpak line should produce nothing; got taps=%v brews=%v casks=%v", got.Taps, got.Brews, got.Casks)
	}
}

func TestParse_CommentSkipped(t *testing.T) {
	got := Parse(`# this is a comment`)
	if len(got.Taps) != 0 || len(got.Brews) != 0 || len(got.Casks) != 0 {
		t.Errorf("comment line should produce nothing; got taps=%v brews=%v casks=%v", got.Taps, got.Brews, got.Casks)
	}
}

func TestParse_BlankLineSkipped(t *testing.T) {
	got := Parse("\n\n   \n")
	if len(got.Taps) != 0 || len(got.Brews) != 0 || len(got.Casks) != 0 {
		t.Errorf("blank lines should produce nothing; got taps=%v brews=%v casks=%v", got.Taps, got.Brews, got.Casks)
	}
}

func TestParse_MixedContent(t *testing.T) {
	input := `# Bluefin CLI Brewfile
tap "anomalyco/tap"
tap "homebrew/cask-fonts"

brew "atuin"
brew "anomalyco/tap/opencode"
cask "font-jetbrains-mono-nerd-font"
cask "ublue-os/tap/visual-studio-code-linux"
flatpak "org.gnome.Evince"
`
	got := Parse(input)

	wantTaps := []string{"anomalyco/tap", "homebrew/cask-fonts"}
	if len(got.Taps) != len(wantTaps) {
		t.Fatalf("Taps = %v, want %v", got.Taps, wantTaps)
	}
	for i, v := range wantTaps {
		if got.Taps[i] != v {
			t.Errorf("Taps[%d] = %q, want %q", i, got.Taps[i], v)
		}
	}

	wantBrews := []string{"atuin", "anomalyco/tap/opencode"}
	if len(got.Brews) != len(wantBrews) {
		t.Fatalf("Brews = %v, want %v", got.Brews, wantBrews)
	}
	for i, v := range wantBrews {
		if got.Brews[i] != v {
			t.Errorf("Brews[%d] = %q, want %q", i, got.Brews[i], v)
		}
	}

	wantCasks := []string{"font-jetbrains-mono-nerd-font", "ublue-os/tap/visual-studio-code-linux"}
	if len(got.Casks) != len(wantCasks) {
		t.Fatalf("Casks = %v, want %v", got.Casks, wantCasks)
	}
	for i, v := range wantCasks {
		if got.Casks[i] != v {
			t.Errorf("Casks[%d] = %q, want %q", i, got.Casks[i], v)
		}
	}
}

func TestParse_TrailingOptions(t *testing.T) {
	got := Parse(`brew "pkg", restart_service: true`)
	if len(got.Brews) != 1 || got.Brews[0] != "pkg" {
		t.Errorf("Brews = %v, want [\"pkg\"]", got.Brews)
	}
}
