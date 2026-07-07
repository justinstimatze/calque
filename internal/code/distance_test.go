package code

import (
	"strings"
	"testing"
)

func TestDistanceBoostSameFileSaturatesAndCaps(t *testing.T) {
	near := &FuncSig{File: "a.go", Line: 10}
	nearB := &FuncSig{File: "a.go", Line: 15} // 5 lines apart — well under the 300-line saturation point
	mid := &FuncSig{File: "a.go", Line: 160}  // 150 lines apart — half of saturation
	far := &FuncSig{File: "a.go", Line: 400}  // 390 lines apart — past saturation

	gotNear := distanceBoost(near, nearB)
	gotMid := distanceBoost(near, mid)
	gotFar := distanceBoost(near, far)

	if gotNear <= 1.0 || gotNear >= gotMid || gotMid >= gotFar {
		t.Fatalf("expected a monotonic ramp 1.0 < near < mid < far, got near=%.4f mid=%.4f far=%.4f", gotNear, gotMid, gotFar)
	}
	if gotFar != 1.0+sameFileBoostCap {
		t.Errorf("a pair past the saturation point should cap at %.2f, got %.4f", 1.0+sameFileBoostCap, gotFar)
	}
	wantMid := 1.0 + 0.5*sameFileBoostCap
	if gotMid != wantMid {
		t.Errorf("a pair at half the saturation distance should be half the cap (%.4f), got %.4f", wantMid, gotMid)
	}
}

func TestDistanceBoostCrossFileSaturatesAndCaps(t *testing.T) {
	a := &FuncSig{File: "pkg/a/one.go", Line: 1}
	sibling := &FuncSig{File: "pkg/a/two.go", Line: 1}    // same directory — 0 hops
	near := &FuncSig{File: "pkg/b/two.go", Line: 1}       // sibling directory — 2 hops (half of the 4-hop saturation)
	distant := &FuncSig{File: "x/y/z/w/five.go", Line: 1} // unrelated, deep tree — past saturation

	gotSibling := distanceBoost(a, sibling)
	gotNear := distanceBoost(a, near)
	gotDistant := distanceBoost(a, distant)

	if gotSibling != 1.0 {
		t.Errorf("same-directory cross-file pair has 0 hops and must not boost, got %.4f", gotSibling)
	}
	wantNear := 1.0 + 0.5*crossFileBoostCap
	if gotNear != wantNear {
		t.Errorf("a 2-hop pair is half the 4-hop saturation, expected %.4f, got %.4f", wantNear, gotNear)
	}
	if gotDistant != 1.0+crossFileBoostCap {
		t.Errorf("a pair past hop-saturation should cap at %.2f, got %.4f", 1.0+crossFileBoostCap, gotDistant)
	}
}

func TestDirHops(t *testing.T) {
	cases := []struct {
		name     string
		a, b     string
		wantHops int
	}{
		{"same directory", "pkg/a/one.go", "pkg/a/two.go", 0},
		{"sibling directories", "pkg/a/one.go", "pkg/b/two.go", 2},
		{"unrelated trees", "pkg/a/b/one.go", "x/y/z/w/two.go", 7},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := dirHops(c.a, c.b); got != c.wantHops {
				t.Errorf("dirHops(%q, %q) = %d, want %d", c.a, c.b, got, c.wantHops)
			}
		})
	}
}

func TestDistBoostReasonEmptyWhenNoop(t *testing.T) {
	a := &FuncSig{File: "a.go", Line: 10}
	b := &FuncSig{File: "a.go", Line: 12}
	if got := distBoostReason(a, b, 1.0); got != "" {
		t.Errorf("distBoostReason(boost=1.0) should be empty (no-op), got %q", got)
	}
}

func TestDistBoostReasonRendersSameFileAndCrossDir(t *testing.T) {
	sameFileA := &FuncSig{File: "a.go", Line: 10}
	sameFileB := &FuncSig{File: "a.go", Line: 50}
	if got := distBoostReason(sameFileA, sameFileB, 1.05); !strings.Contains(got, "same-file") || !strings.Contains(got, "40 lines") {
		t.Errorf("expected a same-file rendering with the line distance, got %q", got)
	}

	crossA := &FuncSig{File: "pkg/a/one.go", Line: 1}
	crossB := &FuncSig{File: "pkg/b/two.go", Line: 1}
	if got := distBoostReason(crossA, crossB, 1.10); !strings.Contains(got, "cross-dir") || !strings.Contains(got, "2 hops") {
		t.Errorf("expected a cross-dir rendering with the hop count, got %q", got)
	}
}
