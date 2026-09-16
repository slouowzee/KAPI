package tui

import (
	"testing"

	"github.com/slouowzee/kapi/internal/packagemanager"
	"github.com/slouowzee/kapi/internal/registry"
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
