package code

import "testing"

func roleSig(file, qual, name string, calls, writes, strs, ret []string, delegates bool) *FuncSig {
	f := &FuncSig{File: file, Qualname: qual, Name: name, Calls: calls, Writes: writes, Strings: strs, RetKeys: ret, Delegates: delegates}
	f.Prepare()
	return f
}

func TestPredicateMatchTerms(t *testing.T) {
	f := roleSig("input_agent.py", "construct", "construct",
		[]string{"_dispatch", "validate"}, []string{"self.canon"}, []string{"refuse"}, []string{"ok"}, false)

	cases := []struct {
		pred string
		want bool
	}{
		{"name:/construct/", true},
		{"name:/^construct$/", true},
		{"name:/Construct/", false}, // RE2 is case-sensitive by default
		{"name:/(?i)Construct/", true},
		{"qual:/construct/", true},
		{"file:input_agent.py", true},
		{"file:*.py", true},
		{"file:*.go", false},
		{"calls:_dispatch", true},
		{"calls:missing", false},
		{"writes:self.canon", true},
		{"emits:refuse", true},
		{"returns:ok", true},
		{"name:/construct/ calls:_dispatch", true}, // AND, both hold
		{"name:/construct/ calls:missing", false},  // AND, one fails
	}
	for _, c := range cases {
		got, err := Match(c.pred, f)
		if err != nil {
			t.Errorf("Match(%q): unexpected error %v", c.pred, err)
			continue
		}
		if got != c.want {
			t.Errorf("Match(%q) = %v, want %v", c.pred, got, c.want)
		}
	}
}

// Implementers must exclude delegating wrappers: a method that forwards to the real
// implementation is not an independent implementation of the role (the §12 insight).
func TestImplementersExcludesDelegates(t *testing.T) {
	real := roleSig("e.go", "verdictClass", "verdictClass", nil, nil, nil, nil, false)
	wrap1 := roleSig("e.go", "Entry.VerdictClass", "VerdictClass", []string{"verdictClass"}, nil, nil, nil, true)
	wrap2 := roleSig("e.go", "ClusterEntry.VerdictClass", "VerdictClass", []string{"verdictClass"}, nil, nil, nil, true)
	sigs := []*FuncSig{real, wrap1, wrap2}

	impls, err := Implementers("name:/(?i)verdictclass/", sigs)
	if err != nil {
		t.Fatal(err)
	}
	if len(impls) != 1 || impls[0] != real {
		got := make([]string, len(impls))
		for i, f := range impls {
			got[i] = f.Key()
		}
		t.Fatalf("delegation exclusion: want [verdictClass], got %v", got)
	}
}

func TestParsePredicateErrors(t *testing.T) {
	for _, bad := range []string{"", "noColon", "bad:", ":value", "name:/[/", "huh:x"} {
		if _, err := ParsePredicate(bad); err == nil {
			t.Errorf("expected an error for %q", bad)
		}
	}
}
