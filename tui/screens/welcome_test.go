package screens

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slouowzee/kapi/internal/cli"
	"github.com/slouowzee/kapi/internal/ecosystem"
)

func wcKey(k string) tea.KeyMsg {
	switch k {
	case "up", "down", "enter", "esc", "space":
		return tea.KeyMsg{Type: keyTypeFor(k)}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	}
}

func keyTypeFor(k string) tea.KeyType {
	switch k {
	case "up":
		return tea.KeyUp
	case "down":
		return tea.KeyDown
	case "enter":
		return tea.KeyEnter
	case "esc":
		return tea.KeyEsc
	case "space":
		return tea.KeySpace
	}
	return tea.KeyRunes
}

func readyWelcome() WelcomeModel {
	m := NewWelcome(80, 24)
	return m.skipAnimation()
}

func TestBuildMenuItems(t *testing.T) {
	tests := []struct {
		name            string
		eco             ecosystem.Ecosystem
		hasGit          bool
		updateAvailable bool
		wantActions     []int
	}{
		{
			name:        "bare directory",
			eco:         ecosystem.ECOSYSTEM_NONE,
			wantActions: []int{MENU_NEW_PROJECT, MENU_SETTINGS},
		},
		{
			name:        "git repo without packages",
			eco:         ecosystem.ECOSYSTEM_NONE,
			hasGit:      true,
			wantActions: []int{MENU_NEW_PROJECT, MENU_GIT_CONFIG, MENU_SETTINGS},
		},
		{
			name:        "packages without git",
			eco:         ecosystem.ECOSYSTEM_JS,
			wantActions: []int{MENU_NEW_PROJECT, MENU_BROWSE_PACKAGES, MENU_SETTINGS},
		},
		{
			name:            "everything available",
			eco:             ecosystem.ECOSYSTEM_BOTH,
			hasGit:          true,
			updateAvailable: true,
			wantActions:     []int{MENU_NEW_PROJECT, MENU_GIT_CONFIG, MENU_BROWSE_PACKAGES, MENU_UPDATE, MENU_SETTINGS},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items := buildMenuItems(tt.eco, tt.hasGit, tt.updateAvailable, "v1.2.3")
			if len(items) != len(tt.wantActions) {
				t.Fatalf("got %d items, want %d (%+v)", len(items), len(tt.wantActions), items)
			}
			for i, action := range tt.wantActions {
				if items[i].action != action {
					t.Errorf("item %d action = %d, want %d", i, items[i].action, action)
				}
			}
		})
	}
}

func TestCurrentAction(t *testing.T) {
	m := readyWelcome()
	m.menuItems = buildMenuItems(ecosystem.ECOSYSTEM_NONE, false, false, "")

	m.cursor = 0
	if got := m.currentAction(); got != MENU_NEW_PROJECT {
		t.Errorf("currentAction() = %d, want MENU_NEW_PROJECT", got)
	}

	m.cursor = len(m.menuItems)
	if got := m.currentAction(); got != -1 {
		t.Errorf("currentAction() out of range = %d, want -1", got)
	}

	m.cursor = -1
	if got := m.currentAction(); got != -1 {
		t.Errorf("currentAction() negative = %d, want -1", got)
	}
}

func TestWelcomeUpdate_TypewriterAnimation(t *testing.T) {
	m := NewWelcome(80, 24)
	if m.logoReady || m.uiReady {
		t.Fatal("a fresh welcome model must start mid-animation")
	}

	// Walk the animation forward one tick per character across every logo
	// line; it must reach logoReady and ask for the UI reveal exactly once,
	// then further ticks must be no-ops.
	var cmd tea.Cmd
	for i := 0; i < 1000 && !m.logoReady; i++ {
		m, cmd = m.Update(tickMsg{})
	}
	if !m.logoReady {
		t.Fatal("logoReady never became true")
	}
	if cmd == nil {
		t.Fatal("expected uiRevealCmd on the tick that completes the logo")
	}
	if m.uiReady {
		t.Fatal("uiReady must wait for uiRevealMsg, not follow logoReady directly")
	}

	beforeLine, beforePos := m.currentLine, m.charPos
	m, cmd = m.Update(tickMsg{})
	if cmd != nil {
		t.Error("ticks after logoReady must return no command")
	}
	if m.currentLine != beforeLine || m.charPos != beforePos {
		t.Error("ticks after logoReady must not mutate the animation state")
	}
}

