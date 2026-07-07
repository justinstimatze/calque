package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/justinstimatze/calque/internal/code"
	"github.com/justinstimatze/calque/internal/llm"
)

// Synthetic-corpus Type-4 recall measurement (reproducible; skips without a key +
// CALQUE_PROBE_REPO, so CI stays green). Ground truth by CONSTRUCTION: rewrite real
// functions into behaviorally-identical, textually-dissimilar variants (known Type-4
// twins), then measure whether the judge recalls them. A behavior-preserving rewrite
// the judge calls "false-alarm" is a recall miss. Set CALQUE_REWRITE_MODEL to a model
// DIFFERENT from the judge (e.g. claude-haiku-4-5) to break the same-model
// self-recognition confound — recall held at 8/8 de-confounded (2026-06-10).
//
//	CALQUE_PROBE_REPO=/path/to/repo CALQUE_TS=... ANTHROPIC_API_KEY=... \
//	  CALQUE_SYNTH_N=8 CALQUE_REWRITE_MODEL=claude-haiku-4-5 \
//	  go test ./cmd/calque/ -run TestSynthRecall -v -timeout 900s
func TestSynthRecall(t *testing.T) {
	repo := os.Getenv("CALQUE_PROBE_REPO")
	if repo == "" || (os.Getenv("ANTHROPIC_API_KEY") == "" && os.Getenv("CALQUE_API_KEY") == "") {
		t.Skip("needs CALQUE_PROBE_REPO + a key")
	}
	n := 8
	if v, err := strconv.Atoi(os.Getenv("CALQUE_SYNTH_N")); err == nil && v > 0 {
		n = v
	}

	all, _, err := code.Extract(repo, []string{"node_modules/**", "dist/**", "**/*.d.ts", "**/__tests__/**", "**/*.test.ts", "**/*.spec.ts"})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range all {
		f.Prepare()
	}
	// Pick originals with rare, informative signatures (the A-side of the candidate
	// pass), deduped, with a substantial body to make the rewrite meaningful.
	cands := code.SignatureCandidates(all, code.SizeGate{MinLines: 4}, 2, 6)
	var originals []*code.FuncSig
	seen := map[string]bool{}
	for _, c := range cands {
		f := c.A
		k := f.File + "::" + f.Qualname
		if seen[k] || f.NLines < 10 || f.NLines > 60 {
			continue
		}
		seen[k] = true
		originals = append(originals, f)
		if len(originals) == n {
			break
		}
	}
	if len(originals) < n {
		t.Logf("only %d suitable originals found (wanted %d)", len(originals), n)
		n = len(originals)
	}

	j, err := llm.NewJudge()
	if err != nil {
		t.Fatal(err)
	}
	// De-confound: rewrite with a DIFFERENT model than we judge with, if set.
	rewriter := j
	if rm := os.Getenv("CALQUE_REWRITE_MODEL"); rm != "" {
		if r, err := llm.NewJudgeModel(rm); err == nil {
			rewriter = r
		}
	}
	fmt.Printf("rewriter=%s  judge=%s\n", rewriter.Model(), j.Model())

	type result struct {
		key     string
		sig     string
		variant string
		verdict llm.Verdict
		err     error
	}
	results := make([]result, n)
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			f := originals[i]
			src := readFuncSource(repo, f, 200)
			results[i].key = f.File + "::" + f.Qualname
			results[i].sig = f.Sig
			variant, err := rewriter.Rewrite(context.Background(), src)
			if err != nil {
				results[i].err = fmt.Errorf("rewrite: %w", err)
				return
			}
			results[i].variant = variant
			v, err := j.JudgePair(context.Background(), llm.PairInput{
				AKey: results[i].key, ASource: src, BKey: results[i].key + " (rewritten variant)", BSource: variant, Sig: f.Sig,
			})
			if err != nil {
				results[i].err = fmt.Errorf("judge: %w", err)
				return
			}
			results[i].verdict = v
		}(i)
	}
	wg.Wait()

	var twin, drift, falseNeg, errored int
	fmt.Printf("\n=== synthetic Type-4 recall: %d behavior-preserving rewrites judged ===\n", n)
	for _, r := range results {
		if r.err != nil {
			errored++
			fmt.Printf("ERR   %s: %v\n", r.key, r.err)
			continue
		}
		switch {
		case r.verdict.Class == "drift":
			drift++
			twin++
			fmt.Printf("drift        %s — %s\n", r.key, r.verdict.Reason)
		case r.verdict.IsTwin():
			twin++
			fmt.Printf("twin(%s) %s — %s\n", r.verdict.Class, r.key, r.verdict.Reason)
		default:
			falseNeg++
			fmt.Printf("MISS(false-alarm) %s — %s\n", r.key, r.verdict.Reason)
		}
	}
	judged := n - errored
	fmt.Printf("\nJUDGE RECALL on synthetic twins: %d/%d classified twin (%d drift); %d false-negative; %d errored\n",
		twin, judged, drift, falseNeg, errored)
	fmt.Println("NOTE: ground truth is by construction (behavior-preserving rewrite). Same-model")
	fmt.Println("rewriter+judge is a residual confound — sample rewrite below for behavior spot-check.")
	// Print one rewrite for manual behavior-preservation inspection.
	for _, r := range results {
		if r.err == nil && r.variant != "" {
			fmt.Printf("\n--- sample original: %s ---\n%s\n--- variant ---\n%s\n", r.key, readFuncSource(repo, originals[0], 200), r.variant)
			break
		}
	}
}
