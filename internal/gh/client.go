// Package gh implements a small typed wrapper around the subset of the
// GitHub REST API that the release-notes tool consumes. It is intentionally
// stdlib-first; all I/O happens through net/http.
package gh

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

// Client is the read-only surface of the GitHub API consumed by the tool.
// All implementations must be safe for concurrent use.
type Client interface {
	ListReleases(ctx context.Context, owner, repo string) ([]Release, error)
	GetContent(ctx context.Context, owner, repo, path, ref string) ([]byte, error)
	Compare(ctx context.Context, owner, repo, base, head string) (CompareResult, error)
	PRsForCommit(ctx context.Context, owner, repo, sha string) ([]PR, error)
	GetPR(ctx context.Context, owner, repo string, number int) (PR, error)
}

// httpClient is the default Client implementation. It wraps an *http.Client
// with the auth header, user-agent, rate-limit retry, and pagination logic
// shared by all endpoints.
type httpClient struct {
	http    *http.Client
	token   string
	ua      string
	baseURL string
	// rateLimitCap caps the maximum sleep we will perform when a 429 response
	// asks us to wait for a reset. Defaults to 5 minutes.
	rateLimitCap time.Duration
}

// New constructs a Client backed by the public GitHub API.
func New(token string) Client {
	return &httpClient{
		http:         &http.Client{Timeout: 30 * time.Second},
		token:        token,
		ua:           userAgent(),
		baseURL:      "https://api.github.com",
		rateLimitCap: 5 * time.Minute,
	}
}

// userAgent reads the binary version embedded by the Go toolchain via
// runtime/debug.ReadBuildInfo and falls back to "dev" for local builds.
func userAgent() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		v := info.Main.Version
		if v != "" && v != "(devel)" {
			return "matthyx-release-notes/" + v
		}
	}
	return "matthyx-release-notes/dev"
}

// do executes a single API request, applying auth + UA + retry policy.
// It returns the parsed body bytes, the response headers and any error.
func (c *httpClient) do(ctx context.Context, method, urlStr string) ([]byte, http.Header, error) {
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, urlStr, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/vnd.github+json, application/vnd.github.groot-preview+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		req.Header.Set("User-Agent", c.ua)

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, nil, fmt.Errorf("http: %w", err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, nil, fmt.Errorf("read body: %w", err)
		}

		switch resp.StatusCode {
		case http.StatusOK:
			return body, resp.Header, nil
		case http.StatusUnauthorized:
			return nil, nil, fmt.Errorf("%w: %s", ErrUnauthorized, urlStr)
		case http.StatusNotFound:
			return nil, nil, fmt.Errorf("%w: %s", ErrNotFound, urlStr)
		case http.StatusForbidden, http.StatusTooManyRequests:
			// 429 / abuse / secondary limit. Try once to honor reset header.
			if attempt == 0 {
				if d := parseResetDelay(resp.Header, time.Now()); d > 0 {
					if d > c.rateLimitCap {
						d = c.rateLimitCap
					}
					fmt.Fprintf(stderr, "INFO: rate limited; sleeping %s before retry of %s\n", d, urlStr)
					select {
					case <-time.After(d):
					case <-ctx.Done():
						return nil, nil, ctx.Err()
					}
					continue
				}
			}
			return nil, nil, fmt.Errorf("%w: %s (status %d)", ErrRateLimited, urlStr, resp.StatusCode)
		default:
			return nil, nil, fmt.Errorf("github: %s -> %d: %s", urlStr, resp.StatusCode, strings.TrimSpace(string(body)))
		}
	}
	return nil, nil, fmt.Errorf("%w: gave up retrying", ErrRateLimited)
}

// parseResetDelay returns the duration to sleep before retrying, given the
// response headers and the current time. Returns 0 if there is no useful
// hint in the headers.
func parseResetDelay(h http.Header, now time.Time) time.Duration {
	if v := h.Get("Retry-After"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil {
			return time.Duration(secs) * time.Second
		}
	}
	if v := h.Get("X-RateLimit-Reset"); v != "" {
		if epoch, err := strconv.ParseInt(v, 10, 64); err == nil {
			d := time.Until(time.Unix(epoch, 0))
			if d < 0 {
				return 0
			}
			return d
		}
		_ = now
	}
	return 0
}

// nextPageURL parses a Link header and returns the URL of the rel="next" page,
// or "" when no next page is announced. Malformed headers return "" without
// raising an error so a single-page endpoint is treated as terminal.
func nextPageURL(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}
	for _, part := range strings.Split(linkHeader, ",") {
		segs := strings.Split(strings.TrimSpace(part), ";")
		if len(segs) < 2 {
			continue
		}
		raw := strings.TrimSpace(segs[0])
		if !strings.HasPrefix(raw, "<") || !strings.HasSuffix(raw, ">") {
			continue
		}
		urlPart := raw[1 : len(raw)-1]
		for _, attr := range segs[1:] {
			attr = strings.TrimSpace(attr)
			if attr == `rel="next"` || attr == `rel=next` {
				if _, err := url.Parse(urlPart); err == nil {
					return urlPart
				}
			}
		}
	}
	return ""
}

// decodeJSON is a small typed helper used by every endpoint.
func decodeJSON[T any](b []byte) (T, error) {
	var out T
	if err := json.Unmarshal(b, &out); err != nil {
		return out, fmt.Errorf("decode: %w", err)
	}
	return out, nil
}
