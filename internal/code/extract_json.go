package code

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// JSON corpus extractor — the cross-substrate sibling of the Python table
// extractor. Each JSON OBJECT (top-level and nested) becomes a "corpus-field"
// entity whose field-name SET is its footprint (-> RetKeys), so an authored corpus
// shape (corpus/.../*.json) can be paired against a code table or schema by the
// key-set pass + judge. Pure Go (encoding/json), no shell-out.
//
// Why dedup by key-set: a corpus typically has MANY near-identical objects (e.g.
// every location file carries the same `temporal_markers` columns). Left raw, a
// common column key's posting list explodes past the candidate pass's fanout cap
// and the genuine cross-substrate pair (corpus shape vs the db.py table) gets
// pruned. The SHAPE is what we pair, so identical-key-set objects collapse to one
// representative.

const (
	jsonMaxDepth      = 6    // recursion ceiling per file
	jsonMaxPerFile    = 200  // object cap per file (a giant corpus can't explode)
	jsonMinFields     = 2    // a 0-1 field object isn't a discriminating shape
	jsonMaxSourceLen  = 4000 // bound the judge blob
	jsonMaxRepresKeys = 400  // bound RetKeys for a pathological wide object
)

// extractJSONBatch parses each .json file into corpus-field entities, deduped by
// key-set across the whole batch (kept first occurrence).
func extractJSONBatch(paths []string, root string) ([]*FuncSig, error) {
	var out []*FuncSig
	seen := map[string]bool{} // sorted key signature -> already represented
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue // best-effort: an unreadable file is skipped, not fatal
		}
		var v any
		if err := json.Unmarshal(data, &v); err != nil {
			continue // not valid JSON (JSONL, comments, etc.) — skip
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			rel = p
		}
		stem := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		c := &jsonCollector{rel: rel, stem: stem}
		c.walk(v, "", 0)
		for _, e := range c.out {
			sig := strings.Join(e.RetKeys, "\x00")
			if seen[sig] {
				continue
			}
			seen[sig] = true
			out = append(out, e)
		}
	}
	return out, nil
}

type jsonCollector struct {
	rel, stem string
	out       []*FuncSig
}

func (c *jsonCollector) walk(v any, label string, depth int) {
	if depth > jsonMaxDepth || len(c.out) >= jsonMaxPerFile {
		return
	}
	switch t := v.(type) {
	case map[string]any:
		c.emit(t, label)
		for k, child := range t {
			c.walk(child, k, depth+1)
		}
	case []any:
		for _, child := range t {
			c.walk(child, label, depth+1)
		}
	}
}

func (c *jsonCollector) emit(obj map[string]any, label string) {
	if len(obj) < jsonMinFields || len(c.out) >= jsonMaxPerFile {
		return
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) > jsonMaxRepresKeys {
		keys = keys[:jsonMaxRepresKeys]
	}
	name := c.stem
	if label != "" {
		name = c.stem + "." + label
	}
	src, _ := json.MarshalIndent(obj, "", "  ")
	if len(src) > jsonMaxSourceLen {
		src = src[:jsonMaxSourceLen]
	}
	c.out = append(c.out, &FuncSig{
		File:     c.rel,
		Qualname: name,
		Name:     name,
		Kind:     "corpus-field",
		Line:     1,
		NLines:   1,
		RetKeys:  keys,
		Source:   string(src),
	})
}
