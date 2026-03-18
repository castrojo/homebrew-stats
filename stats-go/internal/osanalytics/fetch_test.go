package osanalytics

import "testing"

func TestBaseName(t *testing.T) {
	cases := []struct{ input, want string }{
		{"Ubuntu 24.04 LTS", "Ubuntu"},
		{"Ubuntu 22.04 LTS", "Ubuntu"},
		{"Fedora Linux 40", "Fedora Linux"},
		{"macOS Sequoia (15)", "macOS Sequoia"},
		{"Debian GNU/Linux 12", "Debian GNU/Linux"},
	}
	for _, c := range cases {
		got := baseName(c.input)
		if got != c.want {
			t.Errorf("baseName(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestIsMacOS(t *testing.T) {
	if !isMacOS("macOS Sequoia (15)") {
		t.Error("expected macOS to be macOS")
	}
	if isMacOS("Ubuntu 24.04 LTS") {
		t.Error("expected Ubuntu not to be macOS")
	}
}
