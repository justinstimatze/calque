package code

// Blocking index: the candidate-generation step that lets Rank skip scoring the
// vast majority of L×R pairs. A naive Rank is O(L·R) — every pair visited. But
// scorePair's hasAnchor gate REJECTS any pair that does not share a token in one
// of four channels (name-stem, strings, writes, ret), so the only pairs that can
// survive are those sharing ≥1 such token. An inverted index over those channels
// yields exactly the candidate superset; scoring it is output-identical to the
// double loop (see TestRankBlockingEquivalence) while touching far fewer pairs.
//
// The same machinery is reused by the no-footprint discovery pilot over LOOSER
// channels (e.g. calls) that scorePair can't anchor on — see channelSet.

import "github.com/justinstimatze/calque/internal/pairkey"

// channelSet selects which FuncSig token channels a blocking index buckets on.
type channelSet struct {
	stem, strings, writes, ret, calls bool
}

// anchorChannels are exactly the channels scorePair's hasAnchor gate can fire on.
//
// INVARIANT: this MUST stay a superset of every channel anchored in scorePair.
// candidatePairs(L, R, anchorChannels) is then a guaranteed superset of the pairs
// scorePair accepts, which is what makes Rank's blocked output identical to the
// naive L×R loop. If a 5th anchor channel is ever added to scorePair's hasAnchor,
// add it here too — otherwise Rank will silently drop those pairs. The fixtures in
// TestRankBlockingEquivalence cover each channel independently to catch that.
var anchorChannels = channelSet{stem: true, strings: true, writes: true, ret: true}

// tokens returns f's tokens across the selected channels, each namespaced by a
// channel tag so a string "foo" and a stem "foo" land in different buckets.
func (f *FuncSig) tokens(ch channelSet) []string {
	var out []string
	add := func(tag string, s set) {
		for x := range s {
			out = append(out, tag+x)
		}
	}
	if ch.stem {
		add("n\x00", f.stem)
	}
	if ch.strings {
		add("s\x00", f.sStr)
	}
	if ch.writes {
		add("w\x00", f.sWrite)
	}
	if ch.ret {
		add("r\x00", f.sRet)
	}
	if ch.calls {
		add("c\x00", f.sCall)
	}
	return out
}

// candidatePairs returns the unordered {a,b} pairs (a from left, b from right)
// that share at least one token in the selected channels. Each unordered pair is
// emitted once; self-pairs (same Key) are skipped. Orientation follows the naive
// loop's intent — a is drawn from left, b from right — so boundary-mode callers
// keep Left==left-argument. Return order is unspecified; Rank sorts downstream.
func candidatePairs(left, right []*FuncSig, ch channelSet) [][2]*FuncSig {
	// Inverted index over right: token -> functions carrying it.
	index := map[string][]*FuncSig{}
	for _, b := range right {
		for _, tok := range b.tokens(ch) {
			index[tok] = append(index[tok], b)
		}
	}

	var out [][2]*FuncSig
	seen := map[string]bool{}
	for _, a := range left {
		// Collapse multi-token collisions: each candidate b probed once per a.
		hit := map[string]*FuncSig{}
		for _, tok := range a.tokens(ch) {
			for _, b := range index[tok] {
				hit[b.Key()] = b
			}
		}
		for _, b := range hit {
			if a.Key() == b.Key() {
				continue
			}
			pk := pairkey.Key(a.Key(), b.Key())
			if seen[pk] {
				continue
			}
			seen[pk] = true
			out = append(out, [2]*FuncSig{a, b})
		}
	}
	return out
}
