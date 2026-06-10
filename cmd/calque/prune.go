package main

// prune — registry liveness GC. `check` already DETECTS stale entries (adjudicated
// pairs/clusters whose referenced code no longer exists — the dusty-over-months
// problem); `prune` is the remediation it was missing. It re-runs the liveness
// reconciliation and, with --write, surgically removes the stale entries' machine
// lines from .calque/registry.md — backup first, dry-run by default.
//
// Why this exists: a real dogfood run (2026-06-10) found 38/40 registry entries
// stale after the audited repo deleted the file the whole axis pointed at.
// calque flagged every one but offered no way to act on it except hand-editing a
// 40KB markdown file. prune closes that loop.
//
// Scope discipline: prune removes ONLY entries calque can prove are dead (a
// referenced key absent from the freshly-extracted corpus). It refuses to run on
// an empty corpus (a wrong --repo or all-unsupported-source repo would otherwise
// mark everything stale and wipe the registry). It touches only the `- pair:` /
// `- cluster:` lines + their attached attributes; freeform prose/`##` headers are
// left for the human, since the registry's narrative history is the point.

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/justinstimatze/calque/internal/code"
	"github.com/justinstimatze/calque/internal/pairkey"
)

func runPrune(args []string) {
	fs := flag.NewFlagSet("prune", flag.ContinueOnError)
	b := addBoundaryFlags(fs)
	repo, left, right, exclude := b.repo, b.left, b.right, b.exclude
	minScore := fs.Float64("min-score", 0.18, "minimum suspicion score (liveness is score-independent; passed through for the scan)")
	minLines := fs.Int("min-lines", 4, "ignore functions shorter than this many lines")
	regPath := fs.String("registry", ".calque/registry.md", "registry file to prune")
	clusterMinMembers := fs.Int("cluster-min-members", 3, "smallest N-ary cluster to consider")
	clusterMaxFanout := fs.Int("cluster-max-fanout", 8, "private-symbol fanout ceiling for a seam")
	write := fs.Bool("write", false, "remove stale entries in place (writes a .bak backup first)")
	if err := fs.Parse(args); err != nil {
		return
	}

	f, err := computeCheck(*repo, *left, *right, *exclude, *minScore, *minLines, *clusterMinMembers, *clusterMaxFanout, *regPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque prune: %v\n", err)
		os.Exit(1)
	}

	// Refuse on an empty corpus: with zero extracted functions EVERY entry looks
	// dead, so --write would wipe the registry. Almost always a wrong --repo, an
	// over-broad --exclude, or an all-unsupported-language tree.
	regFull := joinRepo(*repo, *regPath)
	corpus := f.Corpus
	if corpus == 0 {
		fmt.Fprintf(os.Stderr, "calque prune: extracted 0 functions under %s — refusing to prune (every entry would look stale).\n", *repo)
		fmt.Fprintf(os.Stderr, "  Check --repo / --exclude, or confirm the source language is supported (%v).\n", code.SupportedExts())
		os.Exit(1)
	}

	// Liveness is judged over the WHOLE extracted corpus, but an --exclude hides
	// files from extraction — so an excluded-but-live function reads as dead. Warn
	// when pruning under an exclude, since a wrong exclude is the one way prune can
	// drop a genuinely-live entry.
	if strings.TrimSpace(*exclude) != "" {
		fmt.Fprintf(os.Stderr, "⚠ pruning with --exclude %q active: any entry whose code lives in an excluded file will read as stale. Prefer no --exclude for prune.\n\n", *exclude)
	}

	staleKeys := map[string]bool{}
	for _, e := range f.Stale {
		staleKeys[pairkey.Key(e.Key1, e.Key2)] = true
	}
	staleClusterKeys := map[string]bool{}
	for _, e := range f.StaleC {
		staleClusterKeys[pairkey.SetKey(e.Keys)] = true
	}

	if len(staleKeys) == 0 && len(staleClusterKeys) == 0 {
		fmt.Printf("# calque prune\n\nno stale entries in %s — registry is live (corpus: %d funcs).\n", regFull, corpus)
		return
	}

	data, err := os.ReadFile(regFull)
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque prune: reading registry: %v\n", err)
		os.Exit(1)
	}
	kept, removed := pruneRegistry(string(data), staleKeys, staleClusterKeys)

	fmt.Printf("# calque prune\n\n")
	fmt.Printf("corpus: %d funcs · stale pairs: %d · stale clusters: %d\n\n", corpus, len(staleKeys), len(staleClusterKeys))
	for _, r := range removed {
		fmt.Printf("- %s\n", r)
	}
	fmt.Println()

	if !*write {
		fmt.Printf("dry run — re-run with --write to remove these from %s (a .bak backup is written first)\n", regFull)
		return
	}
	if err := os.WriteFile(regFull+".bak", data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "calque prune: writing backup: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(regFull, []byte(kept), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "calque prune: writing registry: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("removed %d stale %s from %s (backup: %s.bak)\n", len(removed), plural(len(removed), "entry", "entries"), regFull, regFull)
	fmt.Println("note: any `##` header now describing no entries is left in place — review the prose.")
}

