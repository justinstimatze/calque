package main

// synonym-report — prose axis recall, the harder case: regular English words
// used inconsistently for one concept ("leader" vs "founder" vs "guru";
// "desired" vs "wanted"). Each word is normal English; only the cross-document
// inconsistency is the drift, and lexical surfaces (vocab-report) miss it.
//
// Pipeline: walk prose → tally content words → keep freq ≥ --min (cap
// --max-words) → embed via local ollama → surface pairs/clusters at cosine ≥
// --threshold. The pairs ARE noisy (embedding similarity also captures
// hypernyms, antonyms, topical neighbors); this is a SURFACING tool — a
// tractable candidate list to adjudicate, NOT a gate. (calque's "nose, not judge".)
//
// Ported from cupel (MIT) cmd/cupel/synonym_report.go; generalized to walk any
// repo (internal/corpus) and use internal/embed.

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/justinstimatze/calque/internal/corpus"
	"github.com/justinstimatze/calque/internal/embed"
)

// contentWord matches a word token (letters + internal apostrophes), ≥4 chars.
// Hyphenated compounds are intentionally excluded — vocab-report covers them.
var contentWord = regexp.MustCompile(`\b[a-zA-Z][a-zA-Z']{3,}\b`)

// synonymStoplist drops high-frequency English function words whose embedding
// similarity is uninformative. (cupel's project-specific term suppressions are
// dropped — calque is repo-agnostic; use --threshold/--min to tune noise.)
var synonymStoplist = map[string]bool{
	"that": true, "this": true, "with": true, "from": true, "into": true,
	"have": true, "been": true, "were": true, "they": true, "them": true,
	"some": true, "what": true, "when": true, "where": true, "which": true,
	"there": true, "their": true, "would": true, "could": true, "should": true,
	"about": true, "after": true, "before": true, "between": true, "through": true,
	"during": true, "while": true, "until": true, "than": true, "then": true,
	"because": true, "since": true, "though": true, "although": true, "however": true,
	"thus": true, "also": true, "only": true, "even": true, "more": true,
	"most": true, "less": true, "such": true, "same": true, "many": true,
	"much": true, "very": true, "still": true, "just": true, "back": true,
	"each": true, "both": true, "other": true, "another": true,
	"itself": true, "themselves": true, "ourselves": true,
	"these": true, "those": true,
	"like": true, "well": true, "make": true, "made": true, "take": true,
	"taken": true, "give": true, "given": true, "gets": true, "going": true,
	"comes": true, "came": true, "goes": true, "seem": true, "seems": true,
	"feel": true, "felt": true, "look": true, "looks": true, "looked": true,
	"know": true, "known": true, "knew": true, "show": true, "shows": true,
	"used": true, "uses": true, "using": true,
}

type wordHit struct {
	Word      string
	Count     int
	Locations []vocabLocation
}

func runSynonymReport(args []string) {
	fs := flag.NewFlagSet("synonym-report", flag.ContinueOnError)
	root := fs.String("dir", ".", "repo root to walk for prose")
	ext := fs.String("ext", "", "comma-separated prose extensions (default: md,markdown,mdx,txt,rst)")
	exclude := fs.String("exclude", "", "comma-separated path glob(s) to skip (e.g. refs/**,theory/working/**)")
	minCount := fs.Int("min", 8, "minimum frequency for a word to be a candidate")
	threshold := fs.Float64("threshold", 0.78, "cosine similarity floor for surfacing a pair")
	maxWords := fs.Int("max-words", 1500, "hard cap on candidate vocabulary (highest-freq win)")
	maxLocs := fs.Int("locs", 2, "max example file:line cites per word")
	skipStem := fs.Bool("skip-morph", true, "skip pairs that share a normalized stem (vocab-report --stems territory)")
	pairsMode := fs.Bool("pairs", false, "list individual pairs instead of union-find clustering")
	if err := fs.Parse(args); err != nil {
		return
	}

	files, err := corpus.Walk(*root, corpus.ParseExts(*ext), splitCSV(*exclude))
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque synonym-report: walking %s: %v\n", *root, err)
		os.Exit(1)
	}

	wordHits := map[string]*wordHit{}
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := corpus.StripNonProse(string(raw))
		for _, m := range contentWord.FindAllStringIndex(text, -1) {
			tok := strings.ToLower(text[m[0]:m[1]])
			if synonymStoplist[tok] {
				continue
			}
			h := wordHits[tok]
			if h == nil {
				h = &wordHit{Word: tok}
				wordHits[tok] = h
			}
			h.Count++
			if len(h.Locations) < *maxLocs {
				h.Locations = append(h.Locations, vocabLocation{
					Path: corpus.RelPath(*root, path),
					Line: corpus.LineOf(text, m[0]),
				})
			}
		}
	}

	var candidates []*wordHit
	for _, h := range wordHits {
		if h.Count >= *minCount {
			candidates = append(candidates, h)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Count != candidates[j].Count {
			return candidates[i].Count > candidates[j].Count
		}
		return candidates[i].Word < candidates[j].Word
	})
	if len(candidates) > *maxWords {
		candidates = candidates[:*maxWords]
	}
	if len(candidates) < 2 {
		fmt.Printf("synonym-report: only %d candidate(s) at min=%d; nothing to cluster\n", len(candidates), *minCount)
		return
	}
	fmt.Printf("synonym-report: %d candidate word(s) at min=%d, embedding via %s...\n", len(candidates), *minCount, embed.Model())

	texts := make([]string, len(candidates))
	for i, c := range candidates {
		texts[i] = c.Word
	}
	t0 := time.Now()
	vecs, err := embed.TextsBatched(texts, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "synonym-report: embed failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("synonym-report: embedded %d term(s) in %s\n", len(vecs), time.Since(t0).Round(time.Millisecond))

	type pair struct {
		i, j int
		sim  float64
	}
	var pairs []pair
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if *skipStem && shareWordStem(candidates[i].Word, candidates[j].Word) {
				continue
			}
			if s := embed.Cosine(vecs[i], vecs[j]); s >= *threshold {
				pairs = append(pairs, pair{i, j, s})
			}
		}
	}
	fmt.Printf("synonym-report: %d pair(s) at sim ≥ %.2f (skip-morph=%v)\n", len(pairs), *threshold, *skipStem)

	if *pairsMode {
		sort.Slice(pairs, func(i, j int) bool { return pairs[i].sim > pairs[j].sim })
		fmt.Println()
		for _, p := range pairs {
			fmt.Printf("%.3f  %s ↔ %s  (%d / %d uses)\n", p.sim,
				candidates[p.i].Word, candidates[p.j].Word, candidates[p.i].Count, candidates[p.j].Count)
		}
		return
	}

	// Union-find: connected components in the pair graph.
	parent := make([]int, len(candidates))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	for _, p := range pairs {
		ra, rb := find(p.i), find(p.j)
		if ra != rb {
			parent[ra] = rb
		}
	}
	groups := map[int][]int{}
	for i := range candidates {
		groups[find(i)] = append(groups[find(i)], i)
	}

	type cluster struct {
		Indices []int
		Total   int
	}
	clusters := make([]cluster, 0, len(groups))
	for _, idxs := range groups {
		if len(idxs) < 2 {
			continue
		}
		tot := 0
		for _, i := range idxs {
			tot += candidates[i].Count
		}
		clusters = append(clusters, cluster{idxs, tot})
	}
	sort.Slice(clusters, func(i, j int) bool {
		if clusters[i].Total != clusters[j].Total {
			return clusters[i].Total > clusters[j].Total
		}
		return len(clusters[i].Indices) > len(clusters[j].Indices)
	})

	fmt.Printf("\n%d cluster(s) with ≥2 terms:\n\n", len(clusters))
	for _, c := range clusters {
		members := append([]int(nil), c.Indices...)
		sort.Slice(members, func(i, j int) bool { return candidates[members[i]].Count > candidates[members[j]].Count })
		fmt.Printf("[%d total · %d terms]\n", c.Total, len(members))
		for _, i := range members {
			h := candidates[i]
			fmt.Printf("    %5d  %s\n", h.Count, h.Word)
			for _, l := range h.Locations {
				fmt.Printf("           %s:%d\n", l.Path, l.Line)
			}
		}
		fmt.Println()
	}
}

