package code

import (
	"fmt"
	"sort"
	"strings"

	"github.com/justinstimatze/calque/internal/pairkey"
)

// signalDef is one scoring channel — the SINGLE SOURCE OF TRUTH for calque's
// signal taxonomy. surface (strings) + effect (writes) + role (name) carry the
// most weight (they survive a full rewrite); calls/ret corroborate. Adding a
// channel is one entry in `signals`: scorePair's weighted sum, Reason's evidence
// rendering, the static prior, and the fixed summation order all derive from it,
// so the taxonomy can't drift across those sites — it used to (a known, self-
// flagged dual path: the list lived in `weights`, `channelOrder`, scorePair's
// sig+avail maps, and Reason's switch). The anchor gate (hasAnchor / block.go
// anchorChannels) is deliberately a SUBSET and stays defined separately.
type signalDef struct {
	key    string
	weight float64                               // static prior weight
	sim    func(a, b *FuncSig) float64           // similarity in [0,1]
	avail  func(a, b *FuncSig) bool              // channel present on either side
	render func(a, b *FuncSig, v float64) string // evidence phrase for Reason
}

// signals — SLICE ORDER IS LOAD-BEARING. scorePair sums in this exact order so a
// near-threshold score is reproducible: float addition is non-associative, so
// ranging a map instead would yield run-dependent scores near minScore.
var signals = []signalDef{
	{
		key: "strings", weight: 0.30,
		sim:   func(a, b *FuncSig) float64 { return jaccard(a.sStr, b.sStr) },
		avail: func(a, b *FuncSig) bool { return len(a.sStr) > 0 || len(b.sStr) > 0 },
		render: func(a, b *FuncSig, _ float64) string {
			return fmt.Sprintf("shared-strings=%d", len(intersect(a.sStr, b.sStr)))
		},
	},
	{
		key: "writes", weight: 0.30,
		sim:   func(a, b *FuncSig) float64 { return jaccard(a.sWrite, b.sWrite) },
		avail: func(a, b *FuncSig) bool { return len(a.sWrite) > 0 || len(b.sWrite) > 0 },
		render: func(a, b *FuncSig, _ float64) string {
			w := intersect(a.sWrite, b.sWrite)
			sort.Strings(w)
			return fmt.Sprintf("shared-writes=%v", w)
		},
	},
	{
		key: "name", weight: 0.22,
		sim:   nameSim,
		avail: func(a, b *FuncSig) bool { return len(a.stem) > 0 || len(b.stem) > 0 },
		render: func(a, b *FuncSig, v float64) string {
			shared := intersect(a.stem, b.stem)
			sort.Strings(shared)
			return fmt.Sprintf("name~%.2f(%s)", v, strings.Join(shared, "+"))
		},
	},
	{
		key: "calls", weight: 0.10,
		sim:   func(a, b *FuncSig) float64 { return jaccard(a.sCall, b.sCall) },
		avail: func(a, b *FuncSig) bool { return len(a.sCall) > 0 || len(b.sCall) > 0 },
		render: func(a, b *FuncSig, _ float64) string {
			return fmt.Sprintf("shared-calls=%d", len(intersect(a.sCall, b.sCall)))
		},
	},
	{
		key: "ret", weight: 0.08,
		sim:   func(a, b *FuncSig) float64 { return jaccard(a.sRet, b.sRet) },
		avail: func(a, b *FuncSig) bool { return len(a.sRet) > 0 || len(b.sRet) > 0 },
		render: func(a, b *FuncSig, _ float64) string {
			r := intersect(a.sRet, b.sRet)
			sort.Strings(r)
			return fmt.Sprintf("shared-ret-keys=%v", r)
		},
	},
}

