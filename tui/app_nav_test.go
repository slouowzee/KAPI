package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slouowzee/kapi/internal/ecosystem"
	"github.com/slouowzee/kapi/internal/packagemanager"
	"github.com/slouowzee/kapi/internal/packages"
	"github.com/slouowzee/kapi/internal/registry"
	"github.com/slouowzee/kapi/tui/screens"
)

func key(k string) tea.KeyMsg {
	switch k {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}

func send(t *testing.T, a App, msgs ...tea.Msg) (App, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, msg := range msgs {
		var model tea.Model
		model, cmd = a.Update(msg)
		a = model.(App)
	}
	return a, cmd
}

var laravel = registry.Framework{ID: "laravel", Name: "Laravel", Ecosystem: "php"}

// recapApp returns an app on the summary of a Laravel project with packages.
func recapApp(t *testing.T) App {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	a := App{
		width:             80,
		height:            24,
		selectedDir:       "/tmp/my-app",
		selectedEcosystem: ecosystem.ECOSYSTEM_PHP,
		selectedFramework: laravel,
		selectedGit:       screens.GitConfig{InitLocal: true, CI: "none"},
	}
	a.packages = screens.NewPackagesFromCart(80, 24, laravel, a.selectedDir, []packages.Package{{Name: "laravel/pint"}})
	a, _ = a.goToRecap()
	return a
}

// editSection moves the summary cursor from "Confirm" up to a section.
func editSection(t *testing.T, a App, ups int) App {
	t.Helper()
	for i := 0; i < ups; i++ {
		a, _ = send(t, a, key("up"))
	}
	a, _ = send(t, a, key("enter"))
	return a
}

func TestNav_EditFolderAndGoBackToSummary(t *testing.T) {
	a := recapApp(t)

	a = editSection(t, a, 4) // confirm -> git -> packages -> framework -> folder
	if a.screen != ScreenFolder || !a.editMode {
		t.Fatalf("screen=%v editMode=%v, want folder in edit mode", a.screen, a.editMode)
	}

	a, _ = send(t, a, key("esc"))
	if a.screen != ScreenRecap || a.editMode {
		t.Errorf("screen=%v editMode=%v, want back on the summary", a.screen, a.editMode)
	}
	if a.selectedDir != "/tmp/my-app" {
		t.Errorf("selectedDir = %q, want it unchanged", a.selectedDir)
	}
}

func TestNav_EditFramework(t *testing.T) {
	tests := []struct {
		name         string
		ecosystemKey []string
		wantScreen   Screen
		wantPackages int
	}{
		{name: "same framework keeps packages", ecosystemKey: []string{"enter"}, wantScreen: ScreenRecap, wantPackages: 1},
		{name: "other ecosystem resets packages", ecosystemKey: []string{"down", "enter"}, wantScreen: ScreenPackages, wantPackages: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := recapApp(t)
			a = editSection(t, a, 3)
			if a.screen != ScreenEcosystem {
				t.Fatalf("screen = %v, want ScreenEcosystem", a.screen)
			}
			for _, k := range tt.ecosystemKey {
				a, _ = send(t, a, key(k))
			}
			a, _ = send(t, a, a.framework.Init()())
			a, _ = send(t, a, key("enter"))

			if a.screen != tt.wantScreen {
				t.Errorf("screen = %v, want %v", a.screen, tt.wantScreen)
			}
			if got := len(a.selectedPackages); got != tt.wantPackages {
				t.Errorf("selected packages = %d, want %d", got, tt.wantPackages)
			}
		})
	}
}

func TestNav_CancelPackageEditRestoresCart(t *testing.T) {
	a := recapApp(t)
	a = editSection(t, a, 2)
	if a.screen != ScreenPackages {
		t.Fatalf("screen = %v, want ScreenPackages", a.screen)
	}

	a, _ = send(t, a, key(" "), key("esc"))

	if a.screen != ScreenRecap || len(a.selectedPackages) != 1 {
		t.Errorf("screen=%v packages=%d, want summary with the original cart", a.screen, len(a.selectedPackages))
	}
}

func TestNav_AbandonResetsSelections(t *testing.T) {
	a := recapApp(t)

	a, _ = send(t, a, key("down"), key("enter"))
	if !a.recap.IsAbandonPending() {
		t.Fatal("abandon should ask for confirmation")
	}
	if _, cmd := send(t, a, key("q")); cmd != nil {
		if _, quit := cmd().(tea.QuitMsg); quit {
			t.Fatal("q must not quit while the abandon confirmation is shown")
		}
	}
	a, _ = send(t, a, key("y"))

	if a.screen != ScreenWelcome || a.selectedDir != "" || a.selectedFramework.ID != "" || a.selectedPackages != nil || a.editMode {
		t.Errorf("state not reset after abandon: screen=%v dir=%q fw=%q", a.screen, a.selectedDir, a.selectedFramework.ID)
	}
}

func TestNav_GitDoneRoutesByEcosystem(t *testing.T) {
	tests := []struct {
		name       string
		fw         registry.Framework
		wantScreen Screen
	}{
		{name: "php goes to summary", fw: laravel, wantScreen: ScreenRecap},
		{name: "js asks for a package manager", fw: registry.Framework{ID: "nextjs", Ecosystem: "js"}, wantScreen: ScreenPMSelect},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			a := App{width: 80, height: 24, selectedDir: "/tmp/my-app", selectedFramework: tt.fw, screen: ScreenGit}
			a.git = screens.Git(80, 24, a.selectedDir, screens.GitConfig{InitLocal: true, CI: "none"})

			a, _ = send(t, a, key("enter"))

			if a.screen != tt.wantScreen {
				t.Errorf("screen = %v, want %v", a.screen, tt.wantScreen)
			}
		})
	}
}

func TestNav_PMSelectBackReturnsToGit(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a := App{width: 80, height: 24, selectedFramework: registry.Framework{ID: "nextjs", Ecosystem: "js"}, defaultPM: packagemanager.Bun}
	a, _ = a.goToPMSelect()

	a, _ = send(t, a, key("esc"))

	if a.screen != ScreenGit {
		t.Errorf("screen = %v, want ScreenGit", a.screen)
	}
}

func TestNav_QuitKey(t *testing.T) {
	tests := []struct {
		name     string
		app      func(t *testing.T) App
		wantQuit bool
	}{
		{name: "welcome quits", app: func(t *testing.T) App {
			return App{screen: ScreenWelcome, welcome: screens.NewWelcome(80, 24)}
		}, wantQuit: true},
		{name: "folder path input keeps typing", app: func(t *testing.T) App {
			return App{screen: ScreenFolder, folder: screens.Folder(80, 24, "/tmp")}
		}, wantQuit: false},
		{name: "settings token input keeps typing", app: func(t *testing.T) App {
			t.Setenv("HOME", t.TempDir())
			a := App{screen: ScreenSettings, settings: screens.NewSettings(80, 24)}
			a, _ = send(t, a, key("down"), key("enter"))
			return a
		}, wantQuit: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, cmd := send(t, tt.app(t), key("q"))
			quit := false
			if cmd != nil {
				_, quit = cmd().(tea.QuitMsg)
			}
			if quit != tt.wantQuit {
				t.Errorf("quit = %v, want %v", quit, tt.wantQuit)
			}
		})
	}
}
