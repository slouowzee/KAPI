package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/slouowzee/kapi/internal/testutil"
)

// redirectTo sends every request to ts, so no test can reach the real API.
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func TestCreateRepo_NoToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("no request expected without a token, got %s %s", r.Method, r.URL.Path)
	}))
	defer ts.Close()
	redirectTo(t, ts)

	if _, err := CreateRepo(context.Background(), "", "repo", false); !errors.Is(err, ErrNoToken) {
		t.Errorf("err = %v, want ErrNoToken", err)
	}
}

func TestCreateRepo_Created(t *testing.T) {
	var gotAuth string
	var gotBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		writeJSON(w, http.StatusCreated, map[string]any{"ssh_url": "git@github.com:me/repo.git", "clone_url": "https://github.com/me/repo.git"})
	}))
	defer ts.Close()
	redirectTo(t, ts)

	repo, err := CreateRepo(context.Background(), "secret", "repo", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.SSHURL != "git@github.com:me/repo.git" || repo.CloneURL != "https://github.com/me/repo.git" {
		t.Errorf("repo = %+v", repo)
	}
	if gotAuth != "Bearer secret" {
		t.Errorf("Authorization = %q, want Bearer secret", gotAuth)
	}
	if gotBody["private"] != true || gotBody["name"] != "repo" {
		t.Errorf("request body = %v", gotBody)
	}
}

func TestCreateRepo_NameTaken(t *testing.T) {
	tests := []struct {
		name        string
		private     bool
		existing    map[string]any
		existingErr int
		branches    []map[string]string
		wantReuse   bool
	}{
		{
			name:      "empty repository with the same visibility is reused",
			private:   true,
			existing:  map[string]any{"ssh_url": "git@github.com:me/repo.git", "private": true},
			branches:  []map[string]string{},
			wantReuse: true,
		},
		{
			name:     "public repository is never reused for a private request",
			private:  true,
			existing: map[string]any{"ssh_url": "git@github.com:me/repo.git", "private": false},
			branches: []map[string]string{},
		},
		{
			name:     "private repository is not reused for a public request",
			private:  false,
			existing: map[string]any{"ssh_url": "git@github.com:me/repo.git", "private": true},
			branches: []map[string]string{},
		},
		{
			name:     "repository with a branch is not reused",
			existing: map[string]any{"ssh_url": "git@github.com:me/repo.git", "private": false},
			branches: []map[string]string{{"name": "main"}},
		},
		{
			name:        "missing repository reports the API message",
			existingErr: http.StatusNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("POST /user/repos", func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
					"message": "Repository creation failed.",
					"errors":  []map[string]string{{"message": "name already exists on this account"}},
				})
			})
			mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, http.StatusOK, map[string]string{"login": "me"})
			})
			mux.HandleFunc("GET /repos/me/repo", func(w http.ResponseWriter, r *http.Request) {
				if tt.existingErr != 0 {
					w.WriteHeader(tt.existingErr)
					return
				}
				writeJSON(w, http.StatusOK, tt.existing)
			})
			mux.HandleFunc("GET /repos/me/repo/branches", func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, http.StatusOK, tt.branches)
			})
			ts := httptest.NewServer(mux)
			defer ts.Close()
			redirectTo(t, ts)

			repo, err := CreateRepo(context.Background(), "secret", "repo", tt.private)
			if tt.wantReuse {
				if err != nil || repo.SSHURL == "" {
					t.Fatalf("expected the empty repo to be reused, got %+v, %v", repo, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "name already exists on this account") {
				t.Errorf("err = %v, want the API message", err)
			}
		})
	}
}

func TestCreateRepo_Errors(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{name: "http error", handler: func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusForbidden, map[string]string{"message": "forbidden"})
		}},
		{name: "missing scope", handler: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Accepted-OAuth-Scopes", "repo")
			w.WriteHeader(http.StatusNotFound)
		}},
		{name: "invalid json", handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("not json {{{"))
		}},
		{name: "missing ssh url", handler: func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusCreated, map[string]string{"other": "value"})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(tt.handler)
			defer ts.Close()
			redirectTo(t, ts)

			if _, err := CreateRepo(context.Background(), "secret", "repo", false); err == nil {
				t.Error("expected an error")
			}
		})
	}
}
