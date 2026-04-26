// Package diff turns a pair of parsed values.yaml component lists into a
// list of bumps and groups them by source repo + tag pair for the renderer.
package diff

import (
	"path"
	"sort"

	"github.com/matthyx/release-notes/internal/values"
)

// Bump is a single image-tag change between the old and new values.yaml.
type Bump struct {
	Name        string
	DisplayName string // path.Base(ImageRepo); used for plain-bump bullets.
	ImageRepo   string
	OldTag      string
	NewTag      string
	SourceURL   string // canonical https://github.com/owner/repo, or "".
}

// IsSourceless reports whether a bump has no associated GitHub source repo.
func (b Bump) IsSourceless() bool { return b.SourceURL == "" }

// Compute returns the bumps that occur between two component slices. A bump
// is emitted only when both sides contain the component AND the tags differ.
// Output is stably sorted by Name for deterministic downstream rendering.
func Compute(oldC, newC []values.Component) []Bump {
	oldByName := make(map[string]values.Component, len(oldC))
	for _, c := range oldC {
		oldByName[c.Name] = c
	}
	var out []Bump
	for _, n := range newC {
		o, ok := oldByName[n.Name]
		if !ok {
			continue
		}
		if o.Tag == n.Tag {
			continue
		}
		out = append(out, Bump{
			Name:        n.Name,
			DisplayName: path.Base(n.ImageRepo),
			ImageRepo:   n.ImageRepo,
			OldTag:      o.Tag,
			NewTag:      n.Tag,
			SourceURL:   n.SourceURL,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
