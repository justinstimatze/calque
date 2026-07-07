package main

// calibrate — the adaptive-weights leg. calque's per-channel signal weights ship
// as a hand-tuned static prior (internal/code.weights). `calibrate` reweights them
// from ADJUDICATED registry labels (drift→useful, false-alarm→not-useful): it
// measures how well each channel separates real finds from noise, shrinks that
// observation toward the prior (so few adjudications can't overfit), and — only on
// --write — emits .calque/weights.json. The gate loads that file and scores with it.
//
// §13-clean by construction: the training rows are human verdicts on concrete
// pairs, never a self-scan vibe. The discrimination steal is from sauremilk/drift
// (DESIGN_NOTES §6.1) — drift reweights signal precision from adjudicated git
// outcomes; calque's registry already holds the labels and doctor already computes
// the join, this just feeds it back.

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/justinstimatze/calque/internal/code"
	"github.com/justinstimatze/calque/internal/registry"
)

// weightsFileName is the git-tracked calibrated weight vector the gate loads.
const weightsFileName = "weights.json"

func weightsPath(repo string) string { return filepath.Join(calqueDir(repo), weightsFileName) }

// loadWeights reads .calque/weights.json if present. The bool is false when the
// file is absent (the common case — calque's own repo has none, so it stays on
// the static prior). A malformed file is a hard error: silently falling back to
// the prior would mask a corrupt calibration the user thinks is active.
func loadWeights(repo string) (map[string]float64, bool, error) {
	b, err := os.ReadFile(weightsPath(repo))
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var w map[string]float64
	if err := json.Unmarshal(b, &w); err != nil {
		return nil, false, fmt.Errorf("parsing %s: %w", weightsPath(repo), err)
	}
	return w, true, nil
}

