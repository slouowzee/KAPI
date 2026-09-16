package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slouowzee/kapi/internal/packagemanager"
	"github.com/slouowzee/kapi/internal/registry"
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
