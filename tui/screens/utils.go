package screens

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/slouowzee/kapi/tui/styles"
)

const MAX_PATH_LEN = 50

const (
	ciChoiceGitHub = "github"
	ciChoiceGitLab = "gitlab"
	ciChoiceNone   = "none"
)

func truncatePath(path string) string {
	runes := []rune(path)
	if len(runes) <= MAX_PATH_LEN {
		return path
	}
	return "…" + string(runes[len(runes)-MAX_PATH_LEN+1:])
}

var easterEggRe = regexp.MustCompile(`(?i)^fuck you$`)

var tmpRe = regexp.MustCompile(`(?i)^/tmp(/.*)?$`)

func detectEasterEgg(input string) string {
	trimmed := strings.TrimSpace(input)
	if easterEggRe.MatchString(trimmed) {
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

func renderTextInput(text string, pos int) string {
	runes := []rune(text)
	before := string(runes[:pos])
	after := string(runes[pos:])
	cursor := styles.CursorStyle.Render("█")
	if len(after) > 0 {
		afterRunes := []rune(after)
		cursor = styles.CursorStyle.Render(string(afterRunes[0]))
		after = string(afterRunes[1:])
	}
	return before + cursor + after
}

func scrollWindow(cursor, total, visible int) (start, end int) {
	start = cursor - visible/2
	if start < 0 {
		start = 0
	}
	if start+visible > total {
		start = total - visible
	}
	if start < 0 {
		start = 0
	}
	end = start + visible
	if end > total {
		end = total
	}
	return
}

func isDangerous(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}

	expanded := expandPath(trimmed)
	lower := strings.ToLower(expanded)

	for _, ds := range dangerousSegments {
		seg := strings.ToLower(ds.segment)
		for _, part := range strings.Split(lower, string(filepath.Separator)) {
			if part == seg {
				return ds.msg
			}
		}
	}

	for _, rule := range dangerousRules {
		if rule.re.MatchString(expanded) {
			return rule.msg
		}
	}

	return ""
}
