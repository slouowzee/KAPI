package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/slouowzee/kapi/internal/packagemanager"
	"github.com/slouowzee/kapi/internal/registry"
	"github.com/slouowzee/kapi/tui/screens"
)

func TestRemoteSteps_GithubPrivate_UsesRepoName(t *testing.T) {
	cfg := screens.GitConfig{
		RemoteHost:    "github",
		RemotePrivate: true,
		RepoName:      "my-awesome-repo",
	}
	steps := remoteSteps("/home/user/projects/myproject", cfg)

	if len(steps) == 0 {
		t.Fatal("expected steps for github remote, got none")
	}
	if !strings.Contains(steps[0].Label, "my-awesome-repo") {
		t.Errorf("first step label = %q, want to contain repo name 'my-awesome-repo'", steps[0].Label)
	}
}

func TestRemoteSteps_GithubPublic_UsesRepoName(t *testing.T) {
	cfg := screens.GitConfig{
		RemoteHost:    "github",
		RemotePrivate: false,
		RepoName:      "public-lib",
	}
	steps := remoteSteps("/home/user/projects/myproject", cfg)

	if len(steps) == 0 {
		t.Fatal("expected steps for github remote, got none")
	}
	if !strings.Contains(steps[0].Label, "public-lib") {
		t.Errorf("first step label = %q, want to contain repo name 'public-lib'", steps[0].Label)
	}
}

func TestRemoteSteps_Github_FallsBackToDirBasename(t *testing.T) {
	cfg := screens.GitConfig{
		RemoteHost:    "github",
		RemotePrivate: true,
		RepoName:      "",
	}
	steps := remoteSteps("/home/user/projects/myproject", cfg)

	if len(steps) == 0 {
		t.Fatal("expected steps for github remote, got none")
	}
	if !strings.Contains(steps[0].Label, "myproject") {
		t.Errorf("first step label = %q, want fallback dir basename 'myproject'", steps[0].Label)
	}
}

func TestRemoteSteps_Github_PrivateLabel(t *testing.T) {
	cfg := screens.GitConfig{
		RemoteHost:    "github",
		RemotePrivate: true,
		RepoName:      "repo",
	}
	steps := remoteSteps("/tmp/proj", cfg)

	if !strings.Contains(steps[0].Label, "private") {
		t.Errorf("first step label = %q, want to contain 'private'", steps[0].Label)
	}
}

func TestRemoteSteps_Github_PublicLabel(t *testing.T) {
	cfg := screens.GitConfig{
		RemoteHost:    "github",
		RemotePrivate: false,
		RepoName:      "repo",
	}
	steps := remoteSteps("/tmp/proj", cfg)

	if !strings.Contains(steps[0].Label, "public") {
		t.Errorf("first step label = %q, want to contain 'public'", steps[0].Label)
	}
}

func TestRemoteSteps_Github_ReturnsThreeSteps(t *testing.T) {
	cfg := screens.GitConfig{RemoteHost: "github", RepoName: "repo", InitialCommit: true}
	steps := remoteSteps("/tmp/proj", cfg)

	if len(steps) != 3 {
		t.Errorf("expected 3 steps (create, remote add, push), got %d", len(steps))
	}
}

func TestRemoteSteps_ExistingURL_ReturnsTwoSteps(t *testing.T) {
	cfg := screens.GitConfig{
		RemoteHost:    "custom",
		RemoteURL:     "git@mygit.internal:user/repo.git",
		InitialCommit: true,
	}
	steps := remoteSteps("/tmp/proj", cfg)

	if len(steps) != 2 {
		t.Errorf("expected 2 steps (remote add, push), got %d", len(steps))
	}
}

func TestRemoteSteps_ExistingURL_ContainsURL(t *testing.T) {
	const url = "git@mygit.internal:user/repo.git"
	cfg := screens.GitConfig{RemoteHost: "custom", RemoteURL: url}
	steps := remoteSteps("/tmp/proj", cfg)

	if !strings.Contains(steps[0].Label, url) {
		t.Errorf("first step label = %q, want to contain URL %q", steps[0].Label, url)
	}
}

func TestRemoteSteps_NoRemote_ReturnsNil(t *testing.T) {
	cfg := screens.GitConfig{RemoteHost: "", RemoteURL: ""}
	steps := remoteSteps("/tmp/proj", cfg)

	if steps != nil {
		t.Errorf("expected nil steps when no remote configured, got %v", steps)
	}
}

