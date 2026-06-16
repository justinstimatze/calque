package main

// confess — the drift-confessing-comment axis. A comment that says a function
// MIRRORS another, must be KEPT IN SYNC, or is a COPY OF something is a literal
// self-witness that the function is one side of a twin. Cheap (regex over source),
// high precision, and it catches the twin class whose data-flow shape has drifted
// far enough that the reads / signature passes miss it. GENERATOR + audit: stdout
// only, no writes, no exit code.

import (
	"flag"
	"fmt"
	"os"

	"github.com/justinstimatze/calque/internal/code"
)

func runConfess(args []string) {
	fs := flag.NewFlagSet("confess", flag.ContinueOnError)
	repo := fs.String("repo", ".", "repo root to scan")
	exclude := fs.String("exclude", "", "comma-separated glob(s) to skip entirely (e.g. node_modules/**)")
	if err := fs.Parse(args); err != nil {
		return
	}

	sigs, st, err := code.Extract(*repo, splitCSV(*exclude))
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque confess: walking %s: %v\n", *repo, err)
		os.Exit(1)
	}
	confs := code.FindConfessions(sigs, *repo)
	cands := code.ConfessionCandidates(confs, sigs)

	fmt.Println("# calque — drift-confessing comments (self-witnessed twins)")
	fmt.Println()
	fmt.Printf("scanned %d func(s) in %d file(s); %d confession(s); %d directed twin candidate(s)\n",
		st.Funcs, st.Files, len(confs), len(cands))
	if len(confs) == 0 {
		fmt.Println("\nno drift-confessing comments. (Looking for \"mirrors X\", \"keep in sync\", \"must match\", \"copy of\", …)")
		return
	}
	fmt.Println()
	fmt.Println("A comment saying a function MIRRORS / must stay in sync with another is a literal")
	fmt.Println("self-witness of a twin. Verify each is actually in sync — or collapse to one")
	fmt.Println("authority. Directed candidates name a resolvable twin; the census lists every confession.")

	if len(cands) > 0 {
		fmt.Println("\n## Directed twin candidates (the comment names a resolvable function)")
		fmt.Println()
		for i, c := range cands {
			printCandidate(i+1, c, nil)
		}
	}

	fmt.Println("\n## Confession census (every drift-confessing comment)")
	fmt.Println()
	for _, cf := range confs {
		fmt.Printf("- `%s` (%s:%d) — %q\n", cf.Func.Qualname, cf.Func.File, cf.Line, cf.Text)
	}
}
