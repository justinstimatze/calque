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
	"path/filepath"
	"sort"
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
	noCalib := fs.Bool("no-calibrated-weights", false, "ignore .calque/weights.json; score on the static prior")
	includeTests := fs.Bool("include-tests", false, "rank test↔test pairs too (excluded by default — two test cases sharing a setup/mock fixture are the dominant false twin; test↔prod pairs are always kept)")
	if err := fs.Parse(args); err != nil {
		return
	}

	if applyCalibratedWeights(*repo, *noCalib) {
		fmt.Fprintln(os.Stderr, "calque: calibrated weights active (.calque/weights.json)")
	}
	copts := clusterOptsFrom(*minLines, *clusterMinMembers, *clusterMaxFanout, *top)
	r, err := codeAxis(*repo, *left, *right, *exclude, *minScore, *minLines, *top, copts, !*noClusters, *includeTests)
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque scan: walking %s: %v\n", *repo, err)
		os.Exit(1)
	}
	if r.Stats.Funcs == 0 {
		fmt.Fprintf(os.Stderr, "calque scan: no extractable source under %s (supported: %v; %d code file(s) skipped %v)\n",
			*repo, code.SupportedExts(), r.Stats.Skipped, r.Stats.SkippedExts)
		os.Exit(1)
	}

	fmt.Println("# calque — dual-path suspects")
	fmt.Println()
	fmt.Printf("boundary: `%s`  ×  `%s`\n", orAll(*left), orAll(*right))
	for _, w := range boundaryBiteWarnings(*left, *right, r.All, r.Stats.CodeFiles) {
		fmt.Println(w)
	}
	fmt.Printf("scanned %d func(s) in %d file(s); suspect pairs: %d\n", r.Stats.Funcs, r.Stats.Files, len(r.Pairs))
	if r.Stats.Skipped > 0 {
		fmt.Printf("note: %d code file(s) skipped (no extractor yet): %v\n", r.Stats.Skipped, r.Stats.SkippedExts)
	}
	fmt.Println()
	fmt.Println("calque is recall-only — adjudicate each as drift / contracted-twin-ok / false-alarm,")
	fmt.Println("then record the verdict in .calque/registry.md.")
	fmt.Println()
	for i, s := range r.Pairs {
		fmt.Printf("## %d. %.2f  `%s` (%s:%d)  ≟  `%s` (%s:%d)\n",
			i+1, s.Score, s.Left.Qualname, s.Left.File, s.Left.Line,
			s.Right.Qualname, s.Right.File, s.Right.Line)
		fmt.Printf("- %s%s\n", s.Reason(), falseAlarmSuffix(s))
	}

	if !*noClusters {
		fmt.Println()
		fmt.Printf("# calque — N-ary clusters (shared private seams)\n\n")
		fmt.Printf("%d cluster(s) of >=%d functions sharing a rare private symbol — the\n", len(r.Clusters), copts.MinMembers)
		fmt.Println("sub-function / triple-shell shape pairwise scoring structurally misses (§15).")
		fmt.Println()
		for i, c := range r.Clusters {
			fmt.Printf("## C%d. %.2f  (%d members)  %s\n", i+1, c.Score, len(c.Members), c.Reason())
			for _, m := range c.Members {
				fmt.Printf("- `%s` (%s:%d)\n", m.Qualname, m.File, m.Line)
			}
		}
	}
}

// falseAlarmSuffix renders the inline structural false-alarm hint for a suspect
// pair (e.g. same-receiver / field-copy), or "" when none applies. Advisory only —
// shared by scan and check so the annotation reads identically in both.
func falseAlarmSuffix(s code.Suspicion) string {
	if h := code.FalseAlarmHint(s.Left, s.Right); h != "" {
		return "  ·  structural: " + h + " (often a false alarm — see SKILL.md)"
	}
	return ""
}

