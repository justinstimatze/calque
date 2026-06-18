package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/justinstimatze/calque/internal/code"
	"github.com/justinstimatze/calque/internal/llm"
	"github.com/justinstimatze/calque/internal/pairkey"
	"github.com/justinstimatze/calque/internal/registry"
)

// proposal is one candidate role synthesized from an N-ary private-seam cluster —
// the unit `propose-roles` prints. Pure data, separated from rendering + I/O so the
// core (computeProposals) is testable, mirroring roleResult for the cardinality gate.
type proposal struct {
	Cluster   code.Cluster
	Name      string   // kebab role id derived from the synthesized seam
	Predicate string   // synthesized single-term predicate (calls:/consts:/emits:)
	Approx    bool     // no seam cleanly re-selects all members; predicate is best-effort
	Baseline  []string // sorted member Key()s — the frozen ratchet baseline
	Matched   []string // Implementers(Predicate) keys — the self-verification
	Extra     []string // matched keys NOT in the cluster (predicate too broad)
	Missing   []string // member keys NOT matched (predicate too narrow)
}

// runProposeRoles turns the N-ary touchpoint cluster pass into a role-candidate
// proposer: each suspect cluster becomes a paste-ready `- role:` block whose
// predicate is synthesized from the cluster's shared seam and self-verified against
// the matcher. It is a GENERATOR, not a gate — it prints to stdout, writes nothing,
// and never exits non-zero, so it cannot disturb a repo's `check --strict` state.
func runProposeRoles(args []string) {
	fs := flag.NewFlagSet("propose-roles", flag.ContinueOnError)
	repo := fs.String("repo", ".", "repo root to scan")
	exclude := fs.String("exclude", "", "comma-separated glob(s) to skip entirely (e.g. legacy/**,vendor/**)")
	includeTests := fs.Bool("include-tests", false, "keep all-test clusters too (excluded by default — test functions sharing a helper/setup seam cluster as false twins, polluting the Layer D corpus; a cluster mixing test and production members is always kept)")
	regPath := fs.String("registry", ".calque/registry.md", "registry file (dedup vs declared roles / adjudicated clusters)")
	minLines := fs.Int("min-lines", 4, "ignore functions shorter than this many lines")
	minMembers := fs.Int("cluster-min-members", 3, "smallest cluster to propose a role from (2 includes diluted pairs)")
	maxFanout := fs.Int("cluster-max-fanout", 8, "a private symbol touched by more than this is plumbing, not a seam")
	top := fs.Int("top", 30, "max candidate roles to propose")
	judge := fs.Bool("judge", false, "adjudicate each cluster with the LLM oracle (needs ANTHROPIC_API_KEY; records Layer D labels tagged detector=touchpoint, variety=seam channel)")
	twinsOnly := fs.Bool("twins-only", false, "with --judge, print only clusters the oracle confirms as twins")
	channel := fs.String("channel", "", "judge/print only clusters keyed on this seam channel (calls|consts|emits) — focuses a Layer D run on one detector slice")
	if err := fs.Parse(args); err != nil {
		return
	}

	// Whole-file test exclusion is replaced by the asymmetric cluster gate
	// (ClusterByTouchpoint drops all-test clusters, keeps mixed test/prod ones).
	sigs, st, err := code.Extract(*repo, splitCSV(*exclude))
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque propose-roles: walking %s: %v\n", *repo, err)
		os.Exit(1)
	}
	reg, err := registry.Load(joinRepo(*repo, *regPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque propose-roles: reading registry: %v\n", err)
		os.Exit(1)
	}

	copts := clusterOptsFrom(*minLines, *minMembers, *maxFanout, *top)
	copts.IncludeTests = *includeTests
	clusters := code.ClusterByTouchpoint(sigs, copts)
	props := computeProposals(sigs, clusters, reg)
	if *channel != "" {
		var kept []proposal
		for _, p := range props {
			if predChannel(p.Predicate) == *channel {
				kept = append(kept, p)
			}
		}
		props = kept
	}
	if *judge {
		judgeClusters(*repo, props, *twinsOnly)
		return
	}
	fmt.Print(renderProposals(props, st, *regPath))
}

