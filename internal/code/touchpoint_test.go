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

// TestExternalCallNotSeam pins item 1: a non-underscore call name that resolves to
// no project-defined function (a std / extern-crate method like read_to_end, parent)
// must NOT form a cluster, even when three functions share it. A leading-underscore
// private call, or a non-underscore call that IS a project def, still clusters.
func TestExternalCallNotSeam(t *testing.T) {
	mk := func(file, name string, calls []string) *FuncSig {
		f := &FuncSig{File: file, Qualname: "T." + name, Name: name, NLines: 10, Calls: calls}
		f.Prepare()
		return f
	}
	// Three unrelated fetchers sharing only the std methods read_to_end + parent.
	extern := []*FuncSig{
		mk("a.go", "fetchA", []string{"read_to_end", "parent"}),
		mk("b.go", "fetchB", []string{"read_to_end", "parent"}),
		mk("c.go", "fetchC", []string{"read_to_end", "parent"}),
	}
	if cl := ClusterByTouchpoint(extern, DefaultClusterOptions()); len(cl) != 0 {
		t.Fatalf("std-method calls (read_to_end/parent) must not cluster, got %d: %+v", len(cl), cl)
	}

	// Same shape but the shared calls ARE project-defined functions (geomKernel +
	// spanMath are extracted) — real shared private seams, so the trio must cluster.
	// Two shared seams clear the default MinScore the single std-pair would also have
	// cleared, so the only thing that differs is project-def resolution.
	defs := []*FuncSig{
		mk("k.go", "geomKernel", nil), // resolvable targets
		mk("m.go", "spanMath", nil),
		mk("a.go", "deriveA", []string{"geomKernel", "spanMath"}),
		mk("b.go", "deriveB", []string{"geomKernel", "spanMath"}),
		mk("c.go", "deriveC", []string{"geomKernel", "spanMath"}),
	}
	if cl := ClusterByTouchpoint(defs, DefaultClusterOptions()); len(cl) != 1 {
		t.Fatalf("project-defined shared calls (geomKernel/spanMath) must cluster, got %d: %+v", len(cl), cl)
	}
}

// TestSharedConstClustered pins item 13 (the const-set axis): three functions that
// compute one concept through DIFFERENT access patterns share no read-set, call-set,
// or write-set, but all reference the same two domain constants (V_BELOW + V_ROOF).
// The const channel is the only positive signal linking them — and it clusters.
func TestSharedConstClustered(t *testing.T) {
	mk := func(file, name string, consts []string) *FuncSig {
		f := &FuncSig{File: file, Qualname: "T." + name, Name: name, NLines: 20, Consts: consts}
		f.Prepare()
		return f
	}
	// The motivating shape: a building-span audit, its auto-classify twin in a second
	// module, and a third inverted-broadphase copy — no shared reads, same magic values.
	span := []*FuncSig{
		mk("audit.go", "buildingAudit", []string{"V_BELOW", "V_ROOF"}),
		mk("classify.go", "autoClassify", []string{"V_BELOW", "V_ROOF"}),
		mk("trim.go", "trimBuildings", []string{"V_BELOW", "V_ROOF"}),
	}
	clusters := ClusterByTouchpoint(span, DefaultClusterOptions())
	if len(clusters) != 1 {
		t.Fatalf("expected 1 const-set cluster, got %d: %+v", len(clusters), clusters)
	}
	if len(clusters[0].Members) != 3 {
		t.Fatalf("expected 3 members, got %d", len(clusters[0].Members))
	}
	seams := map[string]bool{}
	for _, s := range clusters[0].Shared {
		seams[s.Name] = true
	}
	if !seams["V_BELOW"] || !seams["V_ROOF"] {
		t.Fatalf("expected shared domain constants V_BELOW + V_ROOF, got %v", seams)
	}
}

// TestConstSeamGates pins the two conservatism gates on the const channel: a
// ubiquitous constant is plumbing (fanout-capped), and a SINGLE shared constant
// among three is too weak to cluster by default (1/3 < MinScore 0.40).
func TestConstSeamGates(t *testing.T) {
	mk := func(name string, consts []string) *FuncSig {
		f := &FuncSig{File: name + ".go", Qualname: "T." + name, Name: name, NLines: 20, Consts: consts}
		f.Prepare()
		return f
	}
	// (a) A constant touched by more than MaxFanout (8) functions is plumbing.
	var ubiq []*FuncSig
	for i := 0; i < 10; i++ {
		ubiq = append(ubiq, mk("u"+string(rune('a'+i)), []string{"MAX_GRID"}))
	}
	if cl := ClusterByTouchpoint(ubiq, DefaultClusterOptions()); len(cl) != 0 {
		t.Fatalf("ubiquitous constant MAX_GRID must fanout-cap, got %d: %+v", len(cl), cl)
	}
	// (b) A trio sharing a SINGLE constant scores 1/3 < 0.40 — one shared magic value
	// among three is too weak (the V_BELOW+V_ROOF pair clears it precisely because two).
	single := []*FuncSig{mk("a", []string{"V_ROOF"}), mk("b", []string{"V_ROOF"}), mk("c", []string{"V_ROOF"})}
	if cl := ClusterByTouchpoint(single, DefaultClusterOptions()); len(cl) != 0 {
		t.Fatalf("a single shared constant among 3 must not cluster (0.33 < 0.40), got %d", len(cl))
	}
	// (c) Universal std constants (TAU, NAN) are stop-listed: even two of them shared
	// across a trio (which would otherwise clear MinScore) must not cluster — sharing
	// TAU/NAN is incidental, not a domain twin.
	std := []*FuncSig{
		mk("a", []string{"TAU", "NAN"}), mk("b", []string{"TAU", "NAN"}), mk("c", []string{"TAU", "NAN"}),
	}
	if cl := ClusterByTouchpoint(std, DefaultClusterOptions()); len(cl) != 0 {
		t.Fatalf("std constants TAU/NAN must be stop-listed (commonConsts), got %d: %+v", len(cl), cl)
	}
}

// TestIsDomainConst pins the SCREAMING_SNAKE predicate: domain magic values qualify;
// too-short all-caps, MixedCaps (Go's const convention — deliberately not captured),
// and ordinary identifiers do not.
func TestIsDomainConst(t *testing.T) {
	for _, s := range []string{"V_BELOW", "MAX_RETRIES", "GRID", "V_ROOF", "ABC"} {
		if !isDomainConst(s) {
			t.Errorf("%q should be a domain constant", s)
		}
	}
	for _, s := range []string{"PI", "ID", "vBelow", "VBelow", "_FOO", "road", "compute"} {
		if isDomainConst(s) {
			t.Errorf("%q should NOT be a domain constant", s)
		}
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
