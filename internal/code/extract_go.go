package code

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"

	"github.com/justinstimatze/calque/internal/corpus"
)

// ExtractGoFile parses one Go file and returns a FuncSig per top-level
// function/method. Tolerant: returns nil on a parse error (skips the file),
// matching the Python extractor's SyntaxError behavior. In-process via go/ast —
// no external dependency (the dependency-free code-axis increment).
func ExtractGoFile(path, root string) []*FuncSig {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil
	}
	rel := corpus.RelPath(root, path)
	var out []*FuncSig
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		name := fd.Name.Name
		qual := name
		if fd.Recv != nil && len(fd.Recv.List) > 0 {
			if rt := recvTypeName(fd.Recv.List[0].Type); rt != "" {
				qual = rt + "." + name
			}
		}
		bv := &goBody{strs: set{}, writes: set{}, retKeys: set{}, calls: set{}}
		ast.Walk(bv, fd.Body)
		start := fset.Position(fd.Pos()).Line
		end := fset.Position(fd.End()).Line
		fs := &FuncSig{
			File: rel, Qualname: qual, Name: name,
			Line: start, NLines: end - start + 1,
			Strings: bv.strs.slice(), Writes: bv.writes.slice(),
			RetKeys: bv.retKeys.slice(), Calls: bv.calls.slice(),
			Delegates: bv.delegates,
		}
		fs.Prepare()
		out = append(out, fs)
	}
	return out
}

type goBody struct {
	strs, writes, retKeys, calls set
	delegates                    bool
}

func (b *goBody) Visit(n ast.Node) ast.Visitor {
	switch t := n.(type) {
	case *ast.BasicLit:
		if t.Kind == token.STRING {
			if v, err := strconv.Unquote(t.Value); err == nil {
				if v = strings.TrimSpace(v); len(v) >= 4 {
					b.strs[v] = struct{}{}
				}
			}
		}
	case *ast.CallExpr:
		switch fn := t.Fun.(type) {
		case *ast.Ident:
			b.calls[fn.Name] = struct{}{}
		case *ast.SelectorExpr:
			b.calls[fn.Sel.Name] = struct{}{}
			if p := attrPath(fn); p != "" {
				if root, _, ok := strings.Cut(p, "."); ok && IsDelegationRoot(root) {
					b.delegates = true
				}
			}
		}
	case *ast.AssignStmt:
		for _, lhs := range t.Lhs {
			b.recordTarget(lhs)
		}
	case *ast.IncDecStmt:
		b.recordTarget(t.X)
	case *ast.ReturnStmt:
		for _, r := range t.Results {
			if cl, ok := r.(*ast.CompositeLit); ok {
				for _, el := range cl.Elts {
					if kv, ok := el.(*ast.KeyValueExpr); ok {
						if k := litKey(kv.Key); k != "" {
							b.retKeys[k] = struct{}{}
						}
					}
				}
			}
		}
	}
	return b
}

func (b *goBody) recordTarget(e ast.Expr) {
	switch t := e.(type) {
	case *ast.SelectorExpr:
		if p := attrPath(t); p != "" {
			b.writes[p] = struct{}{}
		}
	case *ast.IndexExpr:
		if p := attrPath(t.X); p != "" {
			b.writes[p+"[]"] = struct{}{}
		}
	}
}

// attrPath returns the dotted attribute suffix of a selector chain, dropping the
// root identifier: p.field.sub -> "field.sub", x.y -> "y". Empty if the base of
// the chain is not a plain identifier (mirrors legacy/core.py _attr_path).
func attrPath(e ast.Expr) string {
	var parts []string
	cur := e
	for {
		sel, ok := cur.(*ast.SelectorExpr)
		if !ok {
			break
		}
		parts = append(parts, sel.Sel.Name)
		cur = sel.X
	}
	if _, ok := cur.(*ast.Ident); !ok {
		return ""
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, ".")
}

// litKey returns a struct field name (Ident) or map string key (string lit) of a
// composite-literal element key, or "" for non-keyed/other.
func litKey(e ast.Expr) string {
	switch k := e.(type) {
	case *ast.Ident:
		return k.Name
	case *ast.BasicLit:
		if k.Kind == token.STRING {
			if v, err := strconv.Unquote(k.Value); err == nil {
				return v
			}
		}
	}
	return ""
}

// recvTypeName extracts the receiver type name from a method receiver:
// *T -> T, T -> T, generic T[P] -> T.
func recvTypeName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return recvTypeName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr: // T[P]
		return recvTypeName(t.X)
	case *ast.IndexListExpr: // T[P, Q]
		return recvTypeName(t.X)
	}
	return ""
}
