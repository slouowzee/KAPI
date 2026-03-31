package screens

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slouowzee/kapi/internal/ecosystem"
	"github.com/slouowzee/kapi/internal/updater"
	"github.com/slouowzee/kapi/tui/styles"
)

var LOGO_LINES = []string{
	` _  __   _   ___ ___ `,
	`| |/ /  /_\ | _ \_ _|`,
	`| ' <  / _ \|  _/| | `,
	`|_|\_\/_/ \_\_| |___|`,
}

const (
	LOGO_SUBTITLE   = "Keep Accelerating Project Initialization"
	LOGO_GITHUB     = "github.com/slouowzee/KAPI"
	LOGO_GITHUB_URL = "https://github.com/slouowzee/KAPI"
)

// Menu item
const (
	MENU_NEW_PROJECT = iota
	MENU_BROWSE_PACKAGES
	MENU_UPDATE
)

var LOGO_GRADIENT = []lipgloss.Color{
	"#8B6542",
	"#8A7856",
	"#88896A",
	"#7C9A6B",
}

type tickMsg time.Time
type updateCheckMsg updater.UpdateInfo
type uiRevealMsg struct{}

type WelcomeModel struct {
	width  int
	height int

	currentLine int
	charPos     int
	logoReady   bool

	uiReady bool

	updateInfo  updater.UpdateInfo
	updateReady bool
	cursor      int
	menuItems   []menuItem

	// enterPressed is a one-shot flag set when the user presses enter.
	// The parent app reads it, acts on it, then resets the model — this avoids
	// re-triggering transitions on every subsequent message.
	enterPressed bool

	workDir   string
	ecosystem ecosystem.Ecosystem
}

type menuItem struct {
	label  string
	action int
}

func buildMenuItems(eco ecosystem.Ecosystem, updateAvailable bool, latestVersion string) []menuItem {
	items := []menuItem{
		{label: "Start a new project", action: MENU_NEW_PROJECT},
	}
	if eco.HasPackages() {
		items = append(items, menuItem{label: "Browse packages", action: MENU_BROWSE_PACKAGES})
	}
	if updateAvailable {
		items = append(items, menuItem{label: fmt.Sprintf("Update to %s", latestVersion), action: MENU_UPDATE})
	}
	return items
}

func NewWelcome(width, height int) WelcomeModel {
	dir, _ := os.Getwd()
	eco := ecosystem.Detect(dir)
	return WelcomeModel{
		width:     width,
		height:    height,
		cursor:    0,
		workDir:   dir,
		ecosystem: eco,
		menuItems: buildMenuItems(eco, false, ""),
	}
}

