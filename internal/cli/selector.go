package cli

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slouowzee/kapi/tui/styles"
)

func scrollWindow(cursor, total, visible int) (start, end int) {
	start = cursor - visible/2
	if start < 0 {
		start = 0
	}
	if start+visible > total {
		start = total - visible
	}
	if start < 0 {
		start = 0
	}
	end = start + visible
	if end > total {
		end = total
	}
	return
}

type selectorModel struct {
	title   string
	choices []string
	cursor  int
	chosen  string
	warn    string
}

func (m selectorModel) Init() tea.Cmd {
	return nil
}

func (m selectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter":
			m.chosen = m.choices[m.cursor]
			return m, tea.Quit
		}
	}
	return m, nil
}

const selectorVisible = 9

func (m selectorModel) View() string {
	if m.chosen != "" {
		return ""
	}

	total := len(m.choices)
	visible := selectorVisible
	if visible > total {
		visible = total
	}

	windowStart, windowEnd := scrollWindow(m.cursor, total, visible)

	var sb strings.Builder
	sb.WriteString("\n  " + styles.TitleStyle.Render(m.title) + "\n\n")
	if m.warn != "" {
		sb.WriteString("  " + styles.ErrorStyle.Render(m.warn) + "\n\n")
	}

	if windowStart > 0 {
		sb.WriteString("    " + styles.DimStyle.Render(fmt.Sprintf("↑ %d more", windowStart)) + "\n")
	}

	for i, choice := range m.choices[windowStart:windowEnd] {
		absIdx := windowStart + i
		if absIdx == m.cursor {
			sb.WriteString("  " + styles.SelectedStyle.Render("❯ "+choice) + "\n")
		} else {
			sb.WriteString("    " + styles.DimStyle.Render(choice) + "\n")
		}
	}

	if windowEnd < total {
		sb.WriteString("    " + styles.DimStyle.Render(fmt.Sprintf("↓ %d more", total-windowEnd)) + "\n")
	}

	sb.WriteString("\n  " + styles.DimStyle.Render("↑/↓: navigate • enter: select • esc: quit") + "\n\n")
	return sb.String()
}

func PromptChoice(title string, choices []string, warn ...string) (int, error) {
	m := selectorModel{
		title:   title,
		choices: choices,
	}
	if len(warn) > 0 {
		m.warn = warn[0]
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return -1, err
	}

	finalM := finalModel.(selectorModel)
	if finalM.chosen == "" {
		return -1, fmt.Errorf("aborted")
	}

	for i, c := range choices {
		if c == finalM.chosen {
			return i, nil
		}
	}
	return -1, fmt.Errorf("aborted")
}
