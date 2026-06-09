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
