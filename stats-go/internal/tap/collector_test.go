package tap

import "testing"

func TestNormaliseVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"v1.0.0", "1.0.0"},
		{"1.0.0", "1.0.0"},
		{"v1.0.0 ", "1.0.0"},
		{" v1.0.0", "1.0.0"},
		{"", ""},
		{"v0.1.0-beta", "0.1.0-beta"},
	}
	for _, tc := range tests {
		got := normaliseVersion(tc.input)
		if got != tc.want {
			t.Errorf("normaliseVersion(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestPackageStatusString(t *testing.T) {
	tests := []struct {
		name           string
		freshnessKnown bool
		isStale        bool
		want           string
	}{
		{"freshness unknown", false, false, "unknown"},
		{"freshness unknown even if stale flag set", false, true, "unknown"},
		{"current", true, false, "current"},
		{"stale", true, true, "stale"},
	}
	for _, tc := range tests {
		p := &Package{FreshnessKnown: tc.freshnessKnown, IsStale: tc.isStale}
		got := p.StatusString()
		if got != tc.want {
			t.Errorf("[%s] StatusString() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestParseRuby(t *testing.T) {
	t.Run("cask whose URL points to an external repo — no tap download", func(t *testing.T) {
		// framework-tool pattern: upstream release, not hosted by the tap.
		content := `
cask "framework-tool" do
  version "0.6.1"
  desc "Multi-tool for Framework laptops"
  homepage "https://github.com/FrameworkComputer/framework-system"
  url "https://github.com/FrameworkComputer/framework-system/releases/download/v#{version}/framework-tool.AppImage"
end
`
		p := parseRuby("framework-tool", "cask", content)
		if p.SourceOwner != "FrameworkComputer" {
			t.Errorf("SourceOwner = %q, want %q", p.SourceOwner, "FrameworkComputer")
		}
		if p.SourceRepo != "framework-system" {
			t.Errorf("SourceRepo = %q, want %q", p.SourceRepo, "framework-system")
		}
		// DownloadOwner/Repo must be empty — downloads should be 0.
		if p.DownloadOwner != "" {
			t.Errorf("DownloadOwner = %q, want empty (not tap-hosted)", p.DownloadOwner)
		}
		if p.DownloadRepo != "" {
			t.Errorf("DownloadRepo = %q, want empty (not tap-hosted)", p.DownloadRepo)
		}
	})

	t.Run("cask whose URL points to the ublue-os tap — downloads from tap", func(t *testing.T) {
		// wallpaper-style cask: asset is published to the tap's own releases.
		content := `
cask "aurora-wallpapers" do
  version "1.0.0"
  desc "Aurora wallpaper collection"
  homepage "https://github.com/ublue-os/artwork"
  url "https://github.com/ublue-os/homebrew-tap/releases/download/v#{version}/aurora-wallpapers.tar.gz"
end
`
		p := parseRuby("aurora-wallpapers", "cask", content)
		// SourceOwner should come from the non-tap URL (artwork homepage).
		if p.SourceOwner != "ublue-os" || p.SourceRepo != "artwork" {
			t.Errorf("SourceOwner/SourceRepo = %q/%q, want ublue-os/artwork", p.SourceOwner, p.SourceRepo)
		}
		// DownloadOwner/Repo must point to the tap repo.
		if p.DownloadOwner != "ublue-os" {
			t.Errorf("DownloadOwner = %q, want %q", p.DownloadOwner, "ublue-os")
		}
		if p.DownloadRepo != "homebrew-tap" {
			t.Errorf("DownloadRepo = %q, want %q", p.DownloadRepo, "homebrew-tap")
		}
	})

	t.Run("cask from experimental-tap gets download fields set to experimental-tap", func(t *testing.T) {
		content := `
cask "cool-tool" do
  version "2.0.0"
  desc "A cool tool"
  homepage "https://github.com/some-org/cool-tool"
  url "https://github.com/ublue-os/homebrew-experimental-tap/releases/download/v2.0.0/cool-tool.tar.gz"
end
`
		p := parseRuby("cool-tool", "cask", content)
		if p.SourceOwner != "some-org" {
			t.Errorf("SourceOwner = %q, want %q", p.SourceOwner, "some-org")
		}
		if p.DownloadOwner != "ublue-os" {
			t.Errorf("DownloadOwner = %q, want %q", p.DownloadOwner, "ublue-os")
		}
		if p.DownloadRepo != "homebrew-experimental-tap" {
			t.Errorf("DownloadRepo = %q, want %q", p.DownloadRepo, "homebrew-experimental-tap")
		}
	})

	t.Run("cask with only tap URL and no other GitHub URL has no SourceOwner", func(t *testing.T) {
		content := `
cask "tool" do
  version "2.0.0"
  homepage "https://github.com/ublue-os/homebrew-tap"
  url "https://github.com/ublue-os/homebrew-tap/releases/download/v2.0.0/tool.zip"
end
`
		p := parseRuby("tool", "cask", content)
		// Tap URLs must NOT bleed into SourceOwner/SourceRepo.
		if p.SourceOwner == "ublue-os" && (p.SourceRepo == "homebrew-tap" || p.SourceRepo == "homebrew-experimental-tap") {
			t.Errorf("SourceOwner/SourceRepo must not point to the tap repos")
		}
		// But DownloadOwner/Repo should be set.
		if p.DownloadOwner != "ublue-os" {
			t.Errorf("DownloadOwner = %q, want %q", p.DownloadOwner, "ublue-os")
		}
		if p.DownloadRepo != "homebrew-tap" {
			t.Errorf("DownloadRepo = %q, want %q", p.DownloadRepo, "homebrew-tap")
		}
	})

	t.Run("cask with full fields and external GitHub source", func(t *testing.T) {
		content := `
cask "ghostty" do
  version "1.1.3"
  desc "Fast, native, feature-rich terminal emulator"
  homepage "https://ghostty.org"
  url "https://github.com/ghostty-org/ghostty/releases/download/v#{version}/Ghostty.dmg"
end
`
		p := parseRuby("ghostty", "cask", content)
		if p.Name != "ghostty" {
			t.Errorf("Name = %q, want %q", p.Name, "ghostty")
		}
		if p.Type != "cask" {
			t.Errorf("Type = %q, want %q", p.Type, "cask")
		}
		if p.Version != "1.1.3" {
			t.Errorf("Version = %q, want %q", p.Version, "1.1.3")
		}
		if p.Description != "Fast, native, feature-rich terminal emulator" {
			t.Errorf("Description = %q", p.Description)
		}
		if p.Homepage != "https://ghostty.org" {
			t.Errorf("Homepage = %q, want %q", p.Homepage, "https://ghostty.org")
		}
		if p.SourceOwner != "ghostty-org" {
			t.Errorf("SourceOwner = %q, want %q", p.SourceOwner, "ghostty-org")
		}
		if p.SourceRepo != "ghostty" {
			t.Errorf("SourceRepo = %q, want %q", p.SourceRepo, "ghostty")
		}
		// No tap URL → no download tracking.
		if p.DownloadOwner != "" {
			t.Errorf("DownloadOwner = %q, want empty", p.DownloadOwner)
		}
	})

	t.Run("package with no version or description", func(t *testing.T) {
		content := `
formula "minimal" do
  homepage "https://example.com"
end
`
		p := parseRuby("minimal", "formula", content)
		if p.Version != "" {
			t.Errorf("Version = %q, want empty", p.Version)
		}
		if p.Description != "" {
			t.Errorf("Description = %q, want empty", p.Description)
		}
		if p.Homepage != "https://example.com" {
			t.Errorf("Homepage = %q, want %q", p.Homepage, "https://example.com")
		}
	})

	t.Run("empty content produces empty package", func(t *testing.T) {
		p := parseRuby("empty", "formula", "")
		if p.Name != "empty" || p.Type != "formula" {
			t.Errorf("unexpected name/type: %q/%q", p.Name, p.Type)
		}
		if p.Version != "" || p.Description != "" || p.Homepage != "" {
			t.Errorf("expected all string fields empty for empty content")
		}
		if p.DownloadOwner != "" || p.DownloadRepo != "" {
			t.Errorf("expected DownloadOwner/Repo empty for empty content")
		}
	})
}
