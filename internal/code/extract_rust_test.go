package code

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// rustToolchainAvailable reports whether cargo exists AND the syn helper builds +
// extracts, so the Rust tests skip (not fail) on a machine without a Rust
// toolchain — calque's pure-Go CI must not require cargo. The first call triggers
// the one-time `cargo build` (cached thereafter); reusing the real extractRustBatch
// path makes the skip condition match exactly what a scan needs.
func rustToolchainAvailable(t *testing.T, root string) bool {
	t.Helper()
	if _, err := exec.LookPath(cargoBin()); err != nil {
		return false
	}
	dir := t.TempDir()
	f := filepath.Join(dir, "probe.rs")
	if err := os.WriteFile(f, []byte("fn p() -> u8 { 1 }"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := extractRustBatch([]string{f}, dir); err != nil {
		t.Logf("Rust toolchain unavailable (%v) — skipping Rust extractor tests", err)
		return false
	}
	return true
}

func TestExtractRustChannels(t *testing.T) {
	dir := t.TempDir()
	if !rustToolchainAvailable(t, dir) {
		t.Skip("cargo / syn toolchain not available")
	}
	src := `struct Vehicle { speed: f64, gear: u8 }
struct Pose { x: f64, y: f64, heading: f64 }

impl Vehicle {
    fn apply_throttle(&mut self, amount: f64) {
        self.speed += amount;
        self.gear = pick_gear(self.speed);
        log_event("throttle applied to vehicle");
    }
    fn pose(&self) -> Pose {
        Pose { x: 0.0, y: 0.0, heading: 0.0 }
    }
}

struct VehicleAdapter { inner: Vehicle }

impl VehicleAdapter {
    fn apply_throttle(&mut self, amount: f64) {
        self.inner.apply_throttle(amount)
    }
}

fn pick_gear(speed: f64) -> u8 {
    if speed > 30.0 { 3 } else { 1 }
}
fn log_event(_msg: &str) {}
`
	f := filepath.Join(dir, "vehicle.rs")
	if err := os.WriteFile(f, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	sigs, err := extractRustBatch([]string{f}, dir)
	if err != nil {
		t.Fatalf("extractRustBatch: %v", err)
	}

	// Method on an impl → "Type.method"; writes strip the self. root; calls are
	// leaf names; a >=4-char string literal lands in strings; not a delegation.
	ap := sigByQual(sigs, "Vehicle.apply_throttle")
	if ap == nil {
		t.Fatalf("Vehicle.apply_throttle not extracted; got %d sigs", len(sigs))
	}
	assertHas(t, "apply_throttle.writes", ap.Writes, []string{"speed", "gear"})
	// reads = derivation inputs: `self.speed` (a `+=` target and a call arg) is read;
	// `self.gear` is a plain-`=` LHS (pure write) and must NOT appear in reads.
	assertHas(t, "apply_throttle.reads", ap.Reads, []string{"speed"})
	for _, r := range ap.Reads {
		if r == "gear" {
			t.Errorf("apply_throttle.reads must exclude the plain-= LHS 'gear', got %v", ap.Reads)
		}
	}
	assertHas(t, "apply_throttle.calls", ap.Calls, []string{"pick_gear", "log_event"})
	assertHas(t, "apply_throttle.strings", ap.Strings, []string{"throttle applied to vehicle"})
	if ap.Delegates {
		t.Error("Vehicle.apply_throttle should not delegate")
	}
	if ap.Line == 0 || ap.NLines == 0 {
		t.Errorf("apply_throttle line/n_lines should be > 0, got line=%d n_lines=%d", ap.Line, ap.NLines)
	}

	// A returned struct literal contributes its field names as ret_keys.
	pose := sigByQual(sigs, "Vehicle.pose")
	if pose == nil {
		t.Fatal("Vehicle.pose not extracted")
	}
	assertHas(t, "pose.ret_keys", pose.RetKeys, []string{"x", "y", "heading"})

	// self.inner.foo() forwards through a delegation root → delegates=true.
	adapter := sigByQual(sigs, "VehicleAdapter.apply_throttle")
	if adapter == nil || !adapter.Delegates {
		t.Errorf("VehicleAdapter.apply_throttle should delegate (self.inner): %+v", adapter)
	}

	// Free functions are extracted with a bare qualname.
	if sigByQual(sigs, "pick_gear") == nil {
		t.Error("free function pick_gear not extracted")
	}
}

// TestExtractRustConsts pins the const-set channel on the Rust extractor (where the
// UPPER_SNAKE const convention makes it strongest): a bare V_BELOW and a qualified
// geom::V_ROOF path leaf land in consts; lowercase locals do not.
func TestExtractRustConsts(t *testing.T) {
	dir := t.TempDir()
	if !rustToolchainAvailable(t, dir) {
		t.Skip("cargo / syn toolchain not available")
	}
	src := `const V_BELOW: f64 = -1.0;

fn span_test(h: f64) -> bool {
    let limit = V_BELOW;
    let roof = geom::V_ROOF;
    h > limit && h < roof
}
`
	f := filepath.Join(dir, "span.rs")
	if err := os.WriteFile(f, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	sigs, err := extractRustBatch([]string{f}, dir)
	if err != nil {
		t.Fatalf("extractRustBatch: %v", err)
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
	// decl_consts (item 16): only `const V_BELOW` is DECLARED in this file; V_ROOF is
	// referenced via geom::V_ROOF but declared elsewhere, so it must not appear.
	assertHas(t, "span_test.decl_consts", st.DeclConsts, []string{"V_BELOW"})
	for _, c := range st.DeclConsts {
		if c == "V_ROOF" {
			t.Errorf("decl_consts must exclude reference-only V_ROOF, got %v", st.DeclConsts)
		}
	}
}

// A syntactically broken file must be skipped, not abort the batch.
func TestExtractRustSkipsBroken(t *testing.T) {
	dir := t.TempDir()
	if !rustToolchainAvailable(t, dir) {
		t.Skip("cargo / syn toolchain not available")
	}
	good := filepath.Join(dir, "good.rs")
	bad := filepath.Join(dir, "bad.rs")
	os.WriteFile(good, []byte("fn g() -> u8 { helper() }\nfn helper() -> u8 { 1 }"), 0o600)
	os.WriteFile(bad, []byte("fn (((( {"), 0o600)
	sigs, err := extractRustBatch([]string{bad, good}, dir)
	if err != nil {
		t.Fatalf("batch with one broken file errored: %v", err)
	}
	if sigByQual(sigs, "g") == nil {
		t.Error("good file's function dropped when batched with a broken file")
	}
}

// Rust unit tests usually live in a #[cfg(test)] mod in the SAME file as the
// production code, so no file-path rule can see them — the extractor must flag
// them inline. A #[test] fn and an impl method under #[cfg(test)] are test code;
// the production fn alongside them is not.
func TestExtractRustCfgTest(t *testing.T) {
	dir := t.TempDir()
	if !rustToolchainAvailable(t, dir) {
		t.Skip("cargo / syn toolchain not available")
	}
	src := `fn production(h: f64) -> f64 { h * 2.0 }

#[cfg(test)]
mod tests {
    struct H;
    impl H {
        #[test]
        fn checks_span() { let _ = production(1.0); }
    }

    #[test]
    fn checks_root() { let _ = production(2.0); }
}
`
	f := filepath.Join(dir, "span.rs")
	if err := os.WriteFile(f, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	sigs, err := extractRustBatch([]string{f}, dir)
	if err != nil {
		t.Fatalf("extractRustBatch: %v", err)
	}
	prod := sigByQual(sigs, "production")
	if prod == nil {
		t.Fatalf("production fn not extracted; got %d sigs", len(sigs))
	}
	if prod.Test {
		t.Error("production fn must NOT be flagged test")
	}
	for _, qual := range []string{"checks_root", "H.checks_span"} {
		s := sigByQual(sigs, qual)
		if s == nil {
			t.Fatalf("%s not extracted; got %d sigs", qual, len(sigs))
		}
		if !s.Test {
			t.Errorf("%s under #[cfg(test)] must be flagged test", qual)
		}
	}
}
