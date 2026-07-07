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
	_ "embed"
	"regexp"
	"strings"
)

// FuncSig is the signature of one function/method. The JSON tags are the
// extractor interchange format: any extractor (go/ast, python3, …) emits these;
// the scorer consumes them. Derived set/stem forms are computed by Prepare.
type FuncSig struct {
	File     string `json:"file"`
	Qualname string `json:"qualname"` // "Type.Method" or "func"
	Name     string `json:"name"`
	Line     int    `json:"line"`
	NLines   int    `json:"n_lines"`
	// NodeCount is the number of AST nodes visited in the body — a more precise
	// substantiality proxy than NLines (a one-line ternary and a ten-statement
	// one-liner both count as "1 line"). Populated by each extractor's existing
	// body-walk visitor; an extractor that omits it yields 0, a harmless no-op
	// everywhere it's gated on (SizeGate.MinNodes defaults to 0 = disabled).
	NodeCount int      `json:"node_count,omitempty"`
	Strings   []string `json:"strings"`  // emitted string literals (≥4 chars)
	Writes    []string `json:"writes"`   // dotted attribute/field write targets
	Reads     []string `json:"reads"`    // dotted attribute/field READ paths (derivation inputs)
	RetKeys   []string `json:"ret_keys"` // keys of a returned map/struct literal
	Calls     []string `json:"calls"`    // called function/method leaf names
	Consts    []string `json:"consts"`   // referenced SCREAMING_SNAKE domain constants (V_BELOW, GRID)
	// DeclConsts is the SCREAMING_SNAKE constants DECLARED at module/file scope in
	// THIS function's file (repeated across the file's functions). The touchpoint
	// pass unions it across the corpus to gate the const seam channel on
	// project-definition — exactly as project-defined call names gate the call
	// channel — so std/library masqueraders (O_CREATE, RFC3339, JSON) that are
	// referenced but never declared in-project don't form clusters.
	DeclConsts []string `json:"decl_consts,omitempty"`
	Delegates  bool     `json:"delegates"` // body forwards to a wrapped impl

	// Test marks a function as TEST code — either by file convention (IsTestPath:
	// *_test.go, tests.rs, test_*.py, *.test.ts, a tests/ dir, …) or by an
	// extractor's inline detection (the Rust extractor flags #[cfg(test)] / #[test]
	// functions that live inside a production .rs file). The recall passes gate
	// test↔test pairs/clusters by default — two test cases sharing a setup/mock
	// fixture are the dominant false-twin variety — while KEEPING test↔prod pairs:
	// a test that reimplements production construction/derivation is real drift, not
	// noise. Set in Extract; --include-tests opts the test↔test pairs back in.
	Test bool `json:"test,omitempty"`

	// Kind tags non-function entities the CROSS-SUBSTRATE axis extracts: "" = a
	// function (the default — the code axis; zero perturbation to existing callers),
	// "table" = a module-level dict/set/list constant (its keys live in RetKeys),
	// "corpus-field" = a JSON object (its field names live in RetKeys). Only the
	// generator-only `propose-cross` path produces non-"" kinds; the scoring gate
	// (Extract→Rank→check→--strict) never sees them.
	Kind string `json:"kind,omitempty"`

	// Sig is the normalized type signature — "(paramType,…)=>returnType" — a
	// REPRESENTATION-INDEPENDENT invariant: two behavioral (Type-4) twins share a
	// contract even when they share no token. Populated where the language exposes
	// types (TS today); empty otherwise. Experimental: drives the signature-rarity
	// recall pass, not the jaccard scorer.
	Sig string `json:"sig,omitempty"`

	// Source is an in-process judge blob for entities with no clean line span (a
	// JSON object): the corpus-field extractor sets it to the marshaled object so the
	// oracle reads the real shape. NOT part of the extractor interchange (json:"-");
	// empty for functions and tables, which the line-based readFuncSource handles.
	Source string `json:"-"`

	sStr, sWrite, sRead, sRet, sCall, stem set
}

