package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slouowzee/kapi/internal/ecosystem"
	"github.com/slouowzee/kapi/tui/styles"
)

const (
	FOLDER_MODE_MENU = iota
	FOLDER_MODE_INPUT
)

const (
	MENU_ITEM_CURRENT = 0
	MENU_ITEM_CUSTOM  = 1
)

type FolderModel struct {
	width  int
	height int

	mode   int
	cursor int

	workDir    string
	workDirEco ecosystem.Ecosystem

	input       string
	inputCursor int
	suggestions []string
	sugCursor   int

	easterEggMsg string
	dangerMsg    string

	selected string
	done     bool

	backPressed bool
	directInput bool
}

func Folder(width, height int, path string) FolderModel {
	dir, err := os.Getwd()
	if err != nil {
		dir = "."
	}
	if path == "" {
		return FolderModel{
			width:      width,
			height:     height,
			workDir:    dir,
			workDirEco: ecosystem.Detect(dir),
		}
	}
	input := path + string(filepath.Separator)
	return FolderModel{
		width:       width,
		height:      height,
		workDir:     dir,
		workDirEco:  ecosystem.Detect(dir),
		mode:        FOLDER_MODE_INPUT,
		input:       input,
		inputCursor: len([]rune(input)),
		suggestions: listDirs(input),
		directInput: true,
	}
}

func (m *FolderModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m FolderModel) SelectedDir() string { return m.selected }
func (m FolderModel) Done() bool          { return m.done }

func (m FolderModel) IsInputMode() bool { return m.mode == FOLDER_MODE_INPUT }
func (m FolderModel) IsBack() bool      { return m.backPressed }
func (m *FolderModel) ConsumeBack()     { m.backPressed = false }
func (m *FolderModel) ConsumeDone()     { m.done = false }

func (m FolderModel) Init() tea.Cmd { return nil }

func (m FolderModel) Update(msg tea.Msg) (FolderModel, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch m.mode {

		case FOLDER_MODE_MENU:
			switch msg.String() {
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < 1 {
					m.cursor++
				}
			case "enter":
				switch m.cursor {
				case MENU_ITEM_CURRENT:
					m.selected = m.workDir
					m.done = true
				case MENU_ITEM_CUSTOM:
					m.enterInputMode()
				}
			case "esc":
				m.backPressed = true
			}

		case FOLDER_MODE_INPUT:
			switch msg.String() {
			case "enter":
				if m.dangerMsg != "" {
					break
				}
				if m.input == "" {
					break
				}
				if !strings.ContainsRune(m.input, filepath.Separator) {
					break
				}
				m.selected = expandPath(m.input)
				m.done = true
			case "esc":
				if m.directInput {
					m.backPressed = true
					break
				}
				m.mode = FOLDER_MODE_MENU
				m.input = ""
				m.inputCursor = 0
				m.suggestions = nil
				m.easterEggMsg = ""
				m.dangerMsg = ""
			case "tab":
				if len(m.suggestions) > 0 {
					m.input = m.suggestions[m.sugCursor] + string(filepath.Separator)
					m.inputCursor = len([]rune(m.input))
					m.suggestions = listDirs(m.input)
					m.sugCursor = 0
					m.easterEggMsg = detectEasterEgg(m.input)
					m.dangerMsg = isDangerous(m.input)
				}
			case "left":
				if m.inputCursor > 0 {
					m.inputCursor--
				}
			case "right":
				runes := []rune(m.input)
				if m.inputCursor < len(runes) {
					m.inputCursor++
				}
			case "up":
				if m.sugCursor > 0 {
					m.sugCursor--
				}
			case "down":
				if m.sugCursor < len(m.suggestions)-1 {
					m.sugCursor++
				}
			case "backspace":
				runes := []rune(m.input)
				if m.inputCursor > 0 {
					runes = append(runes[:m.inputCursor-1], runes[m.inputCursor:]...)
					m.inputCursor--
					m.input = string(runes)
					m.suggestions = listDirs(m.input)
					m.sugCursor = 0
					m.easterEggMsg = detectEasterEgg(m.input)
					m.dangerMsg = isDangerous(m.input)
				}
			case "ctrl+backspace", "ctrl+w":
				runes := []rune(m.input)
				pos := m.inputCursor
				if pos == 0 {
					break
				}
				if runes[pos-1] == filepath.Separator && pos > 1 {
					pos--
				}
				cut := pos - 1
				for cut > 0 && runes[cut] != filepath.Separator {
					cut--
				}
				if cut > 0 && runes[cut] == filepath.Separator {
					cut++
				} else {
					cut = 0
				}
				runes = append(runes[:cut], runes[m.inputCursor:]...)
				m.inputCursor = cut
				m.input = string(runes)
				m.suggestions = listDirs(m.input)
				m.sugCursor = 0
				m.easterEggMsg = detectEasterEgg(m.input)
				m.dangerMsg = isDangerous(m.input)
			default:
				if len(msg.Runes) > 0 {
					runes := []rune(m.input)
					runes = append(runes[:m.inputCursor], append(msg.Runes, runes[m.inputCursor:]...)...)
					m.inputCursor += len(msg.Runes)
					m.input = string(runes)
					m.suggestions = listDirs(m.input)
					m.sugCursor = 0
					m.easterEggMsg = detectEasterEgg(m.input)
					m.dangerMsg = isDangerous(m.input)
				}
			}
		}
	}

	return m, nil
}

