package code

import (
	"os"
	"path/filepath"
	"testing"
)

// Go module-level table extraction (enables the axis on a Go codebase): exported maps,
// unexported maps with enough keys, and string slices all become "table" entities;
// thin/unexported and non-literal vars are filtered.
func TestExtractGoSymbols(t *testing.T) {
	repo := t.TempDir()
	src := `package x

var HANDLERS = map[string]func(){
	"look": nil,
	"go":   nil,
	"take": nil,
}

var verbTemplates = map[string]string{
	"look": "look at {x}",
	"go":   "go {dir}",
	"take": "take {x}",
}

var Colors = []string{"red", "green", "blue"}

var small = map[string]int{"a": 1} // unexported, <3 keys -> skipped

var notATable = compute() // not a literal -> skipped
`
	if err := os.WriteFile(filepath.Join(repo, "reg.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	ents, _, err := ExtractSymbols(repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]*FuncSig{}
	for _, e := range ents {
		if e.Kind == "table" {
			by[e.Name] = e
		}
	}
	h, ok := by["HANDLERS"]
	if !ok {
		t.Fatalf("HANDLERS not extracted; got %v", keysOf(by))
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
	if _, ok := by["verbTemplates"]; !ok {
		t.Error("verbTemplates (unexported, 3 keys) should be extracted")
	}
	if _, ok := by["Colors"]; !ok {
		t.Error("Colors (exported string slice) should be extracted")
	}
	if _, ok := by["small"]; ok {
		t.Error("small (unexported, 1 key) should be filtered as noise")
	}
}
