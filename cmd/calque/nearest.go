package main

// nearest — the author-time surface. Given the PENDING content of an edit (read
// as a Claude Code PreToolUse hook payload on stdin), it surfaces the existing
// functions that already occupy the same seam as the function being written —
// "before you write this, these already exist" — so the DRY-vs-write-new call is
// made with the twin in view instead of blind. The inverse of `check`/`review`,
// which find drift after both copies exist. Advisory and silent-unless-strong:
// it prints nothing (exit 0) when no corpus function scores above --threshold, so
// the PreToolUse hook injects context only when there's a real twin to report.

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/justinstimatze/calque/internal/code"
)

// calque's normalized suspicion scores run low — genuine dual-path twins land
// ~0.30–0.60 — so the "strong enough to interrupt authoring" bar is ~0.45, not
// the ~0.85 a naive 0–1 intuition would pick. Tool-private on purpose — a hook
// shouldn't hardcode calque's score scale.
const nearestDefaultThreshold = 0.45

// preToolUsePayload is the subset of Claude Code's PreToolUse stdin JSON that
// nearest reads. Edit carries old_string/new_string; Write carries content.
type preToolUsePayload struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		FilePath   string `json:"file_path"`
		OldString  string `json:"old_string"`
		NewString  string `json:"new_string"`
		Content    string `json:"content"`
		ReplaceAll bool   `json:"replace_all"`
	} `json:"tool_input"`
}

func runNearest(args []string) {
	fs := flag.NewFlagSet("nearest", flag.ContinueOnError)
	repo := fs.String("repo", ".", "repository root to search for twins")
	exclude := fs.String("exclude", "", "comma-separated globs to exclude from the corpus")
	stdin := fs.Bool("stdin", false, "read a Claude Code PreToolUse payload (JSON) from stdin and search the PENDING edit")
	threshold := fs.Float64("threshold", nearestDefaultThreshold, "minimum suspicion score to surface a twin")
	top := fs.Int("top", 5, "max twins to surface per pending function")
	minLines := fs.Int("min-lines", 3, "ignore corpus functions shorter than this many lines")
	minNodes := fs.Int("min-nodes", 0, "size gate on AST-node count of the function body; 0 disables (default, matches current behavior exactly)")
	distanceBoost := fs.Bool("distance-boost", false, "boost score for pairs sitting far apart (cross-directory weighted more than same-file line distance); default off")
	includeTests := fs.Bool("include-tests", false, "include test↔test twins (default: only test↔prod and prod↔prod)")
	hookFmt := fs.Bool("hook", false, "emit a Claude Code PreToolUse JSON envelope (hookSpecificOutput.additionalContext) instead of plain text — for use as a settings.json hook")
	if err := fs.Parse(args); err != nil {
		return
	}
	if !*stdin {
		fmt.Fprintln(os.Stderr, "calque nearest: --stdin is required (reads a PreToolUse hook payload); see `calque help`")
		os.Exit(2)
	}

	// Every failure below is silent (exit 0): an advisory author-time hook must
	// never break the edit it is observing. Worst case it simply says nothing.
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return
	}
	var p preToolUsePayload
	if json.Unmarshal(raw, &p) != nil {
		return
	}
	path, content, ok := pendingContent(p)
	if !ok {
		return
	}
	ext := strings.ToLower(filepath.Ext(path))
	if !code.HasExtractor(ext) {
		return // a language calque doesn't parse → nothing to say
	}

	rel := repoRel(*repo, path)
	queries, err := code.ExtractPending(content, ext, rel)
	if err != nil || len(queries) == 0 {
		return // unparseable, or the edit introduced no function → silent
	}

	_ = applyCalibratedWeights(*repo, false)
	code.SetDistanceBoost(*distanceBoost)
	corpus, _, err := code.ExtractCached(*repo, splitCSV(*exclude))
	if err != nil {
		return
	}

	gate := code.SizeGate{MinLines: *minLines, MinNodes: *minNodes}
	var b strings.Builder
	for _, q := range queries {
		for _, h := range code.Nearest(q, corpus, gate, *threshold, *top, *includeTests) {
			fmt.Fprintf(&b, "- `%s` (%s:%d) may already exist — twin of `%s` (%s:%d) [%.2f] %s\n",
				q.Name, rel, q.Line, h.Right.Qualname, h.Right.File, h.Right.Line, h.Score, h.Reason())
		}
	}
	if b.Len() == 0 {
		return // silent-unless-strong: nothing crossed the threshold → say nothing
	}
	msg := "calque (drift-nose): before writing, check these existing functions that share a seam:\n" + b.String()
	if *hookFmt {
		emitHookContext(msg)
		return
	}
	fmt.Print(msg)
}

// pendingContent reconstructs the full post-edit file content from a PreToolUse
// payload — the world as it WILL be after the tool runs, not as it is on disk.
// For Edit it applies old→new onto the current file (so the parsed buffer is a
// complete compilation unit, not a fragment); for Write it is the literal new
// content. A new-file Edit (no disk file) falls back to new_string alone.
func pendingContent(p preToolUsePayload) (path, content string, ok bool) {
	in := p.ToolInput
	switch p.ToolName {
	case "Write":
		return in.FilePath, in.Content, in.FilePath != ""
	case "Edit":
		if in.FilePath == "" {
			return "", "", false
		}
		disk, err := os.ReadFile(in.FilePath)
		if err != nil {
			return in.FilePath, in.NewString, in.NewString != "" // new file via Edit
		}
		s := string(disk)
		if in.ReplaceAll {
			s = strings.ReplaceAll(s, in.OldString, in.NewString)
		} else {
			s = strings.Replace(s, in.OldString, in.NewString, 1)
		}
		return in.FilePath, s, true
	}
	return "", "", false // MultiEdit/other tools: not handled in v1
}

// repoRel renders an absolute payload file_path in the same repo-relative form
// FuncSig.File uses, so a pending function's Key matches its prior version in the
// corpus and Nearest excludes itself. Resolves repo to absolute first because the
// payload path is absolute while --repo is often ".".
func repoRel(repo, path string) string {
	abs, err := filepath.Abs(repo)
	if err != nil {
		return path
	}
	if rel, err := filepath.Rel(abs, path); err == nil {
		return rel
	}
	return path
}

// emitHookContext wraps text in the Claude Code PreToolUse hook envelope so the
// message lands in the model's context as additionalContext WITHOUT blocking the
// edit (PreToolUse honors hookSpecificOutput.additionalContext and lets the tool
// proceed). Emitted only when there's a twin to report; silence is an empty
// stdout, which the hook runner treats as a no-op.
func emitHookContext(text string) {
	var out struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	out.HookSpecificOutput.HookEventName = "PreToolUse"
	out.HookSpecificOutput.AdditionalContext = text
	enc, err := json.Marshal(out)
	if err != nil {
		return
	}
	fmt.Println(string(enc))
}
