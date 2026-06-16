package code

import (
	"os"
	"path/filepath"
	"testing"
)

// TestConfessions pins the drift-confessing-comment axis: a doc comment above a
// function that says it "mirrors" another, plus a "keep in sync" body comment, is
// found and (when it names a resolvable function) turned into a directed twin pair.
func TestConfessions(t *testing.T) {
	dir := t.TempDir()
	src := `package p

// computeAudit mirrors classifyThrough exactly — keep in sync.
func computeAudit() int {
	return 1
}

func classifyThrough() bool {
	return true
}

// honest helper, no twin here.
func unrelated() int {
	return 2
}
`
	f := filepath.Join(dir, "p.go")
	if err := os.WriteFile(f, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	sigs, err := extractGoBatch([]string{f}, dir)
	if err != nil {
		t.Fatalf("extractGoBatch: %v", err)
	}
	for _, s := range sigs {
		s.Prepare() // ConfessionCandidates resolves named twins via the name stem
	}

	confs := FindConfessions(sigs, dir)
	// computeAudit confesses (doc comment ABOVE the func, outside its line span);
	// unrelated does not.
	var confessor *Confession
	for i := range confs {
		if confs[i].Func.Qualname == "computeAudit" {
			confessor = &confs[i]
		}
		if confs[i].Func.Qualname == "unrelated" {
			t.Errorf("unrelated() should not confess; matched %q", confs[i].Text)
		}
	}
	if confessor == nil {
		t.Fatalf("computeAudit's doc-comment confession not found; got %d confessions", len(confs))
	}
	if confessor.Phrase != "mirrors" {
		t.Errorf("phrase = %q, want \"mirrors\"", confessor.Phrase)
	}

	// The comment names classifyThrough → a directed twin candidate.
	cands := ConfessionCandidates(confs, sigs)
	found := false
	for _, c := range cands {
		pair := map[string]bool{c.A.Qualname: true, c.B.Qualname: true}
		if pair["computeAudit"] && pair["classifyThrough"] {
			found = true
			if c.Kind != "confession" {
				t.Errorf("Kind = %q, want confession", c.Kind)
			}
		}
	}
	if !found {
		t.Errorf("expected directed candidate computeAudit ≟ classifyThrough; got %+v", cands)
	}
}
