package cli

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slouowzee/kapi/tui/styles"
)

type selectorModel struct {
	title   string
	choices []string
	cursor  int
	chosen  string
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
			} else {
				m.cursor = len(m.choices) - 1
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			} else {
				m.cursor = 0
			}
		case "enter":
			m.chosen = m.choices[m.cursor]
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m selectorModel) View() string {
	if m.chosen != "" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n  " + styles.TitleStyle.Render(m.title) + "\n\n")

	for i, choice := range m.choices {
		if i == m.cursor {
			sb.WriteString(styles.SelectedStyle.Render("  ❯ "+choice) + "\n")
		} else {
			sb.WriteString("    " + styles.DimStyle.Render(choice) + "\n")
		}
	}
	sb.WriteString("\n  " + styles.DimStyle.Render("↑/↓: navigate • enter: select • esc: quit") + "\n\n")
	return sb.String()
}

func PromptChoice(title string, choices []string) (int, error) {
	m := selectorModel{
		title:   title,
		choices: choices,
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
