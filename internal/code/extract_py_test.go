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

V_BELOW = -1.0

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
	// decl_consts (item 16): the module-level `V_BELOW = -1.0` is DECLARED; V_ROOF is
	// referenced via geom.V_ROOF but declared elsewhere, so it must not appear.
	assertHas(t, "span_test.decl_consts", st.DeclConsts, []string{"V_BELOW"})
	for _, c := range st.DeclConsts {
		if c == "V_ROOF" {
			t.Errorf("decl_consts must exclude reference-only V_ROOF, got %v", st.DeclConsts)
		}
	}
}

// TestExtractPySig pins the Type-4 signature-rarity channel (Sig) on the Python
// extractor: an annotated function's Sig reflects its declared types, an
// unannotated one falls back to "?" per param/return (the None-guard), and a
// method's leading "self" is excluded from the param list — mirroring Go's
// receiver exclusion and TS's exclusion of "this" — so a plain function and a
// method doing the same thing can still bucket together. Skips when python3
// is absent.
func TestExtractPySig(t *testing.T) {
	if _, err := exec.LookPath(pythonBin()); err != nil {
		t.Skip("python3 not available")
	}
	dir := t.TempDir()
	src := `class Widget:
    def resize(self, w: int, h: int) -> "Widget":
        return self

def untyped(a, b):
    return a + b
`
	f := filepath.Join(dir, "widget.py")
	if err := os.WriteFile(f, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	sigs, err := extractPyBatch([]string{f}, dir)
	if err != nil {
		t.Fatalf("extractPyBatch: %v", err)
	}
	resize := sigByQual(sigs, "Widget.resize")
	if resize == nil {
		t.Fatalf("Widget.resize not extracted; got %d sigs", len(sigs))
	}
	if want := `(int,int)=>'Widget'`; resize.Sig != want {
		t.Errorf("Widget.resize.Sig = %q, want %q", resize.Sig, want)
	}
	untyped := sigByQual(sigs, "untyped")
	if untyped == nil {
		t.Fatalf("untyped not extracted; got %d sigs", len(sigs))
	}
	if want := "(?,?)=>?"; untyped.Sig != want {
		t.Errorf("untyped.Sig = %q, want %q", untyped.Sig, want)
	}
}

// TestExtractPySigTypingNoise pins the informativeness side: a signature built
// entirely from typing-module generics and builtins must NOT register as
// informative post-stoplist-extension — it's noise, not a domain type.
func TestExtractPySigTypingNoise(t *testing.T) {
	if _, err := exec.LookPath(pythonBin()); err != nil {
		t.Skip("python3 not available")
	}
	dir := t.TempDir()
	src := `from typing import List, Optional

def collect(xs: List[str]) -> Optional[str]:
    return xs[0] if xs else None
`
	f := filepath.Join(dir, "collect.py")
	if err := os.WriteFile(f, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	sigs, err := extractPyBatch([]string{f}, dir)
	if err != nil {
		t.Fatalf("extractPyBatch: %v", err)
	}
	collect := sigByQual(sigs, "collect")
	if collect == nil {
		t.Fatalf("collect not extracted; got %d sigs", len(sigs))
	}
	if signatureInformative(collect.Sig) {
		t.Errorf("a typing-generic-only signature must not register as informative, got %q", collect.Sig)
	}
}
