package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slouowzee/kapi/internal/packagemanager"
	"github.com/slouowzee/kapi/tui/styles"
)

type pmSelectInstalledMsg struct{ pms []packagemanager.PM }

func detectPMsForSelectCmd() tea.Cmd {
	return func() tea.Msg {
		return pmSelectInstalledMsg{pms: packagemanager.DetectInstalled()}
	}
}

type PMSelectModel struct {
	width  int
	height int

	choices     []packagemanager.PM
	detected    bool
	cursor      int
	preselected packagemanager.PM

	done        bool
	backPressed bool
}

func NewPMSelect(width, height int, preselected packagemanager.PM) PMSelectModel {
	return PMSelectModel{
		width:       width,
		height:      height,
		preselected: preselected,
	}
}

func (m *PMSelectModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m PMSelectModel) Done() bool    { return m.done }
func (m *PMSelectModel) ConsumeDone() { m.done = false }

func (m PMSelectModel) IsBack() bool  { return m.backPressed }
func (m *PMSelectModel) ConsumeBack() { m.backPressed = false }

func (m PMSelectModel) SelectedPM() packagemanager.PM {
	if len(m.choices) == 0 {
		return packagemanager.NPM
	}
	return m.choices[m.cursor]
}

func (m PMSelectModel) Init() tea.Cmd {
	return detectPMsForSelectCmd()
}

func (m PMSelectModel) Update(msg tea.Msg) (PMSelectModel, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case pmSelectInstalledMsg:
		m.detected = true
		if len(msg.pms) > 0 {
			m.choices = msg.pms
		} else {
			m.choices = packagemanager.All()
		}
		m.cursor = cursorForPM(m.choices, m.preselected)

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.backPressed = true
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.choices) > 0 {
				m.done = true
			}
		}
	}

	return m, nil
}

func (m PMSelectModel) View() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(styles.TitleStyle.Render("  Package manager") + "\n")

	if !m.detected {
		sb.WriteString(styles.DimStyle.Render("  Detecting…") + "\n")
		sb.WriteString("\n")
		sb.WriteString(styles.MutedStyle.Render("  [esc] back") + "\n")
		return sb.String()
	}

	if m.preselected != packagemanager.None {
		sb.WriteString(styles.DimStyle.Render(
			fmt.Sprintf("  Default: %s — confirm or pick another.", m.preselected.Label()),
		) + "\n")
	} else {
		sb.WriteString(styles.DimStyle.Render("  Choose a package manager for this project.") + "\n")
	}
	sb.WriteString("\n")

	for i, pm := range m.choices {
		isPreselected := pm == m.preselected
		isCursor := i == m.cursor

		var line string
		if isCursor {
			cur := styles.CursorStyle.Render("  ❯❯")
			lbl := pm.Label()
			if isPreselected {
				lbl += styles.DimStyle.Render("  (default)")
			}
			line = fmt.Sprintf("%s%s\n", cur, styles.SelectedStyle.Render(" "+lbl))
		} else {
			lbl := pm.Label()
			if isPreselected {
				lbl += styles.DimStyle.Render("  (default)")
			}
			line = fmt.Sprintf("      %s\n", styles.DimStyle.Render(lbl))
		}
		sb.WriteString(line)
	}

	sb.WriteString("\n")
	sb.WriteString(styles.MutedStyle.Render("  [↑↓] navigate   [↵] confirm   [esc] back   [q] quit") + "\n")
	return sb.String()
}

func cursorForPM(choices []packagemanager.PM, pm packagemanager.PM) int {
	if pm == packagemanager.None {
		return 0
	}
	for i, c := range choices {
		if c == pm {
			return i
		}
	}
	return 0
}
