package screens

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slouowzee/kapi/internal/cli"
	"github.com/slouowzee/kapi/internal/ecosystem"
	"github.com/slouowzee/kapi/internal/updater"
	"github.com/slouowzee/kapi/tui/styles"
)

const (
	LOGO_SUBTITLE   = "Keep Accelerating Project Initialization"
	LOGO_GITHUB     = "github.com/slouowzee/KAPI"
	LOGO_GITHUB_URL = "https://github.com/slouowzee/KAPI"
)
const (
	MENU_NEW_PROJECT = iota
	MENU_GIT_CONFIG
	MENU_BROWSE_PACKAGES
	MENU_UPDATE
	MENU_SETTINGS
)

type tickMsg time.Time
type updateCheckMsg updater.UpdateInfo
type uiRevealMsg struct{}
type gitDetectWelcomeMsg struct{ hasGit bool }

func detectGitWelcomeCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
		cmd.Dir = dir
		out, err := cmd.Output()
		return gitDetectWelcomeMsg{hasGit: err == nil && strings.TrimSpace(string(out)) == "true"}
	}
}

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

	enterPressed bool

	workDir   string
	ecosystem ecosystem.Ecosystem
	hasGit    bool
}

type menuItem struct {
	label  string
	action int
}

func buildMenuItems(eco ecosystem.Ecosystem, hasGit bool, updateAvailable bool, latestVersion string) []menuItem {
	items := []menuItem{
		{label: "Start a new project", action: MENU_NEW_PROJECT},
	}
	if hasGit {
		items = append(items, menuItem{label: "Git configuration", action: MENU_GIT_CONFIG})
	}
	if eco.HasPackages() {
		items = append(items, menuItem{label: "Browse packages", action: MENU_BROWSE_PACKAGES})
	}
	if updateAvailable {
		items = append(items, menuItem{label: fmt.Sprintf("Update to %s", latestVersion), action: MENU_UPDATE})
	}
	items = append(items, menuItem{label: "Settings", action: MENU_SETTINGS})
	return items
}

func NewWelcome(width, height int) WelcomeModel {
	dir, err := os.Getwd()
	if err != nil {
		dir = "."
	}
	eco := ecosystem.Detect(dir)
	return WelcomeModel{
		width:     width,
		height:    height,
		cursor:    0,
		workDir:   dir,
		ecosystem: eco,
		menuItems: buildMenuItems(eco, false, false, ""),
	}
}

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
		ch := updater.Check(context.Background())
		info := <-ch
		return updateCheckMsg(info)
	}
}

func (m WelcomeModel) Init() tea.Cmd {
	return tea.Batch(tickCmd(), checkUpdateCmd(), detectGitWelcomeCmd(m.workDir))
}

func (m WelcomeModel) skipAnimation() WelcomeModel {
	m.currentLine = len(cli.LogoLines)
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

		if m.currentLine >= len(cli.LogoLines) {
			m.logoReady = true
			return m, uiRevealCmd()
		}

		lineLen := len([]rune(cli.LogoLines[m.currentLine]))
		m.charPos++

		if m.charPos >= lineLen {
			m.charPos = 0
			m.currentLine++
		}

		return m, tickCmd()

	case updateCheckMsg:
		m.updateInfo = updater.UpdateInfo(msg)
		m.updateReady = true
		m.menuItems = buildMenuItems(m.ecosystem, m.hasGit, m.updateInfo.Available, m.updateInfo.LatestVersion)

	case gitDetectWelcomeMsg:
		m.hasGit = msg.hasGit
		m.menuItems = buildMenuItems(m.ecosystem, m.hasGit, m.updateInfo.Available, m.updateInfo.LatestVersion)

	case uiRevealMsg:
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
			m.enterPressed = true
		case "u":
			for i, item := range m.menuItems {
				if item.action == MENU_UPDATE {
					m.cursor = i
					m.enterPressed = true
					break
				}
			}
		}
	}

	return m, nil
}

func hyperlink(url, text string) string {
	return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", url, text)
}

func renderLogoLine(lineIdx int, visible int) string {
	runes := []rune(cli.LogoLines[lineIdx])
	total := len(runes)
	if visible > total {
		visible = total
	}

	color := cli.LogoGradient[lineIdx]
	rendered := lipgloss.NewStyle().Foreground(color).Bold(true).Render(string(runes[:visible]))

	return rendered + strings.Repeat(" ", total-visible)
}

func (m WelcomeModel) View() string {
	var sb strings.Builder

	sb.WriteString("\n")

	for i := range cli.LogoLines {
		var visible int
		switch {
		case i < m.currentLine:
			visible = len([]rune(cli.LogoLines[i]))
		case i == m.currentLine:
			visible = m.charPos
		default:
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

	if m.updateReady && m.updateInfo.Available {
		msg := fmt.Sprintf(
			"  Hey, a new version is available (%s) — press [u] to KAPI !",
			m.updateInfo.LatestVersion,
		)
		sb.WriteString(styles.SuccessStyle.Render(msg) + "\n")
		sb.WriteString("\n")
	}

	for i, item := range m.menuItems {
		if i == m.cursor {
			cursor := styles.CursorStyle.Render("  ❯❯")
			label := styles.SelectedStyle.Render(" " + item.label)
			sb.WriteString(fmt.Sprintf("%s%s\n", cursor, label))
		} else {
			sb.WriteString(fmt.Sprintf("      %s\n", styles.DimStyle.Render(item.label)))
		}
	}

	sb.WriteString("\n")

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

func (m WelcomeModel) IsGitConfigSelected() bool {
	return m.enterPressed && m.currentAction() == MENU_GIT_CONFIG
}

func (m WelcomeModel) IsBrowsePackagesSelected() bool {
	return m.enterPressed && m.currentAction() == MENU_BROWSE_PACKAGES
}

func (m WelcomeModel) IsUpdateSelected() bool {
	return m.enterPressed && m.currentAction() == MENU_UPDATE
}

func (m WelcomeModel) IsSettingsSelected() bool {
	return m.enterPressed && m.currentAction() == MENU_SETTINGS
}

func (m WelcomeModel) WorkDir() string { return m.workDir }

func (m WelcomeModel) Ecosystem() ecosystem.Ecosystem { return m.ecosystem }

func (m WelcomeModel) LatestVersion() string { return m.updateInfo.LatestVersion }

func (m *WelcomeModel) ConsumeEnter() {
	m.enterPressed = false
}
