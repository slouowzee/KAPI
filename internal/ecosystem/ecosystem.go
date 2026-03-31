package ecosystem

import (
	"os"
	"path/filepath"
)

type Ecosystem int

const (
	ECOSYSTEM_NONE Ecosystem = iota
	ECOSYSTEM_PHP
	ECOSYSTEM_JS
	ECOSYSTEM_BOTH
)

var PHP_LOCK_FILES = []string{"composer.lock"}
var JS_LOCK_FILES = []string{
	"package-lock.json",
	"yarn.lock",
	"pnpm-lock.yaml",
	"bun.lock",
	"deno.lock",
}

// Detect checks the given directory for known lock files and returns the
// detected ecosystem(s). It checks PHP and JS independently so that mixed
// projects (monorepo, Laravel + Inertia, etc.) return ECOSYSTEM_BOTH.
func Detect(dir string) Ecosystem {
	hasFile := func(names []string) bool {
		for _, name := range names {
			if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
				return true
			}
		}
		return false
	}

	php := hasFile(PHP_LOCK_FILES)
	js := hasFile(JS_LOCK_FILES)

	switch {
	case php && js:
		return ECOSYSTEM_BOTH
	case php:
		return ECOSYSTEM_PHP
	case js:
		return ECOSYSTEM_JS
	default:
		return ECOSYSTEM_NONE
	}
}

func (e Ecosystem) Label() string {
	switch e {
	case ECOSYSTEM_PHP:
		return "PHP project"
	case ECOSYSTEM_JS:
		return "JS project"
	case ECOSYSTEM_BOTH:
		return "PHP + JS project"
	default:
		return ""
	}
}

func (e Ecosystem) HasPackages() bool {
	return e != ECOSYSTEM_NONE
}
