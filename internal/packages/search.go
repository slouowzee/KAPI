package packages

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
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

const (
	searchTimeout        = 10 * time.Second
	detailTimeout        = 5 * time.Second
	versionsTimeout      = 30 * time.Second
	searchLimitNpm       = 250
	searchLimitPackagist = 100
	maxConcurrentDetails = 10
)

const (
	npmRegistryURL   = "https://registry.npmjs.org"
	npmDownloadsURL  = "https://api.npmjs.org/downloads/point/last-week"
	packagistURL     = "https://packagist.org"
	packagistRepoURL = "https://repo.packagist.org/p2"
)

// npmAbbreviatedMetadata is the "corgi" document: it only lists what installers
// need and is several times lighter than the full packument.
const npmAbbreviatedMetadata = "application/vnd.npm.install-v1+json"

type Package struct {
	Name          string
	Description   string
	LatestVersion string
	// Versions is only filled by FetchVersions: the version lists of popular
	// packages weigh megabytes, so they are fetched on demand.
	Versions      []string
	PinnedVersion string
	Weekly        int64
	// Stars is filled on demand through Stars to stay within GitHub rate limits.
	Stars      int64
	GithubRepo string
}

var (
	githubRepoRe      = regexp.MustCompile(`github\.com[/:]([^/]+/[^/.\s]+?)(?:\.git)?$`)
	githubShorthandRe = regexp.MustCompile(`^(?:github:)?([\w.-]+/[\w.-]+)$`)
)

func extractGithubRepo(repoURL string) string {
	if m := githubRepoRe.FindStringSubmatch(repoURL); len(m) >= 2 {
		return m[1]
	}
	if m := githubShorthandRe.FindStringSubmatch(repoURL); len(m) >= 2 {
		return m[1]
	}
	return ""
}

// Stars returns the GitHub stars of a repository ("owner/name"), or 0 when
// unknown. Results are cached by the trends package.
func Stars(ctx context.Context, repo string) int64 {
	if repo == "" {
		return 0
	}
	stars, _ := trends.FetchStars(ctx, repo, config.GithubToken())
	return stars
}

func getJSON(ctx context.Context, client *http.Client, endpoint, accept string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "kapi-cli")
	req.Header.Set("Accept", accept)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, endpoint)
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}

