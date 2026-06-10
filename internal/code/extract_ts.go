package code

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// tsExtractor is the embedded node/TypeScript-compiler-API extractor (extract_ts.mjs).
// TypeScript's own compiler parses modern TS/TSX robustly, so the Go binary shells
// out to node for .ts/.tsx targets rather than embedding a parser — the same pattern
// as the python3 path for .py. The script emits the same FuncSig JSON the go/ast
// extractor produces.
//
//go:embed extract_ts.mjs
var tsExtractor string

// nodeBin is the Node interpreter to run (CALQUE_NODE override, else node).
func nodeBin() string {
	if n := os.Getenv("CALQUE_NODE"); n != "" {
		return n
	}
	return "node"
}

// extractTSBatch extracts FUNCTIONS from .ts/.tsx paths (the code axis).
func extractTSBatch(paths []string, root string) ([]*FuncSig, error) {
	return runTSExtractor(paths, root, "")
}

// extractTSSymbols extracts module-level TABLES from .ts/.tsx paths (the cross-
// substrate axis) — object/array literal constants, their key set in RetKeys. Shares
// the temp-script + process setup with extractTSBatch via runTSExtractor.
func extractTSSymbols(paths []string, root string) ([]*FuncSig, error) {
	return runTSExtractor(paths, root, "symbols")
}

// runTSExtractor writes the embedded TS extractor to a temp .mjs and runs it once over
// all paths in the given mode ("" = functions, "symbols" = module-level tables) — one
// node process per scan, not per file. Single-sources the temp-script + process setup
// so the function and symbol extractors can't drift (mirrors runPyExtractor; the same
// collapse the self-scan asks for on dual extractor paths).
//
// The script is written to a temp .mjs and run as a file (not `node -e`) because it
// uses import.meta.url to resolve the `typescript` module; -e has no module URL.
func runTSExtractor(paths []string, root, mode string) ([]*FuncSig, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	dir, err := os.MkdirTemp("", "calque-ts-")
	if err != nil {
		return nil, fmt.Errorf("ts extractor: temp dir: %w", err)
	}
	defer os.RemoveAll(dir)
	script := filepath.Join(dir, "extract_ts.mjs")
	if err := os.WriteFile(script, []byte(tsExtractor), 0o600); err != nil {
		return nil, fmt.Errorf("ts extractor: writing script: %w", err)
	}

	args := []string{script, root}
	if mode != "" {
		args = append(args, mode)
	}
	// Exit code 3 inside the script is its "typescript module not found" signal; its
	// stderr explains the fix and runJSONExtractor surfaces it verbatim.
	cmd := exec.Command(nodeBin(), args...)
	return runJSONExtractor(cmd, paths, nodeBin(), "TypeScript", "set CALQUE_NODE or install node")
}
