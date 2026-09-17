package screens

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slouowzee/kapi/internal/config"
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

func TestSettings_SaveGithubToken(t *testing.T) {
	tests := []struct {
		name      string
		typed     string
		wantToken string
	}{
		{name: "save typed token", typed: " ghp_abc123 ", wantToken: "ghp_abc123"},
		{name: "empty input clears token", typed: "", wantToken: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("GITHUB_TOKEN", "")
			if err := config.Save(config.Config{GithubToken: "old", PackageManager: "bun"}); err != nil {
				t.Fatal(err)
			}

			m := NewSettings(80, 24)
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if !m.IsInputMode() {
				t.Fatal("enter on the token item should open the input")
			}
			if tt.typed != "" {
				m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.typed)})
			}
			if view := m.View(); strings.Contains(view, "abc123") {
				t.Error("the token must be masked while typing")
			}
			m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if cmd == nil {
				t.Fatal("enter should save the token")
			}
			m, _ = m.Update(cmd())

			cfg, err := config.Load()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.GithubToken != tt.wantToken || cfg.PackageManager != "bun" {
				t.Errorf("config = %+v, want token %q and package manager kept", cfg, tt.wantToken)
			}
			if m.IsInputMode() || m.lastErr != nil {
				t.Errorf("should be back on the menu without error (err=%v)", m.lastErr)
			}
		})
	}
}
