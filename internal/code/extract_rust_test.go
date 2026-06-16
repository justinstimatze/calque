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
