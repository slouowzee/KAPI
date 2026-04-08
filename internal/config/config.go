package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func GithubToken() string {
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		return tok
	}
	if cfg, err := Load(); err == nil && cfg.GithubToken != "" {
		return cfg.GithubToken
	}
	return ""
}

type TokenScopes struct {
	Repo           bool
	WritePublicKey bool
	WriteGPGKey    bool
}

func FetchTokenScopes(ctx context.Context) (TokenScopes, error) {
	tok := GithubToken()
	if tok == "" {
		return TokenScopes{}, fmt.Errorf("no GitHub token configured")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return TokenScopes{}, fmt.Errorf("build GitHub API request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return TokenScopes{}, fmt.Errorf("fetch GitHub scopes: %w", err)
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return TokenScopes{}, fmt.Errorf("GitHub API: HTTP %d", resp.StatusCode)
	}

	raw := resp.Header.Get("X-OAuth-Scopes")
	if raw == "" {
		return TokenScopes{}, fmt.Errorf("GitHub token missing required scopes")
	}

	var s TokenScopes
	for _, scope := range strings.Split(raw, ",") {
		switch strings.TrimSpace(scope) {
		case "repo":
			s.Repo = true
		case "write:public_key", "admin:public_key":
			s.WritePublicKey = true
		case "write:gpg_key", "admin:gpg_key":
			s.WriteGPGKey = true
		}
	}
	return s, nil
}

type Config struct {
	GithubToken    string `json:"github_token,omitempty"`
	PackageManager string `json:"package_manager,omitempty"`
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

func Save(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "kapi", "config.json"), nil
}
