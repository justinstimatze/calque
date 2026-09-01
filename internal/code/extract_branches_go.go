package code

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"

	"github.com/justinstimatze/calque/internal/corpus"
)

// branchExtractors is the (currently Go-only) per-extension dispatch for the
// sub-function branch axis (propose-branches). Kept separate from
// symbolExtractors/extractors so the function-only scoring gate
// (Extract -> Rank -> check -> --strict) never sees Kind:"branch" entities.
var branchExtractors = map[string]func(paths []string, root string) ([]*FuncSig, error){
	".go": extractGoBranchesBatch,
}

// ExtractBranches walks repo and extracts intra-function conditional ARMS
// (if/else bodies, switch/select case bodies) as Kind:"branch" FuncSig-like
// records — the sub-function "dual-path" axis's corpus (propose-branches).
// Mirrors ExtractSymbols's shape: a third, structurally-separate extraction
// entry point from the function-only Extract, so it never touches the
// scoring gate. Uses prepareSigs (not a bare .Prepare() loop) so branch
// fragments inherit the same file-path Test tagging whole functions get —
// unlike ExtractSymbols's table/corpus-field entities, a branch arm's
// "is this test code" status is meaningful and worth gating on (test fixtures
// tend to be dense with near-identical if/else assertions, the branch axis's
// dominant false-twin variety, same as touchpoint/confess/propose-deriv
// already gate test↔test pairs by default). Go-only this pass (see
// docs/DESIGN_NOTES.md §21).
func ExtractBranches(repo string, exclude []string) ([]*FuncSig, ScanStats, error) {
	st := ScanStats{SkippedExts: map[string]int{}}
	byExt, err := walkExtractable(repo, exclude,
		func(ext string) bool { _, ok := branchExtractors[ext]; return ok }, nil)
	if err != nil {
		return nil, st, err
	}
	var all []*FuncSig
	for ext, paths := range byExt {
		sigs, exErr := branchExtractors[ext](paths, repo)
		if exErr != nil {
			return nil, st, fmt.Errorf("%s branch extractor: %w", ext, exErr)
		}
		prepareSigs(sigs)
		all = append(all, sigs...)
		st.Files += len(paths)
		st.Funcs += len(sigs)
	}
	return all, st, nil
}

func extractGoBranchesBatch(paths []string, root string) ([]*FuncSig, error) {
	return goBatch(paths, root, extractGoBranchesFile)
}

// extractGoBranchesFile parses one Go file and returns a Kind:"branch" FuncSig
// per top-level conditional ARM (if/else body, switch/select case body) found
// inside any function's body. "Top-level" means not already nested inside
// another arm this pass extracted — a conditional inside an already-extracted
// arm contributes to THAT arm's bag via the normal recursive walk, but does
// not also become its own separate top-level entity (bounds the count to the
// number of top-level conditionals, not exponential in nesting depth).
func extractGoBranchesFile(path, root string) []*FuncSig {
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
		qual := qualNameFor(name, fd.Recv)
		bf := &branchFinder{fset: fset, file: rel, qual: qual, name: name}
		ast.Walk(bf, fd.Body)
		out = append(out, bf.frags...)
	}
	return out
}

// branchFinder walks a function body looking for conditional boundary nodes
// (if/switch/type-switch/select). It does not accumulate signals itself —
// each discovered arm is extracted independently via goFuncSigFromBody/
// goFuncSigFromStmts (a fresh goBody per arm). Visit returns nil for a
// boundary node's own subtree once handled, so the outer walk never
// re-discovers a nested conditional as its own separate top-level fragment;
// it still descends normally through non-boundary nodes (for/block/etc.) to
// find sibling conditionals elsewhere in the function.
type branchFinder struct {
	fset             *token.FileSet
	file, qual, name string
	n                int // per-function arm counter, keeps Key() collision-proof
	frags            []*FuncSig
}

func (bf *branchFinder) Visit(n ast.Node) ast.Visitor {
	switch t := n.(type) {
	case *ast.IfStmt:
		bf.extractIf(t)
		return nil
	case *ast.SwitchStmt:
		bf.extractClauses(t.Body)
		return nil
	case *ast.TypeSwitchStmt:
		bf.extractClauses(t.Body)
		return nil
	case *ast.SelectStmt:
		bf.extractClauses(t.Body)
		return nil
	}
	return bf
}

