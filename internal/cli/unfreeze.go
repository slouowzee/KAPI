package cli

import (
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/slouowzee/kapi/internal/config"
	"github.com/slouowzee/kapi/tui/styles"
)

func HandleUnfreeze(args []string) {
	if len(args) > 0 {
		arg := args[0]
		if arg == "--help" || arg == "-h" {
			printUnfreezeHelp()
			return
		}
		unfreezeByName(arg)
		return
	}

	unfreezeBySelector()
}

var errNotFrozen = errors.New("package not frozen")

func unfreezeByName(name string) {
	err := config.Update(func(cfg *config.Config) error {
		if !cfg.RemoveFrozen("", name) {
			return errNotFrozen
		}
		return nil
	})
	if errors.Is(err, errNotFrozen) {
		fmt.Printf("Package '%s' not found in frozen packages.\n", name)
		os.Exit(1)
	}
	if err != nil {
		fmt.Printf("Error saving configuration: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Package %s successfully unfrozen.\n", name)
}

func unfreezeBySelector() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	var entries []struct {
		reg string
		pkg config.FreezeVersionPackage
	}
	for reg, pkgs := range cfg.FreezeVersionPackages {
		for _, p := range pkgs {
			entries = append(entries, struct {
				reg string
				pkg config.FreezeVersionPackage
			}{reg, p})
		}
	}

	if len(entries) == 0 {
		fmt.Println("No frozen packages.")
		return
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].reg != entries[j].reg {
			return entries[i].reg < entries[j].reg
		}
		return entries[i].pkg.Name < entries[j].pkg.Name
	})

	choices := make([]string, len(entries))
	for i, e := range entries {
		line := fmt.Sprintf("%s (%s) @ %s", e.pkg.Name, e.reg, e.pkg.Version)
		if e.pkg.Note != "" {
			note := e.pkg.Note
			if len(note) > 40 {
				note = note[:37] + "..."
			}
			line += " — " + note
		}
		choices[i] = line
	}

	choice, err := PromptChoice("Select a package to unfreeze:", choices)
	if err != nil {
		fmt.Println("Aborted.")
		return
	}

	selected := entries[choice]

	err = config.Update(func(cfg *config.Config) error {
		cfg.RemoveFrozen(selected.reg, selected.pkg.Name)
		return nil
	})
	if err != nil {
		fmt.Printf("Error saving configuration: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Package %s successfully unfrozen.\n", selected.pkg.Name)
}

func printUnfreezeHelp() {
	PrintLogoAndTitle("Unfreeze")
	fmt.Println("  " + styles.MutedStyle.Render("Usage:"))
	fmt.Println("    " + styles.SelectedStyle.Render("kapi unfreeze") + "               Show a list of frozen packages to unfreeze")
	fmt.Println("    " + styles.SelectedStyle.Render("kapi unfreeze <package>") + "  Unfreeze a specific package")
	fmt.Println()
}
