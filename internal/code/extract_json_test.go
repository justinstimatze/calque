package code

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// JSON corpus extraction: each object's field set becomes a corpus-field entity's
// RetKeys (the cross-substrate footprint), with a Source blob for the judge.
func TestExtractJSONCorpusFields(t *testing.T) {
	repo := t.TempDir()
	loc := filepath.Join(repo, "corpus", "locations")
	if err := os.MkdirAll(loc, 0o755); err != nil {
		t.Fatal(err)
	}
	doc := `{
	  "id": "mine",
	  "name": "The Mine",
	  "temporal_markers": [
	    {"source_era": "1890", "description": "ore cart", "trigger": "enter", "intensity": 2},
	    {"source_era": "1920", "description": "collapse", "trigger": "look", "intensity": 5}
	  ]
	}`
	if err := os.WriteFile(filepath.Join(loc, "mine.json"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	ents, _, err := ExtractSymbols(repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	var marker, top *FuncSig
	for _, e := range ents {
		if e.Kind != "corpus-field" {
			t.Errorf("unexpected kind %q", e.Kind)
		}
		if strings.Contains(e.Name, "temporal_markers") {
			marker = e
		} else {
			top = e
		}
	}
	if top == nil {
		t.Fatal("top-level location object not extracted")
	}
	if marker == nil {
		t.Fatal("temporal_markers element object not extracted")
	}
	want := map[string]bool{"source_era": true, "description": true, "trigger": true, "intensity": true}
	for _, k := range marker.RetKeys {
		delete(want, k)
	}
	if len(want) != 0 {
		t.Errorf("marker missing keys %v (got %v)", want, marker.RetKeys)
	}
	if marker.Source == "" || !strings.Contains(marker.Source, "source_era") {
		t.Error("marker Source blob missing or empty")
	}
}

// Identical-shape corpus objects collapse to one representative, so a common column
// key can't blow past the candidate pass's fanout cap and prune the real cross pair.
func TestExtractJSONDedupByKeySet(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "corpus"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"a", "b", "c"} {
		if err := os.WriteFile(filepath.Join(repo, "corpus", n+".json"), []byte(`{"x":1,"y":2,"z":3}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ents, _, err := ExtractSymbols(repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range ents {
		if e.Kind == "corpus-field" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("3 identical-shape objects should dedup to 1 corpus-field, got %d", n)
	}
}
