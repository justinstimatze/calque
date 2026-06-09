package registry

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadRoles covers the role-cardinality forward declarations (DESIGN_NOTES §18):
// full block, explicit expected:0 (a banned role), default expected, and that role
// parsing coexists with the existing pair parsing.
func TestLoadRoles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.md")
	content := `# calque registry

## role: input-constructor
- role: input-constructor
- predicate: name:/[Cc]onstruct/ calls:_dispatch
- expected: 1
- baseline: a.go::A.Construct, b.go::build
- reviewed: 2026-06-09

## role: banned-shell
- role: banned-shell
- predicate: name:/Shell/
- expected: 0

## role: defaulted
- role: defaulted
- predicate: name:/X/

## p1 — drift
- pair: x.go::f | y.go::g
- verdict: drift
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(r.Roles) != 3 {
		t.Fatalf("want 3 roles, got %d: %+v", len(r.Roles), r.Roles)
	}
	full := r.Roles[0]
	if full.Name != "input-constructor" {
		t.Errorf("name = %q", full.Name)
	}
	if full.Predicate != "name:/[Cc]onstruct/ calls:_dispatch" {
		t.Errorf("predicate = %q", full.Predicate)
	}
	if full.Expected != 1 {
		t.Errorf("expected = %d, want 1", full.Expected)
	}
	if len(full.Baseline) != 2 || full.Baseline[0] != "a.go::A.Construct" || full.Baseline[1] != "b.go::build" {
		t.Errorf("baseline = %v", full.Baseline)
	}
	if full.Reviewed != "2026-06-09" {
		t.Errorf("reviewed = %q", full.Reviewed)
	}
	if r.Roles[1].Expected != 0 {
		t.Errorf("explicit expected:0 must be preserved, got %d", r.Roles[1].Expected)
	}
	if r.Roles[2].Expected != 1 {
		t.Errorf("absent expected must default to 1, got %d", r.Roles[2].Expected)
	}
	if len(r.Entries) != 1 {
		t.Errorf("pair parsing must still work alongside roles: want 1 entry, got %d", len(r.Entries))
	}
}

// TestLoadEntryCollapseFields covers the drift collapse-direction fields (§18.7):
// `- canonical:` / `- do-not-resync:` attach to the open pair and round-trip.
func TestLoadEntryCollapseFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.md")
	content := `# calque registry

## p1 — drift (dual replay backend)
- pair: engine.go::replayVCR | engine.go::replayCassette
- verdict: drift
- canonical: engine.go::replayVCR
- do-not-resync: engine.go::replayCassette
- reviewed: 2026-06-09

## p2 — false-alarm (no collapse fields)
- pair: a.go::f | b.go::g
- verdict: false-alarm
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(r.Entries))
	}
	e := r.Entries[0]
	if e.Canonical != "engine.go::replayVCR" {
		t.Errorf("canonical = %q", e.Canonical)
	}
	if e.DoNotResync != "engine.go::replayCassette" {
		t.Errorf("do-not-resync = %q", e.DoNotResync)
	}
	if r.Entries[1].Canonical != "" || r.Entries[1].DoNotResync != "" {
		t.Errorf("collapse fields must default empty, got %q / %q", r.Entries[1].Canonical, r.Entries[1].DoNotResync)
	}
}
