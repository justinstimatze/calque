package main

// check — the ongoing/hookable gate (the keystone of automated use). Runs the
// same scan as `scan`, then DIFFS against the registry: surfaces only NEW
// (un-adjudicated) suspect pairs, suppresses known ones, and reconciles STALE
// registry entries (pairs whose referenced code no longer exists — the
// dusty-over-months problem, handled by liveness, not age-eviction).
//
// Warn-only by default (exit 0) so it can land in a pre-commit / Stop hook
// without blocking; --strict exits 1 when there are new suspects (cupel's
// vocab-audit discipline: sweep the registry, then flip to strict).

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/justinstimatze/calque/internal/code"
	"github.com/justinstimatze/calque/internal/registry"
)

func runCheck(args []string) {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	repo := fs.String("repo", ".", "repo root to scan")
	left := fs.String("left", "", "comma-separated glob(s) for the A side (default: all source)")
	right := fs.String("right", "", "comma-separated glob(s) for the B side (default: all source)")
	exclude := fs.String("exclude", "", "comma-separated glob(s) to skip entirely (e.g. legacy/**,vendor/**)")
	minScore := fs.Float64("min-score", 0.18, "minimum suspicion score to consider")
	minLines := fs.Int("min-lines", 4, "ignore functions shorter than this many lines")
	regPath := fs.String("registry", ".calque/registry.md", "registry file (adjudicated pairs)")
	strict := fs.Bool("strict", false, "exit 1 if there are new (un-adjudicated) suspects")
	if err := fs.Parse(args); err != nil {
		return
	}

	all, _, err := code.Extract(*repo, splitCSV(*exclude))
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque check: %v\n", err)
		os.Exit(1)
	}
	reg, err := registry.Load(joinRepo(*repo, *regPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque check: reading registry: %v\n", err)
		os.Exit(1)
	}

	L := code.Filter(all, *left)
	R := code.Filter(all, *right)
	susp := code.Rank(L, R, *minLines, *minScore, 1<<30)

	var fresh []code.Suspicion
	known := 0
	for _, s := range susp {
		if reg.Has(s.Left.Key(), s.Right.Key()) {
			known++
			continue
		}
		fresh = append(fresh, s)
	}

	// Liveness reconciliation: registry entries whose referenced code is gone.
	live := make(map[string]bool, len(all))
	for _, f := range all {
		live[f.Key()] = true
	}
	var stale []registry.Entry
	for _, e := range reg.Entries {
		if !live[e.Key1] || !live[e.Key2] {
			stale = append(stale, e)
		}
	}

	fmt.Printf("calque check: %d new · %d known (suppressed) · %d stale registry entr%s\n",
		len(fresh), known, len(stale), plural(len(stale), "y", "ies"))

	for _, s := range fresh {
		fmt.Printf("\nNEW  %.2f  `%s` (%s:%d)  ≟  `%s` (%s:%d)\n     %s\n",
			s.Score, s.Left.Qualname, s.Left.File, s.Left.Line,
			s.Right.Qualname, s.Right.File, s.Right.Line, s.Reason())
		fmt.Printf("     adjudicate in %s — add:  - pair: %s | %s\n", *regPath, s.Left.Key(), s.Right.Key())
	}
	for _, e := range stale {
		fmt.Printf("\nSTALE  %s  `%s` ≟ `%s` — referenced code no longer exists; prune or re-home (was: %s%s)\n",
			"", e.Key1, e.Key2, e.Verdict, reviewedSuffix(e))
	}

	if *strict && len(fresh) > 0 {
		os.Exit(1)
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func reviewedSuffix(e registry.Entry) string {
	if e.Reviewed == "" {
		return ""
	}
	return ", reviewed " + e.Reviewed
}

func joinRepo(repo, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(repo, p)
}
