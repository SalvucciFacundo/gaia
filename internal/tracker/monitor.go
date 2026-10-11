package tracker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// Release represents a GitHub release as returned by the REST API.
type Release struct {
	Tag         string    `json:"tag_name"`
	Body        string    `json:"body"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
}

// ReleaseStore abstracts the persistent storage for release tracking data.
type ReleaseStore interface {
	GetLastCheck(ctx context.Context) (*time.Time, string, error) // time, etag, error
	SetLastCheck(ctx context.Context, t time.Time, etag string) error
	SaveRelease(ctx context.Context, release Release) error
	ListReleases(ctx context.Context) ([]Release, error)
}

// GitHubReleaseMonitor monitors upstream GitHub releases with ETag caching.
type GitHubReleaseMonitor struct {
	httpClient *http.Client
	baseURL    string
	owner      string
	repo       string
	db         ReleaseStore
}

// NewGitHubReleaseMonitor creates a new monitor for the given GitHub owner/repo.
func NewGitHubReleaseMonitor(owner, repo string, db ReleaseStore) *GitHubReleaseMonitor {
	return &GitHubReleaseMonitor{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    "https://api.github.com",
		owner:      owner,
		repo:       repo,
		db:         db,
	}
}

// SetBaseURL overrides the base API URL (for testing).
func (m *GitHubReleaseMonitor) SetBaseURL(url string) {
	m.baseURL = url
}

// SetHTTPClient overrides the HTTP client (for testing).
func (m *GitHubReleaseMonitor) SetHTTPClient(client *http.Client) {
	m.httpClient = client
}

// CheckLatest fetches the latest release from GitHub. Returns the release,
// whether it is new (hasNew), and any error. Uses ETag caching to avoid
// unnecessary API calls.
func (m *GitHubReleaseMonitor) CheckLatest(ctx context.Context) (*Release, bool, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", m.baseURL, m.owner, m.repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	m.setAuthHeader(req)

	// ETag caching: send If-None-Match if we have a cached ETag.
	_, etag, _ := m.db.GetLastCheck(ctx)
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	// 304 Not Modified — upstream has not changed.
	if resp.StatusCode == http.StatusNotModified {
		return nil, false, nil
	}

	// Rate limit exceeded.
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		resetTime := resp.Header.Get("X-RateLimit-Reset")
		return nil, false, fmt.Errorf("rate limit exceeded (resets at unix %s). Set GITHUB_TOKEN to increase limit", resetTime)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, false, fmt.Errorf("decode response: %w", err)
	}

	// Cache the new ETag for future requests.
	newETag := resp.Header.Get("ETag")
	if err := m.db.SetLastCheck(ctx, time.Now(), newETag); err != nil {
		return nil, false, fmt.Errorf("cache last check: %w", err)
	}

	hasNew := true
	return &release, hasNew, nil
}

// ListReleases fetches all releases since the given tag. If since is empty,
// returns all releases.
func (m *GitHubReleaseMonitor) ListReleases(ctx context.Context, since string) ([]Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=100", m.baseURL, m.owner, m.repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	m.setAuthHeader(req)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Filter: only releases since the given tag (exclusive).
	if since != "" {
		var filtered []Release
		found := false
		for _, r := range releases {
			if r.Tag == since {
				found = true
				break
			}
			if !found {
				filtered = append(filtered, r)
			}
		}
		releases = filtered
	}

	return releases, nil
}

// setAuthHeader adds an Authorization header if GITHUB_TOKEN is set.
func (m *GitHubReleaseMonitor) setAuthHeader(req *http.Request) {
	token := os.Getenv("GITHUB_TOKEN")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}
