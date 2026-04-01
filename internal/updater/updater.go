package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	CURRENT_VERSION    = "v1.0.0"
	GITHUB_RELEASE_URL = "https://api.github.com/repos/slouowzee/KAPI/releases/latest"
)

type Release struct {
	TagName string `json:"tag_name"`
}

type UpdateInfo struct {
	Available      bool
	CurrentVersion string
	LatestVersion  string
}

func checkLatestVersion() (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}

	req, err := http.NewRequest("GET", GITHUB_RELEASE_URL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "kapi-updater")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	return release.TagName, nil
}

func isNewer(current, latest string) bool {
	return strings.TrimPrefix(latest, "v") != strings.TrimPrefix(current, "v") &&
		latest > current
}

func Check() <-chan UpdateInfo {
	ch := make(chan UpdateInfo, 1)

	go func() {
		latest, err := checkLatestVersion()
		if err != nil {
			ch <- UpdateInfo{Available: false, CurrentVersion: CURRENT_VERSION}
			return
		}

		ch <- UpdateInfo{
			Available:      isNewer(CURRENT_VERSION, latest),
			CurrentVersion: CURRENT_VERSION,
			LatestVersion:  latest,
		}
	}()

	return ch
}
