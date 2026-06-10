package code

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// runJSONExtractor runs a subprocess extractor — one that reads newline-separated
// file paths on stdin and emits the FuncSig JSON array — and parses its output.
// The python3 (.py) and node (.ts/.tsx) extractors both go through here so the
// stdin/stdout/error/unmarshal handling is single-sourced and can't drift between
// them (calque flagged extractPyBatch≟extractTSBatch as a dual path; this is the
// collapse). The caller builds the *exec.Cmd (interpreter + how the script is
// passed); bin/lang/envHint only shape the error messages.
func runJSONExtractor(cmd *exec.Cmd, paths []string, bin, lang, envHint string) ([]*FuncSig, error) {
	cmd.Stdin = strings.NewReader(strings.Join(paths, "\n"))
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		if _, lookErr := exec.LookPath(bin); lookErr != nil {
			return nil, fmt.Errorf("%s not found — needed for %s targets (%s)", bin, lang, envHint)
		}
		return nil, fmt.Errorf("%s extractor failed: %v: %s", lang, err, strings.TrimSpace(errb.String()))
	}
	var sigs []*FuncSig
	if err := json.Unmarshal(out.Bytes(), &sigs); err != nil {
		return nil, fmt.Errorf("%s extractor: malformed output: %w", lang, err)
	}
	return sigs, nil
}
