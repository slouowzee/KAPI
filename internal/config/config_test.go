package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
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
	if cfg.GithubToken != "" || cfg.PackageManager != "" || len(cfg.Favorites) != 0 {
		t.Errorf("Load() on missing file = %+v, want zero value", cfg)
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	setupTempHome(t)

	want := Config{GithubToken: "tok123", PackageManager: "pnpm", Favorites: map[string][]FavoritePackage{"react": {{Name: "react"}}}}
	if err := Save(want); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() after Save() error: %v", err)
	}
	if got.GithubToken != want.GithubToken || got.PackageManager != want.PackageManager || len(got.Favorites) != len(want.Favorites) || got.Favorites["react"][0].Name != want.Favorites["react"][0].Name {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestSaveLoad_FreezeVersionPackages(t *testing.T) {
	setupTempHome(t)

	want := Config{
		FreezeVersionPackages: map[string][]FreezeVersionPackage{
			"npm": {
				{Name: "lodash", Version: "4.17.21", Note: "stable"},
				{Name: "react", Version: "18.2.0"},
			},
			"packagist": {
				{Name: "monolog/monolog", Version: "3.0.0", Note: "last working"},
			},
		},
	}
	if err := Save(want); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() after Save() error: %v", err)
	}

	for reg, wantPkgs := range want.FreezeVersionPackages {
		gotPkgs, ok := got.FreezeVersionPackages[reg]
		if !ok {
			t.Errorf("missing registry %q in loaded config", reg)
			continue
		}
		if len(gotPkgs) != len(wantPkgs) {
			t.Errorf("registry %q: got %d packages, want %d", reg, len(gotPkgs), len(wantPkgs))
			continue
		}
		for i, wantP := range wantPkgs {
			gotP := gotPkgs[i]
			if gotP.Name != wantP.Name || gotP.Version != wantP.Version || gotP.Note != wantP.Note {
				t.Errorf("registry %q[%d] = %+v, want %+v", reg, i, gotP, wantP)
			}
		}
	}
}

func TestSaveLoad_FreezeVersionPackages_Empty(t *testing.T) {
	setupTempHome(t)

	if err := Save(Config{}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(got.FreezeVersionPackages) != 0 {
		t.Errorf("expected empty FreezeVersionPackages, got %+v", got.FreezeVersionPackages)
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

func TestUpdate_ConcurrentUpdatesAreNotLost(t *testing.T) {
	setupTempHome(t)

	const n = 30
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := Update(func(cfg *Config) error {
				if cfg.Favorites == nil {
					cfg.Favorites = make(map[string][]FavoritePackage)
				}
				cfg.Favorites["nextjs"] = append(cfg.Favorites["nextjs"], FavoritePackage{Name: fmt.Sprintf("pkg-%d", i)})
				return nil
			})
			if err != nil {
				t.Errorf("Update: %v", err)
			}
		}(i)
	}
	wg.Wait()

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(cfg.Favorites["nextjs"]); got != n {
		t.Errorf("got %d favorites, want %d", got, n)
	}
}

func TestUpdate_UnreadableConfigIsNotOverwritten(t *testing.T) {
	home := setupTempHome(t)
	path := filepath.Join(home, ".config", "kapi", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	const corrupt = `{"github_token": "keep-me",`
	if err := os.WriteFile(path, []byte(corrupt), 0o600); err != nil {
		t.Fatal(err)
	}

	called := false
	err := Update(func(cfg *Config) error {
		called = true
		return nil
	})

	if err == nil {
		t.Error("expected an error for an unreadable config")
	}
	if called {
		t.Error("fn must not run when the config cannot be loaded")
	}
	data, _ := os.ReadFile(path)
	if string(data) != corrupt {
		t.Errorf("config was modified: %q", data)
	}
}

func TestUpdate_FnErrorSkipsSave(t *testing.T) {
	setupTempHome(t)
	if err := Save(Config{PackageManager: "bun"}); err != nil {
		t.Fatal(err)
	}

	wantErr := errors.New("abort")
	err := Update(func(cfg *Config) error {
		cfg.PackageManager = "npm"
		return wantErr
	})

	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
	cfg, _ := Load()
	if cfg.PackageManager != "bun" {
		t.Errorf("PackageManager = %q, want bun (unchanged)", cfg.PackageManager)
	}
}

func TestSave_LeavesNoTemporaryFiles(t *testing.T) {
	home := setupTempHome(t)
	for i := 0; i < 3; i++ {
		if err := Save(Config{PackageManager: "npm"}); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(home, ".config", "kapi"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "config.json" {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("config dir contains %v, want only config.json", names)
	}
}

func TestConfig_SetAndRemoveFrozen(t *testing.T) {
	var cfg Config
	cfg.SetFrozen("npm", FreezeVersionPackage{Name: "react", Version: "18.0.0"})
	cfg.SetFrozen("npm", FreezeVersionPackage{Name: "vue", Version: "3.0.0"})
	cfg.SetFrozen("npm", FreezeVersionPackage{Name: "react", Version: "19.0.0", Note: "bump"})
	cfg.SetFrozen("packagist", FreezeVersionPackage{Name: "laravel/framework", Version: "11.0.0"})

	if got := cfg.FreezeVersionPackages["npm"]; len(got) != 2 || got[0].Version != "19.0.0" || got[0].Note != "bump" {
		t.Fatalf("SetFrozen should replace existing entries, got %+v", got)
	}

	tests := []struct {
		name     string
		registry string
		pkg      string
		want     bool
	}{
		{name: "wrong registry", registry: "packagist", pkg: "vue", want: false},
		{name: "any registry", registry: "", pkg: "vue", want: true},
		{name: "already removed", registry: "", pkg: "vue", want: false},
		{name: "last of registry", registry: "packagist", pkg: "laravel/framework", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cfg.RemoveFrozen(tt.registry, tt.pkg); got != tt.want {
				t.Errorf("RemoveFrozen(%q, %q) = %v, want %v", tt.registry, tt.pkg, got, tt.want)
			}
		})
	}

	if _, ok := cfg.FreezeVersionPackages["packagist"]; ok {
		t.Error("empty registry should be deleted")
	}
	if got := cfg.FreezeVersionPackages["npm"]; len(got) != 1 || got[0].Name != "react" {
		t.Errorf("npm registry = %+v, want only react", got)
	}
}
