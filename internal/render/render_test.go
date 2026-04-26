package render

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matthyx/release-notes/internal/diff"
	"github.com/matthyx/release-notes/internal/gh"
)

var update = flag.Bool("update", false, "rewrite golden files")

// pr is a small constructor used in tests.
func pr(title, login, urlStr string) gh.PR {
	var p gh.PR
	p.Title = title
	p.User.Login = login
	p.HTMLURL = urlStr
	return p
}

func TestRender_Golden(t *testing.T) {
	model := buildGoldenModel()
	got := Render(model)
	if strings.Contains(got, "\r") {
		t.Fatal("output contains CR byte")
	}
	path := filepath.Join("testdata", "golden_kubescape-operator-1.30.5.md")
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if got != string(want) {
		t.Errorf("render mismatch\n--- got ---\n%s\n--- want ---\n%s\n", got, string(want))
	}
}

func TestRender_NoChanges(t *testing.T) {
	got := Render(Model{
		Intro:       "i",
		ChartOwner:  "o",
		ChartRepo:   "r",
		OldChartTag: "v1",
		NewChartTag: "v2",
	})
	if !strings.Contains(got, "(no changes detected)") {
		t.Errorf("missing empty placeholder, got %q", got)
	}
}

func TestRender_SourcelessBumps(t *testing.T) {
	m := Model{
		ChartOwner:  "o",
		ChartRepo:   "r",
		OldChartTag: "v1",
		NewChartTag: "v2",
		SourcelessBumps: []diff.BumpGroup{
			{DisplayName: "klamav", OldTag: "1.0", NewTag: "1.1", Skip: true},
		},
	}
	got := Render(m)
	if !strings.Contains(got, "* Bump klamav from 1.0 to 1.1") {
		t.Errorf("missing plain bump, got %q", got)
	}
}

// buildGoldenModel hand-builds the Model that the renderer must turn into the
// golden file. Data lifted from the live kubescape-operator-1.30.5 release.
func buildGoldenModel() Model {
	directPRs := []gh.PR{
		pr("Update values.yaml", "Naor-Armo", "https://github.com/kubescape/helm-charts/pull/799"),
	}

	kubevulnGroup := diff.BumpGroup{
		SourceURL: "https://github.com/kubescape/kubevuln",
		OldTag:    "v0.3.105",
		NewTag:    "v0.3.109",
	}
	storageGroup := diff.BumpGroup{
		SourceURL: "https://github.com/kubescape/storage",
		OldTag:    "v0.0.239",
		NewTag:    "v0.0.247",
	}
	nodeAgentGroup := diff.BumpGroup{
		SourceURL: "https://github.com/kubescape/node-agent",
		OldTag:    "v0.3.42",
		NewTag:    "v0.3.47",
	}
	syncGroup := diff.BumpGroup{
		SourceURL: "https://github.com/kubescape/synchronizer",
		OldTag:    "v0.0.131",
		NewTag:    "v0.0.132",
	}

	return Model{
		Intro:       "Kubescape is an E2E Kubernetes cluster security platform",
		ChartOwner:  "kubescape",
		ChartRepo:   "helm-charts",
		OldChartTag: "kubescape-operator-1.30.4",
		NewChartTag: "kubescape-operator-1.30.5",
		DirectPRs:   directPRs,
		ComponentBumps: []ComponentBump{
			{
				Group: kubevulnGroup,
				PRs: []gh.PR{
					pr("Bump github.com/go-git/go-git/v5 from 5.16.2 to 5.16.5", "dependabot[bot]", "https://github.com/kubescape/kubevuln/pull/328"),
					pr("strip unnecessary fields from SBOM to reduce size", "matthyx", "https://github.com/kubescape/kubevuln/pull/327"),
					pr("fix test expectations", "matthyx", "https://github.com/kubescape/kubevuln/pull/329"),
					pr("use fixed StripSBOM from storage v0.0.247", "matthyx", "https://github.com/kubescape/kubevuln/pull/330"),
				},
			},
			{
				Group: storageGroup,
				PRs: []gh.PR{
					pr("add permissions", "bvolovat", "https://github.com/kubescape/storage/pull/281"),
					pr("Fix OpenAPI model names to use dot-notation instead of slash-notation", "matthyx", "https://github.com/kubescape/storage/pull/283"),
					pr("disable slug channel", "shanyl9", "https://github.com/kubescape/storage/pull/285"),
					pr("Fix key not found", "jnathangreeg", "https://github.com/kubescape/storage/pull/287"),
					pr("Implement StripSBOM function to reduce SBOM size by clearing unnecessary fields", "matthyx", "https://github.com/kubescape/storage/pull/288"),
					pr("Preserve relationships in StripSBOM function (needed by kubevuln's filterSBOM)", "matthyx", "https://github.com/kubescape/storage/pull/289"),
				},
			},
			{
				Group: nodeAgentGroup,
				PRs: []gh.PR{
					pr("Strip unused SBOM fields to reduce object size by ~52%", "slashben", "https://github.com/kubescape/node-agent/pull/720"),
					pr("use fixed StripSBOM from storage v0.0.247", "matthyx", "https://github.com/kubescape/node-agent/pull/726"),
					pr("bump github.com/goradd/maps v1.3.0", "matthyx", "https://github.com/kubescape/node-agent/pull/727"),
				},
			},
			{
				Group: syncGroup,
				PRs: []gh.PR{
					pr("fix: bump k8s-interface to v0.0.203 for OCI bare OCID detection", "rotemamsa", "https://github.com/kubescape/synchronizer/pull/140"),
				},
			},
		},
	}
}
