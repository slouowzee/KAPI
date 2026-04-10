package screens

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestInferRemoteHost_GitHub(t *testing.T) {
	cases := []string{
		"https://github.com/user/repo.git",
		"git@github.com:user/repo.git",
		"HTTPS://GITHUB.COM/user/repo",
	}
	for _, url := range cases {
		if got := inferRemoteHost(url); got != "github" {
			t.Errorf("inferRemoteHost(%q) = %q, want github", url, got)
		}
	}
}

func TestInferRemoteHost_GitLab(t *testing.T) {
	cases := []string{
		"https://gitlab.com/user/repo.git",
		"git@gitlab.com:user/repo.git",
	}
	for _, url := range cases {
		if got := inferRemoteHost(url); got != "gitlab" {
			t.Errorf("inferRemoteHost(%q) = %q, want gitlab", url, got)
		}
	}
}

func TestInferRemoteHost_Custom(t *testing.T) {
	cases := []string{
		"https://bitbucket.org/user/repo.git",
		"git@mygit.internal:user/repo.git",
		"",
	}
	for _, url := range cases {
		if got := inferRemoteHost(url); got != "custom" {
			t.Errorf("inferRemoteHost(%q) = %q, want custom", url, got)
		}
	}
}

func TestClamp(t *testing.T) {
	tests := []struct {
		v, min, max, want int
	}{
		{5, 0, 10, 5},
		{-1, 0, 10, 0},
		{11, 0, 10, 10},
		{0, 0, 0, 0},
	}
	for _, tt := range tests {
		if got := clamp(tt.v, tt.min, tt.max); got != tt.want {
			t.Errorf("clamp(%d, %d, %d) = %d, want %d", tt.v, tt.min, tt.max, got, tt.want)
		}
	}
}

func newDetectedGitModel() GitModel {
	return GitModel{
		width:     80,
		height:    24,
		targetDir: "/home/user/myproject",
		detecting: false,
	}
}

func TestGitModelConfig_GithubPrivate_RepoName(t *testing.T) {
	m := newDetectedGitModel()
	m.remoteOpt = gitRemoteGithubPrivate
	m.repoNameInput = "  my-repo  "

	cfg := m.Config()

	if cfg.RemoteHost != "github" {
		t.Errorf("RemoteHost = %q, want github", cfg.RemoteHost)
	}
	if !cfg.RemotePrivate {
		t.Error("RemotePrivate should be true for gitRemoteGithubPrivate")
	}
	if cfg.RepoName != "my-repo" {
		t.Errorf("RepoName = %q, want my-repo (trimmed)", cfg.RepoName)
	}
}

func TestGitModelConfig_GithubPublic_RepoName(t *testing.T) {
	m := newDetectedGitModel()
	m.remoteOpt = gitRemoteGithubPublic
	m.repoNameInput = "public-repo"

	cfg := m.Config()

	if cfg.RemoteHost != "github" {
		t.Errorf("RemoteHost = %q, want github", cfg.RemoteHost)
	}
	if cfg.RemotePrivate {
		t.Error("RemotePrivate should be false for gitRemoteGithubPublic")
	}
	if cfg.RepoName != "public-repo" {
		t.Errorf("RepoName = %q, want public-repo", cfg.RepoName)
	}
}

func TestGitModelConfig_ExistingURL(t *testing.T) {
	m := newDetectedGitModel()
	m.remoteOpt = gitRemoteExisting
	m.urlInput = "https://github.com/user/repo.git"

	cfg := m.Config()

	if cfg.RemoteURL != "https://github.com/user/repo.git" {
		t.Errorf("RemoteURL = %q, want the url", cfg.RemoteURL)
	}
	if cfg.RemoteHost != "github" {
		t.Errorf("RemoteHost = %q, want github (inferred from url)", cfg.RemoteHost)
	}
	if cfg.RepoName != "" {
		t.Errorf("RepoName = %q, want empty for ExistingURL", cfg.RepoName)
	}
}

func TestGitModelConfig_SkipRemote(t *testing.T) {
	m := newDetectedGitModel()
	m.remoteOpt = gitRemoteSkip

	cfg := m.Config()

	if cfg.RemoteHost != "" {
		t.Errorf("RemoteHost = %q, want empty for skip", cfg.RemoteHost)
	}
	if cfg.RepoName != "" {
		t.Errorf("RepoName = %q, want empty for skip", cfg.RepoName)
	}
}

func TestGitModelConfig_InitLocal(t *testing.T) {
	m := newDetectedGitModel()
	m.initOpt = gitInitYes

	cfg := m.Config()
	if !cfg.InitLocal {
		t.Error("InitLocal should be true when initOpt=gitInitYes")
	}

	m.initOpt = gitInitNo
	cfg = m.Config()
	if cfg.InitLocal {
		t.Error("InitLocal should be false when initOpt=gitInitNo")
	}
}

func TestGitModelConfig_CI(t *testing.T) {
	m := newDetectedGitModel()

	m.ciOpt = gitCIGitHub
	if got := m.Config().CI; got != ciChoiceGitHub {
		t.Errorf("CI = %q, want %q", got, ciChoiceGitHub)
	}

	m.ciOpt = gitCIGitLab
	if got := m.Config().CI; got != ciChoiceGitLab {
		t.Errorf("CI = %q, want %q", got, ciChoiceGitLab)
	}

	m.ciOpt = gitCINone
	if got := m.Config().CI; got != ciChoiceNone {
		t.Errorf("CI = %q, want %q", got, ciChoiceNone)
	}
}

func TestGitModelConfig_Collab(t *testing.T) {
	m := newDetectedGitModel()

	m.collabOpt = gitCollabYes
	if !m.Config().Collab {
		t.Error("Collab should be true when collabOpt=gitCollabYes")
	}

	m.collabOpt = gitCollabNo
	if m.Config().Collab {
		t.Error("Collab should be false when collabOpt=gitCollabNo")
	}
}

