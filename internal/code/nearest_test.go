package code

import "testing"

// has reports whether any returned suspect's twin (the corpus side, Right) is
// the named function. Nearest always puts the query on Left, so the match is
// identified by Right.Name.
func nearestHas(ss []Suspicion, name string) bool {
	for _, s := range ss {
		if s.Right.Name == name {
			return true
		}
	}
	return false
}

// TestExtractPending parses an in-memory post-edit buffer and stamps each sig's
// File with the real repo-relative path (so author-time self-exclusion lines up),
// not the throwaway temp path it was materialized to.
func TestExtractPending(t *testing.T) {
	src := "package x\n\nfunc Foo() int {\n\ttotal := 0\n\treturn total\n}\n"
	sigs, err := ExtractPending(src, ".go", "pkg/real.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(sigs) != 1 || sigs[0].Name != "Foo" {
		t.Fatalf("want one sig named Foo, got %+v", sigs)
	}
	if sigs[0].File != "pkg/real.go" {
		t.Errorf("File = %q, want pkg/real.go (the real path, for self-exclusion)", sigs[0].File)
	}
}

// TestNearestSurfacesTwin is the author-time core proof: given a stub of a
// function about to be written, Nearest returns the existing corpus function
// that already occupies its seam (shared writes+strings), ranks it first, and
// leaves an unrelated function out.
func TestNearestSurfacesTwin(t *testing.T) {
	query := fsig("new.go", "computeBounds", []string{"bounds"}, []string{"box.w", "box.h"}, nil, nil)
	twin := fsig("layout.go", "deriveBounds", []string{"bounds"}, []string{"box.w", "box.h"}, nil, nil)
	unrelated := fsig("flags.go", "parseFlags", []string{"verbose"}, []string{"cfg.debug"}, nil, nil)

	got := Nearest(query, []*FuncSig{unrelated, twin}, 4, 0.15, 5, false)
	if len(got) == 0 {
		t.Fatal("Nearest returned nothing; expected the shared-seam twin")
	}
	if got[0].Right.Name != "deriveBounds" {
		t.Errorf("top match = %q, want deriveBounds", got[0].Right.Name)
	}
	if nearestHas(got, "parseFlags") {
		t.Error("unrelated function surfaced as a twin")
	}
}

// TestNearestSkipsSelf: running Nearest on a function already in the corpus must
// never return that function as its own twin (same Key is dropped), so it's safe
// to query an indexed function for its twins.
func TestNearestSkipsSelf(t *testing.T) {
	q := fsig("layout.go", "deriveBounds", []string{"bounds"}, []string{"box.w", "box.h"}, nil, nil)
	twin := fsig("other.go", "computeBounds", []string{"bounds"}, []string{"box.w", "box.h"}, nil, nil)

	got := Nearest(q, []*FuncSig{q, twin}, 4, 0.15, 5, false)
	if nearestHas(got, "deriveBounds") {
		t.Error("Nearest matched the query against itself")
	}
	if !nearestHas(got, "computeBounds") {
		t.Error("Nearest dropped the real twin while excluding self")
	}
}

// TestNearestCorpusMinLines: the minLines floor applies to the corpus side (its
// quality bar) but NOT to the query — an author-time stub is short by nature.
// A twin below minLines is excluded; the same query against a long-enough twin
// surfaces it.
func TestNearestCorpusMinLines(t *testing.T) {
	query := fsig("new.go", "computeBounds", []string{"bounds"}, []string{"box.w", "box.h"}, nil, nil)
	query.NLines = 1 // a bare stub: must still be allowed to query

	short := fsig("layout.go", "deriveBounds", []string{"bounds"}, []string{"box.w", "box.h"}, nil, nil)
	short.NLines = 2

	if got := Nearest(query, []*FuncSig{short}, 4, 0.15, 5, false); len(got) != 0 {
		t.Errorf("corpus twin below minLines should be excluded, got %d match(es)", len(got))
	}

	short.NLines = 10 // now over the floor
	if got := Nearest(query, []*FuncSig{short}, 4, 0.15, 5, false); len(got) == 0 {
		t.Error("a stub query must still surface a long-enough corpus twin")
	}
}
