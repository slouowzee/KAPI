package packagemanager

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		s    string
		want PM
	}{
		{"npm", NPM},
		{"pnpm", PNPM},
		{"yarn", Yarn},
		{"bun", Bun},
		{"", None},
		{"unknown", None},
		{"NPM", None},
	}
	for _, c := range cases {
		if got := Parse(c.s); got != c.want {
			t.Errorf("Parse(%q) = %v, want %v", c.s, got, c.want)
		}
	}
}

func TestString(t *testing.T) {
	cases := []struct {
		pm   PM
		want string
	}{
		{NPM, "npm"},
		{PNPM, "pnpm"},
		{Yarn, "yarn"},
		{Bun, "bun"},
		{None, ""},
	}
	for _, c := range cases {
		if got := c.pm.String(); got != c.want {
			t.Errorf("PM(%d).String() = %q, want %q", c.pm, got, c.want)
		}
	}
}

func TestLabel(t *testing.T) {
	for _, pm := range []PM{NPM, PNPM, Yarn, Bun, None} {
		if got, want := pm.Label(), pm.String(); got != want {
			t.Errorf("PM(%d).Label() = %q, want %q", pm, got, want)
		}
	}
}

func TestAll(t *testing.T) {
	all := All()
	if len(all) != 4 {
		t.Fatalf("All() returned %d items, want 4", len(all))
	}
	for _, pm := range all {
		if pm == None {
			t.Error("All() should not include None")
		}
	}
	want := []PM{NPM, PNPM, Yarn, Bun}
	for i, pm := range all {
		if pm != want[i] {
			t.Errorf("All()[%d] = %v, want %v", i, pm, want[i])
		}
	}
}

func TestExecArgs(t *testing.T) {
	cases := []struct {
		pm   PM
		want []string
	}{
		{NPM, []string{"npx"}},
		{PNPM, []string{"pnpm", "dlx"}},
		{Yarn, []string{"yarn", "dlx"}},
		{Bun, []string{"bunx"}},
		{None, []string{"npx"}},
	}
	for _, c := range cases {
		got := c.pm.ExecArgs()
		if !slices.Equal(got, c.want) {
			t.Errorf("PM(%v).ExecArgs() = %v, want %v", c.pm, got, c.want)
		}
	}
}

func TestCreateArgs(t *testing.T) {
	cases := []struct {
		pm   PM
		want []string
	}{
		{NPM, []string{"npm", "create"}},
		{PNPM, []string{"pnpm", "create"}},
		{Yarn, []string{"yarn", "create"}},
		{Bun, []string{"bun", "create"}},
	}
	for _, c := range cases {
		got := c.pm.CreateArgs()
		if !slices.Equal(got, c.want) {
			t.Errorf("PM(%v).CreateArgs() = %v, want %v", c.pm, got, c.want)
		}
	}
}

func TestInstallArgs(t *testing.T) {
	cases := []struct {
		pm   PM
		want []string
	}{
		{NPM, []string{"npm", "install"}},
		{PNPM, []string{"pnpm", "add"}},
		{Yarn, []string{"yarn", "add"}},
		{Bun, []string{"bun", "add"}},
	}
	for _, c := range cases {
		got := c.pm.InstallArgs()
		if !slices.Equal(got, c.want) {
			t.Errorf("PM(%v).InstallArgs() = %v, want %v", c.pm, got, c.want)
		}
	}
}

func TestCIInstall(t *testing.T) {
	cases := []struct {
		pm   PM
		want string
	}{
		{NPM, "npm ci"},
		{PNPM, "pnpm install --frozen-lockfile"},
		{Yarn, "yarn install --frozen-lockfile"},
		{Bun, "bun install --frozen-lockfile"},
	}
	for _, c := range cases {
		if got := c.pm.CIInstall(); got != c.want {
			t.Errorf("PM(%v).CIInstall() = %q, want %q", c.pm, got, c.want)
		}
	}
}

func TestRunScript(t *testing.T) {
	cases := []struct {
		pm   PM
		want string
	}{
		{NPM, "npm run"},
		{PNPM, "pnpm run"},
		{Yarn, "yarn run"},
		{Bun, "bun run"},
	}
	for _, c := range cases {
		if got := c.pm.RunScript(); got != c.want {
			t.Errorf("PM(%v).RunScript() = %q, want %q", c.pm, got, c.want)
		}
	}
}

func TestCacheKey(t *testing.T) {
	cases := []struct {
		pm   PM
		want string
	}{
		{NPM, "npm"},
		{PNPM, "pnpm"},
		{Yarn, "yarn"},
		{Bun, "bun"},
		{None, "npm"},
	}
	for _, c := range cases {
		if got := c.pm.CacheKey(); got != c.want {
			t.Errorf("PM(%v).CacheKey() = %q, want %q", c.pm, got, c.want)
		}
	}
}

func TestDetectFromLockfile(t *testing.T) {
	dir := t.TempDir()

	// No lockfiles → None
	if got := DetectFromLockfile(dir); got != None {
		t.Errorf("empty dir: got %v, want None", got)
	}

	write := func(name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("package-lock.json")
	if got := DetectFromLockfile(dir); got != NPM {
		t.Errorf("package-lock.json: got %v, want NPM", got)
	}

	write("yarn.lock")
	if got := DetectFromLockfile(dir); got != Yarn {
		t.Errorf("yarn.lock: got %v, want Yarn", got)
	}

	write("pnpm-lock.yaml")
	if got := DetectFromLockfile(dir); got != PNPM {
		t.Errorf("pnpm-lock.yaml: got %v, want PNPM", got)
	}

	write("bun.lock")
	if got := DetectFromLockfile(dir); got != Bun {
		t.Errorf("bun.lock: got %v, want Bun", got)
	}

	dir2 := t.TempDir()
	write2 := func(name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir2, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write2("bun.lockb")
	if got := DetectFromLockfile(dir2); got != Bun {
		t.Errorf("bun.lockb: got %v, want Bun", got)
	}
}
