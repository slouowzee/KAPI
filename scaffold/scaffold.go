package scaffold

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/slouowzee/kapi/internal/config"
	"github.com/slouowzee/kapi/internal/gitconfig"
	"github.com/slouowzee/kapi/internal/github"
	"github.com/slouowzee/kapi/internal/packagemanager"
	"github.com/slouowzee/kapi/internal/packages"
	"github.com/slouowzee/kapi/internal/registry"
)

// Step is one scaffolding action. StreamFn must stop its command when ctx is
// cancelled, which happens when the user aborts.
type Step struct {
	Label    string
	Cmd      *exec.Cmd
	Fn       func() error
	StreamFn func(ctx context.Context, onLine func(string)) error
}

func Plan(
	targetDir string,
	fw registry.Framework,
	selectedPkgs []packages.Package,
	gitCfg gitconfig.GitConfig,
	pm packagemanager.PM,
) []Step {
	var steps []Step

	if fw.Ecosystem == "js" && pm == packagemanager.None {
		pm = packagemanager.NPM
	}

	steps = append(steps, frameworkSteps(targetDir, fw, pm)...)

	if len(selectedPkgs) > 0 {
		steps = append(steps, packageSteps(targetDir, fw, selectedPkgs, pm)...)
	}

	newRepo := gitCfg.InitLocal && !gitCfg.HasExistingGit
	hasRepo := newRepo || gitCfg.HasExistingGit
	hasCommit := gitCfg.HasExistingGit || (newRepo && gitCfg.InitialCommit)

	if newRepo {
		steps = append(steps, Step{
			Label:    "git init",
			StreamFn: streamCmd(targetDir, "git", "init"),
		})
		if gitCfg.UniversalGitignore {
			steps = append(steps, Step{
				Label: "merge universal .gitignore",
				Fn:    mergeGitignoreFn(targetDir, universalGitignore),
			})
		}
	}

	// NOTE: generated files are written before the initial commit so that
	// they end up in the pushed history.
	if gitCfg.Collab {
		steps = append(steps, collabFileSteps(targetDir)...)
	}

	switch gitCfg.CI {
	case "github":
		steps = append(steps, ciGithubStep(targetDir, fw, pm))
	case "gitlab":
		steps = append(steps, ciGitlabStep(targetDir, fw, pm))
	}

	if newRepo && gitCfg.InitialCommit {
		steps = append(steps, initialCommitStep(targetDir))
	}

	if !hasRepo {
		return steps
	}

	remote := remoteSteps(targetDir, gitCfg)
	steps = append(steps, remote...)

	// NOTE: dev is created after the default branch has been pushed, otherwise
	// only dev would reach the remote and become its default branch.
	if gitCfg.Collab && hasCommit {
		steps = append(steps, devBranchStep(targetDir))
		if len(remote) > 0 {
			steps = append(steps, Step{
				Label:    "git push -u origin dev",
				StreamFn: streamGitCmd(targetDir, "push", "-u", "origin", "dev"),
			})
		}
	}

	return steps
}

func remoteSteps(targetDir string, gitCfg gitconfig.GitConfig) []Step {
	if gitCfg.HasExistingRemote {
		return nil
	}
	// NOTE: a freshly initialised repository without a commit has nothing to
	// push, so the remote is only registered.
	hasCommit := gitCfg.InitialCommit || gitCfg.HasExistingGit
	switch gitCfg.RemoteHost {
	case "github":
		name := gitCfg.RepoName
		if name == "" {
			name = filepath.Base(targetDir)
		}
		private := gitCfg.RemotePrivate
		remoteURL := new(string)

		visibility := "public"
		if private {
			visibility = "private"
		}

		steps := []Step{
			{
				Label: "create " + visibility + " GitHub repo: " + name,
				Fn: func() error {
					ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
					defer cancel()
					repo, err := github.CreateRepo(ctx, config.GithubToken(), name, private)
					if err != nil {
						return err
					}
					*remoteURL = repo.SSHURL
					if gitCfg.RemoteHTTPS {
						*remoteURL = repo.CloneURL
					}
					return nil
				},
			},
			{
				Label: "git remote add origin <github url>",
				Fn: func() error {
					return gitSilentCmd(targetDir, "remote", "add", "origin", *remoteURL).Run()
				},
			},
		}
		if hasCommit {
			steps = append(steps, pushStep(targetDir))
		}
		return steps

	default:
		if gitCfg.RemoteURL == "" {
			return nil
		}
		steps := []Step{
			{
				Label:    "git remote add origin " + gitCfg.RemoteURL,
				StreamFn: streamGitCmd(targetDir, "remote", "add", "origin", gitCfg.RemoteURL),
			},
		}
		if hasCommit {
			steps = append(steps, pushStep(targetDir))
		}
		return steps
	}
}

