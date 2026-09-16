// Package github wraps the GitHub REST calls kapi needs to create remotes.
package github

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

const (
	apiURL        = "https://api.github.com"
	clientTimeout = 15 * time.Second
)

var ErrNoToken = errors.New("GitHub token not configured — run: kapi config github.token <token>")

// Repo holds the remote URLs of a repository.
type Repo struct {
	SSHURL   string `json:"ssh_url"`
	CloneURL string `json:"clone_url"`
	// Size is in kilobytes; 0 means nothing was ever pushed.
	Size int `json:"size"`
}

// CreateRepo creates a repository for the authenticated user. When the name is
// already taken by an empty repository of that user (typically left behind by
// a previous kapi run that failed later on), that repository is reused.
func CreateRepo(ctx context.Context, token, name string, private bool) (Repo, error) {
	if token == "" {
		return Repo{}, ErrNoToken
	}

	body, err := json.Marshal(map[string]any{"name": name, "private": private})
	if err != nil {
		return Repo{}, fmt.Errorf("encode GitHub API request body: %w", err)
	}

	client := &http.Client{Timeout: clientTimeout}
	resp, data, err := do(ctx, client, token, http.MethodPost, apiURL+"/user/repos", body)
	if err != nil {
		return Repo{}, err
	}

	switch {
	case resp.StatusCode == http.StatusUnprocessableEntity:
		if repo, ok := reusableRepo(ctx, client, token, name); ok {
			return repo, nil
		}
		return Repo{}, fmt.Errorf("could not create GitHub repo %q: %s", name, apiMessage(data, "the name is already used"))
	case resp.StatusCode != http.StatusCreated:
		if scopeErr := config.CheckGitHubScopeError(resp); scopeErr != nil {
			return Repo{}, scopeErr
		}
		return Repo{}, fmt.Errorf("GitHub API error (HTTP %d): %s", resp.StatusCode, apiMessage(data, string(data)))
	}

	var repo Repo
	if err := json.Unmarshal(data, &repo); err != nil || repo.SSHURL == "" {
		return Repo{}, errors.New("could not parse repository URLs from GitHub response")
	}
	return repo, nil
}

// reusableRepo returns the user's repository with that name if it is empty.
func reusableRepo(ctx context.Context, client *http.Client, token, name string) (Repo, bool) {
	resp, data, err := do(ctx, client, token, http.MethodGet, apiURL+"/user", nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		return Repo{}, false
	}
	var user struct {
		Login string `json:"login"`
	}
	if json.Unmarshal(data, &user) != nil || user.Login == "" {
		return Repo{}, false
	}

	resp, data, err = do(ctx, client, token, http.MethodGet, apiURL+"/repos/"+user.Login+"/"+name, nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		return Repo{}, false
	}
	var repo Repo
	if json.Unmarshal(data, &repo) != nil || repo.SSHURL == "" || repo.Size != 0 {
		return Repo{}, false
	}
	return repo, true
}

func do(ctx context.Context, client *http.Client, token, method, url string, body []byte) (*http.Response, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("build GitHub API request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("GitHub API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read GitHub API response: %w", err)
	}
	return resp, data, nil
}

// apiMessage extracts the most specific error message of a GitHub API error.
func apiMessage(data []byte, fallback string) string {
	var payload struct {
		Message string `json:"message"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if json.Unmarshal(data, &payload) != nil {
		return fallback
	}
	for _, e := range payload.Errors {
		if e.Message != "" {
			return e.Message
		}
	}
	if payload.Message != "" {
		return payload.Message
	}
	return fallback
}
