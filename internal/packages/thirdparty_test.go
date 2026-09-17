package packages

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestFetchDrupalOrgSummary(t *testing.T) {
	tests := []struct {
		name        string
		pkgName     string
		response    map[string]any
		wantDesc    string
		wantWeekly  int64
		wantMachine string
	}{
		{
			name:    "module found",
			pkgName: "drupal/pathauto",
			response: map[string]any{
				"data": []map[string]any{{
					"attributes": map[string]any{
						"body":                        map[string]any{"summary": "Automatically generate URL aliases."},
						"field_active_installs_total": int64(330857),
					},
				}},
			},
			wantDesc:    "Automatically generate URL aliases.",
			wantWeekly:  330857,
			wantMachine: "pathauto",
		},
		{
			name:        "no summary on the module page",
			pkgName:     "drupal/jsonapi_extras",
			response:    map[string]any{"data": []map[string]any{{"attributes": map[string]any{"body": map[string]any{"summary": ""}, "field_active_installs_total": int64(5)}}}},
			wantDesc:    "",
			wantWeekly:  5,
			wantMachine: "jsonapi_extras",
		},
		{
			name:        "module not found",
			pkgName:     "drupal/does-not-exist",
			response:    map[string]any{"data": []map[string]any{}},
			wantDesc:    "",
			wantWeekly:  0,
			wantMachine: "does-not-exist",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMachineName string
			mux := http.NewServeMux()
			mux.HandleFunc("/jsonapi/node/project_module", func(w http.ResponseWriter, r *http.Request) {
				gotMachineName = r.URL.Query().Get("filter[field_project_machine_name]")
				writeJSON(w, tt.response)
			})
			ts := httptest.NewServer(mux)
			defer ts.Close()
			redirectTo(t, ts)

			pkg := &Package{Name: tt.pkgName}
			fetchDrupalOrgSummary(context.Background(), &http.Client{}, pkg)

			if pkg.Description != tt.wantDesc {
				t.Errorf("Description = %q, want %q", pkg.Description, tt.wantDesc)
			}
			if pkg.Weekly != tt.wantWeekly {
				t.Errorf("Weekly = %d, want %d", pkg.Weekly, tt.wantWeekly)
			}
			if gotMachineName != tt.wantMachine {
				t.Errorf("machine name queried = %q, want %q", gotMachineName, tt.wantMachine)
			}
		})
	}
}

func TestFetchDrupalOrgSummary_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	redirectTo(t, ts)

	pkg := &Package{Name: "drupal/pathauto"}
	fetchDrupalOrgSummary(context.Background(), &http.Client{}, pkg)

	if pkg.Description != "" || pkg.Weekly != 0 {
		t.Errorf("expected pkg to stay untouched on HTTP error, got %+v", pkg)
	}
}

