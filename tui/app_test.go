package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slouowzee/kapi/internal/ecosystem"
	"github.com/slouowzee/kapi/internal/packagemanager"
	"github.com/slouowzee/kapi/internal/packages"
	"github.com/slouowzee/kapi/internal/registry"
	"github.com/slouowzee/kapi/scaffold"
	"github.com/slouowzee/kapi/tui/screens"
)

func TestGoToRecap_JSWithoutPackageManager_AsksForIt(t *testing.T) {
	tests := []struct {
		name       string
		fw         registry.Framework
		pm         packagemanager.PM
		wantScreen Screen
	}{
		{name: "js without pm", fw: registry.Framework{ID: "nextjs", Ecosystem: "js"}, pm: packagemanager.None, wantScreen: ScreenPMSelect},
		{name: "js with pm", fw: registry.Framework{ID: "nextjs", Ecosystem: "js"}, pm: packagemanager.Bun, wantScreen: ScreenRecap},
		{name: "php never needs a pm", fw: registry.Framework{ID: "laravel", Ecosystem: "php"}, pm: packagemanager.None, wantScreen: ScreenRecap},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := App{selectedFramework: tt.fw, selectedPM: tt.pm, defaultPM: packagemanager.PNPM}
			a, _ = a.goToRecap()
			if a.screen != tt.wantScreen {
				t.Errorf("screen = %v, want %v", a.screen, tt.wantScreen)
			}
		})
	}
}

func TestUpdate_CtrlCDuringScaffoldDoesNotQuit(t *testing.T) {
	a := App{
		screen: ScreenExec,
		exec:   screens.NewExec(80, 24, []screens.ExecStep{{Label: "step", Fn: func() error { return nil }}}, ""),
	}
	for _, key := range []tea.KeyMsg{{Type: tea.KeyCtrlC}, {Type: tea.KeyRunes, Runes: []rune("q")}} {
		_, cmd := a.Update(key)
		if cmd != nil {
			if _, quit := cmd().(tea.QuitMsg); quit {
				t.Errorf("%q quit the app while scaffolding", key.String())
			}
		}
	}
}

func TestBrowse_MixedProjectAsksForEcosystem(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a := App{browseMode: true, selectedDir: t.TempDir(), screen: ScreenEcosystem}
	a.ecosystem = screens.NewEcosystem(80, 24, a.selectedDir)

	model, _ := a.Update(tea.KeyMsg{Type: tea.KeyEnter})
	a = model.(App)

	if a.screen != ScreenPackages || a.selectedFramework.Ecosystem != "php" {
		t.Errorf("screen=%v ecosystem=%q, want the packages screen for the chosen ecosystem", a.screen, a.selectedFramework.Ecosystem)
	}
}

func TestBrowse_InstallCartIntoCurrentProject(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	withInstallPreflight(t, nil)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	a := App{browseMode: true, selectedDir: dir}
	a, _ = a.startBrowse(ecosystem.ECOSYSTEM_JS)
	a, _ = a.installBrowsedPackages()
	if a.screen != ScreenWelcome || a.browseMode {
		t.Fatalf("empty cart should go back to the menu, got screen %v", a.screen)
	}

	a = App{browseMode: true, selectedDir: dir}
	a, _ = a.startBrowse(ecosystem.ECOSYSTEM_JS)
	a.packages = screens.NewPackagesFromCart(80, 24, a.selectedFramework, dir, []packages.Package{{Name: "zod"}})
	a, _ = a.installBrowsedPackages()
	if a.screen != ScreenExec {
		t.Fatalf("screen = %v, want ScreenExec", a.screen)
	}
	if view := a.exec.View(); !strings.Contains(view, "pnpm add zod") {
		t.Errorf("install should use the lockfile package manager, view:\n%s", view)
	}
}

func TestBrowse_UsesDetectedFramework(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tests := []struct {
		name     string
		manifest string
		wantID   string
	}{
		{name: "detected next app", manifest: `{"dependencies":{"next":"15"}}`, wantID: "nextjs"},
		{name: "unknown project falls back", manifest: `{"dependencies":{"lodash":"4"}}`, wantID: "vanilla-vite"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(tt.manifest), 0o644); err != nil {
				t.Fatal(err)
			}
			a := App{browseMode: true, selectedDir: dir}
			a, _ = a.startBrowse(ecosystem.ECOSYSTEM_JS)
			if a.selectedFramework.ID != tt.wantID {
				t.Errorf("framework = %q, want %q", a.selectedFramework.ID, tt.wantID)
			}
		})
	}
}

func withInstallPreflight(t *testing.T, issues []scaffold.Issue) {
	t.Helper()
	orig := installPreflight
	installPreflight = func(registry.Framework, packagemanager.PM) []scaffold.Issue { return issues }
	t.Cleanup(func() { installPreflight = orig })
}

func TestBrowse_MissingPackageManagerBlocksInstall(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	withInstallPreflight(t, []scaffold.Issue{{Blocking: true, Message: "pnpm is not installed or not in PATH"}})
	dir := t.TempDir()

	a := App{browseMode: true, selectedDir: dir}
	a, _ = a.startBrowse(ecosystem.ECOSYSTEM_JS)
	a.packages = screens.NewPackagesFromCart(120, 40, a.selectedFramework, dir, []packages.Package{{Name: "zod"}})
	a, _ = a.installBrowsedPackages()

	if a.screen != ScreenPackages {
		t.Fatalf("screen = %v, want to stay on the packages screen", a.screen)
	}
	if view := a.packages.View(); !strings.Contains(view, "pnpm is not installed") {
		t.Error("the missing tool should be reported on the packages screen")
	}
}
