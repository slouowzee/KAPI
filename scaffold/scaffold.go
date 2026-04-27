package scaffold

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/slouowzee/kapi/internal/gitconfig"
	"github.com/slouowzee/kapi/internal/packagemanager"
	"github.com/slouowzee/kapi/internal/packages"
	"github.com/slouowzee/kapi/internal/registry"
)

type Step struct {
	Label    string
	Cmd      *exec.Cmd
	Fn       func() error
	StreamFn func(onLine func(string)) error
}

func Plan(
	targetDir string,
	fw registry.Framework,
	selectedPkgs []packages.Package,
	gitCfg gitconfig.GitConfig,
	pm packagemanager.PM,
) []Step {
	var steps []Step

	steps = append(steps, frameworkSteps(targetDir, fw, pm)...)

	if len(selectedPkgs) > 0 {
		steps = append(steps, packageSteps(targetDir, fw, selectedPkgs, pm)...)
	}

	if gitCfg.InitLocal && !gitCfg.HasExistingGit {
		if gitCfg.InitialCommit {
			steps = append(steps, Step{
				Label:    "git init",
				StreamFn: streamCmd(targetDir, "git", "init"),
			})
			if gitCfg.UniversalGitignore {
				steps = append(steps, Step{
					Label: "write universal .gitignore",
					Fn:    writeFileFn(targetDir, ".gitignore", universalGitignore),
				})
			}
			steps = append(steps, initialCommitStep(targetDir))
		}
	}

	if gitCfg.Collab {
		steps = append(steps, collabSteps(targetDir)...)
	}

	switch gitCfg.CI {
	case "github":
		steps = append(steps, ciGithubStep(targetDir, fw, pm))
	case "gitlab":
		steps = append(steps, ciGitlabStep(targetDir, fw, pm))
	}

	steps = append(steps, remoteSteps(targetDir, gitCfg)...)

	return steps
}

