package code

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"

	"github.com/justinstimatze/calque/internal/corpus"
)

// valueExtractors is the (currently Go-only) per-extension dispatch for the
// scattered-value axis (propose-values). Kept separate from Extract's
// function-only extractors and from symbolExtractors so the function-only
// scoring gate (Extract -> Rank -> check -> --strict) never sees
// Kind:"value-site" entities.
var valueExtractors = map[string]func(paths []string, root string) ([]*FuncSig, error){
	".go": extractGoValuesBatch,
}

// ExtractValueSites walks repo and extracts literal-value occurrences bound
// to a nearby identifier (an assignment target, a var/const declaration, or a
// composite-literal field/map key) as Kind:"value-site" FuncSig-like
// records — the scattered-constant axis's corpus (propose-values): a
// `maxRetries`-style value repeated across independent sites with no shared
// symbol backing it. Mirrors ExtractBranches/ExtractSymbols's shape: a
// structurally-separate extraction entry point from the function-only
// Extract. Go-only this pass (see docs/DESIGN_NOTES.md §21).
func ExtractValueSites(repo string, exclude []string) ([]*FuncSig, ScanStats, error) {
	st := ScanStats{SkippedExts: map[string]int{}}
	byExt, err := walkExtractable(repo, exclude,
		func(ext string) bool { _, ok := valueExtractors[ext]; return ok }, nil)
	if err != nil {
		return nil, st, err
	}
	var all []*FuncSig
	for ext, paths := range byExt {
		sigs, exErr := valueExtractors[ext](paths, repo)
		if exErr != nil {
			return nil, st, fmt.Errorf("%s value-site extractor: %w", ext, exErr)
		}
		prepareSigs(sigs)
		all = append(all, sigs...)
		st.Files += len(paths)
		st.Funcs += len(sigs)
	}
	return all, st, nil
}

func extractGoValuesBatch(paths []string, root string) ([]*FuncSig, error) {
	return goBatch(paths, root, extractGoValuesFile)
}

// extractGoValuesFile parses one Go file and returns a Kind:"value-site"
// FuncSig per non-trivial literal bound to a nearby identifier, at both
// package scope (var/const declarations) and function-local scope
// (assignments, composite-literal fields/map keys inside a function body).
//
// Deliberately NOT handled this pass: a bare literal passed as a call
// argument (`retryWithBackoff(3, "op")`). Binding it to the callee's
// PARAMETER name would need type-checking (go/types) to resolve the
// callee's signature — a real dependency this AST-only, zero-type-info
// extractor doesn't carry anywhere else. Left as a follow-up if the
// assignment/declaration/composite-literal channel alone proves insufficient
// recall during calibration, not silently dropped.
func extractGoValuesFile(path, root string) []*FuncSig {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil
	}
	rel := corpus.RelPath(root, path)

	vf := &valueFinder{fset: fset, file: rel}
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Body == nil {
				continue
			}
			vf.name = d.Name.Name
			vf.qual = qualNameFor(vf.name, d.Recv)
			ast.Walk(vf, d.Body)
		case *ast.GenDecl:
			vf.name, vf.qual = "", "" // package scope — no enclosing function
			ast.Walk(vf, d)
		}
	}
	return vf.sites
}

// valueFinder walks a scope (a function body, or a package-level var/const
// declaration) collecting literal-value bindings. qual/name track the
// CURRENT enclosing function ("" at package scope); a literal found inside a
// nested closure is attributed to the outer enclosing function, the same
// simplification ExtractBranches's branchFinder makes.
type valueFinder struct {
	fset       *token.FileSet
	file       string
	qual, name string
	n          int // running per-file counter, keeps Key() collision-proof
	sites      []*FuncSig
}

func (vf *valueFinder) Visit(n ast.Node) ast.Visitor {
	switch t := n.(type) {
	case *ast.AssignStmt:
		vf.fromAssign(t)
	case *ast.ValueSpec:
		vf.fromValueSpec(t)
	case *ast.KeyValueExpr:
		vf.fromKeyValue(t)
	}
	return vf
}

