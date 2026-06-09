package code

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/justinstimatze/calque/internal/glob"
)

// Role-implementer predicates — the matching half of the role-cardinality axis
// (DESIGN_NOTES §18). A role is DECLARED by a predicate; Implementers enumerates the
// functions that satisfy it, and the `cardinality` gate compares that count against the
// declared expectation. This inverts calque's usual mechanism: instead of discovering
// similar pairs after the fact, the author states "role R has one implementation" up
// front and the predicate finds every claimant — so it catches dual paths that share no
// footprint (a stub vs the real call site; two different-API backends), which pairwise
// similarity cannot.
//
// A predicate is an AND of whitespace-separated terms, each testing one FuncSig field:
//
//	name:/regex/    f.Name matches the (Go) RE2 regex
//	qual:/regex/    f.Qualname ("Type.Method" or bare func) matches
//	file:<glob>     f.File matches the glob (gitignore-ish, via internal/glob)
//	calls:<sym>     f.Calls contains the leaf name
//	writes:<field>  f.Writes contains the dotted write target
//	emits:<str>     f.Strings contains the emitted literal
//	returns:<key>   f.RetKeys contains the returned map/struct key
//
// Example: `name:/[Cc]onstruct/ calls:_dispatch` — every constructor-ish function that
// reaches the dispatch seam.

// Predicate is a parsed, AND-composed role-implementer matcher.
type Predicate struct {
	terms []predTerm
}

type predTerm struct {
	kind string
	re   *regexp.Regexp // name|qual|file (file compiled from a glob)
	lit  string         // calls|writes|emits|returns (exact leaf match)
}

// ParsePredicate parses an AND-composed predicate expression. An empty expression or an
// unrecognized/blank term is an error, so a malformed role declaration is reported
// rather than silently matching nothing.
func ParsePredicate(expr string) (*Predicate, error) {
	fields := strings.Fields(expr)
	if len(fields) == 0 {
		return nil, fmt.Errorf("empty predicate")
	}
	p := &Predicate{}
	for _, f := range fields {
		kind, arg, ok := strings.Cut(f, ":")
		if !ok || kind == "" || arg == "" {
			return nil, fmt.Errorf("bad predicate term %q (want kind:value)", f)
		}
		t := predTerm{kind: kind}
		switch kind {
		case "name", "qual":
			re, err := regexp.Compile(strings.Trim(arg, "/"))
			if err != nil {
				return nil, fmt.Errorf("bad regex in %q: %w", f, err)
			}
			t.re = re
		case "file":
			re, err := glob.ToRegexp(arg)
			if err != nil {
				return nil, fmt.Errorf("bad glob in %q: %w", f, err)
			}
			t.re = re
		case "calls", "writes", "emits", "returns":
			t.lit = arg
		default:
			return nil, fmt.Errorf("unknown predicate kind %q in %q", kind, f)
		}
		p.terms = append(p.terms, t)
	}
	return p, nil
}

// Matches reports whether f satisfies every term (AND). It does NOT apply the
// delegation filter — callers that count implementations use Implementers, which does.
func (p *Predicate) Matches(f *FuncSig) bool {
	for _, t := range p.terms {
		if !t.matches(f) {
			return false
		}
	}
	return true
}

func (t predTerm) matches(f *FuncSig) bool {
	switch t.kind {
	case "name":
		return t.re.MatchString(f.Name)
	case "qual":
		return t.re.MatchString(f.Qualname)
	case "file":
		return t.re.MatchString(f.File)
	case "calls":
		return containsStr(f.Calls, t.lit)
	case "writes":
		return containsStr(f.Writes, t.lit)
	case "emits":
		return containsStr(f.Strings, t.lit)
	case "returns":
		return containsStr(f.RetKeys, t.lit)
	}
	return false
}

func containsStr(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

// Match parses pred and reports whether f satisfies it, excluding delegating wrappers
// (see Implementers for why).
func Match(pred string, f *FuncSig) (bool, error) {
	p, err := ParsePredicate(pred)
	if err != nil {
		return false, err
	}
	return !f.Delegates && p.Matches(f), nil
}

// Implementers returns the FuncSigs that satisfy pred. Delegating wrappers
// (f.Delegates — a body that forwards to a wrapped impl) are excluded: an adapter that
// forwards to the real path is not an independent implementation of a role, so counting
// it would inflate the cardinality with thin wrappers (the §12 delegation insight). A
// malformed predicate yields an error and no matches.
func Implementers(pred string, sigs []*FuncSig) ([]*FuncSig, error) {
	p, err := ParsePredicate(pred)
	if err != nil {
		return nil, err
	}
	var out []*FuncSig
	for _, f := range sigs {
		if !f.Delegates && p.Matches(f) {
			out = append(out, f)
		}
	}
	return out, nil
}
