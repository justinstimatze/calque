// Package registry parses .calque/registry.md — calque's durable, git-tracked
// memory of adjudicated dual-path pairs. The check gate consults it to suppress
// already-judged pairs (so it surfaces only NEW drift, not the whole scan) and
// to reconcile STALE entries (pairs whose referenced code no longer exists).
//
// The file stays human-readable markdown; each adjudicated pair just carries two
// machine lines (order-independent, anywhere in the entry):
//
//   - pair: <file::qualname> | <file::qualname>
//   - verdict: drift | contracted-twin-ok | false-alarm
//   - reviewed: <date>            (optional)
//
// Titles and notes around them are freeform. Recency is handled by `reviewed`
// (re-verification cadence) + liveness reconciliation — never age-based eviction
// (evicting an old false-alarm would just resurface the noise).
package registry

import (
	"bufio"
	"os"
	"strings"

	"github.com/justinstimatze/calque/internal/pairkey"
)

// Entry is one adjudicated pair.
type Entry struct {
	Key1, Key2 string
	Verdict    string // raw; use VerdictClass for the leading token
	Reviewed   string
}

// VerdictClass is the leading word of Verdict (e.g. "drift" from "drift (open)").
func (e Entry) VerdictClass() string {
	if f := strings.Fields(e.Verdict); len(f) > 0 {
		return f[0]
	}
	return ""
}

// Registry is the parsed set of adjudicated pairs.
type Registry struct {
	Path    string
	Entries []Entry
	byPair  map[string]int
}

// Load parses the registry at path. A missing file is not an error (returns an
// empty registry) — a repo need not have adjudicated anything yet.
func Load(path string) (*Registry, error) {
	r := &Registry{Path: path, byPair: map[string]int{}}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil
		}
		return r, err
	}
	defer f.Close()

	var cur *Entry
	flush := func() {
		if cur != nil && cur.Key1 != "" && cur.Key2 != "" && cur.Verdict != "" {
			if _, dup := r.byPair[pairkey.Key(cur.Key1, cur.Key2)]; !dup {
				r.byPair[pairkey.Key(cur.Key1, cur.Key2)] = len(r.Entries)
				r.Entries = append(r.Entries, *cur)
			}
		}
		cur = nil
	}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "- pair:"):
			flush()
			v := strings.TrimSpace(strings.TrimPrefix(line, "- pair:"))
			if k1, k2, ok := strings.Cut(v, "|"); ok {
				cur = &Entry{Key1: cleanKey(k1), Key2: cleanKey(k2)}
			}
		case strings.HasPrefix(line, "- verdict:") && cur != nil:
			cur.Verdict = strings.TrimSpace(strings.TrimPrefix(line, "- verdict:"))
		case strings.HasPrefix(line, "- reviewed:") && cur != nil:
			cur.Reviewed = strings.TrimSpace(strings.TrimPrefix(line, "- reviewed:"))
		}
	}
	flush()
	return r, sc.Err()
}

func cleanKey(s string) string { return strings.TrimSpace(strings.Trim(strings.TrimSpace(s), "`")) }

// Has reports whether the unordered pair {k1,k2} is adjudicated.
func (r *Registry) Has(k1, k2 string) bool {
	_, ok := r.byPair[pairkey.Key(k1, k2)]
	return ok
}

// Lookup returns the entry for an unordered pair.
func (r *Registry) Lookup(k1, k2 string) (Entry, bool) {
	if i, ok := r.byPair[pairkey.Key(k1, k2)]; ok {
		return r.Entries[i], true
	}
	return Entry{}, false
}
