package main

// propose-cross — the CROSS-SUBSTRATE axis (generator only). The code axis sees
// functions; the hardest drifts in a content-driven codebase live BETWEEN
// non-function entities: a module-level table mirrored in another file
// (engine.py::HANDLERS ≟ input_agent.py::_VERB_TEMPLATES), or an authored corpus
// shape mirrored by a code table/schema (corpus/*.json field-set ≟ a db schema).
// Those share no surface tokens, so the jaccard gate is structurally blind to them.
//
// This command extracts non-function entities (ExtractSymbols: .py tables, .json
// corpus shapes), pairs them by shared KEY SET (code.KeySetCandidates), and — like
// propose-roles/propose-deep — only PRINTS. No registry writes, no exit code, never
// part of the --strict gate, so it can't disturb the self-clean check.

import (
	"flag"
	"fmt"
	"os"

	"github.com/justinstimatze/calque/internal/code"
	"github.com/justinstimatze/calque/internal/pairkey"
	"github.com/justinstimatze/calque/internal/registry"
)

// readEntitySource returns the source the oracle should see for an entity. A
// corpus-field entity (a JSON object) carries its blob inline in Source — there's
// no clean line span to slice — so use it directly; functions and tables fall back
// to the line-based reader.
func readEntitySource(repo string, f *code.FuncSig, maxLines int) string {
	if f.Source != "" {
		return f.Source
	}
	return readFuncSource(repo, f, maxLines)
}

func runProposeCross(args []string) {
	fs := flag.NewFlagSet("propose-cross", flag.ContinueOnError)
	repo := fs.String("repo", ".", "repo root to scan")
	exclude := fs.String("exclude", "", "comma-separated glob(s) to skip entirely (e.g. node_modules/**,dist/**)")
	regPath := fs.String("registry", ".calque/registry.md", "registry file (dedup vs already-adjudicated pairs)")
	minKeys := fs.Int("min-keys", 3, "ignore entities with fewer than this many keys/fields")
	keyJac := fs.Float64("key-jaccard", 0.5, "min key-set jaccard to pair (1.0 = identical key set)")
	maxFanout := fs.Int("max-fanout", 8, "skip keys shared by more than this many entities (plumbing, not a shape)")
	top := fs.Int("top", 40, "max candidates to print")
	judge := fs.Bool("judge", false, "adjudicate each candidate with the LLM oracle (needs ANTHROPIC_API_KEY; the precision half)")
	twinsOnly := fs.Bool("twins-only", false, "with --judge, print only candidates the oracle confirms as twins")
	if err := fs.Parse(args); err != nil {
		return
	}

	ents, st, err := code.ExtractSymbols(*repo, splitCSV(*exclude))
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque propose-cross: extracting symbols from %s: %v\n", *repo, err)
		os.Exit(1)
	}
	reg, err := registry.Load(joinRepo(*repo, *regPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque propose-cross: reading registry: %v\n", err)
		os.Exit(1)
	}

	// KeySetCandidates already ranks by shape-match strength; dedup vs the registry.
	cands := code.KeySetCandidates(ents, *minKeys, *keyJac, *maxFanout)
	var fresh []code.SigCandidate
	seen := map[string]bool{}
	for _, c := range cands {
		pk := pairkey.Key(c.A.Key(), c.B.Key())
		if seen[pk] || reg.Has(c.A.Key(), c.B.Key()) {
			continue
		}
		seen[pk] = true
		fresh = append(fresh, c)
	}

	tables, fields := 0, 0
	for _, e := range ents {
		switch e.Kind {
		case "table":
			tables++
		case "corpus-field":
			fields++
		}
	}

	fmt.Println("# calque — cross-substrate candidates (non-function entities, shared key set)")
	fmt.Println()
	fmt.Printf("scanned %d entit(ies) in %d file(s) — %d module-level table(s), %d corpus-field shape(s); %d fresh candidate(s)\n",
		st.Funcs, st.Files, tables, fields, len(fresh))
	if len(fresh) == 0 {
		fmt.Println("\nno candidates. (Needs ≥2 entities whose key/field sets overlap — module-level")
		fmt.Println("tables [.py] or JSON corpus objects [.json].)")
		return
	}
	fmt.Println()
	fmt.Println("Tables / corpus shapes that share a key set — a registry mirrored across files")
	fmt.Println("or substrates that the function axis can't see. High recall, low precision:")
	fmt.Println("adjudicate each (drift / contracted-twin-ok / false-alarm), or pass --judge to")
	fmt.Println("have the oracle do it.")
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
