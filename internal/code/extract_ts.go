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

// extractTSBatch runs the embedded extractor once over all .ts/.tsx paths (one node
// process per scan, not per file) and returns the parsed FuncSigs (unprepared).
//
// The script is written to a temp .mjs and run as a file (not `node -e`) because it
// uses import.meta.url to resolve the `typescript` module; -e has no module URL.
func extractTSBatch(paths []string, root string) ([]*FuncSig, error) {
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

	// Exit code 3 inside the script is its "typescript module not found" signal; its
	// stderr explains the fix and runJSONExtractor surfaces it verbatim.
	cmd := exec.Command(nodeBin(), script, root)
	return runJSONExtractor(cmd, paths, nodeBin(), "TypeScript", "set CALQUE_NODE or install node")
}
