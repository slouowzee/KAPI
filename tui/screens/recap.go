package screens

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slouowzee/kapi/internal/packagemanager"
	"github.com/slouowzee/kapi/internal/packages"
	"github.com/slouowzee/kapi/internal/registry"
	"github.com/slouowzee/kapi/tui/styles"
)

type RecapSection int

const (
	RECAP_SECTION_FOLDER RecapSection = iota
	RECAP_SECTION_FRAMEWORK
	RECAP_SECTION_PACKAGES
	RECAP_SECTION_GIT
	RECAP_SECTION_PM
	RECAP_SECTION_CONFIRM
	RECAP_SECTION_ABANDON
)

type RecapModel struct {
	width  int
	height int

	dir       string
	framework registry.Framework
	pkgs      []packages.Package
	gitCfg    GitConfig
	pm        packagemanager.PM

	cursor int

	done           bool
	backSection    RecapSection
	backPressed    bool
	abandonPressed bool
	abandonPending bool
}

type RecapSummary struct {
	Dir       string
	Framework registry.Framework
	Pkgs      []packages.Package
	GitCfg    GitConfig
	PM        packagemanager.PM
}

func NewRecap(width, height int, s RecapSummary) RecapModel {
	return RecapModel{
		width:     width,
		height:    height,
		dir:       s.Dir,
		framework: s.Framework,
		pkgs:      s.Pkgs,
		gitCfg:    s.GitCfg,
		pm:        s.PM,
		cursor:    int(RECAP_SECTION_CONFIRM),
	}
}

func (m *RecapModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m RecapModel) Done() bool             { return m.done }
func (m *RecapModel) ConsumeDone()          { m.done = false }
func (m RecapModel) IsAbandoned() bool      { return m.abandonPressed }
func (m *RecapModel) ConsumeAbandon()       { m.abandonPressed = false }
func (m RecapModel) IsAbandonPending() bool { return m.abandonPending }

func (m RecapModel) IsBack() bool { return m.backPressed }

func (m RecapModel) BackSection() RecapSection { return m.backSection }

func (m *RecapModel) ConsumeBack() { m.backPressed = false }

func (m RecapModel) Init() tea.Cmd { return nil }

func (m RecapModel) Update(msg tea.Msg) (RecapModel, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.abandonPending {
			switch msg.String() {
			case "y", "Y":
				m.abandonPressed = true
			case "n", "N", "esc":
				m.abandonPending = false
			}
			break
		}

		maxCursor := int(RECAP_SECTION_ABANDON)
		if m.framework.Ecosystem == "php" {
			maxCursor = int(RECAP_SECTION_ABANDON) - 1
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.framework.Ecosystem == "php" && m.cursor == int(RECAP_SECTION_PM) {
					m.cursor--
				}
			}
		case "down", "j":
			if m.cursor < maxCursor {
				m.cursor++
				if m.framework.Ecosystem == "php" && m.cursor == int(RECAP_SECTION_PM) {
					m.cursor++
				}
			}
		case "enter":
			sec := RecapSection(m.cursor)
			switch sec {
			case RECAP_SECTION_CONFIRM:
				m.done = true
			case RECAP_SECTION_ABANDON:
				m.abandonPending = true
			default:
				m.backSection = sec
				m.backPressed = true
			}
		case "esc":
			m.backSection = RECAP_SECTION_GIT
			m.backPressed = true
		}
	}
	return m, nil
}

