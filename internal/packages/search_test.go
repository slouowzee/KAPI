package packages

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func redirectTo(t *testing.T, ts *httptest.Server) {
	t.Helper()
	orig := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = orig })
	inner := orig
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		req2 := req.Clone(req.Context())
		req2.URL.Scheme = "http"
		req2.URL.Host = ts.Listener.Addr().String()
		return inner.RoundTrip(req2)
	})
}

func TestExtractGithubRepo(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"https://github.com/owner/repo", "owner/repo"},
		{"https://github.com/owner/repo.git", "owner/repo"},
		{"git+https://github.com/owner/repo.git", "owner/repo"},
		{"git://github.com/owner/repo", "owner/repo"},
		{"git@github.com:owner/repo.git", "owner/repo"},
		{"https://gitlab.com/owner/repo", ""},
		{"", ""},
		{"not-a-url", ""},
	}
	for _, tc := range cases {
		got := extractGithubRepo(tc.input)
		if got != tc.want {
			t.Errorf("extractGithubRepo(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestFetchStars_Success(t *testing.T) {
	const repo = "test-owner/repo-fetchstars-success"

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/"+repo, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"stargazers_count": 42})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	redirectTo(t, ts)

	got := fetchStars(context.Background(), repo)
	if got != 42 {
		t.Errorf("fetchStars = %d, want 42", got)
	}
}

func TestFetchStars_CacheHit(t *testing.T) {
	const repo = "test-owner/repo-fetchstars-cache"

	calls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/"+repo, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"stargazers_count": 99})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	redirectTo(t, ts)

	starsLocalMu.Lock()
	starsLocalCache[repo] = starsLocalEntry{stars: 77, fetchedAt: time.Now()}
	starsLocalMu.Unlock()

	got := fetchStars(context.Background(), repo)
	if got != 77 {
		t.Errorf("fetchStars (cache) = %d, want 77", got)
	}
	if calls != 0 {
		t.Errorf("expected 0 HTTP calls, got %d", calls)
	}
}

func TestFetchStars_Empty(t *testing.T) {
	got := fetchStars(context.Background(), "")
	if got != 0 {
		t.Errorf("fetchStars('') = %d, want 0", got)
	}
}

func TestFetchStars_NonOK(t *testing.T) {
	const repo = "test-owner/repo-fetchstars-nonok"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()
	redirectTo(t, ts)

	got := fetchStars(context.Background(), repo)
	if got != 0 {
		t.Errorf("fetchStars (404) = %d, want 0", got)
	}
}

func TestEnrichNpm(t *testing.T) {
	const pkgName = "test-enrich-npm-unique"
	const repoSlug = "test-owner/repo-enrichnpm"

	mux := http.NewServeMux()

	mux.HandleFunc("/"+pkgName+"/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"version":     "1.2.3",
			"description": "A test package",
			"repository":  map[string]string{"url": "https://github.com/" + repoSlug},
		})
	})

	mux.HandleFunc("/downloads/point/last-week/"+pkgName, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"downloads": int64(5000)})
	})

	mux.HandleFunc("/repos/"+repoSlug, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"stargazers_count": int64(123)})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()
	redirectTo(t, ts)

	client := &http.Client{Timeout: 4 * time.Second}
	pkg := &Package{Name: pkgName}
	enrichNpm(context.Background(), client, pkg)

	if pkg.Version != "1.2.3" {
		t.Errorf("Version = %q, want 1.2.3", pkg.Version)
	}
	if pkg.Description != "A test package" {
		t.Errorf("Description = %q, want 'A test package'", pkg.Description)
	}
	if pkg.Weekly != 5000 {
		t.Errorf("Weekly = %d, want 5000", pkg.Weekly)
	}
	if pkg.Stars != 123 {
		t.Errorf("Stars = %d, want 123", pkg.Stars)
	}
	if pkg.GithubRepo != repoSlug {
		t.Errorf("GithubRepo = %q, want %q", pkg.GithubRepo, repoSlug)
	}
}

func TestEnrichNpm_NonOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	redirectTo(t, ts)

	client := &http.Client{Timeout: 4 * time.Second}
	pkg := &Package{Name: "some-pkg-nonok"}
	enrichNpm(context.Background(), client, pkg)

	if pkg.Version != "" || pkg.Stars != 0 {
		t.Errorf("expected empty enrichment on error, got Version=%q Stars=%d", pkg.Version, pkg.Stars)
	}
}

func TestEnrichPackagist(t *testing.T) {
	const pkgName = "test-vendor/enrichpackagist-unique"
	const repoSlug = "test-owner/repo-enrichpackagist"

	mux := http.NewServeMux()

	mux.HandleFunc("/packages/"+pkgName+".json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"package": map[string]any{
				"repository":  "https://github.com/" + repoSlug,
				"description": "A packagist package",
				"downloads":   map[string]any{"total": int64(8000)},
				"versions": map[string]any{
					"v1.0.0":   map[string]string{"version": "v1.0.0"},
					"v2.0.0":   map[string]string{"version": "v2.0.0"},
					"dev-main": map[string]string{"version": "dev-main"},
				},
			},
		})
	})

	mux.HandleFunc("/repos/"+repoSlug, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"stargazers_count": int64(200)})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()
	redirectTo(t, ts)

	client := &http.Client{Timeout: 4 * time.Second}
	pkg := &Package{Name: pkgName}
	enrichPackagist(context.Background(), client, pkg)

	if pkg.Description != "A packagist package" {
		t.Errorf("Description = %q, want 'A packagist package'", pkg.Description)
	}
	if pkg.Weekly != 8000 {
		t.Errorf("Weekly = %d, want 8000", pkg.Weekly)
	}
	if pkg.Stars != 200 {
		t.Errorf("Stars = %d, want 200", pkg.Stars)
	}
	if pkg.GithubRepo != repoSlug {
		t.Errorf("GithubRepo = %q, want %q", pkg.GithubRepo, repoSlug)
	}
	if pkg.Version != "v2.0.0" {
		t.Errorf("Version = %q, want v2.0.0", pkg.Version)
	}
}