// computeProposals turns each suspect cluster into a candidate role. Pure: no I/O,
// no exit. A cluster is skipped when it is already adjudicated as a `- cluster:`
// verdict, or when an already-declared role's predicate selects the same member set
// (don't re-propose settled ground). For each surviving cluster it synthesizes a
// predicate and self-verifies it by running it back through the matcher.
func computeProposals(sigs []*code.FuncSig, clusters []code.Cluster, reg *registry.Registry) []proposal {
	// Precompute the implementer set-key of every declared role, so a cluster that
	// an existing role already covers isn't proposed again.
	declared := map[string]bool{}
	for _, role := range reg.Roles {
		impls, err := code.Implementers(role.Predicate, sigs)
		if err != nil {
			continue
		}
		keys := make([]string, 0, len(impls))
		for _, f := range impls {
			keys = append(keys, f.Key())
		}
		declared[pairkey.SetKey(keys)] = true
	}

	var out []proposal
	for _, c := range clusters {
		if len(c.Shared) == 0 || len(c.Members) == 0 {
			continue
		}
		memberKeys := make([]string, 0, len(c.Members))
		for _, m := range c.Members {
			memberKeys = append(memberKeys, m.Key())
		}
		if reg.HasCluster(memberKeys) || declared[pairkey.SetKey(memberKeys)] {
			continue
		}

		pred, approx := synthPredicate(c)
		matched, _ := code.Implementers(pred, sigs)

		memberSet := make(map[string]bool, len(memberKeys))
		for _, k := range memberKeys {
			memberSet[k] = true
		}
		matchedSet := make(map[string]bool, len(matched))
		matchedKeys := make([]string, 0, len(matched))
		for _, f := range matched {
			matchedKeys = append(matchedKeys, f.Key())
			matchedSet[f.Key()] = true
		}
		var extra, missing []string
		for _, k := range matchedKeys {
			if !memberSet[k] {
				extra = append(extra, k)
			}
		}
		for _, k := range memberKeys {
			if !matchedSet[k] {
				missing = append(missing, k)
			}
		}

		base := append([]string(nil), memberKeys...)
		sort.Strings(base)
		sort.Strings(extra)
		sort.Strings(missing)
		sort.Strings(matchedKeys)

		out = append(out, proposal{
			Cluster:   c,
			Name:      kebab(strings.TrimLeft(seamName(pred), "_")),
			Predicate: pred,
			Approx:    approx,
			Baseline:  base,
			Matched:   matchedKeys,
			Extra:     extra,
			Missing:   missing,
		})
	}
	return out
}

// synthPredicate picks the single predicate term that best re-selects a cluster's
// members. `calls:`/`emits:`/`consts:` are synthesizable: they test exact membership
// in f.Calls / f.Strings / f.Consts, the same channels seamSymbols pools from.
// (`writes:` matches the full dotted write target, but seamSymbols splits writes into
// components, so a write-component seam has no exact `writes:` term — we don't
// synthesize from it.) Preference: a seam fully covered by Calls, else Consts, else
// Strings, else a best-effort term on the strongest seam in its best channel
// (approx=true; the verify line surfaces the imprecision).
func synthPredicate(c code.Cluster) (pred string, approx bool) {
	n := len(c.Members)
	calls := func(f *code.FuncSig) []string { return f.Calls }
	strs := func(f *code.FuncSig) []string { return f.Strings }
	consts := func(f *code.FuncSig) []string { return f.Consts }

	for _, s := range c.Shared { // rarest-first
		if channelCoverage(c.Members, s.Name, calls) == n {
			return "calls:" + s.Name, false
		}
	}
	for _, s := range c.Shared {
		if channelCoverage(c.Members, s.Name, consts) == n {
			return "consts:" + s.Name, false
		}
	}
	for _, s := range c.Shared {
		if channelCoverage(c.Members, s.Name, strs) == n {
			return "emits:" + s.Name, false
		}
	}
	// Best-effort: the strongest seam in whichever channel covers the most members.
	s := c.Shared[0]
	cc := channelCoverage(c.Members, s.Name, calls)
	ce := channelCoverage(c.Members, s.Name, strs)
	co := channelCoverage(c.Members, s.Name, consts)
	switch {
	case cc >= ce && cc >= co:
		return "calls:" + s.Name, true
	case ce >= co:
		return "emits:" + s.Name, true
	default:
		return "consts:" + s.Name, true
	}
}

