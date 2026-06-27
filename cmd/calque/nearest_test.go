package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPendingContentWrite(t *testing.T) {
	p := preToolUsePayload{ToolName: "Write"}
	p.ToolInput.FilePath = "/x/new.go"
	p.ToolInput.Content = "package x\nfunc A(){}\n"
	path, content, ok := pendingContent(p)
	if !ok || path != "/x/new.go" || content != "package x\nfunc A(){}\n" {
		t.Errorf("Write: got (%q, %q, %v)", path, content, ok)
	}
}

// The load-bearing case: an Edit's new_string is a FRAGMENT, but pendingContent
// composes the full post-edit file from disk so the buffer parses as a unit — the
// failure mode being "search the fragment, extract nothing, hook looks installed
// but stays mute."
func TestPendingContentEditComposesFullFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.go")
	if err := os.WriteFile(f, []byte("package x\n\nfunc A() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := preToolUsePayload{ToolName: "Edit"}
	p.ToolInput.FilePath = f
	p.ToolInput.OldString = "func A() {}"
	p.ToolInput.NewString = "func A() {}\n\nfunc B() int { return 1 }"

	_, content, ok := pendingContent(p)
	if !ok {
		t.Fatal("Edit not handled")
	}
	if !strings.HasPrefix(content, "package x") {
		t.Errorf("composed buffer lost the package clause — would not parse:\n%s", content)
	}
	if !strings.Contains(content, "func B() int") {
		t.Error("composed buffer missing the pending new function B")
	}
	if strings.Count(content, "func A()") != 1 {
		t.Errorf("expected exactly one func A after compose, got:\n%s", content)
	}
}

func TestPendingContentEditNewFileFallback(t *testing.T) {
	p := preToolUsePayload{ToolName: "Edit"}
	p.ToolInput.FilePath = "/nonexistent/x.go"
	p.ToolInput.NewString = "package x\nfunc A(){}\n"
	_, content, ok := pendingContent(p)
	if !ok || !strings.Contains(content, "func A") {
		t.Errorf("new-file Edit fallback: got (%q, %v)", content, ok)
	}
}

func TestPendingContentUnhandledTool(t *testing.T) {
	p := preToolUsePayload{ToolName: "Bash"}
	if _, _, ok := pendingContent(p); ok {
		t.Error("Bash (no pending file content) should not be handled")
	}
}

func TestRepoRelMatchesFuncSigForm(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(dir, "internal", "x.go")
	if got := repoRel(dir, abs); got != filepath.Join("internal", "x.go") {
		t.Errorf("repoRel = %q, want internal/x.go", got)
	}
}
