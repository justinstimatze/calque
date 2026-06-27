package code

import (
	"strings"
)

// Nearest returns the corpus functions most likely to be dual-path twins of
// query, ranked by suspicion score — the author-time inverse of Rank's
// after-the-fact boundary scan. It answers "before I write this, does something
// already occupy its seam?" so a pre-write hook can surface the twin while the
// DRY-vs-write-new call is still cheap.
//
// Unlike Rank it does NOT apply the minLines floor to query itself — an
// author-time query is often a short stub — only to the corpus side, preserving
// the corpus quality bar; and it skips the query's own Key, so running it on an
// already-indexed function never matches itself. Substrate-agnostic: query and
// corpus are plain FuncSigs, so this serves every language calque extracts
// (Go, TypeScript, Python), not only defn-indexed Go. top<=0 returns all matches.
func Nearest(query *FuncSig, corpus []*FuncSig, minLines int, minScore float64, top int, includeTests bool) []Suspicion {
	qkey := query.Key()
	var right []*FuncSig
	for _, f := range corpus {
		if f.NLines >= minLines && !strings.HasPrefix(f.Name, "__") && f.Key() != qkey {
			right = append(right, f)
		}
	}
	if top <= 0 {
		top = len(right) + 1
	}
	return scoreAndRank(candidatePairs([]*FuncSig{query}, right, anchorChannels), minScore, top, includeTests)
}
