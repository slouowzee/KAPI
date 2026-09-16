package packages

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slouowzee/kapi/internal/testutil"
)

func redirectTo(t *testing.T, ts *httptest.Server) {
	t.Helper()
	orig := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = orig })
	inner := orig
	http.DefaultTransport = testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		req2 := req.Clone(req.Context())
		req2.URL.Scheme = "http"
		req2.URL.Host = ts.Listener.Addr().String()
		return inner.RoundTrip(req2)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// unexpectedRequests fails the test when a request reaches a path that was not
// registered, which is how heavy per-package lookups are detected.
func unexpectedRequests(t *testing.T, mux *http.ServeMux) *atomic.Int32 {
	t.Helper()
	var count atomic.Int32
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		t.Errorf("unexpected request to %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	})
	return &count
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
		{"github:owner/repo", "owner/repo"},
		{"owner/repo", "owner/repo"},
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

func TestStars(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GITHUB_TOKEN", "test-token")
	const repo = "test-owner/repo-stars-packages"

	var calls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/"+repo, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		writeJSON(w, map[string]any{"stargazers_count": 42})
	})
	mux.HandleFunc("/repos/test-owner/missing", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	redirectTo(t, ts)

	if got := Stars(context.Background(), repo); got != 42 {
		t.Errorf("Stars = %d, want 42", got)
	}
	if got := Stars(context.Background(), repo); got != 42 || calls.Load() != 1 {
		t.Errorf("cached Stars = %d with %d calls, want 42 with 1 call", got, calls.Load())
	}
	if got := Stars(context.Background(), ""); got != 0 {
		t.Errorf("Stars('') = %d, want 0", got)
	}
	if got := Stars(context.Background(), "test-owner/missing"); got != 0 {
		t.Errorf("Stars(404) = %d, want 0", got)
	}
}

func TestSearchNpm_UsesSearchPayloadOnly(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/-/v1/search", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"objects": []map[string]any{{
				"package": map[string]any{
					"name":        "my-pkg",
					"description": "desc from search",
					"version":     "3.0.0",
					"links":       map[string]any{"repository": "git+https://github.com/owner/my-pkg.git"},
				},
				"downloads": map[string]any{"weekly": int64(100)},
			}},
		})
	})
	unexpectedRequests(t, mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()
	redirectTo(t, ts)

	pkgs, err := SearchNpm(context.Background(), "my-pkg")
	if err != nil {
		t.Fatalf("SearchNpm error: %v", err)
	}
	want := Package{Name: "my-pkg", Description: "desc from search", LatestVersion: "3.0.0", Weekly: 100, GithubRepo: "owner/my-pkg"}
	if len(pkgs) != 1 || !packageEqual(pkgs[0], want) {
		t.Errorf("SearchNpm = %+v, want [%+v]", pkgs, want)
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

func TestSearchPackagist_UsesSearchPayloadOnly(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/search.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"results": []map[string]any{
				{"name": "vendor/pkg", "description": "php pkg", "repository": "https://github.com/vendor/pkg", "downloads": int64(300)},
			},
		})
	})
	unexpectedRequests(t, mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()
	redirectTo(t, ts)

	pkgs, err := SearchPackagist(context.Background(), "pkg")
	if err != nil {
		t.Fatalf("SearchPackagist error: %v", err)
	}
	want := Package{Name: "vendor/pkg", Description: "php pkg", Weekly: 300, GithubRepo: "vendor/pkg"}
	if len(pkgs) != 1 || !packageEqual(pkgs[0], want) {
		t.Errorf("SearchPackagist = %+v, want [%+v]", pkgs, want)
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

func TestFetchDefaults(t *testing.T) {
	mux := http.NewServeMux()
	// NOTE: scoped names are escaped ("@scope%2Fname"), so they are matched as
	// a single path segment.
	mux.HandleFunc("/{pkg}/latest", func(w http.ResponseWriter, r *http.Request) {
		switch r.PathValue("pkg") {
		case "@scope/npm-pkg":
			writeJSON(w, map[string]any{
				"version":     "1.2.3",
				"description": "An npm package",
				"repository":  map[string]string{"type": "git", "url": "git+https://github.com/owner/npm-pkg.git"},
			})
		case "string-repo":
			writeJSON(w, map[string]any{"version": "0.1.0", "repository": "github:owner/string-repo"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	mux.HandleFunc("/downloads/point/last-week/{pkg}", func(w http.ResponseWriter, r *http.Request) {
		downloads := map[string]int64{"@scope/npm-pkg": 5000, "string-repo": 1}
		writeJSON(w, map[string]any{"downloads": downloads[r.PathValue("pkg")]})
	})
	mux.HandleFunc("/search.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"results": []map[string]any{
				{"name": "vendor/pkg-extra", "description": "not this one"},
				{"name": "vendor/pkg", "description": "A packagist package", "repository": "https://github.com/vendor/pkg", "downloads": int64(8000)},
			},
		})
	})
	unexpectedRequests(t, mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()
	redirectTo(t, ts)

	tests := []struct {
		name  string
		names []string
		isPhp bool
		want  []Package
	}{
		{
			name:  "npm",
			names: []string{"@scope/npm-pkg", "string-repo"},
			want: []Package{
				{Name: "@scope/npm-pkg", Description: "An npm package", LatestVersion: "1.2.3", Weekly: 5000, GithubRepo: "owner/npm-pkg"},
				{Name: "string-repo", LatestVersion: "0.1.0", Weekly: 1, GithubRepo: "owner/string-repo"},
			},
		},
		{
			name:  "packagist",
			names: []string{"vendor/pkg"},
			isPhp: true,
			want:  []Package{{Name: "vendor/pkg", Description: "A packagist package", Weekly: 8000, GithubRepo: "vendor/pkg"}},
		},
		{name: "empty", names: []string{}, want: []Package{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FetchDefaults(context.Background(), tt.names, tt.isPhp)
			if !slices.EqualFunc(got, tt.want, packageEqual) {
				t.Errorf("FetchDefaults = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestFetchDefaults_FailureKeepsName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	redirectTo(t, ts)

	got := FetchDefaults(context.Background(), []string{"broken"}, false)
	if len(got) != 1 || !packageEqual(got[0], Package{Name: "broken"}) {
		t.Errorf("FetchDefaults = %+v, want only the name", got)
	}
}

func TestFetchVersions(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/npm-pkg", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); got != npmAbbreviatedMetadata {
			t.Errorf("Accept = %q, want abbreviated metadata", got)
		}
		writeJSON(w, map[string]any{
			"versions": map[string]any{"1.0.0": map[string]any{}, "2.0.0-beta.1": map[string]any{}, "2.0.0": map[string]any{}},
		})
	})
	mux.HandleFunc("/p2/vendor/pkg.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"packages": map[string]any{
				"vendor/pkg": []map[string]string{{"version": "v1.0.0"}, {"version": "dev-main"}, {"version": "v2.0.0"}, {"version": "2.1.x-dev"}},
			},
		})
	})
	mux.HandleFunc("/missing", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	redirectTo(t, ts)

	tests := []struct {
		name    string
		pkg     string
		isPhp   bool
		want    []string
		wantErr bool
	}{
		{name: "npm sorted newest first", pkg: "npm-pkg", want: []string{"2.0.0", "2.0.0-beta.1", "1.0.0"}},
		{name: "packagist without dev branches", pkg: "vendor/pkg", isPhp: true, want: []string{"v2.0.0", "v1.0.0"}},
		{name: "not found", pkg: "missing", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FetchVersions(context.Background(), tt.pkg, tt.isPhp)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("FetchVersions = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFetchDefaults_RespectsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	got := FetchDefaults(ctx, []string{"a", "b", "c"}, false)
	if len(got) != 3 || time.Since(start) > time.Second {
		t.Errorf("cancelled FetchDefaults took %v and returned %d packages", time.Since(start), len(got))
	}
}

func packageEqual(a, b Package) bool {
	return a.Name == b.Name && a.Description == b.Description && a.LatestVersion == b.LatestVersion &&
		a.Weekly == b.Weekly && a.GithubRepo == b.GithubRepo && a.Stars == b.Stars &&
		a.PinnedVersion == b.PinnedVersion && slices.Equal(a.Versions, b.Versions)
}
