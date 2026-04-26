package diff

// BumpGroup is a set of Bumps that share the same (SourceURL, OldTag, NewTag)
// triple — the unit the renderer turns into one compare-URL bullet.
//
// Sourceless bumps are still grouped by (ImageRepo, OldTag, NewTag) so that
// duplicate sourceless components produce a single `* Bump X from A to B`
// line. Such groups carry Skip=true so the renderer routes them through the
// plain-bump path instead of attempting a `/compare/` URL.
type BumpGroup struct {
	SourceURL   string
	OldTag      string
	NewTag      string
	DisplayName string // populated for sourceless / Skip groups
	Members     []Bump
	Skip        bool
}

// Group collapses a slice of Bumps into the minimal set of BumpGroups for
// rendering. Insertion order from the (already sorted) bumps slice is
// preserved so that output is deterministic.
func Group(bumps []Bump) []BumpGroup {
	type key struct {
		src, oldT, newT, display string
	}
	idx := make(map[key]int)
	var groups []BumpGroup
	for _, b := range bumps {
		// Skip latest→latest pairs entirely — there's nothing to compare.
		if b.OldTag == "latest" && b.NewTag == "latest" {
			continue
		}
		k := key{src: b.SourceURL, oldT: b.OldTag, newT: b.NewTag}
		skip := false
		display := ""
		if b.SourceURL == "" {
			// Sourceless: dedupe by (ImageRepo, OldTag, NewTag).
			k.display = b.ImageRepo
			skip = true
			display = b.DisplayName
		} else if b.OldTag == "latest" || b.NewTag == "latest" {
			// One side is latest: we cannot build a usable compare URL.
			skip = true
			display = b.DisplayName
		}
		if i, ok := idx[k]; ok {
			groups[i].Members = append(groups[i].Members, b)
			continue
		}
		idx[k] = len(groups)
		groups = append(groups, BumpGroup{
			SourceURL:   b.SourceURL,
			OldTag:      b.OldTag,
			NewTag:      b.NewTag,
			DisplayName: display,
			Members:     []Bump{b},
			Skip:        skip,
		})
	}
	return groups
}
