package code

import (
	"testing"
)

// TestCallerStemIndexUnionsCallerStems pins the core invariant: every stem
// token of every caller of a given callee ends up in that callee's index
// entry, entirely from FuncSig.Calls + FuncSig.stem (no extraction needed).
func TestCallerStemIndexUnionsCallerStems(t *testing.T) {
	a := &FuncSig{Name: "canBypassRateLimit", Calls: []string{"checkLimit"}}
	b := &FuncSig{Name: "skipThrottleCheck", Calls: []string{"checkLimit"}}
	c := &FuncSig{Name: "unrelatedHelper", Calls: []string{"otherFunc"}}
	for _, f := range []*FuncSig{a, b, c} {
		f.Prepare()
	}

	idx := CallerStemIndex([]*FuncSig{a, b, c})

	entry, ok := idx["checkLimit"]
	if !ok {
		t.Fatal(`idx["checkLimit"] missing`)
	}
	for tok := range a.stem {
		if !entry.has(tok) {
			t.Errorf("checkLimit's caller-stem index missing %q from caller %q", tok, a.Name)
		}
	}
	for tok := range b.stem {
		if !entry.has(tok) {
			t.Errorf("checkLimit's caller-stem index missing %q from caller %q", tok, b.Name)
		}
	}

	otherEntry, ok := idx["otherFunc"]
	if !ok {
		t.Fatal(`idx["otherFunc"] missing`)
	}
	for tok := range a.stem {
		if otherEntry.has(tok) {
			t.Errorf("otherFunc's caller-stem index wrongly picked up %q from a non-caller (%q)", tok, a.Name)
		}
	}

	if _, ok := idx["neverCalled"]; ok {
		t.Error(`idx["neverCalled"] should be absent, not an empty entry`)
	}
}

// TestCallerStemIndexSkipsUnstemmableCallers pins the degrade-gracefully case
// the doc comment promises: a caller with no stem tokens (unprepared, or a
// name stemTokens reduces to nothing) contributes an empty set for its
// callees rather than panicking or polluting the index with "".
func TestCallerStemIndexSkipsUnstemmableCallers(t *testing.T) {
	unprepared := &FuncSig{Name: "whatever", Calls: []string{"target"}} // Prepare() never called
	idx := CallerStemIndex([]*FuncSig{unprepared})
	if _, ok := idx["target"]; ok {
		t.Error(`an unprepared caller (nil stem) should leave "target" absent from the index, not present with an empty/garbage entry`)
	}
}

// TestCallContextCandidatesRequiresBothSignals pins the core recall case:
// two functions with unrelated names, no shared body tokens, and (in this
// fixture) no type signature at all — invisible to every other channel —
// still pair because their respective callers share a name-stem AND tag the
// call result the same way. A third function with an unrelated caller and
// shape must NOT join the pair.
func TestCallContextCandidatesRequiresBothSignals(t *testing.T) {
	callerA := &FuncSig{File: "a.go", Qualname: "alpha", Name: "alpha", NLines: 6,
		Calls:            []string{"checkLimit"},
		CallResultShapes: map[string][]string{"checkLimit": {"ret-nil-checked"}},
	}
	callerB := &FuncSig{File: "b.go", Qualname: "alpha2", Name: "alpha", NLines: 6,
		Calls:            []string{"verifyQuota"},
		CallResultShapes: map[string][]string{"verifyQuota": {"ret-nil-checked"}},
	}
	callerC := &FuncSig{File: "c.go", Qualname: "beta", Name: "beta", NLines: 6,
		Calls:            []string{"otherThing"},
		CallResultShapes: map[string][]string{"otherThing": {"ret-compared"}},
	}
	calleeA := &FuncSig{File: "lib1.go", Qualname: "checkLimit", Name: "checkLimit", NLines: 6}
	calleeB := &FuncSig{File: "lib2.go", Qualname: "verifyQuota", Name: "verifyQuota", NLines: 6}
	calleeC := &FuncSig{File: "lib3.go", Qualname: "otherThing", Name: "otherThing", NLines: 6}

	sigs := []*FuncSig{callerA, callerB, callerC, calleeA, calleeB, calleeC}
	for _, f := range sigs {
		f.Prepare()
	}

	cands := CallContextCandidates(sigs, SizeGate{MinLines: 4}, 0.5, 0.5, 8)

	if len(cands) != 1 {
		t.Fatalf("expected 1 candidate, got %d: %+v", len(cands), cands)
	}
	c := cands[0]
	if c.Kind != "call-context" {
		t.Errorf("Kind = %q, want call-context", c.Kind)
	}
	gotNames := map[string]bool{c.A.Name: true, c.B.Name: true}
	if !gotNames["checkLimit"] || !gotNames["verifyQuota"] {
		t.Errorf("expected checkLimit ≟ verifyQuota, got %s ≟ %s", c.A.Name, c.B.Name)
	}
	if gotNames["otherThing"] {
		t.Error("otherThing has neither caller-stem nor shape overlap with the pair — must not appear")
	}
}