func (m FolderModel) View() string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(styles.TitleStyle.Render("  Where should the project be created?") + "\n")
	sb.WriteString("\n")

	switch m.mode {

	case FOLDER_MODE_MENU:
		dirLabel := "(" + truncatePath(m.workDir) + ")"
		var ecoBadge string
		if m.workDirEco != ecosystem.ECOSYSTEM_NONE {
			ecoBadge = "  " + styles.SuccessStyle.Render(m.workDirEco.Label())
		}
		items := []string{
			"Use current directory",
			"Enter a custom path",
		}
		for i, item := range items {
			if i == m.cursor {
				cur := lipgloss.NewStyle().Foreground(styles.COLOR_PRIMARY).Bold(true).Render("  ❯❯")
				label := styles.SelectedStyle.Render(" " + item)
				if i == MENU_ITEM_CURRENT {
					label += styles.DimStyle.Render("   "+dirLabel) + ecoBadge
				}
				sb.WriteString(fmt.Sprintf("%s%s\n", cur, label))
			} else {
				line := item
				if i == MENU_ITEM_CURRENT {
					line += fmt.Sprintf("   %s%s", styles.DimStyle.Render(dirLabel), ecoBadge)
				}
				sb.WriteString(fmt.Sprintf("      %s\n", styles.DimStyle.Render(line)))
			}
		}

	case FOLDER_MODE_INPUT:
		runes := []rune(m.input)
		before := string(runes[:m.inputCursor])
		after := string(runes[m.inputCursor:])
		inputLine := styles.MutedStyle.Render("  Path: ") +
			before + styles.TitleStyle.Render("_") + after
		if len(m.suggestions) > 0 {
			inputLine += styles.DimStyle.Render(fmt.Sprintf("  %d / %d", m.sugCursor+1, len(m.suggestions)))
		}
		sb.WriteString(inputLine + "\n")

		path := expandPath(m.input)
		if m.dangerMsg != "" {
			sb.WriteString(styles.ErrorStyle.Render("  ✗ "+m.dangerMsg) + "\n")
		} else if m.easterEggMsg != "" {
			sb.WriteString(styles.DimStyle.Render("  "+m.easterEggMsg) + "\n")
		} else if m.input != "" && !strings.ContainsRune(m.input, filepath.Separator) {
			sb.WriteString(styles.DimStyle.Render("  Use an absolute path starting with /") + "\n")
		} else if strings.ContainsRune(m.input, filepath.Separator) {
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				sb.WriteString(styles.SuccessStyle.Render("  ✓ directory exists") + "\n")
				if eco := ecosystem.Detect(path); eco != ecosystem.ECOSYSTEM_NONE {
					sb.WriteString(styles.SubtitleStyle.Render("  ⚠ An existing "+eco.Label()+" was detected here") + "\n")
				}
			} else if m.input != "" {
				sb.WriteString(styles.DimStyle.Render("  ↵ to use this path") + "\n")
			}
		}

		if len(m.suggestions) > 0 {
			sb.WriteString("\n")

			const VISIBLE = 9
			total := len(m.suggestions)

			windowStart, windowEnd := scrollWindow(m.sugCursor, total, VISIBLE)

			if windowStart > 0 {
				sb.WriteString(styles.DimStyle.Render(fmt.Sprintf("      ↑ %d more", windowStart)) + "\n")
			}

			for i, sug := range m.suggestions[windowStart:windowEnd] {
				absIdx := windowStart + i
				name := filepath.Base(sug)
				if absIdx == m.sugCursor {
					cur := lipgloss.NewStyle().Foreground(styles.COLOR_PRIMARY).Bold(true).Render("  ❯❯")
					sb.WriteString(fmt.Sprintf("%s %s\n", cur, styles.SelectedStyle.Render(name)))
				} else {
					sb.WriteString(fmt.Sprintf("      %s\n", styles.DimStyle.Render(name)))
				}
			}

			if windowEnd < total {
				sb.WriteString(styles.DimStyle.Render(fmt.Sprintf("      ↓ %d more", total-windowEnd)) + "\n")
			}
		}
	}

	sb.WriteString("\n")

	var hints string
	switch m.mode {
	case FOLDER_MODE_MENU:
		hints = "  [↑↓] navigate   [↵] select   [esc] back   [q] quit"
	case FOLDER_MODE_INPUT:
		hints = "  [↑↓] suggestions   [tab] complete   [↵] confirm   [esc] back   [ctrl+c] quit"
	}
	sb.WriteString(styles.MutedStyle.Render(hints) + "\n")

	return sb.String()
}

func (m *FolderModel) enterInputMode() {
	m.mode = FOLDER_MODE_INPUT
	m.input = m.workDir + string(filepath.Separator)
	m.inputCursor = len([]rune(m.input))
	m.suggestions = listDirs(m.input)
	m.sugCursor = 0
	m.easterEggMsg = ""
	m.dangerMsg = ""
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		return abs
	}
	return path
}

func listDirs(input string) []string {
	if input == "" || !strings.ContainsRune(input, filepath.Separator) {
		return nil
	}

	dir := input
	prefix := ""
	if !strings.HasSuffix(input, string(filepath.Separator)) {
		dir = filepath.Dir(input)
		prefix = strings.ToLower(filepath.Base(input))
	}

	dir = expandPath(dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var results []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if strings.HasPrefix(strings.ToLower(e.Name()), ".") {
			continue
		}
		if prefix != "" && !strings.HasPrefix(strings.ToLower(e.Name()), prefix) {
			continue
		}
		results = append(results, filepath.Join(dir, e.Name()))
	}
	return results
}
