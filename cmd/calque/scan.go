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

func runScan(args []string) {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	repo := fs.String("repo", ".", "repo root to scan")
	left := fs.String("left", "", "comma-separated glob(s) for the A side (default: all source)")
	right := fs.String("right", "", "comma-separated glob(s) for the B side (default: all source)")
	top := fs.Int("top", 30, "max suspect pairs to report")
	minScore := fs.Float64("min-score", 0.18, "minimum suspicion score to report")
	minLines := fs.Int("min-lines", 4, "ignore functions shorter than this many lines")
	if err := fs.Parse(args); err != nil {
		return
	}

	all, st, err := code.Extract(*repo)
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
}

func orAll(s string) string {
	if strings.TrimSpace(s) == "" {
		return "**/* (all source)"
	}
	return s
}
