package gh

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/mod/semver"
)

// FindPreviousTag returns the tag that immediately precedes newTag (in semver
// order) among the given releases, restricted to those whose tag name has the
// `<chartName>-<semver>` shape. Prereleases are excluded from candidates when
// newTag is itself a stable release; included when newTag is a prerelease.
func FindPreviousTag(releases []Release, chartName, newTag string) (string, error) {
	prefixRE, err := regexp.Compile(`^` + regexp.QuoteMeta(chartName) + `-\d+\.\d+\.\d+(-.*)?$`)
	if err != nil {
		return "", fmt.Errorf("compile prefix: %w", err)
	}
	if !prefixRE.MatchString(newTag) {
		return "", fmt.Errorf("%w: malformed tag %q", ErrNoPredecessor, newTag)
	}
	newSuffix := strings.TrimPrefix(newTag, chartName+"-")
	newSemver := "v" + newSuffix
	if !semver.IsValid(newSemver) {
		return "", fmt.Errorf("%w: %q is not valid semver", ErrNoPredecessor, newTag)
	}
	newIsPrerelease := semver.Prerelease(newSemver) != ""

	type cand struct {
		tag    string
		semver string
	}
	var cands []cand
	for _, r := range releases {
		if !prefixRE.MatchString(r.TagName) {
			continue
		}
		suffix := strings.TrimPrefix(r.TagName, chartName+"-")
		sv := "v" + suffix
		if !semver.IsValid(sv) {
			continue
		}
		if !newIsPrerelease && semver.Prerelease(sv) != "" {
			continue
		}
		// Exclude the input tag itself and anything not strictly less.
		if semver.Compare(sv, newSemver) >= 0 {
			continue
		}
		cands = append(cands, cand{tag: r.TagName, semver: sv})
	}
	if len(cands) == 0 {
		return "", fmt.Errorf("%w: no candidate before %q", ErrNoPredecessor, newTag)
	}
	sort.Slice(cands, func(i, j int) bool {
		// Descending semver — first element is the immediate predecessor.
		return semver.Compare(cands[i].semver, cands[j].semver) > 0
	})
	return cands[0].tag, nil
}
