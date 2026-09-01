package code

import (
	"fmt"
	"sort"
	"strings"

	"github.com/justinstimatze/calque/internal/pairkey"
)

// Package-level: the call-site context axis (SPEC-callsite-context-axis.md).
// It instruments the CALLER instead of the candidate — the one class every
// other channel misses by construction: zero shared body tokens AND no
// distinctive type signature. Two signals combine to replace the anchor a
// shared token would normally provide (see CallContextCandidates):
// caller-role (this file) and call-result shape (FuncSig.CallResultShapes,
// extract_go.go's classifyCallContext).

// CallerStemIndex inverts the corpus's existing Calls edges into a
// callee-leaf-name -> union-of-caller-name-stem-tokens map, entirely from data
// already extracted (FuncSig.Calls, FuncSig.stem via Prepare). No new AST
// walk, no new field, no new extraction code per language — this is the
// caller-role signal, and it's a pure post-processing fold. Callers must have
// run Prepare() on sigs first (stem is derived, unexported); a sig with no
// stem (an unprepared entry, or a name-only-stopwords name) contributes
// nothing for the callees it calls, which degrades gracefully to an empty set
// on lookup, not a panic or a spurious match.
func CallerStemIndex(sigs []*FuncSig) map[string]set {
	idx := map[string]set{}
	for _, f := range sigs {
		if len(f.stem) == 0 {
			continue
		}
		for _, callee := range f.Calls {
			if idx[callee] == nil {
				idx[callee] = set{}
			}
			for tok := range f.stem {
				idx[callee][tok] = struct{}{}
			}
		}
	}
	return idx
}

// CallContextCandidates pairs functions with no shared body token and no
// distinctive type signature, purely on how and where they're invoked:
// overlapping caller-role stems (near-synonym callers, e.g.
// canBypassRateLimit / skipThrottleCheck) AND overlapping call-result shapes
// (both get nil-checked, both feed a downstream call) — see
// SPEC-callsite-context-axis.md §3. Both conditions required: shared caller
// vocabulary alone catches unrelated functions serving one popular caller;
// shared result-shape alone catches "gets its error checked," which is
// nearly every function in idiomatic Go (§5's valence-guard problem —
// unmitigated here; that stoplist is calibrated from real adjudication data
// as a follow-up, not guessed up front).
//
// Not a score.go channel: score.go's channels only ever get evaluated for a
// pair that already passed the existing hasAnchor gate, and a genuinely
// zero-token pair never reaches scorePair at all — the anchor has to live in
// a dedicated recall pass, same as SignatureCandidates/NameStemCandidates.
func CallContextCandidates(sigs []*FuncSig, gate SizeGate, minCallerJaccard, minShapeJaccard float64, maxFanout int) []SigCandidate {
	callerStems := CallerStemIndex(sigs)
	callShapes := CallShapeIndex(sigs)

	// A candidate callee needs both signals present, and its own definition
	// kept by the size gate. Iterating sigs (not a map) keeps the pick
	// deterministic when a leaf name is defined more than once (a method name
	// reused across types) — first-in-file-order wins; that name COLLISION is
	// NameStemCandidates' territory, not this axis's.
	byLeaf := map[string]*FuncSig{}
	for _, f := range sigs {
		if !gate.keep(f) || strings.HasPrefix(f.Name, "__") {
			continue
		}
		if len(callerStems[f.Name]) == 0 || len(callShapes[f.Name]) == 0 {
			continue
		}
		if _, dup := byLeaf[f.Name]; !dup {
			byLeaf[f.Name] = f
		}
	}

	// Inverted caller-stem index over candidate callees only, so comparison
	// stays near-linear like NameStemCandidates: only functions sharing at
	// least one caller-stem token are ever compared directly.
	byToken := map[string][]*FuncSig{}
	for name, f := range byLeaf {
		for tok := range callerStems[name] {
			byToken[tok] = append(byToken[tok], f)
		}
	}

	var out []SigCandidate
	seen := map[string]bool{}
	for _, fns := range byToken {
		if len(fns) < 2 || len(fns) > maxFanout {
			continue // a caller-stem shared by >maxFanout callees is plumbing, not a role
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
				callerJac := jaccard(callerStems[a.Name], callerStems[b.Name])
				if callerJac < minCallerJaccard {
					continue
				}
				shapeJac := jaccard(callShapes[a.Name], callShapes[b.Name])
				if shapeJac < minShapeJaccard {
					continue
				}
				seen[pk] = true
				jac := 0.0
				if s, ok := scorePair(a, b); ok {
					jac = s.Score
				}
				out = append(out, SigCandidate{
					A: a, B: b, Kind: "call-context",
					Sig:       fmt.Sprintf("caller≈%.2f shape≈%.2f", callerJac, shapeJac),
					GroupSize: 2, Jaccard: jac, CrossFile: a.File != b.File,
				})
			}
		}
	}

	// Most gate-invisible first, then most-overlapping-context, then deterministic.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Jaccard != out[j].Jaccard {
			return out[i].Jaccard < out[j].Jaccard
		}
		if out[i].Sig != out[j].Sig {
			return out[i].Sig > out[j].Sig
		}
		return pairkey.Key(out[i].A.Key(), out[i].B.Key()) < pairkey.Key(out[j].A.Key(), out[j].B.Key())
	})
	return out
}

// CallShapeIndex is CallerStemIndex's analog for call-result shape tags:
// callee leaf name -> union of shape tags observed across every call site
// into it, folded from every caller's already-extracted CallResultShapes.
// Same pure-fold, zero-new-extraction shape as CallerStemIndex — see
// SPEC-callsite-context-axis.md §2 for the tag vocabulary.
func CallShapeIndex(sigs []*FuncSig) map[string]set {
	idx := map[string]set{}
	for _, f := range sigs {
		for callee, tags := range f.CallResultShapes {
			if idx[callee] == nil {
				idx[callee] = set{}
			}
			for _, tag := range tags {
				idx[callee][tag] = struct{}{}
			}
		}
	}
	return idx
}
