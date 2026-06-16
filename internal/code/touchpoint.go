package code

// Touchpoint clustering — the N-ary recall pass (DESIGN_NOTES §15).
//
// Pairwise scoring (score.go) compares whole-function signatures, so it sees the
// easy "name twins" but misses the expensive case: a small shared block inlined
// into several large, differently-named functions. The canonical miss is a
// triple-shell input path — three entry methods (two `step`s and a `run`) each
// inlined `[_parse_cmd -> read/clear _canon_cache -> dispatch]`; the few seam
// tokens are swamped in big bodies, and the names share no stem, so pairwise
// Jaccard scores them below threshold and, being pairwise, can't express a trio
// at all.
//
// This pass inverts the problem. It builds an index of *seam symbols* (leading-
// underscore / unexported call, write, or getattr-string names, plus SCREAMING_SNAKE
// domain constants) -> the set of functions that touch each. A symbol touched by a
// SMALL number of functions (2..MaxFanout) is a shared internal seam: those functions
// do the same internal job. This is presence-based, so it survives the dilution that
// defeats Jaccard, and it needs no naming convention beyond "private" — strictly
// more robust than the name-stem signal. Output is a CLUSTER {members, shared
// seam symbols}, the N-ary generalization of a suspect pair.
//
// The domain-constant channel (isDomainConst) is orthogonal to the call/string/write
// channels: it catches the "same computation, different access pattern" twin — two
// functions deriving one concept through inverted broadphases share NO read-set or
// call-set, but both reference the same named magic values (V_BELOW, V_ROOF). That
// twin is invisible to the reads axis by construction; the shared constant is the
// only positive signal linking them.

import (
	"regexp"
	"sort"
	"strings"

	"github.com/justinstimatze/calque/internal/pairkey"
)

// SeamSymbol is one private symbol shared across a cluster's members.
type SeamSymbol struct {
	Name   string  // the seam, e.g. "_parse_cmd"
	Fanout int     // how many in-scope functions touch it (2..MaxFanout)
	Rarity float64 // 1/Fanout, optionally private-boosted — higher = rarer = stronger
}

// Cluster is a set of functions sharing one or more private seam symbols — the
// N-ary suspect (a 2-member cluster is a pair the pairwise pass may have missed
// to dilution; an N>=3 cluster is a shape pairwise cannot express at all).
type Cluster struct {
	Members []*FuncSig
	Shared  []SeamSymbol // strongest (rarest) first
	Score   float64      // sum of shared-symbol Rarity
}

// MemberKeys returns the cluster's member keys (for registry lines / lookup).
func (c Cluster) MemberKeys() []string {
	keys := make([]string, len(c.Members))
	for i, m := range c.Members {
		keys[i] = m.Key()
	}
	return keys
}

// Key is the canonical, order-independent identity of the cluster's member set.
func (c Cluster) Key() string { return pairkey.SetKey(c.MemberKeys()) }

// ClusterOptions tune the touchpoint pass. Zero value is not valid; use
// DefaultClusterOptions and override.
type ClusterOptions struct {
	MinLines   int     // ignore functions shorter than this (dilution-prone noise)
	MinMembers int     // smallest cluster to report (3 keeps it additive to pairs)
	MaxFanout  int     // a symbol touched by more than this is plumbing, not a seam
	MinScore   float64 // drop clusters below this combined rarity
	Top        int     // cap the report
}

// DefaultClusterOptions: report N>=3 clusters (the validated gap pairwise can't
// express), treating a symbol touched by 2..8 functions as a candidate seam.
func DefaultClusterOptions() ClusterOptions {
	return ClusterOptions{MinLines: 4, MinMembers: 3, MaxFanout: 8, MinScore: 0.40, Top: 30}
}

const privateBoost = 1.6 // leading-underscore / unexported seams are the strong tell

var identShaped = regexp.MustCompile(`^_?[a-zA-Z][a-zA-Z0-9_]*$`)

