package code

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
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

	// File-scope declared SCREAMING_SNAKE constants — the const analog of the
	// project-defined call names the touchpoint pass keys on. Go's MixedCaps
	// convention yields few of these, but a project that does name a domain magic
	// value in SCREAMING_SNAKE (const GRID = 60) is admitted to the const seam
	// channel; everything else (std/library refs) is gated out.
	declConsts := goDeclConsts(f)

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
		bv := &goBody{strs: set{}, writes: set{}, retKeys: set{}, calls: set{}, consts: set{}, readsRaw: set{}, pureWrites: set{}, calleeSkip: map[ast.Expr]bool{}}
		ast.Walk(bv, fd.Body)
		start := fset.Position(fd.Pos()).Line
		end := fset.Position(fd.End()).Line
		fs := &FuncSig{
			File: rel, Qualname: qual, Name: name,
			Line: start, NLines: end - start + 1, NodeCount: bv.nodes,
			Strings: bv.strs.slice(), Writes: bv.writes.slice(),
			Reads:   bv.reads(),
			RetKeys: bv.retKeys.slice(), Calls: bv.calls.slice(),
			Consts:     bv.consts.slice(),
			DeclConsts: declConsts,
			Delegates:  bv.delegates,
		}
		out = append(out, fs)
	}
	return out
}

// goDeclConsts returns the SCREAMING_SNAKE constants declared at file scope in a
// parsed Go file (top-level const/var blocks). These are the project-DECLARED
// domain constants the touchpoint pass gates the const seam channel on.
func goDeclConsts(f *ast.File) []string {
	names := set{}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || (gd.Tok != token.CONST && gd.Tok != token.VAR) {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, name := range vs.Names {
				if isDomainConst(name.Name) {
					names[name.Name] = struct{}{}
				}
			}
		}
	}
	if len(names) == 0 {
		return nil
	}
	out := names.slice()
	sort.Strings(out)
	return out
}

// goBatch runs a per-file go/ast extractor over every path in-process, tolerating
// per-file parse errors (the file is skipped). Single-sources the path loop shared
// by the function and table extractors.
func goBatch(paths []string, root string, perFile func(path, root string) []*FuncSig) ([]*FuncSig, error) {
	var all []*FuncSig
	for _, p := range paths {
		all = append(all, perFile(p, root)...)
	}
	return all, nil
}

// extractGoBatch extracts FUNCTIONS from .go paths (the code axis).
func extractGoBatch(paths []string, root string) ([]*FuncSig, error) {
	return goBatch(paths, root, ExtractGoFile)
}

// extractGoSymbols extracts package-level map/slice/array CONSTANTS as "table"
// entities — the Go analogue of the Python `symbols` mode. A `var HANDLERS =
// map[Verb]Handler{…}` or `var verbs = []string{…}` becomes a table whose keys (or
// elements) are its RetKeys, so a Go port's registries pair against corpus shapes /
// SQL schemas the same way Python tables do. In-process via go/ast; this is the
// cross-substrate axis's Go table source, consumed only by propose-cross.
func extractGoSymbols(paths []string, root string) ([]*FuncSig, error) {
	return goBatch(paths, root, extractGoSymbolsFile)
}

func extractGoSymbolsFile(path, root string) []*FuncSig {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil
	}
	rel := corpus.RelPath(root, path)
	var out []*FuncSig
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || (gd.Tok != token.VAR && gd.Tok != token.CONST) {
			continue // only top-level var/const declarations
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vs.Names {
				if i >= len(vs.Values) {
					continue
				}
				cl, ok := vs.Values[i].(*ast.CompositeLit)
				if !ok {
					continue // not a literal table (struct/call/etc.)
				}
				keys, vals := goLiteralKeys(cl)
				if len(keys) == 0 {
					continue
				}
				// Noise control: an exported (package-public) named table, or any
				// literal with enough keys to be a real registry.
				if !(isExportedName(name.Name) || len(keys) >= 3) {
					continue
				}
				start := fset.Position(vs.Pos()).Line
				end := fset.Position(vs.End()).Line
				out = append(out, &FuncSig{
					File: rel, Qualname: name.Name, Name: name.Name, Kind: "table",
					Line: start, NLines: end - start + 1,
					Strings: vals, RetKeys: keys,
				})
			}
		}
	}
	return out
}

