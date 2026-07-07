package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justinstimatze/calque/internal/registry"
)

func TestRegistryParseWarning(t *testing.T) {
	dir := t.TempDir()

	// A registry that parsed entries → no warning (the happy path).
	if w := registryParseWarning(filepath.Join(dir, "anything.md"), 3, 0); w != "" {
		t.Errorf("parsed entries should not warn, got %q", w)
	}

	// A missing file is legitimately empty → no warning.
	if w := registryParseWarning(filepath.Join(dir, "missing.md"), 0, 0); w != "" {
		t.Errorf("missing registry should not warn, got %q", w)
	}

	// A registry with only comments/blanks → no warning (nothing to parse yet).
	empty := filepath.Join(dir, "empty.md")
	os.WriteFile(empty, []byte("# header\n\n> note\n"), 0o644)
	if w := registryParseWarning(empty, 0, 0); w != "" {
		t.Errorf("comment-only registry should not warn, got %q", w)
	}

	// Old Python-era format with content but 0 parsed → warn + name migrate-registry.
	old := filepath.Join(dir, "old.md")
	os.WriteFile(old, []byte("## p — drift\n- left:  a.py::A\n- right: b.py::B\n"), 0o644)
	w := registryParseWarning(old, 0, 0)
	if w == "" || !strings.Contains(w, "migrate-registry") {
		t.Errorf("old-format registry should warn and suggest migrate-registry, got %q", w)
	}

	// Unrecognized non-empty content but 0 parsed → generic format warning.
	junk := filepath.Join(dir, "junk.md")
	os.WriteFile(junk, []byte("some prose with no entries at all\n"), 0o644)
	w = registryParseWarning(junk, 0, 0)
	if w == "" || strings.Contains(w, "migrate-registry") {
		t.Errorf("non-old junk should warn generically (not migrate), got %q", w)
	}
}

// TestUnresolvedDrift: a drift pair with both paths live is unresolved; a drift with a
// dead side has collapsed (→ Stale, not here); a non-drift verdict never qualifies.
func TestUnresolvedDrift(t *testing.T) {
	entries := []registry.Entry{
		{Key1: "a.go::f", Key2: "b.go::g", Verdict: "drift"},                  // both live → unresolved
		{Key1: "a.go::f", Key2: "gone.go::x", Verdict: "drift"},               // one side gone → collapsed (stale)
		{Key1: "a.go::f", Key2: "b.go::g", Verdict: "false-alarm"},            // not drift
		{Key1: "c.go::h", Key2: "d.go::i", Verdict: "contracted-twin-ok (x)"}, // not drift
	}
	live := map[string]bool{"a.go::f": true, "b.go::g": true, "c.go::h": true, "d.go::i": true}

	got := unresolvedDrift(entries, live)
	if len(got) != 1 {
		t.Fatalf("want exactly 1 unresolved drift, got %d: %+v", len(got), got)
	}
	if got[0].Key1 != "a.go::f" || got[0].Key2 != "b.go::g" {
		t.Fatalf("wrong entry surfaced: %+v", got[0])
	}
}

// TestComputeCheckStaleConfidentlyDead: STALE only when a referenced file is gone;
// a referenced symbol absent from the corpus but whose file still exists (an
// --exclude'd/non-function key) is counted as StaleAmbig, not flagged STALE.
func TestComputeCheckStaleConfidentlyDead(t *testing.T) {
	repo := t.TempDir()
	// A real Go file so the corpus is non-empty (prune-style empty-corpus refusal n/a).
	goSrc := "package x\n\n" +
		"func Alpha() int {\n\tn := doThing()\n\tn += doThing()\n\treturn n\n}\n\n" +
		"func doThing() int { return 1 }\n"
	if err := os.WriteFile(filepath.Join(repo, "live.go"), []byte(goSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	regDir := filepath.Join(repo, ".calque")
	if err := os.MkdirAll(regDir, 0o755); err != nil {
		t.Fatal(err)
	}
	reg := "# r\n" +
		"- pair: gone.go::Removed | live.go::Alpha\n- verdict: false-alarm\n" +
		"- pair: live.go::NotExtracted | live.go::Alpha\n- verdict: false-alarm\n"
	if err := os.WriteFile(filepath.Join(regDir, "registry.md"), []byte(reg), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := computeCheck(repo, "", "", "", 0.18, 4, 0, 3, 8, ".calque/registry.md", false, false)
	if err != nil {
		t.Fatal(err)
	}
	// gone.go is a supported source file that doesn't exist → provably dead → STALE.
	if len(f.Stale) != 1 || f.Stale[0].Key1 != "gone.go::Removed" {
		t.Errorf("expected exactly 1 STALE (gone.go::Removed), got %+v", f.Stale)
	}
	// live.go exists but ::NotExtracted isn't a function in the corpus → ambiguous,
	// NOT stale.
	if f.StaleAmbig != 1 {
		t.Errorf("expected StaleAmbig=1 (live file, absent symbol), got %d", f.StaleAmbig)
	}
}