// seamSymbols collects the private/internal symbols a function touches, pooled
// across channels (a seam shows up as a call in one shell and a getattr-string
// or write in another — pooling by name is exactly why this beats per-channel
// Jaccard). Returns identifier-shaped, private symbols only.
func (f *FuncSig) seamSymbols(projectDefs set) set {
	out := set{}
	add := func(s string) {
		if isSeam(s) {
			out[s] = struct{}{}
		}
	}
	for _, c := range f.Calls {
		// An external call — a std / extern-crate method (read_to_end, parent, open,
		// sort_by_key) — is NOT a shared private seam, but its snake_case leaf name
		// passes isSeam's lower-first test and forms false clusters of unrelated
		// fetchers (camber C6/C11/C16). A non-underscore call-name counts only if it
		// resolves to a project-defined symbol; leading-underscore names are
		// unambiguously project-private (std uses none) and pass unconditionally.
		if !isPrivate(c) && !projectDefs.has(c) {
			continue
		}
		add(c)
	}
	for _, s := range f.Strings {
		if identShaped.MatchString(s) {
			add(s)
		}
	}
	for _, w := range f.Writes {
		// a write target is a dotted/indexed path; each component can be a seam
		for _, comp := range strings.FieldsFunc(w, func(r rune) bool { return r == '.' || r == '[' || r == ']' }) {
			add(comp)
		}
	}
	// Domain constants are a fourth seam channel: a SCREAMING_SNAKE name fails the
	// lower-first/underscore isSeam test, but a magic value like V_BELOW shared by a
	// few functions is exactly the rare touchpoint this pass clusters on — and it
	// catches the "same computation, different access pattern" twin that reads (and
	// the call/string/write channels) miss because the shared signal is the constant,
	// not the field-path. commonConsts drops universal std/library values (TAU, NAN,
	// MAX) whose sharing is incidental, mirroring commonIdents on the call channel.
	for _, c := range f.Consts {
		if isDomainConst(c) && !commonConsts.has(c) {
			out[c] = struct{}{}
		}
	}
	return out
}

// commonConsts are language-universal std/library constants that pass the
// SCREAMING_SNAKE shape test but are NOT domain vocabulary — every codebase touches
// TAU/NAN/MAX incidentally, so clustering on them is noise (the const analog of
// commonIdents). Kept tight + std-only; grows via dogfood the way commonIdents did.
var commonConsts = set{
	// Float math / sentinels (Rust std::f64::consts, JS Math, Python math/float)
	"TAU": {}, "NAN": {}, "INF": {}, "INFINITY": {}, "NEG_INFINITY": {},
	"EPSILON": {}, "SQRT_2": {}, "LN_2": {}, "LN_10": {}, "LOG2_E": {}, "LOG10_E": {},
	"FRAC_PI_2": {}, "FRAC_PI_3": {}, "FRAC_PI_4": {}, "FRAC_PI_6": {}, "FRAC_PI_8": {},
	"FRAC_1_PI": {}, "FRAC_1_SQRT_2": {},
	// Numeric limits
	"MIN": {}, "MAX": {}, "MIN_VALUE": {}, "MAX_VALUE": {},
	"MAX_SAFE_INTEGER": {}, "MIN_SAFE_INTEGER": {}, "MIN_POSITIVE": {},
	// Generic enum/flag conventions (not domain vocabulary)
	"ALL": {}, "NONE": {}, "DEFAULT": {}, "EMPTY": {},
	// Go std: os open-flags + time layout names + format/encoding tags surface as
	// SCREAMING references but are library plumbing, not domain vocabulary (recon: they
	// clustered file-opening / timestamp / serialization helpers across unrelated repos).
	"O_RDONLY": {}, "O_WRONLY": {}, "O_RDWR": {}, "O_APPEND": {}, "O_CREATE": {},
	"O_EXCL": {}, "O_SYNC": {}, "O_TRUNC": {},
	"RFC3339": {}, "RFC3339NANO": {}, "RFC1123": {}, "RFC822": {}, "ANSIC": {},
	// JS/TS global objects that match SCREAMING_SNAKE when referenced (JSON.parse, …)
	"JSON": {}, "YAML": {}, "TOML": {}, "CSV": {}, "XML": {}, "HTML": {}, "UTF8": {},
}

