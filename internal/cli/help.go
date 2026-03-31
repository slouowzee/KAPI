package cli

import (
	"fmt"

	"github.com/slouowzee/kapi/tui/styles"
)

func PrintHelp() {
	PrintLogoAndTitle("Keep Accelerating Project Initialization")
	fmt.Println(styles.MutedStyle.Render("  Usage:"))
	fmt.Println("    " + styles.SelectedStyle.Render("kapi") + "                  Launch the interactive TUI")
	fmt.Println("    " + styles.SelectedStyle.Render("kapi config <k> <v>") + "   Set a configuration value")
	fmt.Println("    " + styles.SelectedStyle.Render("kapi help") + "             Show this help message")
	fmt.Println()
}
