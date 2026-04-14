package scaffold

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/slouowzee/kapi/internal/config"
)

const githubClientTimeout = 15 * time.Second

func createGithubRepo(ctx context.Context, name string, private bool) (sshURL string, err error) {
	tok := config.GithubToken()
	if tok == "" {
		return "", errors.New("GitHub token not configured — run: kapi config github.token <token>")
	}

	body, err := json.Marshal(map[string]any{"name": name, "private": private})
	if err != nil {
		return "", fmt.Errorf("could not encode GitHub API request body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.github.com/user/repos", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("could not build GitHub API request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{Timeout: githubClientTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("GitHub API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("could not read GitHub API response: %w", err)
	}

	if resp.StatusCode == http.StatusUnprocessableEntity {
		return "", fmt.Errorf("GitHub repo %q already exists", name)
	}
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("GitHub API error (HTTP %d): %s", resp.StatusCode, string(respBytes))
	}

	var result struct {
		SSHUrl string `json:"ssh_url"`
	}
	if err := json.Unmarshal(respBytes, &result); err != nil || result.SSHUrl == "" {
		return "", errors.New("could not parse SSH URL from GitHub response")
	}
	return result.SSHUrl, nil
}