func TestMoveCursor_SkipsRepoNameWhenNotGithub(t *testing.T) {
	m := newDetectedGitModel()
	m.cursor = gitFieldRemote
	m.remoteOpt = gitRemoteSkip

	m.moveCursor(1)

	if m.cursor == gitFieldRepoName {
		t.Error("cursor should skip gitFieldRepoName when remoteOpt is not github")
	}
}

func TestMoveCursor_SkipsURLWhenNotExisting(t *testing.T) {
	m := newDetectedGitModel()
	m.cursor = gitFieldRemote
	m.remoteOpt = gitRemoteSkip

	m.moveCursor(1)

	if m.cursor == gitFieldURL {
		t.Error("cursor should skip gitFieldURL when remoteOpt is not gitRemoteExisting")
	}
}

func TestMoveCursor_LandsOnRepoNameWhenGithub(t *testing.T) {
	m := newDetectedGitModel()
	m.cursor = gitFieldRemote
	m.remoteOpt = gitRemoteGithubPrivate

	m.moveCursor(1)

	if m.cursor != gitFieldRepoName {
		t.Errorf("cursor = %d, want gitFieldRepoName (%d)", m.cursor, gitFieldRepoName)
	}
}

func TestMoveCursor_LandsOnURLWhenExisting(t *testing.T) {
	m := newDetectedGitModel()
	m.cursor = gitFieldRemote
	m.remoteOpt = gitRemoteExisting

	m.moveCursor(1)

	if m.cursor != gitFieldURL {
		t.Errorf("cursor = %d, want gitFieldURL (%d)", m.cursor, gitFieldURL)
	}
}

func TestMoveCursor_ClampedAtFirstField(t *testing.T) {
	m := newDetectedGitModel()
	m.cursor = gitFieldInit
	m.moveCursor(-1)

	if m.cursor < m.firstField() {
		t.Errorf("cursor %d is below firstField %d", m.cursor, m.firstField())
	}
}

func TestMoveCursor_ClampedAtLastField(t *testing.T) {
	m := newDetectedGitModel()
	m.cursor = gitFieldCI
	m.moveCursor(1)

	if m.cursor > m.lastField() {
		t.Errorf("cursor %d exceeds lastField %d", m.cursor, m.lastField())
	}
}

func gitKeyMsg(key string) tea.KeyMsg {
	switch key {
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "delete":
		return tea.KeyMsg{Type: tea.KeyDelete}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}

func TestHandleRepoNameInput_TypeCharacter(t *testing.T) {
	m := newDetectedGitModel()
	m.repoNameEditing = true
	m.repoNameInput = "hello"
	m.repoNameInputPos = 5

	m, _ = m.handleRepoNameInput(gitKeyMsg("!"))

	if m.repoNameInput != "hello!" {
		t.Errorf("repoNameInput = %q, want hello!", m.repoNameInput)
	}
	if m.repoNameInputPos != 6 {
		t.Errorf("repoNameInputPos = %d, want 6", m.repoNameInputPos)
	}
}

func TestHandleRepoNameInput_Backspace(t *testing.T) {
	m := newDetectedGitModel()
	m.repoNameEditing = true
	m.repoNameInput = "hello"
	m.repoNameInputPos = 5

	m, _ = m.handleRepoNameInput(gitKeyMsg("backspace"))

	if m.repoNameInput != "hell" {
		t.Errorf("repoNameInput = %q, want hell", m.repoNameInput)
	}
	if m.repoNameInputPos != 4 {
		t.Errorf("repoNameInputPos = %d, want 4", m.repoNameInputPos)
	}
}

func TestHandleRepoNameInput_EscCancels(t *testing.T) {
	m := newDetectedGitModel()
	m.repoNameEditing = true
	m.repoNameInput = "hello"

	m, _ = m.handleRepoNameInput(gitKeyMsg("esc"))

	if m.repoNameEditing {
		t.Error("repoNameEditing should be false after esc")
	}
	if m.repoNameInput != "hello" {
		t.Error("repoNameInput should not change on esc")
	}
}

func TestHandleRepoNameInput_EnterConfirms(t *testing.T) {
	m := newDetectedGitModel()
	m.repoNameEditing = true
	m.cursor = gitFieldRepoName
	m.remoteOpt = gitRemoteGithubPrivate

	m, _ = m.handleRepoNameInput(gitKeyMsg("enter"))

	if m.repoNameEditing {
		t.Error("repoNameEditing should be false after enter")
	}
}

func TestHandleRepoNameInput_DeleteAtCursor(t *testing.T) {
	m := newDetectedGitModel()
	m.repoNameEditing = true
	m.repoNameInput = "hello"
	m.repoNameInputPos = 2

	m, _ = m.handleRepoNameInput(gitKeyMsg("delete"))

	if m.repoNameInput != "helo" {
		t.Errorf("repoNameInput = %q, want helo (delete char at cursor)", m.repoNameInput)
	}
	if m.repoNameInputPos != 2 {
		t.Errorf("repoNameInputPos should stay 2 after delete, got %d", m.repoNameInputPos)
	}
}

func TestIsInputMode(t *testing.T) {
	m := newDetectedGitModel()

	if m.IsInputMode() {
		t.Error("IsInputMode should be false initially")
	}

	m.repoNameEditing = true
	if !m.IsInputMode() {
		t.Error("IsInputMode should be true when repoNameEditing")
	}

	m.repoNameEditing = false
	m.urlEditing = true
	if !m.IsInputMode() {
		t.Error("IsInputMode should be true when urlEditing")
	}
}
