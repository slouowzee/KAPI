package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slouowzee/kapi/internal/cli"
	"github.com/slouowzee/kapi/tui"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "config":
			cli.HandleConfig(os.Args[2:])
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

	model, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if app, ok := model.(tui.App); ok {
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