func TestRemoteSteps_CustomHost_NoURL_ReturnsNil(t *testing.T) {
	cfg := screens.GitConfig{RemoteHost: "custom", RemoteURL: ""}
	steps := remoteSteps("/tmp/proj", cfg)

	if steps != nil {
		t.Errorf("expected nil steps when RemoteURL is empty for custom host, got %v", steps)
	}
}

func TestWriteFileFn_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	fn := writeFileFn(dir, "subdir/hello.txt", "hello world")

	if err := fn(); err != nil {
		t.Fatalf("writeFileFn returned error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "subdir", "hello.txt"))
	if err != nil {
		t.Fatalf("failed to read created file: %v", err)
	}
	if string(content) != "hello world" {
		t.Errorf("file content = %q, want 'hello world'", string(content))
	}
}

func TestWriteFileFn_CreatesNestedDirs(t *testing.T) {
	dir := t.TempDir()
	fn := writeFileFn(dir, "a/b/c/file.txt", "nested")

	if err := fn(); err != nil {
		t.Fatalf("writeFileFn returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "a", "b", "c", "file.txt")); err != nil {
		t.Errorf("expected nested file to be created: %v", err)
	}
}

func TestWriteFileFn_OverwritesExistingFile(t *testing.T) {
	dir := t.TempDir()

	fn1 := writeFileFn(dir, "file.txt", "first")
	if err := fn1(); err != nil {
		t.Fatal(err)
	}

	fn2 := writeFileFn(dir, "file.txt", "second")
	if err := fn2(); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "second" {
		t.Errorf("file content after overwrite = %q, want 'second'", string(content))
	}
}

func fw(id string) registry.Framework { return registry.Framework{ID: id, Ecosystem: "php"} }

func TestGithubActionsCI_Laravel_HasEnvCopyAndKeyGenerate(t *testing.T) {
	out := githubActionsCI(fw("laravel"), packagemanager.NPM)
	if !strings.Contains(out, "cp .env.example .env") {
		t.Error("laravel CI should contain 'cp .env.example .env'")
	}
	if !strings.Contains(out, "php artisan key:generate") {
		t.Error("laravel CI should contain 'php artisan key:generate'")
	}
	if !strings.Contains(out, "composer test") {
		t.Error("laravel CI should contain 'composer test'")
	}
}

func TestGithubActionsCI_Lumen_HasEnvCopyAndKeyGenerate(t *testing.T) {
	out := githubActionsCI(fw("lumen"), packagemanager.NPM)
	if !strings.Contains(out, "cp .env.example .env") {
		t.Error("lumen CI should contain 'cp .env.example .env'")
	}
	if !strings.Contains(out, "php artisan key:generate") {
		t.Error("lumen CI should contain 'php artisan key:generate'")
	}
}

func TestGithubActionsCI_Symfony_NoEnvCopy(t *testing.T) {
	out := githubActionsCI(fw("symfony"), packagemanager.NPM)
	if strings.Contains(out, ".env.example") {
		t.Error("symfony CI must not reference .env.example")
	}
	if strings.Contains(out, "key:generate") {
		t.Error("symfony CI must not run key:generate")
	}
	if !strings.Contains(out, "php bin/phpunit") {
		t.Error("symfony CI should run 'php bin/phpunit'")
	}
	if !strings.Contains(out, "APP_ENV=test") {
		t.Error("symfony CI should set APP_ENV=test")
	}
}

func TestGithubActionsCI_ApiPlatform_NoEnvCopy(t *testing.T) {
	out := githubActionsCI(fw("api-platform"), packagemanager.NPM)
	if strings.Contains(out, ".env.example") {
		t.Error("api-platform CI must not reference .env.example")
	}
	if !strings.Contains(out, "php bin/phpunit") {
		t.Error("api-platform CI should run 'php bin/phpunit'")
	}
}

func TestGithubActionsCI_CodeIgniter_CopiesEnvFile(t *testing.T) {
	out := githubActionsCI(fw("codeigniter"), packagemanager.NPM)
	if strings.Contains(out, ".env.example") {
		t.Error("codeigniter CI must not reference .env.example")
	}
	if !strings.Contains(out, "cp env .env") {
		t.Error("codeigniter CI should contain 'cp env .env'")
	}
	if !strings.Contains(out, "composer test") {
		t.Error("codeigniter CI should run 'composer test'")
	}
}

func TestGithubActionsCI_WordPress_NoTestStep(t *testing.T) {
	out := githubActionsCI(fw("wordpress"), packagemanager.NPM)
	if strings.Contains(out, "composer test") {
		t.Error("wordpress CI should not contain 'composer test'")
	}
	if strings.Contains(out, ".env.example") {
		t.Error("wordpress CI must not reference .env.example")
	}
}

func TestGithubActionsCI_VanillaPhp_NoTestStep(t *testing.T) {
	out := githubActionsCI(fw("vanilla-php"), packagemanager.NPM)
	if strings.Contains(out, "composer test") {
		t.Error("vanilla-php CI should not contain 'composer test'")
	}
}

func TestGithubActionsCI_GenericPhp_ComposerTest(t *testing.T) {
	for _, id := range []string{"slim", "yii", "cakephp", "laminas", "drupal", "phalcon", "fuelphp", "leafphp"} {
		out := githubActionsCI(fw(id), packagemanager.NPM)
		if !strings.Contains(out, "composer test") {
			t.Errorf("%s CI should contain 'composer test'", id)
		}
		if strings.Contains(out, ".env.example") {
			t.Errorf("%s CI must not reference .env.example", id)
		}
	}
}

func TestGitlabCI_Laravel_HasEnvCopyAndKeyGenerate(t *testing.T) {
	out := gitlabCI(fw("laravel"), packagemanager.NPM)
	if !strings.Contains(out, "cp .env.example .env") {
		t.Error("laravel gitlab CI should contain 'cp .env.example .env'")
	}
	if !strings.Contains(out, "php artisan key:generate") {
		t.Error("laravel gitlab CI should contain 'php artisan key:generate'")
	}
	if !strings.Contains(out, "composer test") {
		t.Error("laravel gitlab CI should run 'composer test'")
	}
}

func TestGitlabCI_Symfony_UsesBinPhpunit(t *testing.T) {
	out := gitlabCI(fw("symfony"), packagemanager.NPM)
	if strings.Contains(out, ".env.example") {
		t.Error("symfony gitlab CI must not reference .env.example")
	}
	if !strings.Contains(out, "php bin/phpunit") {
		t.Error("symfony gitlab CI should run 'php bin/phpunit'")
	}
}

func TestGitlabCI_CodeIgniter_CopiesEnvFile(t *testing.T) {
	out := gitlabCI(fw("codeigniter"), packagemanager.NPM)
	if !strings.Contains(out, "cp env .env") {
		t.Error("codeigniter gitlab CI should contain 'cp env .env'")
	}
}

func TestGitlabCI_WordPress_NoTestCommand(t *testing.T) {
	out := gitlabCI(fw("wordpress"), packagemanager.NPM)
	if strings.Contains(out, "composer test") {
		t.Error("wordpress gitlab CI should not run 'composer test'")
	}
}

func jsfw(id string) registry.Framework { return registry.Framework{ID: id, Ecosystem: "js"} }

func TestGithubActionsCI_JS_NPM_UsesIfPresentAfter(t *testing.T) {
	out := githubActionsCI(jsfw("nextjs"), packagemanager.NPM)
	if !strings.Contains(out, "npm run test --if-present") {
		t.Errorf("npm CI should contain 'npm run test --if-present', got:\n%s", out)
	}
	if !strings.Contains(out, "npm run build --if-present") {
		t.Errorf("npm CI should contain 'npm run build --if-present', got:\n%s", out)
	}
}

func TestGithubActionsCI_JS_PNPM_UsesIfPresentBefore(t *testing.T) {
	out := githubActionsCI(jsfw("nextjs"), packagemanager.PNPM)
	if !strings.Contains(out, "pnpm run --if-present test") {
		t.Errorf("pnpm CI should contain 'pnpm run --if-present test', got:\n%s", out)
	}
	if strings.Contains(out, "pnpm run test --if-present") {
		t.Errorf("pnpm CI must not put --if-present after script name, got:\n%s", out)
	}
}

func TestGithubActionsCI_JS_Yarn_UsesIfPresentBefore(t *testing.T) {
	out := githubActionsCI(jsfw("nuxt"), packagemanager.Yarn)
	if !strings.Contains(out, "yarn run --if-present test") {
		t.Errorf("yarn CI should contain 'yarn run --if-present test', got:\n%s", out)
	}
	if strings.Contains(out, "yarn run test --if-present") {
		t.Errorf("yarn CI must not put --if-present after script name, got:\n%s", out)
	}
}

func TestGithubActionsCI_JS_Bun_UsesIfPresentBefore(t *testing.T) {
	out := githubActionsCI(jsfw("sveltekit"), packagemanager.Bun)
	if !strings.Contains(out, "npm run test --if-present") {
		t.Errorf("bun CI should fallback to npm run test --if-present, got:\n%s", out)
	}
}

func TestGitlabCI_JS_PNPM_UsesIfPresentBefore(t *testing.T) {
	out := gitlabCI(jsfw("express"), packagemanager.PNPM)
	if !strings.Contains(out, "pnpm run --if-present test") {
		t.Errorf("pnpm gitlab CI should contain 'pnpm run --if-present test', got:\n%s", out)
	}
}

func TestRemoteSteps_ExistingRemote_ReturnsNil(t *testing.T) {
	tests := []struct {
		name string
		cfg  screens.GitConfig
	}{
		{name: "github origin", cfg: screens.GitConfig{HasExistingGit: true, HasExistingRemote: true, RemoteHost: "github", RemoteURL: "git@github.com:me/app.git"}},
		{name: "gitlab origin", cfg: screens.GitConfig{HasExistingGit: true, HasExistingRemote: true, RemoteHost: "gitlab", RemoteURL: "git@gitlab.com:me/app.git"}},
		{name: "custom origin", cfg: screens.GitConfig{HasExistingGit: true, HasExistingRemote: true, RemoteHost: "custom", RemoteURL: "git@mygit.internal:me/app.git"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if steps := remoteSteps("/tmp/proj", tt.cfg); steps != nil {
				t.Errorf("expected no remote steps for an existing origin, got %d", len(steps))
			}
		})
	}
}

func stepLabels(steps []Step) []string {
	labels := make([]string, len(steps))
	for i, s := range steps {
		labels[i] = s.Label
	}
	return labels
}

func hasLabel(steps []Step, prefix string) bool {
	for _, l := range stepLabels(steps) {
		if strings.HasPrefix(l, prefix) {
			return true
		}
	}
	return false
}

func TestPlan_GitSteps(t *testing.T) {
	tests := []struct {
		name      string
		cfg       screens.GitConfig
		wantInit  bool
		wantDev   bool
		wantPush  bool
		wantRepo  bool
		wantFiles bool
	}{
		{
			name:     "init without initial commit",
			cfg:      screens.GitConfig{InitLocal: true, RemoteHost: "custom", RemoteURL: "git@host:me/app.git"},
			wantInit: true, wantRepo: false, wantPush: false,
		},
		{
			name:     "init with initial commit pushes",
			cfg:      screens.GitConfig{InitLocal: true, InitialCommit: true, RemoteHost: "custom", RemoteURL: "git@host:me/app.git"},
			wantInit: true, wantPush: true,
		},
		{
			name:      "no repo skips remote and dev branch",
			cfg:       screens.GitConfig{Collab: true, RemoteHost: "github", RepoName: "app"},
			wantFiles: true,
		},
		{
			name:      "collab with repo creates dev branch",
			cfg:       screens.GitConfig{InitLocal: true, InitialCommit: true, Collab: true},
			wantInit:  true,
			wantDev:   true,
			wantFiles: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps := Plan("/tmp/x/app", fw("laravel"), nil, tt.cfg, packagemanager.None)
			if got := hasLabel(steps, "git init"); got != tt.wantInit {
				t.Errorf("git init present = %v, want %v (%v)", got, tt.wantInit, stepLabels(steps))
			}
			if got := hasLabel(steps, "create dev branch"); got != tt.wantDev {
				t.Errorf("dev branch present = %v, want %v (%v)", got, tt.wantDev, stepLabels(steps))
			}
			if got := hasLabel(steps, "git push"); got != tt.wantPush {
				t.Errorf("push present = %v, want %v (%v)", got, tt.wantPush, stepLabels(steps))
			}
			if got := hasLabel(steps, "create public GitHub repo") || hasLabel(steps, "create private GitHub repo"); got != tt.wantRepo {
				t.Errorf("github repo creation present = %v, want %v (%v)", got, tt.wantRepo, stepLabels(steps))
			}
			if got := hasLabel(steps, "write CONTRIBUTING.md"); got != tt.wantFiles {
				t.Errorf("collab files present = %v, want %v (%v)", got, tt.wantFiles, stepLabels(steps))
			}
		})
	}
}