// shareWordStem returns true when two words probably share a root after suffix
// stripping: stemKey equality, then a ≥4 common prefix, then a small irregular-
// pair list. Precision-only — real synonym candidates pass straight through.
func shareWordStem(a, b string) bool {
	la, lb := strings.ToLower(a), strings.ToLower(b)
	if stemKey(la) == stemKey(lb) {
		return true
	}
	if commonPrefix(la, lb) >= 4 {
		return true
	}
	return irregularPairs[pairKey(la, lb)]
}

func commonPrefix(a, b string) int {
	n := min(len(a), len(b))
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

func pairKey(a, b string) string {
	if a < b {
		return a + "\x00" + b
	}
	return b + "\x00" + a
}

// irregularPairs suppresses morphological siblings the suffix-stripper misses
// (held/hold, found/find) — a false-positive suppression list, not a vocabulary.
var irregularPairs = map[string]bool{
	pairKey("held", "hold"): true, pairKey("held", "holds"): true, pairKey("hold", "holds"): true,
	pairKey("hold", "holding"): true, pairKey("holds", "holding"): true, pairKey("held", "holding"): true,
	pairKey("thought", "think"): true, pairKey("thought", "thinking"): true, pairKey("thought", "thinks"): true,
	pairKey("think", "thinks"): true, pairKey("think", "thinking"): true,
	pairKey("found", "find"): true, pairKey("found", "finds"): true, pairKey("found", "finding"): true,
	pairKey("find", "finding"): true, pairKey("find", "finds"): true, pairKey("finds", "finding"): true,
	pairKey("made", "make"): true, pairKey("made", "makes"): true, pairKey("made", "making"): true,
	pairKey("make", "makes"): true, pairKey("make", "making"): true, pairKey("makes", "making"): true,
	pairKey("said", "say"): true, pairKey("said", "says"): true, pairKey("said", "saying"): true,
	pairKey("say", "says"): true, pairKey("say", "saying"): true, pairKey("says", "saying"): true,
	pairKey("ran", "run"): true, pairKey("ran", "runs"): true, pairKey("ran", "running"): true,
	pairKey("seen", "see"): true, pairKey("seen", "sees"): true, pairKey("seen", "seeing"): true,
	pairKey("kept", "keep"): true, pairKey("kept", "keeps"): true, pairKey("kept", "keeping"): true,
	pairKey("paid", "pay"): true, pairKey("paid", "pays"): true, pairKey("paid", "paying"): true,
	pairKey("sold", "sell"): true, pairKey("sold", "sells"): true, pairKey("sold", "selling"): true,
	pairKey("died", "die"): true, pairKey("died", "dies"): true, pairKey("died", "dying"): true,
	pairKey("dies", "dying"): true, pairKey("death", "die"): true, pairKey("death", "dies"): true,
	pairKey("death", "dying"): true,
	pairKey("won", "win"):     true, pairKey("won", "wins"): true, pairKey("won", "winning"): true,
	pairKey("cannot", "can't"): true, pairKey("woman", "women"): true, pairKey("three", "third"): true,
}
