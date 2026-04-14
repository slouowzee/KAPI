package packages

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/slouowzee/kapi/internal/config"
	"github.com/slouowzee/kapi/internal/semver"
	"github.com/slouowzee/kapi/internal/trends"
)

//go:embed defaults.json
var defaultsJSON []byte

var DefaultsByFramework map[string][]string

func init() {
	var payload struct {
		Defaults map[string][]string `json:"defaults"`
	}
	if err := json.Unmarshal(defaultsJSON, &payload); err != nil {
		panic("kapi: failed to parse embedded defaults.json: " + err.Error())
	}
	DefaultsByFramework = payload.Defaults
}

const searchTimeout = 10 * time.Second
const detailTimeout = 4 * time.Second
const searchLimitNpm = 250
const searchLimitPackagist = 100

type Package struct {
	Name        string
	Description string
	Version     string
	Weekly      int64
	Stars       int64
	GithubRepo  string
}

var githubRepoRe = regexp.MustCompile(`github\.com[/:]([^/]+/[^/.\s]+?)(?:\.git)?$`)

func extractGithubRepo(repoURL string) string {
	m := githubRepoRe.FindStringSubmatch(repoURL)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func fetchStars(ctx context.Context, repo string) int64 {
	if repo == "" {
		return 0
	}
	tok := config.GithubToken()
	stars, _ := trends.FetchStars(ctx, repo, tok)
	return stars
}

func enrichNpm(ctx context.Context, client *http.Client, pkg *Package) {
	encoded := url.PathEscape(pkg.Name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://registry.npmjs.org/"+encoded+"/latest", nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "kapi-cli")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return
	}
	defer resp.Body.Close()

	var meta struct {
		Version     string `json:"version"`
		Description string `json:"description"`
		Repository  struct {
			URL string `json:"url"`
		} `json:"repository"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return
	}

	if pkg.Version == "" {
		pkg.Version = meta.Version
	}
	if pkg.Description == "" {
		pkg.Description = meta.Description
	}
	repo := extractGithubRepo(meta.Repository.URL)
	pkg.GithubRepo = repo
	pkg.Stars = fetchStars(ctx, repo)

	if pkg.Weekly == 0 {
		dlReq, err := http.NewRequestWithContext(ctx, http.MethodGet,
			"https://api.npmjs.org/downloads/point/last-week/"+url.PathEscape(pkg.Name), nil)
		if err == nil {
			dlReq.Header.Set("User-Agent", "kapi-cli")
			dlResp, err := client.Do(dlReq)
			if err == nil {
				if dlResp.StatusCode == http.StatusOK {
					var dl struct {
						Downloads int64 `json:"downloads"`
					}
					if json.NewDecoder(dlResp.Body).Decode(&dl) == nil {
						pkg.Weekly = dl.Downloads
					}
				}
				_ = dlResp.Body.Close()
			}
		}
	}
}

func enrichPackagist(ctx context.Context, client *http.Client, pkg *Package) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://packagist.org/packages/"+pkg.Name+".json", nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "kapi-cli")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return
	}
	defer resp.Body.Close()

	var meta struct {
		Package struct {
			Repository  string `json:"repository"`
			Description string `json:"description"`
			Downloads   struct {
				Total int64 `json:"total"`
			} `json:"downloads"`
			Versions map[string]struct {
				Version string `json:"version"`
			} `json:"versions"`
		} `json:"package"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return
	}

	for v := range meta.Package.Versions {
		if !strings.Contains(v, "dev") && !strings.HasPrefix(v, "v0.") {
			if pkg.Version == "" || semver.Greater(v, pkg.Version) {
				pkg.Version = v
			}
		}
	}
	if pkg.Description == "" {
		pkg.Description = meta.Package.Description
	}
	if pkg.Weekly == 0 {
		pkg.Weekly = meta.Package.Downloads.Total
	}
	repo := extractGithubRepo(meta.Package.Repository)
	pkg.GithubRepo = repo
	pkg.Stars = fetchStars(ctx, repo)
}

func enrichAll(ctx context.Context, pkgs []Package, isPhp bool) []Package {
	const maxConcurrent = 20
	sem := make(chan struct{}, maxConcurrent)

	client := &http.Client{Timeout: detailTimeout}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for i := range pkgs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			if isPhp {
				enrichPackagist(ctx, client, &pkgs[i])
			} else {
				enrichNpm(ctx, client, &pkgs[i])
			}
		}(i)
	}
	wg.Wait()
	return pkgs
}

func SearchNpm(ctx context.Context, query string) ([]Package, error) {
	endpoint := fmt.Sprintf(
		"https://registry.npmjs.org/-/v1/search?text=%s&size=%d",
		url.QueryEscape(query),
		searchLimitNpm,
	)

	client := &http.Client{Timeout: searchTimeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "kapi-cli")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("npm search: HTTP %d", resp.StatusCode)
	}

	var payload struct {
		Objects []struct {
			Package struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Version     string `json:"version"`
			} `json:"package"`
			Downloads struct {
				Weekly int64 `json:"weekly"`
			} `json:"downloads"`
		} `json:"objects"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	results := make([]Package, 0, len(payload.Objects))
	for _, o := range payload.Objects {
		results = append(results, Package{
			Name:        o.Package.Name,
			Description: o.Package.Description,
			Version:     o.Package.Version,
			Weekly:      o.Downloads.Weekly,
		})
	}

	return enrichAll(ctx, results, false), nil
}

func SearchPackagist(ctx context.Context, query string) ([]Package, error) {
	endpoint := fmt.Sprintf(
		"https://packagist.org/search.json?q=%s&per_page=%d",
		url.QueryEscape(query),
		searchLimitPackagist,
	)

	client := &http.Client{Timeout: searchTimeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "kapi-cli")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("packagist search: HTTP %d", resp.StatusCode)
	}

	var payload struct {
		Results []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Downloads   int64  `json:"downloads"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	results := make([]Package, 0, len(payload.Results))
	for _, r := range payload.Results {
		results = append(results, Package{
			Name:        r.Name,
			Description: r.Description,
			Weekly:      r.Downloads,
		})
	}

	return enrichAll(ctx, results, true), nil
}

func FetchDefaults(ctx context.Context, names []string, isPhp bool) []Package {
	pkgs := make([]Package, len(names))
	for i, name := range names {
		pkgs[i] = Package{Name: name}
	}
	return enrichAll(ctx, pkgs, isPhp)
}
