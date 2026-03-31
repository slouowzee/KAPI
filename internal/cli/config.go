package cli

import (
	"fmt"
	"os"

	"github.com/slouowzee/kapi/internal/config"
	"github.com/slouowzee/kapi/tui/styles"
)

func HandleConfig(args []string) {
	if len(args) == 0 {
		printConfigHelp()
		os.Exit(1)
	}

	key := args[0]

	if len(args) == 2 && args[1] == "--help" {
		printKeyHelp(key)
		os.Exit(0)
	}

	if len(args) != 2 {
		printConfigHelp()
		os.Exit(1)
	}

	value := args[1]
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	switch key {
	case "github.token":
		cfg.GithubToken = value
	default:
		fmt.Fprintf(os.Stderr, "Unknown configuration key: %s\n", key)
		os.Exit(1)
	}

	if err := config.Save(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Set %s successfully.\n", key)
}

func printConfigHelp() {
	PrintLogoAndTitle("Configuration")
	fmt.Println("  " + styles.MutedStyle.Render("Usage:") + " kapi config <key> <value>")
	fmt.Println()
	fmt.Println(styles.MutedStyle.Render("  Available keys:"))
	fmt.Println("    " + styles.SelectedStyle.Render("github.token") + "    Set GitHub token to avoid API rate limits")
	fmt.Println()
	fmt.Println(styles.DimStyle.Render("  Tip: Use 'kapi config <key> --help' for details on a specific key."))
	fmt.Println()
}

func printKeyHelp(key string) {
	PrintLogoAndTitle("Config: " + key)
	switch key {
	case "github.token":
		fmt.Println("  Sets your Personal Access Token for GitHub to bypass the 60 requests/hour limit.")
		fmt.Println("  KAPI uses this to display GitHub stars in the framework selection screen.")
		fmt.Println()
		fmt.Println(styles.MutedStyle.Render("  How to get a token:"))

		link := fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\",
			"https://github.com/settings/tokens",
			styles.LinkStyle.Render("GitHub Token Settings"),
		)

		fmt.Println("    1. Go to " + link)
		fmt.Println("    2. Click " + styles.SelectedStyle.Render("Generate new token") + " -> " + styles.SelectedStyle.Render("Generate new token (classic)"))
		fmt.Println("    3. Give it a name (e.g. 'KAPI')")
		fmt.Println("    4. DO NOT select any scopes/permissions (public access is enough)")
		fmt.Println("    5. Click 'Generate token' and copy the result")
		fmt.Println()
		fmt.Println(styles.MutedStyle.Render("  Usage:"))
		fmt.Println("    kapi config github.token " + styles.DimStyle.Render("\"ghp_your_token_here\""))
	default:
		fmt.Printf("  No help available for unknown key: %s\n", key)
	}
	fmt.Println()
}
