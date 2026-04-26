package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matthyx/release-notes/internal/gh"
)

// fakeClient implements gh.Client with canned data for the e2e test.
type fakeClient struct {
	releases []gh.Release
	contents map[string][]byte             // key: ref::path
	compare  map[string]gh.CompareResult   // key: owner/repo/base...head
	prsByCm  map[string]map[string][]gh.PR // key: owner/repo -> sha -> PRs
	prByNum  map[string]map[int]gh.PR      // key: owner/repo -> n -> PR
}

func (f *fakeClient) ListReleases(_ context.Context, _, _ string) ([]gh.Release, error) {
	return f.releases, nil
}
func (f *fakeClient) GetContent(_ context.Context, owner, repo, path, ref string) ([]byte, error) {
	_ = owner
	_ = repo
	key := ref + "::" + path
	if b, ok := f.contents[key]; ok {
		return b, nil
	}
	return nil, fmt.Errorf("not found: %s", key)
}
func (f *fakeClient) Compare(_ context.Context, owner, repo, base, head string) (gh.CompareResult, error) {
	key := owner + "/" + repo + "/" + base + "..." + head
	if v, ok := f.compare[key]; ok {
		return v, nil
	}
	return gh.CompareResult{}, errors.New("no compare for " + key)
}
func (f *fakeClient) PRsForCommit(_ context.Context, owner, repo, sha string) ([]gh.PR, error) {
	if m, ok := f.prsByCm[owner+"/"+repo]; ok {
		return m[sha], nil
	}
	return nil, nil
}
func (f *fakeClient) GetPR(_ context.Context, owner, repo string, n int) (gh.PR, error) {
	if m, ok := f.prByNum[owner+"/"+repo]; ok {
		if pr, ok := m[n]; ok {
			return pr, nil
		}
	}
	return gh.PR{}, gh.ErrNotFound
}

