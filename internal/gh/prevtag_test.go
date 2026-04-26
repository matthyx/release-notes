package gh

import (
	"errors"
	"testing"
	"time"
)

func relAt(tag string, t time.Time) Release { return Release{TagName: tag, CreatedAt: t} }

func TestFindPreviousTag_Basic(t *testing.T) {
	rels := []Release{
		relAt("kubescape-operator-1.30.5", time.Unix(5, 0)),
		relAt("kubescape-operator-1.30.4", time.Unix(4, 0)),
		relAt("kubescape-operator-1.30.3", time.Unix(3, 0)),
	}
	got, err := FindPreviousTag(rels, "kubescape-operator", "kubescape-operator-1.30.5")
	if err != nil {
		t.Fatalf("FindPreviousTag: %v", err)
	}
	if got != "kubescape-operator-1.30.4" {
		t.Errorf("got %q, want kubescape-operator-1.30.4", got)
	}
}

func TestFindPreviousTag_OutOfOrderCreatedAt(t *testing.T) {
	// 1.30.4 came AFTER 1.30.5 chronologically (e.g. patch backport).
	rels := []Release{
		relAt("kubescape-operator-1.30.5", time.Unix(1, 0)),
		relAt("kubescape-operator-1.30.4", time.Unix(99, 0)),
	}
	got, err := FindPreviousTag(rels, "kubescape-operator", "kubescape-operator-1.30.5")
	if err != nil || got != "kubescape-operator-1.30.4" {
		t.Errorf("semver wins over created_at: got=%q err=%v", got, err)
	}
}

func TestFindPreviousTag_StableExcludesPrerelease(t *testing.T) {
	rels := []Release{
		relAt("kubescape-operator-1.30.5", time.Unix(5, 0)),
		relAt("kubescape-operator-1.30.5-rc.1", time.Unix(4, 0)),
		relAt("kubescape-operator-1.30.4", time.Unix(3, 0)),
	}
	got, err := FindPreviousTag(rels, "kubescape-operator", "kubescape-operator-1.30.5")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "kubescape-operator-1.30.4" {
		t.Errorf("got %q, want 1.30.4 (rc must be excluded)", got)
	}
}

func TestFindPreviousTag_PrereleaseInputAcceptsPrerelease(t *testing.T) {
	rels := []Release{
		relAt("kubescape-operator-1.30.5-rc.2", time.Unix(5, 0)),
		relAt("kubescape-operator-1.30.5-rc.1", time.Unix(4, 0)),
		relAt("kubescape-operator-1.30.4", time.Unix(3, 0)),
	}
	got, err := FindPreviousTag(rels, "kubescape-operator", "kubescape-operator-1.30.5-rc.2")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "kubescape-operator-1.30.5-rc.1" {
		t.Errorf("got %q, want rc.1", got)
	}
}

func TestFindPreviousTag_TypoTagDropped(t *testing.T) {
	// `kubescape-operator-0.29.6` is valid semver; we want a typo that is not.
	rels := []Release{
		relAt("kubescape-operator-1.30.4", time.Unix(4, 0)),
		// Suffix `1.30.bad` is non-semver; FindPreviousTag should drop it.
		relAt("kubescape-operator-1.30.bad", time.Unix(99, 0)),
	}
	got, err := FindPreviousTag(rels, "kubescape-operator", "kubescape-operator-1.30.5")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "kubescape-operator-1.30.4" {
		t.Errorf("got %q, want 1.30.4", got)
	}
}

func TestFindPreviousTag_NoCandidate(t *testing.T) {
	rels := []Release{
		relAt("kubescape-operator-1.30.5", time.Unix(5, 0)),
	}
	_, err := FindPreviousTag(rels, "kubescape-operator", "kubescape-operator-1.30.5")
	if !errors.Is(err, ErrNoPredecessor) {
		t.Errorf("want ErrNoPredecessor, got %v", err)
	}
}

func TestFindPreviousTag_BadInput(t *testing.T) {
	_, err := FindPreviousTag(nil, "kubescape-operator", "totally-bogus")
	if !errors.Is(err, ErrNoPredecessor) {
		t.Errorf("want ErrNoPredecessor, got %v", err)
	}
}

func TestFindPreviousTag_FiltersOtherCharts(t *testing.T) {
	rels := []Release{
		relAt("kubescape-operator-1.30.4", time.Unix(4, 0)),
		relAt("other-chart-9.9.9", time.Unix(99, 0)),
	}
	got, err := FindPreviousTag(rels, "kubescape-operator", "kubescape-operator-1.30.5")
	if err != nil || got != "kubescape-operator-1.30.4" {
		t.Errorf("got %q err %v", got, err)
	}
}
