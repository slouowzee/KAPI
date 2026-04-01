package registry

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"time"
)

const API_URL = "#"

const API_TIMEOUT = 3 * time.Second

//go:embed frameworks.json
var fallbackData []byte

type Framework struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Ecosystem        string   `json:"ecosystem"`
	Type             string   `json:"type,omitempty"`
	Description      string   `json:"description"`
	Tags             []string `json:"tags"`
	NpmPackage       string   `json:"npm_package,omitempty"`
	PackagistPackage string   `json:"packagist_package,omitempty"`
	GithubRepo       string   `json:"github_repo,omitempty"`
}

type registryPayload struct {
	Frameworks []Framework `json:"frameworks"`
}

func Load() ([]Framework, error) {
	if live, err := fetchLive(); err == nil {
		return live, nil
	}
	return loadFallback()
}

func fetchLive() ([]Framework, error) {
	if API_URL == "#" {
		// API not up yet
		return nil, errPlaceholder
	}

	client := &http.Client{Timeout: API_TIMEOUT}
	resp, err := client.Get(API_URL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var payload registryPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Frameworks, nil
}

func loadFallback() ([]Framework, error) {
	var payload registryPayload
	if err := json.Unmarshal(fallbackData, &payload); err != nil {
		return nil, err
	}
	return payload.Frameworks, nil
}

var errPlaceholder = placeholderErr("API URL is a placeholder")

type placeholderErr string

func (e placeholderErr) Error() string { return string(e) }
