package code

import (
	"strings"
	"testing"
)

// bigBody returns n unique filler tokens prefixed with p, to pad a FuncSig so a
// shared seam is diluted in pairwise Jaccard (the dilution case, §15).
func bigBody(p string, n int) []string {
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = p + "_filler_" + string(rune('a'+i))
	}
	return out
}

// makeShell builds a large method that, beneath its unique bulk, inlines the
// same private seam: a `_parse_cmd` call and an `_canon_cache` read+write.
func makeShell(typ, method string) *FuncSig {
	f := &FuncSig{
		File:     typ + ".go",
		Qualname: typ + "." + method,
		Name:     method,
		NLines:   40,
		Calls:    append([]string{"_parse_cmd"}, bigBody(method+"c", 12)...),
		Strings:  append([]string{"_canon_cache"}, bigBody(method+"s", 12)...),
		Writes:   append([]string{"_canon_cache"}, bigBody(method+"w", 12)...),
	}
	f.Prepare()
	return f
}

// The headline §15 case: three shells (two `step`s + a `run`) each inline
// [_parse_cmd -> _canon_cache]. Pairwise Jaccard drowns the seam and can't
// express a trio; the touchpoint pass must surface all three as one cluster.
func TestTripleShellClustered(t *testing.T) {
	shells := []*FuncSig{
		makeShell("Engine", "step"),
		makeShell("Session", "stepWeb"),
		makeShell("Engine", "run"),
	}

	// Pairwise must MISS it (that's the gap this pass closes).
	if pairs := Rank(shells, shells, 4, 0.18, 50); len(pairs) != 0 {
		t.Fatalf("expected pairwise to miss the diluted triple, got %d pair(s): %+v", len(pairs), pairs)
	}

	clusters := ClusterByTouchpoint(shells, DefaultClusterOptions())
	if len(clusters) != 1 {
		t.Fatalf("expected exactly 1 cluster, got %d: %+v", len(clusters), clusters)
	}
	c := clusters[0]
	if len(c.Members) != 3 {
		t.Fatalf("expected 3 members in the triple-shell cluster, got %d", len(c.Members))
	}
	seams := map[string]bool{}
	for _, s := range c.Shared {
		seams[s.Name] = true
	}
	if !seams["_parse_cmd"] || !seams["_canon_cache"] {
		t.Fatalf("expected shared seams _parse_cmd and _canon_cache, got %v", seams)
	}
}

// A symbol touched by many functions is plumbing, not a seam — it must not form a
// cluster (the MaxFanout guard).
func TestHighFanoutNotClustered(t *testing.T) {
	var fns []*FuncSig
	for i := 0; i < 12; i++ {
		f := &FuncSig{
			File:     "f.go",
			Qualname: "T.m" + string(rune('a'+i)),
			Name:     "m" + string(rune('a'+i)),
			NLines:   10,
			Calls:    []string{"_helper"}, // touched by all 12 -> fanout 12 > MaxFanout
		}
		f.Prepare()
		fns = append(fns, f)
	}
	if clusters := ClusterByTouchpoint(fns, DefaultClusterOptions()); len(clusters) != 0 {
		t.Fatalf("high-fanout plumbing must not cluster, got %d", len(clusters))
	}
}

// Public/exported symbols are not seams (everyone calls them) — only private ones.
func TestPublicSymbolNotSeam(t *testing.T) {
	if isSeam("Printf") || isSeam("Step") {
		t.Fatal("exported symbols must not be treated as private seams")
	}
	if !isSeam("_parse_cmd") || !isSeam("parseCmd") {
		t.Fatal("leading-underscore and lower-first identifiers must be seams")
	}
	if isSeam("__init__") || isSeam("i") {
		t.Fatal("dunders and one-letter names must not be seams")
	}
}

// A 2-member coincidence below MinMembers must not be reported by default.
func TestPairBelowMinMembers(t *testing.T) {
	a := makeShell("A", "one")
	b := makeShell("B", "two")
	if clusters := ClusterByTouchpoint([]*FuncSig{a, b}, DefaultClusterOptions()); len(clusters) != 0 {
		t.Fatalf("2-member cluster must be suppressed at default MinMembers=3, got %d", len(clusters))
	}
	// ...but reachable when MinMembers is lowered to 2.
	opts := DefaultClusterOptions()
	opts.MinMembers = 2
	if clusters := ClusterByTouchpoint([]*FuncSig{a, b}, opts); len(clusters) != 1 {
		t.Fatalf("expected the pair as a cluster at MinMembers=2, got %d", len(clusters))
	}
}

func TestClusterKeyOrderIndependent(t *testing.T) {
	x := &FuncSig{File: "a.go", Qualname: "T.x"}
	y := &FuncSig{File: "b.go", Qualname: "T.y"}
	c1 := Cluster{Members: []*FuncSig{x, y}}
	c2 := Cluster{Members: []*FuncSig{y, x}}
	if c1.Key() != c2.Key() {
		t.Fatalf("cluster key must be order-independent: %q != %q", c1.Key(), c2.Key())
	}
	if !strings.Contains(c1.Key(), "a.go::T.x") {
		t.Fatalf("cluster key should contain member keys, got %q", c1.Key())
	}
}
