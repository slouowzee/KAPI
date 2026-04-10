package tui

import (
	"os"
	"os/exec"

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
	SCREEN_WELCOME Screen = iota
	SCREEN_FOLDER
	SCREEN_ECOSYSTEM
	SCREEN_FRAMEWORK
	SCREEN_PACKAGES
	SCREEN_GIT
	SCREEN_PM_SELECT
	SCREEN_RECAP
	SCREEN_GIT_CONFIG
	SCREEN_SETTINGS
	SCREEN_EXEC
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
	cfg, _ := config.Load()
	return App{
		screen:     SCREEN_WELCOME,
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
		case SCREEN_WELCOME:
			a.welcome.SetSize(msg.Width, msg.Height)
		case SCREEN_FOLDER:
			a.folder.SetSize(msg.Width, msg.Height)
		case SCREEN_ECOSYSTEM:
			a.ecosystem.SetSize(msg.Width, msg.Height)
		case SCREEN_FRAMEWORK:
			a.framework.SetSize(msg.Width, msg.Height)
		case SCREEN_PACKAGES:
			a.packages.SetSize(msg.Width, msg.Height)
		case SCREEN_GIT:
			a.git.SetSize(msg.Width, msg.Height)
		case SCREEN_PM_SELECT:
			a.pmSelect.SetSize(msg.Width, msg.Height)
		case SCREEN_RECAP:
			a.recap.SetSize(msg.Width, msg.Height)
		case SCREEN_GIT_CONFIG:
			a.gitConfig.SetSize(msg.Width, msg.Height)
		case SCREEN_SETTINGS:
			a.settings.SetSize(msg.Width, msg.Height)
		case SCREEN_EXEC:
			a.exec.SetSize(msg.Width, msg.Height)
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit
		case "q":
			if a.screen == SCREEN_FOLDER && a.folder.IsInputMode() {
				break
			}
			if a.screen == SCREEN_GIT && a.git.IsInputMode() {
				break
			}
			if a.screen == SCREEN_GIT_CONFIG && a.gitConfig.IsInputMode() {
				break
			}
			if a.screen == SCREEN_FRAMEWORK || a.screen == SCREEN_PACKAGES {
				break
			}
			if a.screen == SCREEN_PM_SELECT {
				break
			}
			if a.screen == SCREEN_RECAP && a.recap.IsAbandonPending() {
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
			a.folder = screens.Folder(a.width, a.height, "")
			return a, a.folder.Init()
		}
		if updated.IsGitConfigSelected() {
			a.welcome.ConsumeEnter()
			a.screen = SCREEN_GIT_CONFIG
			a.gitConfig = screens.NewGitConfig(a.width, a.height, updated.WorkDir())
			return a, a.gitConfig.Init()
		}
		if updated.IsSettingsSelected() {
			a.welcome.ConsumeEnter()
			a.screen = SCREEN_SETTINGS
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
			a.screen = SCREEN_PACKAGES
			a.packages = screens.NewPackages(a.width, a.height, fw, a.selectedDir)
			return a, a.packages.Init()
		}
		if updated.IsUpdateSelected() {
			a.welcome.ConsumeEnter()
			latest := updated.LatestVersion()
			if latest == "" {
				latest = "latest"
			}
			installCmd := execCmd("", "go", "install",
				"github.com/slouowzee/kapi@"+latest)
			a.screen = SCREEN_EXEC
			a.exec = screens.NewExec(a.width, a.height, []screens.ExecStep{{
				Label: "go install github.com/slouowzee/kapi@" + latest,
				Cmd:   installCmd,
			}}, "")
			return a, a.exec.Init()
		}
		return a, cmd

	case SCREEN_FOLDER:
		updated, cmd := a.folder.Update(msg)
		a.folder = updated

		if updated.IsBack() {
			a.folder.ConsumeBack()
			if a.editMode {
				a.editMode = false
				return a.goToRecap()
			}
			a.screen = SCREEN_WELCOME
			return a, nil
		}
		if updated.Done() {
			a.folder.ConsumeDone()
			a.selectedDir = updated.SelectedDir()
			if a.editMode {
				a.editMode = false
				return a.goToRecap()
			}
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
			if a.editMode {
				a.editMode = false
				return a.goToRecap()
			}
			a.screen = SCREEN_FOLDER
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
			if a.editMode {
				a.editMode = false
				return a.goToRecap()
			}
			a.screen = SCREEN_ECOSYSTEM
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
			a.screen = SCREEN_PACKAGES
			a.packages = screens.NewPackages(a.width, a.height, a.selectedFramework, a.selectedDir)
			return a, a.packages.Init()
		}
		return a, cmd

	case SCREEN_PACKAGES:
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
				a.screen = SCREEN_WELCOME
				return a, nil
			}
			if a.editMode {
				a.editMode = false
				return a.goToRecap()
			}
			a.screen = SCREEN_FRAMEWORK
			return a, nil
		}
		if updated.Done() {
			a.packages.ConsumeDone()
			if a.browseMode {
				a.browseMode = false
				a.screen = SCREEN_WELCOME
				return a, nil
			}
			a.selectedPackages = a.packages.SelectedPackages()
			if a.editMode {
				a.editMode = false
				return a.goToRecap()
			}
			a.screen = SCREEN_GIT
			a.git = screens.Git(a.width, a.height, a.selectedDir, screens.GitConfig{})
			return a, a.git.Init()
		}
		return a, cmd

	case SCREEN_GIT:
		updated, cmd := a.git.Update(msg)
		a.git = updated

		if updated.IsBack() {
			a.git.ConsumeBack()
			if a.editMode {
				a.editMode = false
				return a.goToRecap()
			}
			a.screen = SCREEN_PACKAGES
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
				a.screen = SCREEN_PM_SELECT
				a.pmSelect = screens.NewPMSelect(a.width, a.height, a.selectedPM)
				return a, a.pmSelect.Init()
			}
			return a.goToRecap()
		}
		return a, cmd

	case SCREEN_PM_SELECT:
		updated, cmd := a.pmSelect.Update(msg)
		a.pmSelect = updated

		if updated.IsBack() {
			a.pmSelect.ConsumeBack()
			if a.editMode {
				a.editMode = false
				return a.goToRecap()
			}
			a.screen = SCREEN_GIT
			return a, nil
		}
		if updated.Done() {
			a.pmSelect.ConsumeDone()
			a.selectedPM = updated.SelectedPM()
			a.editMode = false
			return a.goToRecap()
		}
		return a, cmd

	case SCREEN_RECAP:
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
			a.screen = SCREEN_WELCOME
			a.welcome = screens.NewWelcome(a.width, a.height)
			return a, a.welcome.Init()
		}
		if updated.IsBack() {
			a.recap.ConsumeBack()
			a.editMode = true
			switch updated.BackSection() {
			case screens.RECAP_SECTION_FOLDER:
				a.folder = screens.Folder(a.width, a.height, a.selectedDir)
				a.screen = SCREEN_FOLDER
			case screens.RECAP_SECTION_FRAMEWORK:
				a.ecosystem = screens.NewEcosystem(a.width, a.height, a.selectedDir)
				a.screen = SCREEN_ECOSYSTEM
			case screens.RECAP_SECTION_PACKAGES:
				a.packages = screens.NewPackagesFromCart(a.width, a.height, a.selectedFramework, a.selectedDir, a.selectedPackages)
				a.screen = SCREEN_PACKAGES
			case screens.RECAP_SECTION_GIT:
				a.git = screens.Git(a.width, a.height, a.selectedDir, a.selectedGit)
				a.screen = SCREEN_GIT
			case screens.RECAP_SECTION_PM:
				if a.selectedFramework.Ecosystem == "js" {
					a.pmSelect = screens.NewPMSelect(a.width, a.height, a.selectedPM)
					a.screen = SCREEN_PM_SELECT
					return a, a.pmSelect.Init()
				} else {
					a.git = screens.Git(a.width, a.height, a.selectedDir, a.selectedGit)
					a.screen = SCREEN_GIT
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
			a.screen = SCREEN_EXEC
			a.exec = screens.NewExec(a.width, a.height, execSteps, a.selectedDir)
			return a, a.exec.Init()
		}
		return a, cmd

	case SCREEN_GIT_CONFIG:
		updated, cmd := a.gitConfig.Update(msg)
		a.gitConfig = updated

		if updated.IsBack() {
			a.gitConfig.ConsumeBack()
			a.screen = SCREEN_WELCOME
			return a, nil
		}
		return a, cmd

	case SCREEN_SETTINGS:
		updated, cmd := a.settings.Update(msg)
		a.settings = updated

		if updated.IsBack() {
			a.settings.ConsumeBack()
			if updated.CurrentPM() != packagemanager.None {
				a.selectedPM = updated.CurrentPM()
			}
			a.screen = SCREEN_WELCOME
			return a, nil
		}
		return a, cmd

	case SCREEN_EXEC:
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
	case SCREEN_PACKAGES:
		return a.packages.View()
	case SCREEN_GIT:
		return a.git.View()
	case SCREEN_PM_SELECT:
		return a.pmSelect.View()
	case SCREEN_RECAP:
		return a.recap.View()
	case SCREEN_GIT_CONFIG:
		return a.gitConfig.View()
	case SCREEN_SETTINGS:
		return a.settings.View()
	case SCREEN_EXEC:
		return a.exec.View()
	}
	return ""
}

func (a App) goToRecap() (App, tea.Cmd) {
	a.selectedPackages = a.packages.SelectedPackages()
	a.screen = SCREEN_RECAP
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

func execCmd(dir string, name string, args ...string) *exec.Cmd {
	c := exec.Command(name, args...)
	if dir != "" {
		c.Dir = dir
	}
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c
}
