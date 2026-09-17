package scaffold

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/slouowzee/kapi/internal/registry"
)

const maxNpmNameLen = 214

var (
	npmNameRe          = regexp.MustCompile(`^[a-z0-9][a-z0-9._~-]*$`)
	composerInvalidRe  = regexp.MustCompile(`[^a-z0-9]+`)
	composerSeparators = "-_."
)

// ProjectNameError explains why the target directory name cannot be used by
// the framework's initializer, or returns an empty string when it can.
// JS initializers use the folder name as the npm package name.
func ProjectNameError(fw registry.Framework, targetDir string) string {
	if fw.Ecosystem != "js" {
		return ""
	}
	name := filepath.Base(targetDir)
	if len(name) > maxNpmNameLen || !npmNameRe.MatchString(name) {
		return fmt.Sprintf("%q is not a valid npm package name: use lowercase letters, digits, '-', '_' or '.'", name)
	}
	return ""
}

// composerPackageName turns a folder name into a valid composer package name
// part (lowercase alphanumerics separated by dashes).
func composerPackageName(name string) string {
	sanitized := composerInvalidRe.ReplaceAllString(strings.ToLower(name), "-")
	sanitized = strings.Trim(sanitized, composerSeparators)
	if sanitized == "" {
		return "app"
	}
	return sanitized
}
