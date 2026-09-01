package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/justinstimatze/calque/internal/code"
	"github.com/justinstimatze/calque/internal/pairkey"
	"github.com/justinstimatze/calque/internal/registry"
)

// runProposeContext turns the call-site context axis into a generator: it
// pairs functions with no shared body token and no distinctive type
// signature — the class every other channel misses by construction — purely
// on how and where they're invoked (see docs/SPEC-callsite-context-axis.md).
// Boundary-free, GENERATOR not gate, same discipline as propose-deep/
// propose-branches: stdout only, no registry writes, no --strict wiring.
// Recall-first by design; expect heavy false-alarm volume until §5's
// valence-guard stoplist is calibrated from real adjudication data.
func runProposeContext(args []string) {
	fs := flag.NewFlagSet("propose-context", flag.ContinueOnError)
	repo := fs.String("repo", ".", "repo root to scan")
	exclude := fs.String("exclude", "", "comma-separated glob(s) to skip entirely (e.g. node_modules/**,vendor/**)")
	regPath := fs.String("registry", ".calque/registry.md", "registry file (dedup vs already-adjudicated pairs)")
	minLines := fs.Int("min-lines", 4, "ignore functions shorter than this many lines")
	minNodes := fs.Int("min-nodes", 0, "size gate on AST-node count of the function body; 0 disables (default)")
	minCallerJaccard := fs.Float64("min-caller-jaccard", 0.5, "min jaccard of caller name-stem sets to pair (both callees' callers, not the callees themselves)")
	minShapeJaccard := fs.Float64("min-shape-jaccard", 0.5, "min jaccard of call-result shape tags to pair (ret-nil-checked, ret-err-checked, …)")
	maxFanout := fs.Int("max-fanout", 8, "skip a caller-stem token shared by more than this many candidate callees (plumbing, not a role)")
	top := fs.Int("top", 40, "max fresh candidate pairs to print")
	judge := fs.Bool("judge", false, "adjudicate each candidate with the LLM oracle (needs ANTHROPIC_API_KEY; the precision half)")
	twinsOnly := fs.Bool("twins-only", false, "with --judge, print only pairs the oracle confirms as twins")
	if err := fs.Parse(args); err != nil {
		return
	}

	sigs, st, err := code.Extract(*repo, splitCSV(*exclude))
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque propose-context: walking %s: %v\n", *repo, err)
		os.Exit(1)
	}
	reg, err := registry.Load(joinRepo(*repo, *regPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque propose-context: reading registry: %v\n", err)
		os.Exit(1)
	}

	gate := code.SizeGate{MinLines: *minLines, MinNodes: *minNodes}
	cands := code.CallContextCandidates(sigs, gate, *minCallerJaccard, *minShapeJaccard, *maxFanout)

	seen := map[string]bool{}
	var fresh []code.SigCandidate
	for _, c := range cands {
		pk := pairkey.Key(c.A.Key(), c.B.Key())
		if seen[pk] || reg.Has(c.A.Key(), c.B.Key()) {
			continue
		}
		seen[pk] = true
		fresh = append(fresh, c)
	}

	fmt.Println("# calque — call-site context candidates (zero shared tokens, invoked alike)")
	fmt.Println()
	fmt.Printf("scanned %d func(s) in %d file(s); %d fresh candidate(s)  [min-caller-jaccard=%.2f min-shape-jaccard=%.2f max-fanout=%d]\n",
		st.Funcs, st.Files, len(fresh), *minCallerJaccard, *minShapeJaccard, *maxFanout)
	if len(fresh) == 0 {
		fmt.Println("\nno candidates. (Needs two functions each called from a caller with an overlapping")
		fmt.Println("name-stem AND checked/used the same way at the call site; loosen --min-caller-jaccard")
		fmt.Println("/ --min-shape-jaccard to widen.)")
		return
	}
	fmt.Println()
	fmt.Println("Two functions with NO shared body token and NO distinctive type signature — the")
	fmt.Println("class every other channel misses by construction — paired purely on how and where")
	fmt.Println("they're called: near-synonym caller names, and the same thing done with the result")
	fmt.Println("(both nil-checked, both fed to a downstream call). Experimental, high recall by")
	fmt.Println("design: expect a heavier false-alarm rate than the other generators until this")
	fmt.Println("axis has real adjudication data to calibrate a stoplist from. Adjudicate each")
	fmt.Println("(drift / contracted-twin-ok / false-alarm), or pass --judge.")
	fmt.Println()

	if len(fresh) > *top {
		fmt.Printf("(showing top %d of %d)\n\n", *top, len(fresh))
		fresh = fresh[:*top]
	}

	if *judge {
		runJudge(*repo, fresh, *twinsOnly)
		return
	}
	for i, c := range fresh {
		printCandidate(i+1, c, nil)
	}
}
