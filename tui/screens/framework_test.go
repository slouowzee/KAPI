package screens

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slouowzee/kapi/internal/registry"
	"github.com/slouowzee/kapi/internal/trends"
)

func newLoadedFrameworkModel(frameworks ...registry.Framework) FrameworkModel {
	m := FrameworkModel{
		ecosystem:    "js",
		statsCache:   make(map[string]trends.Stats),
		statsPending: make(map[string]bool),
	}
	m, _ = m.Update(frameworksLoadedMsg{frameworks: frameworks})
	return m
}

func TestFramework_TrendsRequestedOnlyForFocused(t *testing.T) {
	// Eagerly fetching every framework's trends on load is what exhausts the
	// unauthenticated GitHub rate limit and downloads megabytes of packagist
	// data up front; only the one under the cursor should be requested.
	fws := []registry.Framework{
		{ID: "nextjs", Ecosystem: "js"},
		{ID: "nuxt", Ecosystem: "js"},
		{ID: "astro", Ecosystem: "js"},
		{ID: "sveltekit", Ecosystem: "js"},
	}
	m := newLoadedFrameworkModel(fws...)

	if !m.statsPending["nextjs"] {
		t.Error("trends for the focused (first) framework should have been requested")
	}
	for _, id := range []string{"nuxt", "astro", "sveltekit"} {
		if m.statsPending[id] || m.statsCache[id] != (trends.Stats{}) {
			t.Errorf("trends for %q must not be requested until it is focused", id)
		}
	}
}

func TestFramework_CursorMoveRequestsTrendsOnceThenCaches(t *testing.T) {
	fws := []registry.Framework{
		{ID: "nextjs", Ecosystem: "js"},
		{ID: "nuxt", Ecosystem: "js"},
	}
	m := newLoadedFrameworkModel(fws...)
	if !m.statsPending["nextjs"] {
		t.Fatal("initial cursor should request trends")
	}
	m, _ = m.Update(trendsLoadedMsg{frameworkID: "nextjs", stats: trends.Stats{Stars: 5}})

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cmd == nil || !m.statsPending["nuxt"] {
		t.Fatal("moving the cursor to an uncached framework should request its trends")
	}
	m, _ = m.Update(trendsLoadedMsg{frameworkID: "nuxt", stats: trends.Stats{Stars: 10}})

	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if cmd != nil {
		t.Error("moving back to an already-cached framework must not refetch trends")
	}
}

func TestFramework_SearchRefocusesAndRequestsTrends(t *testing.T) {
	fws := []registry.Framework{
		{ID: "nextjs", Name: "Next.js", Ecosystem: "js"},
		{ID: "nuxt", Name: "Nuxt", Ecosystem: "js"},
	}
	m := newLoadedFrameworkModel(fws...)
	m, _ = m.Update(trendsLoadedMsg{frameworkID: "nextjs", stats: trends.Stats{Stars: 5}})

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("nuxt")})

	if len(m.visible) != 1 || m.visible[0].ID != "nuxt" {
		t.Fatalf("visible = %v, want only nuxt", m.visible)
	}
	if cmd == nil || !m.statsPending["nuxt"] {
		t.Error("filtering onto a new focused framework should request its trends")
	}
}

func TestRenderStats_ShowsLoadingUntilCached(t *testing.T) {
	m := newLoadedFrameworkModel(registry.Framework{ID: "nextjs", Name: "Next.js", Ecosystem: "js"})

	if got := m.renderStats(); !contains(got, "Loading stats") {
		t.Errorf("renderStats() = %q, want a loading indicator before trends arrive", got)
	}

	m, _ = m.Update(trendsLoadedMsg{frameworkID: "nextjs", stats: trends.Stats{Stars: 42, LatestVersion: "1.0.0"}})
	if got := m.renderStats(); contains(got, "Loading stats") {
		t.Errorf("renderStats() = %q, should no longer show the loading indicator", got)
	}
}
