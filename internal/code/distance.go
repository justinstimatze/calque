package code

import (
	"fmt"
	"path/filepath"
	"strings"
)

// distanceBoostEnabled gates whether scorePair applies distanceBoost. Defaults
// to false (matches current scoring exactly); SetDistanceBoost flips it from
// the CLI's --distance-boost flag. Mirrors activeWeights/UseWeights/
// ResetWeights: a package-level toggle rather than a threaded parameter,
// since scorePair is called from more than just Rank (NameStemCandidates and
// SignatureCandidates also call it, using its Score as a sort tiebreak) and a
// bool would otherwise need to thread through every one of those call sites
// plus the naiveRank test oracle for what should be a single on/off switch.
var distanceBoostEnabled bool

// SetDistanceBoost turns the distance-decay score boost on or off. Tests that
// enable it must call SetDistanceBoost(false) afterward — the same
// leak-between-cases discipline ResetWeights already exists to guard.
func SetDistanceBoost(enabled bool) { distanceBoostEnabled = enabled }

// Boost caps and saturation points. Deliberately small and bounded (see
// distanceBoost) so distance only ever corroborates an already-anchored score,
// never dominates it. Starting points mirroring slopo.dev's own bounds;
// tunable if a live self-scan shows them off.
const (
	sameFileBoostCap      = 0.10
	sameFileSaturateLines = 300.0
	crossFileBoostCap     = 0.15
	crossFileSaturateHops = 4.0
)

// distanceBoost returns a multiplier in [1.0, 1.15] reflecting how surprising
// it is that a and b converged, given their physical distance in the
// codebase: two near-identical functions sitting far apart (different
// directories weighted more than far-apart lines in one file) are a stronger,
// less-likely-intentional drift signal than two sitting right next to each
// other (often deliberate — overloads, table-driven siblings). Pure function
// of File/Line, both already on FuncSig — no new field needed.
func distanceBoost(a, b *FuncSig) float64 {
	if a.File == b.File {
		d := a.Line - b.Line
		if d < 0 {
			d = -d
		}
		return 1.0 + capFrac(float64(d)/sameFileSaturateLines)*sameFileBoostCap
	}
	hops := dirHops(a.File, b.File)
	return 1.0 + capFrac(float64(hops)/crossFileSaturateHops)*crossFileBoostCap
}

func capFrac(f float64) float64 {
	if f > 1 {
		return 1
	}
	return f
}

// distBoostReason renders a Reason() suffix for a fired distance boost, or ""
// when boost is 1.0 (no-op — the toggle is off, or the pair sits close
// together). Kept out of the signals/signalDef table in score.go since it's a
// post-hoc multiplier on the combined score, not a per-channel jaccard/avail
// similarity channel — rendering it the same way would be misleading.
func distBoostReason(a, b *FuncSig, boost float64) string {
	if boost <= 1.0 {
		return ""
	}
	if a.File == b.File {
		d := a.Line - b.Line
		if d < 0 {
			d = -d
		}
		return fmt.Sprintf("dist-boost=%.2fx(same-file, %d lines)", boost, d)
	}
	hops := dirHops(a.File, b.File)
	return fmt.Sprintf("dist-boost=%.2fx(cross-dir, %d hops)", boost, hops)
}

// dirHops is a tree/LCA distance between two relative file paths: the number
// of directory segments unique to each side once their shared prefix is
// removed. Two files in the same directory are 0 hops; siblings under one
// shared parent are 2 (up one, down one); files in unrelated trees sum both
// sides' full depth.
func dirHops(fileA, fileB string) int {
	segsA := strings.Split(filepath.ToSlash(filepath.Dir(fileA)), "/")
	segsB := strings.Split(filepath.ToSlash(filepath.Dir(fileB)), "/")
	common := 0
	for common < len(segsA) && common < len(segsB) && segsA[common] == segsB[common] {
		common++
	}
	return (len(segsA) - common) + (len(segsB) - common)
}
