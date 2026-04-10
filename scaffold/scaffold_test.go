package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	cfg := screens.GitConfig{RemoteHost: "github", RepoName: "repo"}
	steps := remoteSteps("/tmp/proj", cfg)

	if len(steps) != 3 {
		t.Errorf("expected 3 steps (create, remote add, push), got %d", len(steps))
	}
}

func TestRemoteSteps_ExistingURL_ReturnsTwoSteps(t *testing.T) {
	cfg := screens.GitConfig{
		RemoteHost: "custom",
		RemoteURL:  "git@mygit.internal:user/repo.git",
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
