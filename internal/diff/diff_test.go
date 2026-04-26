package diff

import (
	"testing"

	"github.com/matthyx/release-notes/internal/values"
)

func comp(name, repo, tag, src string) values.Component {
	return values.Component{Name: name, ImageRepo: repo, Tag: tag, SourceURL: src}
}

func TestCompute_NoChange(t *testing.T) {
	old := []values.Component{comp("a", "quay.io/x/a", "v1", "https://github.com/x/a")}
	new := []values.Component{comp("a", "quay.io/x/a", "v1", "https://github.com/x/a")}
	if got := Compute(old, new); len(got) != 0 {
		t.Fatalf("want 0 bumps, got %v", got)
	}
}

func TestCompute_SingleSourced(t *testing.T) {
	old := []values.Component{comp("a", "quay.io/x/a", "v1", "https://github.com/x/a")}
	new := []values.Component{comp("a", "quay.io/x/a", "v2", "https://github.com/x/a")}
	got := Compute(old, new)
	if len(got) != 1 {
		t.Fatalf("want 1, got %v", got)
	}
	b := got[0]
	if b.Name != "a" || b.OldTag != "v1" || b.NewTag != "v2" || b.DisplayName != "a" {
		t.Errorf("bad bump: %+v", b)
	}
	if b.IsSourceless() {
		t.Errorf("should be sourced")
	}
}

func TestCompute_SingleSourceless(t *testing.T) {
	old := []values.Component{comp("b", "docker.io/library/busybox", "1.0", "")}
	new := []values.Component{comp("b", "docker.io/library/busybox", "1.1", "")}
	got := Compute(old, new)
	if len(got) != 1 {
		t.Fatalf("want 1, got %v", got)
	}
	if got[0].DisplayName != "busybox" {
		t.Errorf("DisplayName = %q, want busybox", got[0].DisplayName)
	}
	if !got[0].IsSourceless() {
		t.Errorf("should be sourceless")
	}
}

func TestCompute_AddedRemoved(t *testing.T) {
	old := []values.Component{comp("a", "quay.io/x/a", "v1", "u")}
	new := []values.Component{comp("b", "quay.io/x/b", "v1", "u")}
	if got := Compute(old, new); len(got) != 0 {
		t.Fatalf("added/removed should be ignored, got %v", got)
	}
}

func TestGroup_DedupSameTriple(t *testing.T) {
	bumps := []Bump{
		{Name: "k1", DisplayName: "http-request", ImageRepo: "quay.io/kubescape/http-request", OldTag: "v1", NewTag: "v2", SourceURL: "https://github.com/kubescape/http-request"},
		{Name: "k2", DisplayName: "http-request", ImageRepo: "quay.io/kubescape/http-request", OldTag: "v1", NewTag: "v2", SourceURL: "https://github.com/kubescape/http-request"},
		{Name: "k3", DisplayName: "http-request", ImageRepo: "quay.io/kubescape/http-request", OldTag: "v1", NewTag: "v2", SourceURL: "https://github.com/kubescape/http-request"},
		{Name: "k4", DisplayName: "http-request", ImageRepo: "quay.io/kubescape/http-request", OldTag: "v1", NewTag: "v2", SourceURL: "https://github.com/kubescape/http-request"},
	}
	groups := Group(bumps)
	if len(groups) != 1 {
		t.Fatalf("want 1 group, got %d: %+v", len(groups), groups)
	}
	if len(groups[0].Members) != 4 {
		t.Errorf("want 4 members, got %d", len(groups[0].Members))
	}
	if groups[0].Skip {
		t.Errorf("group should not be Skip")
	}
}

func TestGroup_MultiRange(t *testing.T) {
	bumps := []Bump{
		{Name: "node-agent", DisplayName: "node-agent", ImageRepo: "n", OldTag: "v0.3.42", NewTag: "v0.3.47", SourceURL: "https://github.com/kubescape/node-agent"},
		{Name: "clamav", DisplayName: "klamav", ImageRepo: "c", OldTag: "1.3.1", NewTag: "1.3.2", SourceURL: "https://github.com/kubescape/node-agent"},
	}
	groups := Group(bumps)
	if len(groups) != 2 {
		t.Fatalf("want 2 groups, got %d", len(groups))
	}
}

func TestGroup_LatestSkipped(t *testing.T) {
	bumps := []Bump{
		{Name: "x", DisplayName: "x", ImageRepo: "x", OldTag: "v1", NewTag: "latest", SourceURL: "https://github.com/x/x"},
	}
	groups := Group(bumps)
	if len(groups) != 1 || !groups[0].Skip {
		t.Fatalf("want one Skip group, got %+v", groups)
	}
}

func TestGroup_LatestLatestDropped(t *testing.T) {
	bumps := []Bump{
		{Name: "x", DisplayName: "x", ImageRepo: "x", OldTag: "latest", NewTag: "latest"},
	}
	if g := Group(bumps); len(g) != 0 {
		t.Fatalf("latest→latest should drop, got %+v", g)
	}
}

func TestGroup_SourcelessDedup(t *testing.T) {
	bumps := []Bump{
		{Name: "a", DisplayName: "busybox", ImageRepo: "docker.io/library/busybox", OldTag: "1.0", NewTag: "1.1"},
		{Name: "b", DisplayName: "busybox", ImageRepo: "docker.io/library/busybox", OldTag: "1.0", NewTag: "1.1"},
		// distinct ImageRepo, same tags → separate group
		{Name: "c", DisplayName: "alpine", ImageRepo: "alpine", OldTag: "1.0", NewTag: "1.1"},
	}
	g := Group(bumps)
	if len(g) != 2 {
		t.Fatalf("want 2 sourceless groups, got %d (%+v)", len(g), g)
	}
	for _, gr := range g {
		if !gr.Skip {
			t.Errorf("sourceless group must be Skip")
		}
	}
}
