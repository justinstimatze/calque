package main

// migrate-registry — convert a Python-era calque registry to the Go format.
//
// The Python calque wrote entries as a `## <id> — <verdict>` header plus
// `- left:` / `- right:` lines; the Go parser keys on `- pair: <left> | <right>`
// (+ `- verdict:` / `- reviewed:`). A registry in the old format parses to ZERO
// entries — so every prior adjudication goes invisible and `check` cries wolf on
// the whole repo. Old projects (anything adjudicated before the Go rewrite) need
// this one-time migration.
//
// The conversion is conservative: it PRESERVES all human prose (headers, notes,
// signal/predicted/policy lines) and only (a) inserts the `- pair:` line the Go
// parser needs, and (b) normalizes the `- verdict:` and `- reviewed:` lines to
// their canonical tokens. Idempotent — a file already in the new format (any
// `- pair:` line) is left untouched.

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func runMigrateRegistry(args []string) {
	fs := flag.NewFlagSet("migrate-registry", flag.ContinueOnError)
	in := fs.String("in", ".calque/registry.md", "registry file to migrate (old Python-era format)")
	out := fs.String("out", "", "write the migrated registry here (default: stdout)")
	write := fs.Bool("write", false, "overwrite --in in place (writes a .bak backup first)")
	if err := fs.Parse(args); err != nil {
		return
	}

	data, err := os.ReadFile(*in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque migrate-registry: %v\n", err)
		os.Exit(1)
	}
	migrated, n, alreadyNew := migrateRegistry(string(data))
	if alreadyNew {
		fmt.Fprintf(os.Stderr, "calque migrate-registry: %s already has `- pair:` lines (new format) — nothing to do\n", *in)
		return
	}
	if n == 0 {
		fmt.Fprintf(os.Stderr, "calque migrate-registry: no old-format entries (`- left:`/`- right:`) found in %s\n", *in)
		return
	}

	switch {
	case *write:
		bak := *in + ".bak"
		if err := os.WriteFile(bak, data, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "calque migrate-registry: writing backup %s: %v\n", bak, err)
			os.Exit(1)
		}
		if err := os.WriteFile(*in, []byte(migrated), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "calque migrate-registry: writing %s: %v\n", *in, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "calque migrate-registry: migrated %d entr%s in %s (backup: %s)\n",
			n, plural(n, "y", "ies"), *in, bak)
	case *out != "":
		if err := os.WriteFile(*out, []byte(migrated), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "calque migrate-registry: writing %s: %v\n", *out, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "calque migrate-registry: wrote %d entr%s to %s\n", n, plural(n, "y", "ies"), *out)
	default:
		fmt.Print(migrated)
		fmt.Fprintf(os.Stderr, "calque migrate-registry: %d entr%s migrated (dry run — use --write to overwrite %s)\n",
			n, plural(n, "y", "ies"), *in)
	}
}

var reISODate = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

// migrateRegistry rewrites old-format registry text to the Go format. It returns
// the migrated text, the number of pair entries converted, and whether the input
// was already in the new format (any `- pair:` line → left untouched).
func migrateRegistry(src string) (migrated string, nEntries int, alreadyNew bool) {
	lines := strings.Split(src, "\n")
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "- pair:") || strings.HasPrefix(strings.TrimSpace(l), "- cluster:") {
			return src, 0, true
		}
	}

	var b strings.Builder
	var lastLeft string
	inFence := false
	for _, l := range lines {
		t := strings.TrimSpace(l)
		// Never transform inside a ``` code fence — the registry's own template
		// example lives there and must not become a literal `<file>` entry.
		if strings.HasPrefix(t, "```") {
			inFence = !inFence
			b.WriteString(l + "\n")
			continue
		}
		if inFence {
			b.WriteString(l + "\n")
			continue
		}
		switch {
		case strings.HasPrefix(t, "- left:"):
			lastLeft = strings.TrimSpace(strings.TrimPrefix(t, "- left:"))
			b.WriteString(l + "\n")
		case strings.HasPrefix(t, "- right:"):
			b.WriteString(l + "\n")
			right := strings.TrimSpace(strings.TrimPrefix(t, "- right:"))
			if lastLeft != "" && right != "" {
				// Insert the line the Go parser keys on, right after the pair's
				// two halves are both known.
				b.WriteString("- pair: " + lastLeft + " | " + right + "\n")
				nEntries++
			}
			lastLeft = ""
		case strings.HasPrefix(t, "- verdict:"):
			b.WriteString("- verdict: " + normalizeVerdict(strings.TrimPrefix(t, "- verdict:")) + "\n")
		case strings.HasPrefix(t, "- reviewed:"):
			b.WriteString("- reviewed: " + normalizeReviewed(strings.TrimPrefix(t, "- reviewed:")) + "\n")
		default:
			b.WriteString(l + "\n")
		}
	}
	// strings.Split on a trailing "\n" produced a final empty element; the loop
	// re-added one "\n" per line, so trim the doubled trailing newline.
	return strings.TrimSuffix(b.String(), "\n"), nEntries, false
}

// normalizeVerdict reduces a free-form old verdict (e.g.
// "contracted-twin-ok (collapsed) — was drift") to its canonical token. Priority
// order matters: "was drift" must not match before "contracted-twin-ok".
func normalizeVerdict(s string) string {
	low := strings.ToLower(s)
	for _, v := range []string{"contracted-twin-ok", "false-alarm", "drift"} {
		if strings.Contains(low, v) {
			return v
		}
	}
	return strings.TrimSpace(s)
}

// normalizeReviewed extracts the ISO date from an old reviewed line (e.g.
// "2026-06-05 by calque-oracle; FIXED 2026-06-05 by Claude" → "2026-06-05").
func normalizeReviewed(s string) string {
	if m := reISODate.FindString(s); m != "" {
		return m
	}
	return strings.TrimSpace(s)
}
