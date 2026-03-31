package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/slouowzee/kapi/tui/screens"
)

type Screen int

const (
	SCREEN_WELCOME Screen = iota
	SCREEN_FOLDER
	SCREEN_ECOSYSTEM
	SCREEN_FRAMEWORK
)

type App struct {
	screen    Screen
	width     int
	height    int
	welcome   screens.WelcomeModel
	folder    screens.FolderModel
	ecosystem screens.EcosystemModel
	framework screens.FrameworkModel

	selectedDir       string
	selectedEcosystem int
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
		switch a.screen {
		case SCREEN_WELCOME:
			a.welcome.SetSize(msg.Width, msg.Height)
		case SCREEN_FOLDER:
			a.folder.SetSize(msg.Width, msg.Height)
		case SCREEN_ECOSYSTEM:
			a.ecosystem.SetSize(msg.Width, msg.Height)
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit
		case "q":
			if a.screen == SCREEN_FOLDER && a.folder.IsInputMode() {
				break
			}
			if a.screen == SCREEN_FRAMEWORK {
				break
			}
			return a, tea.Quit
		}
	}

	switch a.screen {
	case SCREEN_WELCOME:
		updated, cmd := a.welcome.Update(msg)
		a.welcome = updated

		if updated.IsNewProjectSelected() {
			a.welcome.ConsumeEnter()
			a.screen = SCREEN_FOLDER
			a.folder = screens.NewFolder(a.width, a.height)
			return a, a.folder.Init()
		}
		if updated.IsBrowsePackagesSelected() {
			a.welcome.ConsumeEnter()
			// TODO: transition to package browser screen
		}
		if updated.IsUpdateSelected() {
			a.welcome.ConsumeEnter()
			// TODO: launch update process
		}
		return a, cmd

	case SCREEN_FOLDER:
		updated, cmd := a.folder.Update(msg)
		a.folder = updated

		if updated.IsBack() {
			a.folder.ConsumeBack()
			a.screen = SCREEN_WELCOME
			return a, nil
		}
		if updated.Done() {
			a.folder.ConsumeDone()
			a.selectedDir = updated.SelectedDir()
			a.screen = SCREEN_ECOSYSTEM
			a.ecosystem = screens.NewEcosystem(a.width, a.height, a.selectedDir)
			return a, a.ecosystem.Init()
		}
		return a, cmd

	case SCREEN_ECOSYSTEM:
		updated, cmd := a.ecosystem.Update(msg)
		a.ecosystem = updated

		if updated.IsBack() {
			a.ecosystem.ConsumeBack()
			a.screen = SCREEN_FOLDER
			return a, nil
		}
		if updated.Done() {
			a.ecosystem.ConsumeDone()
			a.selectedEcosystem = updated.SelectedEcosystem()
			a.screen = SCREEN_FRAMEWORK
			a.framework = screens.NewFramework(a.width, a.height, a.selectedEcosystem, a.selectedDir)
			return a, a.framework.Init()
		}
		return a, cmd

	case SCREEN_FRAMEWORK:
		updated, cmd := a.framework.Update(msg)
		a.framework = updated

		if updated.IsBack() {
			a.framework.ConsumeBack()
			a.screen = SCREEN_ECOSYSTEM
			return a, nil
		}
		if updated.Done() {
			a.framework.ConsumeDone()
			// TODO: transition to packages screen
		}
		return a, cmd
	}

	return a, nil
}

func (a App) View() string {
	switch a.screen {
	case SCREEN_WELCOME:
		return a.welcome.View()
	case SCREEN_FOLDER:
		return a.folder.View()
	case SCREEN_ECOSYSTEM:
		return a.ecosystem.View()
	case SCREEN_FRAMEWORK:
		return a.framework.View()
	}
	return ""
}