func pushStep(targetDir string) Step {
	return Step{
		Label:    "git push -u origin HEAD",
		StreamFn: streamGitCmd(targetDir, "push", "-u", "origin", "HEAD"),
	}
}

func frameworkSteps(targetDir string, fw registry.Framework, pm packagemanager.PM) []Step {
	name := filepath.Base(targetDir)
	parent := filepath.Dir(targetDir)

	switch fw.ID {
	case "laravel":
		return []Step{{
			Label:    "composer create-project laravel/laravel " + name,
			StreamFn: streamCmd(parent, "composer", "create-project", "laravel/laravel", name),
		}}
	case "symfony":
		return []Step{{
			Label:    "symfony new " + name + " --webapp",
			StreamFn: streamCmd(parent, "symfony", "new", name, "--webapp"),
		}}
	case "slim":
		return []Step{{
			Label:    "composer create-project slim/slim-skeleton " + name,
			StreamFn: streamCmd(parent, "composer", "create-project", "slim/slim-skeleton", name),
		}}
	case "lumen":
		return []Step{{
			Label:    "composer create-project laravel/lumen " + name,
			StreamFn: streamCmd(parent, "composer", "create-project", "laravel/lumen", name),
		}}
	case "codeigniter":
		return []Step{{
			Label:    "composer create-project codeigniter4/appstarter " + name,
			StreamFn: streamCmd(parent, "composer", "create-project", "codeigniter4/appstarter", name),
		}}
	case "yii":
		return []Step{{
			Label:    "composer create-project --prefer-dist yiisoft/yii2-app-basic " + name,
			StreamFn: streamCmd(parent, "composer", "create-project", "--prefer-dist", "yiisoft/yii2-app-basic", name),
		}}
	case "wordpress":
		return wordpressSteps(name, parent)
	case "drupal":
		return []Step{{
			Label:    "composer create-project drupal/recommended-project " + name,
			StreamFn: streamCmd(parent, "composer", "create-project", "drupal/recommended-project", name),
		}}
	case "cakephp":
		return []Step{{
			Label:    "composer create-project cakephp/app " + name,
			StreamFn: streamCmd(parent, "composer", "create-project", "cakephp/app", name),
		}}
	case "laminas":
		return []Step{{
			Label:    "composer create-project laminas/laminas-mvc-skeleton " + name,
			StreamFn: streamCmd(parent, "composer", "create-project", "laminas/laminas-mvc-skeleton", name),
		}}
	case "phalcon":
		// NOTE: phalcon/phalcon is the framework library, not installable as a
		// project skeleton; Phalcon has no blank starter, so invo (their own
		// getting-started tutorial app) is the closest official entry point.
		return []Step{{
			Label:    "composer create-project phalcon/invo " + name,
			StreamFn: streamCmd(parent, "composer", "create-project", "phalcon/invo", name),
		}}
	case "fuelphp":
		return []Step{{
			Label:    "composer create-project fuel/fuel " + name,
			StreamFn: streamCmd(parent, "composer", "create-project", "fuel/fuel", name),
		}}
	case "leafphp":
		// NOTE: leafs/leaf is the framework library; leafs/mvc is the actual
		// project skeleton with the MVC structure and routing set up.
		return []Step{{
			Label:    "composer create-project leafs/mvc " + name,
			StreamFn: streamCmd(parent, "composer", "create-project", "leafs/mvc", name),
		}}
	case "api-platform":
		return apiPlatformSteps(targetDir, name, parent)
	case "vanilla-php":
		return vanillaPhpSteps(targetDir, name)

	case "nextjs":
		// NOTE: the flag keeps the chosen package manager when the initializer
		// is run through npx (e.g. Yarn 1 fallback).
		return jsExecStep(parent, pm, "create-next-app@latest", name, "--use-"+pm.CacheKey())
	case "nuxt":
		return jsExecStep(parent, pm, "nuxi@latest", "init", name)
	case "remix":
		return jsExecStep(parent, pm, "create-remix@latest", name)
	case "tanstack-start":
		return jsExecStep(parent, pm, "create-tsrouter-app@latest", name, "--framework", "react", "--add-ons", "start")
	case "astro":
		return jsCreateStep(parent, pm, "astro@latest", name)
	case "gatsby":
		return jsExecStep(parent, pm, "gatsby", "new", name)
	case "sveltekit":
		return jsExecStep(parent, pm, "sv", "create", name)
	case "analog":
		return jsExecStep(parent, pm, "create-nx-workspace@latest", name, "--preset=@analogjs/platform")
	case "hono":
		return jsCreateStep(parent, pm, "hono@latest", name)
	case "react-native":
		return jsExecStep(parent, pm, "@react-native-community/cli@latest", "init", name)

	case "react-vite":
		return jsCreateStreamStep(parent, pm, "vite@latest", viteArgs(pm, name, "react-ts")...)
	case "vue-vite":
		return jsCreateStreamStep(parent, pm, "vite@latest", viteArgs(pm, name, "vue-ts")...)
	case "svelte-vite":
		return jsCreateStreamStep(parent, pm, "vite@latest", viteArgs(pm, name, "svelte-ts")...)
	case "vanilla-vite":
		return jsCreateStreamStep(parent, pm, "vite@latest", viteArgs(pm, name, "vanilla-ts")...)
	case "express":
		return jsExecStreamStep(parent, pm, "express-generator", name)
	case "fastify":
		return jsCreateStreamStep(parent, pm, "fastify@latest", name)
	case "nestjs":
		return jsExecStreamStep(parent, pm, "@nestjs/cli@latest", "new", name, "--package-manager", pm.String())
	case "expo":
		return jsExecStreamStep(parent, pm, "create-expo-app@latest", name)
	default:
	}

	return []Step{mkdirStep(targetDir)}
}