// boundaryBiteWarnings returns prominent warnings when a boundary side's glob
// matched code files ON DISK but the extractor produced ZERO functions from them —
// a FALSE clean (zero suspects because nothing parsed, not because nothing
// diverged), the failure mode a recall-first tool must never report as a clean
// bill. The dominant cause is an unsupported / not-yet-implemented language on one
// side (or a stale binary pointed at a newer repo). Returns nil for whole-repo
// (no-glob) sides and for sides that bit normally. Shared by scan and check so the
// wording reads identically in both.
func boundaryBiteWarnings(left, right string, all []*code.FuncSig, codeFiles []string) []string {
	var out []string
	check := func(side, glob string) {
		if strings.TrimSpace(glob) == "" {
			return // whole-repo default for this side — can't under-bite
		}
		matched := code.MatchGlob(codeFiles, glob)
		if len(matched) == 0 {
			return // no code files on disk match this glob — a visibly empty side, not a false clean
		}
		parsed := len(code.Filter(all, glob))
		noExt := unparsedExts(matched)
		switch {
		case parsed == 0:
			reason := "no functions parsed"
			if len(noExt) > 0 {
				reason = "no extractor for " + strings.Join(noExt, ", ")
			}
			out = append(out, fmt.Sprintf(
				"⚠ boundary cannot bite: %s `%s` matched %d file(s), 0 parsed (%s). Result is NOT a clean bill.",
				side, glob, len(matched), reason))
		case len(noExt) > 0:
			out = append(out, fmt.Sprintf(
				"⚠ partial coverage: %s `%s` matched %d file(s) but type(s) %s have no extractor — some files on this side were not scanned.",
				side, glob, len(matched), strings.Join(noExt, ", ")))
		}
	}
	check("left", left)
	check("right", right)
	return out
}

// unparsedExts returns the distinct extensions among files for which calque has no
// function-axis extractor — the reason those files contributed nothing to the side.
func unparsedExts(files []string) []string {
	seen := map[string]bool{}
	var exts []string
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f))
		if !code.HasExtractor(ext) && !seen[ext] {
			seen[ext] = true
			exts = append(exts, ext)
		}
	}
	sort.Strings(exts)
	return exts
}

// clusterOptsFrom builds ClusterOptions from the common cluster flags — shared by
// scan/check/doctor so the cluster knobs stay single-sourced.
func clusterOptsFrom(minLines, minMembers, maxFanout, top int) code.ClusterOptions {
	o := code.DefaultClusterOptions()
	o.MinLines, o.MinMembers, o.MaxFanout, o.Top = minLines, minMembers, maxFanout, top
	return o
}

// codeAxisResult is the full code-axis recall output (pairs + N-ary clusters +
// the extracted corpus + coverage stats).
type codeAxisResult struct {
	Pairs    []code.Suspicion
	Clusters []code.Cluster
	All      []*code.FuncSig
	Stats    code.ScanStats
}

// codeAxis runs the shared recall pipeline — extract → filter → rank (pairs) +
// touchpoint cluster (N-ary) — single-sourced so scan, check, and doctor can't
// drift on how they recall (the pipeline calque flagged duplicated across them).
// withClusters lets a caller skip the N-ary pass.
func codeAxis(repo, left, right, exclude string, minScore float64, minLines, top int, copts code.ClusterOptions, withClusters, includeTests bool) (codeAxisResult, error) {
	all, st, err := code.Extract(repo, splitCSV(exclude))
	if err != nil {
		return codeAxisResult{}, err
	}
	L := code.Filter(all, left)
	R := code.Filter(all, right)
	copts.IncludeTests = includeTests // single-source the test gate across pairs + clusters
	res := codeAxisResult{All: all, Stats: st, Pairs: code.Rank(L, R, minLines, minScore, top, includeTests)}
	if withClusters {
		res.Clusters = code.ClusterByTouchpoint(unionSigs(L, R), copts)
	}
	return res, nil
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
