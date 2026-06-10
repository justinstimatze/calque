package code

import (
	"math"
	"testing"
)

func sumWeights(w map[string]float64) float64 {
	var s float64
	for _, k := range channelOrder {
		s += w[k]
	}
	return s
}

// A perfectly separable signal (writes high on useful, zero on noise) should
// dominate the calibrated vector once there are enough samples to overcome the
// prior shrinkage.
func TestCalibrateSeparableSignal(t *testing.T) {
	var samples []WeightSample
	for i := 0; i < 20; i++ {
		samples = append(samples, WeightSample{
			Signals: map[string]float64{"writes": 0.9, "strings": 0.1, "name": 0.1, "calls": 0.1, "ret": 0.1},
			Useful:  true,
		})
		samples = append(samples, WeightSample{
			Signals: map[string]float64{"writes": 0.0, "strings": 0.1, "name": 0.1, "calls": 0.1, "ret": 0.1},
			Useful:  false,
		})
	}
	w, stats := CalibrateWeights(samples, DefaultWeights(), 5)
	if math.Abs(sumWeights(w)-1.0) > 1e-9 {
		t.Fatalf("weights must sum to 1, got %v (%f)", w, sumWeights(w))
	}
	// writes is the only channel reading higher on useful → it must carry the most.
	for _, k := range []string{"strings", "name", "calls", "ret"} {
		if w["writes"] <= w[k] {
			t.Errorf("writes (%f) should outweigh %s (%f)", w["writes"], k, w[k])
		}
	}
	if stats.MeanDiff["writes"] <= 0 {
		t.Errorf("writes mean-diff should be positive, got %f", stats.MeanDiff["writes"])
	}
	if stats.NUseful != 20 || stats.NNotUsefl != 20 {
		t.Errorf("class counts wrong: %d useful, %d not", stats.NUseful, stats.NNotUsefl)
	}
}

// With only one label class present there is no discrimination signal, so the
// prior must be returned unchanged.
func TestCalibrateSingleClassReturnsPrior(t *testing.T) {
	samples := []WeightSample{
		{Signals: map[string]float64{"writes": 0.9}, Useful: true},
		{Signals: map[string]float64{"writes": 0.8}, Useful: true},
	}
	prior := DefaultWeights()
	w, stats := CalibrateWeights(samples, prior, 5)
	for _, k := range channelOrder {
		if math.Abs(w[k]-normalizeWeights(prior)[k]) > 1e-9 {
			t.Errorf("single-class should return prior; channel %s got %f want %f", k, w[k], normalizeWeights(prior)[k])
		}
	}
	if stats.NNotUsefl != 0 {
		t.Errorf("expected 0 not-useful, got %d", stats.NNotUsefl)
	}
}

// Shrinkage: with few samples lambda is small, so the result must sit close to
// the prior even when the observed signal is extreme.
func TestCalibrateShrinkageSmallN(t *testing.T) {
	samples := []WeightSample{
		{Signals: map[string]float64{"writes": 1.0, "strings": 0, "name": 0, "calls": 0, "ret": 0}, Useful: true},
		{Signals: map[string]float64{"writes": 0.0, "strings": 0, "name": 0, "calls": 0, "ret": 0}, Useful: false},
	}
	prior := DefaultWeights()
	priorStrength := 50.0 // large prior, tiny n ⇒ lambda ≈ 2/52
	w, stats := CalibrateWeights(samples, prior, priorStrength)
	wantLambda := 2.0 / (2.0 + priorStrength)
	if math.Abs(stats.Lambda-wantLambda) > 1e-9 {
		t.Errorf("lambda = %f, want %f", stats.Lambda, wantLambda)
	}
	// With heavy shrinkage, writes should barely move from its prior share.
	priorNorm := normalizeWeights(prior)
	if w["writes"] <= priorNorm["writes"] {
		t.Errorf("writes should rise a little above prior, got %f vs prior %f", w["writes"], priorNorm["writes"])
	}
	if w["writes"] > priorNorm["writes"]+0.1 {
		t.Errorf("heavy shrinkage should keep writes near prior, got %f vs prior %f", w["writes"], priorNorm["writes"])
	}
	if math.Abs(sumWeights(w)-1.0) > 1e-9 {
		t.Errorf("weights must sum to 1, got %f", sumWeights(w))
	}
}

// No channel discriminates (identical distributions) → fall back to prior.
func TestCalibrateNoDiscriminationFallsBackToPrior(t *testing.T) {
	samples := []WeightSample{
		{Signals: map[string]float64{"writes": 0.5, "strings": 0.5, "name": 0.5, "calls": 0.5, "ret": 0.5}, Useful: true},
		{Signals: map[string]float64{"writes": 0.5, "strings": 0.5, "name": 0.5, "calls": 0.5, "ret": 0.5}, Useful: false},
	}
	prior := DefaultWeights()
	w, _ := CalibrateWeights(samples, prior, 5)
	priorNorm := normalizeWeights(prior)
	for _, k := range channelOrder {
		if math.Abs(w[k]-priorNorm[k]) > 1e-9 {
			t.Errorf("no-discrimination should return prior; channel %s got %f want %f", k, w[k], priorNorm[k])
		}
	}
}

// UseWeights/ResetWeights round-trip: a calibrated vector changes scores but
// ResetWeights restores the prior exactly.
func TestUseAndResetWeights(t *testing.T) {
	defer ResetWeights()
	orig := cloneWeights(activeWeights)
	UseWeights(map[string]float64{"writes": 0.9, "strings": 0.025, "name": 0.025, "calls": 0.025, "ret": 0.025})
	if activeWeights["writes"] == orig["writes"] {
		t.Errorf("UseWeights did not change activeWeights")
	}
	ResetWeights()
	for _, k := range channelOrder {
		if activeWeights[k] != orig[k] {
			t.Errorf("ResetWeights did not restore prior; channel %s got %f want %f", k, activeWeights[k], orig[k])
		}
	}
}

// UseWeights with a partial vector backfills missing channels from the prior, so
// an unmentioned channel never zeroes out.
func TestUseWeightsPartialBackfill(t *testing.T) {
	defer ResetWeights()
	UseWeights(map[string]float64{"writes": 0.5})
	if activeWeights["strings"] != weights["strings"] {
		t.Errorf("missing channel should fall back to prior; strings = %f want %f", activeWeights["strings"], weights["strings"])
	}
	if activeWeights["writes"] != 0.5 {
		t.Errorf("provided channel should be honored; writes = %f want 0.5", activeWeights["writes"])
	}
}
