package main

// review — the CI / pull-request surface. Runs the same code-axis gate as
// `check` and emits each NEW (un-adjudicated) suspect as a GitHub Actions
// workflow annotation (`::warning file=…,line=…::…`), so a reusable Action can
// surface dual-path drift inline on a PR with no hosted service in the loop.
// BYOK-friendly: the deterministic recall pass needs no API key; teams who want
// the precision half run `check`/the generators with their own key as a CI
// secret. Advisory by default (exit 0 — annotations never fail the build);
// `--strict` makes it exit 1 on new suspects for teams that want a hard gate.

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/justinstimatze/calque/internal/code"
)

func runReview(args []string) {
	fs := flag.NewFlagSet("review", flag.ContinueOnError)
	b := addBoundaryFlags(fs)
	repo, left, right, exclude := b.repo, b.left, b.right, b.exclude
	cf := addCheckFlags(fs)
	strict := fs.Bool("strict", false, "exit 1 if there are new (un-adjudicated) suspects (default: advisory, exit 0)")
	if err := fs.Parse(args); err != nil {
		return
	}

	_ = applyCalibratedWeights(*repo, *cf.noCalib)
	f, err := computeCheck(*repo, *left, *right, *exclude, *cf.minScore, *cf.minLines, *cf.clusterMinMembers, *cf.clusterMaxFanout, *cf.regPath, *cf.includeTests)
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque review: %v\n", err)
		os.Exit(1)
	}

	var b2 strings.Builder
	for _, w := range f.Bite {
		// A false-clean boundary is a CI-visible warning in its own right.
		fmt.Fprintln(&b2, ghAnnotation("warning", "", 0, w))
	}
	for _, s := range f.Fresh {
		emitPairAnnotations(&b2, s, *cf.regPath)
	}
	for _, c := range f.FreshC {
		emitClusterAnnotations(&b2, c, *cf.regPath)
	}
	fmt.Fprintln(&b2, ghAnnotation("notice", "", 0, fmt.Sprintf(
		"calque: %d new dual-path suspect(s), %d cluster(s) vs %s — advisory (recall-first); adjudicate to silence.",
		len(f.Fresh), len(f.FreshC), *cf.regPath)))
	fmt.Print(b2.String())
	writeStepSummary(f, *cf.regPath)

	if *strict && (len(f.Fresh) > 0 || len(f.FreshC) > 0) {
		os.Exit(1)
	}
}

// writeStepSummary appends the markdown panel to $GITHUB_STEP_SUMMARY when that
// env var is set (a no-op off CI). The file accumulates across a job's steps, so
// we append rather than truncate. The annotations land inline on the diff; this
// renders an at-a-glance table on the run's summary page.
func writeStepSummary(f checkFindings, regPath string) {
	path := os.Getenv("GITHUB_STEP_SUMMARY")
	if path == "" {
		return
	}
	fh, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque review: step summary: %v\n", err)
		return
	}
	defer fh.Close()
	fmt.Fprintln(fh, renderStepSummary(f, regPath))
}

// renderStepSummary formats the gate findings as a GitHub Actions job-summary
// markdown panel. Pure (no I/O) so it's testable; writeStepSummary does the
// env-check + append.
func renderStepSummary(f checkFindings, regPath string) string {
	var b strings.Builder
	b.WriteString("## calque drift review\n\n")
	for _, w := range f.Bite {
		fmt.Fprintf(&b, "> ⚠ %s\n\n", w)
	}
	if len(f.Fresh) == 0 && len(f.FreshC) == 0 {
		fmt.Fprintf(&b, "✓ No new dual-path drift vs `%s`.\n", regPath)
		return b.String()
	}
	fmt.Fprintf(&b, "**%d** new suspect pair(s) · **%d** new cluster(s) vs `%s` — advisory (recall-first); adjudicate to silence.\n",
		len(f.Fresh), len(f.FreshC), regPath)
	if len(f.Fresh) > 0 {
		b.WriteString("\n| score | suspect | twin of | signal |\n|------:|---------|---------|--------|\n")
		for _, s := range f.Fresh {
			fmt.Fprintf(&b, "| %.2f | `%s` (%s:%d) | `%s` (%s:%d) | %s |\n",
				s.Score, s.Left.Qualname, s.Left.File, s.Left.Line,
				s.Right.Qualname, s.Right.File, s.Right.Line, mdCell(s.Reason()))
		}
	}
	if len(f.FreshC) > 0 {
		b.WriteString("\n### Clusters\n\n| score | members | seam |\n|------:|--------:|------|\n")
		for _, c := range f.FreshC {
			fmt.Fprintf(&b, "| %.2f | %d | %s |\n", c.Score, len(c.Members), mdCell(c.Reason()))
		}
	}
	return b.String()
}

// mdCell makes a string safe inside a Markdown table cell: escape the pipe that
// would end the cell, and flatten any newline.
func mdCell(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.ReplaceAll(s, "|", "\\|")
}

// emitPairAnnotations writes one annotation per endpoint of a suspect pair, so
// the warning lands inline wherever the PR touched code: GitHub only renders
// annotations on changed lines, and anchoring at one arbitrary side would miss
// the diff when the *other* side is the file under review.
func emitPairAnnotations(b *strings.Builder, s code.Suspicion, regPath string) {
	id := pairID(s)
	msg := func(other *code.FuncSig) string {
		return fmt.Sprintf("calque: possible dual-path twin of `%s` (%s:%d) — %s [%s]%s · adjudicate in %s",
			other.Qualname, other.File, other.Line, s.Reason(), id, falseAlarmSuffix(s), regPath)
	}
	fmt.Fprintln(b, ghAnnotation("warning", s.Left.File, s.Left.Line, msg(s.Right)))
	fmt.Fprintln(b, ghAnnotation("warning", s.Right.File, s.Right.Line, msg(s.Left)))
}

func emitClusterAnnotations(b *strings.Builder, c code.Cluster, regPath string) {
	id := clusterID(c)
	for _, m := range c.Members {
		fmt.Fprintln(b, ghAnnotation("warning", m.File, m.Line, fmt.Sprintf(
			"calque: shared-seam cluster of %d functions — %s [%s] · adjudicate in %s",
			len(c.Members), c.Reason(), id, regPath)))
	}
}

// ghAnnotation renders one GitHub Actions workflow command. With file=="" it
// emits a fileless annotation (shown in the run log / Checks summary, not inline
// on a line). Properties and the message are escaped per the Actions spec
// (https://docs.github.com/actions/reference/workflow-commands).
func ghAnnotation(level, file string, line int, message string) string {
	props := ""
	if file != "" {
		props = fmt.Sprintf(" file=%s,line=%d", ghEscapeProp(file), line)
	}
	return fmt.Sprintf("::%s%s::%s", level, props, ghEscapeData(message))
}

// ghEscapeData escapes a workflow-command message. The `%` replacement must run
// first, before any escape introduces its own `%`.
func ghEscapeData(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	s = strings.ReplaceAll(s, "\r", "%0D")
	s = strings.ReplaceAll(s, "\n", "%0A")
	return s
}

// ghEscapeProp escapes a workflow-command property value (additionally `:` and
// `,`, which delimit the property list).
func ghEscapeProp(s string) string {
	s = ghEscapeData(s)
	s = strings.ReplaceAll(s, ":", "%3A")
	s = strings.ReplaceAll(s, ",", "%2C")
	return s
}
