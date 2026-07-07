package code

import "testing"

func TestSignatureInformative(t *testing.T) {
	cases := []struct {
		sig  string
		want bool
	}{
		{"(Task[])=>Promise<Task[]>", true},       // domain type Task
		{"(string)=>WorktreeInfo|null", true},     // domain type WorktreeInfo
		{"()=>Promise<string[]>", false},          // only the Promise generic + primitive
		{"(string)=>Promise<boolean>", false},     // primitives + generic
		{"(string,number)=>boolean", false},       // pure primitives
		{"()=>?", false},                          // untyped
		{"", false},                               // empty
		{"(Record<string,unknown>)=>void", false}, // only stdlib generics + primitives
		{"(ModelTier)=>ModelTier", true},          // domain type ModelTier
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
	cands := SignatureCandidates(sigs, SizeGate{MinLines: 4}, 2, 6)

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

func TestNameStemCandidates(t *testing.T) {
	sigs := []*FuncSig{
		// Near-identical role names (token sets equal modulo order) — should pair.
		mkSig("a.ts", "formatRemainingTime", "formatRemainingTime", "()=>string", 6),
		mkSig("a.ts", "formatTimeRemaining", "formatTimeRemaining", "(number)=>string", 6),
		// Unrelated name — should not pair with either.
		mkSig("b.ts", "parseConfigFile", "parseConfigFile", "(string)=>Config", 6),
		// Shares only the common stem "get" with nothing rich — no pair.
		mkSig("c.ts", "getUser", "getUser", "()=>User", 6),
	}
	cands := NameStemCandidates(sigs, SizeGate{MinLines: 4}, 0.6, 8)
	if len(cands) != 1 {
		t.Fatalf("expected 1 name-stem candidate (the format pair), got %d: %+v", len(cands), cands)
	}
	c := cands[0]
	if c.Kind != "name-stem" {
		t.Errorf("Kind = %q, want name-stem", c.Kind)
	}
	got := map[string]bool{c.A.Name: true, c.B.Name: true}
	if !got["formatRemainingTime"] || !got["formatTimeRemaining"] {
		t.Errorf("expected the format pair, got %s ≟ %s", c.A.Name, c.B.Name)
	}
}

// A stem shared by more than maxFanout functions is plumbing, not a role — skipped.
func TestNameStemFanoutCap(t *testing.T) {
	var sigs []*FuncSig
	for i := 0; i < 10; i++ {
		// All share the stem "handle" but are otherwise distinct → high fanout.
		sigs = append(sigs, mkSig("f.ts", "handle"+string(rune('A'+i)), "handle"+string(rune('A'+i)), "()=>void", 6))
	}
	if got := NameStemCandidates(sigs, SizeGate{MinLines: 4}, 0.6, 8); len(got) != 0 {
		t.Errorf("stem 'handle' shared by 10 funcs exceeds fanout cap 8, expected 0, got %d", len(got))
	}
}

func mkTable(file, name string, keys ...string) *FuncSig {
	f := &FuncSig{File: file, Qualname: name, Name: name, Kind: "table", RetKeys: keys, NLines: 1}
	f.Prepare()
	return f
}

// The headline cross-substrate pair: two registries of the same verbs in different
// files, sharing no surface tokens the jaccard gate could anchor on.
func TestKeySetCandidates(t *testing.T) {
	ents := []*FuncSig{
		mkTable("engine.py", "HANDLERS", "look", "go", "take", "drop", "give"),
		mkTable("input_agent.py", "_VERB_TEMPLATES", "look", "go", "take", "drop"),
		mkTable("db.py", "COLUMNS", "source_era", "description", "trigger"), // disjoint
	}
	cands := KeySetCandidates(ents, 3, 0.5, 8)
	if len(cands) != 1 {
		t.Fatalf("expected 1 key-set candidate (HANDLERS≟_VERB_TEMPLATES), got %d: %+v", len(cands), cands)
	}
	c := cands[0]
	if c.Kind != "key-set" {
		t.Errorf("Kind = %q, want key-set", c.Kind)
	}
	got := map[string]bool{c.A.Name: true, c.B.Name: true}
	if !got["HANDLERS"] || !got["_VERB_TEMPLATES"] {
		t.Errorf("expected HANDLERS≟_VERB_TEMPLATES, got %s ≟ %s", c.A.Name, c.B.Name)
	}
	if !c.CrossFile {
		t.Error("the pair is cross-file")
	}
	if c.Jaccard < 0.79 || c.Jaccard > 0.81 { // 4 shared / 5 union
		t.Errorf("key-set jaccard = %.3f, want ~0.80", c.Jaccard)
	}
}

// minKeys filters thin entities; a 2-key table below minKeys=3 never indexes.
func TestKeySetCandidatesMinKeys(t *testing.T) {
	ents := []*FuncSig{
		mkTable("a.py", "A", "x", "y"),
		mkTable("b.py", "B", "x", "y"),
	}
	if got := KeySetCandidates(ents, 3, 0.5, 8); len(got) != 0 {
		t.Errorf("2-key tables below minKeys=3 should yield 0, got %d", len(got))
	}
}

// A key shared by more than maxFanout entities is plumbing, not a shape — its whole
// posting list is dropped as a JOIN path.
func TestKeySetCandidatesFanoutCap(t *testing.T) {
	var ents []*FuncSig
	for i := 0; i < 10; i++ {
		ents = append(ents, mkTable("f.py", "T"+string(rune('A'+i)), "id", "name", "kind"))
	}
	if got := KeySetCandidates(ents, 3, 0.5, 8); len(got) != 0 {
		t.Errorf("key shared by 10 entities exceeds fanout cap 8, expected 0, got %d", len(got))
	}
}

// A signature group larger than maxMembers is a common shape, not a twin — skipped.
func TestSignatureCandidatesRarityWindow(t *testing.T) {
	var sigs []*FuncSig
	for i := 0; i < 8; i++ {
		sigs = append(sigs, mkSig("f.ts", string(rune('a'+i)), string(rune('a'+i)), "(Foo)=>Bar", 5))
	}
	if got := SignatureCandidates(sigs, SizeGate{MinLines: 4}, 2, 6); len(got) != 0 {
		t.Errorf("group of 8 exceeds maxMembers=6, expected 0 candidates, got %d", len(got))
	}
}
