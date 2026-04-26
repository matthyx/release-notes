package gh

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
)

// GetContent fetches a single file's bytes from the GitHub /contents API and
// returns the base64-decoded payload.
func (c *httpClient) GetContent(ctx context.Context, owner, repo, path, ref string) ([]byte, error) {
	q := url.Values{}
	if ref != "" {
		q.Set("ref", ref)
	}
	urlStr := fmt.Sprintf("%s/repos/%s/%s/contents/%s", c.baseURL, owner, repo, path)
	if len(q) > 0 {
		urlStr += "?" + q.Encode()
	}
	body, _, err := c.do(ctx, "GET", urlStr)
	if err != nil {
		return nil, err
	}
	cont, err := decodeJSON[content](body)
	if err != nil {
		return nil, err
	}
	if cont.Encoding != "base64" {
		return nil, fmt.Errorf("%w: unexpected encoding %q", ErrInvalidContent, cont.Encoding)
	}
	// GitHub returns base64 split on newlines; strip them before decoding.
	cleaned := strings.ReplaceAll(cont.Content, "\n", "")
	cleaned = strings.ReplaceAll(cleaned, "\r", "")
	decoded, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		return nil, fmt.Errorf("%w: base64: %v", ErrInvalidContent, err)
	}
	return decoded, nil
}
