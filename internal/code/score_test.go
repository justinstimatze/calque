package code

import (
	"strings"
	"testing"
)

// TestScorePairDistanceBoostDefaultOff pins the regression-free default: with
// the toggle untouched, scorePair must behave exactly as it did before
// distanceBoost existed — DistBoost is the identity 1.0 and Reason() never
// mentions it.
func TestScorePairDistanceBoostDefaultOff(t *testing.T) {
	a := fsig("a.go", "alpha", []string{"shared_marker_xyz"}, nil, nil, nil)
	b := fsig("z/y/x/w/b.go", "beta", []string{"shared_marker_xyz"}, nil, nil, nil)
	a.Line, b.Line = 1, 1

	s, ok := scorePair(a, b)
	if !ok {
		t.Fatal("expected the shared-string pair to clear the anchor gate")
	}
	if s.DistBoost != 1.0 {
		t.Errorf("DistBoost must default to 1.0 (toggle off), got %.4f", s.DistBoost)
	}
	if strings.Contains(s.Reason(), "dist-boost=") {
		t.Errorf("Reason() must not render dist-boost when the toggle is off, got %q", s.Reason())
	}
}

// TestScorePairDistanceBoostAppliesAndRenders: enabling the toggle boosts a far
// cross-directory pair's score above its unboosted baseline and Reason()
// renders the boost; disabled, the same pair is unaffected.
func TestScorePairDistanceBoostAppliesAndRenders(t *testing.T) {
	a := fsig("a.go", "alpha", []string{"shared_marker_xyz"}, nil, nil, nil)
	b := fsig("z/y/x/w/b.go", "beta", []string{"shared_marker_xyz"}, nil, nil, nil)
	a.Line, b.Line = 1, 1

	base, ok := scorePair(a, b)
	if !ok {
		t.Fatal("expected the shared-string pair to clear the anchor gate")
	}

	SetDistanceBoost(true)
	defer SetDistanceBoost(false)
	boosted, ok := scorePair(a, b)
	if !ok {
		t.Fatal("expected the shared-string pair to clear the anchor gate with boost enabled")
	}

	if boosted.DistBoost <= 1.0 {
		t.Errorf("expected DistBoost > 1.0 for a far cross-directory pair, got %.4f", boosted.DistBoost)
	}
	if boosted.Score <= base.Score {
		t.Errorf("boosted score should exceed the unboosted baseline: boosted=%.4f base=%.4f", boosted.Score, base.Score)
	}
	if !strings.Contains(boosted.Reason(), "dist-boost=") {
		t.Errorf("Reason() should render dist-boost once fired, got %q", boosted.Reason())
	}
	if strings.Contains(base.Reason(), "dist-boost=") {
		t.Errorf("the unboosted baseline's Reason() must not mention dist-boost, got %q", base.Reason())
	}
}

// TestScorePairDistanceBoostClampsAtOne: a pair that is already a near-perfect
// match on every channel must not exceed Score=1.0 once boosted — the
// min(1.0, score*boost) clamp in scorePair.
func TestScorePairDistanceBoostClampsAtOne(t *testing.T) {
	SetDistanceBoost(true)
	defer SetDistanceBoost(false)

	shared := []string{"marker_one", "marker_two"}
	a := fsig("a.go", "computeTotal", shared, shared, shared, shared)
	b := fsig("z/y/x/w/b.go", "computeTotal", shared, shared, shared, shared)
	a.Line, b.Line = 1, 1

	s, ok := scorePair(a, b)
	if !ok {
		t.Fatal("expected an identical pair to clear the anchor gate")
	}
	if s.DistBoost <= 1.0 {
		t.Fatalf("expected a fired boost for this far cross-directory pair, got DistBoost=%.4f", s.DistBoost)
	}
	if s.Score > 1.0 {
		t.Fatalf("Score must be clamped to <= 1.0, got %.6f (DistBoost=%.4f)", s.Score, s.DistBoost)
	}
}
