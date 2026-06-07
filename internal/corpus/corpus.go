// Package corpus walks a prose repository and strips markdown structure down
// to body prose. It is the shared substrate for calque's prose axis — both
// vocab-report and synonym-report walk and strip the same way, so the walk +
// strip logic lives here once (single-sourced, the discipline calque exists to
// enforce) rather than copied per command.
//
// Generalized from cupel (MIT) cmd/cupel/vocab_report.go: cupel walked its own
// fixed layout (README.md + works/ + theory/, excluding theory/working/) and
// stripped cupel-specific citation parentheticals. calque walks an arbitrary
// repo and strips only general markdown structure.
package corpus

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/justinstimatze/calque/internal/glob"
)

// DefaultProseExts are the file extensions treated as prose by default.
var DefaultProseExts = []string{".md", ".markdown", ".mdx", ".txt", ".rst"}

// skipDirs are directory names never descended into (VCS, dependency, build,
// and tool-output dirs whose contents are not authored prose).
var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
	"target":       true,
	".calque":      true,
	".venv":        true,
	"__pycache__":  true,
}

// Walk returns the prose files under root whose extension (lowercased) is in
// exts, in deterministic (sorted) order. It skips VCS/dependency/build dirs and
// any hidden directory (name starting with "."), but never skips root itself.
// exclude is a list of path globs (matched against the repo-relative path, e.g.
// "refs/**", "theory/working/**") whose matches are dropped — the prose analog of
// the code axis's --exclude. Unreadable entries are tolerated.
func Walk(root string, exts, exclude []string) ([]string, error) {
	if len(exts) == 0 {
		exts = DefaultProseExts
	}
	exRe := glob.Compile(exclude)
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // tolerate unreadable entries
		}
		rel := RelPath(root, p)
		if d.IsDir() {
			if p == root {
				return nil
			}
			name := d.Name()
			if skipDirs[name] || strings.HasPrefix(name, ".") || glob.MatchAny(exRe, rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if glob.MatchAny(exRe, rel) {
			return nil
		}
		low := strings.ToLower(p)
		for _, e := range exts {
			if strings.HasSuffix(low, e) {
				out = append(out, p)
				break
			}
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

// ParseExts turns a comma-separated extension list ("md,txt") into normalized
// dotted, lowercased extensions ([".md", ".txt"]). Empty input yields nil
// (callers fall back to DefaultProseExts).
func ParseExts(csv string) []string {
	csv = strings.TrimSpace(csv)
	if csv == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(csv, ",") {
		p = strings.TrimSpace(strings.ToLower(p))
		if p == "" {
			continue
		}
		if !strings.HasPrefix(p, ".") {
			p = "." + p
		}
		out = append(out, p)
	}
	return out
}

var (
	frontMatterFence = regexp.MustCompile(`(?s)\A---\n.*?\n---\n`)
	fencedCodeBlock  = regexp.MustCompile("(?s)```.*?```")
	inlineCode       = regexp.MustCompile("`[^`]*`")
	blockquoteLine   = regexp.MustCompile(`(?m)^\s*>.*$`)
	markdownHeading  = regexp.MustCompile(`(?m)^#+\s+.*$`)
)

// StripNonProse removes leading YAML frontmatter, fenced code blocks, inline
// code spans, blockquote lines (verbatim quotes), and markdown headings. What
// remains is body prose. Order matters: fenced code first, so code contents
// (which may contain `>` or `#`) aren't mistaken for prose structure.
func StripNonProse(s string) string {
	s = frontMatterFence.ReplaceAllString(s, "")
	s = fencedCodeBlock.ReplaceAllString(s, "")
	s = inlineCode.ReplaceAllString(s, "")
	s = blockquoteLine.ReplaceAllString(s, "")
	s = markdownHeading.ReplaceAllString(s, "")
	return s
}

// RelPath returns path relative to root, or path unchanged if that fails.
func RelPath(root, path string) string {
	if r, err := filepath.Rel(root, path); err == nil {
		return r
	}
	return path
}

// LineOf returns the 1-based line number of byte offset off within text.
func LineOf(text string, off int) int {
	if off > len(text) {
		off = len(text)
	}
	return 1 + strings.Count(text[:off], "\n")
}
