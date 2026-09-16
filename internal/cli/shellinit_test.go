package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellInitScript(t *testing.T) {
	tests := []struct {
		shell  string
		wantOK bool
		check  string
	}{
		{shell: "bash", wantOK: true, check: "bash"},
		{shell: "zsh", wantOK: true, check: "zsh"},
		{shell: "fish", wantOK: true, check: "fish"},
		{shell: "nu", wantOK: true},
		{shell: "PowerShell", wantOK: true},
		{shell: "tcsh", wantOK: false},
		{shell: "", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			script, ok := shellInitScript(tt.shell)
			if ok != tt.wantOK {
				t.Fatalf("shellInitScript(%q) ok = %v, want %v", tt.shell, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			for _, want := range []string{"KAPI_SHELL_WRAPPER", ".kapi_last_cd", "kapi shell integration"} {
				if !strings.Contains(script, want) {
					t.Errorf("%s script is missing %q", tt.shell, want)
				}
			}
			if tt.check == "" {
				return
			}
			bin, err := exec.LookPath(tt.check)
			if err != nil {
				t.Skipf("%s not installed", tt.check)
			}
			file := filepath.Join(t.TempDir(), "kapi.sh")
			if err := os.WriteFile(file, []byte(script), 0o644); err != nil {
				t.Fatal(err)
			}
			if out, err := exec.Command(bin, "-n", file).CombinedOutput(); err != nil {
				t.Errorf("%s -n rejected the script: %v\n%s", tt.check, err, out)
			}
		})
	}
}

func TestShellInitScript_MatchesInstallScript(t *testing.T) {
	install, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, shell := range []string{"zsh", "fish", "nu"} {
		script, _ := shellInitScript(shell)
		if !strings.Contains(string(install), strings.TrimSuffix(script, "\n")) {
			t.Errorf("%s integration differs from install.sh", shell)
		}
	}
}
