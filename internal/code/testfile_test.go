package code

import "testing"

// TestIsTestPath is the relocated home of the old propose-deriv testGlobs check:
// the file-level test heuristic must catch the four substrates' conventions
// without false-matching paths that merely CONTAIN "test" as a substring
// (attest/, latest/, contest.go) — over-matching silently drops real code from
// the drift scan.
func TestIsTestPath(t *testing.T) {
	hit := []string{
		"foo_test.go", "internal/code/extract_test.go", // Go
		"tests/test_input_llm.py", "test_foo.py", "pkg/bar_test.py", "conftest.py", // Python
		"src/app.test.ts", "src/app.spec.tsx", "lib/util.test.js", // JS/TS
		"crate/tests/integration.rs", "src/widget/tests.rs", // Rust file conventions (tests/ dir + tests.rs basename)
		"src/__tests__/x.ts", "pkg/testdata/golden.go", // dir conventions
	}
	for _, p := range hit {
		if !IsTestPath(p) {
			t.Errorf("IsTestPath(%q) = false, want true", p)
		}
	}
	miss := []string{
		"internal/code/extract.go", "main.py", "src/app.ts", // ordinary source
		"internal/attest/sign.go", // "test" is a substring of a segment, not a segment
		"pkg/latest/cache.go",     // "test" substring of "latest"
		"contest.go",              // ends in "test.go" but is not "_test.go"
		"src/server/handler.rs",   // a production .rs file must stay prod
	}
	for _, p := range miss {
		if IsTestPath(p) {
			t.Errorf("IsTestPath(%q) = true, want false", p)
		}
	}
}

// TestAllTest covers the cluster-level predicate: a set is "all test" only when
// non-empty and every member is test code.
func TestAllTest(t *testing.T) {
	tf := &FuncSig{Test: true}
	pf := &FuncSig{Test: false}
	cases := []struct {
		name string
		in   []*FuncSig
		want bool
	}{
		{"empty", nil, false},
		{"all-test", []*FuncSig{tf, tf}, true},
		{"mixed", []*FuncSig{tf, pf}, false},
		{"all-prod", []*FuncSig{pf, pf}, false},
	}
	for _, c := range cases {
		if got := allTest(c.in); got != c.want {
			t.Errorf("allTest(%s) = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestAsymmetricTestGateDerivation is the core of the test-awareness change: a
// derivation pair between two TEST functions is fixture noise (gated by default),
// but a test↔production pair — a test recomputing a production quantity — is real
// drift and must survive. A production↔production pair is unaffected.
func TestAsymmetricTestGateDerivation(t *testing.T) {
	// Each pair shares a DISTINCT dotted read-set so only the intended pair forms
	// (dotted paths dodge the structural-share gate; write+ret make them
	// construct/construct, dodging the mutator gate).
	mk := func(file, qual string, reads []string, test bool) *FuncSig {
		f := &FuncSig{File: file, Qualname: qual, Name: qual,
			Reads: reads, Writes: []string{"out"}, RetKeys: []string{"out"}, Test: test}
		f.Prepare()
		return f
	}
	r1 := []string{"a.width", "a.pieces", "a.height"}
	r2 := []string{"b.width", "b.pieces", "b.height"}
	r3 := []string{"c.width", "c.pieces", "c.height"}
	all := []*FuncSig{
		mk("p1.go", "ProdA", r1, false), mk("p2.go", "ProdB", r1, false), // prod↔prod
		mk("t1_test.go", "TestA", r2, true), mk("t2_test.go", "TestB", r2, true), // test↔test
		mk("h_test.go", "TestH", r3, true), mk("p3.go", "ProdH", r3, false), // test↔prod
	}
	has := func(cands []SigCandidate, x, y string) bool {
		for _, c := range cands {
			if (c.A.Qualname == x && c.B.Qualname == y) || (c.A.Qualname == y && c.B.Qualname == x) {
				return true
			}
		}
		return false
	}

	def := SharedDerivationCandidates(all, 2, 0.5, 8, false, false, false)
	if !has(def, "ProdA", "ProdB") {
		t.Error("prod↔prod derivation pair must surface")
	}
	if !has(def, "TestH", "ProdH") {
		t.Error("test↔prod derivation pair must survive the gate (a test recomputing a production quantity is drift)")
	}
	if has(def, "TestA", "TestB") {
		t.Error("test↔test derivation pair must be gated by default (shared fixture, not drift)")
	}

	with := SharedDerivationCandidates(all, 2, 0.5, 8, false, false, true)
	if !has(with, "TestA", "TestB") {
		t.Error("--include-tests must surface the test↔test pair")
	}
}

// TestAsymmetricTestGateCluster proves the same asymmetry for the N-ary pass: an
// all-test cluster is dropped by default and kept with IncludeTests, while a
// cluster mixing test and production members always survives.
func TestAsymmetricTestGateCluster(t *testing.T) {
	markTest := func(fns []*FuncSig, test bool) []*FuncSig {
		for _, f := range fns {
			f.Test = test
		}
		return fns
	}
	allTestShells := markTest([]*FuncSig{
		makeShell("EngineT", "step"), makeShell("SessionT", "stepWeb"), makeShell("EngineT", "run"),
	}, true)
	if got := ClusterByTouchpoint(allTestShells, DefaultClusterOptions()); len(got) != 0 {
		t.Fatalf("all-test cluster must be gated by default, got %d cluster(s)", len(got))
	}
	opts := DefaultClusterOptions()
	opts.IncludeTests = true
	if got := ClusterByTouchpoint(allTestShells, opts); len(got) != 1 {
		t.Fatalf("--include-tests must surface the all-test cluster, got %d", len(got))
	}

	// Mixed cluster: two test shells + one production shell — survives by default.
	mixed := []*FuncSig{makeShell("EngineM", "step"), makeShell("SessionM", "stepWeb"), makeShell("EngineM", "run")}
	mixed[0].Test, mixed[1].Test = true, true // mixed[2] stays production
	if got := ClusterByTouchpoint(mixed, DefaultClusterOptions()); len(got) != 1 {
		t.Fatalf("a cluster mixing test and production members must survive, got %d", len(got))
	}
}
