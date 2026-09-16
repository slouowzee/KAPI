package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/slouowzee/kapi/internal/config"
	"github.com/slouowzee/kapi/internal/packages"
	"github.com/slouowzee/kapi/internal/semver"
	"github.com/slouowzee/kapi/tui/styles"
)

func HandleFreeze(args []string) {
	if len(args) == 0 {
		listFrozen()
		return
	}

	arg := args[0]
	if arg == "--help" || arg == "-h" {
		printFreezeHelp()
		return
	}

	name, version := parseFreezeArg(arg)

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

	note := promptNote()

	err := config.Update(func(cfg *config.Config) error {
		cfg.SetFrozen(registryName, config.FreezeVersionPackage{
			Name:    pkg.Name,
			Version: version,
			Note:    note,
		})
		return nil
	})
	if err != nil {
		fmt.Printf("Error saving configuration: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Package %s version %s successfully cached in config.json\n", pkg.Name, version)
}

// versionSuffixRe matches what can follow the last "/" or "@" as a version:
// 1, 1.2.3, v2.0.0-beta.1… but not package segments such as var-dumper or volt.
var versionSuffixRe = regexp.MustCompile(`^v?\d+(\.\d+)*([-+][0-9A-Za-z.-]+)?$`)

// parseFreezeArg splits "<package>[@<version>]" or "<package>[/<version>]"
// into a package name and an optional version.
func parseFreezeArg(arg string) (name, version string) {
	if i := strings.LastIndex(arg, "@"); i > 0 {
		return arg[:i], arg[i+1:]
	}
	if i := strings.LastIndex(arg, "/"); i != -1 && versionSuffixRe.MatchString(arg[i+1:]) {
		return arg[:i], arg[i+1:]
	}
	return arg, ""
}

func printFreezeHelp() {
	PrintLogoAndTitle("Freeze")
	fmt.Println("  " + styles.MutedStyle.Render("Usage:"))
	fmt.Println("    " + styles.SelectedStyle.Render("kapi freeze <package>[@<version>]") + "  Freezes the version of a package to prevent it from being updated.")
	fmt.Println("    " + styles.SelectedStyle.Render("kapi freeze <package>[/<version>]") + "  Same as above, kept for compatibility.")
	fmt.Println()
}

func listFrozen() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	var registries []string
	for reg, pkgs := range cfg.FreezeVersionPackages {
		if len(pkgs) == 0 {
			continue
		}
		registries = append(registries, reg)
	}

	if len(registries) == 0 {
		fmt.Println("No frozen packages.")
		return
	}

	PrintLogoAndTitle("Frozen Packages")
	sort.Strings(registries)

	for _, reg := range registries {
		pkgs := cfg.FreezeVersionPackages[reg]
		sort.Slice(pkgs, func(i, j int) bool {
			return pkgs[i].Name < pkgs[j].Name
		})
		fmt.Println("  " + styles.SelectedStyle.Render(reg+":") + "\n")
		for _, p := range pkgs {
			line := fmt.Sprintf("    %s @ %s", p.Name, styles.SubtitleStyle.Render(p.Version))
			if p.Note != "" {
				note := p.Note
				if len(note) > 50 {
					note = note[:47] + "..."
				}
				line += "  — " + styles.DimStyle.Render(note)
			}
			fmt.Println(line)
		}
		fmt.Println()
	}
}

func findExactPackage(ctx context.Context, name string) (*packages.Package, string) {
	for _, reg := range []struct {
		name  string
		isPhp bool
	}{{"npm", false}, {"packagist", true}} {
		versions, err := packages.FetchVersions(ctx, name, reg.isPhp)
		if err == nil && len(versions) > 0 {
			return &packages.Package{Name: name, Versions: versions}, reg.name
		}
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

	for i := 0; i < len(npmRes) && i < 25; i++ {
		results = append(results, struct {
			pkg packages.Package
			reg string
		}{npmRes[i], "npm"})
	}
	for i := 0; i < len(packagistRes) && i < 25; i++ {
		results = append(results, struct {
			pkg packages.Package
			reg string
		}{packagistRes[i], "packagist"})
	}

	if len(results) == 0 {
		return nil, ""
	}

	choice := 0
	if len(results) > 1 {
		choice = promptPackageChoice(results)
	}
	selected := results[choice]

	versions, err := packages.FetchVersions(ctx, selected.pkg.Name, selected.reg == "packagist")
	if err != nil || len(versions) == 0 {
		return nil, ""
	}
	selected.pkg.Versions = versions
	return &selected.pkg, selected.reg
}

func promptPackageChoice(results []struct {
	pkg packages.Package
	reg string
}) int {
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
	return choice
}

func promptForVersion(pkg *packages.Package) string {
	sorted := make([]string, len(pkg.Versions))
	copy(sorted, pkg.Versions)
	sort.Slice(sorted, func(i, j int) bool {
		return semver.Greater(sorted[i], sorted[j])
	})

	choice, err := PromptChoice(fmt.Sprintf("Select a version for '%s':", pkg.Name), sorted,
		"⚠ Choosing an older version may cause dependency conflicts with recent framework versions.")
	if err != nil {
		os.Exit(1)
	}
	return sorted[choice]
}

func promptNote() string {
	fmt.Println("Optional note: why are you freezing this package? (press enter to skip)")
	fmt.Print("> ")
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}
