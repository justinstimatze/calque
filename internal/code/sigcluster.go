package code

import (
	"regexp"
	"sort"
	"strings"

	"github.com/justinstimatze/calque/internal/pairkey"
)

// Signature recall — the representation-INDEPENDENT Type-4 candidate pass. The
// jaccard scorer (score.go) indexes surface tokens, so it is blind to behavioral
// twins that share a contract but no token (the textbook Type-4 case: two impls of
// `sessionId → WorktreeInfo`, one reading JSON, one reconstructing from git). The
// normalized type signature `(paramTypes…)=>returnType` IS that shared contract.
// This pass groups functions by a rare, informative signature and emits the pairs
// as twin CANDIDATES — high recall, low precision by nature (many same-shape
// functions do deliberately different jobs), so it is a GENERATOR that feeds an
// adjudicator (you, or a future LLM judge), never a gate.

// richType counts named/structured-type markers in a signature (used for the
// specificity *ranking*).
var richType = regexp.MustCompile(`[A-Z][A-Za-z0-9_]+|\[\]|<|\{`)

// namedType matches a capitalized type identifier.
var namedType = regexp.MustCompile(`[A-Z][A-Za-z0-9_]+`)

// builtinGeneric: capitalized identifiers that are language/stdlib wrappers, not a
// domain contract. A signature whose only capitalized types are these (e.g.
// `()=>Promise<string[]>`) is too generic to anchor a twin — the precision cut that
// separates real domain-typed twins from same-shape-different-job noise.
var builtinGeneric = map[string]bool{
	"Promise": true, "Array": true, "ReadonlyArray": true, "Record": true,
	"Map": true, "Set": true, "WeakMap": true, "WeakSet": true, "Partial": true,
	"Required": true, "Readonly": true, "Pick": true, "Omit": true, "Exclude": true,
	"Extract": true, "Awaited": true, "Returntype": true, "Parameters": true,
	"Iterable": true, "AsyncIterable": true, "Iterator": true, "Object": true,
	"Date": true, "RegExp": true, "Error": true, "Buffer": true, "JSON": true,
}

// signatureInformative reports whether a signature names at least one DOMAIN type
// (a capitalized identifier that isn't a stdlib generic) — the contract-specificity
// a Type-4 twin shares.
func signatureInformative(sig string) bool {
	if sig == "" || sig == "()=>?" {
		return false
	}
	for _, t := range namedType.FindAllString(sig, -1) {
		if !builtinGeneric[t] {
			return true
		}
	}
	return false
}

// opposedVerbs lists lead-verb pairs that share a signature shape but do
// deliberately OPPOSITE jobs — the dominant false-positive class for signature
// recall (insertTask≟updateTask, taskStart≟taskComplete, add≟remove). Filtering
// them is the main cheap precision booster. Symmetric: registered both directions.
var opposedVerbs = buildOpposed([][2]string{
	{"insert", "update"}, {"insert", "delete"}, {"create", "delete"},
	{"create", "update"}, {"add", "remove"}, {"add", "delete"}, {"push", "pop"},
	{"open", "close"}, {"start", "stop"}, {"start", "complete"}, {"start", "end"},
	{"begin", "end"}, {"enter", "exit"}, {"acquire", "release"}, {"lock", "unlock"},
	{"enable", "disable"}, {"show", "hide"}, {"mount", "unmount"}, {"get", "set"},
	{"read", "write"}, {"connect", "disconnect"}, {"register", "unregister"},
	{"attach", "detach"}, {"link", "unlink"}, {"increment", "decrement"},
	{"encode", "decode"}, {"serialize", "deserialize"}, {"expand", "collapse"},
})

func buildOpposed(pairs [][2]string) map[string]map[string]bool {
	m := map[string]map[string]bool{}
	add := func(a, b string) {
		if m[a] == nil {
			m[a] = map[string]bool{}
		}
		m[a][b] = true
	}
	for _, p := range pairs {
		add(p[0], p[1])
		add(p[1], p[0])
	}
	return m
}