// extractIf handles one if/else-if/else chain: each `if`'s Body is an arm; a
// block Else is another arm; an else-if Else recurses as its own boundary —
// each link of the chain is its own binary branch point.
func (bf *branchFinder) extractIf(ifs *ast.IfStmt) {
	bf.addArm(ifs.Body)
	switch els := ifs.Else.(type) {
	case *ast.BlockStmt:
		bf.addArm(els)
	case *ast.IfStmt:
		bf.extractIf(els)
	}
}

// addArm extracts an if/else block body as one branch fragment. Empty arms
// (no statements) are skipped — nothing to compare.
func (bf *branchFinder) addArm(body *ast.BlockStmt) {
	if body == nil || len(body.List) == 0 {
		return
	}
	bf.n++
	bf.tagFrag(goFuncSigFromBody(bf.fset, body, fragSite{bf.file, bf.qual, bf.name}))
}

// addStmts extracts a switch/select case's statement list as one branch
// fragment, spanning [start,end) (the case clause's own position, since a
// bare statement list has no single walkable node with real Pos()/End()).
func (bf *branchFinder) addStmts(stmts []ast.Stmt, start, end token.Pos) {
	if len(stmts) == 0 {
		return
	}
	bf.n++
	bf.tagFrag(goFuncSigFromStmts(bf.fset, clauseBody{stmts, start, end}, fragSite{bf.file, bf.qual, bf.name}))
}

// tagFrag finalizes a fragment built by goFuncSigFromBody/goFuncSigFromStmts
// (which set Qualname to the enclosing function's qualname, for Name-based
// stem matching) by uniquifying Qualname for Key() — line plus a per-function
// arm counter, so two arms starting on the same line (rare, but possible in
// densely formatted one-liners) never collide.
func (bf *branchFinder) tagFrag(fs *FuncSig) {
	fs.Kind = "branch"
	fs.Qualname = fmt.Sprintf("%s#branch@%d.%d", bf.qual, fs.Line, bf.n)
	bf.frags = append(bf.frags, fs)
}

// goFuncSigFromStmts is goFuncSigFromBody's sibling for a bare statement list
// (a switch/select case's Body) rather than a single walkable *ast.BlockStmt.
func goFuncSigFromStmts(fset *token.FileSet, cb clauseBody, site fragSite) *FuncSig {
	bv := &goBody{strs: set{}, writes: set{}, retKeys: set{}, calls: set{}, consts: set{}, readsRaw: set{}, pureWrites: set{}, calleeSkip: map[ast.Expr]bool{}}
	for _, s := range cb.stmts {
		ast.Walk(bv, s)
	}
	startLine := fset.Position(cb.start).Line
	endLine := fset.Position(cb.end).Line
	return &FuncSig{
		File: site.file, Qualname: site.qual, Name: site.name,
		Line: startLine, NLines: endLine - startLine + 1, NodeCount: bv.nodes,
		Strings: bv.strs.slice(), Writes: bv.writes.slice(),
		Reads:   bv.reads(),
		RetKeys: bv.retKeys.slice(), Calls: bv.calls.slice(),
		Consts:    bv.consts.slice(),
		Delegates: bv.delegates,
	}
}

// clauseBody is a switch/select case's statement list plus its own [start,end)
// span — a CaseClause/CommClause's Body has no Lbrace/Rbrace of its own, so
// the span has to travel alongside the statements rather than be derived from
// them. Bundled so goFuncSigFromStmts doesn't take the three as loose params.
type clauseBody struct {
	stmts      []ast.Stmt
	start, end token.Pos
}

// extractClauses handles a switch/type-switch/select body: each CaseClause
// (switch) or CommClause (select) inside becomes one branch fragment. The two
// clause types never appear in the same block, so one type-switch safely
// covers both without a second, byte-identical function.
func (bf *branchFinder) extractClauses(body *ast.BlockStmt) {
	if body == nil {
		return
	}
	for _, stmt := range body.List {
		switch cc := stmt.(type) {
		case *ast.CaseClause:
			bf.addStmts(cc.Body, cc.Pos(), cc.End())
		case *ast.CommClause:
			bf.addStmts(cc.Body, cc.Pos(), cc.End())
		}
	}
}
