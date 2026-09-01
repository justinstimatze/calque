package code

import (
	"os"
	"path/filepath"
	"testing"
)

// hasShape reports whether fs recorded tag among the call-result shapes
// observed for a call site into leaf.
func hasShape(fs *FuncSig, leaf, tag string) bool {
	for _, t := range fs.CallResultShapes[leaf] {
		if t == tag {
			return true
		}
	}
	return false
}

// TestExtractGoCallResultShapes pins every tag in the call-site context axis's
// vocabulary (SPEC-callsite-context-axis.md §2) against one fixture per shape.
// This is the extraction half only — no recall pass reads this field yet.
func TestExtractGoCallResultShapes(t *testing.T) {
	dir := t.TempDir()
	src := `package p

func DirectNilCheck() {
	if lookup(1, 2) != nil {
	}
}

func IfInitErrCheck() error {
	if err := doWork(); err != nil {
		return err
	}
	return nil
}

func TwoStmtErrCheck() error {
	v, err := fetch()
	if err != nil {
		return err
	}
	_ = v
	return nil
}

func TwoStmtNilCheck() {
	x := findThing()
	if x != nil {
	}
}

func PassedToCall() {
	consume(transform())
}

func Passthrough() int {
	return compute()
}

func AssignField() {
	obj.Field = derive()
	m[key] = derive2()
}

func Compared() {
	if count() > 5 {
	}
}
`
	f := filepath.Join(dir, "callctx.go")
	if err := os.WriteFile(f, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	sigs, err := extractGoBatch([]string{f}, dir)
	if err != nil {
		t.Fatalf("extractGoBatch: %v", err)
	}

	cases := []struct {
		fn, leaf, tag string
	}{
		{"DirectNilCheck", "lookup", "ret-nil-checked"},
		{"DirectNilCheck", "lookup", "arg-count:2"},
		{"IfInitErrCheck", "doWork", "ret-err-checked"},
		{"TwoStmtErrCheck", "fetch", "ret-err-checked"},
		{"TwoStmtNilCheck", "findThing", "ret-nil-checked"},
		{"PassedToCall", "transform", "ret-passed-to-call"},
		{"Passthrough", "compute", "ret-returned"},
		{"AssignField", "derive", "ret-assigned-field"},
		{"AssignField", "derive2", "ret-assigned-field"},
		{"Compared", "count", "ret-compared"},
	}
	for _, c := range cases {
		fs := sigByQual(sigs, c.fn)
		if fs == nil {
			t.Fatalf("%s not extracted; got %d sigs", c.fn, len(sigs))
		}
		if !hasShape(fs, c.leaf, c.tag) {
			t.Errorf("%s: callee %q missing tag %q; got %v", c.fn, c.leaf, c.tag, fs.CallResultShapes[c.leaf])
		}
	}

	// TwoStmtNilCheck's checked var isn't named "err" — must NOT also fire
	// ret-err-checked, or the down-weight this tag exists for (§5) is useless.
	if fs := sigByQual(sigs, "TwoStmtNilCheck"); hasShape(fs, "findThing", "ret-err-checked") {
		t.Error("TwoStmtNilCheck: findThing wrongly tagged ret-err-checked (checked var is \"x\", not \"err\")")
	}
	// AssignField's second call site must not spuriously pick up a nil/err-check
	// tag just because the next line happens to not be an IfStmt at all.
	if fs := sigByQual(sigs, "AssignField"); hasShape(fs, "derive", "ret-nil-checked") || hasShape(fs, "derive", "ret-err-checked") {
		t.Errorf("AssignField: derive wrongly tagged a nil/err-check shape; got %v", fs.CallResultShapes["derive"])
	}
}
