package screens

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slouowzee/kapi/internal/testutil"
)

// redirectTo sends every request to ts, so no test can reach the real API.
func redirectTo(t *testing.T, ts *httptest.Server) {
	t.Helper()
	orig := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = orig })
	inner := orig
	http.DefaultTransport = testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		req2 := req.Clone(req.Context())
		req2.URL.Scheme = "http"
		req2.URL.Host = ts.Listener.Addr().String()
		return inner.RoundTrip(req2)
	})
}

func TestRemoteFlow_NameInputThenProtocolChoice(t *testing.T) {
	m := GitConfigModel{step: GITCFG_STEP_REMOTE_NAME_INPUT, remoteRepoName: "  my-app  "}

	updated, cmd := m.handleRemoteNameInput(tea.KeyMsg{Type: tea.KeyEnter})
	if updated.step != GITCFG_STEP_REMOTE_PROTOCOL || cmd != nil {
		t.Fatalf("step = %v, cmd = %v, want GITCFG_STEP_REMOTE_PROTOCOL with no command", updated.step, cmd)
	}
	if updated.remoteRepoName != "my-app" {
		t.Errorf("remoteRepoName = %q, want trimmed 'my-app'", updated.remoteRepoName)
	}
	if updated.remoteProtocolCursor != 0 {
		t.Errorf("remoteProtocolCursor = %d, want 0 (SSH default)", updated.remoteProtocolCursor)
	}

	updated, _ = updated.handleRemoteProtocol(tea.KeyMsg{Type: tea.KeyRight})
	if updated.remoteProtocolCursor != 1 {
		t.Errorf("after right, cursor = %d, want 1 (HTTPS)", updated.remoteProtocolCursor)
	}

	updated, cmd = updated.handleRemoteProtocol(tea.KeyMsg{Type: tea.KeyEnter})
	if !updated.remoteHTTPS {
		t.Error("remoteHTTPS should be true after confirming HTTPS")
	}
	if updated.step != GITCFG_STEP_EXECUTING || cmd == nil {
		t.Errorf("step = %v, cmd = %v, want GITCFG_STEP_EXECUTING with a command", updated.step, cmd)
	}
}

func TestRemoteFlow_EscFromProtocolGoesBackToNameInput(t *testing.T) {
	m := GitConfigModel{step: GITCFG_STEP_REMOTE_PROTOCOL, remoteRepoName: "my-app"}
	updated, _ := m.handleRemoteProtocol(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.step != GITCFG_STEP_REMOTE_NAME_INPUT {
		t.Errorf("step = %v, want GITCFG_STEP_REMOTE_NAME_INPUT", updated.step)
	}
}

func TestExecGithubCreateRepoCmd_ProtocolChoosesRemoteURL(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	t.Setenv("GITHUB_TOKEN", "test-token")

	const sshURL = "git@github.com:me/my-app.git"
	const cloneURL = "https://github.com/me/my-app.git"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"ssh_url": sshURL, "clone_url": cloneURL})
	}))
	defer ts.Close()
	redirectTo(t, ts)

	tests := []struct {
		name  string
		https bool
		want  string
	}{
		{name: "ssh by default", https: false, want: sshURL},
		{name: "https when chosen", https: true, want: cloneURL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
				t.Fatalf("git init: %v\n%s", err, out)
			}

			msg, ok := execGithubCreateRepoCmd("my-app", false, tt.https, dir)().(gitcfgExecMsg)
			if !ok || msg.err != nil {
				t.Fatalf("execGithubCreateRepoCmd: %+v", msg)
			}
			if msg.newRemoteURL != tt.want {
				t.Errorf("newRemoteURL = %q, want %q", msg.newRemoteURL, tt.want)
			}

			c := exec.Command("git", "remote", "get-url", "origin")
			c.Dir = dir
			out, err := c.Output()
			if err != nil {
				t.Fatalf("git remote get-url: %v", err)
			}
			if got := strings.TrimSpace(string(out)); got != tt.want {
				t.Errorf("origin url = %q, want %q", got, tt.want)
			}
		})
	}
}