func (m RecapModel) View() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(styles.TitleStyle.Render("  Summary") + "\n")
	sb.WriteString(styles.DimStyle.Render("  Review your project configuration before scaffolding.") + "\n")
	sb.WriteString("\n")

	isPhp := m.framework.Ecosystem == "php"

	rows := []recapRow{
		{section: RECAP_SECTION_FOLDER, label: "Directory", value: truncatePath(m.dir)},
		{section: RECAP_SECTION_FRAMEWORK, label: "Framework", value: m.framework.Name},
		{section: RECAP_SECTION_PACKAGES, label: "Packages", value: m.packagesValue()},
		{section: RECAP_SECTION_GIT, label: "Git", value: m.gitValue()},
	}
	if !isPhp {
		rows = append(rows, recapRow{section: RECAP_SECTION_PM, label: "Package manager", value: pmDisplayLabel(m.pm)})
	}
	rows = append(rows,
		recapRow{section: RECAP_SECTION_CONFIRM, label: "", value: ""},
		recapRow{section: RECAP_SECTION_ABANDON, label: "", value: ""},
	)

	for _, row := range rows {
		isCursor := m.cursor == int(row.section)

		if row.section == RECAP_SECTION_CONFIRM {
			sb.WriteString("\n")
			if isCursor {
				fmt.Fprintf(&sb, "%s%s\n",
					styles.CursorStyle.Render("  ❯❯"),
					styles.SelectedStyle.Render(" Confirm and scaffold"),
				)
			} else {
				fmt.Fprintf(&sb, "      %s\n", styles.DimStyle.Render("Confirm and scaffold"))
			}
			continue
		}

		if row.section == RECAP_SECTION_ABANDON {
			if m.abandonPending {
				fmt.Fprintf(&sb, "%s%s\n",
					styles.CursorStyle.Render("  ❯❯"),
					styles.ErrorStyle.Render(" Abandon project? All data will be lost. [y] yes  [n / esc] cancel"),
				)
			} else if isCursor {
				fmt.Fprintf(&sb, "%s%s\n",
					styles.CursorStyle.Render("  ❯❯"),
					styles.DimStyle.Render(" Abandon and return to menu"),
				)
			} else {
				fmt.Fprintf(&sb, "      %s\n", styles.DimStyle.Render("Abandon and return to menu"))
			}
			continue
		}

		label := fmt.Sprintf("%-17s", row.label)
		if isCursor {
			val := row.value + styles.DimStyle.Render("  ✎ edit")
			fmt.Fprintf(&sb, "%s%s\n",
				styles.CursorStyle.Render("  ❯❯"),
				styles.SelectedStyle.Render(" "+label+val),
			)
		} else {
			fmt.Fprintf(&sb, "      %s%s\n",
				styles.MutedStyle.Render(label),
				styles.DimStyle.Render(row.value),
			)
		}
	}

	sb.WriteString("\n")
	if m.abandonPending {
		sb.WriteString(styles.MutedStyle.Render("  [y] confirm abandon   [n / esc] cancel") + "\n")
	} else {
		sb.WriteString(styles.MutedStyle.Render("  [↑↓] navigate   [↵] select / confirm   [esc] back   [q] quit") + "\n")
	}
	return sb.String()
}

type recapRow struct {
	section RecapSection
	label   string
	value   string
}

func (m RecapModel) packagesValue() string {
	if len(m.pkgs) == 0 {
		return "none"
	}
	names := make([]string, len(m.pkgs))
	for i, p := range m.pkgs {
		names[i] = p.Name
	}
	if len(names) > 3 {
		return strings.Join(names[:3], ", ") + fmt.Sprintf(" +%d more", len(names)-3)
	}
	return strings.Join(names, ", ")
}

func (m RecapModel) gitValue() string {
	if !m.gitCfg.InitLocal && m.gitCfg.RemoteHost == "" {
		return "none"
	}
	var parts []string
	if m.gitCfg.InitLocal || m.gitCfg.HasExistingGit {
		parts = append(parts, "local")
	}
	if m.gitCfg.UniversalGitignore {
		parts = append(parts, "universal gitignore")
	}
	if m.gitCfg.InitialCommit {
		parts = append(parts, "initial commit")
	}
	switch m.gitCfg.RemoteHost {
	case "github":
		v := "github"
		if m.gitCfg.RemotePrivate {
			v += " (private)"
		} else {
			v += " (public)"
		}
		if m.gitCfg.RepoName != "" {
			v += ": " + m.gitCfg.RepoName
		}
		parts = append(parts, v)
	case "":
	default:
		if m.gitCfg.RemoteURL != "" {
			parts = append(parts, filepath.Base(m.gitCfg.RemoteURL))
		}
	}
	if m.gitCfg.Collab {
		parts = append(parts, "collab")
	}
	if m.gitCfg.CI != "" && m.gitCfg.CI != ciChoiceNone {
		parts = append(parts, m.gitCfg.CI+" CI")
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, "  ·  ")
}