// nameSim is the role-overlap similarity: full credit for an identical stem set,
// jaccard otherwise, dampened 5x when either side is a forwarding adapter — its
// name mirrors what it wraps, so a name match alone is a guaranteed false twin.
func nameSim(a, b *FuncSig) float64 {
	var raw float64
	if len(a.stem) > 0 && setEqual(a.stem, b.stem) {
		raw = 1.0
	} else {
		raw = jaccard(a.stem, b.stem)
	}
	if a.Delegates || b.Delegates {
		return raw * 0.2
	}
	return raw
}

// weights is the static prior, DERIVED from signals so there is one source of
// truth. The immutable fallback CalibrateWeights shrinks toward and ResetWeights
// restores to; scorePair reads activeWeights, not this.
var weights = signalWeights()

// channelOrder is the fixed channel order, DERIVED from signals. The calibration
// subsystem ranges it; scorePair sums over `signals` directly (same order).
var channelOrder = signalKeys()

// activeWeights is the weight vector scorePair actually sums over. It defaults to
// a clone of the static prior; UseWeights swaps in a calibrated vector (from
// .calque/weights.json) and ResetWeights restores the prior. Kept separate so the
// immutable default is always recoverable and calque's own repo (no weights.json)
// trivially stays on defaults.
var activeWeights = cloneWeights(weights)

// signalKeys / signalWeights project the taxonomy onto the legacy shapes the
// calibration code consumes, without duplicating the channel list.
func signalKeys() []string {
	ks := make([]string, len(signals))
	for i, d := range signals {
		ks[i] = d.key
	}
	return ks
}

func signalWeights() map[string]float64 {
	w := make(map[string]float64, len(signals))
	for _, d := range signals {
		w[d.key] = d.weight
	}
	return w
}

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
	type fire struct {
		def signalDef
		v   float64
	}
	var fired []fire
	for _, def := range signals {
		if v := s.Signals[def.key]; v > 0 {
			fired = append(fired, fire{def, v})
		}
	}
	sort.Slice(fired, func(i, j int) bool { return fired[i].v > fired[j].v })
	var bits []string
	for _, f := range fired {
		bits = append(bits, f.def.render(s.Left, s.Right, f.v))
	}
	return strings.Join(bits, "; ")
}

// scorePair scores one pair, or returns (_, false) if it fails the noise gate.
func scorePair(a, b *FuncSig) (Suspicion, bool) {
	// A forwarding adapter is named after what it wraps, so a name match alone is
	// a guaranteed false positive — nameSim keeps a sliver of weight, and the
	// anchor gate below bars it from anchoring.
	delegating := a.Delegates || b.Delegates

	// Per-channel similarity, summing the weighted available ones in the FIXED
	// `signals` order — NOT by ranging a map: float addition is non-associative,
	// so map-iteration order made a near-threshold score flip across minScore
	// between runs — exactly the kind of bug calque exists to catch. Determinism
	// here is correctness, not polish. avail renormalizes so a pair isn't
	// penalized for, e.g., neither side emitting strings.
	sig := make(map[string]float64, len(signals))
	var wsum, score float64
	for _, def := range signals {
		s := def.sim(a, b)
		sig[def.key] = s
		if def.avail(a, b) {
			wsum += activeWeights[def.key]
			score += activeWeights[def.key] * s
		}
	}
	if wsum == 0 {
		wsum = 1
	}
	score /= wsum

	// Gate: require a real role overlap OR a concrete surface/effect overlap. A
	// pair sharing only generic call names is junk; a pure forwarder with no
	// shared surface/effect drops out entirely. This SUBSET is mirrored by
	// block.go's anchorChannels — keep them in lockstep.
	hasAnchor := (sig["name"] >= 0.34 && !delegating) || sig["strings"] > 0 || sig["writes"] > 0 || sig["ret"] > 0
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
func Rank(left, right []*FuncSig, minLines int, minScore float64, top int, includeTests bool) []Suspicion {
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
		// Asymmetric test gate: two test functions sharing signal is almost always
		// a shared setup/mock fixture, not drift — dropped by default. A test↔prod
		// pair survives: a test reimplementing production behavior IS the drift.
		if !includeTests && a.Test && b.Test {
			continue
		}
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
