package semver

import "testing"

func TestGreater(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		// Basic patch
		{"1.2.3", "1.2.2", true},
		{"1.2.2", "1.2.3", false},
		{"1.2.3", "1.2.3", false},
		// Minor
		{"1.10.0", "1.9.0", true},
		{"1.9.0", "1.10.0", false},
		// Major
		{"2.0.0", "1.9.9", true},
		{"1.9.9", "2.0.0", false},
		// v prefix
		{"v1.2.3", "v1.2.2", true},
		{"v2.0.0", "1.9.9", true},
		{"1.0.0", "v0.9.9", true},
		// Zero version
		{"0.0.1", "0.0.2", false},
		{"0.0.2", "0.0.1", true},
		// Equal
		{"v1.0.0", "1.0.0", false},
		// Longer vs shorter
		{"1.0.0.1", "1.0.0", true},
		{"1.0.0", "1.0.0.1", false},
		// Non-numeric segments fall back to lexicographic
		{"1.0.0-beta", "1.0.0-alpha", true},
		{"1.0.0-alpha", "1.0.0-beta", false},
		// Empty strings
		{"", "", false},
		{"1.0.0", "", true},
		{"", "1.0.0", false},
	}
	for _, c := range cases {
		got := Greater(c.a, c.b)
		if got != c.want {
			t.Errorf("Greater(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
