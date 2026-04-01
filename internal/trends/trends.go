package trends

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

func semverGreater(a, b string) bool {
	partsA := strings.Split(strings.TrimPrefix(a, "v"), ".")
	partsB := strings.Split(strings.TrimPrefix(b, "v"), ".")
	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}
	for i := 0; i < maxLen; i++ {
		var sa, sb string
		if i < len(partsA) {
			sa = partsA[i]
		}
		if i < len(partsB) {
			sb = partsB[i]
		}
		na, errA := strconv.Atoi(sa)
		nb, errB := strconv.Atoi(sb)
		if errA == nil && errB == nil {
			if na != nb {
				return na > nb
			}
		} else if sa != sb {
			return sa > sb
		}
	}
	return false
}

const FETCH_TIMEOUT = 5 * time.Second
const STARS_CACHE_TTL = 1 * time.Hour

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

func Fetch(npmPackage, packagistPackage, githubRepo string, githubToken string) Stats {
	client := &http.Client{Timeout: FETCH_TIMEOUT}

	var stats Stats

	switch {
	case npmPackage != "":
		dl, ver, err := fetchNpm(client, npmPackage)
		if err != nil {
			stats.Err = err
		} else {
			stats.WeeklyDownloads = dl
			stats.LatestVersion = ver
		}
	case packagistPackage != "":
		dl, ver, err := fetchPackagist(client, packagistPackage)
		if err != nil {
			stats.Err = err
		} else {
			stats.WeeklyDownloads = dl
			stats.LatestVersion = ver
		}
	}

	if githubRepo != "" {
		stars, err := fetchGithubStars(client, githubRepo, githubToken)
		if err != nil && stats.Err == nil {
			stats.Err = err
		} else {
			stats.Stars = stars
		}
	}

	return stats
}

func fetchNpm(client *http.Client, pkg string) (int64, string, error) {
	encoded := strings.ReplaceAll(pkg, "/", "%2F")

	dlURL := fmt.Sprintf("https://api.npmjs.org/downloads/point/last-week/%s", encoded)
	var dlResp struct {
		Downloads int64 `json:"downloads"`
	}
	if err := getJSON(client, dlURL, &dlResp); err != nil {
		return 0, "", err
	}

	metaURL := fmt.Sprintf("https://registry.npmjs.org/%s/latest", encoded)
	var metaResp struct {
		Version string `json:"version"`
	}
	if err := getJSON(client, metaURL, &metaResp); err != nil {
		return dlResp.Downloads, "", err
	}

	return dlResp.Downloads, metaResp.Version, nil
}

func fetchPackagist(client *http.Client, pkg string) (int64, string, error) {
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
	if err := getJSON(client, url, &resp); err != nil {
		return 0, "", err
	}

	latest := ""
	for v := range resp.Package.Versions {
		if !strings.Contains(v, "dev") && !strings.HasPrefix(v, "v0.") {
			if latest == "" || semverGreater(v, latest) {
				latest = v
			}
		}
	}

	return resp.Package.Downloads.Total, latest, nil
}

func fetchGithubStars(client *http.Client, repo string, token string) (int64, error) {
	starsCacheMu.Lock()
	if entry, ok := starsCache[repo]; ok && time.Since(entry.fetchedAt) < STARS_CACHE_TTL {
		starsCacheMu.Unlock()
		return entry.stars, nil
	}
	starsCacheMu.Unlock()

	url := fmt.Sprintf("https://api.github.com/repos/%s", repo)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "kapi-cli")
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
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

func getJSON(client *http.Client, url string, dest any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "kapi-cli")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}