func TestFetchWordPressOrgSummary(t *testing.T) {
	var gotSlug string
	var gotFields url.Values
	mux := http.NewServeMux()
	mux.HandleFunc("/plugins/info/1.2/", func(w http.ResponseWriter, r *http.Request) {
		gotSlug = r.URL.Query().Get("request[slug]")
		gotFields = r.URL.Query()
		writeJSON(w, map[string]any{
			"version":           "11.1.0",
			"short_description": "Everything you need to launch an online store.",
			"active_installs":   int64(7000000),
		})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	redirectTo(t, ts)

	pkg := &Package{Name: "wpackagist-plugin/woocommerce"}
	fetchWordPressOrgSummary(context.Background(), &http.Client{}, pkg)

	if pkg.Description != "Everything you need to launch an online store." {
		t.Errorf("Description = %q", pkg.Description)
	}
	if pkg.LatestVersion != "11.1.0" {
		t.Errorf("LatestVersion = %q, want 11.1.0", pkg.LatestVersion)
	}
	if pkg.Weekly != 7000000 {
		t.Errorf("Weekly = %d, want 7000000", pkg.Weekly)
	}
	if gotSlug != "woocommerce" {
		t.Errorf("slug queried = %q, want woocommerce", gotSlug)
	}
	// The heavy fields (full description, changelog, screenshots…) must be
	// explicitly turned off, or every call downloads the whole plugin page.
	if got := gotFields.Get("request[fields][description]"); got != "0" {
		t.Errorf("request[fields][description] = %q, want it turned off", got)
	}
	if got := gotFields.Get("request[fields][screenshots]"); got != "0" {
		t.Errorf("request[fields][screenshots] = %q, want it turned off", got)
	}
}

func TestFetchWordPressOrgSummary_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"Plugin not found."}`))
	}))
	defer ts.Close()
	redirectTo(t, ts)

	pkg := &Package{Name: "wpackagist-plugin/does-not-exist"}
	fetchWordPressOrgSummary(context.Background(), &http.Client{}, pkg)

	if pkg.Description != "" || pkg.LatestVersion != "" || pkg.Weekly != 0 {
		t.Errorf("expected pkg to stay untouched for a missing plugin, got %+v", pkg)
	}
}

func TestFetchPackagistSummary_FallsBackToUpstreamRegistry(t *testing.T) {
	tests := []struct {
		name       string
		pkgName    string
		mountPath  string
		respond    func(w http.ResponseWriter)
		wantDesc   string
		wantWeekly int64
	}{
		{
			name:      "drupal module not on packagist",
			pkgName:   "drupal/pathauto",
			mountPath: "/jsonapi/node/project_module",
			respond: func(w http.ResponseWriter) {
				writeJSON(w, map[string]any{"data": []map[string]any{{
					"attributes": map[string]any{
						"body":                        map[string]any{"summary": "Automatically generate URL aliases."},
						"field_active_installs_total": int64(330857),
					},
				}}})
			},
			wantDesc:   "Automatically generate URL aliases.",
			wantWeekly: 330857,
		},
		{
			name:      "wordpress plugin not on packagist",
			pkgName:   "wpackagist-plugin/woocommerce",
			mountPath: "/plugins/info/1.2/",
			respond: func(w http.ResponseWriter) {
				writeJSON(w, map[string]any{"short_description": "Everything you need.", "active_installs": int64(7000000)})
			},
			wantDesc:   "Everything you need.",
			wantWeekly: 7000000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			// Packagist's own search finds nothing for these — that's the
			// whole reason the fallback exists.
			mux.HandleFunc("/search.json", func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, map[string]any{"results": []map[string]any{}})
			})
			mux.HandleFunc(tt.mountPath, func(w http.ResponseWriter, r *http.Request) {
				tt.respond(w)
			})
			ts := httptest.NewServer(mux)
			defer ts.Close()
			redirectTo(t, ts)

			pkg := &Package{Name: tt.pkgName}
			fetchPackagistSummary(context.Background(), &http.Client{}, pkg)

			if pkg.Description != tt.wantDesc {
				t.Errorf("Description = %q, want %q", pkg.Description, tt.wantDesc)
			}
			if pkg.Weekly != tt.wantWeekly {
				t.Errorf("Weekly = %d, want %d", pkg.Weekly, tt.wantWeekly)
			}
		})
	}
}

func TestFetchPackagistSummary_PrefersPackagistSearchWhenFound(t *testing.T) {
	// If Packagist's own search ever does find a match for one of these
	// names, that result must win and the upstream fallback must not run.
	mux := http.NewServeMux()
	mux.HandleFunc("/search.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"results": []map[string]any{
			{"name": "drupal/pathauto", "description": "From packagist search.", "downloads": int64(1)},
		}})
	})
	count := unexpectedRequests(t, mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()
	redirectTo(t, ts)

	pkg := &Package{Name: "drupal/pathauto"}
	fetchPackagistSummary(context.Background(), &http.Client{}, pkg)

	if pkg.Description != "From packagist search." {
		t.Errorf("Description = %q, want the packagist search result", pkg.Description)
	}
	if count.Load() != 0 {
		t.Error("the drupal.org fallback must not be called when packagist search already found a match")
	}
}
