package screens

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/slouowzee/kapi/tui/styles"
)

type gitcfgStep int

const (
	GITCFG_STEP_DETECTING gitcfgStep = iota
	GITCFG_STEP_NO_GIT
	GITCFG_STEP_MENU
	GITCFG_STEP_REMOTE_MENU
	GITCFG_STEP_REMOTE_NAME_INPUT
	GITCFG_STEP_REMOTE_INPUT
	GITCFG_STEP_COLLAB_DETECTING
	GITCFG_STEP_COLLAB_CHECKLIST
	GITCFG_STEP_COLLAB_QUESTIONS
	GITCFG_STEP_CONFIRM_CI
	GITCFG_STEP_SIGNING_DETECTING
	GITCFG_STEP_SIGNING_STATUS
	GITCFG_STEP_SIGNING_FORMAT
	GITCFG_STEP_SIGNING_SCOPE
	GITCFG_STEP_SIGNING_KEY
	GITCFG_STEP_SIGNING_PUSH_GITHUB
	GITCFG_STEP_EXECUTING

	GITCFG_STEP_MANAGE_DETECTING
	GITCFG_STEP_MANAGE_FORMAT
	GITCFG_STEP_MANAGE_LIST
	GITCFG_STEP_MANAGE_CONFIRM_DELETE
	GITCFG_STEP_MANAGE_PUSH_GITHUB
)

const (
	GITCFG_ACTION_REMOTE int = iota
	GITCFG_ACTION_COLLAB
	GITCFG_ACTION_CI
	GITCFG_ACTION_SIGNING
	GITCFG_ACTION_MANAGE_KEYS
	GITCFG_ACTION_BACK
)

var ciOptions = []string{"GitHub Actions", "GitLab CI", "None"}

const signingGenSentinel = "__generate__"

var signingFormats = []string{"GPG", "SSH"}
var signingScopes = []string{"Local", "Global", "Both"}

type collabState struct {
	hasBranchDev      bool
	hasContributing   bool
	hasPRTemplate     bool
	hasIssueTemplates bool
}

type collabQuestion struct {
	label  string
	exists bool
	answer *bool
}

type collabPlan struct {
	createBranch        bool
	writeContributing   bool
	writePRTemplate     bool
	writeIssueTemplates bool
}

func buildCollabQuestions(s collabState) []collabQuestion {
	return []collabQuestion{
		{label: "Create 'dev' branch", exists: s.hasBranchDev},
		{label: "Write CONTRIBUTING.md", exists: s.hasContributing},
		{label: "Write PR template (.github/PULL_REQUEST_TEMPLATE.md)", exists: s.hasPRTemplate},
		{label: "Write Issue templates (.github/ISSUE_TEMPLATE/)", exists: s.hasIssueTemplates},
	}
}

func questionPrompt(q collabQuestion) string {
	if q.exists {
		return fmt.Sprintf("%s — already present, overwrite?", q.label)
	}
	return fmt.Sprintf("%s?", q.label)
}

func planFromAnswers(questions []collabQuestion) collabPlan {
	var p collabPlan
	for i, q := range questions {
		if q.answer == nil || !*q.answer {
			continue
		}
		switch i {
		case 0:
			p.createBranch = true
		case 1:
			p.writeContributing = true
		case 2:
			p.writePRTemplate = true
		case 3:
			p.writeIssueTemplates = true
		}
	}
	return p
}