// commonIdents are language builtins, predeclared types, and ubiquitous helpers
// that pass the lower-first "unexported-looking" test but are NOT hand-written
// seams (everyone uses them) — they would otherwise form spurious clusters.
var commonIdents = set{
	// Go builtins / predeclared
	"string": {}, "rune": {}, "byte": {}, "bool": {}, "error": {}, "int": {},
	"int8": {}, "int16": {}, "int32": {}, "int64": {}, "uint": {}, "uint8": {},
	"uint16": {}, "uint32": {}, "uint64": {}, "uintptr": {}, "float32": {},
	"float64": {}, "complex64": {}, "complex128": {}, "make": {}, "new": {},
	"append": {}, "copy": {}, "delete": {}, "len": {}, "cap": {}, "close": {},
	"panic": {}, "recover": {}, "print": {}, "println": {}, "real": {}, "imag": {},
	"complex": {}, "true": {}, "false": {}, "nil": {}, "iota": {}, "range": {},
	// Python builtins / ubiquitous helpers
	"isinstance": {}, "issubclass": {}, "getattr": {}, "setattr": {}, "hasattr": {},
	"enumerate": {}, "super": {}, "list": {}, "dict": {}, "tuple": {}, "set": {},
	"str": {}, "repr": {}, "sorted": {}, "reversed": {}, "zip": {}, "map": {},
	"filter": {}, "format": {}, "join": {}, "split": {}, "strip": {}, "items": {},
	"keys": {}, "values": {}, "append_": {}, "extend": {}, "update": {},
}

// isSeam reports whether a symbol is a private internal seam: a leading-
// underscore name (Python/dunder-stripped private) or an unexported-looking
// lower-first identifier (Go). Dunders, builtins, and trivially short names are
// excluded.
func isSeam(s string) bool {
	if !identShaped.MatchString(s) {
		return false
	}
	if strings.HasPrefix(s, "__") {
		return false // dunder / name-mangled — not a hand-written seam
	}
	if strings.HasPrefix(s, "_") {
		return len(s) >= 3 // _x is too short to be meaningful
	}
	if commonIdents.has(s) {
		return false // builtin / predeclared / ubiquitous — not a seam
	}
	// unexported-looking (lower-first) identifiers are Go's private convention;
	// require length to avoid one-letter locals/builtins like i, x, len.
	r := s[0]
	return r >= 'a' && r <= 'z' && len(s) >= 4
}

func isPrivate(s string) bool { return strings.HasPrefix(s, "_") }

var screamingSnake = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// isDomainConst reports whether s is a SCREAMING_SNAKE domain constant name
// (V_BELOW, MAX_RETRIES, GRID) — the cross-language convention for a named domain
// magic value. Two functions that compute the same concept through different access
// patterns (an inverted broadphase) share NO read-set yet both reference the same
// domain constants, so this is an N-ary seam channel orthogonal to reads. Go's
// MixedCaps const convention yields few of these without type resolution, so the
// channel is strongest on Rust/Python/TS; that asymmetry is intentional.
func isDomainConst(s string) bool {
	return len(s) >= 3 && screamingSnake.MatchString(s)
}