func TestWelcomeUpdate_UiRevealMsg(t *testing.T) {
	m := NewWelcome(80, 24)
	m.logoReady = true

	m, _ = m.Update(uiRevealMsg{})
	if !m.uiReady {
		t.Error("uiRevealMsg should set uiReady")
	}
}

func TestWelcomeUpdate_UpdateCheckMsg(t *testing.T) {
	m := NewWelcome(80, 24)
	m.hasGit = true
	m.ecosystem = ecosystem.ECOSYSTEM_JS

	m, _ = m.Update(updateCheckMsg{Available: true, LatestVersion: "v9.9.9"})

	if !m.updateReady {
		t.Error("updateReady should be set")
	}
	found := false
	for _, item := range m.menuItems {
		if item.action == MENU_UPDATE {
			found = true
		}
	}
	if !found {
		t.Error("menu should be rebuilt with the update entry once an update is available")
	}
}

func TestWelcomeUpdate_GitDetectMsg(t *testing.T) {
	m := NewWelcome(80, 24)
	m, _ = m.Update(gitDetectWelcomeMsg{hasGit: true})

	if !m.hasGit {
		t.Error("hasGit should be set from the detection message")
	}
	found := false
	for _, item := range m.menuItems {
		if item.action == MENU_GIT_CONFIG {
			found = true
		}
	}
	if !found {
		t.Error("menu should be rebuilt with the git config entry once a repo is detected")
	}
}

func TestWelcomeUpdate_KeyBeforeReady_SkipsAnimationOnly(t *testing.T) {
	tests := []string{"enter", "up", "down", "k", "j", "space", "a"}
	for _, key := range tests {
		t.Run(key, func(t *testing.T) {
			m := NewWelcome(80, 24)
			m.menuItems = buildMenuItems(ecosystem.ECOSYSTEM_NONE, false, false, "")
			startCursor := m.cursor

			m, cmd := m.Update(wcKey(key))

			if !m.logoReady || !m.uiReady {
				t.Fatal("an intentional key before uiReady must skip the animation")
			}
			if cmd != nil {
				t.Error("skipping the animation should not also return a command")
			}
			if m.enterPressed {
				t.Error("the same keypress that skips the animation must not also select the menu item")
			}
			if m.cursor != startCursor {
				t.Error("the same keypress that skips the animation must not also move the cursor")
			}
		})
	}
}

func TestWelcomeUpdate_CursorMovement(t *testing.T) {
	m := readyWelcome()
	m.menuItems = buildMenuItems(ecosystem.ECOSYSTEM_BOTH, true, true, "v2.0.0")
	m.cursor = 0

	m, _ = m.Update(wcKey("up"))
	if m.cursor != 0 {
		t.Errorf("cursor should clamp at 0, got %d", m.cursor)
	}

	m, _ = m.Update(wcKey("down"))
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}

	m.cursor = len(m.menuItems) - 1
	m, _ = m.Update(wcKey("down"))
	if m.cursor != len(m.menuItems)-1 {
		t.Errorf("cursor should clamp at the last item, got %d", m.cursor)
	}

	m.cursor = 0
	m, _ = m.Update(wcKey("j"))
	if m.cursor != 1 {
		t.Errorf("'j' should move down, cursor = %d, want 1", m.cursor)
	}
	m, _ = m.Update(wcKey("k"))
	if m.cursor != 0 {
		t.Errorf("'k' should move up, cursor = %d, want 0", m.cursor)
	}
}

