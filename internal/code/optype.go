package code

// Operation-type (method-stereotype) classification — a coarse label for what a
// function DOES, derived from signals FuncSig already carries (name role-stem +
// writes/ret shape). Used as a precision gate on the derivation pass: two functions
// that read the same fields but perform PROVABLY-DUAL operations — a forward map vs
// its inverse search, or a constructor vs a measure — are not twins, so such a
// candidate pair is suppressed. Lineage: method stereotypes (Dragan/Collard/Maletic),
// computable by light static analysis, here pointed at twin discrimination.
//
// Heuristic and ABLATABLE (doctor --ablate decides if it earns its keep). It only
// ever SUPPRESSES a provably-dual pair — never asserts a twin — and never fires when
// either side is unclassified (""), so a weak signal can't wrongly drop a real twin.

var opInverseSearch = set{
	"project": {}, "search": {}, "find": {}, "nearest": {}, "closest": {},
	"locate": {}, "bisect": {}, "invert": {}, "solve": {}, "lookup": {}, "intersect": {},
}
var opForwardMap = set{
	"sample": {}, "eval": {}, "evaluate": {}, "interpolate": {}, "lerp": {}, "transform": {},
}
var opMeasure = set{
	"measure": {}, "distance": {}, "residual": {}, "error": {}, "deviation": {},
	"continuity": {}, "count": {}, "area": {}, "length": {}, "score": {}, "norm": {},
	"metric": {}, "curvature": {},
}

// opType returns a coarse computation type, or "" when nothing fires (unknown —
// never gates). Name-role signals take precedence over structural ones, in a fixed
// priority order so the result is deterministic regardless of stem map order.
func opType(f *FuncSig) string {
	switch {
	case anyIn(f.stem, opInverseSearch):
		return "inverse-search"
	case anyIn(f.stem, opForwardMap):
		return "forward-map"
	case anyIn(f.stem, opMeasure):
		return "measure"
	case len(f.sRet) > 0:
		return "construct"
	case len(f.sWrite) > 0:
		return "mutate"
	default:
		return ""
	}
}

// opLabel renders an op-type for a candidate's Sig, "?" when unclassified.
func opLabel(o string) string {
	if o == "" {
		return "?"
	}
	return o
}

func anyIn(s, members set) bool {
	for t := range s {
		if members.has(t) {
			return true
		}
	}
	return false
}

// opposedOps reports whether two op-types are a provably-dual pair (no shared
// sub-computation): a forward map and its inverse search, or a constructor and a
// measure. Both sides must classify — an unclassified ("") side never suppresses.
func opposedOps(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	dual := func(x, y string) bool { return (a == x && b == y) || (a == y && b == x) }
	return dual("forward-map", "inverse-search") || dual("construct", "measure")
}
