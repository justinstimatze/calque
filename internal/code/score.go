package code

import (
	"fmt"
	"sort"
	"strings"

	"github.com/justinstimatze/calque/internal/pairkey"
)

// weights: surface (strings) + effect (writes) + role (name) carry the most —
// they survive a full rewrite; calls/ret corroborate. These are the STATIC
// prior/default — the immutable fallback CalibrateWeights shrinks toward and
// ResetWeights restores to. scorePair reads activeWeights, not this.
var weights = map[string]float64{
	"strings": 0.30, "writes": 0.30, "name": 0.22, "calls": 0.10, "ret": 0.08,
}

// activeWeights is the weight vector scorePair actually sums over. It defaults
// to a clone of the static `weights` prior; UseWeights swaps in a calibrated
// vector (from .calque/weights.json) and ResetWeights restores the prior. Kept
// separate from `weights` so the immutable default is always recoverable and so
// calque's own repo (no weights.json) trivially stays on defaults.
var activeWeights = cloneWeights(weights)

// channelOrder fixes the order scorePair sums the weighted signals in. It exists
// so the score is reproducible: ranging a map and accumulating non-associative
// float adds otherwise yields run-dependent scores near the threshold.
var channelOrder = []string{"strings", "writes", "name", "calls", "ret"}

// cloneWeights returns a fresh copy so callers can't alias the static prior.
func cloneWeights(w map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(w))
	for k, v := range w {
		out[k] = v
	}
	return out
}

// DefaultWeights returns a copy of the static prior weight vector.
func DefaultWeights() map[string]float64 { return cloneWeights(weights) }

// UseWeights swaps in a calibrated weight vector for subsequent scoring. Only
// the known channels (channelOrder) are honored; any missing channel falls back
// to its static prior, so a partial or stale vector can't zero out a signal.
// hasAnchor in scorePair is weight-independent, so this never changes which
// pairs anchor — only their scores — keeping the blocking-index superset
// invariant intact.
func UseWeights(w map[string]float64) {
	next := cloneWeights(weights)
	for _, k := range channelOrder {
		if v, ok := w[k]; ok {
			next[k] = v
		}
	}
	activeWeights = next
}

// ResetWeights restores scoring to the static prior. Mainly for tests that must
// not leak a calibrated vector into later cases.
func ResetWeights() { activeWeights = cloneWeights(weights) }

// Suspicion is one ranked suspect pair.
type Suspicion struct {
	Left, Right *FuncSig
	Score       float64
	Signals     map[string]float64
}

// Reason renders the fired signals, strongest first (matches the Python report).
func (s Suspicion) Reason() string {
	type kv struct {
		k string
		v float64
	}
	var fired []kv
	for k, v := range s.Signals {
		if v > 0 {
			fired = append(fired, kv{k, v})
		}
	}
	sort.Slice(fired, func(i, j int) bool { return fired[i].v > fired[j].v })
	var bits []string
	for _, f := range fired {
		switch f.k {
		case "name":
			shared := intersect(s.Left.stem, s.Right.stem)
			sort.Strings(shared)
			bits = append(bits, fmt.Sprintf("name~%.2f(%s)", f.v, strings.Join(shared, "+")))
		case "strings":
			bits = append(bits, fmt.Sprintf("shared-strings=%d", len(intersect(s.Left.sStr, s.Right.sStr))))
		case "writes":
			w := intersect(s.Left.sWrite, s.Right.sWrite)
			sort.Strings(w)
			bits = append(bits, fmt.Sprintf("shared-writes=%v", w))
		case "ret":
			r := intersect(s.Left.sRet, s.Right.sRet)
			sort.Strings(r)
			bits = append(bits, fmt.Sprintf("shared-ret-keys=%v", r))
		case "calls":
			bits = append(bits, fmt.Sprintf("shared-calls=%d", len(intersect(s.Left.sCall, s.Right.sCall))))
		}
	}
	return strings.Join(bits, "; ")
}

