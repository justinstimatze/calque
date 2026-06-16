package code

import (
	"os"
	"path/filepath"
	"strings"
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

	// The confession is a `//` doc comment → the literal "line" register, and its
	// directed candidate is tagged [line] in the Sig for the Layer D matrix.
	if confessor.Register != "line" {
		t.Errorf("Register = %q, want \"line\" (it's a // doc comment)", confessor.Register)
	}

	// The comment names classifyThrough → a directed twin candidate.
	cands := ConfessionCandidates(confs, sigs, false)
	found := false
	for _, c := range cands {
		pair := map[string]bool{c.A.Qualname: true, c.B.Qualname: true}
		if pair["computeAudit"] && pair["classifyThrough"] {
			found = true
			if c.Kind != "confession" {
				t.Errorf("Kind = %q, want confession", c.Kind)
			}
			if !strings.Contains(c.Sig, "[line]") {
				t.Errorf("Sig = %q, want a [line] register tag", c.Sig)
			}
		}
	}
	if !found {
		t.Errorf("expected directed candidate computeAudit ≟ classifyThrough; got %+v", cands)
	}
}

// TestConfessionRegister pins the register discriminator: a dedicated single-line
// comment (// or #) is "line" (literal twin-flag), while a docstring body / block-comment
// continuation is "prose" (figurative). The prose register is gated from directed
// candidates by default — the precision win the Layer D matrix surfaced.
func TestConfessionRegister(t *testing.T) {
	cases := []struct {
		text string
		want string
	}{
		{"// mirrors classifyThrough", "line"},
		{"   /// rust doc: mirrors X", "line"},
		{"//! crate doc mirrors X", "line"},
		{"# Python: mirrors compute_audit", "line"},
		{`    Mirrors cli.cmd_seed_scenario: injects facts,`, "prose"},  // docstring body
		{`themselves to. The player->NPC mirror of get_x."""`, "prose"}, // docstring body w/ closing quote
		{" * mirrors X (JSDoc continuation)", "prose"},
		{"/* mirrors X */", "prose"},
	}
	for _, c := range cases {
		if got := confessionRegister(c.text); got != c.want {
			t.Errorf("confessionRegister(%q) = %q, want %q", c.text, got, c.want)
		}
	}

	// A prose-register confession must NOT become a directed candidate by default,
	// but DOES with includeProse=true. Use a Go block comment (also prose register) so
	// the test stays pure-Go — no external extractor runtime needed in CI.
	dir := t.TempDir()
	src := `package p

func deriveValue() int {
	/* The telemetry here mirrors helperTwin in spirit, not in code. */
	return 1
}

func helperTwin() int {
	return 2
}
`
	f := filepath.Join(dir, "m.go")
	if err := os.WriteFile(f, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	sigs, err := extractGoBatch([]string{f}, dir)
	if err != nil {
		t.Fatalf("extractGoBatch: %v", err)
	}
	for _, s := range sigs {
		s.Prepare()
	}
	confs := FindConfessions(sigs, dir)
	var proseSeen bool
	for _, cf := range confs {
		if cf.Register == "prose" {
			proseSeen = true
		}
	}
	if !proseSeen {
		t.Fatalf("expected a prose-register confession in the docstring; got %+v", confs)
	}
	if got := ConfessionCandidates(confs, sigs, false); len(got) != 0 {
		t.Errorf("prose confession should be gated by default; got %d candidate(s): %+v", len(got), got)
	}
	if got := ConfessionCandidates(confs, sigs, true); len(got) == 0 {
		t.Errorf("prose confession should surface with includeProse=true; got none")
	}
}