// channelCoverage counts how many members carry seam in the given channel.
func channelCoverage(members []*code.FuncSig, seam string, get func(*code.FuncSig) []string) int {
	n := 0
	for _, m := range members {
		for _, v := range get(m) {
			if v == seam {
				n++
				break
			}
		}
	}
	return n
}

// renderProposals prints the candidate role blocks — paste-ready and round-trippable
// by registry.Load (the `- baseline:` line is comma-split there).
func renderProposals(props []proposal, st code.ScanStats, regPath string) string {
	var b strings.Builder
	b.WriteString("# calque — proposed roles (from N-ary private-seam clusters)\n\n")
	fmt.Fprintf(&b, "scanned %d func(s) in %d file(s).\n\n", st.Funcs, st.Files)
	if len(props) == 0 {
		b.WriteString("no role candidates — every private-seam cluster is already declared as a role\n")
		b.WriteString("or adjudicated in the registry, or none were found. ✓\n")
		return b.String()
	}
	fmt.Fprintf(&b, "%d candidate role(s). These are CANDIDATES, not verdicts: calque proposes, you\n", len(props))
	fmt.Fprintf(&b, "adjudicate. Paste the blocks you accept into %s, tighten the\npredicate, then `calque cardinality --strict`.\n\n", regPath)

	for _, p := range props {
		seam := seamName(p.Predicate)
		fmt.Fprintf(&b, "## role: %s   (proposed from a %d-member cluster sharing `%s`)\n", p.Name, len(p.Baseline), seam)
		fmt.Fprintf(&b, "- role: %s\n", p.Name)
		fmt.Fprintf(&b, "- predicate: %s\n", p.Predicate)
		b.WriteString("- expected: 1\n")
		fmt.Fprintf(&b, "- baseline: %s\n", strings.Join(p.Baseline, ", "))

		if len(p.Extra) == 0 && len(p.Missing) == 0 {
			fmt.Fprintf(&b, "# verify: predicate selects %d func(s) — exact match ✓\n", len(p.Matched))
		} else {
			var parts []string
			if len(p.Extra) > 0 {
				parts = append(parts, fmt.Sprintf("also catches %d non-member(s): %s", len(p.Extra), capList(p.Extra, 5)))
			}
			if len(p.Missing) > 0 {
				parts = append(parts, fmt.Sprintf("misses %d member(s): %s", len(p.Missing), capList(p.Missing, 5)))
			}
			fmt.Fprintf(&b, "# verify: predicate selects %d func(s) — %s; tighten before committing.\n", len(p.Matched), strings.Join(parts, "; "))
		}
		if p.Approx {
			b.WriteString("# approximate: no single seam cleanly re-selects all members — best-effort term.\n")
		}
		b.WriteString("# expected:1 asserts these collapse to ONE path. If this is a legitimate N-path,\n")
		fmt.Fprintf(&b, "#   set `- expected: %d` and keep the baseline (the ratchet then flags only NEW members).\n\n", len(p.Baseline))
	}
	return b.String()
}

// seamName returns the value side of a synthesized `kind:value` predicate term.
func seamName(pred string) string {
	if _, v, ok := strings.Cut(pred, ":"); ok {
		return v
	}
	return pred
}

// capList joins keys, truncating to the first n with a "+N more" suffix.
func capList(keys []string, n int) string {
	if len(keys) <= n {
		return strings.Join(keys, ", ")
	}
	return strings.Join(keys[:n], ", ") + fmt.Sprintf(", +%d more", len(keys)-n)
}

