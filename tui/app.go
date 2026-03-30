package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type Screen int

const (
	SCREEN_WELCOME Screen = iota
)

type App struct {
	screen Screen
	width  int
	height int
}

func New() App {
	return App{
		screen: SCREEN_WELCOME,
	}
}

func (a App) Init() tea.Cmd {
	return nil
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return a, tea.Quit
		}
	}

	return a, nil
}

func (a App) View() string {
	return "KAPI — press q to quit\n"
}
