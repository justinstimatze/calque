package code

import (
	"os"
	"path/filepath"
	"testing"
)

// writeBranchFixture writes src to a temp .go file and extracts its Kind:"branch"
// fragments via extractGoBranchesBatch.
func writeBranchFixture(t *testing.T, src string) []*FuncSig {
	t.Helper()
	dir := t.TempDir()
	f := filepath.Join(dir, "branches.go")
	if err := os.WriteFile(f, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	sigs, err := extractGoBranchesBatch([]string{f}, dir)
	if err != nil {
		t.Fatalf("extractGoBranchesBatch: %v", err)
	}
	return sigs
}

// TestExtractGoBranchesFindsDuplicatedIfElseArms is the core claim behind
// Feature C: two arms of one if/else that do the same thing (shared calls,
// writes, and a string literal) extract as two Kind:"branch" fragments that
// Rank scores as a real candidate pair — the "dual path" this axis targets.
func TestExtractGoBranchesFindsDuplicatedIfElseArms(t *testing.T) {
	src := `package p

func handleRequest(legacy bool) string {
	if legacy {
		logEvent("processed_ok")
		result.status = "done"
		return buildResponse("processed_ok")
	} else {
		logEvent("processed_ok")
		result.status = "done"
		return buildResponse("processed_ok")
	}
}
`
	sigs := writeBranchFixture(t, src)
	if len(sigs) != 2 {
		t.Fatalf("expected 2 branch fragments (if arm + else arm), got %d", len(sigs))
	}
	for _, s := range sigs {
		if s.Kind != "branch" {
			t.Errorf("expected Kind=branch, got %q for %s", s.Kind, s.Qualname)
		}
		if s.Name != "handleRequest" {
			t.Errorf("expected Name to be the enclosing function's name, got %q", s.Name)
		}
	}
	if sigs[0].Key() == sigs[1].Key() {
		t.Fatalf("both arms produced the same Key() — collision: %s", sigs[0].Key())
	}

	for _, s := range sigs {
		s.Prepare()
	}
	susps := Rank(sigs, sigs, SizeGate{}, 0.1, 10, true)
	if len(susps) == 0 {
		t.Fatal("expected Rank to surface the duplicated arms as a candidate pair")
	}
	if susps[0].Score < 0.5 {
		t.Errorf("expected a high score for near-identical arms, got %.2f", susps[0].Score)
	}
}

// TestExtractGoBranchesSkipsEmptyArms: an empty else block has nothing to
// compare and should not become a fragment.
func TestExtractGoBranchesSkipsEmptyArms(t *testing.T) {
	src := `package p

func maybeLog(ok bool) {
	if ok {
		logEvent("ok")
	} else {
	}
}
`
	sigs := writeBranchFixture(t, src)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 branch fragment (empty else skipped), got %d", len(sigs))
	}
}

// TestExtractGoBranchesDoesNotDoubleCountNestedConditionals: a conditional
// nested inside an already-extracted arm must not ALSO become its own
// top-level fragment — it contributes to the outer arm's bag only, bounding
// the fragment count to the number of TOP-LEVEL conditionals, not exponential
// in nesting depth.
func TestExtractGoBranchesDoesNotDoubleCountNestedConditionals(t *testing.T) {
	src := `package p

func outer(a, b bool) string {
	if a {
		if b {
			return "inner_true"
		} else {
			return "inner_false"
		}
	} else {
		return "outer_false"
	}
}
`
	sigs := writeBranchFixture(t, src)
	// Top-level: the outer if's Body and Else — exactly 2. The nested if/else
	// inside the outer Body arm must NOT also appear as separate fragments.
	if len(sigs) != 2 {
		t.Fatalf("expected exactly 2 top-level fragments (outer if/else only), got %d: %v", len(sigs), qualnames(sigs))
	}
}

// TestExtractGoBranchesSwitchCases: each non-empty case body becomes its own
// fragment, keyed uniquely.
func TestExtractGoBranchesSwitchCases(t *testing.T) {
	src := `package p

func route(kind string) string {
	switch kind {
	case "a":
		logEvent("route_hit")
		return buildResponse("route_a")
	case "b":
		logEvent("route_hit")
		return buildResponse("route_b")
	default:
	}
	return ""
}
`
	sigs := writeBranchFixture(t, src)
	if len(sigs) != 2 {
		t.Fatalf("expected 2 fragments (case \"a\", case \"b\"; empty default skipped), got %d: %v", len(sigs), qualnames(sigs))
	}
	seen := map[string]bool{}
	for _, s := range sigs {
		if seen[s.Key()] {
			t.Fatalf("duplicate Key() across case fragments: %s", s.Key())
		}
		seen[s.Key()] = true
	}
}

func qualnames(sigs []*FuncSig) []string {
	out := make([]string, len(sigs))
	for i, s := range sigs {
		out[i] = s.Qualname
	}
	return out
}
