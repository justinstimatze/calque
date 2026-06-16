package main

// propose-deriv — the VALUE-DERIVATION drift generator. It surfaces functions that
// derive an output from the SAME input field-set (their `reads`) without routing
// through a shared authority — the dual-path shape where one physical quantity (a
// height, width, offset, centerline) is computed independently in >=2 places and
// silently diverges ("fix one path, the twin still has the bug"). This is the recall
// pass the gated jaccard scorer structurally cannot make: the twins share no surface
// token, only the input fields they consume.
//
// Boundary-free by design — it clusters across the WHOLE corpus, serving the
// standing-audit / batch-cleanup use case (no --left/--right). GENERATOR, not gate:
// stdout only, no registry writes, no exit code. High recall / low precision; the
// judge (or a human) is the precision half.

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/justinstimatze/calque/internal/code"
	"github.com/justinstimatze/calque/internal/pairkey"
	"github.com/justinstimatze/calque/internal/registry"
)

func runProposeDeriv(args []string) {
	fs := flag.NewFlagSet("propose-deriv", flag.ContinueOnError)
	repo := fs.String("repo", ".", "repo root to scan")
	exclude := fs.String("exclude", "", "comma-separated glob(s) to skip entirely (e.g. node_modules/**,dist/**)")
	regPath := fs.String("registry", ".calque/registry.md", "registry file (dedup vs already-adjudicated pairs)")
	minReads := fs.Int("min-reads", 3, "ignore functions reading fewer than this many field-paths (thin = non-discriminating)")
	readJac := fs.Float64("read-jaccard", 0.5, "min read-set jaccard to pair (1.0 = identical input field-set)")
	maxFanout := fs.Int("max-fanout", 8, "skip field-paths read by more than this many functions (plumbing, not a seam)")
	top := fs.Int("top", 40, "max candidates to print")
	judge := fs.Bool("judge", false, "adjudicate each candidate with the LLM oracle (needs ANTHROPIC_API_KEY; the precision half)")
	twinsOnly := fs.Bool("twins-only", false, "with --judge, print only candidates the oracle confirms as twins")
	if err := fs.Parse(args); err != nil {
		return
	}

	sigs, st, err := code.Extract(*repo, splitCSV(*exclude))
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque propose-deriv: walking %s: %v\n", *repo, err)
		os.Exit(1)
	}
	reg, err := registry.Load(joinRepo(*repo, *regPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque propose-deriv: reading registry: %v\n", err)
		os.Exit(1)
	}

	cands := code.SharedDerivationCandidates(sigs, *minReads, *readJac, *maxFanout)
	seen := map[string]bool{}
	var fresh []code.SigCandidate
	for _, c := range cands {
		pk := pairkey.Key(c.A.Key(), c.B.Key())
		if seen[pk] || reg.Has(c.A.Key(), c.B.Key()) {
			continue
		}
		seen[pk] = true
		c.Sig = c.Sig + " · " + derivationLean(c) // collapse-vs-pin hint (finding #1)
		fresh = append(fresh, c)
	}

	fmt.Println("# calque — value-derivation drift (shared input field-set, no shared authority)")
	fmt.Println()
	fmt.Printf("scanned %d func(s) in %d file(s); %d fresh candidate(s)  [min-reads=%d read-jaccard=%.2f max-fanout=%d]\n",
		st.Funcs, st.Files, len(fresh), *minReads, *readJac, *maxFanout)
	if len(fresh) == 0 {
		fmt.Println("\nno candidates. (Looking for functions deriving a value from the same field-set without delegating to one authority — loosen --read-jaccard / --min-reads to widen.)")
		return
	}
	fmt.Println()
	fmt.Println("Functions that DERIVE a value from the SAME input fields without routing through a")
	fmt.Println("shared authority — implementation drift the jaccard gate can't see (the twins share")
	fmt.Println("no surface token, only their inputs). High recall, low precision: adjudicate each")
	fmt.Println("(drift / contracted-twin-ok / false-alarm), or pass --judge. The lean: `collapse` =")
	fmt.Println("same package, extractable to one authority; `pin/cross-pkg` = lean to a differential test.")
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

// derivationLean is the collapse-vs-pin hint (lexicon finding #1): two functions in
// the SAME package can usually be collapsed to one shared helper (the inputs are
// already in reach); across packages the cleaner fix is often a differential test
// (pin) or an extract-to-shared, so flag it for human judgment rather than asserting
// a collapse that may be structurally impossible.
func derivationLean(c code.SigCandidate) string {
	if filepath.Dir(c.A.File) == filepath.Dir(c.B.File) {
		return "collapse"
	}
	return "pin/cross-pkg"
}
