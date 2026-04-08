package ecosystem

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetect(t *testing.T) {
	cases := []struct {
		name  string
		files []string
		want  Ecosystem
	}{
		{"empty dir", nil, ECOSYSTEM_NONE},
		{"php only — composer.lock", []string{"composer.lock"}, ECOSYSTEM_PHP},
		{"js — package-lock.json", []string{"package-lock.json"}, ECOSYSTEM_JS},
		{"js — yarn.lock", []string{"yarn.lock"}, ECOSYSTEM_JS},
		{"js — pnpm-lock.yaml", []string{"pnpm-lock.yaml"}, ECOSYSTEM_JS},
		{"js — bun.lock", []string{"bun.lock"}, ECOSYSTEM_JS},
		{"js — deno.lock", []string{"deno.lock"}, ECOSYSTEM_JS},
		{"both", []string{"composer.lock", "package-lock.json"}, ECOSYSTEM_BOTH},
		{"both with yarn", []string{"composer.lock", "yarn.lock"}, ECOSYSTEM_BOTH},
		{"unrelated file", []string{"README.md"}, ECOSYSTEM_NONE},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range c.files {
				writeFile(t, dir, f)
			}
			if got := Detect(dir); got != c.want {
				t.Errorf("Detect() = %v (%q), want %v (%q)", got, got.Label(), c.want, c.want.Label())
			}
		})
	}
}

func TestDetect_NonexistentDir(t *testing.T) {
	if got := Detect("/nonexistent/path/that/does/not/exist"); got != ECOSYSTEM_NONE {
		t.Errorf("Detect(nonexistent) = %v, want ECOSYSTEM_NONE", got)
	}
}

func TestLabel(t *testing.T) {
	cases := []struct {
		e    Ecosystem
		want string
	}{
		{ECOSYSTEM_NONE, ""},
		{ECOSYSTEM_PHP, "PHP project"},
		{ECOSYSTEM_JS, "JS project"},
		{ECOSYSTEM_BOTH, "PHP + JS project"},
	}
	for _, c := range cases {
		if got := c.e.Label(); got != c.want {
			t.Errorf("Ecosystem(%d).Label() = %q, want %q", c.e, got, c.want)
		}
	}
}

func TestHasPackages(t *testing.T) {
	if ECOSYSTEM_NONE.HasPackages() {
		t.Error("ECOSYSTEM_NONE.HasPackages() should be false")
	}
	for _, e := range []Ecosystem{ECOSYSTEM_PHP, ECOSYSTEM_JS, ECOSYSTEM_BOTH} {
		if !e.HasPackages() {
			t.Errorf("Ecosystem(%d).HasPackages() should be true", e)
		}
	}
}

func TestPHPLockFiles(t *testing.T) {
	if len(PHP_LOCK_FILES) == 0 {
		t.Error("PHP_LOCK_FILES should not be empty")
	}
	found := false
	for _, f := range PHP_LOCK_FILES {
		if f == "composer.lock" {
			found = true
		}
	}
	if !found {
		t.Error("PHP_LOCK_FILES should contain composer.lock")
	}
}

func TestJSLockFiles(t *testing.T) {
	required := []string{"package-lock.json", "yarn.lock", "pnpm-lock.yaml", "bun.lock"}
	for _, r := range required {
		found := false
		for _, f := range JS_LOCK_FILES {
			if f == r {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("JS_LOCK_FILES should contain %q", r)
		}
	}
}
