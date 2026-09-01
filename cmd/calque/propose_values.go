package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/justinstimatze/calque/internal/code"
	"github.com/justinstimatze/calque/internal/pairkey"
	"github.com/justinstimatze/calque/internal/registry"
)

// runProposeValues turns the scattered-value axis into a generator: it
// extracts literal-value occurrences bound to a nearby identifier
// (assignment target, var/const declaration, composite-literal field/map
// key) as Kind:"value-site" entities and pairs them by exact value match,
// filtered by name-stem correlation — a maxRetries-style value repeated
// across independent call sites with no shared symbol backing it, so it can
// (and does) drift out of sync. Its candidates ARE []code.SigCandidate
// (ValueSiteCandidates mirrors KeySetCandidates's shape), so this reuses the
// established generator toolkit verbatim: printCandidate for plain output,
// runJudge for --judge (both propose_deep.go, already used by propose-cross/
// propose-deriv/confess) — no new print/judge code. GENERATOR, not gate:
// stdout only, no registry writes, no exit code, no --strict wiring (see
// docs/DESIGN_NOTES.md §21 and the established graduation precedent
// KeySetCandidates/SharedDerivationCandidates already set).
func runProposeValues(args []string) {
	fs := flag.NewFlagSet("propose-values", flag.ContinueOnError)
	repo := fs.String("repo", ".", "repo root to scan")
	exclude := fs.String("exclude", "", "comma-separated glob(s) to skip entirely (e.g. node_modules/**,vendor/**)")
	regPath := fs.String("registry", ".calque/registry.md", "registry file (dedup vs already-adjudicated pairs)")
	nameJac := fs.Float64("name-jaccard", 0.01, "min name-stem jaccard between the two sites' nearby identifiers to pair (0 = any same-value pair regardless of name; a coincidental shared literal like 3 recurs constantly for unrelated reasons, so 0 is very noisy)")
	maxFanout := fs.Int("max-fanout", 8, "skip values shared by more than this many sites (a coincidentally common literal, not a magic-constant shape)")
	top := fs.Int("top", 40, "max candidates to print")
	judge := fs.Bool("judge", false, "adjudicate each candidate with the LLM oracle (needs ANTHROPIC_API_KEY; the precision half)")
	twinsOnly := fs.Bool("twins-only", false, "with --judge, print only candidates the oracle confirms as twins")
	if err := fs.Parse(args); err != nil {
		return
	}

	sites, st, err := code.ExtractValueSites(*repo, splitCSV(*exclude))
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque propose-values: walking %s: %v\n", *repo, err)
		os.Exit(1)
	}
	reg, err := registry.Load(joinRepo(*repo, *regPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque propose-values: reading registry: %v\n", err)
		os.Exit(1)
	}

	cands := code.ValueSiteCandidates(sites, *nameJac, *maxFanout)
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

	fmt.Println("# calque — scattered-value drift (same literal, no shared symbol)")
	fmt.Println()
	fmt.Printf("scanned %d value-site(s) in %d file(s); %d fresh candidate(s)  [name-jaccard=%.2f max-fanout=%d]\n",
		st.Funcs, st.Files, len(fresh), *nameJac, *maxFanout)
	if len(fresh) == 0 {
		fmt.Println("\nno candidates. (Looking for a literal value repeated at independent sites under a")
		fmt.Println("similarly-named identifier, with no shared const backing it; loosen --name-jaccard")
		fmt.Println("/ --max-fanout to widen.)")
		return
	}
	fmt.Println()
	fmt.Println("The SAME literal value bound to a similarly-named identifier at independent")
	fmt.Println("sites, with no shared const/symbol — each can drift independently (fix one, the")
	fmt.Println("others silently diverge). Adjudicate each (drift / contracted-twin-ok /")
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
