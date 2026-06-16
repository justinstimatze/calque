package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLangOf(t *testing.T) {
	cases := map[string]string{
		"a/b.go": "go", "x.py": "python", "c.tsx": "typescript",
		"d.mjs": "javascript", "e.rs": "rust", "f.md": "other", "noext": "other",
	}
	for path, want := range cases {
		if got := langOf(path); got != want {
			t.Errorf("langOf(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestVarietyOf(t *testing.T) {
	cases := map[string]string{
		"reads≈0.83 [mutate/forward-map] {a,b}": "mutate/forward-map",
		"reads≈1.00 [mutate/mutate] {x}":        "mutate/mutate",
		"name≈foo":                              "",
		"unterminated [open":                    "",
	}
	for sig, want := range cases {
		if got := varietyOf(sig); got != want {
			t.Errorf("varietyOf(%q) = %q, want %q", sig, got, want)
		}
	}
}

func TestAblateCellVerdict(t *testing.T) {
	// support gate: below minSupport, never rules.
	if v := ablateVerdict(ablateCell{drift: 4, falseAlarm: 0}); v != "insufficient" {
		t.Errorf("n=4 perfect precision should be insufficient, got %q", v)
	}
	// pulls weight: precision ≥ 0.50 with support.
	if v := ablateVerdict(ablateCell{drift: 3, twinOK: 2, falseAlarm: 2}); v != "pulls-weight" {
		t.Errorf("5/7 precision with support should pull weight, got %q", v)
	}
	// prune: below threshold with support.
	if v := ablateVerdict(ablateCell{drift: 2, falseAlarm: 10}); v != "prune?" {
		t.Errorf("2/12 precision with support should prune, got %q", v)
	}
	// contracted-twin-ok counts as a real twin, not a false alarm.
	c := ablateCell{}
	c.add("drift")
	c.add("contracted-twin-ok")
	c.add("false-alarm")
	if c.real() != 2 || c.total() != 3 {
		t.Errorf("real=%d total=%d, want real=2 total=3", c.real(), c.total())
	}
}

func TestLoadLabelsDedupLatestWins(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "labels.jsonl")
	// Same detector+a+b written twice; the later verdict must win.
	rows := []Label{
		{Detector: "read-set", AKey: "f.go::A", BKey: "f.go::B", Verdict: "false-alarm"},
		{Detector: "read-set", AKey: "f.go::A", BKey: "f.go::B", Verdict: "drift"},
		{Detector: "read-set", AKey: "f.go::C", BKey: "f.go::D", Verdict: "drift"},
	}
	for _, r := range rows {
		if err := appendJSONL(path, r); err != nil {
			t.Fatal(err)
		}
	}
	got := loadLabels(path)
	if len(got) != 2 {
		t.Fatalf("expected 2 deduped labels, got %d", len(got))
	}
	for _, l := range got {
		if l.AKey == "f.go::A" && l.Verdict != "drift" {
			t.Errorf("latest verdict should win: got %q, want drift", l.Verdict)
		}
	}
}

func TestLoadLabelsMissingFile(t *testing.T) {
	if got := loadLabels(filepath.Join(os.TempDir(), "calque-no-such-labels.jsonl")); len(got) != 0 {
		t.Errorf("missing store should yield no labels, got %v", got)
	}
}