// pruneRegistry returns the registry text with each stale `- pair:`/`- cluster:`
// entry's machine lines removed (the anchor line plus its contiguous attribute
// lines), and a human list of what was removed. Freeform prose is untouched.
func pruneRegistry(text string, stalePairs, staleClusters map[string]bool) (string, []string) {
	lines := strings.Split(text, "\n")
	keep := make([]bool, len(lines))
	for i := range keep {
		keep[i] = true
	}
	var removed []string

	i := 0
	for i < len(lines) {
		t := strings.TrimSpace(lines[i])
		var staleHit, desc string
		switch {
		case strings.HasPrefix(t, "- pair:"):
			v := strings.TrimSpace(strings.TrimPrefix(t, "- pair:"))
			if k1, k2, ok := strings.Cut(v, "|"); ok {
				if key := pairkey.Key(cleanRegKey(k1), cleanRegKey(k2)); stalePairs[key] {
					staleHit, desc = key, "pair: "+v
				}
			}
		case strings.HasPrefix(t, "- cluster:"):
			v := strings.TrimSpace(strings.TrimPrefix(t, "- cluster:"))
			var keys []string
			for _, part := range strings.Split(v, "|") {
				if k := cleanRegKey(part); k != "" {
					keys = append(keys, k)
				}
			}
			if len(keys) >= 2 {
				if key := pairkey.SetKey(keys); staleClusters[key] {
					staleHit, desc = key, "cluster: "+v
				}
			}
		}

		if staleHit == "" {
			i++
			continue
		}

		// Remove the anchor line + its contiguous attribute lines (verdict/reviewed/
		// canonical/do-not-resync). Stop at the next entry starter or any non-attribute
		// line, so a shared `##` header and neighbouring entries are preserved.
		keep[i] = false
		j := i + 1
		for j < len(lines) && isEntryAttribute(strings.TrimSpace(lines[j])) {
			keep[j] = false
			j++
		}
		removed = append(removed, desc)
		i = j
	}

	var out []string
	for i, ln := range lines {
		if keep[i] {
			out = append(out, ln)
		}
	}
	return strings.Join(out, "\n"), removed
}

// isEntryAttribute reports whether a (trimmed) line is an attribute that attaches
// to the preceding pair/cluster entry — i.e. should be removed with it.
func isEntryAttribute(t string) bool {
	for _, p := range []string{"- verdict:", "- reviewed:", "- canonical:", "- do-not-resync:", "- signal:", "- policy:", "- note:", "- predicted:"} {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}

// cleanRegKey mirrors registry.cleanKey (unexported): trim whitespace + backticks.
func cleanRegKey(s string) string {
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(s), "`"))
}
