package gh

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient returns an httpClient pointed at the given test server.
func newTestClient(srv *httptest.Server) *httpClient {
	return &httpClient{
		http:         srv.Client(),
		token:        "test-token",
		ua:           "test-ua",
		baseURL:      srv.URL,
		rateLimitCap: 5 * time.Second,
	}
}

func TestListReleases_Pagination(t *testing.T) {
	page1 := []Release{{TagName: "v1"}, {TagName: "v2"}}
	page2 := []Release{{TagName: "v3"}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("missing auth header: %q", got)
		}
		if got := r.Header.Get("User-Agent"); got != "test-ua" {
			t.Errorf("missing UA: %q", got)
		}
		// Page identification by query param.
		if r.URL.Query().Get("page") == "2" {
			_ = json.NewEncoder(w).Encode(page2)
			return
		}
		// First page: announce next page in Link header.
		w.Header().Set("Link", `<`+r.Host+`/x?page=2>; rel="next"`)
		// We rebuild against srv.URL because Host may not match.
		w.Header().Set("Link", `<`+httpServerURL(r)+`?page=2>; rel="next"`)
		_ = json.NewEncoder(w).Encode(page1)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	got, err := c.ListReleases(context.Background(), "o", "r")
	if err != nil {
		t.Fatalf("ListReleases: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("want 3 releases, got %d", len(got))
	}
}

// httpServerURL reconstructs the server URL from the request.
func httpServerURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func TestRateLimit_RetryThenSucceed(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_ = json.NewEncoder(w).Encode([]Release{{TagName: "v1"}})
	}))
	defer srv.Close()

	c := newTestClient(srv)
	var sink bytes.Buffer
	old := stderr
	stderr = &sink
	defer func() { stderr = old }()

	got, err := c.ListReleases(context.Background(), "o", "r")
	if err != nil {
		t.Fatalf("ListReleases: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("want 1, got %d", len(got))
	}
	if !strings.Contains(sink.String(), "rate limited") {
		t.Errorf("expected rate-limit log, got %q", sink.String())
	}
}

func TestRateLimit_GivesUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.ListReleases(context.Background(), "o", "r")
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("want ErrRateLimited, got %v", err)
	}
}

func TestUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	c := newTestClient(srv)
	_, err := c.ListReleases(context.Background(), "o", "r")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("want ErrUnauthorized, got %v", err)
	}
}

func TestNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := newTestClient(srv)
	_, err := c.ListReleases(context.Background(), "o", "r")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestGetContent_Decode(t *testing.T) {
	want := "hello world\n"
	body := content{
		Encoding: "base64",
		Content:  base64.StdEncoding.EncodeToString([]byte(want)),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()
	c := newTestClient(srv)
	got, err := c.GetContent(context.Background(), "o", "r", "f", "main")
	if err != nil {
		t.Fatalf("GetContent: %v", err)
	}
	if string(got) != want {
		t.Errorf("got %q want %q", string(got), want)
	}
}

func TestGetContent_BadEncoding(t *testing.T) {
	body := content{Encoding: "rot13", Content: "x"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()
	c := newTestClient(srv)
	_, err := c.GetContent(context.Background(), "o", "r", "f", "")
	if !errors.Is(err, ErrInvalidContent) {
		t.Errorf("want ErrInvalidContent, got %v", err)
	}
}

func TestGetContent_BadBase64(t *testing.T) {
	body := content{Encoding: "base64", Content: "!!!not-base64!!!"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()
	c := newTestClient(srv)
	_, err := c.GetContent(context.Background(), "o", "r", "f", "")
	if !errors.Is(err, ErrInvalidContent) {
		t.Errorf("want ErrInvalidContent, got %v", err)
	}
}

func TestPRsForCommit(t *testing.T) {
	prs := []PR{{Number: 7, Title: "x"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(prs)
	}))
	defer srv.Close()
	c := newTestClient(srv)
	got, err := c.PRsForCommit(context.Background(), "o", "r", "abc")
	if err != nil {
		t.Fatalf("PRsForCommit: %v", err)
	}
	if len(got) != 1 || got[0].Number != 7 {
		t.Errorf("got %+v", got)
	}
}

func TestCompare(t *testing.T) {
	res := CompareResult{Commits: []Commit{{SHA: "abc"}}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(res)
	}))
	defer srv.Close()
	c := newTestClient(srv)
	got, err := c.Compare(context.Background(), "o", "r", "v1", "v2")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(got.Commits) != 1 || got.Commits[0].SHA != "abc" {
		t.Errorf("got %+v", got)
	}
}

