package trends

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

func TestGetJSON_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": 42})
	}))
	defer ts.Close()
	redirectTo(t, ts)

	var result struct {
		Value int `json:"value"`
	}
	err := getJSON(context.Background(), "https://example.com/test", &result)
	if err != nil {
		t.Fatalf("getJSON error: %v", err)
	}
	if result.Value != 42 {
		t.Errorf("Value = %d, want 42", result.Value)
	}
}

func TestGetJSON_NonOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()
	redirectTo(t, ts)

	var result struct{}
	err := getJSON(context.Background(), "https://example.com/missing", &result)
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error %q should mention 404", err.Error())
	}
}

func TestGetJSON_BadJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-json{{"))
	}))
	defer ts.Close()
	redirectTo(t, ts)

	var result struct{ Value int }
	err := getJSON(context.Background(), "https://example.com/bad", &result)
	if err == nil {
		t.Fatal("expected JSON decode error, got nil")
	}
}

func TestFetchNpm_Success(t *testing.T) {
	const pkg = "test-fetchnpm-success"

	mux := http.NewServeMux()
	mux.HandleFunc("/downloads/point/last-week/"+pkg, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"downloads": int64(1234)})
	})
	mux.HandleFunc("/"+pkg+"/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"version": "2.3.4"})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()
	redirectTo(t, ts)

	dl, ver, err := fetchNpm(context.Background(), pkg)
	if err != nil {
		t.Fatalf("fetchNpm error: %v", err)
	}
	if dl != 1234 {
		t.Errorf("downloads = %d, want 1234", dl)
	}
	if ver != "2.3.4" {
		t.Errorf("version = %q, want 2.3.4", ver)
	}
}

func TestFetchNpm_DownloadError(t *testing.T) {
	const pkg = "test-fetchnpm-dlerr"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	redirectTo(t, ts)

	_, _, err := fetchNpm(context.Background(), pkg)
	if err == nil {
		t.Fatal("expected error from download endpoint, got nil")
	}
}

func TestFetchNpm_MetaError(t *testing.T) {
	const pkg = "test-fetchnpm-metaerr"

	mux := http.NewServeMux()
	mux.HandleFunc("/downloads/point/last-week/"+pkg, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"downloads": int64(50)})
	})
	mux.HandleFunc("/"+pkg+"/latest", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()
	redirectTo(t, ts)

	dl, _, err := fetchNpm(context.Background(), pkg)
	if err == nil {
		t.Fatal("expected error from meta endpoint, got nil")
	}
	if dl != 50 {
		t.Errorf("downloads = %d, want 50 on meta error", dl)
	}
}

func TestFetchPackagist_Success(t *testing.T) {
	const pkg = "test-vendor/fetchpackagist-success"

	mux := http.NewServeMux()
	mux.HandleFunc("/packages/"+pkg+".json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"package": map[string]any{
				"downloads": map[string]any{"total": int64(9999)},
				"versions": map[string]any{
					"v3.0.0":   map[string]string{"version": "v3.0.0"},
					"v2.0.0":   map[string]string{"version": "v2.0.0"},
					"dev-main": map[string]string{"version": "dev-main"},
				},
			},
		})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()
	redirectTo(t, ts)

	dl, ver, err := fetchPackagist(context.Background(), pkg)
	if err != nil {
		t.Fatalf("fetchPackagist error: %v", err)
	}
	if dl != 9999 {
		t.Errorf("total = %d, want 9999", dl)
	}
	if ver != "v3.0.0" {
		t.Errorf("version = %q, want v3.0.0", ver)
	}
}

