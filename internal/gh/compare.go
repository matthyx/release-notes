package gh

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Compare returns the commits between base and head on the given repo via the
// /compare endpoint. The slice mirrors GitHub's chronological ordering.
func (c *httpClient) Compare(ctx context.Context, owner, repo, base, head string) (CompareResult, error) {
	urlStr := fmt.Sprintf("%s/repos/%s/%s/compare/%s...%s", c.baseURL, owner, repo, base, head)
	body, _, err := c.do(ctx, "GET", urlStr)
	if err != nil {
		return CompareResult{}, err
	}
	return decodeJSON[CompareResult](body)
}

// PRsForCommit returns every PR linked by GitHub to the given commit SHA.
// This is the primary source for PR attribution.
func (c *httpClient) PRsForCommit(ctx context.Context, owner, repo, sha string) ([]PR, error) {
	urlStr := fmt.Sprintf("%s/repos/%s/%s/commits/%s/pulls", c.baseURL, owner, repo, sha)
	body, _, err := c.do(ctx, "GET", urlStr)
	if err != nil {
		return nil, err
	}
	return decodeJSON[[]PR](body)
}

// GetPR fetches a single PR by number — used as the fallback path when the
// commit message contains a `(#N)` reference but the commits/{sha}/pulls API
// returns no linked PRs.
func (c *httpClient) GetPR(ctx context.Context, owner, repo string, number int) (PR, error) {
	urlStr := fmt.Sprintf("%s/repos/%s/%s/pulls/%d", c.baseURL, owner, repo, number)
	body, _, err := c.do(ctx, "GET", urlStr)
	if err != nil {
		return PR{}, err
	}
	return decodeJSON[PR](body)
}

// prRefRE captures `(#NNN)` in the first line of a commit message — the
// pattern Git uses for squash-merge commit messages.
var prRefRE = regexp.MustCompile(`\(#(\d+)\)`)

// ExtractPRs collects PRs for every commit using the primary API path
// (/commits/{sha}/pulls). For commits where the API returns no linked PRs,
// it falls back to scanning the commit message for `(#N)` references and
// hydrating each via GetPR. Results are deduplicated by PR number and
// sorted by (MergedAt asc, Number asc).
func ExtractPRs(ctx context.Context, c Client, owner, repo string, commits []Commit) ([]PR, error) {
	seen := make(map[int]PR)
	for _, cm := range commits {
		prs, err := c.PRsForCommit(ctx, owner, repo, cm.SHA)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("PRsForCommit %s: %w", cm.SHA, err)
		}
		if len(prs) > 0 {
			for _, p := range prs {
				if _, ok := seen[p.Number]; !ok {
					seen[p.Number] = p
				}
			}
			continue
		}
		// Fallback regex path. Only the first commit-message line is
		// considered; merge commits commonly carry the (#N) marker there.
		first := cm.Commit.Message
		if i := strings.IndexByte(first, '\n'); i >= 0 {
			first = first[:i]
		}
		matches := prRefRE.FindAllStringSubmatch(first, -1)
		for _, m := range matches {
			n, err := strconv.Atoi(m[1])
			if err != nil {
				continue
			}
			if _, ok := seen[n]; ok {
				continue
			}
			pr, err := c.GetPR(ctx, owner, repo, n)
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					continue
				}
				return nil, fmt.Errorf("GetPR #%d: %w", n, err)
			}
			short := cm.SHA
			if len(short) > 8 {
				short = short[:8]
			}
			fmt.Fprintf(stderr, "INFO: PR #%d for %s/%s inferred from commit message (API returned no linked PRs for %s)\n", n, owner, repo, short)
			seen[n] = pr
		}
	}
	out := make([]PR, 0, len(seen))
	for _, p := range seen {
		out = append(out, p)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].MergedAt.Equal(out[j].MergedAt) {
			return out[i].MergedAt.Before(out[j].MergedAt)
		}
		return out[i].Number < out[j].Number
	})
	return out, nil
}