// fromAssign handles `name := literal` / `name = literal` (and the
// multi-assign form `a, b := 1, 2`, matched positionally). A mismatched
// count (`a, err := f()`) has no positional literal to bind, so it's skipped.
func (vf *valueFinder) fromAssign(a *ast.AssignStmt) {
	if len(a.Lhs) != len(a.Rhs) {
		return
	}
	for i, lhs := range a.Lhs {
		id, ok := lhs.(*ast.Ident)
		if !ok || id.Name == "_" {
			continue
		}
		if v, ok := literalValue(a.Rhs[i]); ok {
			vf.add(id.Name, v, a.Rhs[i].Pos())
		}
	}
}

// fromValueSpec handles `var name = literal` / `const name = literal`
// (top-level or function-local), matched positionally like fromAssign.
func (vf *valueFinder) fromValueSpec(vs *ast.ValueSpec) {
	if len(vs.Names) != len(vs.Values) {
		return
	}
	for i, id := range vs.Names {
		if id.Name == "_" {
			continue
		}
		if v, ok := literalValue(vs.Values[i]); ok {
			vf.add(id.Name, v, vs.Values[i].Pos())
		}
	}
}

// fromKeyValue handles a composite-literal field or map key bound to a
// literal value: `Config{MaxRetries: 3}` (Key is an Ident, the field name)
// or `map[string]int{"maxRetries": 3}` (Key is a string BasicLit).
func (vf *valueFinder) fromKeyValue(kv *ast.KeyValueExpr) {
	name := ""
	switch k := kv.Key.(type) {
	case *ast.Ident:
		name = k.Name
	case *ast.BasicLit:
		if k.Kind == token.STRING {
			if s, err := strconv.Unquote(k.Value); err == nil {
				name = s
			}
		}
	}
	if name == "" {
		return
	}
	if v, ok := literalValue(kv.Value); ok {
		vf.add(name, v, kv.Value.Pos())
	}
}

func (vf *valueFinder) add(name, value string, pos token.Pos) {
	vf.n++
	line := vf.fset.Position(pos).Line
	qual := name
	if vf.qual != "" {
		qual = vf.qual + "." + name
	}
	vf.sites = append(vf.sites, &FuncSig{
		File: vf.file, Name: name, Value: value, Kind: "value-site",
		Qualname: fmt.Sprintf("%s#value@%d.%d", qual, line, vf.n),
		Line:     line, NLines: 1,
	})
}

// literalValue returns a literal expression's canonical string value, and
// false for anything that isn't a literal or is a trivial one not worth
// tracking (0, 1, -1, true, false, "") — the same exclude-list standard
// magic-number linters use, keeping the corpus from drowning in noise.
func literalValue(e ast.Expr) (string, bool) {
	switch t := e.(type) {
	case *ast.BasicLit:
		return basicLitValue(t)
	case *ast.UnaryExpr:
		return unaryLiteralValue(t)
	}
	return "", false
}

// basicLitValue handles literalValue's *ast.BasicLit case: an int/float
// literal (excluding the trivial 0/1), or a non-empty string literal.
func basicLitValue(t *ast.BasicLit) (string, bool) {
	switch t.Kind {
	case token.INT, token.FLOAT:
		if t.Value == "0" || t.Value == "1" {
			return "", false
		}
		return t.Value, true
	case token.STRING:
		s, err := strconv.Unquote(t.Value)
		if err != nil || s == "" {
			return "", false
		}
		return s, true
	}
	return "", false
}

// unaryLiteralValue handles literalValue's *ast.UnaryExpr case: only a
// negated literal (-1, -3, ...) counts; any other unary op is not a literal.
func unaryLiteralValue(t *ast.UnaryExpr) (string, bool) {
	if t.Op != token.SUB {
		return "", false
	}
	v, ok := literalValue(t.X)
	if !ok {
		return "", false
	}
	return "-" + v, true
}
