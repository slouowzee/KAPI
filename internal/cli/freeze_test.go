package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/slouowzee/kapi/internal/config"
)

func setupTempHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	return tmp
}

func writeTestConfig(t *testing.T, cfg config.Config) {
	t.Helper()
	home := os.Getenv("HOME")
	dir := filepath.Join(home, ".config", "kapi")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
}

func captureStdout(t *testing.T) func() string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	return func() string {
		_ = w.Close()
		os.Stdout = orig
		var buf strings.Builder
		_, _ = io.Copy(&buf, r)
		return buf.String()
	}
}

func TestListFrozen_Empty(t *testing.T) {
	setupTempHome(t)
	writeTestConfig(t, config.Config{})

	done := captureStdout(t)

	listFrozen()

	output := done()
	if !strings.Contains(output, "No frozen packages.") {
		t.Errorf("expected 'No frozen packages.', got: %s", output)
	}
}

func TestListFrozen_WithPackages(t *testing.T) {
	setupTempHome(t)
	writeTestConfig(t, config.Config{
		FreezeVersionPackages: map[string][]config.FreezeVersionPackage{
			"npm": {
				{Name: "lodash", Version: "4.17.21", Note: "stable"},
				{Name: "react", Version: "18.2.0"},
			},
			"packagist": {
				{Name: "monolog/monolog", Version: "3.0.0", Note: "last working version"},
			},
		},
	})

	done := captureStdout(t)

	listFrozen()

	output := done()
	if !strings.Contains(output, "lodash") {
		t.Errorf("expected output to contain 'lodash', got: %s", output)
	}
	if !strings.Contains(output, "4.17.21") {
		t.Errorf("expected output to contain '4.17.21', got: %s", output)
	}
	if !strings.Contains(output, "monolog/monolog") {
		t.Errorf("expected output to contain 'monolog/monolog', got: %s", output)
	}
	if !strings.Contains(output, "packagist:") {
		t.Errorf("expected output to contain registry 'packagist:', got: %s", output)
	}
	if !strings.Contains(output, "npm:") {
		t.Errorf("expected output to contain registry 'npm:', got: %s", output)
	}
	if !strings.Contains(output, "stable") {
		t.Errorf("expected output to contain note 'stable', got: %s", output)
	}
}

func TestListFrozen_SortedAlphabetically(t *testing.T) {
	setupTempHome(t)
	writeTestConfig(t, config.Config{
		FreezeVersionPackages: map[string][]config.FreezeVersionPackage{
			"npm": {
				{Name: "zzz", Version: "1.0.0"},
				{Name: "aaa", Version: "2.0.0"},
			},
		},
	})

	done := captureStdout(t)

	listFrozen()

	output := done()
	aaaIdx := strings.Index(output, "aaa")
	zzzIdx := strings.Index(output, "zzz")
	if aaaIdx < 0 || zzzIdx < 0 {
		t.Fatal("expected both packages in output")
	}
	if aaaIdx > zzzIdx {
		t.Errorf("expected 'aaa' before 'zzz', got reversed order")
	}
}

func TestUnfreezeByName_RemovesFromConfig(t *testing.T) {
	setupTempHome(t)
	writeTestConfig(t, config.Config{
		FreezeVersionPackages: map[string][]config.FreezeVersionPackage{
			"npm": {
				{Name: "lodash", Version: "4.17.21"},
				{Name: "react", Version: "18.2.0"},
			},
		},
	})

	unfreezeByName("lodash")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	pkgs := cfg.FreezeVersionPackages["npm"]
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package after unfreeze, got %d", len(pkgs))
	}
	if pkgs[0].Name != "react" {
		t.Errorf("expected remaining package 'react', got %s", pkgs[0].Name)
	}
}

func TestUnfreezeByName_RemovesRegistryIfEmpty(t *testing.T) {
	setupTempHome(t)
	writeTestConfig(t, config.Config{
		FreezeVersionPackages: map[string][]config.FreezeVersionPackage{
			"npm": {
				{Name: "lodash", Version: "4.17.21"},
			},
		},
	})

	unfreezeByName("lodash")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.FreezeVersionPackages["npm"]; ok {
		t.Error("expected npm registry to be removed after unfreezing last package")
	}
}

func TestParseFreezeArg(t *testing.T) {
	tests := []struct {
		arg         string
		wantName    string
		wantVersion string
	}{
		{arg: "react", wantName: "react"},
		{arg: "react@18.2.0", wantName: "react", wantVersion: "18.2.0"},
		{arg: "react/18.2.0", wantName: "react", wantVersion: "18.2.0"},
		{arg: "@tanstack/react-query", wantName: "@tanstack/react-query"},
		{arg: "@tanstack/react-query@5.0.0", wantName: "@tanstack/react-query", wantVersion: "5.0.0"},
		{arg: "@types/node/20.1.0", wantName: "@types/node", wantVersion: "20.1.0"},
		{arg: "symfony/var-dumper", wantName: "symfony/var-dumper"},
		{arg: "symfony/validator", wantName: "symfony/validator"},
		{arg: "livewire/volt", wantName: "livewire/volt"},
		{arg: "symfony/var-dumper/v7.1.0", wantName: "symfony/var-dumper", wantVersion: "v7.1.0"},
		{arg: "laravel/framework@11.0.0-beta.1", wantName: "laravel/framework", wantVersion: "11.0.0-beta.1"},
		{arg: "vendor/2fa", wantName: "vendor/2fa"},
	}
	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			name, version := parseFreezeArg(tt.arg)
			if name != tt.wantName || version != tt.wantVersion {
				t.Errorf("parseFreezeArg(%q) = (%q, %q), want (%q, %q)", tt.arg, name, version, tt.wantName, tt.wantVersion)
			}
		})
	}
}