func TestFetchPackagist_Error(t *testing.T) {
	const pkg = "test-vendor/fetchpackagist-err"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer ts.Close()
	redirectTo(t, ts)

	_, _, err := fetchPackagist(context.Background(), pkg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchGithubStars_Success(t *testing.T) {
	const repo = "test-owner/fetchgithubstars-success"

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/"+repo, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"stargazers_count": int64(777)})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()
	redirectTo(t, ts)

	stars, err := fetchGithubStars(context.Background(), repo, "")
	if err != nil {
		t.Fatalf("fetchGithubStars error: %v", err)
	}
	if stars != 777 {
		t.Errorf("stars = %d, want 777", stars)
	}
}

func TestFetchGithubStars_CacheHit(t *testing.T) {
	const repo = "test-owner/fetchgithubstars-cache"

	calls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/"+repo, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"stargazers_count": int64(1)})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	redirectTo(t, ts)

	starsCacheMu.Lock()
	starsCache[repo] = starsEntry{stars: 42, fetchedAt: time.Now()}
	starsCacheMu.Unlock()

	stars, err := fetchGithubStars(context.Background(), repo, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stars != 42 {
		t.Errorf("stars = %d, want 42 (from cache)", stars)
	}
	if calls != 0 {
		t.Errorf("expected 0 HTTP calls, got %d", calls)
	}
}

func TestFetchGithubStars_NonOK(t *testing.T) {
	const repo = "test-owner/fetchgithubstars-nonok"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()
	redirectTo(t, ts)

	_, err := fetchGithubStars(context.Background(), repo, "")
	if err == nil {
		t.Fatal("expected error for 403, got nil")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error %q should mention 403", err.Error())
	}
}

func TestFetch_Npm(t *testing.T) {
	const pkg = "test-fetch-npm-integration"
	const repo = "test-owner/repo-fetch-npm"

	mux := http.NewServeMux()
	mux.HandleFunc("/downloads/point/last-week/"+pkg, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"downloads": int64(500)})
	})
	mux.HandleFunc("/"+pkg+"/latest", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"version": "1.0.0"})
	})
	mux.HandleFunc("/repos/"+repo, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"stargazers_count": int64(88)})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()
	redirectTo(t, ts)

	stats := Fetch(context.Background(), pkg, "", repo, "")
	if stats.Err != nil {
		t.Fatalf("Fetch error: %v", stats.Err)
	}
	if stats.WeeklyDownloads != 500 {
		t.Errorf("WeeklyDownloads = %d, want 500", stats.WeeklyDownloads)
	}
	if stats.LatestVersion != "1.0.0" {
		t.Errorf("LatestVersion = %q, want 1.0.0", stats.LatestVersion)
	}
	if stats.Stars != 88 {
		t.Errorf("Stars = %d, want 88", stats.Stars)
	}
}

func TestFetch_Packagist(t *testing.T) {
	const pkg = "test-vendor/fetch-packagist-integration"
	const repo = "test-owner/repo-fetch-packagist"

	mux := http.NewServeMux()
	mux.HandleFunc("/packages/"+pkg+".json", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"package": map[string]any{
				"downloads": map[string]any{"total": int64(400)},
				"versions":  map[string]any{"v1.1.0": map[string]string{"version": "v1.1.0"}},
			},
		})
	})
	mux.HandleFunc("/repos/"+repo, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"stargazers_count": int64(30)})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()
	redirectTo(t, ts)

	stats := Fetch(context.Background(), "", pkg, repo, "")
	if stats.Err != nil {
		t.Fatalf("Fetch error: %v", stats.Err)
	}
	if stats.WeeklyDownloads != 400 {
		t.Errorf("WeeklyDownloads = %d, want 400", stats.WeeklyDownloads)
	}
	if stats.LatestVersion != "v1.1.0" {
		t.Errorf("LatestVersion = %q, want v1.1.0", stats.LatestVersion)
	}
	if stats.Stars != 30 {
		t.Errorf("Stars = %d, want 30", stats.Stars)
	}
}

func TestFetch_NoPackage_OnlyRepo(t *testing.T) {
	const repo = "test-owner/repo-fetch-onlyrepo"

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/"+repo, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"stargazers_count": int64(15)})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()
	redirectTo(t, ts)

	stats := Fetch(context.Background(), "", "", repo, "")
	if stats.Err != nil {
		t.Fatalf("Fetch error: %v", stats.Err)
	}
	if stats.Stars != 15 {
		t.Errorf("Stars = %d, want 15", stats.Stars)
	}
	if stats.WeeklyDownloads != 0 {
		t.Errorf("WeeklyDownloads = %d, want 0 (no package)", stats.WeeklyDownloads)
	}
}
