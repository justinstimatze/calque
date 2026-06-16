package code

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestExtractPyReadsSkipsCallee pins the read-set callee rule on the Python
// extractor: a method call's leaf name (road.compute in self.road.compute()) must
// NOT appear in reads — a call name is not a field read — while the call's receiver
// (road) and a genuine field read (terrain.height) still contribute. Mirrors the Go
// calleeSkip behavior (TestExtractGoReads). Skips when python3 is absent so pure-Go
// CI stays green.
func TestExtractPyReadsSkipsCallee(t *testing.T) {
	if _, err := exec.LookPath(pythonBin()); err != nil {
		t.Skip("python3 not available")
	}
	dir := t.TempDir()
	src := `class Vehicle:
    def derive(self):
        w = self.road.compute()
        h = self.terrain.height
        return {"w": w, "h": h}
`
	f := filepath.Join(dir, "vehicle.py")
	if err := os.WriteFile(f, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	sigs, err := extractPyBatch([]string{f}, dir)
	if err != nil {
		t.Fatalf("extractPyBatch: %v", err)
	}
	d := sigByQual(sigs, "Vehicle.derive")
	if d == nil {
		t.Fatalf("Vehicle.derive not extracted; got %d sigs", len(sigs))
	}
	// Receiver field + genuine field read survive; the call name does not.
	assertHas(t, "derive.reads", d.Reads, []string{"road", "terrain", "terrain.height"})
	for _, r := range d.Reads {
		if r == "road.compute" {
			t.Errorf("derive.reads must exclude the callee 'road.compute', got %v", d.Reads)
		}
	}
	assertHas(t, "derive.calls", d.Calls, []string{"compute"})
}

// TestExtractPyConsts pins the const-set channel on the Python extractor: a bare
// SCREAMING_SNAKE Name (V_BELOW) and a qualified attribute leaf (geom.V_ROOF) land in
// consts; lowercase names do not. Skips when python3 is absent.
func TestExtractPyConsts(t *testing.T) {
	if _, err := exec.LookPath(pythonBin()); err != nil {
		t.Skip("python3 not available")
	}
	dir := t.TempDir()
	src := `import geom

def span_test(h):
    limit = V_BELOW
    roof = geom.V_ROOF
    return limit < h < roof
`
	f := filepath.Join(dir, "span.py")
	if err := os.WriteFile(f, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	sigs, err := extractPyBatch([]string{f}, dir)
	if err != nil {
		t.Fatalf("extractPyBatch: %v", err)
	}
	st := sigByQual(sigs, "span_test")
	if st == nil {
		t.Fatalf("span_test not extracted; got %d sigs", len(sigs))
	}
	assertHas(t, "span_test.consts", st.Consts, []string{"V_BELOW", "V_ROOF"})
	for _, c := range st.Consts {
		if c == "limit" || c == "geom" {
			t.Errorf("span_test.consts must exclude locals, got %v", st.Consts)
		}
	}
}
