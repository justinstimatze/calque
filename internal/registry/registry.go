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

// ClusterEntry is one adjudicated N-ary cluster (a set of >=2 function keys that
// share a private seam — the touchpoint pass's unit). Keyed on the member SET,
// order-independent, so it survives the members being reported in any order.
type ClusterEntry struct {
	Keys     []string
	Verdict  string
	Reviewed string
}

// VerdictClass is the leading word of Verdict.
func (e ClusterEntry) VerdictClass() string { return verdictClass(e.Verdict) }

// verdictClass returns the leading word of a verdict ("drift" from "drift (open)").
// Single-sourced so Entry and ClusterEntry can't drift — calque eats its dogfood.
func verdictClass(verdict string) string {
	if f := strings.Fields(verdict); len(f) > 0 {
		return f[0]
	}
	return ""
}

// VerdictClass is the leading word of Verdict (e.g. "drift" from "drift (open)").
func (e Entry) VerdictClass() string { return verdictClass(e.Verdict) }

// Registry is the parsed set of adjudicated pairs and N-ary clusters.
type Registry struct {
	Path       string
	Entries    []Entry
	Clusters   []ClusterEntry
	byPair     map[string]int
	byClusters map[string]int
}

// Load parses the registry at path. A missing file is not an error (returns an
// empty registry) — a repo need not have adjudicated anything yet.
func Load(path string) (*Registry, error) {
	r := &Registry{Path: path, byPair: map[string]int{}, byClusters: map[string]int{}}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil
		}
		return r, err
	}
	defer f.Close()

	// One parser state, since `- verdict:`/`- reviewed:` attach to whichever of a
	// pair or cluster was opened most recently.
	var curPair *Entry
	var curCluster *ClusterEntry
	setVerdict := func(v string) {
		switch {
		case curCluster != nil:
			curCluster.Verdict = v
		case curPair != nil:
			curPair.Verdict = v
		}
	}
	setReviewed := func(v string) {
		switch {
		case curCluster != nil:
			curCluster.Reviewed = v
		case curPair != nil:
			curPair.Reviewed = v
		}
	}
	flush := func() {
		if curPair != nil && curPair.Key1 != "" && curPair.Key2 != "" && curPair.Verdict != "" {
			k := pairkey.Key(curPair.Key1, curPair.Key2)
			if _, dup := r.byPair[k]; !dup {
				r.byPair[k] = len(r.Entries)
				r.Entries = append(r.Entries, *curPair)
			}
		}
		if curCluster != nil && len(curCluster.Keys) >= 2 && curCluster.Verdict != "" {
			k := pairkey.SetKey(curCluster.Keys)
			if _, dup := r.byClusters[k]; !dup {
				r.byClusters[k] = len(r.Clusters)
				r.Clusters = append(r.Clusters, *curCluster)
			}
		}
		curPair, curCluster = nil, nil
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
				curPair = &Entry{Key1: cleanKey(k1), Key2: cleanKey(k2)}
			}
		case strings.HasPrefix(line, "- cluster:"):
			flush()
			v := strings.TrimSpace(strings.TrimPrefix(line, "- cluster:"))
			var keys []string
			for _, part := range strings.Split(v, "|") {
				if k := cleanKey(part); k != "" {
					keys = append(keys, k)
				}
			}
			if len(keys) >= 2 {
				curCluster = &ClusterEntry{Keys: keys}
			}
		case strings.HasPrefix(line, "- verdict:"):
			setVerdict(strings.TrimSpace(strings.TrimPrefix(line, "- verdict:")))
		case strings.HasPrefix(line, "- reviewed:"):
			setReviewed(strings.TrimSpace(strings.TrimPrefix(line, "- reviewed:")))
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

// HasCluster reports whether the member SET {keys...} is adjudicated.
func (r *Registry) HasCluster(keys []string) bool {
	_, ok := r.byClusters[pairkey.SetKey(keys)]
	return ok
}

// LookupCluster returns the entry for a member set.
func (r *Registry) LookupCluster(keys []string) (ClusterEntry, bool) {
	if i, ok := r.byClusters[pairkey.SetKey(keys)]; ok {
		return r.Clusters[i], true
	}
	return ClusterEntry{}, false
}
