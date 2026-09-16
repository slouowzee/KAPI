package screens

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewSettings_ReportsConfigErrorInView(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".config", "kapi", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}

	m := NewSettings(80, 24)

	if m.lastErr == nil || m.lastMsg == "" {
		t.Error("config load error should be shown in the settings screen")
	}
}
