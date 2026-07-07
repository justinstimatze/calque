package code

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/justinstimatze/calque/internal/pairkey"
)

// naiveRank is the pre-blocking O(L·R) double loop, kept verbatim as the oracle
// the blocking Rank must match. If Rank ever diverges from this, the blocking
// index dropped a pair scorePair would have accepted — a recall regression.
func naiveRank(left, right []*FuncSig, minLines int, minScore float64, top int) []Suspicion {
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
	var out []Suspicion
	for _, a := range L {
		for _, b := range R {
			if a.Key() == b.Key() {
				continue
			}
			if a.Test && b.Test { // mirror Rank's default test↔test gate (includeTests=false)
				continue
			}
			if s, ok := scorePair(a, b); ok && s.Score >= minScore {
				out = append(out, s)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	seen := map[string]bool{}
	var dz []Suspicion
	for _, s := range out {
		pk := pairkey.Key(s.Left.Key(), s.Right.Key())
		if seen[pk] {
			continue
		}
		seen[pk] = true
		dz = append(dz, s)
	}
	if len(dz) > top {
		dz = dz[:top]
	}
	return dz
}

// normalize keys a result set on the UNORDERED pair -> score, dropping Left/Right
// orientation (never a guaranteed property of the old loop on score-ties) so the
// comparison asserts what matters: same pairs, same scores.
func normalize(ss []Suspicion) map[string]float64 {
	m := make(map[string]float64, len(ss))
	for _, s := range ss {
		m[pairkey.Key(s.Left.Key(), s.Right.Key())] = s.Score
	}
	return m
}

func fsig(file, name string, strs, writes, ret, calls []string) *FuncSig {
	f := &FuncSig{File: file, Qualname: name, Name: name, NLines: 10,
		Strings: strs, Writes: writes, RetKeys: ret, Calls: calls}
	f.Prepare()
	return f
}

// TestRankBlockingEquivalence is the recall-preservation proof: blocking Rank
// must produce the same pairs+scores as the naive double loop. Fixtures anchor
// via EACH channel independently (stem, strings, writes, ret) so that dropping
// any channel from anchorChannels — e.g. forgetting to add a new scorePair anchor
// here — is caught. A calls-only pair (no anchor) must appear in neither.
func TestRankBlockingEquivalence(t *testing.T) {
	fixtures := []*FuncSig{
		// stem-only anchor: identical name stem, no shared surface/effect.
		fsig("a.go", "parseConfig", nil, nil, nil, nil),
		fsig("b.go", "parseConfig", nil, nil, nil, nil),
		// string-only anchor: disjoint stems, one shared literal.
		fsig("c.go", "alpha", []string{"shared_marker_xyz"}, nil, nil, nil),
		fsig("d.go", "beta", []string{"shared_marker_xyz"}, nil, nil, nil),
		// write-only anchor.
		fsig("e.go", "gamma", nil, []string{"obj.field_marker"}, nil, nil),
		fsig("f.go", "delta", nil, []string{"obj.field_marker"}, nil, nil),
		// ret-only anchor.
		fsig("g.go", "epsilon", nil, nil, []string{"status_key"}, nil),
		fsig("h.go", "zeta", nil, nil, []string{"status_key"}, nil),
		// calls-only: NOT an anchor channel -> must not surface in either Rank.
		fsig("i.go", "eta", nil, nil, nil, []string{"_shared_helper"}),
		fsig("j.go", "theta", nil, nil, nil, []string{"_shared_helper"}),
	}

	got := normalize(Rank(fixtures, fixtures, SizeGate{MinLines: 4}, 0.18, 1<<30, false))
	want := normalize(naiveRank(fixtures, fixtures, 4, 0.18, 1<<30))
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("blocking Rank != naive Rank on fixtures:\n got=%v\nwant=%v", got, want)
	}
	// Exactly the four anchoring pairs; the calls-only pair excluded.
	if len(got) != 4 {
		t.Fatalf("expected 4 anchoring pairs (stem/string/write/ret), got %d: %v", len(got), got)
	}
}

// TestRankBlockingEquivalenceCorpus runs the same proof over calque's own Go
// corpus — the realistic mix of anchor channels and scores. Also asserts the
// index actually blocks (fewer candidates than the full L×R product).
func TestRankBlockingEquivalenceCorpus(t *testing.T) {
	sigs, _, err := Extract(".", []string{"*.py", "**/*.py"})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(sigs) < 20 {
		t.Fatalf("corpus too small to be a meaningful test: %d funcs", len(sigs))
	}

	got := normalize(Rank(sigs, sigs, SizeGate{MinLines: 4}, 0.18, 1<<30, false))
	want := normalize(naiveRank(sigs, sigs, 4, 0.18, 1<<30))
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("blocking Rank != naive Rank on corpus: %d vs %d pairs", len(got), len(want))
	}

	// Evidence of the win: candidates generated << naive L×R visits.
	var kept []*FuncSig
	for _, f := range sigs {
		if f.NLines >= 4 && !strings.HasPrefix(f.Name, "__") {
			kept = append(kept, f)
		}
	}
	cands := len(candidatePairs(kept, kept, anchorChannels))
	naive := len(kept) * len(kept)
	if cands >= naive {
		t.Fatalf("blocking did not reduce work: %d candidates vs %d naive visits", cands, naive)
	}
	t.Logf("blocking: %d candidates vs %d naive L×R visits (%.1f%% of pairs scored)",
		cands, naive, 100*float64(cands)/float64(naive))
}

// TestCandidatePairsEmptyChannels: a function with no tokens in any anchor channel
// indexes nowhere and can never anchor, so it generates no candidates.
func TestCandidatePairsEmptyChannels(t *testing.T) {
	a := fsig("a.go", "x", nil, nil, nil, nil) // name "x": one-letter, but stem is {x}
	blank := &FuncSig{File: "b.go", Qualname: "", Name: "", NLines: 10}
	blank.Prepare() // empty name -> empty stem; no strings/writes/ret
	if got := candidatePairs([]*FuncSig{blank}, []*FuncSig{a}, anchorChannels); len(got) != 0 {
		t.Fatalf("a token-less function must generate no candidates, got %d", len(got))
	}
}
