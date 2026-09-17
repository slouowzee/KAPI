package screens

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slouowzee/kapi/internal/cli"
	"github.com/slouowzee/kapi/internal/config"
	"github.com/slouowzee/kapi/internal/packagemanager"
	"github.com/slouowzee/kapi/tui/styles"
)

type settingsStep int

const (
	SettingsStepMenu settingsStep = iota
	SettingsStepPM
	SettingsStepToken
)

const (
	settingsItemPM = iota
	settingsItemToken
	settingsItemCount
)

func saveTokenCmd(token string) tea.Cmd {
	return func() tea.Msg {
		err := config.Update(func(cfg *config.Config) error {
			cfg.GithubToken = token
			return nil
		})
		return settingsTokenSavedMsg{err: err, cleared: token == ""}
	}
}

type settingsTokenSavedMsg struct {
	err     error
	cleared bool
}

type settingsSavedMsg struct{ err error }
type settingsInstalledPMsMsg struct{ pms []packagemanager.PM }

func detectInstalledPMsCmd() tea.Cmd {
	return func() tea.Msg {
		return settingsInstalledPMsMsg{pms: packagemanager.DetectInstalled()}
	}
}

func savePMCmd(pm packagemanager.PM) tea.Cmd {
	return func() tea.Msg {
		err := config.Update(func(cfg *config.Config) error {
			cfg.PackageManager = pm.String()
			return nil
		})
		return settingsSavedMsg{err: err}
	}
}

type SettingsModel struct {
	width  int
	height int

	step settingsStep

	currentPM packagemanager.PM

	menuCursor   int
	savedToken   string
	tokenInput   string
	tokenFromEnv bool

	installedPMs []packagemanager.PM
	pmsDetected  bool

	pmCursor int

	lastMsg string
	lastErr error

	backPressed bool
}

func NewSettings(width, height int) SettingsModel {
	cfg, err := config.Load()
	m := SettingsModel{
		width:        width,
		height:       height,
		step:         SettingsStepMenu,
		currentPM:    packagemanager.Parse(cfg.PackageManager),
		savedToken:   cfg.GithubToken,
		tokenFromEnv: os.Getenv("GITHUB_TOKEN") != "",
	}
	// NOTE: writing to stderr would corrupt the alternate screen.
	if err != nil {
		m.lastErr = err
		m.lastMsg = "Could not load config: " + err.Error()
	}
	return m
}

func (m *SettingsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m SettingsModel) IsBack() bool                 { return m.backPressed }
func (m *SettingsModel) ConsumeBack()                { m.backPressed = false }
func (m SettingsModel) CurrentPM() packagemanager.PM { return m.currentPM }

// IsInputMode reports whether the user is typing, so q must not quit.
func (m SettingsModel) IsInputMode() bool { return m.step == SettingsStepToken }

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
		m.step = SettingsStepMenu

	case settingsTokenSavedMsg:
		m.step = SettingsStepMenu
		switch {
		case msg.err != nil:
			m.lastErr = msg.err
			m.lastMsg = "Failed to save: " + msg.err.Error()
		case msg.cleared:
			m.lastErr = nil
			m.lastMsg = "GitHub token removed."
		default:
			m.lastErr = nil
			m.lastMsg = "GitHub token saved."
		}

	case tea.KeyMsg:
		switch m.step {
		case SettingsStepMenu:
			return m.handleMenuKey(msg)
		case SettingsStepPM:
			return m.handlePMKey(msg)
		case SettingsStepToken:
			return m.handleTokenKey(msg)
		}
	}

	return m, nil
}

func (m SettingsModel) handleMenuKey(msg tea.KeyMsg) (SettingsModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.backPressed = true
	case "up", "k":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
	case "down", "j":
		if m.menuCursor < settingsItemCount-1 {
			m.menuCursor++
		}
	case "enter":
		switch m.menuCursor {
		case settingsItemPM:
			if !m.pmsDetected {
				break
			}
			m.lastMsg = ""
			m.lastErr = nil
			m.pmCursor = m.pmCursorFor(m.currentPM)
			m.step = SettingsStepPM
		case settingsItemToken:
			m.lastMsg = ""
			m.lastErr = nil
			m.tokenInput = ""
			m.step = SettingsStepToken
		}
	}
	return m, nil
}

func (m SettingsModel) handleTokenKey(msg tea.KeyMsg) (SettingsModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.tokenInput = ""
		m.step = SettingsStepMenu
	case tea.KeyEnter:
		token := strings.TrimSpace(m.tokenInput)
		m.tokenInput = ""
		m.savedToken = token
		return m, saveTokenCmd(token)
	case tea.KeyBackspace:
		if runes := []rune(m.tokenInput); len(runes) > 0 {
			m.tokenInput = string(runes[:len(runes)-1])
		}
	case tea.KeyCtrlU:
		m.tokenInput = ""
	case tea.KeyRunes, tea.KeySpace:
		m.tokenInput += string(msg.Runes)
	}
	return m, nil
}

func (m SettingsModel) handlePMKey(msg tea.KeyMsg) (SettingsModel, tea.Cmd) {
	choices := m.pmChoices()
	switch msg.String() {
	case "esc":
		m.lastMsg = ""
		m.lastErr = nil
		m.step = SettingsStepMenu
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
	case SettingsStepPM:
		return m.viewPM()
	case SettingsStepToken:
		return m.viewToken()
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
		pmLabel += "  detecting…"
	}
	tokenLabel := "not set"
	if m.savedToken != "" {
		tokenLabel = cli.MaskToken(m.savedToken)
	}

	items := []struct{ name, value string }{
		{name: "Package manager", value: pmLabel},
		{name: "GitHub token", value: tokenLabel},
	}
	for i, item := range items {
		label := fmt.Sprintf("%-18s%s", item.name, styles.DimStyle.Render(item.value+" ›"))
		if i == m.menuCursor {
			fmt.Fprintf(&sb, "%s%s\n", styles.CursorStyle.Render("  ❯❯"), styles.SelectedStyle.Render(" "+label))
		} else {
			fmt.Fprintf(&sb, "      %s\n", styles.MutedStyle.Render(label))
		}
	}

	sb.WriteString("\n")
	sb.WriteString(styles.MutedStyle.Render("  [↑↓] navigate   [↵] select   [esc] back   [q] quit") + "\n")
	return sb.String()
}

func (m SettingsModel) viewToken() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(styles.TitleStyle.Render("  GitHub token") + "\n")
	sb.WriteString(styles.DimStyle.Render("  Classic token with the repo, write:ssh_signing_key and write:gpg_key scopes.") + "\n")
	sb.WriteString(styles.DimStyle.Render("  Leave empty and confirm to remove the saved token.") + "\n")
	if m.tokenFromEnv {
		sb.WriteString(styles.SubtitleStyle.Render("  ⚠ GITHUB_TOKEN is set in your environment and takes precedence.") + "\n")
	}
	sb.WriteString("\n")

	masked := strings.Repeat("•", len([]rune(m.tokenInput)))
	fmt.Fprintf(&sb, "  %s%s%s\n", styles.MutedStyle.Render("Token: "), masked, styles.TitleStyle.Render("_"))

	sb.WriteString("\n")
	sb.WriteString(styles.MutedStyle.Render("  [↵] save   [ctrl+u] clear   [esc] cancel") + "\n")
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
