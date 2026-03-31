package screens

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

const MAX_PATH_LEN = 50

func truncatePath(path string) string {
	runes := []rune(path)
	if len(runes) <= MAX_PATH_LEN {
		return path
	}
	return "…" + string(runes[len(runes)-MAX_PATH_LEN+1:])
}

var fuckYouRe = regexp.MustCompile(`(?i)^fuck you$`)

var tmpRe = regexp.MustCompile(`(?i)^/tmp(/.*)?$`)

func detectEasterEgg(input string) string {
	trimmed := strings.TrimSpace(input)
	if fuckYouRe.MatchString(trimmed) {
		return fmt.Sprintf("Wym '%s'?, fuck you too dude.", trimmed)
	}
	if tmpRe.MatchString(trimmed) {
		return "A temporary project? Bold life choice. ( ͡° ͜ʖ ͡°)"
	}
	return ""
}

type dangerousRule struct {
	re  *regexp.Regexp
	msg string
}

// dangerousRules lists paths that are genuinely unsafe to use as a project
// root. Matching is case-insensitive and full-string anchored.
var dangerousRules = []dangerousRule{
	{regexp.MustCompile(`(?i)^/$`), "Nope. Not the filesystem root. Very bad idea bro ngl."},
	{regexp.MustCompile(`(?i)^/usr(/.*)?$`), "That's a system directory. Pick somewhere else."},
	{regexp.MustCompile(`(?i)^/s?bin(/.*)?$`), "That's a system directory. Pick somewhere else."},
	{regexp.MustCompile(`(?i)^/lib(32|64)?(/.*)?$`), "That's a system directory. Pick somewhere else."},
	{regexp.MustCompile(`(?i)^/boot(/.*)?$`), "The bootloader partition? You got great taste for fucking up your system don't you?"},
	{regexp.MustCompile(`(?i)^/sys(/.*)?$`), "/sys is a kernel interface, not a folder."},
	{regexp.MustCompile(`(?i)^/proc(/.*)?$`), "/proc is a kernel interface, not a folder."},
	{regexp.MustCompile(`(?i)^/dev(/.*)?$`), "/dev is for device files, not projects."},
	{regexp.MustCompile(`(?i)^/var(/.*)?$`), "That's system runtime data territory. Not here."},
	{regexp.MustCompile(`(?i)^/run(/.*)?$`), "/run is for system runtime files. Not here."},
	{regexp.MustCompile(`(?i)^/etc(/.*)?$`), "/etc is for system config. You don't want to go there."},
	{regexp.MustCompile(`(?i)^/lost\+found(/.*)?$`), "/lost+found is a filesystem recovery directory. Not here."},
	{regexp.MustCompile(`(?i)^/snap(/.*)?$`), "/snap is managed by the package manager. Not here."},
	{regexp.MustCompile(`(?i)^/System(/.*)?$`), "That's a macOS system directory protected by SIP. Absolutely not !"},
	{regexp.MustCompile(`(?i)^/Library(/.*)?$`), "That's a macOS system library directory. Pick somewhere else."},
	{regexp.MustCompile(`(?i)^/private/(etc|var|tmp)(/.*)?$`), "That's a macOS system alias. Pick somewhere else."},
	{regexp.MustCompile(`(?i)(^|/)\.git/?$`), "Inside .git? That would corrupt the repository."},
}

// dangerousSegments lists path segments that make a path unsafe regardless of
// where they appear — e.g. inside /home/user/node_modules/myapp.
var dangerousSegments = []struct {
	segment string
	msg     string
}{
	{"node_modules", "What? You shouldn't use node_modules as a project root. I'm not letting you."},
	{"vendor", "What? You shouldn't use vendor as a project root, I'm not letting you."},
}

// isDangerous checks whether the input matches a known dangerous path or
// contains a dangerous segment.
func isDangerous(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}

	expanded := expandPath(trimmed)
	lower := strings.ToLower(expanded)

	// Check for dangerous segments anywhere in the path
	for _, ds := range dangerousSegments {
		seg := strings.ToLower(ds.segment)
		for _, part := range strings.Split(lower, string(filepath.Separator)) {
			if part == seg {
				return ds.msg
			}
		}
	}

	// Check full-path dangerous rules
	for _, rule := range dangerousRules {
		if rule.re.MatchString(expanded) {
			return rule.msg
		}
	}

	return ""
}
