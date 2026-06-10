package code

import (
	"fmt"
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

// NameStemCandidates is the representation-independent recall pass that DOESN'T need
// types — so it works for every language (signature recall is TS-only). It pairs
// functions whose name-stem token SETS are near-identical (jaccard >= minJaccard):
// two functions named for the same role, regardless of token order or a different
// word ("formatRemainingTime" ≟ "formatTimeRemaining" = 1.0). This catches the twin
// class signature recall misses — different/absent type signatures but the same role.
// Cheap and noisier than signature recall; the judge is the precision filter. An
// inverted stem index keeps it near-linear (only functions sharing a stem are scored),
// and a fanout cap skips ultra-common stems (get/handle/…) that would pair everything.
func NameStemCandidates(sigs []*FuncSig, minLines int, minJaccard float64, maxFanout int) []SigCandidate {
	idx := map[string][]*FuncSig{}
	for _, f := range sigs {
		if f.NLines < minLines || strings.HasPrefix(f.Name, "__") || len(f.stem) == 0 {
			continue
		}
		for tok := range f.stem {
			idx[tok] = append(idx[tok], f)
		}
	}
	var out []SigCandidate
	seen := map[string]bool{}
	for _, fns := range idx {
		if len(fns) < 2 || len(fns) > maxFanout {
			continue // a stem shared by >maxFanout funcs is plumbing, not a role
		}
		for i := 0; i < len(fns); i++ {
			for j := i + 1; j < len(fns); j++ {
				a, b := fns[i], fns[j]
				if a.Key() == b.Key() {
					continue
				}
				pk := pairkey.Key(a.Key(), b.Key())
				if seen[pk] {
					continue
				}
				nj := jaccard(a.stem, b.stem)
				if nj < minJaccard {
					continue
				}
				seen[pk] = true
				jac := 0.0
				if s, ok := scorePair(a, b); ok {
					jac = s.Score
				}
				out = append(out, SigCandidate{
					A: a, B: b, Kind: "name-stem",
					Sig:       fmt.Sprintf("name≈%.2f %s", nj, a.Name),
					GroupSize: 2, Jaccard: jac, CrossFile: a.File != b.File,
				})
			}
		}
	}
	// Strongest first: highest name-stem jaccard (rendered into Sig), then most
	// gate-invisible, then deterministic.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Sig != out[j].Sig {
			return out[i].Sig > out[j].Sig
		}
		if out[i].Jaccard != out[j].Jaccard {
			return out[i].Jaccard < out[j].Jaccard
		}
		return pairkey.Key(out[i].A.Key(), out[i].B.Key()) < pairkey.Key(out[j].A.Key(), out[j].B.Key())
	})
	return out
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

// SigCandidate is one Type-4 twin candidate.
type SigCandidate struct {
	A, B      *FuncSig
	Kind      string  // "signature" | "name-stem" — how it was generated
	Sig       string  // the shared signature, or "name≈<stems>" for a name-stem match
	GroupSize int     // signature-group size (smaller = rarer); 2 for a name-stem pair
	Jaccard   float64 // the jaccard scorer's score for this pair (how gate-(in)visible it is)
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
					A: a, B: b, Kind: "signature", Sig: sig, GroupSize: len(fns),
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

// KeySetCandidates is the CROSS-SUBSTRATE recall pass. It pairs NON-FUNCTION
// entities (module-level tables, JSON corpus-field objects — see ExtractSymbols)
// whose KEY SETS (RetKeys) overlap by jaccard >= minJaccard: two registries of the
// same verbs (engine.py::HANDLERS ≟ input_agent.py::_VERB_TEMPLATES), or an
// authored corpus shape and the code table/schema that mirrors it. This is the
// pairing the code axis structurally CANNOT make — the two sides aren't functions
// and share no surface tokens the jaccard gate anchors on. An inverted key index
// keeps it near-linear; the fanout cap drops ubiquitous keys (id/name) as JOIN
// paths only — a real twin still surfaces through a rarer shared key, and jaccard
// is computed over the FULL key sets regardless. High recall, low precision; the
// judge is the precision filter. minKeys filters thin (non-discriminating) entities.
func KeySetCandidates(entities []*FuncSig, minKeys int, minJaccard float64, maxFanout int) []SigCandidate {
	idx := map[string][]*FuncSig{}
	for _, e := range entities {
		if len(e.sRet) < minKeys {
			continue
		}
		for k := range e.sRet {
			idx[k] = append(idx[k], e)
		}
	}
	var out []SigCandidate
	seen := map[string]bool{}
	for _, ents := range idx {
		if len(ents) < 2 || len(ents) > maxFanout {
			continue // a key shared by >maxFanout entities is plumbing, not a shape
		}
		for i := 0; i < len(ents); i++ {
			for j := i + 1; j < len(ents); j++ {
				a, b := ents[i], ents[j]
				if a.Key() == b.Key() {
					continue
				}
				pk := pairkey.Key(a.Key(), b.Key())
				if seen[pk] {
					continue
				}
				kj := jaccard(a.sRet, b.sRet)
				if kj < minJaccard {
					continue
				}
				seen[pk] = true
				out = append(out, SigCandidate{
					A: a, B: b, Kind: "key-set",
					Sig:       fmt.Sprintf("keys≈%.2f {%s}", kj, sharedKeysLabel(a, b)),
					GroupSize: 2, Jaccard: kj, CrossFile: a.File != b.File,
				})
			}
		}
	}
	// Strongest shared SHAPE first (highest key-set jaccard), then cross-file, then
	// deterministic. Unlike the function passes there's no gate to be invisible to —
	// these entities never enter scorePair — so Jaccard here carries the key-set
	// overlap and ranks descending (match strength), not ascending.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Jaccard != out[j].Jaccard {
			return out[i].Jaccard > out[j].Jaccard
		}
		if out[i].CrossFile != out[j].CrossFile {
			return out[i].CrossFile
		}
		return pairkey.Key(out[i].A.Key(), out[i].B.Key()) < pairkey.Key(out[j].A.Key(), out[j].B.Key())
	})
	return out
}

// sharedKeysLabel renders up to 4 shared RetKeys (sorted) for a candidate's human
// label — what the two entities have in common at a glance.
func sharedKeysLabel(a, b *FuncSig) string {
	var shared []string
	for k := range a.sRet {
		if b.sRet.has(k) {
			shared = append(shared, k)
		}
	}
	sort.Strings(shared)
	if len(shared) > 4 {
		shared = append(shared[:4], "…")
	}
	return strings.Join(shared, ",")
}