func TestWelcomeUpdate_Enter(t *testing.T) {
	m := readyWelcome()
	m.menuItems = buildMenuItems(ecosystem.ECOSYSTEM_NONE, false, false, "")
	m.cursor = 0

	m, _ = m.Update(wcKey("enter"))
	if !m.enterPressed {
		t.Error("enter should set enterPressed")
	}
	if !m.IsNewProjectSelected() {
		t.Error("expected IsNewProjectSelected once enter is pressed on that item")
	}
}

func TestWelcomeUpdate_UpdateShortcut(t *testing.T) {
	m := readyWelcome()
	m.menuItems = buildMenuItems(ecosystem.ECOSYSTEM_BOTH, true, true, "v3.0.0")
	m.cursor = 0

	m, _ = m.Update(wcKey("u"))

	if !m.enterPressed {
		t.Error("'u' should set enterPressed when an update is available")
	}
	if !m.IsUpdateSelected() {
		t.Error("'u' should select the update menu item")
	}
}

func TestWelcomeUpdate_UpdateShortcut_NoUpdateAvailable(t *testing.T) {
	m := readyWelcome()
	m.menuItems = buildMenuItems(ecosystem.ECOSYSTEM_NONE, false, false, "")
	m.cursor = 0

	m, _ = m.Update(wcKey("u"))

	if m.enterPressed {
		t.Error("'u' must be a no-op when there is no update menu item")
	}
	if m.cursor != 0 {
		t.Error("'u' must not move the cursor when there is no update menu item")
	}
}

func TestWelcomeSelectionAccessors(t *testing.T) {
	tests := []struct {
		name   string
		action int
		check  func(WelcomeModel) bool
	}{
		{"new project", MENU_NEW_PROJECT, WelcomeModel.IsNewProjectSelected},
		{"git config", MENU_GIT_CONFIG, WelcomeModel.IsGitConfigSelected},
		{"browse packages", MENU_BROWSE_PACKAGES, WelcomeModel.IsBrowsePackagesSelected},
		{"update", MENU_UPDATE, WelcomeModel.IsUpdateSelected},
		{"settings", MENU_SETTINGS, WelcomeModel.IsSettingsSelected},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := readyWelcome()
			m.menuItems = []menuItem{{label: "x", action: tt.action}}
			m.cursor = 0

			if tt.check(m) {
				t.Fatal("should not be selected before enter is pressed")
			}

			m.enterPressed = true
			if !tt.check(m) {
				t.Error("should be selected once enter is pressed on the matching item")
			}

			m.ConsumeEnter()
			if m.enterPressed {
				t.Error("ConsumeEnter should reset enterPressed")
			}
			if tt.check(m) {
				t.Error("should not be selected after ConsumeEnter")
			}
		})
	}
}

func TestWelcomeAccessors(t *testing.T) {
	m := NewWelcome(80, 24)
	if m.WorkDir() != m.workDir {
		t.Errorf("WorkDir() = %q, want %q", m.WorkDir(), m.workDir)
	}
	if m.Ecosystem() != m.ecosystem {
		t.Errorf("Ecosystem() = %v, want %v", m.Ecosystem(), m.ecosystem)
	}

	m.updateInfo.LatestVersion = "v5.5.5"
	if m.LatestVersion() != "v5.5.5" {
		t.Errorf("LatestVersion() = %q, want v5.5.5", m.LatestVersion())
	}
}

func TestSkipAnimation(t *testing.T) {
	m := NewWelcome(80, 24)
	m.currentLine = 1
	m.charPos = 3

	m = m.skipAnimation()

	if m.currentLine != len(cli.LogoLines) {
		t.Errorf("currentLine = %d, want %d", m.currentLine, len(cli.LogoLines))
	}
	if m.charPos != 0 {
		t.Errorf("charPos = %d, want 0", m.charPos)
	}
	if !m.logoReady || !m.uiReady {
		t.Error("skipAnimation should mark both the logo and the UI ready")
	}
}
