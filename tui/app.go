package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slouowzee/kapi/internal/config"
	"github.com/slouowzee/kapi/internal/ecosystem"
	"github.com/slouowzee/kapi/internal/packagemanager"
	"github.com/slouowzee/kapi/internal/packages"
	"github.com/slouowzee/kapi/internal/registry"
	"github.com/slouowzee/kapi/scaffold"
	"github.com/slouowzee/kapi/tui/screens"
)

type Screen int

const (
	ScreenWelcome Screen = iota
	ScreenFolder
	ScreenEcosystem
	ScreenFramework
	ScreenPackages
	ScreenGit
	ScreenPMSelect
	ScreenRecap
	ScreenGitConfig
	ScreenSettings
	ScreenExec
	ScreenUpdateInfo
)

type App struct {
	screen    Screen
	width     int
	height    int
	welcome   screens.WelcomeModel
	folder    screens.FolderModel
	ecosystem screens.EcosystemModel
	framework screens.FrameworkModel
	packages  screens.PackagesModel
	git       screens.GitModel
	pmSelect  screens.PMSelectModel
	recap     screens.RecapModel
	gitConfig screens.GitConfigModel
	settings  screens.SettingsModel
	exec      screens.ExecModel
	update    screens.UpdateInfoModel

	selectedDir       string
	selectedEcosystem ecosystem.Ecosystem
	selectedFramework registry.Framework
	selectedPackages  []packages.Package
	selectedPM        packagemanager.PM
	selectedGit       screens.GitConfig

	cdDir      string
	browseMode bool
	editMode   bool
}

