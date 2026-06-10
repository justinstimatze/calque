package code

import (
	_ "embed"
	"os"
	"os/exec"
)

// pyExtractor is the embedded Python AST extractor (extract.py). Python's own
// ast parses modern Python robustly, so the Go binary shells out to it for .py
// targets rather than embedding a parser. The script emits the same FuncSig JSON
// the go/ast extractor produces.
//
//go:embed extract.py
var pyExtractor string

// pythonBin is the interpreter to run (CALQUE_PYTHON override, else python3).
func pythonBin() string {
	if p := os.Getenv("CALQUE_PYTHON"); p != "" {
		return p
	}
	return "python3"
}

// extractPyBatch runs the embedded extractor once over all .py paths (one
// process per scan, not per file) and returns the parsed FuncSigs (unprepared).
func extractPyBatch(paths []string, root string) ([]*FuncSig, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	cmd := exec.Command(pythonBin(), "-c", pyExtractor, root)
	return runJSONExtractor(cmd, paths, pythonBin(), "Python", "set CALQUE_PYTHON or install python3")
}
