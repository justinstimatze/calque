// Package pairkey computes the canonical key for an unordered pair of strings,
// so {a,b} and {b,a} map to the same key. Single-sourced because calque's own
// self-scan flagged duplicate copies of this helper in internal/code and
// internal/registry — exactly the duplication calque exists to kill.
package pairkey

// Key returns a stable key for the unordered pair {a, b}.
func Key(a, b string) string {
	if a <= b {
		return a + "\x00" + b
	}
	return b + "\x00" + a
}