func New() App {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "kapi: warning: could not load config: %v\n", err)
	}
	return App{
		screen:     ScreenWelcome,
		welcome:    screens.NewWelcome(0, 0),
		selectedPM: packagemanager.Parse(cfg.PackageManager),
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
		case ScreenWelcome:
			a.welcome.SetSize(msg.Width, msg.Height)
		case ScreenFolder:
			a.folder.SetSize(msg.Width, msg.Height)
		case ScreenEcosystem:
			a.ecosystem.SetSize(msg.Width, msg.Height)
		case ScreenFramework:
			a.framework.SetSize(msg.Width, msg.Height)
		case ScreenPackages:
			a.packages.SetSize(msg.Width, msg.Height)
		case ScreenGit:
			a.git.SetSize(msg.Width, msg.Height)
		case ScreenPMSelect:
			a.pmSelect.SetSize(msg.Width, msg.Height)
		case ScreenRecap:
			a.recap.SetSize(msg.Width, msg.Height)
		case ScreenGitConfig:
			a.gitConfig.SetSize(msg.Width, msg.Height)
		case ScreenSettings:
			a.settings.SetSize(msg.Width, msg.Height)
		case ScreenExec:
			a.exec.SetSize(msg.Width, msg.Height)
		case ScreenUpdateInfo:
			a.update.SetSize(msg.Width, msg.Height)
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit
		case "q":
			if a.screen == ScreenFolder && a.folder.IsInputMode() {
				break
			}
			if a.screen == ScreenGit && a.git.IsInputMode() {
				break
			}
			if a.screen == ScreenGitConfig && a.gitConfig.IsInputMode() {
				break
			}
			if a.screen == ScreenFramework || a.screen == ScreenPackages {
				break
			}
			if a.screen == ScreenPMSelect {
				break
			}
			if a.screen == ScreenRecap && a.recap.IsAbandonPending() {
				break
			}
			return a, tea.Quit
		}
	}

	switch a.screen {
	case ScreenWelcome:
		updated, cmd := a.welcome.Update(msg)
		a.welcome = updated

		if updated.IsNewProjectSelected() {
			a.welcome.ConsumeEnter()
			a.screen = ScreenFolder
			a.folder = screens.Folder(a.width, a.height, "")
			return a, a.folder.Init()
		}
		if updated.IsGitConfigSelected() {
			a.welcome.ConsumeEnter()
			a.screen = ScreenGitConfig
			a.gitConfig = screens.NewGitConfig(a.width, a.height, updated.WorkDir())
			return a, a.gitConfig.Init()
		}
		if updated.IsSettingsSelected() {
			a.welcome.ConsumeEnter()
			a.screen = ScreenSettings
			a.settings = screens.NewSettings(a.width, a.height)
			return a, a.settings.Init()
		}
		if updated.IsBrowsePackagesSelected() {
			a.welcome.ConsumeEnter()
			eco := updated.Ecosystem()
			fw := browseFallbackFramework(eco)
			a.selectedDir = updated.WorkDir()
			a.selectedFramework = fw
			a.browseMode = true
			a.screen = ScreenPackages
			a.packages = screens.NewPackages(a.width, a.height, fw, a.selectedDir)
			return a, a.packages.Init()
		}
		if updated.IsUpdateSelected() {
			a.welcome.ConsumeEnter()
			latest := updated.LatestVersion()
			if latest == "" {
				latest = "latest"
			}
			a.screen = ScreenUpdateInfo
			a.update = screens.NewUpdateInfo(a.width, a.height, latest)
			return a, a.update.Init()
		}
		return a, cmd

	case ScreenFolder:
		updated, cmd := a.folder.Update(msg)
		a.folder = updated

		if updated.IsBack() {
			a.folder.ConsumeBack()
			if a.editMode {
				a.editMode = false
				return a.goToRecap()
			}
			a.screen = ScreenWelcome
			return a, nil
		}
		if updated.Done() {
			a.folder.ConsumeDone()
			a.selectedDir = updated.SelectedDir()
			if a.editMode {
				a.editMode = false
				return a.goToRecap()
			}
			a.screen = ScreenEcosystem
			a.ecosystem = screens.NewEcosystem(a.width, a.height, a.selectedDir)
			return a, a.ecosystem.Init()
		}
		return a, cmd

	case ScreenEcosystem:
		updated, cmd := a.ecosystem.Update(msg)
		a.ecosystem = updated

		if updated.IsBack() {
			a.ecosystem.ConsumeBack()
			if a.editMode {
				a.editMode = false
				return a.goToRecap()
			}
			a.screen = ScreenFolder
			return a, nil
		}
		if updated.Done() {
			a.ecosystem.ConsumeDone()
			newEco := updated.SelectedEcosystem()
			if a.editMode && newEco != a.selectedEcosystem {
				a.selectedPackages = nil
				a.selectedPM = packagemanager.None
			}
			a.selectedEcosystem = newEco
			a.screen = ScreenFramework
			a.framework = screens.NewFramework(a.width, a.height, a.selectedEcosystem, a.selectedDir)
			return a, a.framework.Init()
		}
		return a, cmd

	case ScreenFramework:
		updated, cmd := a.framework.Update(msg)
		a.framework = updated

		if updated.IsBack() {
			a.framework.ConsumeBack()
			if a.editMode {
				a.editMode = false
				return a.goToRecap()
			}
			a.screen = ScreenEcosystem
			return a, nil
		}
		if updated.Done() {
			a.framework.ConsumeDone()
			newFW := updated.SelectedFramework()
			frameworkChanged := newFW.ID != a.selectedFramework.ID
			if a.editMode && frameworkChanged {
				a.selectedPackages = nil
				if newFW.Ecosystem != a.selectedFramework.Ecosystem {
					a.selectedPM = packagemanager.None
				}
			}
			a.selectedFramework = newFW
			if a.editMode && !frameworkChanged {
				a.editMode = false
				return a.goToRecap()
			}
			a.screen = ScreenPackages
			a.packages = screens.NewPackages(a.width, a.height, a.selectedFramework, a.selectedDir)
			return a, a.packages.Init()
		}
		return a, cmd

	case ScreenPackages:
		updated, cmd := a.packages.Update(msg)
		a.packages = updated

		if updated.IsBackCancelled() {
			a.packages.ConsumeBackCancelled()
			a.selectedPackages = updated.SavedPackages()
			a.editMode = false
			return a.goToRecap()
		}
		if updated.IsBack() {
			a.packages.ConsumeBack()
			if a.browseMode {
				a.browseMode = false
				a.screen = ScreenWelcome
				return a, nil
			}
			if a.editMode {
				a.editMode = false
				return a.goToRecap()
			}
			a.screen = ScreenFramework
			return a, nil
		}
		if updated.Done() {
			a.packages.ConsumeDone()
			if a.browseMode {
				a.browseMode = false
				a.screen = ScreenWelcome
				return a, nil
			}
			a.selectedPackages = a.packages.SelectedPackages()
			if a.editMode {
				a.editMode = false
				return a.goToRecap()
			}
			a.screen = ScreenGit
			a.git = screens.Git(a.width, a.height, a.selectedDir, screens.GitConfig{})
			return a, a.git.Init()
		}
		return a, cmd

	case ScreenGit:
		updated, cmd := a.git.Update(msg)
		a.git = updated

		if updated.IsBack() {
			a.git.ConsumeBack()
			if a.editMode {
				a.editMode = false
				return a.goToRecap()
			}
			a.screen = ScreenPackages
			return a, nil
		}
		if updated.Done() {
			a.git.ConsumeDone()
			a.selectedGit = updated.Config()
			a.selectedPackages = a.packages.SelectedPackages()

			if a.editMode {
				a.editMode = false
				return a.goToRecap()
			}

			if a.selectedFramework.Ecosystem == "js" {
				a.screen = ScreenPMSelect
				a.pmSelect = screens.NewPMSelect(a.width, a.height, a.selectedPM)
				return a, a.pmSelect.Init()
			}
			return a.goToRecap()
		}
		return a, cmd

	case ScreenPMSelect:
		updated, cmd := a.pmSelect.Update(msg)
		a.pmSelect = updated

		if updated.IsBack() {
			a.pmSelect.ConsumeBack()
			if a.editMode {
				a.editMode = false
				return a.goToRecap()
			}
			a.screen = ScreenGit
			return a, nil
		}
		if updated.Done() {
			a.pmSelect.ConsumeDone()
			a.selectedPM = updated.SelectedPM()
			a.editMode = false
			return a.goToRecap()
		}
		return a, cmd

	case ScreenRecap:
		updated, cmd := a.recap.Update(msg)
		a.recap = updated

		if updated.IsAbandoned() {
			a.recap.ConsumeAbandon()
			a.selectedDir = ""
			a.selectedEcosystem = 0
			a.selectedFramework = registry.Framework{}
			a.selectedPackages = nil
			a.selectedPM = packagemanager.None
			a.selectedGit = screens.GitConfig{}
			a.editMode = false
			a.screen = ScreenWelcome
			a.welcome = screens.NewWelcome(a.width, a.height)
			return a, a.welcome.Init()
		}
		if updated.IsBack() {
			a.recap.ConsumeBack()
			a.editMode = true
			switch updated.BackSection() {
			case screens.RECAP_SECTION_FOLDER:
				a.folder = screens.Folder(a.width, a.height, a.selectedDir)
				a.screen = ScreenFolder
			case screens.RECAP_SECTION_FRAMEWORK:
				a.ecosystem = screens.NewEcosystem(a.width, a.height, a.selectedDir)
				a.screen = ScreenEcosystem
			case screens.RECAP_SECTION_PACKAGES:
				a.packages = screens.NewPackagesFromCart(a.width, a.height, a.selectedFramework, a.selectedDir, a.selectedPackages)
				a.screen = ScreenPackages
				return a, a.packages.Init()
			case screens.RECAP_SECTION_GIT:
				a.git = screens.Git(a.width, a.height, a.selectedDir, a.selectedGit)
				a.screen = ScreenGit
			case screens.RECAP_SECTION_PM:
				if a.selectedFramework.Ecosystem == "js" {
					a.pmSelect = screens.NewPMSelect(a.width, a.height, a.selectedPM)
					a.screen = ScreenPMSelect
					return a, a.pmSelect.Init()
				} else {
					a.git = screens.Git(a.width, a.height, a.selectedDir, a.selectedGit)
					a.screen = ScreenGit
				}
			}
			return a, nil
		}
		if updated.Done() {
			a.recap.ConsumeDone()
			steps := scaffold.Plan(a.selectedDir, a.selectedFramework, a.selectedPackages, a.selectedGit, a.selectedPM)
			execSteps := make([]screens.ExecStep, len(steps))
			for i, s := range steps {
				execSteps[i] = screens.ExecStep{Label: s.Label, Cmd: s.Cmd, Fn: s.Fn, StreamFn: s.StreamFn}
			}
			a.screen = ScreenExec
			a.exec = screens.NewExec(a.width, a.height, execSteps, a.selectedDir)
			return a, a.exec.Init()
		}
		return a, cmd

	case ScreenGitConfig:
		updated, cmd := a.gitConfig.Update(msg)
		a.gitConfig = updated

		if updated.IsBack() {
			a.gitConfig.ConsumeBack()
			a.screen = ScreenWelcome
			return a, nil
		}
		return a, cmd

	case ScreenSettings:
		updated, cmd := a.settings.Update(msg)
		a.settings = updated

		if updated.IsBack() {
			a.settings.ConsumeBack()
			if updated.CurrentPM() != packagemanager.None {
				a.selectedPM = updated.CurrentPM()
			}
			a.screen = ScreenWelcome
			return a, nil
		}
		return a, cmd

	case ScreenExec:
		updated, cmd := a.exec.Update(msg)
		a.exec = updated

		if updated.ShouldReturnToRecap() {
			a.exec.ConsumeReturnToRecap()
			return a.goToRecap()
		}
		if updated.Done() {
			if updated.HasErr() {
				return a, cmd
			}
			if updated.CdRequested() {
				a.cdDir = a.selectedDir
			}
			return a, tea.Quit
		}
		return a, cmd

	case ScreenUpdateInfo:
		updated, cmd := a.update.Update(msg)
		a.update = updated

		if updated.IsDone() {
			a.update.ConsumeDone()
			a.screen = ScreenWelcome
			return a, nil
		}
		return a, cmd
	}

	return a, nil
}

