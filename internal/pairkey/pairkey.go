// Package pairkey computes the canonical key for an unordered pair of strings,
// so {a,b} and {b,a} map to the same key. Single-sourced because calque's own
// self-scan flagged duplicate copies of this helper in internal/code and
// internal/registry — exactly the duplication calque exists to kill.
package pairkey

import (
	"sort"
	"strings"
)

// Key returns a stable key for the unordered pair {a, b}.
func Key(a, b string) string {
	if a <= b {
		return a + "\x00" + b
	}
	return b + "\x00" + a
}

// SetKey returns a stable key for an unordered SET of strings — the N-ary
// generalization of Key, used to identify a touchpoint cluster (a set of
// functions sharing one private seam) independent of member order. Duplicates
// are collapsed so {a,a,b} and {a,b} map to the same key.
func SetKey(members []string) string {
	if len(members) == 0 {
		return ""
	}
	uniq := make([]string, 0, len(members))
	seen := make(map[string]struct{}, len(members))
	for _, m := range members {
		if _, dup := seen[m]; dup {
			continue
		}
		seen[m] = struct{}{}
		uniq = append(uniq, m)
	}
	sort.Strings(uniq)
	return strings.Join(uniq, "\x00")
}
