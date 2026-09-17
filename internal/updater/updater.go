package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/slouowzee/kapi/internal/semver"
)

const githubReleaseURL = "https://api.github.com/repos/slouowzee/kapi/releases/latest"

// version is injected at release time:
//
//	-ldflags "-X github.com/slouowzee/kapi/internal/updater.version=v1.2.3"
var version string

// CurrentVersion is the running version: the injected one, else the module
// version recorded by `go install`/`go build`, else "dev".
var CurrentVersion = resolveVersion(version, readBuildVersion())

func readBuildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return info.Main.Version
}

func resolveVersion(injected, build string) string {
	if injected != "" {
		return injected
	}
	if build != "" && build != "(devel)" {
		return build
	}
	return "dev"
}

type Release struct {
	TagName string `json:"tag_name"`
}

type UpdateInfo struct {
	Available      bool
	CurrentVersion string
	LatestVersion  string
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

var defaultClient HTTPClient = &http.Client{Timeout: 5 * time.Second}

func checkLatestVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubReleaseURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "kapi-updater")

	resp, err := defaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

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
