package main

// propose-deep — the representation-independent Type-4 candidate generator. The
// jaccard `scan`/`check` gate scores surface tokens, so it is structurally blind to
// behavioral twins that share a contract but no token (two impls of
// `sessionId → WorktreeInfo`, one reading JSON, one rebuilding from git). This pass
// groups functions by a rare, informative TYPE SIGNATURE — the shared contract — and
// emits the pairs as twin candidates, each tagged with the jaccard score that proves
// how invisible it is to the current gate.
//
// GENERATOR, not gate: stdout only, no registry writes, no exit code — it cannot
// disturb a repo's `check --strict`. Signature recall is high-recall / low-precision
// by nature (many same-shape functions do different jobs), so the output is a
// candidate list for an adjudicator, not a verdict. Signatures are extracted for
// TS/TSX today; Go/Python functions carry no signature yet, so this finds nothing
// on those repos until their extractors emit types.

import (
	"flag"
	"fmt"
	"os"

	"github.com/justinstimatze/calque/internal/code"
	"github.com/justinstimatze/calque/internal/registry"
)

func runProposeDeep(args []string) {
	fs := flag.NewFlagSet("propose-deep", flag.ContinueOnError)
	repo := fs.String("repo", ".", "repo root to scan")
	exclude := fs.String("exclude", "", "comma-separated glob(s) to skip entirely (e.g. node_modules/**,dist/**)")
	regPath := fs.String("registry", ".calque/registry.md", "registry file (dedup vs already-adjudicated pairs)")
	minLines := fs.Int("min-lines", 4, "ignore functions shorter than this many lines")
	minMembers := fs.Int("sig-min-members", 2, "smallest signature group to propose from (2 = a rare shared contract)")
	maxMembers := fs.Int("sig-max-members", 6, "largest signature group to consider (above this the shape is common, not a twin)")
	top := fs.Int("top", 40, "max candidates to print")
	if err := fs.Parse(args); err != nil {
		return
	}

	sigs, st, err := code.Extract(*repo, splitCSV(*exclude))
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque propose-deep: walking %s: %v\n", *repo, err)
		os.Exit(1)
	}
	reg, err := registry.Load(joinRepo(*repo, *regPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque propose-deep: reading registry: %v\n", err)
		os.Exit(1)
	}

	cands := code.SignatureCandidates(sigs, *minLines, *minMembers, *maxMembers)
	// Dedup against already-adjudicated pairs (don't re-propose a settled verdict).
	var fresh []code.SigCandidate
	for _, c := range cands {
		if reg.Has(c.A.Key(), c.B.Key()) {
			continue
		}
		fresh = append(fresh, c)
	}

	withSig := 0
	for _, f := range sigs {
		if f.Sig != "" {
			withSig++
		}
	}

	fmt.Println("# calque — Type-4 candidates (shared contract, representation-independent)")
	fmt.Println()
	fmt.Printf("scanned %d func(s) in %d file(s); %d carry a type signature; %d fresh candidate(s)\n",
		st.Funcs, st.Files, withSig, len(fresh))
	if withSig == 0 {
		fmt.Println()
		fmt.Println("No functions carried a type signature — signatures are extracted for TS/TSX only")
		fmt.Println("today. (Go/Python signature extraction is a planned extension.)")
		return
	}
	fmt.Println()
	fmt.Println("Twins sharing a rare type signature but NO surface tokens — the gate scores")
	fmt.Println("them near zero (the `jac` column). High recall, low precision: adjudicate each")
	fmt.Println("as drift / contracted-twin-ok / false-alarm before trusting it.")
	fmt.Println()

	if len(fresh) > *top {
		fmt.Printf("(showing top %d of %d)\n\n", *top, len(fresh))
		fresh = fresh[:*top]
	}
	for i, c := range fresh {
		fmt.Printf("## %d. `%s`  (group %d, jac %.2f%s)\n", i+1, c.Sig, c.GroupSize, c.Jaccard, crossFileMark(c.CrossFile))
		fmt.Printf("- `%s` (%s:%d)\n", c.A.Qualname, c.A.File, c.A.Line)
		fmt.Printf("- `%s` (%s:%d)\n", c.B.Qualname, c.B.File, c.B.Line)
		fmt.Printf("  adjudicate:  - pair: %s::%s | %s::%s\n", c.A.File, c.A.Qualname, c.B.File, c.B.Qualname)
	}
}

func crossFileMark(cross bool) string {
	if cross {
		return ", cross-file"
	}
	return ""
}
