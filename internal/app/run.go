// Package app is the orchestration root. It composes the gh client, the
// values parser, the diff/group passes, and the renderer into a single
// Run function consumed by cmd/release-notes.
package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/matthyx/release-notes/internal/diff"
	"github.com/matthyx/release-notes/internal/gh"
	"github.com/matthyx/release-notes/internal/render"
	"github.com/matthyx/release-notes/internal/values"
)

// stderr is the package log sink. Tests redirect.
var stderr io.Writer = os.Stderr

const (
	chartOwner    = "kubescape"
	chartRepo     = "helm-charts"
	chartName     = "kubescape-operator"
	valuesPath    = "charts/kubescape-operator/values.yaml"
	defaultIntro  = "Kubescape is an E2E Kubernetes cluster security platform"
	maxComponents = 4
)

// chartTagRE enforces the chart-tag prefix shape.
var chartTagRE = regexp.MustCompile(`^kubescape-operator-\d+\.\d+\.\d+(-.*)?$`)

// repoURLRE pulls the owner/repo from a canonical GitHub URL.
var repoURLRE = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+)$`)

// Run renders release notes for the given new chart tag. Output is written
// to w; logs and warnings go to stderr.
func Run(ctx context.Context, w io.Writer, c gh.Client, newChartTag string) error {
	if !chartTagRE.MatchString(newChartTag) {
		return fmt.Errorf("invalid chart tag %q (expected kubescape-operator-N.N.N[-suffix])", newChartTag)
	}

	releases, err := c.ListReleases(ctx, chartOwner, chartRepo)
	if err != nil {
		return fmt.Errorf("list releases: %w", err)
	}
	oldChartTag, err := gh.FindPreviousTag(releases, chartName, newChartTag)
	if err != nil {
		return err
	}

	// Two values.yaml fetches in parallel.
	var oldBytes, newBytes []byte
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		b, err := c.GetContent(gctx, chartOwner, chartRepo, valuesPath, oldChartTag)
		if err != nil {
			return fmt.Errorf("get old values.yaml: %w", err)
		}
		oldBytes = b
		return nil
	})
	g.Go(func() error {
		b, err := c.GetContent(gctx, chartOwner, chartRepo, valuesPath, newChartTag)
		if err != nil {
			return fmt.Errorf("get new values.yaml: %w", err)
		}
		newBytes = b
		return nil
	})
	if err := g.Wait(); err != nil {
		return err
	}

	oldComps, err := values.Parse(oldBytes)
	if err != nil {
		return fmt.Errorf("parse old values.yaml: %w", err)
	}
	newComps, err := values.Parse(newBytes)
	if err != nil {
		return fmt.Errorf("parse new values.yaml: %w", err)
	}

	bumps := diff.Compute(oldComps, newComps)
	groups := diff.Group(bumps)

	// Direct chart PRs.
	chartCmp, err := c.Compare(ctx, chartOwner, chartRepo, oldChartTag, newChartTag)
	if err != nil {
		return fmt.Errorf("compare chart: %w", err)
	}
	directPRs, err := gh.ExtractPRs(ctx, c, chartOwner, chartRepo, chartCmp.Commits)
	if err != nil {
		return fmt.Errorf("extract chart PRs: %w", err)
	}

	// Per-component compares with bounded concurrency.
	componentBumps := make([]render.ComponentBump, 0, len(groups))
	sourcelessGroups := make([]diff.BumpGroup, 0)
	type result struct {
		idx int
		prs []gh.PR
		err error
	}
	type job struct {
		idx   int
		group diff.BumpGroup
	}
	var sourcedJobs []job
	for _, gr := range groups {
		if gr.Skip {
			sourcelessGroups = append(sourcelessGroups, gr)
			continue
		}
		idx := len(componentBumps)
		componentBumps = append(componentBumps, render.ComponentBump{Group: gr})
		sourcedJobs = append(sourcedJobs, job{idx: idx, group: gr})
	}

	results := make([]result, len(sourcedJobs))
	cg, cctx := errgroup.WithContext(ctx)
	cg.SetLimit(maxComponents)
	var mu sync.Mutex
	for i, j := range sourcedJobs {
		i, j := i, j
		cg.Go(func() error {
			owner, repo, ok := parseRepo(j.group.SourceURL)
			if !ok {
				return fmt.Errorf("invalid source URL %q", j.group.SourceURL)
			}
			cmp, err := c.Compare(cctx, owner, repo, j.group.OldTag, j.group.NewTag)
			if err != nil {
				mu.Lock()
				results[i] = result{idx: j.idx, err: err}
				mu.Unlock()
				return nil // degrade, do not cancel siblings.
			}
			prs, err := gh.ExtractPRs(cctx, c, owner, repo, cmp.Commits)
			if err != nil {
				mu.Lock()
				results[i] = result{idx: j.idx, err: err}
				mu.Unlock()
				return nil
			}
			mu.Lock()
			results[i] = result{idx: j.idx, prs: prs}
			mu.Unlock()
			return nil
		})
	}
	if err := cg.Wait(); err != nil {
		return err
	}

	// Apply results, demoting failed groups to plain-bump bullets.
	final := make([]render.ComponentBump, 0, len(componentBumps))
	for i, cb := range componentBumps {
		var r result
		// find result for this idx
		for _, rr := range results {
			if rr.idx == i {
				r = rr
				break
			}
		}
		if r.err != nil {
			fmt.Fprintf(stderr, "WARN: compare failed for %s (%s...%s): %v — degrading to plain bump\n",
				cb.Group.SourceURL, cb.Group.OldTag, cb.Group.NewTag, r.err)
			demoted := cb.Group
			demoted.Skip = true
			if demoted.DisplayName == "" && len(demoted.Members) > 0 {
				demoted.DisplayName = demoted.Members[0].DisplayName
			}
			sourcelessGroups = append(sourcelessGroups, demoted)
			continue
		}
		cb.PRs = r.prs
		final = append(final, cb)
	}

	model := render.Model{
		Intro:           defaultIntro,
		ChartOwner:      chartOwner,
		ChartRepo:       chartRepo,
		OldChartTag:     oldChartTag,
		NewChartTag:     newChartTag,
		DirectPRs:       directPRs,
		ComponentBumps:  final,
		SourcelessBumps: sourcelessGroups,
	}
	out := render.Render(model)
	if _, err := fmt.Fprint(w, out); err != nil {
		return err
	}
	return nil
}

// parseRepo extracts the (owner, repo) pair from a canonical
// `https://github.com/owner/repo` URL. Returns ok=false on any other shape.
func parseRepo(srcURL string) (string, string, bool) {
	srcURL = strings.TrimSuffix(srcURL, "/")
	m := repoURLRE.FindStringSubmatch(srcURL)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}
