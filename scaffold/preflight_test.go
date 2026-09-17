package scaffold

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/slouowzee/kapi/internal/config"
	"github.com/slouowzee/kapi/internal/gitconfig"
	"github.com/slouowzee/kapi/internal/packagemanager"
	"github.com/slouowzee/kapi/internal/registry"
)

func fakePreflightEnv(installed []string, gitValues map[string]string, token string, scopes config.TokenScopes, scopesErr error) preflightEnv {
	return preflightEnv{
		lookPath: func(file string) (string, error) {
			for _, tool := range installed {
				if tool == file {
					return "/usr/bin/" + file, nil
				}
			}
			return "", errors.New("not found")
		},
		gitConfig: func(_, key string) string { return gitValues[key] },
		token:     func() string { return token },
		scopes:    func(context.Context) (config.TokenScopes, error) { return scopes, scopesErr },
	}
}

func TestPreflight(t *testing.T) {
	identity := map[string]string{"user.name": "Me", "user.email": "me@example.com"}
	allTools := []string{"php", "composer", "symfony", "node", "npm", "npx", "pnpm", "git"}

	tests := []struct {
		name         string
		dir          string
		fw           registry.Framework
		gitCfg       gitconfig.GitConfig
		pm           packagemanager.PM
		withPackages bool
		env          preflightEnv
		wantBlocking []string
		wantWarnings []string
	}{
		{
			name: "everything available",
			fw:   jsfw("nextjs"),
			pm:   packagemanager.PNPM,
			env:  fakePreflightEnv(allTools, identity, "tok", config.TokenScopes{Repo: true}, nil),
		},
		{
			name:         "missing composer and symfony cli",
			fw:           registry.Framework{ID: "symfony", Ecosystem: "php"},
			env:          fakePreflightEnv(nil, identity, "", config.TokenScopes{}, nil),
			wantBlocking: []string{"php is not installed", "composer is not installed", "symfony is not installed"},
		},
		{
			name:         "missing package manager",
			fw:           jsfw("nextjs"),
			pm:           packagemanager.Bun,
			withPackages: true,
			env:          fakePreflightEnv([]string{"git"}, identity, "", config.TokenScopes{}, nil),
			wantBlocking: []string{"bun is not installed", "bunx is not installed"},
		},
		{
			name:         "npm without node",
			fw:           jsfw("nextjs"),
			pm:           packagemanager.NPM,
			env:          fakePreflightEnv([]string{"npm", "npx"}, identity, "", config.TokenScopes{}, nil),
			wantBlocking: []string{"node is not installed"},
		},
		{
			name:         "invalid js project name",
			dir:          "My App",
			fw:           jsfw("nextjs"),
			pm:           packagemanager.NPM,
			env:          fakePreflightEnv(allTools, identity, "", config.TokenScopes{}, nil),
			wantBlocking: []string{"not a valid npm package name"},
		},
		{
			name:         "initial commit without git identity",
			fw:           fw("laravel"),
			gitCfg:       gitconfig.GitConfig{InitLocal: true, InitialCommit: true},
			env:          fakePreflightEnv(allTools, map[string]string{}, "", config.TokenScopes{}, nil),
			wantBlocking: []string{"git user.name is not set", "git user.email is not set"},
		},
		{
			name:   "no identity needed without initial commit",
			fw:     fw("laravel"),
			gitCfg: gitconfig.GitConfig{InitLocal: true},
			env:    fakePreflightEnv(allTools, map[string]string{}, "", config.TokenScopes{}, nil),
		},
		{
			name:         "github remote without token",
			fw:           fw("laravel"),
			gitCfg:       gitconfig.GitConfig{InitLocal: true, InitialCommit: true, RemoteHost: "github"},
			env:          fakePreflightEnv(allTools, identity, "", config.TokenScopes{}, nil),
			wantBlocking: []string{"GitHub token is required"},
		},
		{
			name:         "github token without repo scope",
			fw:           fw("laravel"),
			gitCfg:       gitconfig.GitConfig{InitLocal: true, RemoteHost: "github"},
			env:          fakePreflightEnv(allTools, identity, "tok", config.TokenScopes{WriteGPGKey: true}, nil),
			wantBlocking: []string{"missing the repo scope"},
		},
		{
			name:         "unverifiable scopes only warn",
			fw:           fw("laravel"),
			gitCfg:       gitconfig.GitConfig{InitLocal: true, RemoteHost: "github"},
			env:          fakePreflightEnv(allTools, identity, "tok", config.TokenScopes{}, errors.New("offline")),
			wantWarnings: []string{"could not verify the GitHub token scopes"},
		},
		{
			name:   "existing remote needs no token",
			fw:     fw("laravel"),
			gitCfg: gitconfig.GitConfig{HasExistingGit: true, HasExistingRemote: true, RemoteHost: "github"},
			env:    fakePreflightEnv(allTools, identity, "", config.TokenScopes{}, nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "app")
			if tt.dir != "" {
				dir = filepath.Join(t.TempDir(), tt.dir)
			}
			issues := preflight(context.Background(), tt.env, dir, tt.fw, tt.gitCfg, tt.pm, tt.withPackages)

			var blocking, warnings []string
			for _, issue := range issues {
				if issue.Blocking {
					blocking = append(blocking, issue.Message)
				} else {
					warnings = append(warnings, issue.Message)
				}
			}
			assertIssues(t, "blocking", blocking, tt.wantBlocking)
			assertIssues(t, "warning", warnings, tt.wantWarnings)
			if HasBlocking(issues) != (len(tt.wantBlocking) > 0) {
				t.Errorf("HasBlocking = %v, want %v", HasBlocking(issues), len(tt.wantBlocking) > 0)
			}
		})
	}
}

