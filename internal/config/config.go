package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func GithubToken() string {
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		return tok
	}
	if cfg, err := Load(); err == nil && cfg.GithubToken != "" {
		return cfg.GithubToken
	}
	if tok := ghAuthToken(); tok != "" {
		return tok
	}
	return ""
}

func ghAuthToken() string {
	path, err := exec.LookPath("gh")
	if err != nil || path == "" {
		return ""
	}
	out, err := exec.Command(path, "auth", "token").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// TokenScopes lists the classic token scopes kapi relies on. SSH keys are
// registered as signing keys, which GitHub requires to verify signed commits.
type TokenScopes struct {
	Repo               bool
	WriteSSHSigningKey bool
	WriteGPGKey        bool
}

func FetchTokenScopes(ctx context.Context) (TokenScopes, error) {
	tok := GithubToken()
	if tok == "" {
		return TokenScopes{}, errors.New("no GitHub token configured")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return TokenScopes{}, fmt.Errorf("build GitHub API request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return TokenScopes{}, fmt.Errorf("fetch GitHub scopes: %w", err)
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return TokenScopes{}, fmt.Errorf("GitHub API: HTTP %d", resp.StatusCode)
	}

	raw := resp.Header.Get("X-OAuth-Scopes")
	if raw == "" {
		return TokenScopes{}, errors.New("GitHub token missing required scopes")
	}

	var s TokenScopes
	for _, scope := range strings.Split(raw, ",") {
		switch strings.TrimSpace(scope) {
		case "repo":
			s.Repo = true
		case "write:ssh_signing_key", "admin:ssh_signing_key":
			s.WriteSSHSigningKey = true
		case "write:gpg_key", "admin:gpg_key":
			s.WriteGPGKey = true
		}
	}
	return s, nil
}

func CheckGitHubScopeError(resp *http.Response) error {
	if resp.StatusCode == http.StatusNotFound {
		if acc := resp.Header.Get("X-Accepted-OAuth-Scopes"); acc != "" {
			has := resp.Header.Get("X-OAuth-Scopes")
			return fmt.Errorf("GitHub token missing required scopes (has: %s, needs: %s)", has, acc)
		}
	}
	return nil
}

type Config struct {
	GithubToken           string                            `json:"github_token,omitempty"`
	PackageManager        string                            `json:"package_manager,omitempty"`
	Favorites             map[string][]FavoritePackage      `json:"favorites,omitempty"`
	FreezeVersionPackages map[string][]FreezeVersionPackage `json:"freeze_version_packages,omitempty"`
}

type FavoritePackage struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type FreezeVersionPackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Note    string `json:"note,omitempty"`
}

// SetFrozen adds or replaces the frozen version of a package for a registry
// ("npm" or "packagist").
func (c *Config) SetFrozen(registry string, pkg FreezeVersionPackage) {
	if c.FreezeVersionPackages == nil {
		c.FreezeVersionPackages = make(map[string][]FreezeVersionPackage)
	}
	for i, f := range c.FreezeVersionPackages[registry] {
		if f.Name == pkg.Name {
			c.FreezeVersionPackages[registry][i] = pkg
			return
		}
	}
	c.FreezeVersionPackages[registry] = append(c.FreezeVersionPackages[registry], pkg)
}

// RemoveFrozen removes a frozen package from the given registry, or from any
// registry when registry is empty. It reports whether a package was removed.
func (c *Config) RemoveFrozen(registry, name string) bool {
	for reg, pkgs := range c.FreezeVersionPackages {
		if registry != "" && reg != registry {
			continue
		}
		for i, p := range pkgs {
			if p.Name != name {
				continue
			}
			if len(pkgs) == 1 {
				delete(c.FreezeVersionPackages, reg)
			} else {
				c.FreezeVersionPackages[reg] = append(pkgs[:i:i], pkgs[i+1:]...)
			}
			return true
		}
	}
	return false
}

func Load() (Config, error) {
	var cfg Config
	path, err := configPath()
	if err != nil {
		return cfg, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	err = json.Unmarshal(data, &cfg)
	return cfg, err
}

// writeMu serializes config writes inside the process: TUI commands run in
// concurrent goroutines and would otherwise overwrite each other's changes.
var writeMu sync.Mutex

// Update loads the config, applies fn and saves the result. If the config
// cannot be read, fn is not called and nothing is written, so an unreadable
// file is never replaced by an empty config.
func Update(fn func(cfg *Config) error) error {
	writeMu.Lock()
	defer writeMu.Unlock()

	cfg, err := Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := fn(&cfg); err != nil {
		return err
	}
	return save(cfg)
}

func Save(cfg Config) error {
	writeMu.Lock()
	defer writeMu.Unlock()
	return save(cfg)
}

// save writes the config to a temporary file and renames it over the real
// one, so a crash mid-write never leaves a truncated config behind.
func save(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "config-*.json")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set config permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "kapi", "config.json"), nil
}
