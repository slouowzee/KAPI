package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slouowzee/kapi/tui/styles"
)

const (
	FOLDER_MODE_MENU = iota
	FOLDER_MODE_INPUT
	FOLDER_MODE_CONFIRM
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

	workDir string

	input       string
	inputCursor int
	suggestions []string
	sugCursor   int

	easterEggMsg string
	dangerMsg    string

	confirmPath   string
	confirmCursor int

	createError error

	selected string
	done     bool

	backPressed bool
}

func NewFolder(width, height int) FolderModel {
	dir, _ := os.Getwd()
	return FolderModel{
		width:   width,
		height:  height,
		workDir: dir,
	}
}

func (m *FolderModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SelectedDir returns the confirmed directory path.
// The caller should check Done() before using this.
func (m FolderModel) SelectedDir() string { return m.selected }
func (m FolderModel) Done() bool          { return m.done }

// IsInputMode returns true when the folder screen is in typing mode.
// The parent app uses this to avoid intercepting printable keys like q.
func (m FolderModel) IsInputMode() bool {
	return m.mode == FOLDER_MODE_INPUT
}

func (m FolderModel) IsBack() bool { return m.backPressed }

func (m *FolderModel) ConsumeBack() { m.backPressed = false }

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
				path := expandPath(m.input)
				if m.input == "" {
					break
				} else if !strings.ContainsRune(m.input, filepath.Separator) {
					break
				} else if info, err := os.Stat(path); err == nil && info.IsDir() {
					// Path exists — select it directly
					m.selected = path
					m.done = true
				} else {
					// Path does not exist — ask for confirmation to create it
					m.confirmPath = path
					m.confirmCursor = 0
					m.createError = nil
					m.mode = FOLDER_MODE_CONFIRM
				}
			case "esc":
				m.mode = FOLDER_MODE_MENU
				m.input = ""
				m.inputCursor = 0
				m.suggestions = nil
				m.easterEggMsg = ""
				m.dangerMsg = ""
			case "tab":
				// Apply the highlighted suggestion
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

		case FOLDER_MODE_CONFIRM:
			switch msg.String() {
			case "up", "k":
				if m.confirmCursor > 0 {
					m.confirmCursor--
				}
			case "down", "j":
				if m.confirmCursor < 1 {
					m.confirmCursor++
				}
			case "enter":
				if m.confirmCursor == 0 {
					if err := os.MkdirAll(m.confirmPath, 0755); err != nil {
						m.createError = err
					} else {
						m.selected = m.confirmPath
						m.done = true
					}
				} else {
					m.mode = FOLDER_MODE_INPUT
					m.input = m.confirmPath
					m.suggestions = listDirs(m.input)
					m.sugCursor = 0
					m.createError = nil
				}
			case "esc":
				m.mode = FOLDER_MODE_INPUT
				m.input = m.confirmPath
				m.suggestions = listDirs(m.input)
				m.sugCursor = 0
				m.createError = nil
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
		label0 := fmt.Sprintf("Use current directory   %s", styles.DimStyle.Render("("+truncatePath(m.workDir)+")"))
		items := []string{
			label0,
			"Enter a custom path",
		}
		for i, item := range items {
			if i == m.cursor {
				cur := lipgloss.NewStyle().Foreground(styles.COLOR_PRIMARY).Bold(true).Render("  ❯❯")
				label := styles.SelectedStyle.Render(" " + item)
				sb.WriteString(fmt.Sprintf("%s%s\n", cur, label))
			} else {
				sb.WriteString(fmt.Sprintf("      %s\n", styles.DimStyle.Render(item)))
			}
		}

	case FOLDER_MODE_INPUT:
		runes := []rune(m.input)
		before := string(runes[:m.inputCursor])
		after := string(runes[m.inputCursor:])
		inputLine := styles.MutedStyle.Render("  Path: ") +
			before + styles.TitleStyle.Render("_") + after
		if len(m.suggestions) > 0 {
			counter := styles.DimStyle.Render(fmt.Sprintf("  %d / %d", m.sugCursor+1, len(m.suggestions)))
			inputLine += counter
		}
		sb.WriteString(inputLine + "\n")

		// Inline validation feedback
		path := expandPath(m.input)
		if m.dangerMsg != "" {
			sb.WriteString(styles.ErrorStyle.Render("  ✗ "+m.dangerMsg) + "\n")
		} else if m.easterEggMsg != "" {
			sb.WriteString(styles.DimStyle.Render("  "+m.easterEggMsg) + "\n")
		} else if m.input != "" && !strings.ContainsRune(m.input, filepath.Separator) {
			sb.WriteString(styles.DimStyle.Render("  Use an absolute path starting with /") + "\n")
		} else if strings.ContainsRune(m.input, filepath.Separator) {
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				sb.WriteString(styles.SuccessStyle.Render("  ✓ valid directory") + "\n")
			} else if m.input != "" {
				sb.WriteString(styles.DimStyle.Render("  ↵ to create this directory") + "\n")
			}
		}

		// Suggestions list with a scrolling window of up to 9 visible entries.
		// The window follows the cursor so the selected item is always visible.
		if len(m.suggestions) > 0 {
			sb.WriteString("\n")

			const VISIBLE = 9
			total := len(m.suggestions)

			// Compute the start of the window so that sugCursor stays inside it.
			// We try to keep the cursor in the middle of the window when possible.
			windowStart := m.sugCursor - VISIBLE/2
			if windowStart < 0 {
				windowStart = 0
			}
			if windowStart+VISIBLE > total {
				windowStart = total - VISIBLE
			}
			if windowStart < 0 {
				windowStart = 0
			}
			windowEnd := windowStart + VISIBLE
			if windowEnd > total {
				windowEnd = total
			}

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

	case FOLDER_MODE_CONFIRM:
		sb.WriteString(styles.MutedStyle.Render("  Directory to create:") + "\n")
		sb.WriteString("  " + styles.TitleStyle.Render(m.confirmPath) + "\n")
		sb.WriteString("\n")
		sb.WriteString(styles.SubtitleStyle.Render("  This directory does not exist yet. Create it?") + "\n")
		sb.WriteString("\n")

		if m.createError != nil {
			sb.WriteString(styles.ErrorStyle.Render("  ✗ "+m.createError.Error()) + "\n")
			sb.WriteString("\n")
		}

		confirmItems := []string{"Yes, create it", "No, go back"}
		for i, item := range confirmItems {
			if i == m.confirmCursor {
				cur := lipgloss.NewStyle().Foreground(styles.COLOR_PRIMARY).Bold(true).Render("  ❯❯")
				sb.WriteString(fmt.Sprintf("%s%s\n", cur, styles.SelectedStyle.Render(" "+item)))
			} else {
				sb.WriteString(fmt.Sprintf("      %s\n", styles.DimStyle.Render(item)))
			}
		}
	}

	sb.WriteString("\n")

	// Hints bar
	var hints string
	switch m.mode {
	case FOLDER_MODE_MENU:
		hints = "  [↑↓] navigate   [↵] select   [esc] back   [q] quit"
	case FOLDER_MODE_INPUT:
		hints = "  [↑↓] suggestions   [tab] complete   [↵] confirm   [esc] back   [ctrl+c] quit"
	case FOLDER_MODE_CONFIRM:
		hints = "  [↑↓] navigate   [↵] confirm   [esc] back   [q] quit"
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
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// listDirs returns subdirectory names that match the current input prefix,
// used to power the tab-completion suggestions.
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
