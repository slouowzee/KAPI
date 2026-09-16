package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slouowzee/kapi/internal/cli"
	"github.com/slouowzee/kapi/internal/updater"
	"github.com/slouowzee/kapi/scaffold"
	"github.com/slouowzee/kapi/tui"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "config":
			cli.HandleConfig(os.Args[2:])
			return
		case "freeze":
			cli.HandleFreeze(os.Args[2:])
			return
		case "unfreeze":
			cli.HandleUnfreeze(os.Args[2:])
			return
		case "shell-init":
			cli.HandleShellInit(os.Args[2:])
			return
		case "version", "-version", "--version", "-v":
			fmt.Println(updater.CurrentVersion)
			return
		case "help", "--help", "-h":
			cli.PrintHelp()
			return
		}
	}

	p := tea.NewProgram(
		tui.New(),
		tea.WithAltScreen(),
	)

	// NOTE: Bubble Tea handles SIGINT and SIGTERM but not SIGHUP (terminal
	// closed); stop the program cleanly so running commands are stopped too.
	hangup := make(chan os.Signal, 1)
	signal.Notify(hangup, syscall.SIGHUP)
	go func() {
		if _, ok := <-hangup; ok {
			p.Kill()
		}
	}()

	model, err := p.Run()
	signal.Stop(hangup)
	close(hangup)
	scaffold.StopRunningCommands()
	if err != nil {
		if errors.Is(err, tea.ErrProgramKilled) || errors.Is(err, tea.ErrInterrupted) {
			os.Exit(130)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if app, ok := model.(tui.App); ok {
		if err := app.ConfigError(); err != nil {
			fmt.Fprintf(os.Stderr, "kapi: warning: could not load config: %v\n", err)
		}
		if dir := app.FinalDir(); dir != "" {
			writeCdTarget(dir)
		}
	}
}

func writeCdTarget(dir string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(home, ".kapi_last_cd"), []byte(dir), 0o644)
}