// mkdirStep creates a directory natively: `mkdir -p` does not exist on Windows.
func mkdirStep(dir string) Step {
	return Step{
		Label: "mkdir " + dir,
		Fn:    func() error { return os.MkdirAll(dir, 0o755) },
	}
}

// viteArgs builds create-vite arguments. Only npm needs the extra `--` to
// forward flags; other package managers would pass it through and make
// create-vite ignore the template.
func viteArgs(pm packagemanager.PM, name, template string) []string {
	args := []string{name}
	if pm == packagemanager.NPM || pm == packagemanager.None {
		args = append(args, "--")
	}
	// NOTE: streamed steps have no stdin, so prompts must be disabled.
	return append(args, "--template", template, "--no-interactive")
}

func jsExecStep(dir string, pm packagemanager.PM, pkg string, extra ...string) []Step {
	argv := append(append([]string(nil), pm.ExecArgs()...), pkg)
	argv = append(argv, extra...)
	return []Step{{Label: strings.Join(argv, " "), Cmd: cmdSlice(dir, argv)}}
}

// jsCreateStep runs an interactive `<pm> create <pkg>` initializer.
func jsCreateStep(dir string, pm packagemanager.PM, pkg string, extra ...string) []Step {
	argv := append(append([]string(nil), pm.CreateArgs()...), pkg)
	argv = append(argv, extra...)
	return []Step{{Label: strings.Join(argv, " "), Cmd: cmdSlice(dir, argv)}}
}

func jsExecStreamStep(dir string, pm packagemanager.PM, pkg string, extra ...string) []Step {
	argv := append(append([]string(nil), pm.ExecArgs()...), pkg)
	argv = append(argv, extra...)
	return []Step{{Label: strings.Join(argv, " "), StreamFn: streamCmdSlice(dir, argv)}}
}

