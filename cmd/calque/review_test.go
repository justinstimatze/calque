package main

import (
	"strings"
	"testing"

	"github.com/justinstimatze/calque/internal/code"
)

// The `%` escape must run first, or a later escape's own `%` gets double-encoded.
func TestGHEscapeData(t *testing.T) {
	if got := ghEscapeData("100% done\nnext\r"); got != "100%25 done%0Anext%0D" {
		t.Errorf("ghEscapeData = %q", got)
	}
}

// Property values additionally escape the `:` and `,` that delimit the list.
func TestGHEscapeProp(t *testing.T) {
	if got := ghEscapeProp("a/b.go:12,x"); got != "a/b.go%3A12%2Cx" {
		t.Errorf("ghEscapeProp = %q", got)
	}
}

// mdCell must not let a value's pipe break out of its table cell.
func TestMdCell(t *testing.T) {
	if got := mdCell("a|b\nc"); got != "a\\|b c" {
		t.Errorf("mdCell = %q", got)
	}
}

func TestRenderStepSummary(t *testing.T) {
	// Empty findings → the clean-bill panel, no table.
	clean := renderStepSummary(checkFindings{}, ".calque/registry.md")
	if !strings.Contains(clean, "## calque drift review") || !strings.Contains(clean, "✓ No new dual-path drift") {
		t.Errorf("clean summary missing header/clean-bill:\n%s", clean)
	}
	if strings.Contains(clean, "| score |") {
		t.Errorf("clean summary must not render a table:\n%s", clean)
	}

	// One suspect → header line + a table row naming both sides.
	mk := func(file, name string, line int) *code.FuncSig {
		f := &code.FuncSig{File: file, Qualname: name, Name: name, Line: line}
		f.Prepare()
		return f
	}
	f := checkFindings{Fresh: []code.Suspicion{{
		Left: mk("a.go", "Foo", 10), Right: mk("b.go", "Bar", 20), Score: 0.42,
		Signals: map[string]float64{},
	}}}
	got := renderStepSummary(f, ".calque/registry.md")
	if !strings.Contains(got, "**1** new suspect pair(s)") || !strings.Contains(got, "| score | suspect | twin of | signal |") {
		t.Errorf("suspect summary missing count/table header:\n%s", got)
	}
	if !strings.Contains(got, "`Foo` (a.go:10)") || !strings.Contains(got, "`Bar` (b.go:20)") {
		t.Errorf("suspect row must name both sides:\n%s", got)
	}
}

func TestGHAnnotation(t *testing.T) {
	withFile := ghAnnotation("warning", "cmd/x.go", 12, "drift here")
	if withFile != "::warning file=cmd/x.go,line=12::drift here" {
		t.Errorf("file annotation = %q", withFile)
	}
	// A fileless annotation omits the file/line props entirely.
	fileless := ghAnnotation("notice", "", 0, "0 new suspects")
	if fileless != "::notice::0 new suspects" {
		t.Errorf("fileless annotation = %q", fileless)
	}
	if strings.Contains(fileless, "file=") {
		t.Errorf("fileless annotation must not carry a file= prop, got %q", fileless)
	}
}
