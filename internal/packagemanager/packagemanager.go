package packagemanager

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type PM int

const (
	None PM = iota
	NPM
	PNPM
	Yarn
	Bun
)

func All() []PM { return []PM{NPM, PNPM, Yarn, Bun} }

func (pm PM) String() string {
	switch pm {
	case NPM:
		return "npm"
	case PNPM:
		return "pnpm"
	case Yarn:
		return "yarn"
	case Bun:
		return "bun"
	default:
		return ""
	}
}

func (pm PM) Label() string { return pm.String() }

func Parse(s string) PM {
	switch s {
	case "npm":
		return NPM
	case "pnpm":
		return PNPM
	case "yarn":
		return Yarn
	case "bun":
		return Bun
	default:
		return None
	}
}

// yarnSupportsDlx is a variable so tests can simulate either Yarn flavour.
var yarnSupportsDlx = sync.OnceValue(detectYarnDlx)

// detectYarnDlx reports whether `yarn dlx` is available: Yarn 2+ and the
// Corepack shim support it, a globally installed Yarn 1 does not.
func detectYarnDlx() bool {
	path, err := exec.LookPath("yarn")
	if err != nil {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil && strings.Contains(resolved, "corepack") {
		return true
	}
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		return false
	}
	return !strings.HasPrefix(strings.TrimSpace(string(out)), "1.")
}

func (pm PM) ExecArgs() []string {
	switch pm {
	case PNPM:
		return []string{"pnpm", "dlx"}
	case Yarn:
		if !yarnSupportsDlx() {
			// NOTE: Yarn 1 has no dlx; npx ships with Node and runs the same package.
			return []string{"npx"}
		}
		return []string{"yarn", "dlx"}
	case Bun:
		return []string{"bunx"}
	default:
		return []string{"npx"}
	}
}

func (pm PM) CreateArgs() []string {
	switch pm {
	case PNPM:
		return []string{"pnpm", "create"}
	case Yarn:
		return []string{"yarn", "create"}
	case Bun:
		return []string{"bun", "create"}
	default:
		return []string{"npm", "create"}
	}
}

func (pm PM) InstallArgs() []string {
	switch pm {
	case PNPM:
		return []string{"pnpm", "add"}
	case Yarn:
		return []string{"yarn", "add"}
	case Bun:
		return []string{"bun", "add"}
	default:
		return []string{"npm", "install"}
	}
}

func (pm PM) CIInstall() string {
	switch pm {
	case PNPM:
		return "pnpm install --frozen-lockfile"
	case Yarn:
		return "yarn install --frozen-lockfile"
	case Bun:
		return "bun install --frozen-lockfile"
	default:
		return "npm ci"
	}
}

func (pm PM) RunIfPresent(script string) string {
	switch pm {
	case NPM:
		return "npm run " + script + " --if-present"
	case PNPM:
		return "pnpm run --if-present " + script
	case Yarn:
		// NOTE: neither Yarn 1 nor Yarn Berry supports --if-present.
		return `if node -e "(require('./package.json').scripts||{}).` + script + `||process.exit(1)"; then yarn run ` + script + `; fi`
	case Bun:
		return "npm run " + script + " --if-present"
	default:
		return "npm run " + script + " --if-present"
	}
}

func (pm PM) CacheKey() string {
	switch pm {
	case PNPM:
		return "pnpm"
	case Yarn:
		return "yarn"
	case Bun:
		return "bun"
	default:
		return "npm"
	}
}

func DetectFromLockfile(dir string) PM {
	type lockfile struct {
		file string
		pm   PM
	}
	candidates := []lockfile{
		{"bun.lock", Bun},
		{"bun.lockb", Bun},
		{"pnpm-lock.yaml", PNPM},
		{"yarn.lock", Yarn},
		{"package-lock.json", NPM},
	}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(dir, c.file)); err == nil {
			return c.pm
		}
	}
	return None
}

func DetectInstalled() []PM {
	order := []PM{Bun, PNPM, Yarn, NPM}
	var found []PM
	for _, pm := range order {
		if _, err := exec.LookPath(pm.String()); err == nil {
			found = append(found, pm)
		}
	}
	return found
}
