package code

import "testing"

func TestSignatureInformative(t *testing.T) {
	cases := []struct {
		sig  string
		want bool
	}{
		{"(Task[])=>Promise<Task[]>", true},                  // domain type Task
		{"(string)=>WorktreeInfo|null", true},                // domain type WorktreeInfo
		{"()=>Promise<string[]>", false},                     // only the Promise generic + primitive
		{"(string)=>Promise<boolean>", false},                // primitives + generic
		{"(string,number)=>boolean", false},                  // pure primitives
		{"()=>?", false},                                     // untyped
		{"", false},                                          // empty
		{"(Record<string,unknown>)=>void", false},            // only stdlib generics + primitives
		{"(ModelTier)=>ModelTier", true},                     // domain type ModelTier
	}
	for _, c := range cases {
		if got := signatureInformative(c.sig); got != c.want {
			t.Errorf("signatureInformative(%q) = %v, want %v", c.sig, got, c.want)
		}
	}
}

func TestOpposedVerbs(t *testing.T) {
	if !opposed("insertTask", "updateTask") {
		t.Error("insert/update should be opposed")
	}
	if !opposed("addItem", "removeItem") {
		t.Error("add/remove should be opposed")
	}
	if !opposed("taskStart", "taskComplete") {
		t.Error("start/complete should be opposed")
	}
	if opposed("getWorktreeForSession", "getWorktreeInfo") {
		t.Error("get/get must NOT be opposed (both reads — a real twin shape)")
	}
	if opposed("findDecisions", "searchDecisions") {
		t.Error("find/search are synonyms, not opposed")
	}
}

func mkSig(file, qual, name, sig string, nlines int) *FuncSig {
	f := &FuncSig{File: file, Qualname: qual, Name: name, Sig: sig, NLines: nlines}
	f.Prepare()
	return f
}

func TestSignatureCandidates(t *testing.T) {
	sigs := []*FuncSig{
		// A real domain-typed twin shape across two files (no shared tokens).
		mkSig("a.ts", "getWorktreeForSession", "getWorktreeForSession", "(string)=>WorktreeInfo|null", 6),
		mkSig("b.ts", "getWorktreeInfo", "getWorktreeInfo", "(string)=>WorktreeInfo|null", 8),
		// An opposed CRUD pair sharing a domain sig — must be filtered out.
		mkSig("c.ts", "insertTask", "insertTask", "(Task)=>void", 5),
		mkSig("c.ts", "deleteTask", "deleteTask", "(Task)=>void", 5),
		// A primitive-only sig — never a candidate.
		mkSig("d.ts", "toUpper", "toUpper", "(string)=>string", 5),
		mkSig("d.ts", "trim", "trim", "(string)=>string", 5),
		// Too short — filtered by minLines.
		mkSig("e.ts", "tiny", "tiny", "(Task)=>void", 2),
	}
	cands := SignatureCandidates(sigs, 4, 2, 6)

	// Exactly one candidate: the worktree pair. (opposed CRUD filtered; primitive sig
	// not informative; the void/Task group has only the tiny short one left after
	// filtering insert/delete out, so no pair.)
	if len(cands) != 1 {
		t.Fatalf("expected 1 candidate, got %d: %+v", len(cands), cands)
	}
	c := cands[0]
	if c.A.Name != "getWorktreeForSession" && c.B.Name != "getWorktreeForSession" {
		t.Errorf("expected the worktree twin, got %s ≟ %s", c.A.Name, c.B.Name)
	}
	if !c.CrossFile {
		t.Error("worktree twin is cross-file")
	}
}

// A signature group larger than maxMembers is a common shape, not a twin — skipped.
func TestSignatureCandidatesRarityWindow(t *testing.T) {
	var sigs []*FuncSig
	for i := 0; i < 8; i++ {
		sigs = append(sigs, mkSig("f.ts", string(rune('a'+i)), string(rune('a'+i)), "(Foo)=>Bar", 5))
	}
	if got := SignatureCandidates(sigs, 4, 2, 6); len(got) != 0 {
		t.Errorf("group of 8 exceeds maxMembers=6, expected 0 candidates, got %d", len(got))
	}
}