func jsCreateStreamStep(dir string, pm packagemanager.PM, pkg string, extra ...string) []Step {
	argv := append(append([]string(nil), pm.CreateArgs()...), pkg)
	argv = append(argv, extra...)
	return []Step{{Label: strings.Join(argv, " "), StreamFn: streamCmdSlice(dir, argv)}}
}

func wordpressSteps(name, parent string) []Step {
	return []Step{{
		Label:    "composer create-project roots/bedrock " + name,
		StreamFn: streamCmd(parent, "composer", "create-project", "roots/bedrock", name),
	}}
}

func vanillaPhpComposerName(folder string) string {
	part := composerPackageName(folder)
	return part + "/" + part
}

// apiPlatformSteps installs API Platform as a Symfony pack: api-platform/api-platform
// (the old standalone distribution) is abandoned in favor of requiring the pack
// into a fresh Symfony skeleton.
func apiPlatformSteps(targetDir, name, parent string) []Step {
	return []Step{
		{
			Label:    "composer create-project symfony/skeleton " + name,
			StreamFn: streamCmd(parent, "composer", "create-project", "symfony/skeleton", name),
		},
		{
			Label:    "composer require api-platform/api-pack",
			StreamFn: streamCmd(targetDir, "composer", "require", "api-platform/api-pack"),
		},
	}
}

func vanillaPhpSteps(targetDir, name string) []Step {
	return []Step{
		mkdirStep(targetDir),
		{
			Label:    "composer init (in " + name + ")",
			StreamFn: streamCmd(targetDir, "composer", "init", "--no-interaction", "--name="+vanillaPhpComposerName(name)),
		},
	}
}

// InstallPlan returns the steps adding packages to an existing project.
func InstallPlan(dir string, fw registry.Framework, pkgs []packages.Package, pm packagemanager.PM) []Step {
	if fw.Ecosystem == "js" && pm == packagemanager.None {
		pm = packagemanager.NPM
	}
	return packageSteps(dir, fw, pkgs, pm)
}

func packageSteps(targetDir string, fw registry.Framework, pkgs []packages.Package, pm packagemanager.PM) []Step {
	if len(pkgs) == 0 {
		return nil
	}

	names := make([]string, len(pkgs))
	var frozenInfo []string
	for i, p := range pkgs {
		if p.PinnedVersion != "" {
			if fw.Ecosystem == "php" {
				names[i] = p.Name + ":" + p.PinnedVersion
			} else {
				names[i] = p.Name + "@" + p.PinnedVersion
			}
			frozenInfo = append(frozenInfo, fmt.Sprintf("  • %s frozen at %s", p.Name, p.PinnedVersion))
		} else {
			names[i] = p.Name
		}
	}

	var steps []Step

	if fw.Ecosystem == "php" {
		args := append([]string{"require"}, names...)
		steps = append(steps, Step{
			Label:    "composer require " + strings.Join(names, " "),
			StreamFn: streamCmdSlice(targetDir, append([]string{"composer"}, args...)),
		})
	} else {
		argv := append(append([]string(nil), pm.InstallArgs()...), names...)
		steps = append(steps, Step{
			Label:    strings.Join(argv, " "),
			StreamFn: streamCmdSlice(targetDir, argv),
		})
	}

	if len(frozenInfo) > 0 {
		for _, info := range frozenInfo {
			steps = append(steps, Step{
				Label: info,
				Fn:    func() error { return nil },
			})
		}
	}

	return steps
}

func devBranchStep(targetDir string) Step {
	return Step{
		Label: "create dev branch",
		Fn: func() error {
			// NOTE: an existing repository may already have a dev branch; it is
			// left untouched rather than reset.
			if gitSilentCmd(targetDir, "rev-parse", "--verify", "--quiet", "refs/heads/dev").Run() == nil {
				return nil
			}
			return gitSilentCmd(targetDir, "checkout", "-b", "dev").Run()
		},
	}
}