// Key uniquely identifies a function within a scan.
func (f *FuncSig) Key() string { return f.File + "::" + f.Qualname }

// Prepare computes the derived set + stem forms. Call once after loading.
func (f *FuncSig) Prepare() {
	f.sStr = toSet(f.Strings)
	f.sWrite = toSet(f.Writes)
	f.sRead = toSet(f.Reads)
	f.sRet = toSet(f.RetKeys)
	f.sCall = toSet(f.Calls)
	f.stem = stemTokens(f.Name)
}

type set map[string]struct{}

func toSet(xs []string) set {
	s := make(set, len(xs))
	for _, x := range xs {
		s[x] = struct{}{}
	}
	return s
}

func (s set) has(x string) bool { _, ok := s[x]; return ok }

func (s set) slice() []string {
	out := make([]string, 0, len(s))
	for x := range s {
		out = append(out, x)
	}
	return out
}

func jaccard(a, b set) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	inter := 0
	for x := range a {
		if b.has(x) {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func intersect(a, b set) []string {
	var out []string
	for x := range a {
		if b.has(x) {
			out = append(out, x)
		}
	}
	return out
}

// rolePrefixes are stripped when normalizing a name to its role stem, so
// _handle_leave_town and leave_town collapse to the same contract stem.
var rolePrefixes = set{
	"handle": {}, "do": {}, "try": {}, "resolve": {}, "check": {}, "run": {},
	"apply": {}, "process": {}, "on": {}, "maybe": {}, "build": {}, "make": {},
	"get": {}, "compute": {},
}

var stopwords = set{
	"the": {}, "a": {}, "an": {}, "to": {}, "for": {}, "of": {}, "and": {}, "or": {},
}

// delegationRoots name wrapper attributes that mark a method as forwarding to a
// wrapped impl (self._engine.step(...)) — an adapter, not a reimplementation.
var delegationRoots = set{
	"_engine": {}, "_impl": {}, "_inner": {}, "_delegate": {},
	"_wrapped": {}, "_backend": {}, "_real": {}, "_target": {},
}

var camelBoundary = regexp.MustCompile(`([a-z0-9])([A-Z])`)

// normTokens: strip leading underscores, split camelCase, lowercase, split on _.
func normTokens(name string) []string {
	n := strings.TrimLeft(strings.TrimSpace(name), "_")
	n = camelBoundary.ReplaceAllString(n, "${1}_${2}")
	n = strings.ToLower(n)
	var out []string
	for _, t := range strings.Split(n, "_") {
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// stemTokens: role tokens of a name — peel one leading role prefix, drop stopwords.
func stemTokens(name string) set {
	toks := normTokens(name)
	if len(toks) > 1 && rolePrefixes.has(toks[0]) {
		toks = toks[1:]
	}
	out := set{}
	for _, t := range toks {
		if t != "" && !stopwords.has(t) {
			out[t] = struct{}{}
		}
	}
	return out
}

// rolePrefix returns the single leading role-prefix of a name, or "".
func rolePrefix(name string) string {
	toks := normTokens(name)
	if len(toks) > 1 && rolePrefixes.has(toks[0]) {
		return toks[0]
	}
	return ""
}

// IsDelegationRoot reports whether the first component of a dotted attribute
// path is a known wrapper attribute (used by extractors to set Delegates).
func IsDelegationRoot(root string) bool { return delegationRoots.has(root) }

// SizeGate bounds which functions the recall passes consider substantial
// enough to score, on two independent axes: MinLines (raw line span) and
// MinNodes (AST-node count of the body — a more precise substantiality proxy,
// since a one-line ternary and a ten-statement one-liner both count as "1
// line" but have very different NodeCount). The zero value disables both
// floors — exactly today's behavior from before this struct existed, so every
// existing caller that only cared about line count keeps its exact result by
// leaving MinNodes at 0.
type SizeGate struct {
	MinLines int
	MinNodes int
}

// keep reports whether f clears both size floors.
func (g SizeGate) keep(f *FuncSig) bool {
	return f.NLines >= g.MinLines && f.NodeCount >= g.MinNodes
}
