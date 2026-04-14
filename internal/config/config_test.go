package config

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/slouowzee/kapi/internal/testutil"
)

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

func setupTempHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	return tmp
}

func TestGithubToken_FromEnv(t *testing.T) {
	setupTempHome(t)
	t.Setenv("GITHUB_TOKEN", "env-token-abc")
	if got := GithubToken(); got != "env-token-abc" {
		t.Errorf("GithubToken() = %q, want env-token-abc", got)
	}
}

func TestGithubToken_FromConfigFile(t *testing.T) {
	tmp := setupTempHome(t)
	t.Setenv("GITHUB_TOKEN", "")

	dir := filepath.Join(tmp, ".config", "kapi")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(Config{GithubToken: "file-token-xyz"})
	if err := os.WriteFile(filepath.Join(dir, "config.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	if got := GithubToken(); got != "file-token-xyz" {
		t.Errorf("GithubToken() = %q, want file-token-xyz", got)
	}
}

func TestGithubToken_EnvTakesPriorityOverFile(t *testing.T) {
	tmp := setupTempHome(t)
	t.Setenv("GITHUB_TOKEN", "env-wins")

	dir := filepath.Join(tmp, ".config", "kapi")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(Config{GithubToken: "file-loses"})
	if err := os.WriteFile(filepath.Join(dir, "config.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	if got := GithubToken(); got != "env-wins" {
		t.Errorf("GithubToken() = %q, want env-wins", got)
	}
}

func TestGithubToken_Empty(t *testing.T) {
	setupTempHome(t)
	t.Setenv("GITHUB_TOKEN", "")
	if got := GithubToken(); got != "" {
		t.Errorf("GithubToken() = %q, want empty string", got)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	setupTempHome(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() on missing file returned error: %v", err)
	}
	if cfg != (Config{}) {
		t.Errorf("Load() on missing file = %+v, want zero value", cfg)
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	setupTempHome(t)

	want := Config{GithubToken: "tok123", PackageManager: "pnpm"}
	if err := Save(want); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() after Save() error: %v", err)
	}
	if got != want {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestSave_OverwritesPreviousValue(t *testing.T) {
	setupTempHome(t)

	if err := Save(Config{GithubToken: "old"}); err != nil {
		t.Fatal(err)
	}
	if err := Save(Config{GithubToken: "new"}); err != nil {
		t.Fatal(err)
	}

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.GithubToken != "new" {
		t.Errorf("GithubToken after overwrite = %q, want new", got.GithubToken)
	}
}

func TestSave_FilePermissions(t *testing.T) {
	tmp := setupTempHome(t)

	if err := Save(Config{GithubToken: "secret"}); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(tmp, ".config", "kapi", "config.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config.json permissions = %04o, want 0600", perm)
	}
}

func TestSave_CreatesParentDirs(t *testing.T) {
	tmp := setupTempHome(t)

	if err := Save(Config{}); err != nil {
		t.Fatalf("Save() error when parent dirs missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, ".config", "kapi", "config.json")); err != nil {
		t.Errorf("expected config.json to be created: %v", err)
	}
}

func TestFetchTokenScopes_NoToken(t *testing.T) {
	setupTempHome(t)
	t.Setenv("GITHUB_TOKEN", "")

	_, err := FetchTokenScopes(context.Background())
	if err == nil {
		t.Error("FetchTokenScopes() with no token: expected error, got nil")
	}
}

func TestFetchTokenScopes_AllScopes(t *testing.T) {
	setupTempHome(t)
	t.Setenv("GITHUB_TOKEN", "valid-token")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-OAuth-Scopes", "repo, write:public_key, write:gpg_key")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer ts.Close()
	redirectTo(t, ts)

	got, err := FetchTokenScopes(context.Background())
	if err != nil {
		t.Fatalf("FetchTokenScopes() unexpected error: %v", err)
	}
	want := TokenScopes{Repo: true, WritePublicKey: true, WriteGPGKey: true}
	if got != want {
		t.Errorf("FetchTokenScopes() = %+v, want %+v", got, want)
	}
}

func TestFetchTokenScopes_AdminScopeAliases(t *testing.T) {
	setupTempHome(t)
	t.Setenv("GITHUB_TOKEN", "valid-token")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-OAuth-Scopes", "admin:public_key, admin:gpg_key")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer ts.Close()
	redirectTo(t, ts)

	got, err := FetchTokenScopes(context.Background())
	if err != nil {
		t.Fatalf("FetchTokenScopes() unexpected error: %v", err)
	}
	if !got.WritePublicKey || !got.WriteGPGKey {
		t.Errorf("admin:* alias not recognised: %+v", got)
	}
}

func TestFetchTokenScopes_OnlyRepo(t *testing.T) {
	setupTempHome(t)
	t.Setenv("GITHUB_TOKEN", "valid-token")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-OAuth-Scopes", "repo")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer ts.Close()
	redirectTo(t, ts)

	got, err := FetchTokenScopes(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := TokenScopes{Repo: true}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestFetchTokenScopes_EmptyScopeHeader(t *testing.T) {
	setupTempHome(t)
	t.Setenv("GITHUB_TOKEN", "valid-token")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer ts.Close()
	redirectTo(t, ts)

	_, err := FetchTokenScopes(context.Background())
	if err == nil {
		t.Error("expected error for empty scope header, got nil")
	}
}

func TestFetchTokenScopes_HTTPError(t *testing.T) {
	setupTempHome(t)
	t.Setenv("GITHUB_TOKEN", "bad-token")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()
	redirectTo(t, ts)

	_, err := FetchTokenScopes(context.Background())
	if err == nil {
		t.Error("expected error for HTTP 401, got nil")
	}
}

func TestFetchTokenScopes_BearerAuth(t *testing.T) {
	setupTempHome(t)
	const tok = "my-secret-token"
	t.Setenv("GITHUB_TOKEN", tok)

	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("X-OAuth-Scopes", "repo")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer ts.Close()
	redirectTo(t, ts)

	if _, err := FetchTokenScopes(context.Background()); err != nil {
		t.Fatal(err)
	}
	if want := "Bearer " + tok; gotAuth != want {
		t.Errorf("Authorization header = %q, want %q", gotAuth, want)
	}
}
