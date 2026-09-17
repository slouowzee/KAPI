package scaffold

import (
	"strings"
	"testing"

	"github.com/slouowzee/kapi/internal/gitconfig"
	"github.com/slouowzee/kapi/internal/packagemanager"
)

func TestProjectNameError(t *testing.T) {
	tests := []struct {
		name    string
		eco     string
		dir     string
		wantErr bool
	}{
		{name: "valid js name", eco: "js", dir: "/home/me/my-app", wantErr: false},
		{name: "js uppercase", eco: "js", dir: "/home/me/MyApp", wantErr: true},
		{name: "js space", eco: "js", dir: "/home/me/my app", wantErr: true},
		{name: "js leading dot", eco: "js", dir: "/home/me/.app", wantErr: true},
		{name: "js leading underscore", eco: "js", dir: "/home/me/_app", wantErr: true},
		{name: "js too long", eco: "js", dir: "/home/me/" + strings.Repeat("a", 215), wantErr: true},
		{name: "php uppercase is fine", eco: "php", dir: "/home/me/MyApp", wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fw := jsfw("nextjs")
			fw.Ecosystem = tt.eco
			if got := ProjectNameError(fw, tt.dir) != ""; got != tt.wantErr {
				t.Errorf("ProjectNameError(%q) error = %v, want %v", tt.dir, got, tt.wantErr)
			}
		})
	}
}

func TestComposerPackageName(t *testing.T) {
	tests := map[string]string{
		"my-app":      "my-app",
		"MyApp":       "myapp",
		"My Cool App": "my-cool-app",
		"__app__":     "app",
		"app.v2":      "app-v2",
		"!!!":         "app",
	}
	for in, want := range tests {
		if got := composerPackageName(in); got != want {
			t.Errorf("composerPackageName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVanillaPhp_UsesValidComposerName(t *testing.T) {
	steps := Plan("/tmp/x/My App", fw("vanilla-php"), nil, gitconfig.GitConfig{}, packagemanager.None)
	found := false
	for _, s := range steps {
		if strings.Contains(s.Label, "composer init") {
			found = true
		}
	}
	if !found {
		t.Fatalf("composer init step missing: %v", stepLabels(steps))
	}
	if got := vanillaPhpComposerName("My App"); got != "my-app/my-app" {
		t.Errorf("composer name = %q, want my-app/my-app", got)
	}
}
