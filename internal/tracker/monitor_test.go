package tracker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// mockReleaseStore implements ReleaseStore for tests.
type mockReleaseStore struct {
	lastCheck time.Time
	etag      string
	releases  []Release
}

func (m *mockReleaseStore) GetLastCheck(ctx context.Context) (*time.Time, string, error) {
	if m.lastCheck.IsZero() {
		return nil, "", nil
	}
	t := m.lastCheck
	return &t, m.etag, nil
}

func (m *mockReleaseStore) SetLastCheck(ctx context.Context, t time.Time, etag string) error {
	m.lastCheck = t
	m.etag = etag
	return nil
}

func (m *mockReleaseStore) SaveRelease(ctx context.Context, release Release) error {
	m.releases = append(m.releases, release)
	return nil
}

func (m *mockReleaseStore) ListReleases(ctx context.Context) ([]Release, error) {
	return m.releases, nil
}

func TestCheckLatest_FetchSuccess(t *testing.T) {
	expected := Release{
		Tag:         "v2.0.0",
		Body:        "feat: new feature\nfix: bug fix",
		PublishedAt: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		HTMLURL:     "https://github.com/org/repo/releases/tag/v2.0.0",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/releases/latest" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("ETag", `"abc123"`)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer srv.Close()

	store := &mockReleaseStore{}
	monitor := NewGitHubReleaseMonitor("owner", "repo", store)
	monitor.SetBaseURL(srv.URL)
	monitor.SetHTTPClient(srv.Client())

	ctx := context.Background()
	release, hasNew, err := monitor.CheckLatest(ctx)
	if err != nil {
		t.Fatalf("CheckLatest() error: %v", err)
	}
	if !hasNew {
		t.Error("expected hasNew=true for first fetch")
	}
	if release.Tag != expected.Tag {
		t.Errorf("tag = %q, want %q", release.Tag, expected.Tag)
	}
	if release.Body != expected.Body {
		t.Errorf("body = %q, want %q", release.Body, expected.Body)
	}

	// Verify ETag was cached.
	_, cachedETag, _ := store.GetLastCheck(ctx)
	if cachedETag != `"abc123"` {
		t.Errorf("cached etag = %q, want %q", cachedETag, `"abc123"`)
	}
}

func TestCheckLatest_ETagCache(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.Header.Get("If-None-Match") == `"stale-etag"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"stale-etag"`)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Release{Tag: "v2.0.0", Body: "test"})
	}))
	defer srv.Close()

	store := &mockReleaseStore{}
	monitor := NewGitHubReleaseMonitor("owner", "repo", store)
	monitor.SetBaseURL(srv.URL)
	monitor.SetHTTPClient(srv.Client())

	ctx := context.Background()

	// First call: should fetch.
	release, hasNew, err := monitor.CheckLatest(ctx)
	if err != nil {
		t.Fatalf("first CheckLatest() error: %v", err)
	}
	if !hasNew {
		t.Error("expected hasNew=true on first fetch")
	}
	if release.Tag != "v2.0.0" {
		t.Errorf("tag = %q, want v2.0.0", release.Tag)
	}
	if callCount != 1 {
		t.Errorf("call count after first = %d, want 1", callCount)
	}

	// Second call: should get 304.
	_, hasNew2, err := monitor.CheckLatest(ctx)
	if err != nil {
		t.Fatalf("second CheckLatest() error: %v", err)
	}
	if hasNew2 {
		t.Error("expected hasNew=false on ETag cache hit")
	}
	if callCount != 2 {
		t.Errorf("call count after second = %d, want 2", callCount)
	}
}

func TestCheckLatest_AuthToken(t *testing.T) {
	var authHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Release{Tag: "v1.0.0", Body: "test"})
	}))
	defer srv.Close()

	// Set GITHUB_TOKEN and restore after.
	os.Setenv("GITHUB_TOKEN", "test-token-123")
	defer os.Unsetenv("GITHUB_TOKEN")

	store := &mockReleaseStore{}
	monitor := NewGitHubReleaseMonitor("owner", "repo", store)
	monitor.SetBaseURL(srv.URL)
	monitor.SetHTTPClient(srv.Client())

	_, _, err := monitor.CheckLatest(context.Background())
	if err != nil {
		t.Fatalf("CheckLatest() error: %v", err)
	}

	if authHeader != "Bearer test-token-123" {
		t.Errorf("Authorization header = %q, want %q", authHeader, "Bearer test-token-123")
	}
}

func TestCheckLatest_NoToken(t *testing.T) {
	var authHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Release{Tag: "v1.0.0", Body: "test"})
	}))
	defer srv.Close()

	// Ensure GITHUB_TOKEN is not set.
	os.Unsetenv("GITHUB_TOKEN")

	store := &mockReleaseStore{}
	monitor := NewGitHubReleaseMonitor("owner", "repo", store)
	monitor.SetBaseURL(srv.URL)
	monitor.SetHTTPClient(srv.Client())

	_, _, err := monitor.CheckLatest(context.Background())
	if err != nil {
		t.Fatalf("CheckLatest() error: %v", err)
	}

	if authHeader != "" {
		t.Errorf("expected no Authorization header, got %q", authHeader)
	}
}

func TestCheckLatest_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Reset", "9999999999")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	store := &mockReleaseStore{}
	monitor := NewGitHubReleaseMonitor("owner", "repo", store)
	monitor.SetBaseURL(srv.URL)
	monitor.SetHTTPClient(srv.Client())

	_, _, err := monitor.CheckLatest(context.Background())
	if err == nil {
		t.Fatal("expected rate limit error")
	}
}

func TestListReleases_All(t *testing.T) {
	releases := []Release{
		{Tag: "v2.0.0", Body: "latest", HTMLURL: "https://github.com/org/repo/releases/tag/v2.0.0"},
		{Tag: "v1.0.0", Body: "first", HTMLURL: "https://github.com/org/repo/releases/tag/v1.0.0"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(releases)
	}))
	defer srv.Close()

	store := &mockReleaseStore{}
	monitor := NewGitHubReleaseMonitor("owner", "repo", store)
	monitor.SetBaseURL(srv.URL)
	monitor.SetHTTPClient(srv.Client())

	ctx := context.Background()
	result, err := monitor.ListReleases(ctx, "")
	if err != nil {
		t.Fatalf("ListReleases() error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("got %d releases, want 2", len(result))
	}
}

func TestListReleases_Since(t *testing.T) {
	releases := []Release{
		{Tag: "v3.0.0", Body: "latest"},
		{Tag: "v2.0.0", Body: "middle"},
		{Tag: "v1.0.0", Body: "first"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(releases)
	}))
	defer srv.Close()

	store := &mockReleaseStore{}
	monitor := NewGitHubReleaseMonitor("owner", "repo", store)
	monitor.SetBaseURL(srv.URL)
	monitor.SetHTTPClient(srv.Client())

	ctx := context.Background()
	result, err := monitor.ListReleases(ctx, "v2.0.0")
	if err != nil {
		t.Fatalf("ListReleases() error: %v", err)
	}

	// Since "v2.0.0" should exclude v2.0.0 and v1.0.0, return only v3.0.0.
	if len(result) != 1 {
		t.Fatalf("got %d releases, want 1", len(result))
	}
	if result[0].Tag != "v3.0.0" {
		t.Errorf("tag = %q, want v3.0.0", result[0].Tag)
	}
}
