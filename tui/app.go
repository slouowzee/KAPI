package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/slouowzee/kapi/tui/screens"
)

type Screen int

const (
	SCREEN_WELCOME Screen = iota
)

type App struct {
	screen  Screen
	width   int
	height  int
	welcome screens.WelcomeModel
}

func New() App {
	return App{
		screen:  SCREEN_WELCOME,
		welcome: screens.NewWelcome(0, 0),
	}
}

func (a App) Init() tea.Cmd {
	return a.welcome.Init()
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		if a.screen == SCREEN_WELCOME {
			a.welcome.SetSize(msg.Width, msg.Height)
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return a, tea.Quit
		}
	}

	switch a.screen {
	case SCREEN_WELCOME:
		updated, cmd := a.welcome.Update(msg)
		a.welcome = updated
		if updated.IsNewProjectSelected() {
			// TODO: transition to folder selection screen
		}
		if updated.IsBrowsePackagesSelected() {
			// TODO: transition to package browser screen
		}
		if updated.IsUpdateSelected() {
			// TODO: launch update process
		}
		return a, cmd
	}

	return a, nil
}

func (a App) View() string {
	switch a.screen {
	case SCREEN_WELCOME:
		return a.welcome.View()
	}
	return ""
}
