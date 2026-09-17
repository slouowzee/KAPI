package cli

import (
	"fmt"
	"os"

	"github.com/slouowzee/kapi/internal/config"
	"github.com/slouowzee/kapi/internal/packagemanager"
	"github.com/slouowzee/kapi/tui/styles"
)

func HandleConfig(args []string) {
	if len(args) == 0 {
		listConfig()
		return
	}

	key := args[0]

	if key == "--help" || key == "-h" {
		printConfigHelp()
		os.Exit(0)
	}

	if len(args) == 2 && args[1] == "--help" {
		printKeyHelp(key)
		os.Exit(0)
	}

	if len(args) == 1 {
		handleConfigGet(key)
		return
	}

	if len(args) != 2 {
		printConfigHelp()
		os.Exit(1)
	}

	value := args[1]

	var apply func(cfg *config.Config)
	switch key {
	case "github.token":
		apply = func(cfg *config.Config) { cfg.GithubToken = value }
	case "package.manager":
		pmValue, ok := resolvePackageManagerValue(value)
		if !ok {
			fmt.Fprintf(os.Stderr, "Invalid package manager %q. Valid values: npm, pnpm, yarn, bun, none\n", value)
			os.Exit(1)
		}
		apply = func(cfg *config.Config) { cfg.PackageManager = pmValue }
	default:
		fmt.Fprintf(os.Stderr, "Unknown configuration key: %s\n", key)
		os.Exit(1)
	}

	if err := config.Update(func(cfg *config.Config) error {
		apply(cfg)
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Set %s successfully.\n", key)
}

func handleConfigGet(key string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	switch key {
	case "github.token":
		printConfigValue(key, githubTokenDisplay(cfg.GithubToken))
	case "package.manager":
		printConfigValue(key, packageManagerDisplay(cfg.PackageManager))
	default:
		fmt.Fprintf(os.Stderr, "Unknown configuration key: %s\n", key)
		os.Exit(1)
	}
}

// listConfig prints every known configuration key with its current value,
// for `kapi config` run with no arguments.
func listConfig() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}
	printConfigValue("github.token", githubTokenDisplay(cfg.GithubToken))
	printConfigValue("package.manager", packageManagerDisplay(cfg.PackageManager))
}

func printConfigValue(key, value string) {
	fmt.Println(styles.MutedStyle.Render("  "+key) + "  " + value)
}

func githubTokenDisplay(tok string) string {
	if tok == "" {
		return styles.DimStyle.Render("(not set)")
	}
	return MaskToken(tok)
}

func packageManagerDisplay(pm string) string {
	if pm == "" {
		return styles.DimStyle.Render("(not set)")
	}
	return pm
}

// resolvePackageManagerValue turns a `kapi config package.manager <value>`
// argument into the string to store. An empty value or "none" clears the
// saved preference instead of being rejected as invalid.
func resolvePackageManagerValue(value string) (resolved string, ok bool) {
	if value == "" || value == "none" {
		return "", true
	}
	pm := packagemanager.Parse(value)
	if pm == packagemanager.None {
		return "", false
	}
	return pm.String(), true
}

// MaskToken hides a secret while keeping enough of it to recognise it.
func MaskToken(tok string) string {
	const show = 7
	const tail = 4
	if len(tok) <= show+tail {
		return "***"
	}
	return tok[:show] + "***" + tok[len(tok)-tail:]
}

func printConfigHelp() {
	PrintLogoAndTitle("Configuration")
	fmt.Println("  " + styles.MutedStyle.Render("Usage:"))
	fmt.Println("    " + styles.SelectedStyle.Render("kapi config") + "                 List every configuration value")
	fmt.Println("    " + styles.SelectedStyle.Render("kapi config <key>") + "          Read a configuration value")
	fmt.Println("    " + styles.SelectedStyle.Render("kapi config <key> <value>") + "  Set a configuration value")
	fmt.Println()
	fmt.Println(styles.MutedStyle.Render("  Available keys:"))
	fmt.Println("    " + styles.SelectedStyle.Render("github.token") + "      Set GitHub token to avoid API rate limits")
	fmt.Println("    " + styles.SelectedStyle.Render("package.manager") + "   Set default JS package manager (npm|pnpm|yarn|bun)")
	fmt.Println()
	fmt.Println(styles.DimStyle.Render("  Tip: Use 'kapi config <key> --help' for details on a specific key."))
	fmt.Println()
}

func printKeyHelp(key string) {
	PrintLogoAndTitle("Config: " + key)
	switch key {
	case "github.token":
		fmt.Println("  Sets your Personal Access Token (classic) for GitHub.")
		fmt.Println("  KAPI uses it to display GitHub stars and to unlock additional features.")
		fmt.Println()
		fmt.Println(styles.MutedStyle.Render("  How to get a token:"))

		link := fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\",
			"https://github.com/settings/tokens",
			styles.LinkStyle.Render("GitHub Token Settings"),
		)

		fmt.Println("    1. Go to " + link)
		fmt.Println("    2. Click " + styles.SelectedStyle.Render("Generate new token") + " → " + styles.SelectedStyle.Render("Generate new token (classic)"))
		fmt.Println("    3. Give it a name (e.g. " + styles.DimStyle.Render("'KAPI'") + ")")
		fmt.Println("    4. Select the scopes you need:")
		fmt.Println()
		fmt.Println("       " + styles.SelectedStyle.Render("(no scope)") + "             GitHub stars display, public repo trends")
		fmt.Println("       " + styles.SelectedStyle.Render("repo") + "                   Create public & private repositories")
		fmt.Println("       " + styles.SelectedStyle.Render("write:ssh_signing_key") + "  Push SSH signing keys to your GitHub account")
		fmt.Println("       " + styles.SelectedStyle.Render("write:gpg_key") + "          Push GPG signing keys to your GitHub account")
		fmt.Println()
		fmt.Println("    " + styles.DimStyle.Render("Tip: select all three for the full KAPI experience."))
		fmt.Println("    5. Click 'Generate token' and copy the result")
		fmt.Println()
		fmt.Println(styles.MutedStyle.Render("  Usage:"))
		fmt.Println("    kapi config github.token " + styles.DimStyle.Render("\"ghp_your_token_here\""))
	case "package.manager":
		fmt.Println("  Sets the default JS package manager used when scaffolding new projects.")
		fmt.Println("  You can still override it per-project during the wizard.")
		fmt.Println()
		fmt.Println(styles.MutedStyle.Render("  Valid values:") + "  npm   pnpm   yarn   bun   " + styles.DimStyle.Render("(or empty/\"none\" to clear)"))
		fmt.Println()
		fmt.Println(styles.MutedStyle.Render("  Usage:"))
		fmt.Println("    kapi config package.manager " + styles.DimStyle.Render("pnpm"))
	default:
		fmt.Printf("  No help available for unknown key: %s\n", key)
	}
	fmt.Println()
}
