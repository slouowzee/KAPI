package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slouowzee/kapi/internal/ecosystem"
	"github.com/slouowzee/kapi/tui/styles"
)

type ecosystemItem struct {
	label string
	desc  string
	eco   ecosystem.Ecosystem
}

var ecosystemItems = []ecosystemItem{
	{label: "PHP", desc: "Laravel, Symfony, WordPress and more", eco: ecosystem.ECOSYSTEM_PHP},
	{label: "JS/TS", desc: "React, Vue, Next.js and more", eco: ecosystem.ECOSYSTEM_JS},
}

type EcosystemModel struct {
	width  int
	height int

	cursor int

	targetDir    string
	targetDirEco ecosystem.Ecosystem

	selected ecosystem.Ecosystem
	done     bool

	backPressed bool
}

func NewEcosystem(width, height int, targetDir string) EcosystemModel {
	return EcosystemModel{
		width:        width,
		height:       height,
		targetDir:    targetDir,
		targetDirEco: ecosystem.Detect(targetDir),
		selected:     ecosystem.ECOSYSTEM_NONE,
	}
}

func (m *EcosystemModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m EcosystemModel) SelectedEcosystem() ecosystem.Ecosystem { return m.selected }
func (m EcosystemModel) Done() bool                             { return m.done }

func (m EcosystemModel) IsBack() bool  { return m.backPressed }
func (m *EcosystemModel) ConsumeBack() { m.backPressed = false }
func (m *EcosystemModel) ConsumeDone() { m.done = false }

func (m EcosystemModel) Init() tea.Cmd { return nil }

func (m EcosystemModel) Update(msg tea.Msg) (EcosystemModel, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(ecosystemItems)-1 {
				m.cursor++
			}
		case "enter":
			if m.cursor >= 0 && m.cursor < len(ecosystemItems) {
				m.selected = ecosystemItems[m.cursor].eco
			}
			m.done = true
		case "esc":
			m.backPressed = true
		}
	}

	return m, nil
}

func (m EcosystemModel) View() string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(styles.TitleStyle.Render("  Which ecosystem are you targeting?") + "\n")
	subtitle := styles.DimStyle.Render("  " + truncatePath(m.targetDir))
	if m.targetDirEco != ecosystem.ECOSYSTEM_NONE {
		subtitle += "  " + styles.SuccessStyle.Render(m.targetDirEco.Label())
	}
	sb.WriteString(subtitle + "\n")
	sb.WriteString("\n")

	for i, item := range ecosystemItems {
		if i == m.cursor {
			cur := styles.CursorStyle.Render("  ❯❯")
			label := styles.SelectedStyle.Render(" " + item.label)
			desc := styles.DimStyle.Render("   " + item.desc)
			fmt.Fprintf(&sb, "%s%s%s\n", cur, label, desc)
		} else {
			fmt.Fprintf(&sb, "      %s\n", styles.DimStyle.Render(item.label))
		}
	}

	sb.WriteString("\n")
	sb.WriteString(styles.MutedStyle.Render("  [↑↓] navigate   [↵] select   [esc] back   [q] quit") + "\n")

	return sb.String()
}
