package main

// check — the ongoing/hookable gate (the keystone of automated use). Runs the
// same scan as `scan`, then DIFFS against the registry: surfaces only NEW
// (un-adjudicated) suspect pairs, suppresses known ones, and reconciles STALE
// registry entries (pairs whose referenced code no longer exists — the
// dusty-over-months problem, handled by liveness, not age-eviction).
//
// Warn-only by default (exit 0) so it can land in a pre-commit / Stop hook
// without blocking; --strict exits 1 when there are new suspects (cupel's
// vocab-audit discipline: sweep the registry, then flip to strict).

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/justinstimatze/calque/internal/code"
	"github.com/justinstimatze/calque/internal/registry"
)

func runCheck(args []string) {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	b := addBoundaryFlags(fs)
	repo, left, right, exclude := b.repo, b.left, b.right, b.exclude
	minScore := fs.Float64("min-score", 0.18, "minimum suspicion score to consider")
	minLines := fs.Int("min-lines", 4, "ignore functions shorter than this many lines")
	regPath := fs.String("registry", ".calque/registry.md", "registry file (adjudicated pairs)")
	strict := fs.Bool("strict", false, "exit 1 if there are new (un-adjudicated) suspects")
	clusterMinMembers := fs.Int("cluster-min-members", 3, "smallest N-ary cluster to consider (2 includes diluted pairs)")
	clusterMaxFanout := fs.Int("cluster-max-fanout", 8, "a private symbol touched by more than this is plumbing, not a seam")
	noFireLog := fs.Bool("no-fire-log", false, "do not append NEW suspects to .calque/fires.jsonl (calibration telemetry)")
	if err := fs.Parse(args); err != nil {
		return
	}

	f, err := computeCheck(*repo, *left, *right, *exclude, *minScore, *minLines, *clusterMinMembers, *clusterMaxFanout, *regPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque check: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(renderCheck(f, *regPath))

	if !*noFireLog && (len(f.Fresh) > 0 || len(f.FreshC) > 0) {
		logFires(*repo, f.Fresh, f.FreshC)
	}

	if *strict && (len(f.Fresh) > 0 || len(f.FreshC) > 0) {
		os.Exit(1)
	}
}

// checkFindings is the pure result of the code-axis gate: the new/known split
// over pairs and N-ary clusters plus stale (dangling) registry entries. Isolated
// from printing + os.Exit so both the CLI and the MCP server share one core.
type checkFindings struct {
	Fresh      []code.Suspicion
	Known      int
	FreshC     []code.Cluster
	KnownC     int
	Stale      []registry.Entry
	StaleC     []registry.ClusterEntry
	Unresolved []registry.Entry // known drift, both paths still live — not yet collapsed
	Warn       string           // non-empty when the registry exists but parsed to zero entries
}

// computeCheck runs the scan, diffs against the registry, and returns the
// new/known/stale split — the shared core behind `calque check` (CLI) and the
// calque_check MCP tool. No side effects (no print, no fire-log, no exit).
func computeCheck(repo, left, right, exclude string, minScore float64, minLines, clusterMinMembers, clusterMaxFanout int, regPath string) (checkFindings, error) {
	copts := clusterOptsFrom(minLines, clusterMinMembers, clusterMaxFanout, 1<<30)
	r, err := codeAxis(repo, left, right, exclude, minScore, minLines, 1<<30, copts, true)
	if err != nil {
		return checkFindings{}, err
	}
	reg, err := registry.Load(joinRepo(repo, regPath))
	if err != nil {
		return checkFindings{}, fmt.Errorf("reading registry: %w", err)
	}

	var f checkFindings
	f.Warn = registryParseWarning(joinRepo(repo, regPath), len(reg.Entries), len(reg.Clusters))
	for _, s := range r.Pairs {
		if reg.Has(s.Left.Key(), s.Right.Key()) {
			f.Known++
			continue
		}
		f.Fresh = append(f.Fresh, s)
	}

	// N-ary clusters (the touchpoint pass): same new/known split, keyed on the
	// member SET (§15).
	for _, c := range r.Clusters {
		if reg.HasCluster(c.MemberKeys()) {
			f.KnownC++
			continue
		}
		f.FreshC = append(f.FreshC, c)
	}

	// Liveness reconciliation: registry entries whose referenced code is gone.
	live := make(map[string]bool, len(r.All))
	for _, fn := range r.All {
		live[fn.Key()] = true
	}
	for _, e := range reg.Entries {
		if !live[e.Key1] || !live[e.Key2] {
			f.Stale = append(f.Stale, e)
		}
	}
	for _, e := range reg.Clusters {
		for _, k := range e.Keys {
			if !live[k] {
				f.StaleC = append(f.StaleC, e)
				break
			}
		}
	}

	// Known drift whose both paths are still live = a dual path not yet collapsed.
	// Surfaced (warn-only) with its recorded collapse direction so a later agent
	// collapses the doomed path instead of re-syncing it (§18.7).
	f.Unresolved = unresolvedDrift(reg.Entries, live)
	return f, nil
}

// registryParseWarning returns a warning when a registry file exists and has
// real content but parsed to zero entries — almost always a format/path
// mismatch (e.g. a Python-era registry the Go parser can't read), the failure
// mode that otherwise makes `check` silently treat the whole repo as new.
func registryParseWarning(path string, entries, clusters int) string {
	if entries > 0 || clusters > 0 {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "" // a missing registry is legitimately empty, not a warning
	}
	hasContent, oldFormat := false, false
	for _, line := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, ">") {
			continue
		}
		hasContent = true
		if strings.HasPrefix(t, "- left:") || strings.HasPrefix(t, "- right:") {
			oldFormat = true
		}
	}
	if !hasContent {
		return ""
	}
	if oldFormat {
		return fmt.Sprintf("registry %s has content but parsed 0 entries — looks like the Python-era format; run `calque migrate-registry --in %s --write`", path, path)
	}
	return fmt.Sprintf("registry %s has content but parsed 0 entries — wrong format or path? entries are `- pair: a | b` / `- cluster: a | b | …` lines", path)
}

