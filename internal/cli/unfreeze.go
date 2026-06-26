package cli

import (
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

func unfreezeByName(name string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	found := false
	for reg, pkgs := range cfg.FreezeVersionPackages {
		for i, p := range pkgs {
			if p.Name == name {
				if len(pkgs) == 1 {
					delete(cfg.FreezeVersionPackages, reg)
				} else {
					cfg.FreezeVersionPackages[reg] = append(pkgs[:i], pkgs[i+1:]...)
				}
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		fmt.Printf("Package '%s' not found in frozen packages.\n", name)
		os.Exit(1)
	}

	if err := config.Save(cfg); err != nil {
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

	if len(cfg.FreezeVersionPackages[selected.reg]) == 1 {
		delete(cfg.FreezeVersionPackages, selected.reg)
	} else {
		pkgs := cfg.FreezeVersionPackages[selected.reg]
		for i, p := range pkgs {
			if p.Name == selected.pkg.Name {
				cfg.FreezeVersionPackages[selected.reg] = append(pkgs[:i], pkgs[i+1:]...)
				break
			}
		}
	}

	if err := config.Save(cfg); err != nil {
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
