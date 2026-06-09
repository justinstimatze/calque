package main

import (
	"testing"

	"github.com/justinstimatze/calque/internal/code"
	"github.com/justinstimatze/calque/internal/registry"
)

func cardSig(file, qual, name string) *code.FuncSig {
	f := &code.FuncSig{File: file, Qualname: qual, Name: name}
	f.Prepare()
	return f
}

func TestComputeCardinality(t *testing.T) {
	sigs := []*code.FuncSig{
		cardSig("a.go", "A.Parse", "Parse"),
		cardSig("b.go", "B.Parse", "Parse"),
	}
	role := func(expected int, baseline ...string) registry.RoleEntry {
		return registry.RoleEntry{Name: "parser", Predicate: "name:/Parse/", Expected: expected, Baseline: baseline}
	}

	// Two implementers vs expected 1 → violation listing both.
	res := computeCardinality(sigs, []registry.RoleEntry{role(1)})
	if !res[0].Violation() {
		t.Fatalf("two implementers vs expected 1 must violate")
	}
	if len(res[0].Impls) != 2 {
		t.Fatalf("want 2 implementers, got %d", len(res[0].Impls))
	}

	// Two implementers vs expected 2 → clean (a declared legitimate multi-path).
	res = computeCardinality(sigs, []registry.RoleEntry{role(2)})
	if res[0].Violation() {
		t.Fatalf("two implementers vs expected 2 must be clean")
	}

	// Frozen-baseline ratchet: expected 2 but baseline only allows A.Parse →
	// B.Parse is novel → violation even though the count is within Expected.
	res = computeCardinality(sigs, []registry.RoleEntry{role(2, "a.go::A.Parse")})
	if !res[0].Violation() {
		t.Fatalf("baseline ratchet must violate on a new member")
	}
	if len(res[0].Novel) != 1 || res[0].Novel[0] != "b.go::B.Parse" {
		t.Fatalf("ratchet must flag b.go::B.Parse as novel, got %v", res[0].Novel)
	}

	// A malformed predicate is a violation, not a silent pass.
	res = computeCardinality(sigs, []registry.RoleEntry{{Name: "x", Predicate: "huh:nope", Expected: 1}})
	if !res[0].Violation() || res[0].BadPredicate == nil {
		t.Fatalf("a bad predicate must be reported as a violation")
	}
}