func (a App) View() string {
	switch a.screen {
	case ScreenWelcome:
		return a.welcome.View()
	case ScreenFolder:
		return a.folder.View()
	case ScreenEcosystem:
		return a.ecosystem.View()
	case ScreenFramework:
		return a.framework.View()
	case ScreenPackages:
		return a.packages.View()
	case ScreenGit:
		return a.git.View()
	case ScreenPMSelect:
		return a.pmSelect.View()
	case ScreenRecap:
		return a.recap.View()
	case ScreenGitConfig:
		return a.gitConfig.View()
	case ScreenSettings:
		return a.settings.View()
	case ScreenExec:
		return a.exec.View()
	case ScreenUpdateInfo:
		return a.update.View()
	}
	return ""
}

func (a App) goToRecap() (App, tea.Cmd) {
	a.selectedPackages = a.packages.SelectedPackages()
	a.screen = ScreenRecap
	a.recap = screens.NewRecap(a.width, a.height, screens.RecapSummary{
		Dir:       a.selectedDir,
		Framework: a.selectedFramework,
		Pkgs:      a.selectedPackages,
		GitCfg:    a.selectedGit,
		PM:        a.selectedPM,
	})
	return a, a.recap.Init()
}

func (a App) FinalDir() string { return a.cdDir }

func browseFallbackFramework(eco ecosystem.Ecosystem) registry.Framework {
	switch eco {
	case ecosystem.ECOSYSTEM_PHP:
		return registry.Framework{ID: "vanilla-php", Name: "PHP project", Ecosystem: "php"}
	case ecosystem.ECOSYSTEM_JS, ecosystem.ECOSYSTEM_BOTH:
		return registry.Framework{ID: "vanilla-vite", Name: "JS project", Ecosystem: "js"}
	default:
		return registry.Framework{ID: "vanilla-vite", Name: "project", Ecosystem: "js"}
	}
}