// kebab turns a seam symbol into a readable role id: camelCase humps and underscores
// both become hyphens, lowercased (e.g. _resolve_clause → resolve-clause, scorePair →
// score-pair). Only a suggestion — the human renames as they adjudicate.
func kebab(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '_':
			b.WriteByte('-')
		case c >= 'A' && c <= 'Z':
			if i > 0 && s[i-1] >= 'a' && s[i-1] <= 'z' {
				b.WriteByte('-')
			}
			b.WriteByte(c - 'A' + 'a')
		default:
			b.WriteByte(c)
		}
	}
	out := b.String()
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	return strings.Trim(out, "-")
}

// predChannel returns the kind of a synthesized predicate term ("calls"/"consts"/
// "emits") — the seam channel the cluster was keyed on. It becomes the Layer D
// label's variety, so the ablation matrix separates the const-set slice
// (touchpoint·consts) from the call-seam slice (touchpoint·calls).
func predChannel(pred string) string {
	if k, _, ok := strings.Cut(pred, ":"); ok {
		return k
	}
	return pred
}

// judgeClusters adjudicates each proposed cluster with the LLM oracle and records a
// Layer D label per cluster — the touchpoint detector's precision half (the matrix
// can't measure a detector with no judged labels, so the const-set axis was invisible
// until this path existed). The cluster is N-ary but the label store + judge are
// pair-shaped, so each cluster is judged on its two representative members (the
// shared seam links all members; two of them exercise it) and the label is tagged
// detector=touchpoint, variety=<seam channel>. Disk-cached → re-runs are free.
// Generator semantics preserved: stdout only, no registry writes, no exit code.
func judgeClusters(repo string, props []proposal, twinsOnly bool) {
	j, err := llm.NewJudge()
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque propose-roles --judge: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("# calque — judged touchpoint clusters (Layer D labels: detector=touchpoint)\n\n")
	fmt.Printf("oracle: judging %d cluster(s) with %s (cached results are free)...\n\n", len(props), j.Model())

	var drift, contracted, falseAlarm, failed, shown int
	for _, p := range props {
		c := p.Cluster
		if len(c.Members) < 2 {
			continue
		}
		a, b := c.Members[0], c.Members[1] // two representatives of the shared-seam group
		channel := predChannel(p.Predicate)
		in := llm.PairInput{
			AKey:    a.File + "::" + a.Qualname,
			ASource: readFuncSource(repo, a, 200),
			BKey:    b.File + "::" + b.Qualname,
			BSource: readFuncSource(repo, b, 200),
			Sig:     "[" + channel + "] " + c.Reason(),
		}
		v, err := j.JudgePair(context.Background(), in)
		if err != nil {
			failed++
			fmt.Fprintf(os.Stderr, "  judge %s ≟ %s: %v\n", a.Qualname, b.Qualname, err)
			continue
		}
		// Tag the label with detector=touchpoint and variety=seam channel via the Sig.
		recordLabel(repo, code.SigCandidate{A: a, B: b, Kind: "touchpoint", Sig: "[" + channel + "]"}, v)
		switch v.Class {
		case llm.ClassDrift:
			drift++
		case llm.ClassContractedTwinOK:
			contracted++
		default:
			falseAlarm++
		}
		if twinsOnly && !v.IsTwin() {
			continue
		}
		shown++
		fmt.Printf("## %d. [%s] %s   (%d members)\n", shown, channel, p.Name, len(p.Baseline))
		fmt.Printf("- predicate: %s\n", p.Predicate)
		fmt.Printf("- members: %s\n", strings.Join(p.Baseline, ", "))
		fmt.Printf("  oracle: %s (%s) — %s\n", v.Class, v.Confidence, v.Reason)
	}
	fmt.Printf("\noracle: %d drift · %d contracted-twin-ok · %d false-alarm (of %d judged)",
		drift, contracted, falseAlarm, len(props)-failed)
	if failed > 0 {
		fmt.Printf(" · %d errored", failed)
	}
	fmt.Println()
}
