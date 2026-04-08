package packagemanager

import (
	"os"
	"os/exec"
	"path/filepath"
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

func (pm PM) Exec() string {
	switch pm {
	case PNPM:
		return "pnpm"
	case Yarn:
		return "yarn"
	case Bun:
		return "bunx"
	default:
		return "npx"
	}
}

func (pm PM) ExecArgs() []string {
	switch pm {
	case PNPM:
		return []string{"pnpm", "dlx"}
	case Yarn:
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

func (pm PM) RunScript() string {
	switch pm {
	case Bun:
		return "bun run"
	default:
		return pm.String() + " run"
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