// renderCheck formats the gate findings as the human/agent-readable report
// shared by the CLI and the MCP tool.
func renderCheck(f checkFindings, regPath string) string {
	var b strings.Builder
	if f.Warn != "" {
		fmt.Fprintf(&b, "⚠ %s\n\n", f.Warn)
	}
	nStale := len(f.Stale) + len(f.StaleC)
	fmt.Fprintf(&b, "calque check: pairs %d new · %d known | clusters %d new · %d known | %d stale registry entr%s\n",
		len(f.Fresh), f.Known, len(f.FreshC), f.KnownC, nStale, plural(nStale, "y", "ies"))
	if n := len(f.Unresolved); n > 0 {
		fmt.Fprintf(&b, "%d known drift pair%s not yet collapsed (warn-only — see below).\n", n, plural(n, "", "s"))
	}

	for _, s := range f.Fresh {
		fmt.Fprintf(&b, "\nNEW  %.2f  [%s]  `%s` (%s:%d)  ≟  `%s` (%s:%d)\n     %s\n",
			s.Score, pairID(s), s.Left.Qualname, s.Left.File, s.Left.Line,
			s.Right.Qualname, s.Right.File, s.Right.Line, s.Reason())
		fmt.Fprintf(&b, "     adjudicate in %s — add:  - pair: %s | %s\n", regPath, s.Left.Key(), s.Right.Key())
	}
	for _, c := range f.FreshC {
		fmt.Fprintf(&b, "\nNEW-CLUSTER  %.2f  [%s]  (%d members)  %s\n", c.Score, clusterID(c), len(c.Members), c.Reason())
		for _, m := range c.Members {
			fmt.Fprintf(&b, "     `%s` (%s:%d)\n", m.Qualname, m.File, m.Line)
		}
		fmt.Fprintf(&b, "     adjudicate in %s — add:  - cluster: %s\n", regPath, strings.Join(c.MemberKeys(), " | "))
	}
	for _, e := range f.Stale {
		fmt.Fprintf(&b, "\nSTALE  `%s` ≟ `%s` — referenced code no longer exists; prune or re-home (was: %s%s)\n",
			e.Key1, e.Key2, e.Verdict, reviewedSuffix(e))
	}
	for _, e := range f.StaleC {
		fmt.Fprintf(&b, "\nSTALE-CLUSTER  {%s} — a referenced member no longer exists; prune or re-home (was: %s)\n",
			strings.Join(e.Keys, ", "), e.Verdict)
	}
	for _, e := range f.Unresolved {
		fmt.Fprintf(&b, "\nDRIFT (unresolved)  `%s` ≟ `%s` — known drift, both paths still live%s.\n",
			e.Key1, e.Key2, reviewedSuffix(e))
		switch {
		case e.Canonical != "" && e.DoNotResync != "":
			fmt.Fprintf(&b, "     collapse to `%s`; do NOT re-sync `%s` (it is the doomed path).\n", e.Canonical, e.DoNotResync)
		case e.Canonical != "":
			fmt.Fprintf(&b, "     collapse toward the canonical path `%s`.\n", e.Canonical)
		case e.DoNotResync != "":
			fmt.Fprintf(&b, "     do NOT re-sync `%s` (it is the doomed path); collapse it away.\n", e.DoNotResync)
		default:
			fmt.Fprintf(&b, "     collapse direction not recorded — add `- canonical:` / `- do-not-resync:` so a later agent collapses the right path.\n")
		}
	}
	return b.String()
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func reviewedSuffix(e registry.Entry) string {
	if e.Reviewed == "" {
		return ""
	}
	return ", reviewed " + e.Reviewed
}

// unresolvedDrift returns the registry's `drift`-verdict pairs whose BOTH paths are
// still live — known dual paths that have not yet been collapsed. (A drift whose one
// side is gone collapsed already and surfaces in the Stale pass instead.) These are a
// standing reminder so a later agent collapses the doomed path rather than re-syncing
// it; the per-entry Canonical / DoNotResync fields carry the direction (§18.7).
func unresolvedDrift(entries []registry.Entry, live map[string]bool) []registry.Entry {
	var out []registry.Entry
	for _, e := range entries {
		if e.VerdictClass() == "drift" && live[e.Key1] && live[e.Key2] {
			out = append(out, e)
		}
	}
	return out
}

func joinRepo(repo, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(repo, p)
}
