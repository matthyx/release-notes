package gh

import (
	"context"
	"fmt"
)

// maxReleasePages caps pagination on /releases at 20 pages × 100 = 2000 entries.
const maxReleasePages = 20

// ListReleases returns every release for the given repository, paginated by
// the rel="next" Link header up to maxReleasePages.
func (c *httpClient) ListReleases(ctx context.Context, owner, repo string) ([]Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=100", c.baseURL, owner, repo)
	var out []Release
	for page := 0; page < maxReleasePages && url != ""; page++ {
		body, hdr, err := c.do(ctx, "GET", url)
		if err != nil {
			return nil, err
		}
		batch, err := decodeJSON[[]Release](body)
		if err != nil {
			return nil, err
		}
		out = append(out, batch...)
		url = nextPageURL(hdr.Get("Link"))
	}
	return out, nil
}