func collabFileSteps(targetDir string) []Step {
	return []Step{
		{
			Label: "write CONTRIBUTING.md",
			Fn:    writeFileFn(targetDir, "CONTRIBUTING.md", contributingMd),
		},
		{
			Label: "write .github/PULL_REQUEST_TEMPLATE.md",
			Fn:    writeFileFn(targetDir, filepath.Join(".github", "PULL_REQUEST_TEMPLATE.md"), prTemplate),
		},
		{
			Label: "write .github/ISSUE_TEMPLATE/bug_report.md",
			Fn:    writeFileFn(targetDir, filepath.Join(".github", "ISSUE_TEMPLATE", "bug_report.md"), bugReportTemplate),
		},
		{
			Label: "write .github/ISSUE_TEMPLATE/feature_request.md",
			Fn:    writeFileFn(targetDir, filepath.Join(".github", "ISSUE_TEMPLATE", "feature_request.md"), featureTemplate),
		},
	}
}

func ciGithubStep(targetDir string, fw registry.Framework, pm packagemanager.PM) Step {
	path := filepath.Join(".github", "workflows", "ci.yml")
	return Step{
		Label: "write " + path,
		Fn:    writeFileFn(targetDir, path, githubActionsCI(fw, pm)),
	}
}

func ciGitlabStep(targetDir string, fw registry.Framework, pm packagemanager.PM) Step {
	return Step{
		Label: "write .gitlab-ci.yml",
		Fn:    writeFileFn(targetDir, ".gitlab-ci.yml", gitlabCI(fw, pm)),
	}
}

func cmd(dir string, name string, args ...string) *exec.Cmd {
	c := exec.Command(name, args...)
	if dir != "" {
		c.Dir = dir
	}
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c
}

func cmdSlice(dir string, argv []string) *exec.Cmd {
	if len(argv) == 0 {
		return cmd(dir, "true")
	}
	return cmd(dir, argv[0], argv[1:]...)
}

func gitSilentCmd(targetDir string, args ...string) *exec.Cmd {
	c := exec.Command("git", args...)
	if targetDir != "" {
		c.Dir = targetDir
	}
	return c
}

type streamFunc = func(ctx context.Context, onLine func(string)) error

// maxOutputLine bounds a single streamed output line (progress bars may emit
// very long lines without newlines).
const maxOutputLine = 1024 * 1024

// abortGracePeriod is how long a command gets to exit after being signalled,
// and how long its output is still read once it has exited, before its pipes
// are forcibly closed. It is a variable so tests can shorten it.
var abortGracePeriod = 5 * time.Second

func streamCmd(dir string, name string, args ...string) streamFunc {
	return func(ctx context.Context, onLine func(string)) error {
		c := exec.CommandContext(ctx, name, args...)
		if dir != "" {
			c.Dir = dir
		}
		configureCancel(c)
		c.WaitDelay = abortGracePeriod
		return runStreamed(c, onLine)
	}
}

func streamCmdSlice(dir string, argv []string) streamFunc {
	if len(argv) == 0 {
		return func(context.Context, func(string)) error { return nil }
	}
	return streamCmd(dir, argv[0], argv[1:]...)
}

func streamGitCmd(targetDir string, args ...string) streamFunc {
	return streamCmd(targetDir, "git", args...)
}

func runStreamed(c *exec.Cmd, onLine func(string)) error {
	w := &lineWriter{onLine: onLine, maxLine: maxOutputLine}
	c.Stdout = w
	c.Stderr = w
	if c.WaitDelay == 0 {
		c.WaitDelay = abortGracePeriod
	}
	if err := c.Start(); err != nil {
		return err
	}
	untrack := trackRunning(c)
	defer untrack()

	err := c.Wait()
	w.flush()
	// NOTE: ErrWaitDelay means the command succeeded but a background process
	// it started kept the output open; the step itself is done.
	if errors.Is(err, exec.ErrWaitDelay) {
		return nil
	}
	return err
}

func initialCommitStep(targetDir string) Step {
	return Step{
		Label: `git add -A && git commit -m "chore: initial commit"`,
		Fn: func() error {
			addCmd := exec.Command("git", "add", "-A")
			addCmd.Dir = targetDir
			if err := addCmd.Run(); err != nil {
				return err
			}
			statusCmd := exec.Command("git", "status", "--porcelain")
			statusCmd.Dir = targetDir
			out, err := statusCmd.Output()
			if err != nil {
				return err
			}
			if strings.TrimSpace(string(out)) == "" {
				return nil
			}
			commitCmd := exec.Command("git", "commit", "-m", "chore: initial commit")
			commitCmd.Dir = targetDir
			return commitCmd.Run()
		},
	}
}

