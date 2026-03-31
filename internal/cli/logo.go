package cli

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/slouowzee/kapi/tui/styles"
)

func PrintLogoAndTitle(title string) {
	fmt.Println()
	lines := []string{
		` _  __   _   ___ ___ `,
		`| |/ /  /_\ | _ \_ _|`,
		`| ' <  / _ \|  _/| | `,
		`|_|\_\/_/ \_\_| |___|`,
	}
	colors := []lipgloss.Color{
		"#8B6542",
		"#8A7856",
		"#88896A",
		"#7C9A6B",
	}

	repoLink := fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\",
		"https://github.com/slouowzee/kapi",
		styles.LinkStyle.Render("github.com/slouowzee/kapi"),
	)

	for i, line := range lines {
		rendered := lipgloss.NewStyle().Foreground(colors[i]).Bold(true).Render(line)
		switch i {
		case 2:
			fmt.Println("  " + rendered + "   " + repoLink)
		case 3:
			fmt.Println("  " + rendered + "   " + styles.MutedStyle.Render(title))
		default:
			fmt.Println("  " + rendered)
		}
	}
	fmt.Println()
}