// TestCallContextCandidatesShapeOnlyInsufficient pins §5's AND-gate: caller-
// stem overlap alone, with disjoint call-result shapes, must NOT pair — the
// whole point of requiring both signals is that either alone is a known false-
// positive trap (shared caller vocabulary catches unrelated functions serving
// one popular caller).
func TestCallContextCandidatesShapeOnlyInsufficient(t *testing.T) {
	callerA := &FuncSig{File: "a.go", Qualname: "alpha", Name: "alpha", NLines: 6,
		Calls:            []string{"checkLimit"},
		CallResultShapes: map[string][]string{"checkLimit": {"ret-nil-checked"}},
	}
	callerB := &FuncSig{File: "b.go", Qualname: "alpha2", Name: "alpha", NLines: 6,
		Calls:            []string{"verifyQuota"},
		CallResultShapes: map[string][]string{"verifyQuota": {"ret-compared"}}, // disjoint shape
	}
	calleeA := &FuncSig{File: "lib1.go", Qualname: "checkLimit", Name: "checkLimit", NLines: 6}
	calleeB := &FuncSig{File: "lib2.go", Qualname: "verifyQuota", Name: "verifyQuota", NLines: 6}

	sigs := []*FuncSig{callerA, callerB, calleeA, calleeB}
	for _, f := range sigs {
		f.Prepare()
	}

	if got := CallContextCandidates(sigs, SizeGate{MinLines: 4}, 0.5, 0.5, 8); len(got) != 0 {
		t.Errorf("caller-stem overlap with disjoint shape tags must not pair, got %d: %+v", len(got), got)
	}
}

// TestCallContextCandidatesFanoutCap: a caller-stem token shared by more than
// maxFanout distinct candidate callees is plumbing (a popular caller calling
// many unrelated things), not a role — skipped, mirroring
// TestNameStemFanoutCap's convention for the name-stem pass.
func TestCallContextCandidatesFanoutCap(t *testing.T) {
	var sigs []*FuncSig
	for i := 0; i < 10; i++ {
		leaf := "target" + string(rune('A'+i))
		caller := &FuncSig{File: "c.go", Qualname: "shared" + string(rune('A'+i)), Name: "shared", NLines: 6,
			Calls:            []string{leaf},
			CallResultShapes: map[string][]string{leaf: {"ret-nil-checked"}},
		}
		callee := &FuncSig{File: "lib.go", Qualname: leaf, Name: leaf, NLines: 6}
		sigs = append(sigs, caller, callee)
	}
	for _, f := range sigs {
		f.Prepare()
	}

	if got := CallContextCandidates(sigs, SizeGate{MinLines: 4}, 0.5, 0.5, 8); len(got) != 0 {
		t.Errorf("caller-stem 'shared' shared by 10 callees exceeds fanout cap 8, expected 0, got %d: %+v", len(got), got)
	}
}
