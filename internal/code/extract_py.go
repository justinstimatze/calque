package code

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
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
	cmd.Stdin = strings.NewReader(strings.Join(paths, "\n"))
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		if _, lookErr := exec.LookPath(pythonBin()); lookErr != nil {
			return nil, fmt.Errorf("%s not found — needed for Python targets (set CALQUE_PYTHON or install python3)", pythonBin())
		}
		return nil, fmt.Errorf("python extractor failed: %v: %s", err, strings.TrimSpace(errb.String()))
	}
	var sigs []*FuncSig
	if err := json.Unmarshal(out.Bytes(), &sigs); err != nil {
		return nil, fmt.Errorf("python extractor: malformed output: %w", err)
	}
	return sigs, nil
}
