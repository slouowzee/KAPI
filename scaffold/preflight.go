package scaffold

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/slouowzee/kapi/internal/config"
	"github.com/slouowzee/kapi/internal/gitconfig"
	"github.com/slouowzee/kapi/internal/packagemanager"
	"github.com/slouowzee/kapi/internal/registry"
)

// Issue is a problem detected before scaffolding. Blocking issues would make
// a step fail; the others are warnings.
type Issue struct {
	Blocking bool
	Message  string
}

type preflightEnv struct {
	lookPath  func(file string) (string, error)
	gitConfig func(dir, key string) string
	token     func() string
	scopes    func(ctx context.Context) (config.TokenScopes, error)
}

var defaultPreflightEnv = preflightEnv{
	lookPath:  exec.LookPath,
	gitConfig: gitConfigValue,
	token:     config.GithubToken,
	scopes:    config.FetchTokenScopes,
}

func gitConfigValue(dir, key string) string {
	c := exec.Command("git", "config", "--get", key)
	c.Dir = dir
	out, err := c.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Preflight checks what the planned steps need so problems are reported on
// the summary instead of failing halfway through scaffolding.
func Preflight(ctx context.Context, targetDir string, fw registry.Framework, gitCfg gitconfig.GitConfig, pm packagemanager.PM, withPackages bool) []Issue {
	return preflight(ctx, defaultPreflightEnv, targetDir, fw, gitCfg, pm, withPackages)
}

func preflight(ctx context.Context, env preflightEnv, targetDir string, fw registry.Framework, gitCfg gitconfig.GitConfig, pm packagemanager.PM, withPackages bool) []Issue {
	var issues []Issue
	blocking := func(format string, args ...any) {
		issues = append(issues, Issue{Blocking: true, Message: fmt.Sprintf(format, args...)})
	}
	warning := func(format string, args ...any) {
		issues = append(issues, Issue{Message: fmt.Sprintf(format, args...)})
	}

	if msg := ProjectNameError(fw, targetDir); msg != "" {
		blocking("%s", msg)
	}

	for _, tool := range requiredTools(fw, gitCfg, pm, withPackages) {
		if _, err := env.lookPath(tool); err != nil {
			blocking("%s is not installed or not in PATH", tool)
		}
	}

	if info, err := os.Stat(targetDir); err == nil {
		if !info.IsDir() {
			blocking("%s exists and is not a directory", targetDir)
		} else if entries, err := os.ReadDir(targetDir); err == nil && len(entries) > 0 {
			warning("%s is not empty: most initializers refuse to write into it", targetDir)
		}
	}

	newRepo := gitCfg.InitLocal && !gitCfg.HasExistingGit
	if newRepo && gitCfg.InitialCommit {
		dir := existingDir(targetDir)
		for _, key := range []string{"user.name", "user.email"} {
			if env.gitConfig(dir, key) == "" {
				blocking("git %s is not set: run git config --global %s <value>", key, key)
			}
		}
	}

	hasRepo := newRepo || gitCfg.HasExistingGit
	if hasRepo && gitCfg.RemoteHost == "github" && !gitCfg.HasExistingRemote {
		if env.token() == "" {
			blocking("a GitHub token is required to create the repository: kapi config github.token <token>")
		} else if scopes, err := env.scopes(ctx); err != nil {
			warning("could not verify the GitHub token scopes: %v", err)
		} else if !scopes.Repo {
			blocking("the GitHub token is missing the repo scope needed to create repositories")
		}
	}

	return issues
}

// requiredTools lists the executables the planned steps run.
func requiredTools(fw registry.Framework, gitCfg gitconfig.GitConfig, pm packagemanager.PM, withPackages bool) []string {
	tools := map[string]struct{}{}
	switch fw.Ecosystem {
	case "php":
		tools["composer"] = struct{}{}
		if fw.ID == "symfony" {
			tools["symfony"] = struct{}{}
		}
	case "js":
		if pm == packagemanager.None {
			pm = packagemanager.NPM
		}
		tools[pm.ExecArgs()[0]] = struct{}{}
		tools[pm.CreateArgs()[0]] = struct{}{}
		if withPackages {
			tools[pm.InstallArgs()[0]] = struct{}{}
		}
	}
	if gitCfg.InitLocal || gitCfg.HasExistingGit {
		tools["git"] = struct{}{}
	}

	list := make([]string, 0, len(tools))
	for tool := range tools {
		list = append(list, tool)
	}
	sort.Strings(list)
	return list
}

// existingDir returns dir or its closest existing parent, where git commands
// can run before the project directory is created.
func existingDir(dir string) string {
	for {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		dir = parent
	}
}

// HasBlocking reports whether any issue prevents scaffolding.
func HasBlocking(issues []Issue) bool {
	for _, issue := range issues {
		if issue.Blocking {
			return true
		}
	}
	return false
}