func TestRun_E2E(t *testing.T) {
	oldB, err := os.ReadFile(filepath.Join("..", "values", "testdata", "values_old.yaml"))
	if err != nil {
		t.Fatalf("read old: %v", err)
	}
	newB, err := os.ReadFile(filepath.Join("..", "values", "testdata", "values_new.yaml"))
	if err != nil {
		t.Fatalf("read new: %v", err)
	}

	mkPR := func(num int, title, login, urlStr string) gh.PR {
		var p gh.PR
		p.Number = num
		p.Title = title
		p.User.Login = login
		p.HTMLURL = urlStr
		return p
	}

	f := &fakeClient{
		releases: []gh.Release{
			{TagName: "kubescape-operator-1.30.5"},
			{TagName: "kubescape-operator-1.30.4"},
			{TagName: "kubescape-operator-1.30.3"},
		},
		contents: map[string][]byte{
			"kubescape-operator-1.30.4::charts/kubescape-operator/values.yaml": oldB,
			"kubescape-operator-1.30.5::charts/kubescape-operator/values.yaml": newB,
		},
		// Tags from the pinned 1.30.4→1.30.5 fixture files.
		// operator and http-request are unchanged between the two releases (same tag) so they
		// produce no BumpGroup and require no compare entry.
		compare: map[string]gh.CompareResult{
			"kubescape/helm-charts/kubescape-operator-1.30.4...kubescape-operator-1.30.5": {Commits: []gh.Commit{{SHA: "chart-sha-1"}}},
			"kubescape/kubevuln/v0.3.105...v0.3.109":                                      {Commits: []gh.Commit{{SHA: "kv1"}, {SHA: "kv2"}, {SHA: "kv3"}, {SHA: "kv4"}}},
			"kubescape/storage/v0.0.239...v0.0.247":                                       {Commits: []gh.Commit{{SHA: "st1"}, {SHA: "st2"}, {SHA: "st3"}, {SHA: "st4"}, {SHA: "st5"}, {SHA: "st6"}}},
			"kubescape/node-agent/v0.3.42...v0.3.47":                                      {Commits: []gh.Commit{{SHA: "na1"}, {SHA: "na2"}, {SHA: "na3"}}},
			"kubescape/synchronizer/v0.0.131...v0.0.132":                                  {Commits: []gh.Commit{{SHA: "sy1"}}},
		},
		prsByCm: map[string]map[string][]gh.PR{
			"kubescape/helm-charts": {
				"chart-sha-1": {mkPR(799, "Update values.yaml", "Naor-Armo", "https://github.com/kubescape/helm-charts/pull/799")},
			},
			"kubescape/kubevuln": {
				"kv1": {mkPR(328, "Bump github.com/go-git/go-git/v5 from 5.16.2 to 5.16.5", "dependabot[bot]", "https://github.com/kubescape/kubevuln/pull/328")},
				"kv2": {mkPR(327, "strip unnecessary fields from SBOM to reduce size", "matthyx", "https://github.com/kubescape/kubevuln/pull/327")},
				"kv3": {mkPR(329, "fix test expectations", "matthyx", "https://github.com/kubescape/kubevuln/pull/329")},
				"kv4": {mkPR(330, "use fixed StripSBOM from storage v0.0.247", "matthyx", "https://github.com/kubescape/kubevuln/pull/330")},
			},
			"kubescape/storage": {
				"st1": {mkPR(281, "add permissions", "bvolovat", "https://github.com/kubescape/storage/pull/281")},
				"st2": {mkPR(283, "Fix OpenAPI model names to use dot-notation instead of slash-notation", "matthyx", "https://github.com/kubescape/storage/pull/283")},
				"st3": {mkPR(285, "disable slug channel", "shanyl9", "https://github.com/kubescape/storage/pull/285")},
				"st4": {mkPR(287, "Fix key not found", "jnathangreeg", "https://github.com/kubescape/storage/pull/287")},
				"st5": {mkPR(288, "Implement StripSBOM function to reduce SBOM size by clearing unnecessary fields", "matthyx", "https://github.com/kubescape/storage/pull/288")},
				"st6": {mkPR(289, "Preserve relationships in StripSBOM function (needed by kubevuln's filterSBOM)", "matthyx", "https://github.com/kubescape/storage/pull/289")},
			},
			"kubescape/node-agent": {
				"na1": {mkPR(720, "Strip unused SBOM fields to reduce object size by ~52%", "slashben", "https://github.com/kubescape/node-agent/pull/720")},
				"na2": {mkPR(726, "use fixed StripSBOM from storage v0.0.247", "matthyx", "https://github.com/kubescape/node-agent/pull/726")},
				"na3": {mkPR(727, "bump github.com/goradd/maps v1.3.0", "matthyx", "https://github.com/kubescape/node-agent/pull/727")},
			},
			"kubescape/synchronizer": {
				"sy1": {mkPR(140, "fix: bump k8s-interface to v0.0.203 for OCI bare OCID detection", "rotemamsa", "https://github.com/kubescape/synchronizer/pull/140")},
			},
		},
	}

	// Suppress stderr noise from values WARN logs and our own logs.
	old := stderr
	var sink strings.Builder
	stderr = &sink
	defer func() { stderr = old }()

	var out strings.Builder
	if err := Run(context.Background(), &out, f, "kubescape-operator-1.30.5"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := out.String()

	if !strings.Contains(got, "Kubescape is an E2E Kubernetes cluster security platform") {
		t.Errorf("missing intro")
	}
	if !strings.Contains(got, "* Update values.yaml by @Naor-Armo in https://github.com/kubescape/helm-charts/pull/799") {
		t.Errorf("missing direct PR\n%s", got)
	}
	if !strings.Contains(got, "**Full Changelog**: https://github.com/kubescape/helm-charts/compare/kubescape-operator-1.30.4...kubescape-operator-1.30.5") {
		t.Errorf("missing Full Changelog")
	}
	// All four sourced compare bullets present (tags match 1.30.4→1.30.5 fixture).
	for _, want := range []string{
		"https://github.com/kubescape/kubevuln/compare/v0.3.105...v0.3.109",
		"https://github.com/kubescape/storage/compare/v0.0.239...v0.0.247",
		"https://github.com/kubescape/node-agent/compare/v0.3.42...v0.3.47",
		"https://github.com/kubescape/synchronizer/compare/v0.0.131...v0.0.132",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing compare bullet %q", want)
		}
	}
	// Each kubevuln PR present
	for _, want := range []string{
		"https://github.com/kubescape/kubevuln/pull/327",
		"https://github.com/kubescape/kubevuln/pull/328",
		"https://github.com/kubescape/kubevuln/pull/329",
		"https://github.com/kubescape/kubevuln/pull/330",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing kubevuln PR %q", want)
		}
	}

	// Sourceless / plain-bump must use DisplayName.
	for _, banned := range []string{
		"* Bump quay.io/kubescape/prometheus-exporter",
		"* Bump quay.io/kubescape/http-request",
	} {
		if strings.Contains(got, banned) {
			t.Errorf("plain bump must not contain registry path: %q", banned)
		}
	}

	// And ensure output never carries CR.
	if strings.Contains(got, "\r") {
		t.Fatal("CR byte in output")
	}
}

func TestRun_BadTag(t *testing.T) {
	if err := Run(context.Background(), &strings.Builder{}, &fakeClient{}, "bogus"); err == nil {
		t.Fatal("want error for bad tag")
	}
}