func writeFileFn(targetDir, relPath, content string) func() error {
	return func() error {
		fullPath := filepath.Join(targetDir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(fullPath, []byte(content), 0o644)
	}
}

// mergeGitignoreFn appends the universal rules that are missing from the
// framework's own .gitignore instead of replacing it, so framework-specific
// rules (storage keys, secrets, build dirs…) are preserved.
func mergeGitignoreFn(targetDir, universal string) func() error {
	return func() error {
		path := filepath.Join(targetDir, ".gitignore")
		existing, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			return os.WriteFile(path, []byte(universal), 0o644)
		}
		if err != nil {
			return fmt.Errorf("read .gitignore: %w", err)
		}

		present := make(map[string]struct{})
		for _, line := range strings.Split(string(existing), "\n") {
			present[strings.TrimSpace(line)] = struct{}{}
		}

		var missing []string
		for _, line := range strings.Split(universal, "\n") {
			rule := strings.TrimSpace(line)
			if rule == "" || strings.HasPrefix(rule, "#") {
				continue
			}
			if _, ok := present[rule]; ok {
				continue
			}
			present[rule] = struct{}{}
			missing = append(missing, rule)
		}
		if len(missing) == 0 {
			return nil
		}

		var sb strings.Builder
		sb.Write(existing)
		if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("\n# Added by kapi\n")
		sb.WriteString(strings.Join(missing, "\n"))
		sb.WriteString("\n")
		return os.WriteFile(path, []byte(sb.String()), 0o644)
	}
}

const contributingMd = "# Contributing\n\n" +
	"Thank you for considering contributing to this project!\n\n" +
	"## Workflow\n\n" +
	"1. Fork the repository and create a branch from `dev`.\n" +
	"2. Make your changes with clear, focused commits.\n" +
	"3. Open a pull request targeting the `dev` branch.\n" +
	"4. Wait for review — we'll get back to you as soon as possible.\n\n" +
	"## Code Style\n\n" +
	"- Follow the existing style of the codebase.\n" +
	"- Write tests for any new behaviour.\n" +
	"- Keep PRs small and focused.\n"

const prTemplate = "## Summary\n\n" +
	"<!-- What does this PR do? -->\n\n" +
	"## Changes\n\n" +
	"- \n\n" +
	"## Testing\n\n" +
	"<!-- How was this tested? -->\n\n" +
	"## Checklist\n\n" +
	"- [ ] Tests added or updated\n" +
	"- [ ] Documentation updated if needed\n" +
	"- [ ] No unrelated changes included\n"

const bugReportTemplate = "---\n" +
	"name: Bug report\n" +
	"about: Report a reproducible bug\n" +
	"labels: bug\n" +
	"---\n\n" +
	"## Describe the bug\n\n" +
	"<!-- A clear and concise description of the bug. -->\n\n" +
	"## Steps to reproduce\n\n" +
	"1. \n" +
	"2. \n\n" +
	"## Expected behaviour\n\n" +
	"## Actual behaviour\n\n" +
	"## Environment\n\n" +
	"- OS:\n" +
	"- Version:\n"

const featureTemplate = "---\n" +
	"name: Feature request\n" +
	"about: Suggest a new feature or improvement\n" +
	"labels: enhancement\n" +
	"---\n\n" +
	"## Problem to solve\n\n" +
	"<!-- What problem does this feature address? -->\n\n" +
	"## Proposed solution\n\n" +
	"## Alternatives considered\n"

const universalGitignore = `# Environment variables
.env
.env.*
!.env.example
!.env.test

# Dependencies
node_modules/
vendor/

# Build outputs
/dist/
/build/
/.next/
/.nuxt/
/.output/
/.svelte-kit/
/.angular/
/out/
public/build/
var/cache/
var/log/

# Caches
.cache/
.eslintcache
.tsbuildinfo
.phpunit.result.cache

# IDE / OS
.idea/
.vscode/
*.swp
.DS_Store

# Logs
logs/
*.log
npm-debug.log*
yarn-debug.log*

# Coverage
coverage/
`

