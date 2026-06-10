package code

import (
	"os"
	"path/filepath"
	"testing"
)

// The TS symbol extractor is the cross-substrate axis's TS leg (mirror of the Go and
// Python table extractors): module-level object/array literal constants become `table`
// entities whose key set is their footprint, so the key-set + judge machinery can pair
// a TS table with a Python or Go one. Skips when no node/typescript toolchain (CI is
// pure-Go).
func TestExtractTSSymbols(t *testing.T) {
	repo := t.TempDir()
	if !tsToolchainAvailable(t, repo) {
		t.Skip("node + typescript not available")
	}
	src := "export const HANDLERS = { go: 'move', take: 'grab', drop: 'release' };\n" +
		"const _VERB_TEMPLATES = { go: {}, take: {}, look: {} };\n" +
		"export const VERBS = ['north', 'south', 'east', 'west'];\n" +
		"const small = { a: 1 };\n" + // 1 key, non-upper → noise, must be skipped
		"function notATable() { const local = { x: 1, y: 2, z: 3 }; return local; }\n"
	if err := os.WriteFile(filepath.Join(repo, "verbs.ts"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	ents, _, err := ExtractSymbols(repo, nil)
	if err != nil {
		t.Fatal(err)
	}

	byName := map[string]*FuncSig{}
	for _, e := range ents {
		byName[e.Name] = e
	}

	// HANDLERS: object literal → property names are the footprint, string values land
	// in the strings channel.
	h := byName["HANDLERS"]
	if h == nil {
		t.Fatalf("HANDLERS table not extracted; got %v", names(ents))
	}
	if h.Kind != "table" {
		t.Errorf("HANDLERS kind = %q, want table", h.Kind)
	}
	wantKeys := map[string]bool{"go": true, "take": true, "drop": true}
	for _, k := range h.RetKeys {
		if !wantKeys[k] {
			t.Errorf("HANDLERS unexpected key %q", k)
		}
		delete(wantKeys, k)
	}
	if len(wantKeys) != 0 {
		t.Errorf("HANDLERS missing keys %v (got %v)", wantKeys, h.RetKeys)
	}

	// VERBS: array literal → string elements are the footprint.
	v := byName["VERBS"]
	if v == nil {
		t.Fatalf("VERBS table not extracted; got %v", names(ents))
	}
	if len(v.RetKeys) != 4 {
		t.Errorf("VERBS keys = %v, want 4 directions", v.RetKeys)
	}

	// _VERB_TEMPLATES: UPPER-cased (underscore prefix) name qualifies even at 3 keys.
	if byName["_VERB_TEMPLATES"] == nil {
		t.Errorf("_VERB_TEMPLATES not extracted (isUpperName should accept _-prefixed)")
	}

	// Noise control: a 1-key non-upper literal and a function-local literal must NOT
	// surface as module-level tables.
	if byName["small"] != nil {
		t.Errorf("small (1 key, non-upper) should be filtered as noise")
	}
	if byName["local"] != nil {
		t.Errorf("function-local literal leaked as a module-level table")
	}
}

func names(ents []*FuncSig) []string {
	out := make([]string, 0, len(ents))
	for _, e := range ents {
		out = append(out, e.Name)
	}
	return out
}
