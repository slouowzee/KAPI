package screens

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slouowzee/kapi/tui/styles"
)

const (
	gitFieldInit   = 0
	gitFieldRemote = 1
	gitFieldURL    = 2
	gitFieldCollab = 3
	gitFieldCI     = 4
)

const (
	gitInitNo  = 0
	gitInitYes = 1
)

const (
	gitRemoteSkip          = 0
	gitRemoteGithubPrivate = 1
	gitRemoteGithubPublic  = 2
	gitRemoteExisting      = 3
)

const (
	gitCollabNo  = 0
	gitCollabYes = 1
)

const (
	gitCINone   = 0
	gitCIGitHub = 1
	gitCIGitLab = 2
)

func inferRemoteHost(url string) string {
	lower := strings.ToLower(url)
	if strings.Contains(lower, "github.com") {
		return "github"
	}
	if strings.Contains(lower, "gitlab.com") {
		return "gitlab"
	}
	return "custom"
}

func existingAncestor(dir string) string {
	for {
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		dir = parent
	}
}

type GitConfig struct {
	InitLocal      bool
	HasExistingGit bool

	RemoteURL     string
	RemoteHost    string
	RemotePrivate bool
	Collab        bool
	CI            string
}

type gitDetectionMsg struct {
	hasGit    bool
	remoteURL string
}

func detectGitCmd(targetDir string) tea.Cmd {
	return func() tea.Msg {
		var hasGit bool
		var remoteURL string

		checkDir := existingAncestor(targetDir)

		cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
		cmd.Dir = checkDir
		out, err := cmd.Output()
		if err == nil && strings.TrimSpace(string(out)) == "true" {
			hasGit = true

			cmdRemote := exec.Command("git", "remote", "get-url", "origin")
			cmdRemote.Dir = checkDir
			outRemote, err := cmdRemote.Output()
			if err == nil {
				remoteURL = strings.TrimSpace(string(outRemote))
			}
		}

		return gitDetectionMsg{hasGit: hasGit, remoteURL: remoteURL}
	}
}

type GitModel struct {
	width     int
	height    int
	targetDir string

	detecting   bool
	hasGit      bool
	detectedURL string

	cursor int

	initOpt   int 
	remoteOpt int 
	collabOpt int 
	ciOpt     int 

	urlInput    string
	urlInputPos int
	urlEditing  bool
	done        bool
	backPressed bool
}

func Git(width, height int, targetDir string, cfg GitConfig) GitModel {
	if cfg == (GitConfig{}) {
		return GitModel{
			width:     width,
			height:    height,
			targetDir: targetDir,
			detecting: true,
		}
	}

	m := GitModel{
		width:       width,
		height:      height,
		targetDir:   targetDir,
		detecting:   false,
		hasGit:      cfg.HasExistingGit,
		detectedURL: cfg.RemoteURL,
		cursor:      gitFieldCI,
	}
	if cfg.InitLocal {
		m.initOpt = gitInitYes
	}
	switch {
	case cfg.HasExistingGit && cfg.RemoteURL != "":
		m.remoteOpt = gitRemoteExisting
		m.urlInput = cfg.RemoteURL
	case cfg.RemoteHost == "github" && cfg.RemotePrivate:
		m.remoteOpt = gitRemoteGithubPrivate
	case cfg.RemoteHost == "github" && !cfg.RemotePrivate:
		m.remoteOpt = gitRemoteGithubPublic
	case cfg.RemoteURL != "":
		m.remoteOpt = gitRemoteExisting
		m.urlInput = cfg.RemoteURL
	default:
		m.remoteOpt = gitRemoteSkip
	}
	if cfg.Collab {
		m.collabOpt = gitCollabYes
	}
	switch cfg.CI {
	case ciChoiceGitHub:
		m.ciOpt = gitCIGitHub
	case ciChoiceGitLab:
		m.ciOpt = gitCIGitLab
	default:
		m.ciOpt = gitCINone
	}
	return m
}

