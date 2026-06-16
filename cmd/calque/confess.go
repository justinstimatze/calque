package main

// confess — the drift-confessing-comment axis. A comment that says a function
// MIRRORS another, must be KEPT IN SYNC, or is a COPY OF something is a literal
// self-witness that the function is one side of a twin. Cheap (regex over source),
// high precision, and it catches the twin class whose data-flow shape has drifted
// far enough that the reads / signature passes miss it. GENERATOR + audit: stdout
// only, no writes, no exit code. --judge adjudicates the directed twin candidates
// (the comment names a resolvable function) and records each verdict to the Layer D
// label store under the `confession` detector.

import (
	"flag"
	"fmt"
	"os"

	"github.com/justinstimatze/calque/internal/code"
	"github.com/justinstimatze/calque/internal/pairkey"
	"github.com/justinstimatze/calque/internal/registry"
)

func runConfess(args []string) {
	fs := flag.NewFlagSet("confess", flag.ContinueOnError)
	repo := fs.String("repo", ".", "repo root to scan")
	exclude := fs.String("exclude", "", "comma-separated glob(s) to skip entirely (e.g. node_modules/**); test files are excluded by default")
	includeTests := fs.Bool("include-tests", false, "scan test files too (excluded by default — a confession in a test fixture rarely names a production twin)")
	includeProse := fs.Bool("include-prose", false, "also emit directed candidates from the figurative \"prose\" register (docstring/block-comment narrative); by default only literal line-comment confessions become candidates (the high-precision register)")
	regPath := fs.String("registry", ".calque/registry.md", "registry file (dedup directed candidates vs already-adjudicated pairs)")
	top := fs.Int("top", 40, "max directed candidates to judge/print")
	judge := fs.Bool("judge", false, "adjudicate each directed twin candidate with the LLM oracle (needs ANTHROPIC_API_KEY; the precision half)")
	twinsOnly := fs.Bool("twins-only", false, "with --judge, print only candidates the oracle confirms as twins")
	if err := fs.Parse(args); err != nil {
		return
	}

	excl := splitCSV(*exclude)
	if !*includeTests {
		excl = append(excl, testGlobs...)
	}
	sigs, st, err := code.Extract(*repo, excl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque confess: walking %s: %v\n", *repo, err)
		os.Exit(1)
	}
	confs := code.FindConfessions(sigs, *repo)
	cands := code.ConfessionCandidates(confs, sigs, *includeProse)

	// Dedup the directed candidates vs the registry so --judge never re-pays for an
	// already-adjudicated pair (the census stays whole — every confession shows).
	reg, err := registry.Load(joinRepo(*repo, *regPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque confess: reading registry: %v\n", err)
		os.Exit(1)
	}
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

	var prose int
	for _, c := range confs {
		if c.Register == "prose" {
			prose++
		}
	}

	fmt.Println("# calque — drift-confessing comments (self-witnessed twins)")
	fmt.Println()
	testNote := " · tests excluded (--include-tests to scan them)"
	if *includeTests {
		testNote = " · tests included"
	}
	proseNote := ""
	if prose > 0 && !*includeProse {
		proseNote = fmt.Sprintf(" · %d in the figurative \"prose\" register gated from candidates (--include-prose to keep)", prose)
	}
	fmt.Printf("scanned %d func(s) in %d file(s); %d confession(s); %d fresh directed twin candidate(s)%s%s\n",
		st.Funcs, st.Files, len(confs), len(fresh), testNote, proseNote)
	if len(confs) == 0 {
		fmt.Println("\nno drift-confessing comments. (Looking for \"mirrors X\", \"keep in sync\", \"must match\", \"copy of\", …)")
		return
	}
	fmt.Println()
	fmt.Println("A comment saying a function MIRRORS / must stay in sync with another is a literal")
	fmt.Println("self-witness of a twin. Verify each is actually in sync — or collapse to one")
	fmt.Println("authority. Directed candidates name a resolvable twin; the census lists every confession.")

	if len(fresh) > 0 {
		fmt.Println("\n## Directed twin candidates (the comment names a resolvable function)")
		fmt.Println()
		if len(fresh) > *top {
			fmt.Printf("(showing top %d of %d)\n\n", *top, len(fresh))
			fresh = fresh[:*top]
		}
		if *judge {
			runJudge(*repo, fresh, *twinsOnly)
		} else {
			for i, c := range fresh {
				printCandidate(i+1, c, nil)
			}
		}
	}

	fmt.Println("\n## Confession census (every drift-confessing comment; [register] line=literal prose=figurative)")
	fmt.Println()
	for _, cf := range confs {
		fmt.Printf("- [%s] `%s` (%s:%d) — %q\n", cf.Register, cf.Func.Qualname, cf.Func.File, cf.Line, cf.Text)
	}
}
