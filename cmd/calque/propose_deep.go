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
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/justinstimatze/calque/internal/code"
	"github.com/justinstimatze/calque/internal/llm"
	"github.com/justinstimatze/calque/internal/pairkey"
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
	noNameStem := fs.Bool("no-name-stem", false, "skip the name-stem recall pass (signature only)")
	nameJac := fs.Float64("name-jaccard", 0.6, "name-stem pass: min name-token jaccard to pair (1.0 = identical token set)")
	nameFanout := fs.Int("name-max-fanout", 8, "name-stem pass: skip stems shared by more than this many functions")
	judge := fs.Bool("judge", false, "adjudicate each candidate with the LLM oracle (needs ANTHROPIC_API_KEY; the precision half)")
	twinsOnly := fs.Bool("twins-only", false, "with --judge, print only candidates the oracle confirms as twins")
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

	// Union two representation-independent recall passes: signature (TS-only, rare
	// domain-typed signature) + name-stem (every language, near-identical role name).
	// Signature candidates take precedence on conflict; dedup vs already-adjudicated.
	sigCands := code.SignatureCandidates(sigs, *minLines, *minMembers, *maxMembers)
	var nameCands []code.SigCandidate
	if !*noNameStem {
		nameCands = code.NameStemCandidates(sigs, *minLines, *nameJac, *nameFanout)
	}
	seen := map[string]bool{}
	var fresh []code.SigCandidate
	addCands := func(cs []code.SigCandidate) {
		for _, c := range cs {
			pk := pairkey.Key(c.A.Key(), c.B.Key())
			if seen[pk] || reg.Has(c.A.Key(), c.B.Key()) {
				continue
			}
			seen[pk] = true
			fresh = append(fresh, c)
		}
	}
	addCands(sigCands)
	addCands(nameCands)

	// Unified ranking across both kinds: most gate-INVISIBLE first (lowest jaccard
	// score = the pair the existing scan/check gate is most blind to = the highest
	// unique Type-4 value). Stable, so each kind's own ordering breaks ties.
	sort.SliceStable(fresh, func(i, j int) bool {
		if fresh[i].Jaccard != fresh[j].Jaccard {
			return fresh[i].Jaccard < fresh[j].Jaccard
		}
		return fresh[i].CrossFile && !fresh[j].CrossFile
	})

	withSig := 0
	for _, f := range sigs {
		if f.Sig != "" {
			withSig++
		}
	}

	fmt.Println("# calque — Type-4 candidates (shared contract, representation-independent)")
	fmt.Println()
	fmt.Printf("scanned %d func(s) in %d file(s); %d with a type signature; %d fresh candidate(s) (%d signature, %d name-stem)\n",
		st.Funcs, st.Files, withSig, len(fresh), len(sigCands), len(nameCands))
	if len(fresh) == 0 {
		fmt.Println("\nno candidates. (Signature recall needs TS/TSX; name-stem needs near-identical role names.)")
		return
	}
	fmt.Println()
	fmt.Println("Twins by shared contract — a rare type SIGNATURE (no surface tokens) or a")
	fmt.Println("near-identical NAME role — that the jaccard gate scores near zero (`jac`). High")
	fmt.Println("recall, low precision: adjudicate each (drift / contracted-twin-ok / false-alarm),")
	fmt.Println("or pass --judge to have the oracle do it.")
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

// printCandidate renders one candidate, with the oracle's verdict when present.
func printCandidate(n int, c code.SigCandidate, v *llm.Verdict) {
	fmt.Printf("## %d. [%s] `%s`  (jac %.2f%s)\n", n, c.Kind, c.Sig, c.Jaccard, crossFileMark(c.CrossFile))
	fmt.Printf("- `%s` (%s:%d)\n", c.A.Qualname, c.A.File, c.A.Line)
	fmt.Printf("- `%s` (%s:%d)\n", c.B.Qualname, c.B.File, c.B.Line)
	if v != nil {
		fmt.Printf("  oracle: %s (%s) — %s\n", v.Class, v.Confidence, v.Reason)
	}
	fmt.Printf("  adjudicate:  - pair: %s::%s | %s::%s\n", c.A.File, c.A.Qualname, c.B.File, c.B.Qualname)
}

// runJudge adjudicates the candidates with the LLM oracle (bounded concurrency,
// disk-cached) and prints each with its verdict. Generator semantics are preserved:
// stdout only, no writes, no exit code.
func runJudge(repo string, cands []code.SigCandidate, twinsOnly bool) {
	j, err := llm.NewJudge()
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque propose-deep --judge: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\noracle: judging %d candidate(s) with %s (cached results are free)...\n\n", len(cands), j.Model())

	verdicts := make([]*llm.Verdict, len(cands))
	const workers = 4
	var wg sync.WaitGroup
	ch := make(chan int)
	var mu sync.Mutex
	var failed int
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range ch {
				c := cands[i]
				in := llm.PairInput{
					AKey:    c.A.File + "::" + c.A.Qualname,
					ASource: readEntitySource(repo, c.A, 200),
					BKey:    c.B.File + "::" + c.B.Qualname,
					BSource: readEntitySource(repo, c.B, 200),
					Sig:     c.Sig,
				}
				v, err := j.JudgePair(context.Background(), in)
				if err != nil {
					mu.Lock()
					failed++
					mu.Unlock()
					fmt.Fprintf(os.Stderr, "  judge %s ≟ %s: %v\n", c.A.Qualname, c.B.Qualname, err)
					continue
				}
				verdicts[i] = &v
			}
		}()
	}
	for i := range cands {
		ch <- i
	}
	close(ch)
	wg.Wait()

	var drift, contracted, falseAlarm, shown int
	for i, c := range cands {
		v := verdicts[i]
		if v == nil {
			continue // errored — already reported to stderr
		}
		switch v.Class {
		case "drift":
			drift++
		case "contracted-twin-ok":
			contracted++
		default:
			falseAlarm++
		}
		if twinsOnly && !v.IsTwin() {
			continue
		}
		shown++
		printCandidate(shown, c, v)
	}
	fmt.Printf("\noracle: %d drift · %d contracted-twin-ok · %d false-alarm (of %d judged)",
		drift, contracted, falseAlarm, len(cands)-failed)
	if failed > 0 {
		fmt.Printf(" · %d errored", failed)
	}
	fmt.Println()
	if drift > 0 {
		fmt.Println("drift = independent impls that can diverge → collapse to one source.")
	}
}

// readFuncSource reads a function's source text from disk via File+Line+NLines,
// capped at maxLines to bound the oracle's token cost. Best-effort: an unreadable
// file yields an empty body (the oracle still has the signature + name).
func readFuncSource(repo string, f *code.FuncSig, maxLines int) string {
	fh, err := os.Open(joinRepo(repo, f.File))
	if err != nil {
		return ""
	}
	defer fh.Close()
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	start := f.Line
	end := f.Line + f.NLines
	if f.NLines > maxLines {
		end = f.Line + maxLines
	}
	var b []string
	ln := 0
	for sc.Scan() {
		ln++
		if ln >= start && ln < end {
			b = append(b, sc.Text())
		}
		if ln >= end {
			break
		}
	}
	out := ""
	for _, l := range b {
		out += l + "\n"
	}
	return out
}

func crossFileMark(cross bool) string {
	if cross {
		return ", cross-file"
	}
	return ""
}
