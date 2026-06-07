// Package glob compiles path globs to anchored regexps and matches paths against
// a set of them. Single-sourced (the code axis and the prose corpus walker both
// need it) — calque must not carry the duplication it exists to detect.
package glob

import (
	"regexp"
	"strings"
)

// ToRegexp compiles a path glob into an anchored regexp. `**/` matches zero or
// more leading directories; `**` matches across separators; `*` and `?` do not
// cross `/`.
func ToRegexp(g string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(g); i++ {
		c := g[i]
		switch c {
		case '*':
			if i+1 < len(g) && g[i+1] == '*' {
				i++ // consume the second '*'
				if i+1 < len(g) && g[i+1] == '/' {
					i++ // consume the '/'
					b.WriteString("(?:.*/)?")
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '[', ']', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

// Compile turns a list of globs into regexps, skipping blanks and invalid globs.
func Compile(globs []string) []*regexp.Regexp {
	var res []*regexp.Regexp
	for _, g := range globs {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		if re, err := ToRegexp(g); err == nil {
			res = append(res, re)
		}
	}
	return res
}

// MatchAny reports whether path matches any of the compiled patterns.
func MatchAny(res []*regexp.Regexp, path string) bool {
	for _, re := range res {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}
