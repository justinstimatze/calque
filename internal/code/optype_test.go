package code

import "testing"

func mkOp(name string, reads, writes, ret []string) *FuncSig {
	f := &FuncSig{File: "f.go", Qualname: name, Name: name, Reads: reads, Writes: writes, RetKeys: ret}
	f.Prepare()
	return f
}

func TestOpTypeClassify(t *testing.T) {
	cases := []struct {
		name        string
		writes, ret []string
		want        string
	}{
		{"sampleEdge", nil, []string{"x"}, "forward-map"},               // name beats the ret_keys → forward, not construct
		{"projectPoint", nil, nil, "inverse-search"},                    // name
		{"continuity", nil, nil, "measure"},                             // name
		{"buildRibbon", []string{"m"}, []string{"v", "u"}, "construct"}, // ret_keys → construct
		{"applyThrottle", []string{"speed"}, nil, "mutate"},             // writes, no ret, no name signal
		{"frobnicate", nil, nil, ""},                                    // nothing fires
	}
	for _, c := range cases {
		got := opType(mkOp(c.name, []string{"a.b"}, c.writes, c.ret))
		if got != c.want {
			t.Errorf("opType(%s) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestOpTypeGateSuppression(t *testing.T) {
	shared := []string{"road.pieces", "road.width", "terrain.h"}
	sample := mkOp("sample", shared, nil, []string{"x", "y"})                  // forward-map
	project := mkOp("project", shared, nil, nil)                               // inverse-search
	continuity := mkOp("continuity", shared, nil, nil)                         // measure
	build := mkOp("buildRibbon", shared, []string{"mesh"}, []string{"v", "u"}) // construct
	sampleB := mkOp("sampleEdge", shared, nil, []string{"x", "y"})             // forward-map (twin of sample)
	mutA := mkOp("applyDrift", shared, []string{"out"}, nil)                   // mutate (bare field-mutator)
	mutB := mkOp("applyTrim", shared, []string{"out"}, nil)                    // mutate (twin of applyDrift)
	all := []*FuncSig{sample, project, continuity, build, sampleB, mutA, mutB}
	for _, f := range all {
		f.Reads = shared
		f.Prepare()
	}

	pairSet := func(cands []SigCandidate) map[string]bool {
		pairs := map[string]bool{}
		for _, c := range cands {
			pairs[c.A.Qualname+"|"+c.B.Qualname] = true
			pairs[c.B.Qualname+"|"+c.A.Qualname] = true
		}
		return pairs
	}

	pairs := pairSet(SharedDerivationCandidates(all, 2, 0.5, 8, false, false, false))
	// Provably-dual pairs suppressed.
	if pairs["sample|project"] {
		t.Error("forward-map ≟ inverse-search (sample ≟ project) should be suppressed")
	}
	if pairs["continuity|buildRibbon"] {
		t.Error("measure ≟ construct (continuity ≟ buildRibbon) should be suppressed")
	}
	// Same-operation twin kept.
	if !pairs["sample|sampleEdge"] {
		t.Error("two forward maps sharing the read-set (sample ≟ sampleEdge) should survive the gate")
	}
	// Bare mutator twin gated by default — the read-set's dominant false-twin variety.
	if pairs["applyDrift|applyTrim"] {
		t.Error("two bare mutators (applyDrift ≟ applyTrim) should be gated by default")
	}

	// ...and surfaced when the user opts back in with includeMutators=true.
	withMut := pairSet(SharedDerivationCandidates(all, 2, 0.5, 8, true, false, false))
	if !withMut["applyDrift|applyTrim"] {
		t.Error("applyDrift ≟ applyTrim should surface with includeMutators=true")
	}
}