// repositoryURL reads the "repository" field of an npm manifest, which is
// either a string or an object with a url.
func repositoryURL(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var obj struct {
		URL string `json:"url"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		return obj.URL
	}
	return ""
}

func SearchNpm(ctx context.Context, query string) ([]Package, error) {
	endpoint := fmt.Sprintf("%s/-/v1/search?text=%s&size=%d", npmRegistryURL, url.QueryEscape(query), searchLimitNpm)

	var payload struct {
		Objects []struct {
			Package struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Version     string `json:"version"`
				Links       struct {
					Repository string `json:"repository"`
				} `json:"links"`
			} `json:"package"`
			Downloads struct {
				Weekly int64 `json:"weekly"`
			} `json:"downloads"`
		} `json:"objects"`
	}
	client := &http.Client{Timeout: searchTimeout}
	if err := getJSON(ctx, client, endpoint, "application/json", &payload); err != nil {
		return nil, fmt.Errorf("npm search: %w", err)
	}

	results := make([]Package, 0, len(payload.Objects))
	for _, o := range payload.Objects {
		results = append(results, Package{
			Name:          o.Package.Name,
			Description:   o.Package.Description,
			LatestVersion: o.Package.Version,
			Weekly:        o.Downloads.Weekly,
			GithubRepo:    extractGithubRepo(o.Package.Links.Repository),
		})
	}
	return results, nil
}

type packagistSearchResult struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Repository  string `json:"repository"`
	Downloads   int64  `json:"downloads"`
}

func searchPackagist(ctx context.Context, client *http.Client, query string, limit int) ([]packagistSearchResult, error) {
	endpoint := fmt.Sprintf("%s/search.json?q=%s&per_page=%d", packagistURL, url.QueryEscape(query), limit)
	var payload struct {
		Results []packagistSearchResult `json:"results"`
	}
	if err := getJSON(ctx, client, endpoint, "application/json", &payload); err != nil {
		return nil, fmt.Errorf("packagist search: %w", err)
	}
	return payload.Results, nil
}

func (r packagistSearchResult) toPackage() Package {
	return Package{
		Name:        r.Name,
		Description: r.Description,
		Weekly:      r.Downloads,
		GithubRepo:  extractGithubRepo(r.Repository),
	}
}

func SearchPackagist(ctx context.Context, query string) ([]Package, error) {
	client := &http.Client{Timeout: searchTimeout}
	found, err := searchPackagist(ctx, client, query, searchLimitPackagist)
	if err != nil {
		return nil, err
	}
	results := make([]Package, 0, len(found))
	for _, r := range found {
		results = append(results, r.toPackage())
	}
	return results, nil
}

// fetchNpmSummary fills a package from its latest manifest (a few kilobytes)
// and its weekly downloads.
func fetchNpmSummary(ctx context.Context, client *http.Client, pkg *Package) {
	var manifest struct {
		Version     string          `json:"version"`
		Description string          `json:"description"`
		Repository  json.RawMessage `json:"repository"`
	}
	if err := getJSON(ctx, client, npmRegistryURL+"/"+url.PathEscape(pkg.Name)+"/latest", "application/json", &manifest); err == nil {
		pkg.LatestVersion = manifest.Version
		if pkg.Description == "" {
			pkg.Description = manifest.Description
		}
		pkg.GithubRepo = extractGithubRepo(repositoryURL(manifest.Repository))
	}

	var downloads struct {
		Downloads int64 `json:"downloads"`
	}
	if err := getJSON(ctx, client, npmDownloadsURL+"/"+url.PathEscape(pkg.Name), "application/json", &downloads); err == nil {
		pkg.Weekly = downloads.Downloads
	}
}

// fetchPackagistSummary fills a package from the packagist search API, whose
// response is tiny compared to the full package document.
func fetchPackagistSummary(ctx context.Context, client *http.Client, pkg *Package) {
	found, err := searchPackagist(ctx, client, pkg.Name, 5)
	if err != nil {
		return
	}
	for _, r := range found {
		if strings.EqualFold(r.Name, pkg.Name) {
			summary := r.toPackage()
			summary.Name = pkg.Name
			*pkg = summary
			return
		}
	}
}

// FetchDefaults returns the named packages with their description, latest
// version and downloads. Packages that cannot be fetched keep only their name.
func FetchDefaults(ctx context.Context, names []string, isPhp bool) []Package {
	pkgs := make([]Package, len(names))
	for i, name := range names {
		pkgs[i] = Package{Name: name}
	}

	client := &http.Client{Timeout: detailTimeout}
	sem := make(chan struct{}, maxConcurrentDetails)
	var wg sync.WaitGroup
	for i := range pkgs {
		wg.Add(1)
		go func(pkg *Package) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			if isPhp {
				fetchPackagistSummary(ctx, client, pkg)
			} else {
				fetchNpmSummary(ctx, client, pkg)
			}
		}(&pkgs[i])
	}
	wg.Wait()
	return pkgs
}

// FetchVersions returns the published versions of a package, newest first.
// Development branches are excluded.
func FetchVersions(ctx context.Context, name string, isPhp bool) ([]string, error) {
	client := &http.Client{Timeout: versionsTimeout}
	var versions []string

	if isPhp {
		var payload struct {
			Packages map[string][]struct {
				Version string `json:"version"`
			} `json:"packages"`
		}
		if err := getJSON(ctx, client, packagistRepoURL+"/"+name+".json", "application/json", &payload); err != nil {
			return nil, fmt.Errorf("fetch %s versions: %w", name, err)
		}
		for _, v := range payload.Packages[name] {
			if v.Version != "" && !strings.HasPrefix(v.Version, "dev-") && !strings.HasSuffix(v.Version, "-dev") {
				versions = append(versions, v.Version)
			}
		}
	} else {
		var payload struct {
			Versions map[string]json.RawMessage `json:"versions"`
		}
		if err := getJSON(ctx, client, npmRegistryURL+"/"+url.PathEscape(name), npmAbbreviatedMetadata, &payload); err != nil {
			return nil, fmt.Errorf("fetch %s versions: %w", name, err)
		}
		for v := range payload.Versions {
			versions = append(versions, v)
		}
	}

	sort.Slice(versions, func(i, j int) bool {
		return semver.Greater(versions[i], versions[j])
	})
	return versions, nil
}
