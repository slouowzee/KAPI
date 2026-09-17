package cli

import (
	"strings"
	"testing"

	"github.com/slouowzee/kapi/internal/config"
)

func TestResolvePackageManagerValue(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		want   string
		wantOK bool
	}{
		{name: "npm", value: "npm", want: "npm", wantOK: true},
		{name: "pnpm", value: "pnpm", want: "pnpm", wantOK: true},
		{name: "empty clears", value: "", want: "", wantOK: true},
		{name: "none clears", value: "none", want: "", wantOK: true},
		{name: "invalid", value: "cargo", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := resolvePackageManagerValue(tt.value)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("resolved = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHandleConfig_ClearsPackageManager(t *testing.T) {
	setupTempHome(t)
	writeTestConfig(t, config.Config{PackageManager: "bun", GithubToken: "keep-me"})

	done := captureStdout(t)
	HandleConfig([]string{"package.manager", "none"})
	done()

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PackageManager != "" {
		t.Errorf("PackageManager = %q, want cleared", cfg.PackageManager)
	}
	if cfg.GithubToken != "keep-me" {
		t.Errorf("GithubToken = %q, want unchanged", cfg.GithubToken)
	}
}

func TestHandleConfig_NoArgsListsEveryKey(t *testing.T) {
	setupTempHome(t)
	writeTestConfig(t, config.Config{PackageManager: "pnpm", GithubToken: "ghp_1234567890abcdef"})

	done := captureStdout(t)
	HandleConfig(nil)
	out := done()

	if !strings.Contains(out, "github.token") || !strings.Contains(out, "package.manager") {
		t.Errorf("output = %q, want both keys listed", out)
	}
	if !strings.Contains(out, "pnpm") {
		t.Errorf("output = %q, want the package manager value", out)
	}
	if strings.Contains(out, "1234567890abcdef") {
		t.Error("the full token must not be printed by the list")
	}
}

func TestHandleConfig_NoArgsListsUnsetKeys(t *testing.T) {
	setupTempHome(t)
	writeTestConfig(t, config.Config{})

	done := captureStdout(t)
	HandleConfig(nil)
	out := done()

	if strings.Count(out, "(not set)") != 2 {
		t.Errorf("output = %q, want both keys shown as not set", out)
	}
}
