package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ReturnsFrameworks(t *testing.T) {
	frameworks, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(frameworks) == 0 {
		t.Fatal("Load() returned empty slice, expected at least one framework")
	}
}

func TestLoad_AllFrameworksHaveRequiredFields(t *testing.T) {
	frameworks, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	for _, f := range frameworks {
		if f.ID == "" {
			t.Errorf("framework %q has empty ID", f.Name)
		}
		if f.Name == "" {
			t.Errorf("framework ID=%q has empty Name", f.ID)
		}
		if f.Ecosystem == "" {
			t.Errorf("framework %q (ID=%q) has empty Ecosystem", f.Name, f.ID)
		}
	}
}

func TestLoad_EcosystemValues(t *testing.T) {
	frameworks, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	validEcosystems := map[string]bool{"php": true, "js": true, "ts": true}
	for _, f := range frameworks {
		if !validEcosystems[f.Ecosystem] {
			t.Errorf("framework %q has unexpected ecosystem %q", f.Name, f.Ecosystem)
		}
	}
}

func TestLoad_UniqueIDs(t *testing.T) {
	frameworks, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	seen := make(map[string]bool, len(frameworks))
	for _, f := range frameworks {
		if seen[f.ID] {
			t.Errorf("duplicate framework ID: %q", f.ID)
		}
		seen[f.ID] = true
	}
}

func TestLoad_IsIdempotent(t *testing.T) {
	first, err := Load()
	if err != nil {
		t.Fatalf("first Load() error: %v", err)
	}
	second, err := Load()
	if err != nil {
		t.Fatalf("second Load() error: %v", err)
	}
	if len(first) != len(second) {
		t.Errorf("Load() returned different lengths: %d vs %d", len(first), len(second))
	}
}

func TestLoad_FrameworksCanBeDetected(t *testing.T) {
	frameworks, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range frameworks {
		if f.ID != "vanilla-php" && len(f.Detect) == 0 {
			t.Errorf("framework %q has no detect rule", f.ID)
		}
	}
}

func TestDetectProjectFramework(t *testing.T) {
	tests := []struct {
		name      string
		eco       string
		file      string
		manifest  string
		wantID    string
		wantFound bool
	}{
		{name: "next app", eco: "js", file: "package.json", manifest: `{"dependencies":{"next":"15","react":"19"}}`, wantID: "nextjs", wantFound: true},
		{name: "remix beats react vite", eco: "js", file: "package.json", manifest: `{"dependencies":{"@remix-run/react":"2","react":"19"},"devDependencies":{"vite":"6"}}`, wantID: "remix", wantFound: true},
		{name: "react with vite", eco: "js", file: "package.json", manifest: `{"dependencies":{"react":"19"},"devDependencies":{"vite":"6"}}`, wantID: "react-vite", wantFound: true},
		{name: "plain vite", eco: "js", file: "package.json", manifest: `{"devDependencies":{"vite":"6"}}`, wantID: "vanilla-vite", wantFound: true},
		{name: "expo beats react native", eco: "js", file: "package.json", manifest: `{"dependencies":{"expo":"52","react-native":"0.76"}}`, wantID: "expo", wantFound: true},
		{name: "laravel", eco: "php", file: "composer.json", manifest: `{"require":{"php":"^8.2","laravel/framework":"^12.0"}}`, wantID: "laravel", wantFound: true},
		{name: "api platform beats symfony", eco: "php", file: "composer.json", manifest: `{"require":{"symfony/framework-bundle":"7","api-platform/core":"4"}}`, wantID: "api-platform", wantFound: true},
		{name: "drupal alternative dependency", eco: "php", file: "composer.json", manifest: `{"require":{"drupal/core":"^11"}}`, wantID: "drupal", wantFound: true},
		{name: "wrong ecosystem", eco: "php", file: "package.json", manifest: `{"dependencies":{"next":"15"}}`, wantFound: false},
		{name: "unknown dependencies", eco: "js", file: "package.json", manifest: `{"dependencies":{"lodash":"4"}}`, wantFound: false},
		{name: "invalid manifest", eco: "js", file: "package.json", manifest: `{not json`, wantFound: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, tt.file), []byte(tt.manifest), 0o644); err != nil {
				t.Fatal(err)
			}
			fw, found := DetectProjectFramework(dir, tt.eco)
			if found != tt.wantFound || fw.ID != tt.wantID {
				t.Errorf("DetectProjectFramework = (%q, %v), want (%q, %v)", fw.ID, found, tt.wantID, tt.wantFound)
			}
		})
	}
}