// scorePair scores one pair, or returns (_, false) if it fails the noise gate.
func scorePair(a, b *FuncSig) (Suspicion, bool) {
	// A forwarding adapter is named after what it wraps, so a name match alone is
	// a guaranteed false positive — keep a sliver of weight but bar it anchoring.
	delegating := a.Delegates || b.Delegates
	var nameRaw float64
	if len(a.stem) > 0 && setEqual(a.stem, b.stem) {
		nameRaw = 1.0
	} else {
		nameRaw = jaccard(a.stem, b.stem)
	}
	name := nameRaw
	if delegating {
		name = nameRaw * 0.2
	}
	sig := map[string]float64{
		"strings": jaccard(a.sStr, b.sStr),
		"writes":  jaccard(a.sWrite, b.sWrite),
		"name":    name,
		"calls":   jaccard(a.sCall, b.sCall),
		"ret":     jaccard(a.sRet, b.sRet),
	}
	// Renormalize over signals available on both-or-either side, so a pair isn't
	// penalized for, e.g., neither emitting strings.
	avail := map[string]bool{
		"strings": len(a.sStr) > 0 || len(b.sStr) > 0,
		"writes":  len(a.sWrite) > 0 || len(b.sWrite) > 0,
		"name":    len(a.stem) > 0 || len(b.stem) > 0,
		"calls":   len(a.sCall) > 0 || len(b.sCall) > 0,
		"ret":     len(a.sRet) > 0 || len(b.sRet) > 0,
	}
	// Sum in a FIXED channel order, not by ranging `avail` (a map): float addition
	// is non-associative, so map-iteration order made a near-threshold score flip
	// across minScore between runs — a non-reproducible gate, exactly the kind of
	// bug calque exists to catch. Determinism here is correctness, not polish.
	var wsum, score float64
	for _, k := range channelOrder {
		if avail[k] {
			wsum += activeWeights[k]
			score += activeWeights[k] * sig[k]
		}
	}
	if wsum == 0 {
		wsum = 1
	}
	score /= wsum

	// Gate: require a real role overlap OR a concrete surface/effect overlap. A
	// pair sharing only generic call names is junk; a pure forwarder with no
	// shared surface/effect drops out entirely.
	hasAnchor := (name >= 0.34 && !delegating) || sig["strings"] > 0 || sig["writes"] > 0 || sig["ret"] > 0
	if !hasAnchor {
		return Suspicion{}, false
	}
	return Suspicion{Left: a, Right: b, Score: score, Signals: sig}, true
}

func setEqual(a, b set) bool {
	if len(a) != len(b) {
		return false
	}
	for x := range a {
		if !b.has(x) {
			return false
		}
	}
	return true
}

// Rank scores every left×right pair and returns the top suspects. Pairs are
// deduped on the UNORDERED pair {left,right} — fixing the original Python's
// symmetric-output bug (a self-scan, where left==right, otherwise emits both
// A≟B and B≟A). calque must not carry the bug it detects.
func Rank(left, right []*FuncSig, minLines int, minScore float64, top int) []Suspicion {
	keep := func(fs []*FuncSig) []*FuncSig {
		var out []*FuncSig
		for _, f := range fs {
			if f.NLines >= minLines && !strings.HasPrefix(f.Name, "__") {
				out = append(out, f)
			}
		}
		return out
	}
	L, R := keep(left), keep(right)

	// Block first: candidatePairs yields only pairs sharing an anchor-channel
	// token, which is a superset of every pair scorePair can accept — so this is
	// output-identical to the naive L×R double loop, just far fewer scorePair
	// calls. anchorChannels MUST track scorePair's hasAnchor gate (see block.go).
	var out []Suspicion
	for _, p := range candidatePairs(L, R, anchorChannels) {
		a, b := p[0], p[1]
		if s, ok := scorePair(a, b); ok && s.Score >= minScore {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })

	seen := map[string]bool{}
	var deduped []Suspicion
	for _, s := range out {
		pk := pairkey.Key(s.Left.Key(), s.Right.Key())
		if seen[pk] {
			continue
		}
		seen[pk] = true
		deduped = append(deduped, s)
	}
	if len(deduped) > top {
		deduped = deduped[:top]
	}
	return deduped
}
