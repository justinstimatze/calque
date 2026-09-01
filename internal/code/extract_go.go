// Package code is calque's code axis: per-language extractors produce a common
// FuncSig (the divergence-robust signature of a function), and a single
// language-agnostic scorer ranks cross-boundary pairs by how much they smell
// like the same contract — the recall half of a dual-path / behavioral-twin
// (Type-4) finder.
//
// Why these signals: a dual path (two impls of one contract that drifted) is
// dissimilar by construction in token/AST shape — that is the bug. So we index
// what stays invariant under a rewrite: emitted string literals (what it SAYS),
// attribute write-targets (what it MUTATES), returned keys (what it HANDS BACK),
// called leaf names (what it leans on), and the name stem (its ROLE).
//
// Ported from the original Python calque. The extraction half
// is per-language (go/ast here; an embedded python3 script for Python); this
// file is the shared types + scoring substrate.
package code

import (
	"bytes"
	_ "embed"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"

	"github.com/justinstimatze/calque/internal/corpus"
)

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
	imports := goImportMap(f)

	var out []*FuncSig
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		name := fd.Name.Name
		qual := qualNameFor(name, fd.Recv)
		fs := goFuncSigFromBody(fset, fd.Body, fragSite{rel, qual, name})
		fs.DeclConsts = declConsts
		fs.Sig = goSignatureOf(fset, imports, fd.Type)
		out = append(out, fs)
	}
	return out
}

// goFuncSigFromBody walks body with a fresh goBody accumulator and builds the
// FuncSig its signal bags describe. Shared by ExtractGoFile (whole function
// bodies) and ExtractBranches (sub-tree branch-arm bodies) — the walk-then-build
// step is identical, only the subtree, Kind, and key differ per caller.
func goFuncSigFromBody(fset *token.FileSet, body ast.Node, site fragSite) *FuncSig {
	bv := &goBody{strs: set{}, writes: set{}, retKeys: set{}, calls: set{}, consts: set{}, readsRaw: set{}, pureWrites: set{}, calleeSkip: map[ast.Expr]bool{}}
	ast.Walk(bv, body)
	start := fset.Position(body.Pos()).Line
	end := fset.Position(body.End()).Line
	return &FuncSig{
		File: site.file, Qualname: site.qual, Name: site.name,
		Line: start, NLines: end - start + 1, NodeCount: bv.nodes,
		Strings: bv.strs.slice(), Writes: bv.writes.slice(),
		Reads:   bv.reads(),
		RetKeys: bv.retKeys.slice(), Calls: bv.calls.slice(),
		Consts:    bv.consts.slice(),
		Delegates: bv.delegates,
	}
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

// qualNameFor builds a function's Qualname the same way every Go extractor
// does: "Type.Method" for a method (via recvTypeName on the receiver), or
// bare name for a plain function. Shared by ExtractGoFile, extractGoBranchesFile,
// and extractGoValuesFile — three copies of this exact snippet is precisely
// the sub-function duplication propose-branches is designed to catch, and it
// caught its own author's.
func qualNameFor(name string, recv *ast.FieldList) string {
	if recv != nil && len(recv.List) > 0 {
		if rt := recvTypeName(recv.List[0].Type); rt != "" {
			return rt + "." + name
		}
	}
	return name
}

// goImportMap builds a local-identifier -> import-path map for a parsed file, so
// signature rendering (goTypeSig) can distinguish stdlib-qualified types (noise
// for the Type-4 signature-rarity axis) from same-module/third-party ones
// (signal). The local identifier is the import's alias if given, else the
// path's last segment (Go's own default-name convention).
func goImportMap(f *ast.File) map[string]string {
	m := make(map[string]string, len(f.Imports))
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		local := path[strings.LastIndex(path, "/")+1:]
		if imp.Name != nil {
			local = imp.Name.Name
		}
		m[local] = path
	}
	return m
}

