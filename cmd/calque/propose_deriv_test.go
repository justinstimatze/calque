package main

import (
	"testing"

	"github.com/justinstimatze/calque/internal/glob"
)

// The default test-file exclusion must catch the four substrates' test
// conventions without false-matching paths that merely contain "test" as a
// substring (attest/, latest/, contest.go) — over-exclusion silently drops real
// code from the drift scan.
func TestTestGlobs(t *testing.T) {
	res := glob.Compile(testGlobs)
	hit := []string{
		"foo_test.go", "internal/code/extract_test.go", // Go
		"tests/test_input_llm.py", "test_foo.py", "pkg/bar_test.py", // Python
		"src/app.test.ts", "src/app.spec.tsx", "lib/util.test.js", // JS/TS
		"crate/tests/integration.rs", "src/__tests__/x.ts", // dir conventions
	}
	for _, p := range hit {
		if !glob.MatchAny(res, p) {
			t.Errorf("test glob should match %q", p)
		}
	}
	miss := []string{
		"internal/code/extract.go", "main.py", "src/app.ts", // ordinary source
		"internal/attest/sign.go", // "test" not a path segment
		"pkg/latest/cache.go",     // "test" substring of a dir
		"contest.go",              // ends in test.go but not _test.go
	}
	for _, p := range miss {
		if glob.MatchAny(res, p) {
			t.Errorf("test glob should NOT match %q", p)
		}
	}
}