func TestSearchNpm(t *testing.T) {
	const pkgName = "my-test-pkg-searchnpm"
	const repoSlug = "test-owner/repo-searchnpm"

	mux := http.NewServeMux()

	mux.HandleFunc("/-/v1/search", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"objects": []map[string]any{
				{
					"package": map[string]any{
						"name":        pkgName,
						"description": "desc from search",
						"version":     "3.0.0",
					},
					"downloads": map[string]any{"weekly": int64(100)},
				},
			},
		})
	})

	mux.HandleFunc("/"+pkgName+"/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"version":    "3.0.0",
			"repository": map[string]string{"url": "https://github.com/" + repoSlug},
		})
	})

	mux.HandleFunc("/downloads/point/last-week/"+pkgName, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"downloads": int64(999)})
	})

	mux.HandleFunc("/repos/"+repoSlug, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"stargazers_count": int64(55)})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()
	redirectTo(t, ts)

	pkgs, err := SearchNpm(context.Background(), pkgName)
	if err != nil {
		t.Fatalf("SearchNpm error: %v", err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 result, got %d", len(pkgs))
	}
	p := pkgs[0]
	if p.Name != pkgName {
		t.Errorf("Name = %q, want %q", p.Name, pkgName)
	}
	if p.Version != "3.0.0" {
		t.Errorf("Version = %q, want 3.0.0", p.Version)
	}
	if p.Stars != 55 {
		t.Errorf("Stars = %d, want 55", p.Stars)
	}
}

func TestSearchNpm_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()
	redirectTo(t, ts)

	_, err := SearchNpm(context.Background(), "anything")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error %q should mention HTTP 503", err.Error())
	}
}

func TestSearchPackagist(t *testing.T) {
	const pkgName = "test-vendor/searchpackagist-pkg"
	const repoSlug = "test-owner/repo-searchpackagist"

	mux := http.NewServeMux()

	mux.HandleFunc("/search.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"name": pkgName, "description": "php pkg", "downloads": int64(300)},
			},
		})
	})

	mux.HandleFunc("/packages/"+pkgName+".json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"package": map[string]any{
				"repository":  "https://github.com/" + repoSlug,
				"description": "php pkg",
				"downloads":   map[string]any{"total": int64(300)},
				"versions":    map[string]any{"v1.5.0": map[string]string{"version": "v1.5.0"}},
			},
		})
	})

	mux.HandleFunc("/repos/"+repoSlug, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"stargazers_count": int64(10)})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()
	redirectTo(t, ts)

	pkgs, err := SearchPackagist(context.Background(), "searchpackagist")
	if err != nil {
		t.Fatalf("SearchPackagist error: %v", err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 result, got %d", len(pkgs))
	}
	p := pkgs[0]
	if p.Name != pkgName {
		t.Errorf("Name = %q, want %q", p.Name, pkgName)
	}
	if p.Stars != 10 {
		t.Errorf("Stars = %d, want 10", p.Stars)
	}
}

func TestSearchPackagist_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer ts.Close()
	redirectTo(t, ts)

	_, err := SearchPackagist(context.Background(), "anything")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Errorf("error %q should mention HTTP 502", err.Error())
	}
}

func TestFetchDefaults_Npm(t *testing.T) {
	const pkgName = "test-fetchdefaults-npm"
	const repoSlug = "test-owner/repo-fetchdefaults-npm"

	mux := http.NewServeMux()
	mux.HandleFunc("/"+pkgName+"/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"version":    "0.1.0",
			"repository": map[string]string{"url": "https://github.com/" + repoSlug},
		})
	})
	mux.HandleFunc("/downloads/point/last-week/"+pkgName, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"downloads": int64(20)})
	})
	mux.HandleFunc("/repos/"+repoSlug, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"stargazers_count": int64(7)})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()
	redirectTo(t, ts)

	pkgs := FetchDefaults(context.Background(), []string{pkgName}, false)
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 result, got %d", len(pkgs))
	}
	if pkgs[0].Name != pkgName {
		t.Errorf("Name = %q, want %q", pkgs[0].Name, pkgName)
	}
	if pkgs[0].Version != "0.1.0" {
		t.Errorf("Version = %q, want 0.1.0", pkgs[0].Version)
	}
}

func TestFetchDefaults_Empty(t *testing.T) {
	pkgs := FetchDefaults(context.Background(), []string{}, false)
	if len(pkgs) != 0 {
		t.Errorf("expected 0 results, got %d", len(pkgs))
	}
}
