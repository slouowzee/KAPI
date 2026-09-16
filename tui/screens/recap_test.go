package screens

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slouowzee/kapi/internal/registry"
)

func gitValue(cfg GitConfig) string {
	m := RecapModel{gitCfg: cfg}
	return m.gitValue()
}

func TestGitValue_None(t *testing.T) {
	cfg := GitConfig{}
	if got := gitValue(cfg); got != "none" {
		t.Errorf("gitValue() = %q, want none", got)
	}
}

func TestGitValue_LocalOnly(t *testing.T) {
	cfg := GitConfig{InitLocal: true}
	got := gitValue(cfg)
	if got == "none" {
		t.Error("gitValue() should not be none when InitLocal=true")
	}
	if !strings.Contains(got, "local") {
		t.Errorf("gitValue() = %q, want it to contain 'local'", got)
	}
}

func TestGitValue_GithubPrivate_WithRepoName(t *testing.T) {
	cfg := GitConfig{
		InitLocal:     true,
		RemoteHost:    "github",
		RemotePrivate: true,
		RepoName:      "my-project",
	}
	got := gitValue(cfg)
	if !strings.Contains(got, "github (private)") {
		t.Errorf("gitValue() = %q, want to contain 'github (private)'", got)
	}
	if !strings.Contains(got, "my-project") {
		t.Errorf("gitValue() = %q, want to contain repo name 'my-project'", got)
	}
}

func TestGitValue_GithubPublic_WithRepoName(t *testing.T) {
	cfg := GitConfig{
		InitLocal:     true,
		RemoteHost:    "github",
		RemotePrivate: false,
		RepoName:      "open-source",
	}
	got := gitValue(cfg)
	if !strings.Contains(got, "github (public)") {
		t.Errorf("gitValue() = %q, want to contain 'github (public)'", got)
	}
	if !strings.Contains(got, "open-source") {
		t.Errorf("gitValue() = %q, want to contain repo name 'open-source'", got)
	}
}

func TestGitValue_GithubPrivate_WithoutRepoName(t *testing.T) {
	cfg := GitConfig{
		InitLocal:     true,
		RemoteHost:    "github",
		RemotePrivate: true,
		RepoName:      "",
	}
	got := gitValue(cfg)
	if !strings.Contains(got, "github (private)") {
		t.Errorf("gitValue() = %q, want to contain 'github (private)'", got)
	}
	if strings.Contains(got, ": ") {
		t.Errorf("gitValue() = %q, should not contain ': ' when RepoName is empty", got)
	}
}

func TestGitValue_CustomRemoteURL(t *testing.T) {
	cfg := GitConfig{
		InitLocal:  true,
		RemoteHost: "custom",
		RemoteURL:  "git@mygit.internal:user/myrepo.git",
	}
	got := gitValue(cfg)
	if !strings.Contains(got, "myrepo.git") {
		t.Errorf("gitValue() = %q, want to contain basename of remote URL", got)
	}
}

func TestGitValue_WithCollab(t *testing.T) {
	cfg := GitConfig{InitLocal: true, Collab: true}
	got := gitValue(cfg)
	if !strings.Contains(got, "collab") {
		t.Errorf("gitValue() = %q, want to contain 'collab'", got)
	}
}

func TestGitValue_WithGithubCI(t *testing.T) {
	cfg := GitConfig{InitLocal: true, CI: ciChoiceGitHub}
	got := gitValue(cfg)
	if !strings.Contains(got, "github CI") {
		t.Errorf("gitValue() = %q, want to contain 'github CI'", got)
	}
}

func TestGitValue_WithGitlabCI(t *testing.T) {
	cfg := GitConfig{InitLocal: true, CI: ciChoiceGitLab}
	got := gitValue(cfg)
	if !strings.Contains(got, "gitlab CI") {
		t.Errorf("gitValue() = %q, want to contain 'gitlab CI'", got)
	}
}

func TestGitValue_CINone_NotShown(t *testing.T) {
	cfg := GitConfig{InitLocal: true, CI: ciChoiceNone}
	got := gitValue(cfg)
	if strings.Contains(got, "CI") {
		t.Errorf("gitValue() = %q, should not contain 'CI' when CI=none", got)
	}
}

func TestGitValue_ExistingGit_WithRemote(t *testing.T) {
	cfg := GitConfig{
		HasExistingGit: true,
		RemoteHost:     "github",
		RemotePrivate:  false,
		RepoName:       "repo",
	}
	got := gitValue(cfg)
	if !strings.Contains(got, "local") {
		t.Errorf("gitValue() = %q, want to contain 'local' when HasExistingGit=true", got)
	}
}

func TestGitValue_ExistingGit_NoRemote_IsNone(t *testing.T) {
	cfg := GitConfig{HasExistingGit: true, RemoteHost: ""}
	got := gitValue(cfg)
	if got != "none" {
		t.Errorf("gitValue() = %q, want none when only HasExistingGit and no remote", got)
	}
}

func TestGitValue_UniversalGitignore(t *testing.T) {
	cfg := GitConfig{InitLocal: true, UniversalGitignore: true}
	got := gitValue(cfg)
	if !strings.Contains(got, "universal gitignore") {
		t.Errorf("gitValue() = %q, want to contain 'universal gitignore'", got)
	}
}

func TestRecap_InvalidProjectNameBlocksConfirm(t *testing.T) {
	tests := []struct {
		name     string
		dir      string
		eco      string
		wantDone bool
	}{
		{name: "valid js name", dir: "/tmp/my-app", eco: "js", wantDone: true},
		{name: "invalid js name", dir: "/tmp/My App", eco: "js", wantDone: false},
		{name: "php accepts uppercase", dir: "/tmp/MyApp", eco: "php", wantDone: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewRecap(80, 24, RecapSummary{Dir: tt.dir, Framework: registry.Framework{ID: "x", Ecosystem: tt.eco}})
			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if updated.Done() != tt.wantDone {
				t.Errorf("Done() = %v, want %v", updated.Done(), tt.wantDone)
			}
		})
	}
}