func (m *GitModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m GitModel) Config() GitConfig {
	cfg := GitConfig{
		InitLocal:      m.initOpt == gitInitYes,
		HasExistingGit: m.hasGit,
		Collab:         m.collabOpt == gitCollabYes,
	}
	switch m.ciOpt {
	case gitCIGitHub:
		cfg.CI = ciChoiceGitHub
	case gitCIGitLab:
		cfg.CI = ciChoiceGitLab
	default:
		cfg.CI = ciChoiceNone
	}
	if m.hasGit && m.detectedURL != "" {
		cfg.RemoteURL = m.detectedURL
		cfg.RemoteHost = inferRemoteHost(m.detectedURL)
	} else {
		switch m.remoteOpt {
		case gitRemoteGithubPrivate:
			cfg.RemoteHost = "github"
			cfg.RemotePrivate = true
		case gitRemoteGithubPublic:
			cfg.RemoteHost = "github"
			cfg.RemotePrivate = false
		case gitRemoteExisting:
			cfg.RemoteURL = strings.TrimSpace(m.urlInput)
			cfg.RemoteHost = inferRemoteHost(cfg.RemoteURL)
		}
	}
	return cfg
}

func (m GitModel) Done() bool        { return m.done }
func (m *GitModel) ConsumeDone()     { m.done = false }
func (m GitModel) IsBack() bool      { return m.backPressed }
func (m *GitModel) ConsumeBack()     { m.backPressed = false }
func (m GitModel) IsInputMode() bool { return m.urlEditing }

func (m GitModel) Init() tea.Cmd {
	return detectGitCmd(m.targetDir)
}

func (m GitModel) Update(msg tea.Msg) (GitModel, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case gitDetectionMsg:
		m.detecting = false
		m.hasGit = msg.hasGit
		m.detectedURL = msg.remoteURL
		if m.hasGit {
			m.initOpt = gitInitNo
			if msg.remoteURL != "" {
				m.remoteOpt = gitRemoteExisting
				m.urlInput = msg.remoteURL
			}
		}

	case tea.KeyMsg:
		if m.detecting {
			break
		}
		if m.urlEditing {
			return m.handleURLInput(msg)
		}
		switch msg.String() {
		case "up", "k":
			m.moveCursor(-1)
		case "down", "j":
			m.moveCursor(1)
		case "left", "h":
			m.cycleOption(-1)
		case "right", "l":
			m.cycleOption(1)
		case "enter":
			m.handleEnter()
		case "esc":
			m.backPressed = true
		}
	}
	return m, nil
}

func (m GitModel) firstField() int {
	if m.hasGit {
		return gitFieldCollab
	}
	return gitFieldInit
}

func (m GitModel) lastField() int {
	return gitFieldCI
}

func (m *GitModel) moveCursor(delta int) {
	next := m.cursor + delta
	if next == gitFieldURL && m.remoteOpt != gitRemoteExisting {
		next += delta
	}
	first := m.firstField()
	last := m.lastField()
	if next < first {
		next = first
	}
	if next > last {
		next = last
	}
	m.cursor = next
}

func (m *GitModel) cycleOption(delta int) {
	switch m.cursor {
	case gitFieldInit:
		m.initOpt = clamp(m.initOpt+delta, gitInitNo, gitInitYes)
	case gitFieldRemote:
		if m.hasGit && m.detectedURL != "" {
			return
		}
		m.remoteOpt = clamp(m.remoteOpt+delta, 0, 3)
	case gitFieldCollab:
		m.collabOpt = clamp(m.collabOpt+delta, gitCollabNo, gitCollabYes)
	case gitFieldCI:
		m.ciOpt = clamp(m.ciOpt+delta, 0, 2)
	}
}

func (m *GitModel) handleEnter() {
	switch m.cursor {
	case gitFieldURL:
		m.urlEditing = true
	case gitFieldCI:
		m.done = true
	default:
		m.moveCursor(1)
	}
}

