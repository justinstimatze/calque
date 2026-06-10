package main

import (
	"strings"
	"testing"

	"github.com/justinstimatze/calque/internal/pairkey"
)

const pruneFixture = `# calque registry

## Some prose header — explains a thing
This narrative should survive a prune untouched.

- pair: a.go::Live | b.go::Live
- verdict: drift
- reviewed: 2026-06-01
- pair: a.go::Dead | b.go::Gone
- verdict: false-alarm
- reviewed: 2026-06-01
- note: a coincidental stem

## A cluster section
- cluster: x.go::A | x.go::B | x.go::C
- verdict: contracted-twin-ok
- reviewed: 2026-06-02

- cluster: y.go::Old | y.go::Older
- verdict: contracted-twin-ok
`

func TestPruneRegistryRemovesOnlyStale(t *testing.T) {
	stalePairs := map[string]bool{
		pairkey.Key("a.go::Dead", "b.go::Gone"): true,
	}
	staleClusters := map[string]bool{
		pairkey.SetKey([]string{"y.go::Old", "y.go::Older"}): true,
	}
	out, removed := pruneRegistry(pruneFixture, stalePairs, staleClusters)

	if len(removed) != 2 {
		t.Fatalf("expected 2 removed, got %d: %v", len(removed), removed)
	}
	// Stale pair + its attributes gone (incl. the trailing - note:).
	if strings.Contains(out, "Dead") || strings.Contains(out, "b.go::Gone") {
		t.Error("stale pair line survived")
	}
	if strings.Contains(out, "a coincidental stem") {
		t.Error("stale pair's - note: attribute survived")
	}
	// Stale cluster gone.
	if strings.Contains(out, "y.go::Old") {
		t.Error("stale cluster survived")
	}
	// Live entries preserved.
	if !strings.Contains(out, "- pair: a.go::Live | b.go::Live") {
		t.Error("live pair was removed")
	}
	if !strings.Contains(out, "- cluster: x.go::A | x.go::B | x.go::C") {
		t.Error("live cluster was removed")
	}
	// Live pair keeps its own attributes.
	if !strings.Contains(out, "- verdict: drift") || !strings.Contains(out, "- reviewed: 2026-06-01") {
		t.Error("live pair's attributes were collateral-removed")
	}
	// Prose untouched.
	if !strings.Contains(out, "This narrative should survive a prune untouched.") {
		t.Error("prose was removed")
	}
	if !strings.Contains(out, "## A cluster section") {
		t.Error("a header was removed")
	}
}

func TestPruneRegistryNoStaleIsNoop(t *testing.T) {
	out, removed := pruneRegistry(pruneFixture, map[string]bool{}, map[string]bool{})
	if len(removed) != 0 {
		t.Errorf("expected 0 removed, got %d", len(removed))
	}
	if out != pruneFixture {
		t.Error("no-stale prune should return the text byte-identical")
	}
}

// A live pair whose attributes are removed by a *neighbouring* stale entry must
// not happen — verify adjacency boundaries hold when stale and live entries touch.
func TestPruneRegistryAdjacentEntries(t *testing.T) {
	// Dead is immediately followed by Live with no blank line between blocks.
	fixture := `- pair: a::Dead | b::Dead
- verdict: drift
- pair: a::Live | b::Live
- verdict: drift
- reviewed: 2026-06-09`
	stale := map[string]bool{pairkey.Key("a::Dead", "b::Dead"): true}
	out, removed := pruneRegistry(fixture, stale, map[string]bool{})
	if len(removed) != 1 {
		t.Fatalf("expected 1 removed, got %d", len(removed))
	}
	if strings.Contains(out, "Dead") {
		t.Error("stale entry survived")
	}
	if !strings.Contains(out, "- pair: a::Live | b::Live") || !strings.Contains(out, "- reviewed: 2026-06-09") {
		t.Errorf("adjacent live entry was damaged:\n%s", out)
	}
}
