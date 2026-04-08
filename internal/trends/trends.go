package trends

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/slouowzee/kapi/internal/semver"
)

const fetchTimeout = 5 * time.Second
const starsCacheTTL = 1 * time.Hour

var httpClient = &http.Client{Timeout: fetchTimeout}

type starsEntry struct {
	stars     int64
	fetchedAt time.Time
}

var (
	starsCacheMu sync.Mutex
	starsCache   = make(map[string]starsEntry)
)

type Stats struct {
	WeeklyDownloads int64
	Stars           int64
	LatestVersion   string
	Err             error
}

func Fetch(ctx context.Context, npmPackage, packagistPackage, githubRepo string, githubToken string) Stats {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	var stats Stats

	switch {
	case npmPackage != "":
		dl, ver, err := fetchNpm(ctx, npmPackage)
		if err != nil {
			stats.Err = err
		} else {
			stats.WeeklyDownloads = dl
			stats.LatestVersion = ver
		}
	case packagistPackage != "":
		dl, ver, err := fetchPackagist(ctx, packagistPackage)
		if err != nil {
			stats.Err = err
		} else {
			stats.WeeklyDownloads = dl
			stats.LatestVersion = ver
		}
	}

	if githubRepo != "" {
		stars, err := fetchGithubStars(ctx, githubRepo, githubToken)
		if err != nil && stats.Err == nil {
			stats.Err = err
		} else {
			stats.Stars = stars
		}
	}

	return stats
}

func fetchNpm(ctx context.Context, pkg string) (int64, string, error) {
	encoded := strings.ReplaceAll(pkg, "/", "%2F")

	dlURL := fmt.Sprintf("https://api.npmjs.org/downloads/point/last-week/%s", encoded)
	var dlResp struct {
		Downloads int64 `json:"downloads"`
	}
	if err := getJSON(ctx, dlURL, &dlResp); err != nil {
		return 0, "", err
	}

	metaURL := fmt.Sprintf("https://registry.npmjs.org/%s/latest", encoded)
	var metaResp struct {
		Version string `json:"version"`
	}
	if err := getJSON(ctx, metaURL, &metaResp); err != nil {
		return dlResp.Downloads, "", err
	}

	return dlResp.Downloads, metaResp.Version, nil
}

func fetchPackagist(ctx context.Context, pkg string) (int64, string, error) {
	url := fmt.Sprintf("https://packagist.org/packages/%s.json", pkg)

	var resp struct {
		Package struct {
			Downloads struct {
				Total int64 `json:"total"`
			} `json:"downloads"`
			Versions map[string]struct {
				Version string `json:"version"`
			} `json:"versions"`
		} `json:"package"`
	}
	if err := getJSON(ctx, url, &resp); err != nil {
		return 0, "", err
	}

	latest := ""
	for v := range resp.Package.Versions {
		if !strings.Contains(v, "dev") && !strings.HasPrefix(v, "v0.") {
			if latest == "" || semver.Greater(v, latest) {
				latest = v
			}
		}
	}

	return resp.Package.Downloads.Total, latest, nil
}

func fetchGithubStars(ctx context.Context, repo string, token string) (int64, error) {
	starsCacheMu.Lock()
	if entry, ok := starsCache[repo]; ok && time.Since(entry.fetchedAt) < starsCacheTTL {
		starsCacheMu.Unlock()
		return entry.stars, nil
	}
	starsCacheMu.Unlock()

	url := fmt.Sprintf("https://api.github.com/repos/%s", repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "kapi-cli")
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	var payload struct {
		Stars int64 `json:"stargazers_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, err
	}

	starsCacheMu.Lock()
	starsCache[repo] = starsEntry{stars: payload.Stars, fetchedAt: time.Now()}
	starsCacheMu.Unlock()

	return payload.Stars, nil
}

func getJSON(ctx context.Context, url string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "kapi-cli")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}
