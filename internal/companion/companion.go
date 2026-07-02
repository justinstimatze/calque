// Package companion is calque's belt-and-suspenders pass: it shells out to
// established Type-1/2 (textual/near-textual clone) detectors — jscpd, dupl —
// so a `calque scan` gives full duplicate-type coverage in one run, not just
// calque's own Type-4 (behavioral-twin) axis. calque's README is explicit
// that Type-1/2 is out of scope for its own engine; this package doesn't
// change that boundary, it just runs the tools that already own it and
// surfaces their report alongside calque's, clearly attributed.
//
// Every tool here is optional and best-effort: absent from $PATH ⇒ skipped
// with an install hint, never fetched or auto-installed (no npx, no `go
// install`). A tool that runs but exits non-zero still has its output
// surfaced — jscpd's exit code reflects its own --threshold, not an error
// calque should swallow the report over.
package companion

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"
)

// companionTimeout bounds one tool's run so a runaway subprocess can't hang
// `calque scan` indefinitely on a large or pathological repo.
const companionTimeout = 2 * time.Minute

// tool describes one external clone detector: its binary name and how to
// build its argv from a repo root.
type tool struct {
	name string
	args func(repo string) []string
}

// tools is the fixed set calque knows how to run. jscpd covers every
// language calque itself extracts (Go/TS/Python/Rust/Svelte) via its generic
// tokenizer; dupl is Go-only (go/parser) but a single static binary with no
// npm/node dependency — kept as a second, narrower option rather than a
// replacement.
var tools = []tool{
	{name: "jscpd", args: func(repo string) []string { return []string{repo} }},
	{name: "dupl", args: func(repo string) []string { return []string{repo} }},
}

// Section is one companion tool's outcome: either its raw report (Ran) or the
// reason it didn't run (Skip).
type Section struct {
	Tool   string
	Ran    bool
	Output string // raw stdout+stderr, only set when Ran
	Skip   string // why it didn't run, only set when !Ran
}

// Run executes every companion tool found on $PATH against repo and returns
// one Section per tool, in the fixed tools order (deterministic output).
func Run(repo string) []Section {
	out := make([]Section, 0, len(tools))
	for _, t := range tools {
		path, err := exec.LookPath(t.name)
		if err != nil {
			out = append(out, Section{Tool: t.name, Skip: installHint(t.name)})
			continue
		}
		out = append(out, runTool(t, path, repo))
	}
	return out
}

func runTool(t tool, path, repo string) Section {
	ctx, cancel := context.WithTimeout(context.Background(), companionTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, t.args(repo)...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run() // best-effort: a non-zero exit (e.g. jscpd's --threshold) is not a calque error
	output := strings.TrimSpace(buf.String())
	if output == "" && err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			output = "(timed out after " + companionTimeout.String() + ")"
		} else {
			output = "(companion tool error: " + err.Error() + ")"
		}
	}
	return Section{Tool: t.name, Ran: true, Output: output}
}

func installHint(name string) string {
	switch name {
	case "jscpd":
		return "not found on $PATH — npm install -g jscpd (multi-language Type-1/2 clone coverage: Go/TS/Python/Rust/Svelte)"
	case "dupl":
		return "not found on $PATH — go install github.com/mibk/dupl@latest (Go-only Type-1/2 clone coverage)"
	default:
		return "not found on $PATH"
	}
}