// writeWeights persists a calibrated vector (rounded for a stable diff).
func writeWeights(repo string, w map[string]float64) error {
	if err := os.MkdirAll(calqueDir(repo), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(roundWeightsJSON(w), "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(weightsPath(repo), b, 0o644)
}

// roundWeightsJSON rounds to 4 decimals so re-running calibrate on unchanged
// labels yields an unchanged file (no float-jitter diff).
func roundWeightsJSON(w map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(w))
	for k, v := range w {
		out[k] = float64(int(v*1e4+0.5)) / 1e4
	}
	return out
}

// applyCalibratedWeights loads .calque/weights.json and activates it for scoring,
// unless disabled. Returns true if a calibrated vector was applied. Called by the
// gating commands (scan/check/doctor) before they recall; calibrate itself does
// NOT call this (it must train on the static prior, not its own prior output).
func applyCalibratedWeights(repo string, disabled bool) bool {
	if disabled {
		return false
	}
	w, ok, err := loadWeights(repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque: %v (scoring on default weights)\n", err)
		return false
	}
	if !ok {
		return false
	}
	code.UseWeights(w)
	return true
}

// runCalibrate derives a weight vector from adjudicated labels and reports it;
// --write persists it to .calque/weights.json (never auto-written — calibration
// is a deliberate act, not a side effect of a scan).
func runCalibrate(args []string) {
	fs := flag.NewFlagSet("calibrate", flag.ContinueOnError)
	b := addBoundaryFlags(fs)
	repo, left, right, exclude := b.repo, b.left, b.right, b.exclude
	minScore := fs.Float64("min-score", 0.05, "low floor so adjudicated pairs aren't filtered out before joining labels")
	minLines := fs.Int("min-lines", 4, "ignore functions shorter than this many lines")
	minNodes := fs.Int("min-nodes", 0, "size gate on AST-node count of the function body; 0 disables (default, matches current behavior exactly)")
	distanceBoost := fs.Bool("distance-boost", false, "boost score for pairs sitting far apart before joining labels — off by default since calibration trains on per-channel Signals, not the boosted Score")
	regPath := fs.String("registry", ".calque/registry.md", "registry file (adjudicated verdicts)")
	priorStrength := fs.Float64("prior-strength", 8, "shrinkage: higher keeps weights nearer the static prior (lambda=n/(n+this))")
	write := fs.Bool("write", false, "persist the calibrated vector to .calque/weights.json")
	if err := fs.Parse(args); err != nil {
		return
	}

	// Train on the static prior, never on an already-calibrated vector — else
	// calibrate would chase its own tail. Reset defensively in case a prior call
	// in-process left activeWeights swapped.
	code.ResetWeights()

	gate := code.SizeGate{MinLines: *minLines, MinNodes: *minNodes}
	copts := clusterOptsFrom(*minLines, *minNodes, 3, 8, 1<<30)
	r, err := codeAxis(*repo, *left, *right, *exclude, *minScore, gate, 1<<30, copts, *distanceBoost, false, false) // calibrate the gate's real behavior: test↔test excluded
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque calibrate: %v\n", err)
		os.Exit(1)
	}
	reg, err := registry.Load(joinRepo(*repo, *regPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque calibrate: reading registry: %v\n", err)
		os.Exit(1)
	}
	tagOf := loadFireTags(filepath.Join(calqueDir(*repo), "fire-tags.jsonl"))

	// Build training rows: each suspect pair joined to its label; keep only the
	// decisive classes (useful / not-useful). "mixed" (contracted-twin-ok) and
	// "untagged" carry no separation signal, so they're excluded.
	var samples []code.WeightSample
	var nUseful, nNot, nSkip int
	for _, s := range r.Pairs {
		label := resolveLabel(verdictClassFor(reg, s), pairID(s), tagOf)
		switch label {
		case "useful":
			samples = append(samples, code.WeightSample{Signals: s.Signals, Useful: true})
			nUseful++
		case "not-useful":
			samples = append(samples, code.WeightSample{Signals: s.Signals, Useful: false})
			nNot++
		default:
			nSkip++
		}
	}

	fmt.Println("# calque — weight calibration")
	fmt.Println()
	fmt.Printf("adjudicated pairs: %d useful, %d not-useful (%d unlabeled/mixed skipped)\n", nUseful, nNot, nSkip)

	if nUseful == 0 || nNot == 0 {
		fmt.Println()
		fmt.Println("⚠ need BOTH a labeled-useful and a labeled-not-useful pair to measure")
		fmt.Println("  discrimination. Adjudicate more suspects in the registry, then re-run.")
		fmt.Println("  (No file written; the gate stays on the static prior.)")
		return
	}

	w, stats := code.CalibrateWeights(samples, code.DefaultWeights(), *priorStrength)

	// §13 fragility guard: with a thin minority class the observed distribution is
	// a few points, so the vector reflects those specific pairs more than a real
	// trend. Shrinkage softens it, but the user should know before trusting it.
	const minClassForTrust = 3
	if min(nUseful, nNot) < minClassForTrust {
		fmt.Printf("⚠ thin minority class (min(%d,%d) < %d) — this vector overfits a handful of\n", nUseful, nNot, minClassForTrust)
		fmt.Println("  pairs; treat it as provisional and adjudicate more before relying on it.")
	}

	fmt.Printf("shrinkage lambda: %.3f (n=%d, prior-strength=%.0f)\n", stats.Lambda, stats.N, *priorStrength)
	fmt.Println()
	fmt.Println("| channel | prior | mean-diff | calibrated |")
	fmt.Println("|---|---|---|---|")
	prior := code.DefaultWeights()
	for _, k := range code.SortedChannels() {
		fmt.Printf("| %s | %.3f | %+.3f | %.3f |\n", k, prior[k], stats.MeanDiff[k], w[k])
	}
	fmt.Println()

	if !*write {
		fmt.Printf("dry run — re-run with --write to persist to %s\n", weightsPath(*repo))
		return
	}
	if err := writeWeights(*repo, w); err != nil {
		fmt.Fprintf(os.Stderr, "calque calibrate: writing weights: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s — scans on this repo now load it (override with --no-calibrated-weights)\n", weightsPath(*repo))
}

// verdictClassFor returns the registry verdict class for a suspect pair, or "".
func verdictClassFor(reg *registry.Registry, s code.Suspicion) string {
	if e, ok := reg.Lookup(s.Left.Key(), s.Right.Key()); ok {
		return e.VerdictClass()
	}
	return ""
}