// ClusterByTouchpoint finds N-ary suspect clusters: sets of functions sharing
// one or more rare private seam symbols. See the package doc for why.
func ClusterByTouchpoint(sigs []*FuncSig, opts ClusterOptions) []Cluster {
	// Scope: real functions only (mirror Rank's keep filter).
	var pool []*FuncSig
	for _, f := range sigs {
		if f.NLines >= opts.MinLines && !strings.HasPrefix(f.Name, "__") {
			pool = append(pool, f)
		}
	}
	n := float64(len(pool))
	if n < 2 {
		return nil
	}

	// Project-defined symbol names (every extracted function/method leaf), so a
	// call seam can be required to resolve to one — excluding std / extern-crate
	// methods that otherwise masquerade as private seams. Built from all sigs (not
	// just the >=MinLines pool) so a short helper is still a resolvable call target.
	projectDefs := set{}
	for _, f := range sigs {
		if f.Name != "" {
			projectDefs[f.Name] = struct{}{}
		}
	}

	// Inverted index: seam symbol -> functions touching it (deduped by Key, so a
	// self-scan where a fn appears on both sides doesn't double-count).
	index := map[string][]*FuncSig{}
	for _, f := range pool {
		seen := map[string]bool{}
		for sym := range f.seamSymbols(projectDefs) {
			k := f.Key()
			if seen[sym+"\x00"+k] {
				continue
			}
			seen[sym+"\x00"+k] = true
			index[sym] = append(index[sym], f)
		}
	}

	// Candidate seam groups: a symbol touched by 2..MaxFanout functions.
	type group struct {
		sym     SeamSymbol
		members []*FuncSig
	}
	var groups []group
	for sym, members := range index {
		members = dedupSigs(members)
		fanout := len(members)
		if fanout < 2 || fanout > opts.MaxFanout {
			continue
		}
		// Rarity is kept repo-size-independent on purpose (1/fanout, not a global
		// IDF) so MinScore means the same thing on a 50-func and a 10k-func repo.
		rarity := 1.0 / float64(fanout)
		if isPrivate(sym) {
			rarity *= privateBoost
		}
		groups = append(groups, group{
			sym:     SeamSymbol{Name: sym, Fanout: fanout, Rarity: rarity},
			members: members,
		})
	}

	// Coalesce: process largest member-sets first; fold a group whose members are
	// a SUBSET of an already-formed cluster into that cluster (its symbol is shared
	// by a subset, still relevant). Subset-merge is deterministic and avoids the
	// transitive-chain blow-up a naive union-find would cause.
	sort.Slice(groups, func(i, j int) bool {
		if len(groups[i].members) != len(groups[j].members) {
			return len(groups[i].members) > len(groups[j].members)
		}
		return groups[i].sym.Name < groups[j].sym.Name
	})

	var clusters []*Cluster
	for _, g := range groups {
		gset := keySet(g.members)
		var host *Cluster
		for _, c := range clusters {
			if subsetOf(gset, keySet(c.Members)) {
				host = c
				break
			}
		}
		if host != nil {
			host.Shared = append(host.Shared, g.sym)
			host.Score += g.sym.Rarity
		} else {
			clusters = append(clusters, &Cluster{
				Members: g.members,
				Shared:  []SeamSymbol{g.sym},
				Score:   g.sym.Rarity,
			})
		}
	}

	var out []Cluster
	for _, c := range clusters {
		if len(c.Members) < opts.MinMembers || c.Score < opts.MinScore {
			continue
		}
		sort.Slice(c.Shared, func(i, j int) bool {
			if c.Shared[i].Rarity != c.Shared[j].Rarity {
				return c.Shared[i].Rarity > c.Shared[j].Rarity
			}
			return c.Shared[i].Name < c.Shared[j].Name
		})
		sort.Slice(c.Members, func(i, j int) bool { return c.Members[i].Key() < c.Members[j].Key() })
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Key() < out[j].Key()
	})
	if opts.Top > 0 && len(out) > opts.Top {
		out = out[:opts.Top]
	}
	return out
}

// Reason renders a cluster's shared seams, strongest first.
func (c Cluster) Reason() string {
	bits := make([]string, 0, len(c.Shared))
	for _, s := range c.Shared {
		bits = append(bits, s.Name)
	}
	return "shared seam(s): " + strings.Join(bits, ", ")
}

func dedupSigs(fs []*FuncSig) []*FuncSig {
	seen := map[string]bool{}
	out := fs[:0]
	for _, f := range fs {
		if seen[f.Key()] {
			continue
		}
		seen[f.Key()] = true
		out = append(out, f)
	}
	return out
}

func keySet(fs []*FuncSig) set {
	s := make(set, len(fs))
	for _, f := range fs {
		s[f.Key()] = struct{}{}
	}
	return s
}

func subsetOf(a, b set) bool {
	if len(a) > len(b) {
		return false
	}
	for x := range a {
		if !b.has(x) {
			return false
		}
	}
	return true
}
