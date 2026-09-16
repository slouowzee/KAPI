package screens

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slouowzee/kapi/internal/packagemanager"
	"github.com/slouowzee/kapi/internal/registry"
	"github.com/slouowzee/kapi/scaffold"
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

func TestGitValue_WithoutNewRepo(t *testing.T) {
	tests := []struct {
		name string
		cfg  GitConfig
		want string
	}{
		{name: "existing repo is reported", cfg: GitConfig{HasExistingGit: true}, want: "local"},
		{name: "collab and ci without repo", cfg: GitConfig{Collab: true, CI: ciChoiceGitHub}, want: "collab  ·  github CI"},
		{name: "commit option ignored without init", cfg: GitConfig{InitialCommit: true, UniversalGitignore: true, CI: ciChoiceNone}, want: "none"},
		{name: "commit option ignored on existing repo", cfg: GitConfig{HasExistingGit: true, InitialCommit: true}, want: "local"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gitValue(tt.cfg); got != tt.want {
				t.Errorf("gitValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGitValue_UniversalGitignore(t *testing.T) {
	cfg := GitConfig{InitLocal: true, UniversalGitignore: true}
	got := gitValue(cfg)
	if !strings.Contains(got, "universal gitignore") {
		t.Errorf("gitValue() = %q, want to contain 'universal gitignore'", got)
	}
}

func TestRecap_PreflightGatesConfirm(t *testing.T) {
	tests := []struct {
		name     string
		issues   []scaffold.Issue
		checked  bool
		wantDone bool
	}{
		{name: "still checking", checked: false, wantDone: false},
		{name: "no issues", checked: true, wantDone: true},
		{name: "warnings only", checked: true, issues: []scaffold.Issue{{Message: "dir not empty"}}, wantDone: true},
		{name: "blocking issue", checked: true, issues: []scaffold.Issue{{Blocking: true, Message: "composer missing"}}, wantDone: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewRecap(80, 24, RecapSummary{Dir: "/tmp/my-app", Framework: registry.Framework{ID: "laravel", Ecosystem: "php"}})
			if tt.checked {
				m, _ = m.Update(preflightDoneMsg{issues: tt.issues})
			}
			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if updated.Done() != tt.wantDone {
				t.Errorf("Done() = %v, want %v", updated.Done(), tt.wantDone)
			}
			for _, issue := range tt.issues {
				if !strings.Contains(updated.View(), issue.Message) {
					t.Errorf("issue %q is not displayed", issue.Message)
				}
			}
		})
	}
}

func TestRecap_InitRunsPreflight(t *testing.T) {
	orig := runPreflight
	t.Cleanup(func() { runPreflight = orig })
	var gotDir string
	runPreflight = func(_ context.Context, dir string, _ registry.Framework, _ GitConfig, _ packagemanager.PM, _ bool) []scaffold.Issue {
		gotDir = dir
		return []scaffold.Issue{{Blocking: true, Message: "boom"}}
	}

	m := NewRecap(80, 24, RecapSummary{Dir: "/tmp/my-app"})
	m, _ = m.Update(m.Init()())

	if gotDir != "/tmp/my-app" || m.CanScaffold() {
		t.Errorf("preflight not applied: dir=%q canScaffold=%v", gotDir, m.CanScaffold())
	}
}

func TestGitValue_GithubHTTPS(t *testing.T) {
	got := gitValue(GitConfig{InitLocal: true, RemoteHost: "github", RemotePrivate: true, RemoteHTTPS: true, RepoName: "repo"})
	if !strings.Contains(got, "github (private, https): repo") {
		t.Errorf("gitValue() = %q, want the https protocol shown", got)
	}
}

func TestRecap_CursorReachesAbandon(t *testing.T) {
	for _, eco := range []string{"php", "js"} {
		t.Run(eco, func(t *testing.T) {
			m := NewRecap(80, 24, RecapSummary{Framework: registry.Framework{ID: "x", Ecosystem: eco}})
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
			if RecapSection(m.cursor) != RECAP_SECTION_ABANDON {
				t.Errorf("cursor = %d, want RECAP_SECTION_ABANDON", m.cursor)
			}
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if !m.IsAbandonPending() {
				t.Error("enter on abandon should ask for confirmation")
			}
		})
	}
}

func TestRecap_PHPSkipsPackageManagerRow(t *testing.T) {
	m := NewRecap(80, 24, RecapSummary{Framework: registry.Framework{ID: "laravel", Ecosystem: "php"}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if RecapSection(m.cursor) != RECAP_SECTION_GIT {
		t.Errorf("cursor = %d, want RECAP_SECTION_GIT (package manager row skipped)", m.cursor)
	}
}
