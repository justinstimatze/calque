package code

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExtractGoReads pins the read-set rule on the always-available Go extractor:
// a `+=` target and a call-argument field are reads; a plain-`=` LHS is a pure write
// and must NOT appear in reads (the derivation-input footprint).
func TestExtractGoReads(t *testing.T) {
	dir := t.TempDir()
	src := `package p

type Road struct{ width float64 }
type Vehicle struct {
	speed float64
	gear  int
	road  Road
}

func (v *Vehicle) ApplyThrottle(amount float64) {
	v.speed += amount
	v.gear = pickGear(v.road.width)
}

func pickGear(w float64) int { return 1 }
`
	f := filepath.Join(dir, "vehicle.go")
	if err := os.WriteFile(f, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	sigs, err := extractGoBatch([]string{f}, dir)
	if err != nil {
		t.Fatalf("extractGoBatch: %v", err)
	}
	at := sigByQual(sigs, "Vehicle.ApplyThrottle")
	if at == nil {
		t.Fatalf("Vehicle.ApplyThrottle not extracted; got %d sigs", len(sigs))
	}
	// `v.speed` (+=) and `v.road.width` (call arg) are read; the prefix `road` too.
	assertHas(t, "ApplyThrottle.reads", at.Reads, []string{"speed", "road.width", "road"})
	// `v.gear` is a plain-`=` LHS — a pure write, excluded from reads.
	for _, r := range at.Reads {
		if r == "gear" {
			t.Errorf("ApplyThrottle.reads must exclude the plain-= LHS 'gear', got %v", at.Reads)
		}
	}
	assertHas(t, "ApplyThrottle.writes", at.Writes, []string{"speed", "gear"})
}

func TestSharedDerivationCandidates(t *testing.T) {
	mk := func(file, qual string, reads, writes, ret []string, delegates bool) *FuncSig {
		f := &FuncSig{File: file, Qualname: qual, Name: qual, Reads: reads, Writes: writes, RetKeys: ret, Delegates: delegates}
		f.Prepare()
		return f
	}
	shared := []string{"road.width", "road.pieces", "terrain.height"}
	// buildRoad/renderRibbon both CONSTRUCT a value (return a record) from the same
	// input field-set — the strong derivation variety, surfaced by default.
	a := mk("worldgen/build.go", "buildRoad", shared, []string{"mesh"}, []string{"mesh"}, false)
	b := mk("client/render.go", "renderRibbon", shared, []string{"verts"}, []string{"verts"}, false)
	d := mk("client/adapter.go", "adapt", shared, []string{"out"}, []string{"out"}, true)                             // delegates → excluded
	r := mk("client/log.go", "logWidths", shared, nil, nil, false)                                                    // no writes/ret → not a derivation
	e := mk("x/y.go", "unrelated", []string{"road.width", "foo.bar", "baz.qux"}, []string{"z"}, []string{"z"}, false) // jaccard < 0.5

	cands := SharedDerivationCandidates([]*FuncSig{a, b, d, r, e}, 2, 0.5, 8, false, false)
	if len(cands) != 1 {
		t.Fatalf("want exactly 1 candidate (buildRoad≟renderRibbon), got %d: %+v", len(cands), cands)
	}
	c := cands[0]
	pair := map[string]bool{c.A.Qualname: true, c.B.Qualname: true}
	if !pair["buildRoad"] || !pair["renderRibbon"] {
		t.Errorf("expected buildRoad≟renderRibbon, got %s≟%s", c.A.Qualname, c.B.Qualname)
	}
	if c.Kind != "read-set" {
		t.Errorf("Kind = %q, want read-set", c.Kind)
	}
	if !c.CrossFile {
		t.Error("buildRoad≟renderRibbon should be CrossFile")
	}
}

// TestStructuralShareGate pins item 10: when the fields two functions SHARE are a
// small whole-object field set (a Pose's bare {x,z,hdg}), the overlap is structural
// co-access of one struct, not derivation drift — gated by default, opt-in via the
// includeStructural flag. A pair sharing a DOTTED domain path is a specific quantity
// and must survive (not discounted).
func TestStructuralShareGate(t *testing.T) {
	mk := func(qual string, reads, ret []string) *FuncSig {
		f := &FuncSig{File: qual + ".go", Qualname: qual, Name: qual, Reads: reads, RetKeys: ret}
		f.Prepare()
		return f
	}
	pose := []string{"x", "z", "hdg"} // bare whole-Pose field set → structural
	a := mk("distanceTo", pose, []string{"d"})
	b := mk("headingError", pose, []string{"e"})
	// A real twin sharing a DOTTED domain quantity must NOT be discounted.
	dotted := []string{"road.width", "road.pieces", "terrain.height"}
	c1 := mk("buildRoad", dotted, []string{"mesh"})
	c2 := mk("renderRibbon", dotted, []string{"verts"})
	all := []*FuncSig{a, b, c1, c2}

	pairSet := func(cands []SigCandidate) map[string]bool {
		m := map[string]bool{}
		for _, c := range cands {
			m[c.A.Qualname+"|"+c.B.Qualname] = true
			m[c.B.Qualname+"|"+c.A.Qualname] = true
		}
		return m
	}

	def := pairSet(SharedDerivationCandidates(all, 2, 0.5, 8, false, false))
	if def["distanceTo|headingError"] {
		t.Error("whole-Pose field share (distanceTo ≟ headingError) should be gated by default")
	}
	if !def["buildRoad|renderRibbon"] {
		t.Error("dotted-domain-path twin (buildRoad ≟ renderRibbon) must survive the structural gate")
	}

	with := pairSet(SharedDerivationCandidates(all, 2, 0.5, 8, false, true))
	if !with["distanceTo|headingError"] {
		t.Error("distanceTo ≟ headingError should surface with includeStructural=true")
	}
}
