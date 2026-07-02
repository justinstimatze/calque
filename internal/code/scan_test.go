package code

import "testing"

// TestHasExtractorJSVariants pins the extractor-dispatch gap an external field
// report surfaced: .js/.jsx/.mjs/.cjs were in codeExts (counted as "code") but
// never routed to an extractor, so scripts using those extensions silently
// extracted zero functions. extractTSBatch already handles them (ScriptKind.TSX
// for .jsx, ScriptKind.TS — a superset grammar — for the rest); this only pins
// the dispatch-map entry, not the TS-parser round-trip (extract_ts_test.go does
// that, gated on node availability).
func TestHasExtractorJSVariants(t *testing.T) {
	for _, ext := range []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".svelte"} {
		if !HasExtractor(ext) {
			t.Errorf("HasExtractor(%q) = false, want true", ext)
		}
	}
}
