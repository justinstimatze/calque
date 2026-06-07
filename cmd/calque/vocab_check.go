package main

// vocab-check — the PROSE gate (the prose-axis analog of `check`). Walks the same
// compound surface as vocab-report and flags hyphenated compounds at frequency
// >= threshold that are NOT in the allow-list — the deterministic surface that
// catches an invented compound noun-stack ("longing-to-be-chosen",
// "performance-of-virtue") proliferating session-by-session before it entrenches.
//
// The allow-list (`.calque/vocab-allowlist.txt`, one slug per line, # comments)
// is the prose registry — the contracted-twin-ok set for vocabulary. calque is
// substrate-general, so unlike cupel's vocab-audit there is no auto-seed from
// project-specific catalogs (engines/clusters/glossary); `--bootstrap` prints the
// current compound tail to seed the allow-list instead.
//
// Warn-only by default (exit 0, hookable); --strict exits 1 on violations — the
// cupel discipline: sweep the tail into the allow-list, then flip to strict.
// Ported from cupel cmd/cupel/vocab_audit.go (MIT, attribution preserved).

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/justinstimatze/calque/internal/corpus"
)

func runVocabCheck(args []string) {
	fs := flag.NewFlagSet("vocab-check", flag.ContinueOnError)
	root := fs.String("dir", ".", "repo root to walk for prose")
	ext := fs.String("ext", "", "comma-separated prose extensions (default: md,markdown,mdx,txt,rst)")
	exclude := fs.String("exclude", "", "comma-separated path glob(s) to skip (e.g. refs/**,theory/working/**)")
	allowlistPath := fs.String("allowlist", ".calque/vocab-allowlist.txt", "allow-list file (one slug per line; # comments) — the prose registry")
	seedCmd := fs.String("seed-cmd", "", "shell command whose stdout is merged into the allow-list (the seeder contract: one slug per line, # comments) — e.g. a project's own catalog→slug command. Run with cwd=--dir.")
	threshold := fs.Int("min", 5, "minimum frequency to flag a missing compound")
	maxLocs := fs.Int("locs", 2, "max example file:line cites per flagged compound")
	strict := fs.Bool("strict", false, "exit 1 on violations (default warn-only)")
	bootstrap := fs.Bool("bootstrap", false, "print the current compound tail (freq >= min) to seed the allow-list, then exit")
	quiet := fs.Bool("quiet", false, "suppress the 'clean' line when there are no violations")
	if err := fs.Parse(args); err != nil {
		return
	}

	sorted, nFiles, err := tallyCompounds(*root, corpus.ParseExts(*ext), splitCSV(*exclude), *maxLocs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque vocab-check: walking %s: %v\n", *root, err)
		os.Exit(1)
	}
	if nFiles == 0 {
		fmt.Fprintf(os.Stderr, "calque vocab-check: no prose files under %s\n", *root)
		os.Exit(1)
	}

	if *bootstrap {
		fmt.Println("# calque vocab allow-list — the prose registry (contracted-twin-ok vocabulary).")
		fmt.Printf("# Seeded from compounds at freq >= %d. Remove any that are actually drift; the\n", *threshold)
		fmt.Println("# rest are known vocabulary. One slug per line; lines starting with # are comments.")
		fmt.Println()
		for _, h := range sorted {
			if h.Count >= *threshold {
				fmt.Println(h.Term)
			}
		}
		return
	}

	allow, warn := buildVocabAllowlist(joinRepo(*root, *allowlistPath), *seedCmd, *root)
	if warn != "" {
		fmt.Fprintf(os.Stderr, "calque vocab-check: %s\n", warn)
	}
	violations := compoundViolations(sorted, allow, *threshold)

	if len(violations) == 0 {
		if !*quiet {
			fmt.Fprintf(os.Stderr, "calque vocab-check: clean across %d file(s) — %d allow-listed compound(s); threshold freq >= %d\n",
				nFiles, len(allow), *threshold)
		}
		return
	}

	mode := "warning"
	if *strict {
		mode = "violation"
	}
	fmt.Fprintf(os.Stderr, "calque vocab-check: %d %s(s) — compounds at freq >= %d not in the allow-list:\n",
		len(violations), mode, *threshold)
	for _, h := range violations {
		fmt.Fprintf(os.Stderr, "%5d  %s\n", h.Count, h.Term)
		for _, l := range h.Locations {
			fmt.Fprintf(os.Stderr, "       %s:%d\n", l.Path, l.Line)
		}
	}
	fmt.Fprintf(os.Stderr, "\nFix: (a) promote to %s (add the slug on its own line); (b) rewrite so freq drops below %d; or (c) consolidate the synonyms (see `vocab-report --stems`).\n",
		*allowlistPath, *threshold)

	if *strict {
		os.Exit(1)
	}
}

