package code

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// nodeCountDiscriminates asserts the shared claim behind NodeCount across every
// extractor: two functions can share the same NLines (both are a single physical
// source line) yet differ sharply in body substantiality — a semicolon-packed
// one-liner visits far more AST nodes than a trivial one-liner. If NodeCount were
// just NLines in disguise, this would fail; SizeGate.MinNodes would be no more
// precise than --min-lines already is.
func nodeCountDiscriminates(t *testing.T, sigs []*FuncSig, trivialQual, packedQual string) {
	t.Helper()
	trivial := sigByQual(sigs, trivialQual)
	packed := sigByQual(sigs, packedQual)
	if trivial == nil || packed == nil {
		t.Fatalf("expected both %q and %q extracted; got %d sigs", trivialQual, packedQual, len(sigs))
	}
	if trivial.NLines != packed.NLines {
		t.Fatalf("fixture invariant broken: expected equal NLines (both single physical lines), got trivial=%d packed=%d", trivial.NLines, packed.NLines)
	}
	if trivial.NodeCount == 0 || packed.NodeCount == 0 {
		t.Errorf("NodeCount should be populated (>0) for both fixtures, got trivial=%d packed=%d", trivial.NodeCount, packed.NodeCount)
	}
	if packed.NodeCount <= trivial.NodeCount {
		t.Errorf("packed (4 statements) should have a higher NodeCount than trivial despite equal NLines=%d: trivial=%d packed=%d",
			trivial.NLines, trivial.NodeCount, packed.NodeCount)
	}
}

func TestExtractGoNodeCountDiscriminatesFromNLines(t *testing.T) {
	dir := t.TempDir()
	src := `package p

func trivial() int { return 1 }

func packed() int { a := 1; a += 2; a += 3; a += 4; return a }
`
	f := filepath.Join(dir, "nodecount.go")
	if err := os.WriteFile(f, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	sigs, err := extractGoBatch([]string{f}, dir)
	if err != nil {
		t.Fatalf("extractGoBatch: %v", err)
	}
	nodeCountDiscriminates(t, sigs, "trivial", "packed")
}

func TestExtractPyNodeCountDiscriminatesFromNLines(t *testing.T) {
	if _, err := exec.LookPath(pythonBin()); err != nil {
		t.Skip("python3 not available")
	}
	dir := t.TempDir()
	src := `def trivial(): return 1

def packed(): a = 1; a += 2; a += 3; a += 4; return a
`
	f := filepath.Join(dir, "nodecount.py")
	if err := os.WriteFile(f, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	sigs, err := extractPyBatch([]string{f}, dir)
	if err != nil {
		t.Fatalf("extractPyBatch: %v", err)
	}
	nodeCountDiscriminates(t, sigs, "trivial", "packed")
}

func TestExtractTSNodeCountDiscriminatesFromNLines(t *testing.T) {
	dir := t.TempDir()
	if !tsToolchainAvailable(t, dir) {
		t.Skip("node + typescript not available")
	}
	src := `function trivial() { return 1; }
function packed() { let a = 1; a += 2; a += 3; a += 4; return a; }
`
	f := filepath.Join(dir, "nodecount.ts")
	if err := os.WriteFile(f, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	sigs, err := extractTSBatch([]string{f}, dir)
	if err != nil {
		t.Fatalf("extractTSBatch: %v", err)
	}
	nodeCountDiscriminates(t, sigs, "trivial", "packed")
}

func TestExtractRustNodeCountDiscriminatesFromNLines(t *testing.T) {
	dir := t.TempDir()
	if !rustToolchainAvailable(t, dir) {
		t.Skip("cargo / syn toolchain not available")
	}
	src := `fn trivial() -> i32 { 1 }
fn packed() -> i32 { let mut a = 1; a += 2; a += 3; a += 4; a }
`
	f := filepath.Join(dir, "nodecount.rs")
	if err := os.WriteFile(f, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	sigs, err := extractRustBatch([]string{f}, dir)
	if err != nil {
		t.Fatalf("extractRustBatch: %v", err)
	}
	nodeCountDiscriminates(t, sigs, "trivial", "packed")
}
