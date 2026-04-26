// Package render turns a fully-resolved release-notes Model into the
// markdown body of a GitHub release. Output is LF-only and intentionally
// matches the upstream format byte-for-byte.
package render

import (
	"strings"

	"github.com/matthyx/release-notes/internal/diff"
	"github.com/matthyx/release-notes/internal/gh"
)

// ComponentBump pairs a BumpGroup with the PRs discovered in its compare
// range. Skip=true groups (sourceless or latest-tag) carry no PRs.
type ComponentBump struct {
	Group diff.BumpGroup
	PRs   []gh.PR
}

// Model is the input to Render. The orchestrator (internal/app) is
// responsible for assembling it.
type Model struct {
	Intro           string
	ChartOwner      string
	ChartRepo       string
	OldChartTag     string
	NewChartTag     string
	DirectPRs       []gh.PR
	ComponentBumps  []ComponentBump
	SourcelessBumps []diff.BumpGroup
}

// Render produces the markdown body. Output is LF-only and ends with a
// trailing newline.
func Render(m Model) string {
	var b strings.Builder
	if m.Intro != "" {
		b.WriteString(m.Intro)
		b.WriteString("\n\n")
	}
	b.WriteString("## What's Changed\n")

	hasContent := len(m.DirectPRs) > 0 || len(m.ComponentBumps) > 0 || len(m.SourcelessBumps) > 0
	if !hasContent {
		b.WriteString("(no changes detected)\n")
	} else {
		for _, p := range m.DirectPRs {
			writeBullet(&b, "* ", prLine(p))
		}
		for _, cb := range m.ComponentBumps {
			b.WriteString("* ")
			b.WriteString(cb.Group.SourceURL)
			b.WriteString("/compare/")
			b.WriteString(cb.Group.OldTag)
			b.WriteString("...")
			b.WriteString(cb.Group.NewTag)
			b.WriteByte('\n')
			for _, pr := range cb.PRs {
				writeBullet(&b, "  * ", prLine(pr))
			}
		}
		for _, sg := range m.SourcelessBumps {
			name := sg.DisplayName
			if name == "" && len(sg.Members) > 0 {
				name = sg.Members[0].DisplayName
			}
			b.WriteString("* Bump ")
			b.WriteString(name)
			b.WriteString(" from ")
			b.WriteString(sg.OldTag)
			b.WriteString(" to ")
			b.WriteString(sg.NewTag)
			b.WriteByte('\n')
		}
	}
	b.WriteString("\n**Full Changelog**: https://github.com/")
	b.WriteString(m.ChartOwner)
	b.WriteByte('/')
	b.WriteString(m.ChartRepo)
	b.WriteString("/compare/")
	b.WriteString(m.OldChartTag)
	b.WriteString("...")
	b.WriteString(m.NewChartTag)
	b.WriteByte('\n')
	return b.String()
}

func writeBullet(b *strings.Builder, prefix, text string) {
	b.WriteString(prefix)
	b.WriteString(text)
	b.WriteByte('\n')
}

func prLine(p gh.PR) string {
	return p.Title + " by @" + p.User.Login + " in " + p.HTMLURL
}