func (m GitModel) handleURLInput(msg tea.KeyMsg) (GitModel, tea.Cmd) {
	runes := []rune(m.urlInput)
	switch msg.String() {
	case "esc":
		m.urlEditing = false
	case "enter":
		m.urlEditing = false
		m.moveCursor(1)
	case "left":
		if m.urlInputPos > 0 {
			m.urlInputPos--
		}
	case "right":
		if m.urlInputPos < len(runes) {
			m.urlInputPos++
		}
	case "backspace":
		if m.urlInputPos > 0 {
			m.urlInput = string(append(runes[:m.urlInputPos-1], runes[m.urlInputPos:]...))
			m.urlInputPos--
		}
	case "delete":
		if m.urlInputPos < len(runes) {
			m.urlInput = string(append(runes[:m.urlInputPos], runes[m.urlInputPos+1:]...))
		}
	default:
		if len(msg.Runes) > 0 {
			m.urlInput = string(append(runes[:m.urlInputPos], append(msg.Runes, runes[m.urlInputPos:]...)...))
			m.urlInputPos += len(msg.Runes)
		}
	}
	return m, nil
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func (m GitModel) View() string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(styles.TitleStyle.Render("  Git Configuration") + "\n")
	sb.WriteString(styles.DimStyle.Render("  Set up version control and collaboration") + "\n")
	sb.WriteString("\n")

	if m.detecting {
		sb.WriteString(styles.DimStyle.Render("  Detecting Git repository...") + "\n")
		sb.WriteString("\n")
		sb.WriteString(styles.MutedStyle.Render("  [esc] back") + "\n")
		return sb.String()
	}

	if m.hasGit {
		sb.WriteString(m.renderLockedRow("Local repo", "repository detected"))
	} else {
		opts := []string{"No", "Yes"}
		sb.WriteString(m.renderRow(gitFieldInit, "Local repo", opts, m.initOpt))
	}

	if m.hasGit && m.detectedURL != "" {
		sb.WriteString(m.renderLockedRow("Remote", m.detectedURL))
	} else {
		opts := []string{"Skip", "GitHub private", "GitHub public", "Existing URL"}
		sb.WriteString(m.renderRow(gitFieldRemote, "Remote", opts, m.remoteOpt))
	}

	if m.remoteOpt == gitRemoteExisting && !(m.hasGit && m.detectedURL != "") {
		sb.WriteString(m.renderURLRow())
	}

	collabOpts := []string{"No", "Yes"}
	sb.WriteString(m.renderRow(gitFieldCollab, "Collab", collabOpts, m.collabOpt))

	ciOpts := []string{"None", "GitHub Actions", "GitLab CI"}
	sb.WriteString(m.renderRow(gitFieldCI, "CI/CD", ciOpts, m.ciOpt))

	sb.WriteString("\n")
	if m.urlEditing {
		sb.WriteString(styles.MutedStyle.Render("  [←→] move   [↵] confirm   [esc] cancel") + "\n")
	} else {
		sb.WriteString(styles.MutedStyle.Render("  [↑↓] field   [←→] option   [↵] next / confirm   [esc] back   [q] quit") + "\n")
	}
	return sb.String()
}

func (m GitModel) renderRow(field int, label string, opts []string, selected int) string {
	isFocused := m.cursor == field
	paddedLabel := fmt.Sprintf("%-14s", label)

	var optStr strings.Builder
	for i, opt := range opts {
		if i > 0 {
			optStr.WriteString(styles.DimStyle.Render("  ·  "))
		}
		if i == selected {
			if isFocused {
				optStr.WriteString(styles.SelectedStyle.Render(opt))
			} else {
				optStr.WriteString(styles.SubtitleStyle.Render(opt))
			}
		} else {
			optStr.WriteString(styles.DimStyle.Render(opt))
		}
	}

	if isFocused {
		return fmt.Sprintf("%s%s\n",
			styles.CursorStyle.Render("  ❯❯"),
			styles.SelectedStyle.Render(" "+paddedLabel)+optStr.String(),
		)
	}
	return fmt.Sprintf("      %s%s\n",
		styles.MutedStyle.Render(paddedLabel),
		optStr.String(),
	)
}

func (m GitModel) renderLockedRow(label, value string) string {
	paddedLabel := fmt.Sprintf("%-14s", label)
	return fmt.Sprintf("      %s%s\n",
		styles.MutedStyle.Render(paddedLabel),
		styles.SuccessStyle.Render(value),
	)
}

func (m GitModel) renderURLRow() string {
	isFocused := m.cursor == gitFieldURL
	paddedLabel := fmt.Sprintf("%-14s", "URL")

	var content string
	if m.urlEditing {
		content = renderTextInput(m.urlInput, m.urlInputPos)
	} else if m.urlInput != "" {
		content = styles.SubtitleStyle.Render(m.urlInput)
	} else {
		content = styles.DimStyle.Render("enter remote URL…")
	}

	if isFocused {
		return fmt.Sprintf("%s%s\n",
			styles.CursorStyle.Render("  ❯❯"),
			styles.SelectedStyle.Render(" "+paddedLabel)+content,
		)
	}
	return fmt.Sprintf("      %s%s\n",
		styles.MutedStyle.Render(paddedLabel),
		content,
	)
}
