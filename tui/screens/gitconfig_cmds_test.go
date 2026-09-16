package screens

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestExecGitSigningCmd_SwitchingFormatUpdatesGpgFormat(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	tests := []struct {
		format string
		want   string
	}{
		{format: "ssh", want: "ssh"},
		{format: "gpg", want: "openpgp"},
	}
	for _, tt := range tests {
		msg := execGitSigningCmd(dir, tt.format, "local", "KEY")().(gitcfgSigningDoneMsg)
		if msg.err != nil {
			t.Fatalf("%s signing: %v", tt.format, msg.err)
		}
		c := exec.Command("git", "config", "--local", "gpg.format")
		c.Dir = dir
		c.Env = os.Environ()
		out, err := c.Output()
		if err != nil {
			t.Fatalf("read gpg.format: %v", err)
		}
		if got := strings.TrimSpace(string(out)); got != tt.want {
			t.Errorf("after %s signing gpg.format = %q, want %q", tt.format, got, tt.want)
		}
	}
}
