package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompoundViolations(t *testing.T) {
	hits := []*vocabHit{
		{Term: "longing-to-be-chosen", Count: 9},
		{Term: "dual-path", Count: 7},     // allow-listed
		{Term: "rare-compound", Count: 2}, // below threshold
		{Term: "performance-of-virtue", Count: 5},
	}
	allow := map[string]bool{"dual-path": true}
	got := compoundViolations(hits, allow, 5)
	if len(got) != 2 {
		t.Fatalf("expected 2 violations, got %d: %+v", len(got), got)
	}
	want := map[string]bool{"longing-to-be-chosen": true, "performance-of-virtue": true}
	for _, h := range got {
		if !want[h.Term] {
			t.Errorf("unexpected violation %q (allow-listed or below threshold leaked)", h.Term)
		}
	}
}

func TestLoadAllowlist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vocab-allowlist.txt")
	content := "# a comment\ndual-path\n\n  meta-bug  \n# another\nname-stem\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	allow := loadAllowlist(path)
	for _, want := range []string{"dual-path", "meta-bug", "name-stem"} {
		if !allow[want] {
			t.Errorf("allow-list missing %q", want)
		}
	}
	if allow["# a comment"] || allow[""] {
		t.Error("comments/blanks must not be admitted")
	}
	if len(allow) != 3 {
		t.Errorf("expected 3 entries, got %d", len(allow))
	}
}

// A missing allow-list is empty, not an error (no compound known yet).
func TestLoadAllowlistMissing(t *testing.T) {
	allow := loadAllowlist(filepath.Join(t.TempDir(), "does-not-exist.txt"))
	if len(allow) != 0 {
		t.Errorf("missing allow-list should be empty, got %d", len(allow))
	}
}
