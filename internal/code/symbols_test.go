package code

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Python module-level table extraction (the cross-substrate axis's non-function
// entities). Skips when python3 is absent so pure-Go CI stays green.
func TestExtractSymbolsPyTables(t *testing.T) {
	if _, err := exec.LookPath(pythonBin()); err != nil {
		t.Skip("python3 not available")
	}
	repo := t.TempDir()
	src := `
HANDLERS = {"look": h_look, "go": h_go, "take": h_take}
_VERB_TEMPLATES = {"look": "look at {x}", "go": "go {dir}", "take": "take {x}"}
small = {"a": 1}                  # lowercase, 1 key < minKeys -> skipped
x = compute()                     # not a literal -> skipped
COLORS = ["red", "green", "blue"] # uppercase string list -> set table
`
	if err := os.WriteFile(filepath.Join(repo, "engine.py"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	ents, _, err := ExtractSymbols(repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]*FuncSig{}
	for _, e := range ents {
		by[e.Name] = e
	}
	h, ok := by["HANDLERS"]
	if !ok {
		t.Fatalf("HANDLERS table not extracted; got %v", keysOf(by))
	}
	if h.Kind != "table" {
		t.Errorf("HANDLERS Kind = %q, want table", h.Kind)
	}
	want := map[string]bool{"look": true, "go": true, "take": true}
	for _, k := range h.RetKeys {
		if !want[k] {
			t.Errorf("unexpected key %q in HANDLERS", k)
		}
		delete(want, k)
	}
	if len(want) != 0 {
		t.Errorf("HANDLERS missing keys: %v", want)
	}
	if _, ok := by["_VERB_TEMPLATES"]; !ok {
		t.Error("_VERB_TEMPLATES not extracted")
	}
	if _, ok := by["COLORS"]; !ok {
		t.Error("COLORS (uppercase string list) not extracted")
	}
	if _, ok := by["small"]; ok {
		t.Error("lowercase 1-key dict 'small' should be filtered as noise")
	}
}

func keysOf(m map[string]*FuncSig) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
