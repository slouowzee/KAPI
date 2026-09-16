package screens

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

func newFrozenViewModel(names ...string) PackagesModel {
	m := PackagesModel{
		framework:    registry.Framework{ID: "nextjs"},
		inFreezeView: true,
		freezeData:   make(map[string]config.FreezeVersionPackage),
	}
	for _, n := range names {
		m.freezeData[n] = config.FreezeVersionPackage{Name: n, Version: "1.0.0"}
	}
	return m
}

func TestFrozenPackagesList_IsSorted(t *testing.T) {
	m := newFrozenViewModel("zod", "axios", "react", "motion", "dayjs", "clsx")
	for i := 0; i < 20; i++ {
		entries := m.frozenPackagesList()
		for j := 1; j < len(entries); j++ {
			if entries[j-1].Name > entries[j].Name {
				t.Fatalf("entries not sorted: %v before %v", entries[j-1].Name, entries[j].Name)
			}
		}
	}
}

func TestFrozenView_ActsOnFilteredEntry(t *testing.T) {
	m := newFrozenViewModel("axios", "react", "zod")
	m.query = "zo"
	m.freezeViewCursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})

	if len(updated.cart) != 1 || updated.cart[0].Name != "zod" {
		t.Errorf("cart = %+v, want the filtered entry zod", updated.cart)
	}
}

func TestFrozenView_CursorClampedToFilteredList(t *testing.T) {
	m := newFrozenViewModel("axios", "react", "zod")
	m.freezeViewCursor = 2

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})

	if n := len(updated.getFilteredFrozen()); updated.freezeViewCursor >= n {
		t.Errorf("cursor %d out of range for %d filtered entries", updated.freezeViewCursor, n)
	}
}

func TestLoadFreezeData_OnlyCurrentRegistry(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	err := config.Update(func(cfg *config.Config) error {
		cfg.SetFrozen("npm", config.FreezeVersionPackage{Name: "react", Version: "18.0.0"})
		cfg.SetFrozen("packagist", config.FreezeVersionPackage{Name: "laravel/framework", Version: "11.0.0"})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		isPhp   bool
		want    string
		notWant string
	}{
		{isPhp: false, want: "react", notWant: "laravel/framework"},
		{isPhp: true, want: "laravel/framework", notWant: "react"},
	}
	for _, tt := range tests {
		data := loadFreezeData(tt.isPhp)
		if _, ok := data[tt.want]; !ok {
			t.Errorf("isPhp=%v: missing %s", tt.isPhp, tt.want)
		}
		if _, ok := data[tt.notWant]; ok {
			t.Errorf("isPhp=%v: %s from the other registry must be ignored", tt.isPhp, tt.notWant)
		}
	}
}

func TestFreezeActionMsg_SyncsCartPins(t *testing.T) {
	tests := []struct {
		name       string
		freezeData map[string]config.FreezeVersionPackage
		want       string
	}{
		{name: "freeze pins a cart item", freezeData: map[string]config.FreezeVersionPackage{"zod": {Name: "zod", Version: "3.22.0"}}, want: "3.22.0"},
		{name: "unfreeze clears the pin", freezeData: map[string]config.FreezeVersionPackage{}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := PackagesModel{cart: []packages.Package{{Name: "zod", PinnedVersion: "3.0.0"}}}
			updated, _ := m.Update(freezeActionMsg{action: "frozen", name: "zod", freezeData: tt.freezeData})
			if got := updated.cart[0].PinnedVersion; got != tt.want {
				t.Errorf("PinnedVersion = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPackages_FreezeLoadsVersionsOnDemand(t *testing.T) {
	m := PackagesModel{results: []packages.Package{{Name: "zod"}}}

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	if cmd == nil || m.versionsFor != "zod" || m.selectVersion {
		t.Fatalf("ctrl+g should request versions (cmd=%v versionsFor=%q select=%v)", cmd != nil, m.versionsFor, m.selectVersion)
	}

	m, _ = m.Update(versionsLoadedMsg{name: "other", versions: []string{"1.0.0"}})
	if m.selectVersion {
		t.Fatal("versions of another package must be ignored")
	}

	m, _ = m.Update(versionsLoadedMsg{name: "zod", versions: []string{"3.0.0", "2.0.0"}})
	if !m.selectVersion || m.freezeTargetName != "zod" || len(m.versionList) != 2 {
		t.Errorf("version picker not opened: select=%v target=%q versions=%v", m.selectVersion, m.freezeTargetName, m.versionList)
	}
}

func TestPackages_StarsRequestedOnceForFocusedPackage(t *testing.T) {
	m := PackagesModel{results: []packages.Package{{Name: "zod", GithubRepo: "colinhacks/zod"}, {Name: "no-repo"}}}

	m, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if cmd == nil {
		t.Fatal("stars of the focused package should be requested")
	}
	m, cmd = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if cmd != nil {
		t.Error("stars must not be requested twice")
	}

	m, _ = m.Update(starsLoadedMsg{repo: "colinhacks/zod", stars: 30000})
	if m.stars["colinhacks/zod"] != 30000 {
		t.Errorf("stars = %d, want 30000", m.stars["colinhacks/zod"])
	}

	m.cursor = 1
	if _, cmd = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24}); cmd != nil {
		t.Error("packages without repository must not trigger a request")
	}
}
