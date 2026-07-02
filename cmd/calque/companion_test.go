package main

import (
	"strings"
	"testing"

	"github.com/justinstimatze/calque/internal/companion"
)

func TestRenderCompanionsRanAndSkipped(t *testing.T) {
	out := renderCompanions([]companion.Section{
		{Tool: "jscpd", Ran: true, Output: "found 2 clones"},
		{Tool: "dupl", Skip: "not found on $PATH — go install github.com/mibk/dupl@latest"},
	})
	if !strings.Contains(out, "## jscpd") || !strings.Contains(out, "found 2 clones") {
		t.Errorf("missing jscpd report; got %q", out)
	}
	if !strings.Contains(out, "dupl: not found on $PATH") {
		t.Errorf("missing dupl skip line; got %q", out)
	}
	if strings.Contains(out, "## dupl") {
		t.Errorf("skipped tool must not get a report heading; got %q", out)
	}
}

func TestRenderCompanionsEmptyOutput(t *testing.T) {
	out := renderCompanions([]companion.Section{{Tool: "jscpd", Ran: true, Output: ""}})
	if !strings.Contains(out, "(no duplicates reported)") {
		t.Errorf("expected the no-duplicates placeholder; got %q", out)
	}
}