func remoteSteps(targetDir string, gitCfg gitconfig.GitConfig) []Step {
	switch gitCfg.RemoteHost {
	case "github":
		name := gitCfg.RepoName
		if name == "" {
			name = filepath.Base(targetDir)
		}
		private := gitCfg.RemotePrivate
		sshURL := new(string)

		visibility := "public"
		if private {
			visibility = "private"
		}

		return []Step{
			{
				Label: "create " + visibility + " GitHub repo: " + name,
				Fn: func() error {
					ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
					defer cancel()
					url, err := createGithubRepo(ctx, name, private)
					if err != nil {
						return err
					}
					*sshURL = url
					return nil
				},
			},
			{
				Label: "git remote add origin <github url>",
				Fn: func() error {
					return gitSilentCmd(targetDir, "remote", "add", "origin", *sshURL).Run()
				},
			},
			{
				Label:    "git push -u origin HEAD",
				StreamFn: streamGitCmd(targetDir, "push", "-u", "origin", "HEAD"),
			},
		}

	default:
		if gitCfg.RemoteURL == "" {
			return nil
		}
		return []Step{
			{
				Label:    "git remote add origin " + gitCfg.RemoteURL,
				StreamFn: streamGitCmd(targetDir, "remote", "add", "origin", gitCfg.RemoteURL),
			},
			{
				Label:    "git push -u origin HEAD",
				StreamFn: streamGitCmd(targetDir, "push", "-u", "origin", "HEAD"),
			},
		}
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
		return []Step{{
			Label:    "composer create-project phalcon/phalcon " + name,
			StreamFn: streamCmd(parent, "composer", "create-project", "phalcon/phalcon", name),
		}}
	case "fuelphp":
		return []Step{{
			Label:    "composer create-project fuel/fuel " + name,
			StreamFn: streamCmd(parent, "composer", "create-project", "fuel/fuel", name),
		}}
	case "leafphp":
		return []Step{{
			Label:    "composer create-project leafs/leaf " + name,
			StreamFn: streamCmd(parent, "composer", "create-project", "leafs/leaf", name),
		}}
	case "api-platform":
		return []Step{{
			Label:    "composer create-project api-platform/api-platform " + name,
			StreamFn: streamCmd(parent, "composer", "create-project", "api-platform/api-platform", name),
		}}
	case "vanilla-php":
		return vanillaPhpSteps(targetDir, name, parent)

	case "nextjs":
		return jsExecStep(parent, pm, "create-next-app@latest", name)
	case "nuxt":
		return jsExecStep(parent, pm, "nuxi@latest", "init", name)
	case "remix":
		return jsExecStep(parent, pm, "create-remix@latest", name)
	case "tanstack-start":
		return jsExecStep(parent, pm, "create-tsrouter-app@latest", name, "--framework", "react", "--add-ons", "start")
	case "astro":
		return jsExecStep(parent, pm, "astro@latest", name)
	case "gatsby":
		return jsExecStep(parent, pm, "gatsby", "new", name)
	case "sveltekit":
		return jsExecStep(parent, pm, "sv", "create", name)
	case "analog":
		return jsExecStep(parent, pm, "create-nx-workspace@latest", name, "--preset=@analogjs/platform")
	case "hono":
		return jsExecStep(parent, pm, "hono@latest", name)
	case "react-native":
		return jsExecStep(parent, pm, "@react-native-community/cli@latest", "init", name)

	case "react-vite":
		return jsCreateStreamStep(parent, pm, "vite@latest", name, "--", "--template", "react-ts")
	case "vue-vite":
		return jsCreateStreamStep(parent, pm, "vite@latest", name, "--", "--template", "vue-ts")
	case "svelte-vite":
		return jsCreateStreamStep(parent, pm, "vite@latest", name, "--", "--template", "svelte-ts")
	case "vanilla-vite":
		return jsCreateStreamStep(parent, pm, "vite@latest", name, "--", "--template", "vanilla-ts")
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

	return []Step{{
		Label:    "mkdir " + targetDir,
		StreamFn: streamCmd("", "mkdir", "-p", targetDir),
	}}
}

func jsExecStep(dir string, pm packagemanager.PM, pkg string, extra ...string) []Step {
	argv := append(append([]string(nil), pm.ExecArgs()...), pkg)
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

func vanillaPhpSteps(targetDir, name, parent string) []Step {
	return []Step{
		{
			Label:    "mkdir " + name,
			StreamFn: streamCmd(parent, "mkdir", "-p", name),
		},
		{
			Label:    "composer init (in " + name + ")",
			StreamFn: streamCmd(targetDir, "composer", "init", "--no-interaction", "--name="+name+"/"+name),
		},
	}
}

func packageSteps(targetDir string, fw registry.Framework, pkgs []packages.Package, pm packagemanager.PM) []Step {
	if len(pkgs) == 0 {
		return nil
	}
	names := make([]string, len(pkgs))
	for i, p := range pkgs {
		names[i] = p.Name
	}

	if fw.Ecosystem == "php" {
		args := append([]string{"require"}, names...)
		return []Step{{
			Label:    "composer require " + strings.Join(names, " "),
			StreamFn: streamCmdSlice(targetDir, append([]string{"composer"}, args...)),
		}}
	}
	argv := append(append([]string(nil), pm.InstallArgs()...), names...)
	return []Step{{
		Label:    strings.Join(argv, " "),
		StreamFn: streamCmdSlice(targetDir, argv),
	}}
}

func collabSteps(targetDir string) []Step {
	return []Step{
		{
			Label:    "create dev branch",
			StreamFn: streamGitCmd(targetDir, "checkout", "-b", "dev"),
		},
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

func streamCmd(dir string, name string, args ...string) func(onLine func(string)) error {
	return func(onLine func(string)) error {
		c := exec.Command(name, args...)
		if dir != "" {
			c.Dir = dir
		}
		return runStreamed(c, onLine)
	}
}

func streamCmdSlice(dir string, argv []string) func(onLine func(string)) error {
	if len(argv) == 0 {
		return func(onLine func(string)) error { return nil }
	}
	return streamCmd(dir, argv[0], argv[1:]...)
}

func streamGitCmd(targetDir string, args ...string) func(onLine func(string)) error {
	return streamCmd(targetDir, "git", args...)
}

func runStreamed(c *exec.Cmd, onLine func(string)) error {
	stdout, err := c.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := c.StderrPipe()
	if err != nil {
		return err
	}
	if err := c.Start(); err != nil {
		return err
	}

	lines := make(chan string)
	var wg sync.WaitGroup

	scanPipe := func(r io.Reader) {
		defer wg.Done()
		s := bufio.NewScanner(r)
		for s.Scan() {
			lines <- s.Text()
		}
	}

	wg.Add(2)
	go scanPipe(stdout)
	go scanPipe(stderr)

	go func() {
		wg.Wait()
		close(lines)
	}()

	for line := range lines {
		onLine(line)
	}

	return c.Wait()
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
			"      - run: composer test\n"
	}
}

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
			"    - composer test\n"
	}
}