func TestNextPageURL(t *testing.T) {
	cases := map[string]string{
		``: "",
		`<https://api.github.com/x?page=2>; rel="next", <https://api.github.com/x?page=5>; rel="last"`: "https://api.github.com/x?page=2",
		// malformed → empty (we don't error)
		`malformed`: "",
		// only "last", no "next"
		`<https://api.github.com/x?page=5>; rel="last"`: "",
	}
	for hdr, want := range cases {
		if got := nextPageURL(hdr); got != want {
			t.Errorf("nextPageURL(%q) = %q, want %q", hdr, got, want)
		}
	}
}

// fakeClient lets us drive ExtractPRs without httptest.
type fakeClient struct {
	prsByCommit map[string][]PR
	prByNumber  map[int]PR
	notFound    map[int]bool
	calls       []string
}

func (f *fakeClient) ListReleases(_ context.Context, _, _ string) ([]Release, error) {
	return nil, nil
}
func (f *fakeClient) GetContent(_ context.Context, _, _, _, _ string) ([]byte, error) {
	return nil, nil
}
func (f *fakeClient) Compare(_ context.Context, _, _, _, _ string) (CompareResult, error) {
	return CompareResult{}, nil
}
func (f *fakeClient) PRsForCommit(_ context.Context, _, _, sha string) ([]PR, error) {
	f.calls = append(f.calls, "prs:"+sha)
	return f.prsByCommit[sha], nil
}
func (f *fakeClient) GetPR(_ context.Context, _, _ string, n int) (PR, error) {
	f.calls = append(f.calls, fmt.Sprintf("get:%d", n))
	if f.notFound[n] {
		return PR{}, ErrNotFound
	}
	pr, ok := f.prByNumber[n]
	if !ok {
		return PR{}, ErrNotFound
	}
	return pr, nil
}

func TestExtractPRs_Primary(t *testing.T) {
	f := &fakeClient{
		prsByCommit: map[string][]PR{
			"sha1": {{Number: 1, Title: "a", MergedAt: time.Unix(1, 0)}},
			"sha2": {{Number: 2, Title: "b", MergedAt: time.Unix(2, 0)}},
		},
	}
	got, err := ExtractPRs(context.Background(), f, "o", "r", []Commit{{SHA: "sha1"}, {SHA: "sha2"}})
	if err != nil {
		t.Fatalf("ExtractPRs: %v", err)
	}
	if len(got) != 2 || got[0].Number != 1 || got[1].Number != 2 {
		t.Errorf("got %+v", got)
	}
}

func TestExtractPRs_RegexFallback(t *testing.T) {
	cm := Commit{SHA: "sha1"}
	cm.Commit.Message = "Bump x to y (#42)"
	f := &fakeClient{
		prsByCommit: map[string][]PR{}, // empty → regex fires
		prByNumber: map[int]PR{
			42: {Number: 42, Title: "Bump x to y", MergedAt: time.Unix(10, 0)},
		},
	}
	var sink bytes.Buffer
	old := stderr
	stderr = &sink
	defer func() { stderr = old }()
	got, err := ExtractPRs(context.Background(), f, "o", "r", []Commit{cm})
	if err != nil {
		t.Fatalf("ExtractPRs: %v", err)
	}
	if len(got) != 1 || got[0].Number != 42 {
		t.Errorf("regex fallback failed: %+v", got)
	}
	if !strings.Contains(sink.String(), "INFO: PR #42") {
		t.Errorf("expected INFO log, got %q", sink.String())
	}
}

func TestExtractPRs_DedupAndSort(t *testing.T) {
	f := &fakeClient{
		prsByCommit: map[string][]PR{
			"sha1": {
				{Number: 5, MergedAt: time.Unix(50, 0)},
				{Number: 3, MergedAt: time.Unix(30, 0)},
			},
			"sha2": {
				{Number: 5, MergedAt: time.Unix(50, 0)}, // dup
				{Number: 4, MergedAt: time.Unix(40, 0)},
			},
		},
	}
	got, err := ExtractPRs(context.Background(), f, "o", "r", []Commit{{SHA: "sha1"}, {SHA: "sha2"}})
	if err != nil {
		t.Fatalf("ExtractPRs: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 (deduped), got %d: %+v", len(got), got)
	}
	if got[0].Number != 3 || got[1].Number != 4 || got[2].Number != 5 {
		t.Errorf("sort order wrong: %+v", got)
	}
}
