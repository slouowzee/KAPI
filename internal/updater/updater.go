package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/slouowzee/kapi/internal/semver"
)

const (
	CurrentVersion   = "v1.0.0"
	githubReleaseURL = "https://api.github.com/repos/slouowzee/KAPI/releases/latest"
)

type Release struct {
	TagName string `json:"tag_name"`
}

type UpdateInfo struct {
	Available      bool
	CurrentVersion string
	LatestVersion  string
}

func checkLatestVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubReleaseURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "kapi-updater")

	resp, err := http.DefaultClient.Do(req)
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
	return semver.Greater(latest, current)
}

func Check(ctx context.Context) <-chan UpdateInfo {
	ch := make(chan UpdateInfo, 1)

	go func() {
		latest, err := checkLatestVersion(ctx)
		if err != nil {
			ch <- UpdateInfo{Available: false, CurrentVersion: CurrentVersion}
			return
		}

		ch <- UpdateInfo{
			Available:      isNewer(CurrentVersion, latest),
			CurrentVersion: CurrentVersion,
			LatestVersion:  latest,
		}
	}()

	return ch
}
