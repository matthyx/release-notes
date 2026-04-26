package values

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse_Edge(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "values_edge.yaml"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var sink bytes.Buffer
	old := stderr
	stderr = &sink
	defer func() { stderr = old }()

	got, err := Parse(b)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	byName := map[string]Component{}
	for _, c := range got {
		byName[c.Name] = c
	}

	cases := []struct {
		name      string
		want      string // expected SourceURL ("" for sourceless)
		wantTag   string
		wantImage string
	}{
		{"foo", "https://github.com/kubescape/foo", "v1.0.0", "quay.io/kubescape/foo"},
		{"baz", "https://github.com/kubescape/baz", "v0.1.0", "quay.io/kubescape/baz"},
		{"busybox", "", "1.36.1", "docker.io/library/busybox"},
		{"nodeAgent", "https://github.com/kubescape/node-agent", "v0.3.94", "quay.io/kubescape/node-agent"},
		{"startupJitterContainer", "", "latest", "busybox"},
	}
	for _, tc := range cases {
		c, ok := byName[tc.name]
		if !ok {
			t.Errorf("missing component %q (got: %v)", tc.name, componentNames(got))
			continue
		}
		if c.SourceURL != tc.want {
			t.Errorf("%s: SourceURL = %q, want %q", tc.name, c.SourceURL, tc.want)
		}
		if c.Tag != tc.wantTag {
			t.Errorf("%s: Tag = %q, want %q", tc.name, c.Tag, tc.wantTag)
		}
		if c.ImageRepo != tc.wantImage {
			t.Errorf("%s: ImageRepo = %q, want %q", tc.name, c.ImageRepo, tc.wantImage)
		}
	}

	// Depth-4 image must be skipped: nothing under deepNest should appear.
	for _, c := range got {
		if strings.HasPrefix(c.Path, "deepNest.") {
			t.Errorf("depth-4 image emitted: %+v", c)
		}
	}

	// Sourceless WARN log must fire for at least one component.
	if !strings.Contains(sink.String(), "WARN: no source URL") {
		t.Errorf("expected sourceless WARN, stderr=%q", sink.String())
	}
}

func TestParse_Old(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "values_old.yaml"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	got, err := Parse(b)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := map[string]string{ // name -> tag
		"kubescape":             "v3.0.48",
		"operator":              "v0.2.128",
		"kubevuln":              "v0.3.105",
		"hostScanner":           "v1.0.78",
		"storage":               "v0.0.239",
		"nodeAgent":             "v0.3.42",
		"clamav":                "1.3.1-34_alpha",
		"synchronizer":          "v0.0.131",
		"otelCollector":         "0.108.0",
		"prometheusExporter":    "v0.2.11",
		"helmReleaseUpgrader":   "v0.1.0",
		"kubescapeScheduler":    "v0.2.16",
		"kubevulnScheduler":     "v0.2.16",
		"registryScanScheduler": "v0.2.16",
	}
	byName := map[string]Component{}
	for _, c := range got {
		byName[c.Name] = c
	}
	for n, tag := range want {
		c, ok := byName[n]
		if !ok {
			t.Errorf("old: missing %q", n)
			continue
		}
		if c.Tag != tag {
			t.Errorf("old: %s tag = %q, want %q", n, c.Tag, tag)
		}
	}
}

func TestParse_New(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "values_new.yaml"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	got, err := Parse(b)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// Tags from the pinned kubescape-operator-1.30.5 fixture.
	want := map[string]string{
		"kubescape":             "v3.0.48",
		"operator":              "v0.2.128",
		"kubevuln":              "v0.3.109",
		"storage":               "v0.0.247",
		"nodeAgent":             "v0.3.47",
		"synchronizer":          "v0.0.132",
		"prometheusExporter":    "v0.2.11",
		"kubescapeScheduler":    "v0.2.16",
		"kubevulnScheduler":     "v0.2.16",
		"registryScanScheduler": "v0.2.16",
	}
	byName := map[string]Component{}
	for _, c := range got {
		byName[c.Name] = c
	}
	for n, tag := range want {
		c, ok := byName[n]
		if !ok {
			t.Errorf("new: missing %q", n)
			continue
		}
		if c.Tag != tag {
			t.Errorf("new: %s tag = %q, want %q", n, c.Tag, tag)
		}
	}
	// Sanity: storage should canonicalize to https://github.com/kubescape/storage.
	if c, ok := byName["storage"]; ok && c.SourceURL != "https://github.com/kubescape/storage" {
		t.Errorf("storage SourceURL = %q", c.SourceURL)
	}
	// Sanity: kubescape comment has /tree/master/httphandler — must be canonicalized.
	if c, ok := byName["kubescape"]; ok && c.SourceURL != "https://github.com/kubescape/kubescape" {
		t.Errorf("kubescape SourceURL = %q", c.SourceURL)
	}
}

func TestParse_Errors(t *testing.T) {
	if _, err := Parse([]byte("not: yaml: : :")); err == nil {
		t.Fatal("expected yaml parse error")
	}
}

func TestCanonicalizeRepoURL(t *testing.T) {
	cases := map[string]string{
		"https://github.com/owner/repo":                "https://github.com/owner/repo",
		"https://github.com/owner/repo/":               "https://github.com/owner/repo",
		"https://github.com/owner/repo.git":            "https://github.com/owner/repo",
		"https://github.com/owner/repo/tree/main/path": "https://github.com/owner/repo",
		"http://github.com/owner/repo":                 "http://github.com/owner/repo",
		"https://github.com/owner/repo);":              "https://github.com/owner/repo",
	}
	for in, want := range cases {
		if got := canonicalizeRepoURL(in); got != want {
			t.Errorf("canonicalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func componentNames(cs []Component) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.Name)
	}
	return out
}
