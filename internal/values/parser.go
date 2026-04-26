// Package values parses kubescape-operator helm chart values.yaml files
// to extract image bumps and the GitHub source URLs annotated in YAML
// comments.
package values

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Component describes a single image: { repository, tag } block discovered
// in the values.yaml tree, plus the canonical GitHub repo URL extracted from
// the surrounding `# -- source code: https://github.com/owner/repo` comment
// (if any).
type Component struct {
	Name      string // immediate parent map key of the image: block
	ImageRepo string // image.repository value
	Tag       string // image.tag value
	SourceURL string // canonical https://github.com/owner/repo, or "" if none
	Path      string // dotted path for diagnostics, e.g. "nodeAgent.image"
}

// stderr is the package-level log sink. Tests redirect it.
var stderr io.Writer = os.Stderr

// srcRE matches the documented source-code comment forms:
//
//	# -- source code: https://github.com/owner/repo[/tree/...] (public repo)
//	# source code - https://github.com/owner/repo
var srcRE = regexp.MustCompile(`(?i)#?\s*-{0,2}\s*source\s+code\s*[:\-]\s*(https?://github\.com/[^\s)]+)`)

// maxImageDepth caps the depth at which we accept an image: mapping.
// Root document is depth 0; immediate top-level keys are depth 1, so:
//
//	componentName.image          → depth 2
//	componentName.subKey.image   → depth 3
//	componentName.k1.k2.image    → depth 4 (skipped)
const maxImageDepth = 3

// Parse walks the provided YAML bytes and returns one Component for every
// image: { repository, tag } mapping found at depth ≤ 3.
func Parse(b []byte) ([]Component, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("yaml: %w", err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, fmt.Errorf("empty yaml document")
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("yaml root is not a mapping")
	}
	var out []Component
	walk(root, "", "", 1, &out)
	return out, nil
}

// walk performs a depth-first traversal of the mapping tree, emitting a
// Component when it encounters an `image:` key whose value carries both
// `repository` and `tag` and whose depth is within the cutoff.
func walk(n *yaml.Node, parentName, parentPath string, depth int, out *[]Component) {
	if n == nil || n.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key := n.Content[i]
		val := n.Content[i+1]
		if key.Kind != yaml.ScalarNode {
			continue
		}
		path := key.Value
		if parentPath != "" {
			path = parentPath + "." + key.Value
		}
		if key.Value == "image" && val.Kind == yaml.MappingNode {
			if depth <= maxImageDepth {
				if comp, ok := imageComponent(parentName, path, key, val); ok {
					*out = append(*out, comp)
				}
			}
			// Don't recurse further into the image mapping itself.
			continue
		}
		if val.Kind == yaml.MappingNode {
			walk(val, key.Value, path, depth+1, out)
		}
	}
}

// imageComponent extracts a Component from an `image:` mapping node. Returns
// ok=false when the mapping lacks repository/tag (or tag is empty).
func imageComponent(name, path string, keyNode, valNode *yaml.Node) (Component, bool) {
	var (
		repo, tag                        string
		repoKey, repoVal, tagKey, tagVal *yaml.Node
	)
	for i := 0; i+1 < len(valNode.Content); i += 2 {
		k := valNode.Content[i]
		v := valNode.Content[i+1]
		if k.Kind != yaml.ScalarNode || v.Kind != yaml.ScalarNode {
			continue
		}
		switch k.Value {
		case "repository":
			repo = v.Value
			repoKey = k
			repoVal = v
		case "tag":
			tag = v.Value
			tagKey = k
			tagVal = v
		}
	}
	if repo == "" || tag == "" {
		return Component{}, false
	}
	src := extractSource(keyNode, valNode, repoKey, repoVal, tagKey, tagVal)
	if src == "" {
		fmt.Fprintf(stderr, "WARN: no source URL found for component %q (image: %s) — will emit plain bump\n", name, repo)
	}
	return Component{
		Name:      name,
		ImageRepo: repo,
		Tag:       tag,
		SourceURL: src,
		Path:      path,
	}, true
}

// extractSource scans the head/line/foot comments attached to the image mapping
// and its repository/tag child nodes (both key and value) for the first source-
// code URL match, then canonicalizes it.
func extractSource(nodes ...*yaml.Node) string {
	candidates := []string{}
	for _, n := range nodes {
		if n == nil {
			continue
		}
		if n.HeadComment != "" {
			candidates = append(candidates, n.HeadComment)
		}
		if n.LineComment != "" {
			candidates = append(candidates, n.LineComment)
		}
		if n.FootComment != "" {
			candidates = append(candidates, n.FootComment)
		}
	}
	for _, c := range candidates {
		for _, line := range strings.Split(c, "\n") {
			if m := srcRE.FindStringSubmatch(line); m != nil {
				return canonicalizeRepoURL(m[1])
			}
		}
	}
	return ""
}

// canonicalizeRepoURL trims tracking suffixes (`(public repo)`, trailing
// punctuation, `.git`, `/tree/...`) and returns the canonical
// `https://github.com/owner/repo` form.
func canonicalizeRepoURL(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimRight(raw, ".,);")
	raw = strings.TrimSuffix(raw, ".git")
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return ""
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host) + "/" + parts[0] + "/" + parts[1]
}