// opposed reports whether two names are identical except for ONE token each, and
// that differing pair is an opposed verb (insertTask↔deleteTask, taskStart↔
// taskComplete). Position-independent — the discriminating token need not lead
// (taskStart leads with the noun). A pair differing in more than one token (e.g.
// getWorktreeForSession↔getWorktreeInfo) is NOT filtered: it may be a real twin.
func opposed(n1, n2 string) bool {
	s1, s2 := tokenSet(n1), tokenSet(n2)
	only1, only2 := diffTokens(s1, s2), diffTokens(s2, s1)
	if len(only1) == 1 && len(only2) == 1 {
		a, b := only1[0], only2[0]
		return opposedVerbs[a] != nil && opposedVerbs[a][b]
	}
	return false
}

func tokenSet(name string) map[string]bool {
	s := map[string]bool{}
	for _, t := range normTokens(name) {
		s[t] = true
	}
	return s
}

// diffTokens returns the tokens in a that are not in b.
func diffTokens(a, b map[string]bool) []string {
	var out []string
	for t := range a {
		if !b[t] {
			out = append(out, t)
		}
	}
	return out
}

// SigCandidate is one signature-matched twin candidate.
type SigCandidate struct {
	A, B      *FuncSig
	Sig       string
	GroupSize int     // how many functions share this signature (smaller = rarer = stronger)
	Jaccard   float64 // the current scorer's score for this pair (shows how gate-invisible it is)
	CrossFile bool
}

// SignatureCandidates groups functions by rare informative signature and returns
// twin candidates, ranked most-promising first (rarest signature, cross-file,
// richest type, most gate-invisible). Opposed-verb pairs are dropped. minMembers/
// maxMembers bound the rarity window — a signature shared by 2 functions is a strong
// signal; one shared by 50 is a common shape, not a twin.
func SignatureCandidates(sigs []*FuncSig, minLines, minMembers, maxMembers int) []SigCandidate {
	bySig := map[string][]*FuncSig{}
	for _, f := range sigs {
		if !signatureInformative(f.Sig) {
			continue
		}
		if f.NLines < minLines || strings.HasPrefix(f.Name, "__") {
			continue
		}
		bySig[f.Sig] = append(bySig[f.Sig], f)
	}

	var out []SigCandidate
	seen := map[string]bool{}
	for sig, fns := range bySig {
		if len(fns) < minMembers || len(fns) > maxMembers {
			continue
		}
		for i := 0; i < len(fns); i++ {
			for j := i + 1; j < len(fns); j++ {
				a, b := fns[i], fns[j]
				if a.Key() == b.Key() {
					continue
				}
				if opposed(a.Name, b.Name) {
					continue
				}
				pk := pairkey.Key(a.Key(), b.Key())
				if seen[pk] {
					continue
				}
				seen[pk] = true
				jac := 0.0
				if s, ok := scorePair(a, b); ok {
					jac = s.Score
				}
				out = append(out, SigCandidate{
					A: a, B: b, Sig: sig, GroupSize: len(fns),
					Jaccard: jac, CrossFile: a.File != b.File,
				})
			}
		}
	}

	sort.Slice(out, func(i, j int) bool {
		ci, cj := out[i], out[j]
		if ci.GroupSize != cj.GroupSize { // rarer first
			return ci.GroupSize < cj.GroupSize
		}
		if ci.CrossFile != cj.CrossFile { // cross-file first
			return ci.CrossFile
		}
		if rich(ci.Sig) != rich(cj.Sig) { // richer signature first
			return rich(ci.Sig) > rich(cj.Sig)
		}
		if ci.Jaccard != cj.Jaccard { // most gate-invisible first
			return ci.Jaccard < cj.Jaccard
		}
		// deterministic tiebreak
		return pairkey.Key(ci.A.Key(), ci.B.Key()) < pairkey.Key(cj.A.Key(), cj.B.Key())
	})
	return out
}

// rich counts the named/structured-type markers in a signature (specificity).
func rich(sig string) int { return len(richType.FindAllString(sig, -1)) }
