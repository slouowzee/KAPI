package screens

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/slouowzee/kapi/internal/config"
	"github.com/slouowzee/kapi/internal/packages"
	"github.com/slouowzee/kapi/internal/registry"
)

func newFavoritesModel(t *testing.T, names ...string) PackagesModel {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	m := PackagesModel{framework: registry.Framework{ID: "nextjs"}}
	m.favorites = make([]packages.Package, 0, 16)
	for _, n := range names {
		m.favorites = append(m.favorites, packages.Package{Name: n})
	}
	return m
}

func TestToggleFavorite_ConcurrentSaveHasNoRace(t *testing.T) {
	m := newFavoritesModel(t, "a", "b", "c", "d", "e", "f")

	cmd := m.toggleFavorite(m.favorites[0])
	done := make(chan struct{})
	go func() {
		cmd()
		close(done)
	}()
	_ = m.toggleFavorite(m.favorites[0])
	<-done
}

func TestToggleFavorite_LatestToggleWins(t *testing.T) {
	m := newFavoritesModel(t)

	first := m.toggleFavorite(packages.Package{Name: "zod"})
	second := m.toggleFavorite(packages.Package{Name: "zod"})

	// Commands may run in any order; the older one must not win.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); second() }()
	wg.Wait()
	first()

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Favorites["nextjs"]; len(got) != 0 {
		t.Errorf("favorites = %+v, want empty after add then remove", got)
	}
}

func TestToggleFavorite_UnreadableConfigIsKept(t *testing.T) {
	m := newFavoritesModel(t)
	path := filepath.Join(os.Getenv("HOME"), ".config", "kapi", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	const corrupt = `{"github_token": "ghp_keep",`
	if err := os.WriteFile(path, []byte(corrupt), 0o600); err != nil {
		t.Fatal(err)
	}

	msg := m.toggleFavorite(packages.Package{Name: "zod"})()

	saved, ok := msg.(favoriteSavedMsg)
	if !ok || saved.err == nil {
		t.Fatalf("expected a favoriteSavedMsg with an error, got %#v", msg)
	}
	if data, _ := os.ReadFile(path); string(data) != corrupt {
		t.Errorf("config was overwritten: %q", data)
	}

	updated, _ := m.Update(msg)
	if updated.freezeStatus == "" {
		t.Error("the save error should be shown to the user")
	}
}