func contributingContent() string {
	return `# Contributing

## Workflow

` + "```" + `
main          ← production (stable releases only)
└── dev       ← integration (all features merged here first)
    └── feat/your-feature  ← your daily work
` + "```" + `

## Branch naming

` + "```" + `
<type>/<short-description>
` + "```" + `

| Type | Pattern |
|---|---|
| Feature | ` + "`feat/<description>`" + ` |
| Bug fix | ` + "`fix/<description>`" + ` |
| Refactor | ` + "`refactor/<description>`" + ` |
| Documentation | ` + "`docs/<description>`" + ` |
| Chore | ` + "`chore/<description>`" + ` |

## Pull Requests

- One PR = one feature or fix — keep it focused
- Always target ` + "`dev`" + `
- Fill in the PR description
- Assign at least 1 reviewer before submitting
- The author cannot approve their own PR

## Commit messages

Follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/).

` + "```" + `
<type>: <short description in lowercase>

[optional body — explain the why, not the what]
` + "```" + `

## Commit signing

Signing commits is strongly encouraged to verify authorship.
Each collaborator must enable it individually (it is a local machine setting).

` + "```" + `bash
# GPG
git config --local commit.gpgsign true
git config --local user.signingkey <YOUR_GPG_KEY_FINGERPRINT>

# SSH (Git ≥ 2.34)
git config --local gpg.format ssh
git config --local commit.gpgsign true
git config --local user.signingkey ~/.ssh/id_ed25519.pub
` + "```"
}

func prTemplateContent() string {
	return `## Summary

<!-- Describe what this PR does and why. -->

## Changes

- 

## Checklist

- [ ] Tests pass
- [ ] Code follows project conventions
- [ ] PR targets ` + "`dev`" + `, not ` + "`main`" + `
`
}

func bugReportContent() string {
	return `---
name: Bug report
about: Something isn't working as expected
labels: bug
---

## Description

<!-- What happened? What did you expect? -->

## Steps to reproduce

1. 
2. 

## Environment

- OS:
- Version:
`
}

func featureRequestContent() string {
	return `---
name: Feature request
about: Suggest an improvement or new feature
labels: enhancement
---

## Problem

<!-- What problem does this solve? -->

## Proposed solution

<!-- How should it work? -->

## Alternatives considered

<!-- Other approaches you thought about. -->
`
}

// signingKeyLabel returns the display label for the key type based on format.
func signingKeyLabel(format string) string {
	if format == "ssh" {
		return "SSH public key"
	}
	return "GPG key fingerprint"
}

// renderKeyEntries renders a list of signing/manage key entries with cursor.
// genLabel is the label shown for the signingGenSentinel entry.
func (m GitConfigModel) renderKeyEntries(keys []string, cursor int, format, genLabel string) string {
	var sb strings.Builder
	for i, k := range keys {
		var display string
		if k == signingGenSentinel {
			sb.WriteString("\n")
			display = genLabel
		} else if format == "ssh" {
			display = filepath.Base(k)
		} else {
			display = k
			if len(display) > 16 {
				display = display[len(display)-16:]
			}
		}
		if i == cursor {
			fmt.Fprintf(&sb, "%s%s\n", styles.CursorStyle.Render("    ❯❯ "), styles.SelectedStyle.Render(display))
		} else {
			fmt.Fprintf(&sb, "       %s\n", styles.DimStyle.Render(display))
		}
	}
	return sb.String()
}

// renderFormatList renders the GPG/SSH format selector.
// cursor is the currently selected index, avail indicates which formats are available.
func (m GitConfigModel) renderFormatList(cursor int, avail []bool) string {
	var sb strings.Builder
	formatUnavail := []string{"(gpg not found in PATH)", "(ssh-keygen not found in PATH)"}
	for i, opt := range signingFormats {
		switch {
		case !avail[i] && i == cursor:
			fmt.Fprintf(&sb, "%s%s %s\n",
				styles.CursorStyle.Render("    ❯❯ "),
				styles.DimStyle.Render(opt),
				styles.DimStyle.Render(formatUnavail[i]),
			)
		case !avail[i]:
			fmt.Fprintf(&sb, "       %s %s\n",
				styles.DimStyle.Render(opt),
				styles.DimStyle.Render(formatUnavail[i]),
			)
		case i == cursor:
			fmt.Fprintf(&sb, "%s%s\n", styles.CursorStyle.Render("    ❯❯ "), styles.SelectedStyle.Render(opt))
		default:
			fmt.Fprintf(&sb, "       %s\n", styles.DimStyle.Render(opt))
		}
	}
	return sb.String()
}

