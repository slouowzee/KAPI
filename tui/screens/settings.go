package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slouowzee/kapi/internal/config"
	"github.com/slouowzee/kapi/internal/packagemanager"
	"github.com/slouowzee/kapi/tui/styles"
)

type settingsStep int

const (
	SETTINGS_STEP_MENU settingsStep = iota
	SETTINGS_STEP_PM
)

type settingsSavedMsg struct{ err error }
type settingsInstalledPMsMsg struct{ pms []packagemanager.PM }

func detectInstalledPMsCmd() tea.Cmd {
	return func() tea.Msg {
		return settingsInstalledPMsMsg{pms: packagemanager.DetectInstalled()}
	}
}

func savePMCmd(pm packagemanager.PM) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return settingsSavedMsg{err: err}
		}
		cfg.PackageManager = pm.String()
		return settingsSavedMsg{err: config.Save(cfg)}
	}
}

type SettingsModel struct {
	width  int
	height int

	step settingsStep

	currentPM packagemanager.PM

	installedPMs []packagemanager.PM
	pmsDetected  bool

	pmCursor int

	lastMsg string
	lastErr error

	backPressed bool
}

func NewSettings(width, height int) SettingsModel {
	cfg, _ := config.Load()
	return SettingsModel{
		width:     width,
		height:    height,
		step:      SETTINGS_STEP_MENU,
		currentPM: packagemanager.Parse(cfg.PackageManager),
	}
}

func (m *SettingsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m SettingsModel) IsBack() bool                 { return m.backPressed }
func (m *SettingsModel) ConsumeBack()                { m.backPressed = false }
func (m SettingsModel) CurrentPM() packagemanager.PM { return m.currentPM }

func (m SettingsModel) Init() tea.Cmd {
	return detectInstalledPMsCmd()
}

func (m SettingsModel) Update(msg tea.Msg) (SettingsModel, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case settingsInstalledPMsMsg:
		m.installedPMs = msg.pms
		m.pmsDetected = true
		m.pmCursor = m.pmCursorFor(m.currentPM)

	case settingsSavedMsg:
		if msg.err != nil {
			m.lastErr = msg.err
			m.lastMsg = "Failed to save: " + msg.err.Error()
		} else {
			m.lastErr = nil
			if m.currentPM == packagemanager.None {
				m.lastMsg = "Preference cleared."
			} else {
				m.lastMsg = "Saved."
			}
		}
		m.step = SETTINGS_STEP_MENU

	case tea.KeyMsg:
		switch m.step {
		case SETTINGS_STEP_MENU:
			return m.handleMenuKey(msg)
		case SETTINGS_STEP_PM:
			return m.handlePMKey(msg)
		}
	}

	return m, nil
}

func (m SettingsModel) handleMenuKey(msg tea.KeyMsg) (SettingsModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.backPressed = true
	case "enter":
		if !m.pmsDetected {
			break
		}
		m.lastMsg = ""
		m.lastErr = nil
		m.pmCursor = m.pmCursorFor(m.currentPM)
		m.step = SETTINGS_STEP_PM
	}
	return m, nil
}

func (m SettingsModel) handlePMKey(msg tea.KeyMsg) (SettingsModel, tea.Cmd) {
	choices := m.pmChoices()
	switch msg.String() {
	case "esc":
		m.lastMsg = ""
		m.lastErr = nil
		m.step = SETTINGS_STEP_MENU
	case "up", "k":
		if m.pmCursor > 0 {
			m.pmCursor--
		}
	case "down", "j":
		if m.pmCursor < len(choices)-1 {
			m.pmCursor++
		}
	case "enter":
		chosen := choices[m.pmCursor]
		m.currentPM = chosen
		return m, savePMCmd(chosen)
	}
	return m, nil
}

func (m SettingsModel) pmChoices() []packagemanager.PM {
	var base []packagemanager.PM
	if m.pmsDetected && len(m.installedPMs) > 0 {
		base = m.installedPMs
	} else {
		base = packagemanager.All()
	}
	return append([]packagemanager.PM{packagemanager.None}, base...)
}

func (m SettingsModel) pmCursorFor(pm packagemanager.PM) int {
	for i, p := range m.pmChoices() {
		if p == pm {
			return i
		}
	}
	return 0
}

func (m SettingsModel) View() string {
	switch m.step {
	case SETTINGS_STEP_PM:
		return m.viewPM()
	default:
		return m.viewMenu()
	}
}

func (m SettingsModel) viewMenu() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(styles.TitleStyle.Render("  Settings") + "\n")
	sb.WriteString("\n")

	if m.lastMsg != "" {
		if m.lastErr != nil {
			sb.WriteString(styles.ErrorStyle.Render("  "+m.lastMsg) + "\n\n")
		} else {
			sb.WriteString(styles.SuccessStyle.Render("  "+m.lastMsg) + "\n\n")
		}
	}

	pmLabel := pmDisplayLabel(m.currentPM)
	if !m.pmsDetected {
		pmLabel += styles.DimStyle.Render("  detecting…")
	}
	label := fmt.Sprintf("Package manager   %s", styles.DimStyle.Render(pmLabel+" ›"))
	cursor := styles.CursorStyle.Render("  ❯❯")
	sb.WriteString(fmt.Sprintf("%s%s\n", cursor, styles.SelectedStyle.Render(" "+label)))

	sb.WriteString("\n")
	sb.WriteString(styles.MutedStyle.Render("  [↵] select   [esc] back   [q] quit") + "\n")
	return sb.String()
}

func (m SettingsModel) viewPM() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(styles.TitleStyle.Render("  Package manager") + "\n")

	choices := m.pmChoices()
	if m.pmsDetected && len(m.installedPMs) > 0 {
		sb.WriteString(styles.DimStyle.Render("  Showing package managers installed on this machine.") + "\n")
	} else if m.pmsDetected {
		sb.WriteString(styles.DimStyle.Render("  No package manager detected — showing all options.") + "\n")
	} else {
		sb.WriteString(styles.DimStyle.Render("  Detecting…") + "\n")
	}
	sb.WriteString("\n")

	for i, pm := range choices {
		isSelected := pm == m.currentPM
		isCursor := i == m.pmCursor

		lbl := pmDisplayLabel(pm)
		if isSelected {
			lbl += styles.SuccessStyle.Render("  ✓ current")
		}

		var line string
		if isCursor {
			cur := styles.CursorStyle.Render("  ❯❯")
			line = fmt.Sprintf("%s%s\n", cur, styles.SelectedStyle.Render(" "+lbl))
		} else {
			line = fmt.Sprintf("      %s\n", styles.DimStyle.Render(lbl))
		}
		sb.WriteString(line)
	}

	sb.WriteString("\n")
	sb.WriteString(styles.MutedStyle.Render("  [↑↓] navigate   [↵] select   [esc] back   [q] quit") + "\n")
	return sb.String()
}

func pmDisplayLabel(pm packagemanager.PM) string {
	if pm == packagemanager.None {
		return "No preference"
	}
	return pm.Label()
}