// goStdlibPkg reports whether an import path is a standard-library package —
// every stdlib path's first slash-delimited segment has no dot (context,
// net/http, encoding/json); every module path does (github.com/..., golang.org/x/...).
// A filesystem/GOROOT-free heuristic, so this works against an arbitrary target
// repo without needing its build environment.
func goStdlibPkg(path string) bool {
	seg := path
	if i := strings.Index(path, "/"); i >= 0 {
		seg = path[:i]
	}
	return !strings.Contains(seg, ".")
}

// goTypeSig renders a type expression for the signature-rarity axis (Sig). A
// top-level (or pointer-to) selector resolving to a stdlib import renders as
// just the lowercase package name (context.Context -> context) instead of the
// qualified type — this fails signatureInformative's capitalized-type check
// with zero changes needed in sigcluster.go, and needs no stoplist maintenance
// as Go's stdlib grows. Any other type (same-module, third-party, unqualified)
// renders via go/format.Node, whitespace-collapsed exactly like extract_ts.mjs's
// typeText. Deeper nesting (a stdlib type inside a slice/map element) isn't
// unwrapped this pass — an accepted, scoped simplification.
func goTypeSig(fset *token.FileSet, imports map[string]string, expr ast.Expr) string {
	target := expr
	if star, ok := target.(*ast.StarExpr); ok {
		target = star.X
	}
	if sel, ok := target.(*ast.SelectorExpr); ok {
		if pkg, ok := sel.X.(*ast.Ident); ok {
			if path, known := imports[pkg.Name]; known && goStdlibPkg(path) {
				return pkg.Name
			}
		}
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, expr); err != nil {
		return "?"
	}
	return strings.Join(strings.Fields(buf.String()), "")
}

// goSignatureOf builds the representation-independent "(paramType,...)=>returnType"
// signature string for the Type-4 signature-rarity recall pass (SignatureCandidates),
// mirroring extract_ts.mjs's signatureOf. Declared types only — Go has no
// annotation-inference ambiguity here, every function is fully statically typed.
// Flattens grouped param/result names (func f(x, y int) shares one *ast.Field,
// rendered once per name) and Go's multi-value returns: zero results -> "()",
// one -> the bare type (no parens, matching TS's convention), two or more ->
// "(T1,T2)". Type parameters (generics) are read as ordinary identifiers,
// matching how the TS side never inspects a function's own generic bounds either.
func goSignatureOf(fset *token.FileSet, imports map[string]string, ft *ast.FuncType) string {
	params := flattenFieldTypes(fset, imports, ft.Params)
	results := flattenFieldTypes(fset, imports, ft.Results)
	var ret string
	switch len(results) {
	case 0:
		ret = "()"
	case 1:
		ret = results[0]
	default:
		ret = "(" + strings.Join(results, ",") + ")"
	}
	return "(" + strings.Join(params, ",") + ")=>" + ret
}

// flattenFieldTypes renders each field's type in a param/result list, repeating
// a field's type once per grouped name (func f(x, y int) shares one *ast.Field,
// rendered once for x and once for y) or once for an unnamed field. Shared by
// goSignatureOf's param and result passes — the two were byte-identical logic
// applied to ft.Params/ft.Results before this extraction.
func flattenFieldTypes(fset *token.FileSet, imports map[string]string, fl *ast.FieldList) []string {
	if fl == nil {
		return nil
	}
	var out []string
	for _, field := range fl.List {
		t := goTypeSig(fset, imports, field.Type)
		n := len(field.Names)
		if n == 0 {
			n = 1
		}
		for i := 0; i < n; i++ {
			out = append(out, t)
		}
	}
	return out
}

// fragSite names WHERE a function or sub-function fragment sits: its file,
// qualified enclosing-function name (for Key()), and the enclosing function's
// own name (for Name-based stem matching). Shared by goFuncSigFromBody and
// goFuncSigFromStmts — the two ways an AST subtree becomes a fragment — so
// the same file/qual/name triple isn't threaded through both as three loose
// parameters.
type fragSite struct {
	file, qual, name string
}
