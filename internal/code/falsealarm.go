package code

import "strings"

// FalseAlarmHint returns a short STRUCTURAL tag for a suspect pair when it matches
// a shape that is usually a false alarm rather than drift. Advisory only — it never
// gates anything (the pair already surfaced); it just names the structure inline so
// an adjudicating agent or human triages faster. "" when nothing matches. calque
// does not decide drift-vs-twin here; it surfaces the fact and the SKILL.md
// "reading the output" guide explains why each shape is usually noise.
//
//   - "same-receiver": both functions are methods on the SAME receiver type. Two
//     methods on one type naturally share that type's fields, so field/name overlap
//     between them is expected, not a contract collision.
//   - "field-copy": both functions are projection / DTO mappers — each writes back
//     (most of) the same fields it reads (copying struct A's fields onto struct B)
//     rather than deriving a new quantity, so a shared field-set is structural.
//
// same-receiver takes precedence when both apply (it's the stronger structural
// fact). Both are deliberately conservative so a real twin is rarely mislabeled.
func FalseAlarmHint(a, b *FuncSig) string {
	if sameReceiver(a, b) {
		return "same-receiver"
	}
	if fieldCopy(a) && fieldCopy(b) {
		return "field-copy"
	}
	return ""
}

// sameReceiver reports whether a and b are methods on the same receiver/type — the
// "Type." prefix of their qualname matches and is non-empty. A free function
// (qualname with no ".") never matches.
func sameReceiver(a, b *FuncSig) bool {
	ra := receiverOf(a.Qualname)
	return ra != "" && ra == receiverOf(b.Qualname)
}

func receiverOf(qual string) string {
	if i := strings.LastIndex(qual, "."); i > 0 {
		return qual[:i]
	}
	return ""
}

// fieldCopy reports whether a function looks like a projection / DTO mapper: it
// writes back (a majority of) the same field-paths it reads, rather than deriving a
// new quantity from them. Proxy over the root-dropped dotted forms both signals
// already carry; requires at least two writes (one copied field is too thin to
// call) and that ≥60% of written paths also appear among the reads.
func fieldCopy(f *FuncSig) bool {
	if len(f.sWrite) < 2 {
		return false
	}
	var shared int
	for w := range f.sWrite {
		if f.sRead.has(w) {
			shared++
		}
	}
	return float64(shared)/float64(len(f.sWrite)) >= 0.6
}
