package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/justinstimatze/calque/internal/code"
)

// writeWeights → loadWeights round-trips, and applyCalibratedWeights activates
// the loaded vector for scoring.
func TestWeightsRoundTrip(t *testing.T) {
	defer code.ResetWeights()
	repo := t.TempDir()
	want := map[string]float64{"strings": 0.5, "writes": 0.2, "name": 0.15, "calls": 0.1, "ret": 0.05}

	if err := writeWeights(repo, want); err != nil {
		t.Fatalf("writeWeights: %v", err)
	}
	got, ok, err := loadWeights(repo)
	if err != nil || !ok {
		t.Fatalf("loadWeights: ok=%v err=%v", ok, err)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("channel %s round-tripped to %f, want %f", k, got[k], v)
		}
	}

	code.ResetWeights()
	if !applyCalibratedWeights(repo, false) {
		t.Fatal("applyCalibratedWeights should report a vector applied")
	}
	// --no-calibrated-weights must refuse to load even when the file exists.
	code.ResetWeights()
	if applyCalibratedWeights(repo, true) {
		t.Error("applyCalibratedWeights(disabled=true) should not load")
	}
}

// A repo with no weights.json loads nothing (the common case — the gate stays on
// the static prior) and is not an error.
func TestLoadWeightsAbsent(t *testing.T) {
	_, ok, err := loadWeights(t.TempDir())
	if ok || err != nil {
		t.Errorf("absent weights.json: ok=%v err=%v, want false,nil", ok, err)
	}
}

// A malformed weights.json is a hard error, never a silent fallback that masks a
// corrupt calibration the user believes is active.
func TestLoadWeightsMalformed(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".calque"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(weightsPath(repo), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadWeights(repo); err == nil {
		t.Error("malformed weights.json should error")
	}
}
