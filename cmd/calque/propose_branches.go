package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/justinstimatze/calque/internal/code"
	"github.com/justinstimatze/calque/internal/pairkey"
	"github.com/justinstimatze/calque/internal/registry"
)

// runProposeBranches turns the sub-function branch axis into a generator: it
// extracts intra-function conditional arms as Kind:"branch" entities and
// scores them with the same Rank pipeline the function axis uses, so a
// "dual path" living BELOW function granularity (two arms of one function,
// or two arms of different functions, that drifted apart) is comparable at
// all. Boundary-free by design, like propose-deriv/propose-cross. GENERATOR,
// not gate: stdout only, no registry writes, no exit code, no --strict
// wiring (see docs/DESIGN_NOTES.md §21 and the established graduation
// precedent KeySetCandidates/SharedDerivationCandidates already set).
func runProposeBranches(args []string) {
	fs := flag.NewFlagSet("propose-branches", flag.ContinueOnError)
	repo := fs.String("repo", ".", "repo root to scan")
	exclude := fs.String("exclude", "", "comma-separated glob(s) to skip entirely (e.g. node_modules/**,vendor/**)")
	includeTests := fs.Bool("include-tests", false, "keep test↔test pairs too (excluded by default — test fixtures are dense with near-identical if/else assertions, the branch axis's dominant false-twin variety; a test arm paired with a production arm is always kept)")
	regPath := fs.String("registry", ".calque/registry.md", "registry file (dedup vs already-adjudicated pairs)")
	minLines := fs.Int("min-lines", 4, "ignore branch arms shorter than this many lines")
	minNodes := fs.Int("min-nodes", 0, "size gate on AST-node count of the arm body; 0 disables (default)")
	minScore := fs.Float64("min-score", 0.5, "min suspicion score to surface")
	top := fs.Int("top", 40, "max fresh candidate pairs to print")
	judge := fs.Bool("judge", false, "adjudicate each candidate with the LLM oracle (needs ANTHROPIC_API_KEY; the precision half)")
	twinsOnly := fs.Bool("twins-only", false, "with --judge, print only pairs the oracle confirms as twins")
	if err := fs.Parse(args); err != nil {
		return
	}

	sigs, st, err := code.ExtractBranches(*repo, splitCSV(*exclude))
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque propose-branches: walking %s: %v\n", *repo, err)
		os.Exit(1)
	}
	reg, err := registry.Load(joinRepo(*repo, *regPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque propose-branches: reading registry: %v\n", err)
		os.Exit(1)
	}

	// Rank requires a cap, but dedupe-then-truncate (not Rank's own top) is the
	// propose-* convention (matches propose-deriv/propose-cross): filter fresh
	// candidates against the registry FIRST, then cap to *top, so an
	// already-adjudicated pair scoring higher than a fresh one can't silently
	// crowd the fresh one out of the printed list.
	const rankNoCap = 1_000_000
	gate := code.SizeGate{MinLines: *minLines, MinNodes: *minNodes}
	susps := code.Rank(sigs, sigs, gate, *minScore, rankNoCap, *includeTests)

	seen := map[string]bool{}
	var fresh []code.SigCandidate
	for _, s := range susps {
		c := suspicionToCandidate(s)
		pk := pairkey.Key(c.A.Key(), c.B.Key())
		if seen[pk] || reg.Has(c.A.Key(), c.B.Key()) {
			continue
		}
		seen[pk] = true
		fresh = append(fresh, c)
	}

	fmt.Println("# calque — sub-function dual-path branches (intra-function drift)")
	fmt.Println()
	testNote := " · test↔test pairs gated (--include-tests to keep; test↔prod always kept)"
	if *includeTests {
		testNote = " · test↔test pairs included"
	}
	fmt.Printf("scanned %d branch fragment(s) in %d file(s); %d fresh candidate(s)  [min-lines=%d min-nodes=%d min-score=%.2f]%s\n",
		st.Funcs, st.Files, len(fresh), *minLines, *minNodes, *minScore, testNote)
	if len(fresh) == 0 {
		fmt.Println("\nno candidates. (Looking for two conditional arms — if/else bodies, switch/select")
		fmt.Println("cases — anywhere in the corpus that read as the same contract; loosen --min-score")
		fmt.Println("/ --min-lines to widen, or check --min-nodes if arms are dense one-liners.)")
		return
	}
	fmt.Println()
	fmt.Println("Two conditional ARMS (not whole functions) that read as the same contract — a")
	fmt.Println("dual path living BELOW function granularity: a legacy vs. new branch of one `if`,")
	fmt.Println("or two regions of a function/file that duplicate each other without being")
	fmt.Println("siblings of a single conditional. Adjudicate each (drift / contracted-twin-ok /")
	fmt.Println("false-alarm), or pass --judge.")
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

// suspicionToCandidate wraps a Rank-produced Suspicion (Left/Right *FuncSig)
// as a code.SigCandidate so propose-branches can reuse the SAME print/judge
// toolkit (printCandidate, runJudge, recordLabel) every other propose-*
// command already uses — Suspicion isn't SigCandidate-shaped natively, but
// the conversion is a pure relabeling, not a lossy one (Reason() already
// renders every fired signal, so nothing is lost by carrying it as Sig).
func suspicionToCandidate(s code.Suspicion) code.SigCandidate {
	return code.SigCandidate{
		A: s.Left, B: s.Right, Kind: "branch",
		Sig:       s.Reason(),
		Jaccard:   s.Score,
		GroupSize: 2,
		CrossFile: s.Left.File != s.Right.File,
	}
}
