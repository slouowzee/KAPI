package registry

import (
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

func TestLoadFallback_DirectCall(t *testing.T) {
	frameworks, err := loadFallback()
	if err != nil {
		t.Fatalf("loadFallback() error: %v", err)
	}
	if len(frameworks) == 0 {
		t.Fatal("loadFallback() returned empty slice")
	}
}
