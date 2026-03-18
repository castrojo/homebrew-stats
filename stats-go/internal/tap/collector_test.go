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