func TestPreflight_NonEmptyDirectoryWarns(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := fakePreflightEnv([]string{"php", "composer"}, nil, "", config.TokenScopes{}, nil)

	issues := preflight(context.Background(), env, dir, fw("laravel"), gitconfig.GitConfig{}, packagemanager.None, false)

	if len(issues) != 1 || issues[0].Blocking || !strings.Contains(issues[0].Message, "is not empty") {
		t.Errorf("issues = %+v, want a single non-empty directory warning", issues)
	}
}

func assertIssues(t *testing.T, kind string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s issues = %q, want %d matching %q", kind, got, len(want), want)
		return
	}
	for _, w := range want {
		found := false
		for _, g := range got {
			if strings.Contains(g, w) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s issues = %q, want one containing %q", kind, got, w)
		}
	}
}

func TestPreflight_ScopeCheckTimesOut(t *testing.T) {
	orig := scopeCheckTimeout
	scopeCheckTimeout = 50 * time.Millisecond
	t.Cleanup(func() { scopeCheckTimeout = orig })

	env := fakePreflightEnv([]string{"php", "composer", "git"}, map[string]string{"user.name": "Me", "user.email": "me@example.com"}, "tok", config.TokenScopes{}, nil)
	env.scopes = func(ctx context.Context) (config.TokenScopes, error) {
		<-ctx.Done() // simulates an unreachable GitHub API
		return config.TokenScopes{}, ctx.Err()
	}

	start := time.Now()
	issues := preflight(context.Background(), env, filepath.Join(t.TempDir(), "app"), fw("laravel"), gitconfig.GitConfig{InitLocal: true, InitialCommit: true, RemoteHost: "github"}, packagemanager.None, false)

	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("preflight took %v, want the scope check to give up quickly", elapsed)
	}
	if HasBlocking(issues) || len(issues) != 1 || !strings.Contains(issues[0].Message, "could not verify") {
		t.Errorf("issues = %+v, want a single non-blocking warning", issues)
	}
}

func TestInstallPreflight(t *testing.T) {
	tests := []struct {
		name      string
		fw        registry.Framework
		pm        packagemanager.PM
		installed []string
		want      []string
	}{
		{name: "composer project ready", fw: fw("laravel"), installed: []string{"php", "composer"}},
		{name: "composer missing", fw: fw("laravel"), installed: []string{"php"}, want: []string{"composer is not installed"}},
		{name: "pnpm missing", fw: jsfw("nextjs"), pm: packagemanager.PNPM, installed: []string{"node"}, want: []string{"pnpm is not installed"}},
		{name: "bun needs no node", fw: jsfw("nextjs"), pm: packagemanager.Bun, installed: []string{"bun"}},
		{name: "default npm needs node", fw: jsfw("nextjs"), installed: []string{"npm"}, want: []string{"node is not installed"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := fakePreflightEnv(tt.installed, nil, "", config.TokenScopes{}, nil)
			var got []string
			for _, issue := range installPreflight(env, tt.fw, tt.pm) {
				got = append(got, issue.Message)
			}
			assertIssues(t, "blocking", got, tt.want)
		})
	}
}
