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

	cands := SharedDerivationCandidates([]*FuncSig{a, b, d, r, e}, 2, 0.5, 8, false)
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
