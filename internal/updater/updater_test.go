package updater

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestIsNewer(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"v1.0.0", "v1.0.1", true},
		{"v1.0.0", "v2.0.0", true},
		{"v1.0.0", "v1.0.0", false},
		{"v1.0.1", "v1.0.0", false},
		{"v2.0.0", "v1.9.9", false},
	}
	for _, c := range cases {
		got := isNewer(c.current, c.latest)
		if got != c.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}

func TestCurrentVersion(t *testing.T) {
	if CurrentVersion == "" {
		t.Error("CurrentVersion should not be empty")
	}
	if CurrentVersion[0] != 'v' {
		t.Errorf("CurrentVersion = %q: expected v prefix", CurrentVersion)
	}
}

func TestCheckLatestVersion_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Release{TagName: "v2.0.0"})
	}))
	defer ts.Close()
	redirectTo(t, ts)

	ver, err := checkLatestVersion(context.Background())
	if err != nil {
		t.Fatalf("checkLatestVersion() unexpected error: %v", err)
	}
	if ver != "v2.0.0" {
		t.Errorf("checkLatestVersion() = %q, want v2.0.0", ver)
	}
}

func TestCheckLatestVersion_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	redirectTo(t, ts)

	_, err := checkLatestVersion(context.Background())
	if err == nil {
		t.Error("expected error for HTTP 500, got nil")
	}
}

func TestCheckLatestVersion_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer ts.Close()
	redirectTo(t, ts)

	_, err := checkLatestVersion(context.Background())
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestCheckLatestVersion_ContextCancelled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(Release{TagName: "v2.0.0"})
	}))
	defer ts.Close()
	redirectTo(t, ts)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := checkLatestVersion(ctx)
	if err == nil {
		t.Error("expected context deadline error, got nil")
	}
}

func TestCheckLatestVersion_SetsCorrectHeaders(t *testing.T) {
	var gotUA, gotAccept string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotAccept = r.Header.Get("Accept")
		_ = json.NewEncoder(w).Encode(Release{TagName: "v1.0.0"})
	}))
	defer ts.Close()
	redirectTo(t, ts)

	if _, err := checkLatestVersion(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotUA != "kapi-updater" {
		t.Errorf("User-Agent = %q, want kapi-updater", gotUA)
	}
	if gotAccept != "application/vnd.github.v3+json" {
		t.Errorf("Accept = %q, want application/vnd.github.v3+json", gotAccept)
	}
}

func TestCheck_UpdateAvailable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(Release{TagName: "v999.0.0"})
	}))
	defer ts.Close()
	redirectTo(t, ts)

	ch := Check(context.Background())
	info := <-ch

	if !info.Available {
		t.Errorf("Check().Available = false, want true (current=%s, latest=v999.0.0)", CurrentVersion)
	}
	if info.CurrentVersion != CurrentVersion {
		t.Errorf("CurrentVersion = %q, want %q", info.CurrentVersion, CurrentVersion)
	}
	if info.LatestVersion != "v999.0.0" {
		t.Errorf("LatestVersion = %q, want v999.0.0", info.LatestVersion)
	}
}

func TestCheck_NoUpdate(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(Release{TagName: CurrentVersion})
	}))
	defer ts.Close()
	redirectTo(t, ts)

	ch := Check(context.Background())
	info := <-ch

	if info.Available {
		t.Errorf("Check().Available = true, want false (same version)")
	}
}

func TestCheck_HTTPFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()
	redirectTo(t, ts)

	ch := Check(context.Background())
	info := <-ch

	if info.Available {
		t.Error("Check() on HTTP error: Available should be false")
	}
	if info.CurrentVersion != CurrentVersion {
		t.Errorf("CurrentVersion = %q, want %q", info.CurrentVersion, CurrentVersion)
	}
}

func TestCheck_ChannelIsBuffered(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(Release{TagName: "v1.0.0"})
	}))
	defer ts.Close()
	redirectTo(t, ts)

	ch := Check(context.Background())
	time.Sleep(100 * time.Millisecond)

	select {
	case <-ch:
	default:
		t.Error("expected result to be available without blocking")
	}
}
