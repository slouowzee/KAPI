package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/slouowzee/kapi/internal/config"
	"github.com/slouowzee/kapi/internal/packages"
	"github.com/slouowzee/kapi/internal/semver"
	"github.com/slouowzee/kapi/tui/styles"
)

func HandleFreeze(args []string) {
	if len(args) == 0 {
		printFreezeHelp()
		os.Exit(1)
	}

	arg := args[0]
	if arg == "--help" || arg == "-h" {
		printFreezeHelp()
		return
	}

	var name, version string
	lastIdx := strings.LastIndex(arg, "/")
	if lastIdx != -1 {
		maybeVer := arg[lastIdx+1:]
		if len(maybeVer) > 0 && (maybeVer[0] >= '0' && maybeVer[0] <= '9' || maybeVer[0] == 'v') {
			name = arg[:lastIdx]
			version = maybeVer
		} else {
			name = arg
		}
	} else {
		name = arg
	}

	fmt.Printf("Searching for package '%s'...\n", name)

	ctx := context.Background()

	pkg, registryName := findExactPackage(ctx, name)

	if pkg == nil {
		fmt.Printf("Exact package not found, performing fuzzy search...\n")
		pkg, registryName = fuzzySearchPackage(ctx, name)
		if pkg == nil {
			fmt.Printf("Package '%s' not found in registries.\n", name)
			os.Exit(1)
		}
	} else {
		fmt.Printf("Found exact match (%s)\n", registryName)
	}

	if version == "" {
		version = promptForVersion(pkg)
	} else {
		foundVer := false
		for _, v := range pkg.Versions {
			if v == version || v == "v"+version || "v"+v == version || v == strings.TrimPrefix(version, "v") {
				foundVer = true
				version = v
				break
			}
		}

		if !foundVer {
			fmt.Printf("Version '%s' does not exist for package '%s' (%s).\n", version, pkg.Name, registryName)
			os.Exit(1)
		}
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	if cfg.FreezeVersionPackages == nil {
		cfg.FreezeVersionPackages = make(map[string][]config.FreezeVersionPackage)
	}

	freezePkg := config.FreezeVersionPackage{
		Name:        pkg.Name,
		Version:     version,
		Description: pkg.Description,
	}

	exists := false
	for i, f := range cfg.FreezeVersionPackages[registryName] {
		if f.Name == pkg.Name {
			cfg.FreezeVersionPackages[registryName][i].Version = version
			exists = true
			break
		}
	}

	if !exists {
		cfg.FreezeVersionPackages[registryName] = append(cfg.FreezeVersionPackages[registryName], freezePkg)
	}

	if err := config.Save(cfg); err != nil {
		fmt.Printf("Error saving configuration: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Package %s version %s successfully cached in config.json\n", pkg.Name, version)
}

func printFreezeHelp() {
	PrintLogoAndTitle("Freeze")
	fmt.Println("  " + styles.MutedStyle.Render("Usage:"))
	fmt.Println("    " + styles.SelectedStyle.Render("kapi freeze <package>[/<version>]") + "  Freezes the version of a package to prevent it from being updated.")
	fmt.Println()
}

func findExactPackage(ctx context.Context, name string) (*packages.Package, string) {
	res := packages.FetchDefaults(ctx, []string{name}, false)
	if len(res) > 0 && len(res[0].Versions) > 0 {
		return &res[0], "npm"
	}
	res = packages.FetchDefaults(ctx, []string{name}, true)
	if len(res) > 0 && len(res[0].Versions) > 0 {
		return &res[0], "packagist"
	}
	return nil, ""
}

func fuzzySearchPackage(ctx context.Context, query string) (*packages.Package, string) {
	var results []struct {
		pkg packages.Package
		reg string
	}

	npmRes, _ := packages.SearchNpm(ctx, query)
	packagistRes, _ := packages.SearchPackagist(ctx, query)

	for i := 0; i < len(npmRes) && i < 10; i++ {
		if len(npmRes[i].Versions) > 0 {
			results = append(results, struct {
				pkg packages.Package
				reg string
			}{npmRes[i], "npm"})
		}
	}
	for i := 0; i < len(packagistRes) && i < 10; i++ {
		if len(packagistRes[i].Versions) > 0 {
			results = append(results, struct {
				pkg packages.Package
				reg string
			}{packagistRes[i], "packagist"})
		}
	}

	if len(results) == 0 {
		return nil, ""
	}

	if len(results) == 1 {
		return &results[0].pkg, results[0].reg
	}

	var choices []string
	for _, r := range results {
		desc := r.pkg.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		choices = append(choices, fmt.Sprintf("%s (%s) - %s", r.pkg.Name, r.reg, desc))
	}

	choice, err := PromptChoice("Multiple packages found. Select one:", choices)
	if err != nil {
		os.Exit(1)
	}
	return &results[choice].pkg, results[choice].reg
}

func promptForVersion(pkg *packages.Package) string {
	sorted := make([]string, len(pkg.Versions))
	copy(sorted, pkg.Versions)
	sort.Slice(sorted, func(i, j int) bool {
		return semver.Greater(sorted[i], sorted[j])
	})

	var display []string
	if len(sorted) > 10 {
		display = append(display, sorted[:10]...)
	} else {
		display = append(display, sorted...)
	}

	choice, err := PromptChoice(fmt.Sprintf("Select a version for '%s':", pkg.Name), display)
	if err != nil {
		os.Exit(1)
	}
	return display[choice]
}