func githubActionsCI(fw registry.Framework, pm packagemanager.PM) string {
	if fw.Ecosystem == "php" {
		return githubActionsCIPhp(fw.ID)
	}

	var setupSteps string
	if pm.String() == "bun" {
		setupSteps = `      - uses: actions/setup-node@v4
        with:
          node-version: '24'
      - uses: oven-sh/setup-bun@v2
        with:
          bun-version: latest`
	} else if pm.String() == "pnpm" {
		setupSteps = `      - uses: pnpm/action-setup@v3
        with:
          version: latest
      - uses: actions/setup-node@v4
        with:
          node-version: '24'
          cache: 'pnpm'`
	} else {
		setupSteps = fmt.Sprintf(`      - uses: actions/setup-node@v4
        with:
          node-version: '24'
          cache: '%s'`, pm.CacheKey())
	}

	return fmt.Sprintf(`name: CI

on:
  push:
    branches: [main, dev]
  pull_request:

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
%s
      - run: %s
      - run: %s
      - run: %s
`, setupSteps, pm.CIInstall(), pm.RunIfPresent("test"), pm.RunIfPresent("build"))
}

func githubActionsCIPhp(fwID string) string {
	const header = `name: CI

on:
  push:
    branches: [main, dev]
  pull_request:

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: shivammathur/setup-php@v2
        with:
          php-version: '8.5'
      - run: composer install --prefer-dist --no-progress
`
	switch fwID {
	case "laravel", "lumen":
		return header +
			"      - run: cp .env.example .env\n" +
			"      - run: php artisan key:generate\n" +
			"      - run: composer test\n"
	case "symfony", "api-platform":
		return header +
			"      - run: APP_ENV=test php bin/phpunit\n"
	case "codeigniter":
		return header +
			"      - run: cp env .env\n" +
			"      - run: composer test\n"
	case "wordpress", "vanilla-php":
		return header
	default:
		return header +
			"      - run: " + composerTestIfPresent + "\n"
	}
}

// composerTestIfPresent runs `composer test` only when the skeleton defines
// that script; several skeletons (Drupal, Yii…) ship without one.
const composerTestIfPresent = `if composer run-script --list | grep -qE '^[[:space:]]+test([[:space:]]|$)'; then composer test; else echo "No composer test script, skipping"; fi`

func gitlabCI(fw registry.Framework, pm packagemanager.PM) string {
	if fw.Ecosystem == "php" {
		return gitlabCIPhp(fw.ID)
	}

	var beforeScript string
	if pm.String() == "bun" {
		beforeScript = `    - npm install -g bun
    - ` + pm.CIInstall()
	} else if pm.String() == "pnpm" {
		beforeScript = `    - npm install -g pnpm
    - ` + pm.CIInstall()
	} else {
		beforeScript = `    - ` + pm.CIInstall()
	}

	return fmt.Sprintf(`image: node:24

stages:
  - test

test:
  stage: test
  cache:
    paths:
      - node_modules/
  before_script:
%s
  script:
    - %s
    - %s
`, beforeScript, pm.RunIfPresent("test"), pm.RunIfPresent("build"))
}

func gitlabCIPhp(fwID string) string {
	const header = `image: php:8.5

stages:
  - test

test:
  stage: test
  before_script:
    - apt-get update -qq && apt-get install -y -qq git unzip
    - curl -sS https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer
    - composer install --prefer-dist --no-progress
`
	switch fwID {
	case "laravel", "lumen":
		return header +
			"    - cp .env.example .env\n" +
			"    - php artisan key:generate\n" +
			"  script:\n" +
			"    - composer test\n"
	case "symfony", "api-platform":
		return header +
			"  script:\n" +
			"    - APP_ENV=test php bin/phpunit\n"
	case "codeigniter":
		return header +
			"    - cp env .env\n" +
			"  script:\n" +
			"    - composer test\n"
	case "wordpress", "vanilla-php":
		return header +
			"  script:\n" +
			"    - echo \"No tests configured\"\n"
	default:
		return header +
			"  script:\n" +
			"    - " + composerTestIfPresent + "\n"
	}
}