func (m GitConfigModel) View() string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(styles.TitleStyle.Render("  Git Configuration") + "\n")
	sb.WriteString(styles.DimStyle.Render("  Manage version control for the current repository") + "\n")
	sb.WriteString("\n")

	if m.detecting {
		sb.WriteString(styles.DimStyle.Render("  Detecting Git repository...") + "\n")
		sb.WriteString("\n")
		sb.WriteString(styles.MutedStyle.Render("  [esc] back   [ctrl+c] quit") + "\n")
		return sb.String()
	}

	if m.step == GITCFG_STEP_NO_GIT {
		sb.WriteString(styles.ErrorStyle.Render("  ✗ Not a git repository") + "\n")
		sb.WriteString(styles.DimStyle.Render("  Run 'git init' in this directory first.") + "\n")
		sb.WriteString("\n")
		sb.WriteString(styles.MutedStyle.Render("  [esc] back   [ctrl+c] quit") + "\n")
		return sb.String()
	}

	if m.remoteURL != "" {
		sb.WriteString(styles.SuccessStyle.Render("  ✓ Remote: ") + styles.DimStyle.Render(m.remoteURL) + "\n")
	} else {
		sb.WriteString(styles.DimStyle.Render("  - No remote configured") + "\n")
	}

	if m.lastMsg != "" {
		sb.WriteString("\n")
		if m.lastErr != nil {
			sb.WriteString(styles.ErrorStyle.Render("  ✗ "+m.lastMsg) + "\n")
		} else {
			sb.WriteString(styles.SuccessStyle.Render("  ✓ "+m.lastMsg) + "\n")
		}
	}
	sb.WriteString("\n")

	switch m.step {
	case GITCFG_STEP_MENU:
		sb.WriteString(styles.SelectedStyle.Render("  What would you like to do?") + "\n")
		for i, item := range m.menuItems {
			disabled := m.isActionDisabled(i)
			hint := ""
			if i == GITCFG_ACTION_REMOTE && m.scopesFetched && !m.scopes.Repo {
				hint = " (no GitHub repo scope)"
			}
			switch {
			case disabled && i == m.menuCursor:
				fmt.Fprintf(&sb, "%s%s %s\n",
					styles.CursorStyle.Render("    ❯❯ "),
					styles.DimStyle.Render(item),
					styles.DimStyle.Render("(token scope required)"),
				)
			case disabled:
				fmt.Fprintf(&sb, "       %s %s\n",
					styles.DimStyle.Render(item),
					styles.DimStyle.Render("(token scope required)"),
				)
			case i == m.menuCursor:
				line := item
				if hint != "" {
					line += styles.DimStyle.Render(hint)
				}
				fmt.Fprintf(&sb, "%s%s\n", styles.CursorStyle.Render("    ❯❯ "), styles.SelectedStyle.Render(line))
			default:
				line := styles.DimStyle.Render(item)
				if hint != "" {
					line += styles.DimStyle.Render(hint)
				}
				fmt.Fprintf(&sb, "       %s\n", line)
			}
		}
		if m.scopesFetched && (m.isActionDisabled(GITCFG_ACTION_SIGNING) || !m.scopes.Repo) {
			sb.WriteString("\n")
			sb.WriteString(styles.DimStyle.Render("  Some actions require additional token scopes.") + "\n")
			sb.WriteString(styles.DimStyle.Render("  Run: kapi config github.token --help") + "\n")
		}
		sb.WriteString("\n")
		sb.WriteString(styles.MutedStyle.Render("  [↑↓] navigate   [enter] select   [esc] back   [q] quit") + "\n")

	case GITCFG_STEP_REMOTE_MENU:
		sb.WriteString(styles.SelectedStyle.Render("  Remote Configuration:") + "\n")
		opts := []string{
			"Create private repo on GitHub",
			"Create public repo on GitHub",
			"Enter remote URL manually",
		}
		for i, opt := range opts {
			if (i == 0 || i == 1) && !m.scopes.Repo {
				if i == m.remoteMenuCursor {
					fmt.Fprintf(&sb, "%s%s %s\n", styles.CursorStyle.Render("    ❯❯ "), styles.DimStyle.Render(opt), styles.DimStyle.Render("(requires 'repo' scope)"))
				} else {
					fmt.Fprintf(&sb, "       %s %s\n", styles.DimStyle.Render(opt), styles.DimStyle.Render("(requires 'repo' scope)"))
				}
			} else {
				if i == m.remoteMenuCursor {
					fmt.Fprintf(&sb, "%s%s\n", styles.CursorStyle.Render("    ❯❯ "), styles.SelectedStyle.Render(opt))
				} else {
					fmt.Fprintf(&sb, "       %s\n", styles.DimStyle.Render(opt))
				}
			}
		}
		sb.WriteString("\n")
		sb.WriteString(styles.MutedStyle.Render("  [↑↓] navigate   [enter] select   [esc] back   [q] quit") + "\n")

	case GITCFG_STEP_REMOTE_NAME_INPUT:
		sb.WriteString(styles.SelectedStyle.Render("  Enter repository name for GitHub:") + "\n")
		sb.WriteString("    > " + renderTextInput(m.remoteRepoName, m.remoteNamePos) + "\n")
		sb.WriteString("\n")
		sb.WriteString(styles.MutedStyle.Render("  [enter] confirm   [esc] back   [ctrl+c] quit") + "\n")

	case GITCFG_STEP_REMOTE_INPUT:
		sb.WriteString(styles.SelectedStyle.Render("  Enter remote URL (SSH or HTTPS):") + "\n")
		sb.WriteString("    > " + renderTextInput(m.inputURL, m.inputURLPos) + "\n")
		sb.WriteString("\n")
		sb.WriteString(styles.MutedStyle.Render("  [enter] confirm   [esc] back   [ctrl+c] quit") + "\n")

	case GITCFG_STEP_COLLAB_DETECTING:
		sb.WriteString(styles.DimStyle.Render("  Scanning repository...") + "\n")
		sb.WriteString("\n")

	case GITCFG_STEP_COLLAB_CHECKLIST:
		sb.WriteString(styles.SelectedStyle.Render("  Collaborative setup — current state:") + "\n")
		sb.WriteString("\n")
		sb.WriteString(m.renderCollabChecklist())
		sb.WriteString("\n")
		sb.WriteString(styles.MutedStyle.Render("  [enter] continue   [esc] back   [q] quit") + "\n")

	case GITCFG_STEP_COLLAB_QUESTIONS:
		sb.WriteString(m.renderCollabProgress())
		sb.WriteString("\n")
		if m.collabQIndex < len(m.collabQuestions) {
			q := m.collabQuestions[m.collabQIndex]
			sb.WriteString(styles.SelectedStyle.Render("  "+questionPrompt(q)) + "\n")
			for i, opt := range []string{"Yes", "No"} {
				if i == m.collabCursor {
					fmt.Fprintf(&sb, "%s%s\n", styles.CursorStyle.Render("    ❯❯ "), styles.SelectedStyle.Render(opt))
				} else {
					fmt.Fprintf(&sb, "       %s\n", styles.DimStyle.Render(opt))
				}
			}
		}
		sb.WriteString("\n")
		sb.WriteString(styles.MutedStyle.Render("  [↑↓] navigate   [enter] select   [esc] back to checklist   [q] quit") + "\n")

	case GITCFG_STEP_CONFIRM_CI:
		sb.WriteString(styles.SelectedStyle.Render("  Generate CI/CD workflows?") + "\n")
		sb.WriteString(styles.DimStyle.Render("    (Automated linting and testing pipelines)") + "\n")
		for i, opt := range ciOptions {
			if i == m.ciCursor {
				fmt.Fprintf(&sb, "%s%s\n", styles.CursorStyle.Render("    ❯❯ "), styles.SelectedStyle.Render(opt))
			} else {
				fmt.Fprintf(&sb, "       %s\n", styles.DimStyle.Render(opt))
			}
		}
		sb.WriteString("\n")
		sb.WriteString(styles.MutedStyle.Render("  [↑↓] navigate   [enter] select   [esc] back   [q] quit") + "\n")

	case GITCFG_STEP_EXECUTING, GITCFG_STEP_SIGNING_PUSH_GITHUB, GITCFG_STEP_MANAGE_PUSH_GITHUB:
		sb.WriteString(styles.DimStyle.Render("  "+m.execMsg) + "\n")
		sb.WriteString("\n")

	case GITCFG_STEP_SIGNING_DETECTING:
		sb.WriteString(styles.DimStyle.Render("  Detecting signing configuration...") + "\n")
		sb.WriteString("\n")

	case GITCFG_STEP_SIGNING_STATUS:
		sb.WriteString(styles.SelectedStyle.Render("  Commit signing — current state:") + "\n")
		sb.WriteString("\n")
		if m.signingLocalActive {
			sb.WriteString(styles.SuccessStyle.Render("  ✓ Local signing enabled") + "\n")
		} else {
			sb.WriteString(styles.DimStyle.Render("  ✗ Local signing disabled") + "\n")
		}
		if m.signingGlobalActive {
			sb.WriteString(styles.SuccessStyle.Render("  ✓ Global signing enabled") + "\n")
		} else {
			sb.WriteString(styles.DimStyle.Render("  ✗ Global signing disabled") + "\n")
		}
		sb.WriteString("\n")
		gpgCount := len(m.signingGPGKeys)
		sshCount := len(m.signingSSHKeys)
		if m.signingGPGAvailable {
			sb.WriteString(styles.DimStyle.Render(fmt.Sprintf("  GPG keys: %d", gpgCount)) + "\n")
		} else {
			sb.WriteString(styles.DimStyle.Render("  GPG: not available (gpg not found in PATH)") + "\n")
		}
		if m.signingSSHKeygenAvail {
			sb.WriteString(styles.DimStyle.Render(fmt.Sprintf("  SSH keys: %d", sshCount)) + "\n")
		} else {
			sb.WriteString(styles.DimStyle.Render("  SSH: not available (ssh-keygen not found in PATH)") + "\n")
		}
		sb.WriteString("\n")
		sb.WriteString(styles.MutedStyle.Render("  [enter] configure   [esc] back   [q] quit") + "\n")

	case GITCFG_STEP_SIGNING_FORMAT:
		sb.WriteString(styles.SelectedStyle.Render("  Signing format:") + "\n")
		avail := []bool{m.signingGPGAvailable, m.signingSSHKeygenAvail}
		sb.WriteString(m.renderFormatList(m.signingFormatCursor, avail))
		sb.WriteString("\n")
		sb.WriteString(styles.MutedStyle.Render("  [↑↓] navigate   [enter] select   [esc] back   [q] quit") + "\n")

	case GITCFG_STEP_SIGNING_SCOPE:
		sb.WriteString(styles.SelectedStyle.Render("  Apply signing to:") + "\n")
		for i, opt := range signingScopes {
			if i == m.signingScopeCursor {
				fmt.Fprintf(&sb, "%s%s\n", styles.CursorStyle.Render("    ❯❯ "), styles.SelectedStyle.Render(opt))
			} else {
				fmt.Fprintf(&sb, "       %s\n", styles.DimStyle.Render(opt))
			}
		}
		sb.WriteString("\n")
		sb.WriteString(styles.MutedStyle.Render("  [↑↓] navigate   [enter] select   [esc] back   [q] quit") + "\n")

	case GITCFG_STEP_SIGNING_KEY:
		keys := m.signingKeys()
		label := signingKeyLabel(m.signingFormat)
		genLabel := "Generate a new GPG key  (gpg --gen-key)"
		if m.signingFormat == "ssh" {
			genLabel = "Generate a new SSH key  (ssh-keygen -t ed25519)"
		}
		sb.WriteString(styles.SelectedStyle.Render("  Select "+label+":") + "\n")
		sb.WriteString(m.renderKeyEntries(keys, m.signingKeyCursor, m.signingFormat, genLabel))
		sb.WriteString("\n")
		sb.WriteString(styles.MutedStyle.Render("  [↑↓] navigate   [enter] select/generate   [esc] back   [q] quit") + "\n")

	case GITCFG_STEP_MANAGE_DETECTING:
		sb.WriteString(styles.DimStyle.Render("  Detecting signing configuration...") + "\n")
		sb.WriteString("\n")

	case GITCFG_STEP_MANAGE_FORMAT:
		sb.WriteString(styles.SelectedStyle.Render("  Manage keys - Format:") + "\n")
		avail := []bool{m.signingGPGAvailable, m.signingSSHKeygenAvail}
		sb.WriteString(m.renderFormatList(m.manageFormatCursor, avail))
		sb.WriteString("\n")
		sb.WriteString(styles.MutedStyle.Render("  [↑↓] navigate   [enter] select   [esc] back   [q] quit") + "\n")

	case GITCFG_STEP_MANAGE_LIST:
		keys := m.manageKeysList()
		label := signingKeyLabel(m.manageFormat)
		genLabel := "Generate a new GPG key  (gpg --gen-key)"
		if m.manageFormat == "ssh" {
			genLabel = "Generate a new SSH key  (ssh-keygen -t ed25519)"
		}
		sb.WriteString(styles.SelectedStyle.Render("  Manage "+label+"s:") + "\n")
		sb.WriteString(m.renderKeyEntries(keys, m.manageListCursor, m.manageFormat, genLabel))
		sb.WriteString("\n")
		sb.WriteString(styles.MutedStyle.Render("  [↑↓] navigate   [enter] push to github/generate   [x] delete key   [esc] back   [q] quit") + "\n")

	case GITCFG_STEP_MANAGE_CONFIRM_DELETE:
		sb.WriteString(styles.ErrorStyle.Render("  Are you sure you want to delete this key from your machine?") + "\n")
		label := m.manageKeyToDelete
		if m.manageFormat == "ssh" {
			label = filepath.Base(label)
		}
		display := label
		if m.manageFormat == "gpg" && len(label) > 16 {
			display = label[len(label)-16:]
		}
		sb.WriteString(styles.DimStyle.Render("    "+display) + "\n\n")

		opts := []string{"No, keep it", "Yes, delete it"}
		for i, opt := range opts {
			if i == m.manageDeleteCursor {
				fmt.Fprintf(&sb, "%s%s\n", styles.CursorStyle.Render("    ❯❯ "), styles.SelectedStyle.Render(opt))
			} else {
				fmt.Fprintf(&sb, "       %s\n", styles.DimStyle.Render(opt))
			}
		}
		sb.WriteString("\n")
		sb.WriteString(styles.MutedStyle.Render("  [↑↓] navigate   [enter] select   [esc] back   [q] quit") + "\n")
	}

	return sb.String()
}

func (m GitConfigModel) renderCollabChecklist() string {
	var sb strings.Builder
	s := m.collabState
	items := []struct {
		label  string
		exists bool
	}{
		{"dev branch", s.hasBranchDev},
		{"CONTRIBUTING.md", s.hasContributing},
		{"PR template", s.hasPRTemplate},
		{"Issue templates", s.hasIssueTemplates},
	}
	for _, item := range items {
		if item.exists {
			sb.WriteString(styles.SuccessStyle.Render("  ✓ "+item.label) + "\n")
		} else {
			sb.WriteString(styles.DimStyle.Render("  ✗ "+item.label) + "\n")
		}
	}
	return sb.String()
}

func (m GitConfigModel) renderCollabProgress() string {
	var sb strings.Builder
	for i := 0; i < m.collabQIndex; i++ {
		q := m.collabQuestions[i]
		if q.answer != nil && *q.answer {
			sb.WriteString(styles.SuccessStyle.Render("  ✓ "+q.label) + "\n")
		} else {
			sb.WriteString(styles.DimStyle.Render("  - "+q.label+" (skip)") + "\n")
		}
	}
	return sb.String()
}
