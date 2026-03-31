package main

import (
	"fmt"
	"os"

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

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