// SetSize updates the terminal dimensions without resetting any state.
// Called on tea.WindowSizeMsg to handle terminal resizes gracefully.
func (m *WelcomeModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func tickCmd() tea.Cmd {
	return tea.Tick(18*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func uiRevealCmd() tea.Cmd {
	return tea.Tick(400*time.Millisecond, func(t time.Time) tea.Msg {
		return uiRevealMsg{}
	})
}

func checkUpdateCmd() tea.Cmd {
	return func() tea.Msg {
		ch := updater.Check()
		info := <-ch
		return updateCheckMsg(info)
	}
}

func (m WelcomeModel) Init() tea.Cmd {
	return tea.Batch(tickCmd(), checkUpdateCmd())
}

func (m WelcomeModel) skipAnimation() WelcomeModel {
	m.currentLine = len(LOGO_LINES)
	m.charPos = 0
	m.logoReady = true
	m.uiReady = true
	return m
}

func (m WelcomeModel) currentAction() int {
	if m.cursor >= 0 && m.cursor < len(m.menuItems) {
		return m.menuItems[m.cursor].action
	}
	return -1
}

func (m WelcomeModel) Update(msg tea.Msg) (WelcomeModel, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		if m.logoReady {
			return m, nil
		}

		if m.currentLine >= len(LOGO_LINES) {
			m.logoReady = true
			return m, uiRevealCmd()
		}

		lineLen := len([]rune(LOGO_LINES[m.currentLine]))
		m.charPos++

		if m.charPos >= lineLen {
			m.charPos = 0
			m.currentLine++
		}

		return m, tickCmd()

	case updateCheckMsg:
		m.updateInfo = updater.UpdateInfo(msg)
		m.updateReady = true
		m.menuItems = buildMenuItems(m.ecosystem, m.updateInfo.Available, m.updateInfo.LatestVersion)

	case uiRevealMsg:
		// NOTE: guard against duplicate uiRevealMsg arriving after a skip
		if !m.uiReady {
			m.uiReady = true
		}

	case tea.KeyMsg:
		key := msg.String()
		isIntentional := len(key) == 1 || key == "enter" || key == "space" ||
			key == "up" || key == "down" || key == "k" || key == "j"
		if isIntentional && !m.uiReady {
			m = m.skipAnimation()
			return m, nil
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.menuItems)-1 {
				m.cursor++
			}
		case "enter":
			// NOTE: set a one-shot flag — the parent reads it and resets the model
			m.enterPressed = true
		case "u":
			// NOTE: update action will be handled by parent app model
		}
	}

	return m, nil
}

func hyperlink(url, text string) string {
	return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", url, text)
}

func renderLogoLine(lineIdx int, visible int) string {
	runes := []rune(LOGO_LINES[lineIdx])
	total := len(runes)
	if visible > total {
		visible = total
	}

	color := LOGO_GRADIENT[lineIdx]
	rendered := lipgloss.NewStyle().Foreground(color).Bold(true).Render(string(runes[:visible]))

	return rendered + strings.Repeat(" ", total-visible)
}

func (m WelcomeModel) View() string {
	var sb strings.Builder

	sb.WriteString("\n")

	for i := range LOGO_LINES {
		var visible int
		switch {
		case i < m.currentLine:
			// Fully revealed line
			visible = len([]rune(LOGO_LINES[i]))
		case i == m.currentLine:
			// Currently animating line
			visible = m.charPos
		default:
			// Not yet reached — render blank placeholder
			visible = 0
		}

		rendered := renderLogoLine(i, visible)

		if i == 2 && m.uiReady {
			githubLink := hyperlink(
				LOGO_GITHUB_URL,
				styles.LinkStyle.Render(LOGO_GITHUB),
			)
			row := lipgloss.JoinHorizontal(lipgloss.Top, rendered, "   ", githubLink)
			sb.WriteString(row + "\n")
		} else if i == 3 && m.uiReady {
			row := lipgloss.JoinHorizontal(lipgloss.Top, rendered, "   ", styles.MutedStyle.Render(LOGO_SUBTITLE))
			sb.WriteString(row + "\n")
		} else {
			sb.WriteString(rendered + "\n")
		}
	}

	if !m.uiReady {
		return sb.String()
	}

	sb.WriteString("\n")

	// Working directory + ecosystem badge
	pathStr := truncatePath(m.workDir)
	pathLine := styles.MutedStyle.Render("  " + pathStr)
	if label := m.ecosystem.Label(); label != "" {
		badge := lipgloss.NewStyle().Foreground(styles.COLOR_SUCCESS).Render("  · " + label)
		pathLine = pathLine + badge
	}
	sb.WriteString(pathLine + "\n")

	sb.WriteString("\n")

	if m.updateReady && m.updateInfo.Available {
		msg := fmt.Sprintf(
			"  KAPI is updating from %s to %s, press [u] to KAPI !",
			m.updateInfo.CurrentVersion,
			m.updateInfo.LatestVersion,
		)
		sb.WriteString(styles.SuccessStyle.Render(msg) + "\n")
		sb.WriteString("\n")
	}

	// Menu
	for i, item := range m.menuItems {
		if i == m.cursor {
			cursor := lipgloss.NewStyle().Foreground(styles.COLOR_PRIMARY).Bold(true).Render("  ❯❯")
			label := styles.SelectedStyle.Render(" " + item.label)
			sb.WriteString(fmt.Sprintf("%s%s\n", cursor, label))
		} else {
			sb.WriteString(fmt.Sprintf("      %s\n", styles.DimStyle.Render(item.label)))
		}
	}

	sb.WriteString("\n")

	// Hints bar
	hints := "  [↑↓] navigate   [↵] select   [q] quit"
	if m.updateReady && m.updateInfo.Available {
		hints = fmt.Sprintf(
			"  [↑↓] navigate   [↵] select   [u] update to %s   [q] quit",
			m.updateInfo.LatestVersion,
		)
	}
	sb.WriteString(styles.MutedStyle.Render(hints) + "\n")

	return sb.String()
}

func (m WelcomeModel) IsNewProjectSelected() bool {
	return m.enterPressed && m.currentAction() == MENU_NEW_PROJECT
}

func (m WelcomeModel) IsBrowsePackagesSelected() bool {
	return m.enterPressed && m.currentAction() == MENU_BROWSE_PACKAGES
}

func (m WelcomeModel) IsUpdateSelected() bool {
	return m.enterPressed && m.currentAction() == MENU_UPDATE
}

// ConsumeEnter resets the one-shot enterPressed flag after the parent has acted on it.
func (m *WelcomeModel) ConsumeEnter() {
	m.enterPressed = false
}
