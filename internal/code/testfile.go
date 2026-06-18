package code

import (
	"path/filepath"
	"strings"
)

// IsTestPath reports whether a repo-relative source path is TEST code by language
// convention — the file-level half of test attribution. The other half is
// per-extractor inline detection (the Rust extractor flags #[cfg(test)] / #[test]
// functions that live in an otherwise-production .rs file, which no path rule can
// see). This is the SINGLE definition of "what's a test file" — scan/check,
// propose-deriv, and calibration all consult it, so calque can't drift on its own
// test-handling (the exact multi-definition bug it exists to catch).
//
// Conservative by construction: it matches established naming/dir conventions, not
// the word "test" anywhere. A flagship boundary like `--right "testing.py"` (a
// production-side harness, not a unit-test file) is deliberately NOT matched — the
// user frames that pairing explicitly, and it is the canonical test↔prod case the
// asymmetric gate must keep.
func IsTestPath(rel string) bool {
	rel = filepath.ToSlash(rel)

	// Directory conventions: an integration-test or fixture tree (Rust `tests/`,
	// pytest `tests/`, jest `__tests__/`, Go `testdata/`, mock dirs). Any path
	// segment that is one of these marks every file beneath it as test code.
	for _, seg := range strings.Split(rel, "/") {
		switch strings.ToLower(seg) {
		case "tests", "test", "__tests__", "testdata", "__mocks__":
			return true
		}
	}

	base := strings.ToLower(filepath.Base(rel))
	switch {
	case strings.HasSuffix(base, "_test.go"): // Go
		return true
	case strings.HasSuffix(base, "_test.py") || (strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py")): // pytest / unittest
		return true
	case base == "conftest.py": // pytest shared fixtures
		return true
	case base == "tests.rs" || base == "test.rs" || strings.HasSuffix(base, "_test.rs"): // Rust dedicated test file
		return true
	case strings.Contains(base, ".test.") || strings.Contains(base, ".spec."): // JS/TS/JSX jest / vitest / jasmine
		return true
	}
	return false
}

// allTest reports whether every function in a set is test code — used to drop an
// N-ary cluster whose members are all tests (a shared setup/fixture seam), while a
// cluster mixing test and production members survives (a test re-deriving a
// production seam is the interesting case).
func allTest(fns []*FuncSig) bool {
	for _, f := range fns {
		if !f.Test {
			return false
		}
	}
	return len(fns) > 0
}
