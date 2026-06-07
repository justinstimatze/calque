package main

// vocab-report — prose axis recall: a read-only frequency surface of
// hyphenated-compound vocabulary across a prose repo. Compounds proliferate
// session-by-session ("X-leg", "X-register", "longing-to-be-chosen"); without a
// deterministic surface of what's actually there, a "small vocabulary"
// discipline drifts. Pure text, no gate — surfacing first (the recall nose for
// prose). --stems groups morphological siblings, the basic synonym-drift signal.
//
// Ported from cupel (MIT) cmd/cupel/vocab_report.go; generalized to walk any
// repo via internal/corpus.

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/justinstimatze/calque/internal/corpus"
)

// hyphenatedCompound matches lowercase tokens with one or more internal hyphens
// (or slashes). Anchored at word boundaries; allows internal digits.
var hyphenatedCompound = regexp.MustCompile(`\b[a-z][a-z0-9]*(?:[-/][a-z][a-z0-9]*){1,}\b`)

type vocabLocation struct {
	Path string
	Line int
}

type vocabHit struct {
	Term      string
	Count     int
	Locations []vocabLocation
}

func runVocabReport(args []string) {
	fs := flag.NewFlagSet("vocab-report", flag.ContinueOnError)
	root := fs.String("dir", ".", "repo root to walk for prose")
	ext := fs.String("ext", "", "comma-separated prose extensions (default: md,markdown,mdx,txt,rst)")
	minCount := fs.Int("min", 3, "minimum frequency to show")
	maxCount := fs.Int("max", 0, "maximum frequency to show (0 = no upper bound)")
	maxLocs := fs.Int("locs", 3, "max example file:line cites per term")
	showAll := fs.Bool("all", false, "show every compound, regardless of --min")
	stems := fs.Bool("stems", false, "group morphological siblings (slot-test/slot-tested) into clusters with ≥2 variants — the compound synonym-drift signature")
	if err := fs.Parse(args); err != nil {
		return
	}

	files, err := corpus.Walk(*root, corpus.ParseExts(*ext))
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque vocab-report: walking %s: %v\n", *root, err)
		os.Exit(1)
	}
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "calque vocab-report: no prose files under %s\n", *root)
		os.Exit(1)
	}

	hits := map[string]*vocabHit{}
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := corpus.StripNonProse(string(raw))
		for _, m := range hyphenatedCompound.FindAllStringIndex(text, -1) {
			tok := text[m[0]:m[1]]
			h := hits[tok]
			if h == nil {
				h = &vocabHit{Term: tok}
				hits[tok] = h
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

	sorted := make([]*vocabHit, 0, len(hits))
	for _, h := range hits {
		sorted = append(sorted, h)
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Count != sorted[j].Count {
			return sorted[i].Count > sorted[j].Count
		}
		return sorted[i].Term < sorted[j].Term
	})

	if *stems {
		emitStemClusters(sorted, *minCount, *maxLocs)
		return
	}

	printed := 0
	fmt.Printf("vocab-report: scanned %d file(s); %d distinct compound(s)\n", len(files), len(hits))
	switch {
	case *showAll:
		fmt.Println("count  term")
	case *maxCount > 0:
		fmt.Printf("count  term  (showing %d ≤ freq ≤ %d)\n", *minCount, *maxCount)
	default:
		fmt.Printf("count  term  (showing freq ≥ %d; --all for every compound)\n", *minCount)
	}
	for _, h := range sorted {
		if !*showAll && h.Count < *minCount {
			break
		}
		if *maxCount > 0 && h.Count > *maxCount {
			continue
		}
		fmt.Printf("%5d  %s\n", h.Count, h.Term)
		for _, l := range h.Locations {
			fmt.Printf("       %s:%d\n", l.Path, l.Line)
		}
		printed++
	}
	fmt.Printf("\nvocab-report: %d term(s) shown\n", printed)
}

// commonSuffixes is the morphological-suffix list stemKey strips from the last
// component of a compound. Longer suffixes first. Small on purpose — false
// stems are tolerable since the stem is only used for grouping, not display.
var commonSuffixes = []string{
	"izations", "ization",
	"esses", "ies", "ing", "ed", "es", "s",
	"er", "est", "ly",
	"tion", "sion", "ment", "able", "ible",
}

// stemKey strips a recognized suffix from the final component of a compound and
// rejoins. "slot-tested"/"slot-testing" → "slot-test". A component must remain
// ≥3 chars after stripping or the original is kept (avoids overstripping).
func stemKey(term string) string {
	parts := strings.Split(term, "-")
	last := parts[len(parts)-1]
	for _, suf := range commonSuffixes {
		if strings.HasSuffix(last, suf) && len(last)-len(suf) >= 3 {
			parts[len(parts)-1] = last[:len(last)-len(suf)]
			return strings.Join(parts, "-")
		}
	}
	return term
}

type stemCluster struct {
	Stem     string
	Variants []*vocabHit
	Total    int
}

// emitStemClusters groups compounds by stem and surfaces clusters with ≥2
// distinct variants — the basic synonym-drift signature in compound form.
func emitStemClusters(sorted []*vocabHit, minCount, maxLocs int) {
	byStem := map[string]*stemCluster{}
	for _, h := range sorted {
		s := stemKey(h.Term)
		c := byStem[s]
		if c == nil {
			c = &stemCluster{Stem: s}
			byStem[s] = c
		}
		c.Variants = append(c.Variants, h)
		c.Total += h.Count
	}
	clusters := make([]*stemCluster, 0, len(byStem))
	for _, c := range byStem {
		if len(c.Variants) < 2 || c.Total < minCount {
			continue
		}
		clusters = append(clusters, c)
	}
	sort.Slice(clusters, func(i, j int) bool {
		if clusters[i].Total != clusters[j].Total {
			return clusters[i].Total > clusters[j].Total
		}
		return clusters[i].Stem < clusters[j].Stem
	})
	fmt.Printf("vocab-report (stem clusters): %d cluster(s) with ≥2 variants and total freq ≥ %d\n", len(clusters), minCount)
	fmt.Println("each cluster groups variants under a normalized stem; multiple variants ≈ synonym drift")
	fmt.Println()
	for _, c := range clusters {
		fmt.Printf("[%d total]  stem=%q  variants=%d\n", c.Total, c.Stem, len(c.Variants))
		for _, v := range c.Variants {
			fmt.Printf("    %5d  %s\n", v.Count, v.Term)
			for i, l := range v.Locations {
				if i >= maxLocs {
					break
				}
				fmt.Printf("           %s:%d\n", l.Path, l.Line)
			}
		}
		fmt.Println()
	}
}
