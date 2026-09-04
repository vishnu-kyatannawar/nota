package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Repo is the GitHub repository releases come from.
const Repo = "vishnu-kyatannawar/nota"

const (
	defaultAPI      = "https://api.github.com"
	defaultDownload = "https://github.com"
	// A check runs in the background and must never hold anything up; a
	// download is larger but still bounded, so neither waits forever.
	checkTimeout    = 15 * time.Second
	downloadTimeout = 5 * time.Minute
)

// Release is the newest published release.
type Release struct {
	// Tag is the git tag, "v4.2.0" — the version with its leading v.
	Tag     string  `json:"tag"`
	Version Version `json:"version"`
	// URL is the human release page, for when installing is not possible.
	URL string `json:"url"`
}

// Client talks to GitHub. The two base URLs are fields so tests can point the
// whole thing at a local server and never touch the network.
type Client struct {
	HTTP *http.Client
	// Repo is "owner/name".
	Repo string
	// API is the GitHub API base; Download is the release-asset host.
	API      string
	Download string
	// Agent identifies this build; GitHub rejects requests without one.
	Agent string
}

// NewClient returns a client for the real GitHub, identifying itself with the
// running version.
func NewClient(version string) *Client {
	return &Client{
		HTTP:     &http.Client{Timeout: downloadTimeout},
		Repo:     Repo,
		API:      defaultAPI,
		Download: defaultDownload,
		Agent:    "nota/" + version,
	}
}

// get issues a GET that carries the user agent and fails on any non-200.
func (c *Client) get(ctx context.Context, url string, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request for %s: %w", url, err)
	}
	req.Header.Set("User-Agent", c.Agent)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting %s: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		// Drain a little so the connection can be reused, then give up.
		_, _ = io.CopyN(io.Discard, resp.Body, 512)
		_ = resp.Body.Close()
		// 403 here is almost always the unauthenticated rate limit.
		return nil, fmt.Errorf("%s returned %s", url, resp.Status)
	}
	return resp, nil
}

// Latest asks GitHub for the most recent release. A draft or prerelease is
// never returned by this endpoint, so whatever comes back is public.
func (c *Client) Latest(ctx context.Context) (Release, error) {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	url := fmt.Sprintf("%s/repos/%s/releases/latest", c.API, c.Repo)
	resp, err := c.get(ctx, url, "application/vnd.github+json")
	if err != nil {
		return Release{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	// Only the tag matters; the release title has been inconsistent across
	// versions and the generated body is a commit list, not a changelog.
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return Release{}, fmt.Errorf("reading the latest release: %w", err)
	}
	v, err := ParseVersion(body.TagName)
	if err != nil {
		return Release{}, fmt.Errorf("latest release: %w", err)
	}
	return Release{
		Tag:     body.TagName,
		Version: v,
		URL:     fmt.Sprintf("https://github.com/%s/releases/tag/%s", c.Repo, body.TagName),
	}, nil
}
