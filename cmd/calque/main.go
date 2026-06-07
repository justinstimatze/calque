// Command calque is a substrate-general drift nose: it surfaces the meta-bug
// "one contract / concept / value expressed in N places that silently drift"
// across code, prose, and (planned) config — recall-first, with a persistent
// registry and calibration. A single Go binary so a repo points at one path.
//
// Axes (substrate-specific recall extractors; one shared spine):
//
//	code  — scan            dual-path / behavioral-twin (Type-4) suspects
//	prose — vocab-report    hyphenated-compound frequency surface
//	        synonym-report  embedding near-synonyms (single words)
//	config/env              (planned; see DESIGN_NOTES §14)
//
// Spine (substrate-general): check (registry-aware gate) · doctor + mark-fire
// (calibration) · the registry at .calque/registry.md.
//
// Consolidated from two sibling tools that independently grew the same loop:
// calque (code; formerly Python) and cupel (prose; Go, MIT). See DESIGN_NOTES §16.
package main

import (
	"fmt"
	"os"
	"runtime/debug"
)

// version is "dev" by default and baked at release time via -ldflags:
//
//	go install -ldflags "-X main.version=$(git describe --tags --always --dirty)" ./cmd/calque
//
// The git tag is the single source of truth — there is no hand-maintained
// version constant to drift. buildVersion() resolves it. (Pattern: hindcast.)
var version = "dev"

// buildVersion reports the binary's version, preferring (in order): a release
// value baked via -ldflags; the module version when installed with
// `go install …@vX.Y.Z`; the embedded VCS commit (+dirty) for local `go build`;
// then "dev" (e.g. a tarball build outside a git tree).
func buildVersion() string {
	if version != "dev" {
		return version
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	if v := bi.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	var rev, dirty string
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) >= 12 {
				rev = s.Value[:12]
			} else {
				rev = s.Value
			}
		case "vcs.modified":
			if s.Value == "true" {
				dirty = "-dirty"
			}
		}
	}
	if rev != "" {
		return rev + dirty
	}
	return version
}

const help = `calque — substrate-general drift nose

Surfaces "one contract/concept/value defined in N places that drift" — recall
first, you (or an LLM) adjudicate, the registry remembers.

Usage:
  calque scan            code axis: rank dual-path / behavioral-twin (Type-4)
                         suspects across a boundary  [--left G --right G ...]
  calque vocab-report    prose axis: frequency surface of hyphenated compounds
  calque synonym-report  prose axis: embedding near-synonyms (single words)
  calque check           spine: registry-aware gate — flag new/drifted vs the
                         registry  (warn-only; --strict to exit 1)
  calque hook            spine: wire check into a git pre-commit / Stop hook
                         (calque hook install — auto-installs pre-commit)
  calque doctor          spine: calibration rollup (fire-rate, hit-rate)
  calque mark-fire <id> <verdict>   spine: tag a finding  useful|mixed|not-useful
  calque version         print the version (git tag is the source of truth)

Registry: .calque/registry.md (git-tracked memory of adjudicated pairs/groups).
Docs: docs/DESIGN_NOTES.md (architecture), docs/PATTERN_CATALOG.md (the shapes).
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(help)
		return
	}
	switch os.Args[1] {
	case "scan":
		runScan(os.Args[2:])
	case "vocab-report":
		runVocabReport(os.Args[2:])
	case "synonym-report":
		runSynonymReport(os.Args[2:])
	case "check":
		runCheck(os.Args[2:])
	case "hook":
		runHook(os.Args[2:])
	case "doctor":
		runDoctor(os.Args[2:])
	case "mark-fire":
		runMarkFire(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("calque", buildVersion())
	case "-h", "--help", "help":
		fmt.Print(help)
	default:
		fmt.Fprintf(os.Stderr, "calque: unknown command %q\n\n%s", os.Args[1], help)
		os.Exit(2)
	}
}

// --- subcommand stubs (ported incrementally; see DESIGN_NOTES §16) ---
//
// Each prints its port plan and exits 70 (EX_SOFTWARE: not ready) so the
// skeleton builds and runs while the axes land one at a time.

func runDoctor(args []string) {
	stub("doctor", "spine: calibration rollup — move from cupel cmd/cupel/calib.go")
}

func runMarkFire(args []string) {
	stub("mark-fire", "spine: tag a finding verdict — move from cupel cmd/cupel/calib.go")
}

func stub(name, plan string) {
	fmt.Fprintf(os.Stderr, "calque %s: not yet implemented — %s\n", name, plan)
	os.Exit(70)
}
