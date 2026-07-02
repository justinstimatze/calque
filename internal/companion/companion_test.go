package companion

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestRunSkipsAbsentTools pins the graceful-degrade path: with neither tool on
// $PATH, Run must never attempt to fetch/install one — every section reports
// Skip with an install hint, none report Ran.
func TestRunSkipsAbsentTools(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // empty dir on $PATH — neither tool resolvable
	sections := Run(".")
	if len(sections) != len(tools) {
		t.Fatalf("got %d sections, want %d", len(sections), len(tools))
	}
	for _, s := range sections {
		if s.Ran {
			t.Errorf("%s: Ran = true with empty $PATH", s.Tool)
		}
		if s.Skip == "" {
			t.Errorf("%s: Skip is empty", s.Tool)
		}
	}
}

// TestRunCapturesToolOutput proves the LookPath+exec+capture plumbing works
// against a fake "jscpd" on $PATH, without depending on the real npm package
// being installed on the test machine.
func TestRunCapturesToolOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake binary is a POSIX shell script")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "jscpd")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho fake-clone-report\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	sections := Run(".")
	var jscpd *Section
	for i := range sections {
		if sections[i].Tool == "jscpd" {
			jscpd = &sections[i]
		}
	}
	if jscpd == nil || !jscpd.Ran {
		t.Fatalf("jscpd section missing or not run: %+v", jscpd)
	}
	if !strings.Contains(jscpd.Output, "fake-clone-report") {
		t.Errorf("output = %q, want to contain fake-clone-report", jscpd.Output)
	}

	var dupl *Section
	for i := range sections {
		if sections[i].Tool == "dupl" {
			dupl = &sections[i]
		}
	}
	if dupl == nil || dupl.Ran {
		t.Errorf("dupl section should be skipped (not on fake $PATH): %+v", dupl)
	}
}

// TestRunNonZeroExitStillSurfacesOutput pins the "best-effort" contract: a
// tool exiting non-zero (jscpd's own --threshold behavior) must still have
// its stdout surfaced, not be treated as a calque error.
func TestRunNonZeroExitStillSurfacesOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake binary is a POSIX shell script")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "dupl")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho clones-found-report\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	sections := Run(".")
	for _, s := range sections {
		if s.Tool != "dupl" {
			continue
		}
		if !s.Ran {
			t.Fatalf("dupl section not run: %+v", s)
		}
		if !strings.Contains(s.Output, "clones-found-report") {
			t.Errorf("non-zero exit swallowed the report; output = %q", s.Output)
		}
	}
}
