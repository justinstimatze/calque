package main

import (
	"strings"
	"testing"

	"github.com/justinstimatze/calque/internal/code"
)

// TestBoundaryBiteWarnings pins the false-clean guard: a boundary side whose glob
// matches files on disk but parses zero functions must be loudly flagged, not
// reported as a clean bill. .java has no extractor, so it stands in for any
// unsupported / not-yet-implemented language on one side.
func TestBoundaryBiteWarnings(t *testing.T) {
	sig := func(file string) *code.FuncSig { return &code.FuncSig{File: file, Name: "f"} }

	// (1) left matched 2 unsupported files, parsed nothing → cannot-bite warning.
	w := boundaryBiteWarnings("**/*.java", "", nil, []string{"src/A.java", "src/B.java"})
	if len(w) != 1 || !strings.Contains(w[0], "boundary cannot bite") || !strings.Contains(w[0], "left") ||
		!strings.Contains(w[0], "no extractor for .java") || !strings.Contains(w[0], "2 file(s)") {
		t.Fatalf("expected one cannot-bite warning naming .java/2 files, got %v", w)
	}

	// (2) a side that bit normally (matched files AND parsed funcs) → no warning.
	all := []*code.FuncSig{sig("src/a.go"), sig("src/b.go")}
	if w := boundaryBiteWarnings("**/*.go", "", all, []string{"src/a.go", "src/b.go"}); len(w) != 0 {
		t.Errorf("a healthy boundary must not warn, got %v", w)
	}

	// (3) the whole-repo default (empty glob) can never under-bite → no warning.
	if w := boundaryBiteWarnings("", "", nil, []string{"src/A.java"}); len(w) != 0 {
		t.Errorf("empty glob must not warn, got %v", w)
	}

	// (4) glob matches nothing on disk → a visibly empty side, not a false clean.
	if w := boundaryBiteWarnings("**/*.rb", "", nil, []string{"src/A.java"}); len(w) != 0 {
		t.Errorf("glob matching no files must not warn, got %v", w)
	}

	// (5) partial coverage: the side parsed SOME funcs but also matched files of an
	// unsupported type → a softer "partial coverage" warning, not cannot-bite.
	all = []*code.FuncSig{sig("src/a.go")}
	codeFiles := []string{"src/a.go", "src/legacy.java"}
	w = boundaryBiteWarnings("src/**", "", all, codeFiles)
	if len(w) != 1 || !strings.Contains(w[0], "partial coverage") || !strings.Contains(w[0], ".java") {
		t.Fatalf("expected one partial-coverage warning naming .java, got %v", w)
	}

	// (6) the right side is checked too, and named as "right".
	w = boundaryBiteWarnings("", "**/*.java", nil, []string{"src/A.java"})
	if len(w) != 1 || !strings.Contains(w[0], "right") {
		t.Fatalf("expected a warning for the right side, got %v", w)
	}
}
