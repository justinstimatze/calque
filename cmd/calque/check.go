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
	"strings"

	"github.com/justinstimatze/calque/internal/code"
	"github.com/justinstimatze/calque/internal/registry"
)

func runCheck(args []string) {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	b := addBoundaryFlags(fs)
	repo, left, right, exclude := b.repo, b.left, b.right, b.exclude
	minScore := fs.Float64("min-score", 0.18, "minimum suspicion score to consider")
	minLines := fs.Int("min-lines", 4, "ignore functions shorter than this many lines")
	regPath := fs.String("registry", ".calque/registry.md", "registry file (adjudicated pairs)")
	strict := fs.Bool("strict", false, "exit 1 if there are new (un-adjudicated) suspects")
	clusterMinMembers := fs.Int("cluster-min-members", 3, "smallest N-ary cluster to consider (2 includes diluted pairs)")
	clusterMaxFanout := fs.Int("cluster-max-fanout", 8, "a private symbol touched by more than this is plumbing, not a seam")
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

	// N-ary clusters (the touchpoint pass): same new/known split, keyed on the
	// member SET (§15). Whole-corpus, so clustered over the union of the scope.
	copts := code.DefaultClusterOptions()
	copts.MinLines = *minLines
	copts.MinMembers = *clusterMinMembers
	copts.MaxFanout = *clusterMaxFanout
	copts.Top = 1 << 30
	clusters := code.ClusterByTouchpoint(unionSigs(L, R), copts)
	var freshC []code.Cluster
	knownC := 0
	for _, c := range clusters {
		if reg.HasCluster(c.MemberKeys()) {
			knownC++
			continue
		}
		freshC = append(freshC, c)
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
	var staleC []registry.ClusterEntry
	for _, e := range reg.Clusters {
		for _, k := range e.Keys {
			if !live[k] {
				staleC = append(staleC, e)
				break
			}
		}
	}

	fmt.Printf("calque check: pairs %d new · %d known | clusters %d new · %d known | %d stale registry entr%s\n",
		len(fresh), known, len(freshC), knownC, len(stale)+len(staleC), plural(len(stale)+len(staleC), "y", "ies"))

	for _, s := range fresh {
		fmt.Printf("\nNEW  %.2f  `%s` (%s:%d)  ≟  `%s` (%s:%d)\n     %s\n",
			s.Score, s.Left.Qualname, s.Left.File, s.Left.Line,
			s.Right.Qualname, s.Right.File, s.Right.Line, s.Reason())
		fmt.Printf("     adjudicate in %s — add:  - pair: %s | %s\n", *regPath, s.Left.Key(), s.Right.Key())
	}
	for _, c := range freshC {
		fmt.Printf("\nNEW-CLUSTER  %.2f  (%d members)  %s\n", c.Score, len(c.Members), c.Reason())
		for _, m := range c.Members {
			fmt.Printf("     `%s` (%s:%d)\n", m.Qualname, m.File, m.Line)
		}
		fmt.Printf("     adjudicate in %s — add:  - cluster: %s\n", *regPath, strings.Join(c.MemberKeys(), " | "))
	}
	for _, e := range stale {
		fmt.Printf("\nSTALE  `%s` ≟ `%s` — referenced code no longer exists; prune or re-home (was: %s%s)\n",
			e.Key1, e.Key2, e.Verdict, reviewedSuffix(e))
	}
	for _, e := range staleC {
		fmt.Printf("\nSTALE-CLUSTER  {%s} — a referenced member no longer exists; prune or re-home (was: %s)\n",
			strings.Join(e.Keys, ", "), e.Verdict)
	}

	if *strict && (len(fresh) > 0 || len(freshC) > 0) {
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