// buildVocabAllowlist loads the file allow-list and, if a seed command is given,
// merges its stdout in under the seeder contract. Best-effort: a seed failure
// returns a non-empty warning string but still yields the file allow-list, so a
// broken seeder can't wedge the gate. Shared by the CLI and the MCP tool.
func buildVocabAllowlist(allowlistPath, seedCmd, dir string) (map[string]bool, string) {
	allow := loadAllowlist(allowlistPath)
	if seedCmd == "" {
		return allow, ""
	}
	seeded, err := runSeedCmd(seedCmd, dir)
	if err != nil {
		return allow, fmt.Sprintf("seed-cmd failed (%v) — continuing with file allow-list only", err)
	}
	for k := range seeded {
		allow[k] = true
	}
	return allow, ""
}

// vocabFindings is the pure result of the prose gate, isolated from print +
// os.Exit so the CLI and the MCP server share one core.
type vocabFindings struct {
	Violations []*vocabHit
	NFiles     int
	NAllow     int
	Threshold  int
	Warn       string
}

// computeVocabCheck runs the compound walk, builds the allow-list (file +
// optional seed command), and returns the violations — the shared core behind
// `calque vocab-check` (CLI) and the calque_vocab_check MCP tool.
func computeVocabCheck(root, ext, exclude, allowlistPath, seedCmd string, threshold, maxLocs int) (vocabFindings, error) {
	sorted, nFiles, err := tallyCompounds(root, corpus.ParseExts(ext), splitCSV(exclude), maxLocs)
	if err != nil {
		return vocabFindings{}, err
	}
	allow, warn := buildVocabAllowlist(joinRepo(root, allowlistPath), seedCmd, root)
	return vocabFindings{
		Violations: compoundViolations(sorted, allow, threshold),
		NFiles:     nFiles,
		NAllow:     len(allow),
		Threshold:  threshold,
		Warn:       warn,
	}, nil
}

// renderVocabCheck formats the prose-gate findings as the human/agent-readable
// report shared by the CLI and the MCP tool.
func renderVocabCheck(f vocabFindings, allowlistPath string) string {
	var b strings.Builder
	if f.Warn != "" {
		fmt.Fprintf(&b, "calque vocab-check: %s\n", f.Warn)
	}
	if len(f.Violations) == 0 {
		fmt.Fprintf(&b, "calque vocab-check: clean across %d file(s) — %d allow-listed compound(s); threshold freq >= %d\n",
			f.NFiles, f.NAllow, f.Threshold)
		return b.String()
	}
	fmt.Fprintf(&b, "calque vocab-check: %d warning(s) — compounds at freq >= %d not in the allow-list:\n",
		len(f.Violations), f.Threshold)
	for _, h := range f.Violations {
		fmt.Fprintf(&b, "%5d  %s\n", h.Count, h.Term)
		for _, l := range h.Locations {
			fmt.Fprintf(&b, "       %s:%d\n", l.Path, l.Line)
		}
	}
	fmt.Fprintf(&b, "\nFix: (a) promote to %s (add the slug on its own line); (b) rewrite so freq drops below %d; or (c) consolidate the synonyms (see `vocab-report --stems`).\n",
		allowlistPath, f.Threshold)
	return b.String()
}

// compoundViolations returns the compounds at or above threshold that are not in
// the allow-list (sorted input is preserved — frequency desc). This is the prose
// gate's core predicate, isolated for testing.
func compoundViolations(sorted []*vocabHit, allow map[string]bool, threshold int) []*vocabHit {
	var out []*vocabHit
	for _, h := range sorted {
		if h.Count >= threshold && !allow[h.Term] {
			out = append(out, h)
		}
	}
	return out
}

// parseAllowlist reads the slug list (one slug per line, # comments + blanks
// ignored) — the seeder contract, shared by the file allow-list and seed-cmd
// stdout.
func parseAllowlist(r io.Reader) map[string]bool {
	out := map[string]bool{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out[line] = true
	}
	return out
}

// loadAllowlist reads the prose registry file. A missing file is empty (no
// compound is known yet) — not an error.
func loadAllowlist(path string) map[string]bool {
	fh, err := os.Open(path)
	if err != nil {
		return map[string]bool{}
	}
	defer fh.Close()
	return parseAllowlist(fh)
}

// runSeedCmd runs the seed command (cwd=dir) and parses its stdout as a slug
// list — the plugin point that lets a project supply a bespoke catalog→allow-list
// seeder (e.g. `cupel vocab-seed`) without calque knowing the catalog shape.
func runSeedCmd(seedCmd, dir string) (map[string]bool, error) {
	cmd := exec.Command("sh", "-c", seedCmd)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseAllowlist(bytes.NewReader(out)), nil
}
