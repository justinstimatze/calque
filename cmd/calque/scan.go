package main

// scan — code axis recall: rank dual-path / behavioral-twin (Type-4) suspects
// across a boundary. Walks --repo, extracts a FuncSig per function (go/ast for
// Go; python3 subprocess for Python lands next), and ranks left×right pairs by
// shared contract-invariant signal. Recall-only — a ranked list to adjudicate.
//
// Default (no --left/--right): self-scan all source against itself — the meta
// case (calque must not carry the bug it detects).

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/justinstimatze/calque/internal/code"
)

// boundaryFlags are the scope flags every code-axis command shares (scan, check,
// hook). Single-sourced so the flag taxonomy can't drift across subcommands —
// calque eats its own dogfood (the N-ary touchpoint pass flagged this exact
// repetition across runScan/runCheck/runHook).
type boundaryFlags struct{ repo, left, right, exclude *string }

func addBoundaryFlags(fs *flag.FlagSet) boundaryFlags {
	return boundaryFlags{
		repo:    fs.String("repo", ".", "repo root to scan"),
		left:    fs.String("left", "", "comma-separated glob(s) for the A side (default: all source)"),
		right:   fs.String("right", "", "comma-separated glob(s) for the B side (default: all source)"),
		exclude: fs.String("exclude", "", "comma-separated glob(s) to skip entirely (e.g. legacy/**,vendor/**)"),
	}
}

func runScan(args []string) {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	b := addBoundaryFlags(fs)
	repo, left, right, exclude := b.repo, b.left, b.right, b.exclude
	top := fs.Int("top", 30, "max suspect pairs to report")
	minScore := fs.Float64("min-score", 0.18, "minimum suspicion score to report")
	minLines := fs.Int("min-lines", 4, "ignore functions shorter than this many lines")
	noClusters := fs.Bool("no-clusters", false, "skip the N-ary private-seam clustering pass")
	clusterMinMembers := fs.Int("cluster-min-members", 3, "smallest N-ary cluster to report (2 includes diluted pairs)")
	clusterMaxFanout := fs.Int("cluster-max-fanout", 8, "a private symbol touched by more than this is plumbing, not a seam")
	if err := fs.Parse(args); err != nil {
		return
	}

	all, st, err := code.Extract(*repo, splitCSV(*exclude))
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque scan: walking %s: %v\n", *repo, err)
		os.Exit(1)
	}
	if st.Funcs == 0 {
		fmt.Fprintf(os.Stderr, "calque scan: no extractable source under %s (supported: %v; %d code file(s) skipped %v)\n",
			*repo, code.SupportedExts(), st.Skipped, st.SkippedExts)
		os.Exit(1)
	}

	L := code.Filter(all, *left)
	R := code.Filter(all, *right)
	susp := code.Rank(L, R, *minLines, *minScore, *top)

	fmt.Println("# calque — dual-path suspects")
	fmt.Println()
	fmt.Printf("boundary: `%s`  ×  `%s`\n", orAll(*left), orAll(*right))
	fmt.Printf("scanned %d func(s) in %d file(s); suspect pairs: %d\n", st.Funcs, st.Files, len(susp))
	if st.Skipped > 0 {
		fmt.Printf("note: %d code file(s) skipped (no extractor yet): %v\n", st.Skipped, st.SkippedExts)
	}
	fmt.Println()
	fmt.Println("calque is recall-only — adjudicate each as drift / contracted-twin-ok / false-alarm,")
	fmt.Println("then record the verdict in .calque/registry.md.")
	fmt.Println()
	for i, s := range susp {
		fmt.Printf("## %d. %.2f  `%s` (%s:%d)  ≟  `%s` (%s:%d)\n",
			i+1, s.Score, s.Left.Qualname, s.Left.File, s.Left.Line,
			s.Right.Qualname, s.Right.File, s.Right.Line)
		fmt.Printf("- %s\n", s.Reason())
	}

	if !*noClusters {
		opts := code.DefaultClusterOptions()
		opts.MinLines = *minLines
		opts.MinMembers = *clusterMinMembers
		opts.MaxFanout = *clusterMaxFanout
		opts.Top = *top
		clusters := code.ClusterByTouchpoint(unionSigs(L, R), opts)
		fmt.Println()
		fmt.Printf("# calque — N-ary clusters (shared private seams)\n\n")
		fmt.Printf("%d cluster(s) of >=%d functions sharing a rare private symbol — the\n", len(clusters), opts.MinMembers)
		fmt.Println("sub-function / triple-shell shape pairwise scoring structurally misses (§15).")
		fmt.Println()
		for i, c := range clusters {
			fmt.Printf("## C%d. %.2f  (%d members)  %s\n", i+1, c.Score, len(c.Members), c.Reason())
			for _, m := range c.Members {
				fmt.Printf("- `%s` (%s:%d)\n", m.Qualname, m.File, m.Line)
			}
		}
	}
}

// unionSigs returns the deduped union of two FuncSig slices, keyed on Key().
// Clustering is whole-corpus (N-ary), not boundary-based, so it runs over the
// union of the scoped sides rather than a left×right product.
func unionSigs(a, b []*code.FuncSig) []*code.FuncSig {
	seen := make(map[string]bool, len(a)+len(b))
	out := make([]*code.FuncSig, 0, len(a)+len(b))
	for _, fs := range [][]*code.FuncSig{a, b} {
		for _, f := range fs {
			if seen[f.Key()] {
				continue
			}
			seen[f.Key()] = true
			out = append(out, f)
		}
	}
	return out
}

func orAll(s string) string {
	if strings.TrimSpace(s) == "" {
		return "**/* (all source)"
	}
	return s
}

// splitCSV splits a comma list into trimmed non-empty parts.
func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
