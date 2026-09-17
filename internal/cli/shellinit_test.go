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

// TestInstallScript_GeneratesIntegrationFromBinary guards against
// re-introducing a hardcoded copy of the shell integration text in
// install.sh: it must call `kapi shell-init` on the freshly installed
// binary instead, so the two can never drift apart again.
func TestInstallScript_GeneratesIntegrationFromBinary(t *testing.T) {
	install, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(install)

	if !strings.Contains(content, `shell-init`) {
		t.Error("install.sh should call `kapi shell-init` to generate the integration snippet")
	}
	for _, shell := range []string{"zsh", "fish", "nu"} {
		script, _ := shellInitScript(shell)
		if strings.Contains(content, strings.TrimSuffix(script, "\n")) {
			t.Errorf("install.sh embeds a hardcoded copy of the %s integration instead of generating it", shell)
		}
	}
}