// goLiteralKeys returns the string keys (map literal) or string elements (slice/
// array literal) of a composite literal, plus string values where present.
func goLiteralKeys(cl *ast.CompositeLit) (keys, vals []string) {
	switch cl.Type.(type) {
	case *ast.MapType:
		for _, el := range cl.Elts {
			kv, ok := el.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if k := litKey(kv.Key); k != "" {
				keys = append(keys, k)
				if v := goStringLit(kv.Value); v != "" {
					vals = append(vals, v)
				}
			}
		}
	case *ast.ArrayType:
		for _, el := range cl.Elts {
			if v := goStringLit(el); v != "" {
				keys = append(keys, v)
			}
		}
	}
	keys = dedupStrings(keys)
	sort.Strings(keys)
	vals = dedupStrings(vals)
	sort.Strings(vals)
	return keys, vals
}

func goStringLit(e ast.Expr) string {
	bl, ok := e.(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return ""
	}
	if v, err := strconv.Unquote(bl.Value); err == nil {
		return v
	}
	return ""
}

// isExportedName reports whether a Go identifier is package-exported (Capitalized).
func isExportedName(name string) bool {
	return name != "" && name[0] >= 'A' && name[0] <= 'Z'
}

type goBody struct {
	strs, writes, retKeys, calls, consts set
	// readsRaw is every dotted field-path seen in a value position; pureWrites is
	// the LHS of plain `=` assignments. reads() returns readsRaw \ pureWrites (the
	// derivation input footprint), mirroring writes on the read side.
	readsRaw, pureWrites set
	// calleeSkip holds selector nodes that are a call's callee (exec.LookPath), so
	// the read pass skips them — a call name is not a field read.
	calleeSkip map[ast.Expr]bool
	delegates  bool
	// nodes counts every AST node visited in the body — see Visit's n == nil guard
	// for why this isn't a plain unconditional increment.
	nodes int
}

// reads returns the derivation read-set: field-paths read in a value position,
// excluding those that appear only as a plain-`=` assignment target.
func (b *goBody) reads() []string {
	out := make([]string, 0, len(b.readsRaw))
	for r := range b.readsRaw {
		if !b.pureWrites.has(r) {
			out = append(out, r)
		}
	}
	return out
}

func (b *goBody) Visit(n ast.Node) ast.Visitor {
	// ast.Walk calls Visit(nil) once after a node's children finish — guard it so
	// the node count isn't roughly doubled (one real increment per node, one
	// nil-sentinel call per node's children completing).
	if n == nil {
		return b
	}
	b.nodes++
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
			// The callee selector itself (exec.LookPath, fmt.Errorf) is a CALL, not a
			// field read — skip it for reads so the leaf name doesn't pollute the
			// derivation footprint. Its receiver (fn.X) is a separate node still
			// visited, so a domain-object method call (self.road.compute()) still
			// contributes the receiver field "road".
			b.calleeSkip[fn] = true
			if p := attrPath(fn); p != "" {
				if root, _, ok := strings.Cut(p, "."); ok && IsDelegationRoot(root) {
					b.delegates = true
				}
			}
		}
	case *ast.SelectorExpr:
		// Every field-path read in a value position. The plain-`=` LHS is removed
		// later via pureWrites (reads()); compound-assign / inc-dec targets stay
		// (read-modify-write). A call's own callee selector is skipped (calleeSkip)
		// so stdlib/method call names don't masquerade as field reads.
		if b.calleeSkip[t] {
			break
		}
		if p := attrPath(t); p != "" {
			b.readsRaw[p] = struct{}{}
		}
	case *ast.Ident:
		// A referenced SCREAMING_SNAKE identifier is a domain constant (V_BELOW).
		// Go's MixedCaps const convention means this fires rarely on Go, but the
		// selector leaf of pkg.V_BELOW reaches this Ident node too, so cross-package
		// constants are still caught. Powers the const-set touchpoint channel.
		if isDomainConst(t.Name) {
			b.consts[t.Name] = struct{}{}
		}
	case *ast.AssignStmt:
		for _, lhs := range t.Lhs {
			b.recordTarget(lhs)
			if t.Tok == token.ASSIGN {
				if p := attrPath(lhs); p != "" {
					b.pureWrites[p] = struct{}{}
				}
			}
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
// the chain is not a plain identifier (mirrors the original _attr_path).
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
