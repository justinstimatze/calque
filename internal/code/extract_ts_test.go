package code

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// tsToolchainAvailable reports whether node + a resolvable typescript exist, so the
// TS tests skip (not fail) on a machine without a JS toolchain — calque's Go build
// must not require node.
func tsToolchainAvailable(t *testing.T, root string) bool {
	t.Helper()
	if _, err := exec.LookPath(nodeBin()); err != nil {
		return false
	}
	// A trivial extraction round-trips only if `typescript` resolves; reuse the real
	// path so the skip condition matches what extractTSBatch actually needs.
	dir := t.TempDir()
	f := filepath.Join(dir, "probe.ts")
	if err := os.WriteFile(f, []byte("export function p(){ return 1 }"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := extractTSBatch([]string{f}, dir); err != nil {
		t.Logf("TS toolchain unavailable (%v) — skipping TS extractor tests", err)
		return false
	}
	return true
}

func sigByQual(sigs []*FuncSig, qual string) *FuncSig {
	for _, s := range sigs {
		if s.Qualname == qual {
			return s
		}
	}
	return nil
}

func TestExtractTSChannels(t *testing.T) {
	dir := t.TempDir()
	if !tsToolchainAvailable(t, dir) {
		t.Skip("node + typescript not available")
	}
	src := `class Engine {
  private _backend: any;
  step(name: string): { ok: boolean; code: number } {
    this.state.count = 1;
    this.cache["key"] = 2;
    doThing(name);
    return { ok: true, code: 200 };
  }
  forward(x: number) {
    return this._backend.run(x);
  }
}
const build = (n: number) => {
  helper.prep();
  return { built: n, label: "ready-now" };
};
function plain() { return 42; }
`
	f := filepath.Join(dir, "fix.ts")
	if err := os.WriteFile(f, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	sigs, err := extractTSBatch([]string{f}, dir)
	if err != nil {
		t.Fatalf("extractTSBatch: %v", err)
	}

	step := sigByQual(sigs, "Engine.step")
	if step == nil {
		t.Fatalf("Engine.step not extracted; got %d sigs", len(sigs))
	}
	assertHas(t, "step.writes", step.Writes, []string{"this.state.count", "this.cache[]"})
	assertHas(t, "step.ret_keys", step.RetKeys, []string{"ok", "code"})
	assertHas(t, "step.calls", step.Calls, []string{"doThing"})
	if step.Delegates {
		t.Error("Engine.step should not delegate")
	}

	fwd := sigByQual(sigs, "Engine.forward")
	if fwd == nil || !fwd.Delegates {
		t.Errorf("Engine.forward should delegate (this._backend.run): %+v", fwd)
	}

	build := sigByQual(sigs, "build")
	if build == nil {
		t.Fatal("const-arrow `build` not extracted")
	}
	assertHas(t, "build.strings", build.Strings, []string{"ready-now"})
	assertHas(t, "build.ret_keys", build.RetKeys, []string{"built", "label"})

	if plain := sigByQual(sigs, "plain"); plain == nil {
		t.Error("top-level function `plain` not extracted")
	}
}

// TestExtractTSReadsSkipsCallee pins the read-set callee rule on the TS extractor:
// a method call's leaf name (this.road.compute in this.road.compute()) must NOT
// appear in reads, while the call's receiver (this.road) and a genuine field read
// (this.terrain.height) still contribute. Mirrors the Go calleeSkip behavior.
func TestExtractTSReadsSkipsCallee(t *testing.T) {
	dir := t.TempDir()
	if !tsToolchainAvailable(t, dir) {
		t.Skip("node + typescript not available")
	}
	src := `class Vehicle {
  derive() {
    const w = this.road.compute();
    const h = this.terrain.height;
    return { w, h };
  }
}
`
	f := filepath.Join(dir, "vehicle.ts")
	if err := os.WriteFile(f, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	sigs, err := extractTSBatch([]string{f}, dir)
	if err != nil {
		t.Fatalf("extractTSBatch: %v", err)
	}
	d := sigByQual(sigs, "Vehicle.derive")
	if d == nil {
		t.Fatalf("Vehicle.derive not extracted; got %d sigs", len(sigs))
	}
	assertHas(t, "derive.reads", d.Reads, []string{"this.road", "this.terrain", "this.terrain.height"})
	for _, r := range d.Reads {
		if r == "this.road.compute" {
			t.Errorf("derive.reads must exclude the callee 'this.road.compute', got %v", d.Reads)
		}
	}
	assertHas(t, "derive.calls", d.Calls, []string{"compute"})
}

// TestExtractTSConsts pins the const-set channel on the TS extractor: a bare
// SCREAMING_SNAKE Identifier (V_BELOW) and a qualified property leaf (geom.V_ROOF)
// land in consts; lowercase identifiers do not.
func TestExtractTSConsts(t *testing.T) {
	dir := t.TempDir()
	if !tsToolchainAvailable(t, dir) {
		t.Skip("node + typescript not available")
	}
	src := `import * as geom from "./geom";
function spanTest(h: number): boolean {
  const limit = V_BELOW;
  const roof = geom.V_ROOF;
  return h > limit && h < roof;
}
`
	f := filepath.Join(dir, "span.ts")
	if err := os.WriteFile(f, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	sigs, err := extractTSBatch([]string{f}, dir)
	if err != nil {
		t.Fatalf("extractTSBatch: %v", err)
	}
	st := sigByQual(sigs, "spanTest")
	if st == nil {
		t.Fatalf("spanTest not extracted; got %d sigs", len(sigs))
	}
	assertHas(t, "spanTest.consts", st.Consts, []string{"V_BELOW", "V_ROOF"})
	for _, c := range st.Consts {
		if c == "limit" || c == "geom" {
			t.Errorf("spanTest.consts must exclude locals, got %v", st.Consts)
		}
	}
}

// A .tsx file with JSX must parse (the extractor selects ScriptKind.TSX).
func TestExtractTSXParses(t *testing.T) {
	dir := t.TempDir()
	if !tsToolchainAvailable(t, dir) {
		t.Skip("node + typescript not available")
	}
	src := `export function View(props: {title: string}) {
  log("render-view");
  return <div className="x">{props.title}</div>;
}
`
	f := filepath.Join(dir, "v.tsx")
	if err := os.WriteFile(f, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	sigs, err := extractTSBatch([]string{f}, dir)
	if err != nil {
		t.Fatalf("extractTSBatch(.tsx): %v", err)
	}
	v := sigByQual(sigs, "View")
	if v == nil {
		t.Fatalf("View not extracted from .tsx; got %d sigs", len(sigs))
	}
	assertHas(t, "View.calls", v.Calls, []string{"log"})
	assertHas(t, "View.strings", v.Strings, []string{"render-view"})
}

// A syntactically broken file must be skipped, not abort the batch.
func TestExtractTSSkipsBroken(t *testing.T) {
	dir := t.TempDir()
	if !tsToolchainAvailable(t, dir) {
		t.Skip("node + typescript not available")
	}
	good := filepath.Join(dir, "good.ts")
	bad := filepath.Join(dir, "bad.ts")
	os.WriteFile(good, []byte("export function g(){ return helper(); }"), 0o600)
	os.WriteFile(bad, []byte("export function (((( {"), 0o600)
	sigs, err := extractTSBatch([]string{bad, good}, dir)
	if err != nil {
		t.Fatalf("batch with one broken file errored: %v", err)
	}
	if sigByQual(sigs, "g") == nil {
		t.Error("good file's function dropped when batched with a broken file")
	}
}

func assertHas(t *testing.T, label string, got, want []string) {
	t.Helper()
	set := map[string]bool{}
	for _, g := range got {
		set[g] = true
	}
	for _, w := range want {
		if !set[w] {
			t.Errorf("%s: missing %q (got %v)", label, w, got)
		}
	}
}
