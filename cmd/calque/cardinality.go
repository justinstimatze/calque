package main

// cardinality — the role-cardinality gate (DESIGN_NOTES §18), calque's
// declare-and-gate axis. It reads role declarations from the registry ("role R should
// have Expected implementations"), enumerates each role's implementers across the repo
// via its predicate, and flags any role whose live count exceeds Expected — or, when a
// frozen Baseline is declared, any implementer outside it (the ratchet, §18.7).
//
// Unlike scan/check (discover-similar-after-the-fact), this enforces a FORWARD
// declaration, so it catches the dual paths pairwise similarity misses by construction:
// implementations that share no footprint (a stub vs the real path; two different-API
// backends), and recurrence (a re-added implementation — nothing else forbids one).

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/justinstimatze/calque/internal/code"
	"github.com/justinstimatze/calque/internal/registry"
)

// roleResult is the cardinality verdict for one declared role — the pure unit the gate
// reports on, separated from printing + os.Exit so a test (and a future MCP tool) can
// share the same core as the CLI.
type roleResult struct {
	Role         registry.RoleEntry
	Impls        []*code.FuncSig
	Novel        []string // implementer keys outside a non-empty Baseline (the ratchet)
	BadPredicate error
}

// Vacuous reports a declared role whose predicate matched NO implementers despite
// expecting at least one — a stale declaration (renamed/deleted implementation) or a
// typo'd predicate. Without this guard such a role passes silently (0 > Expected is
// false), so the gate would be *vacuously green*: the worst failure for a gate, because
// the invariant looks enforced while checking nothing. A role with `- expected: 0` (a
// deliberate ban) that matches nothing is correct, not vacuous.
func (rr roleResult) Vacuous() bool {
	return rr.BadPredicate == nil && rr.Role.Expected >= 1 && len(rr.Impls) == 0
}

// Violation reports whether the role's declared cardinality is broken: a malformed
// predicate, a vacuous (zero-match) declaration, more implementers than Expected, or
// any implementer past a frozen baseline.
func (rr roleResult) Violation() bool {
	return rr.BadPredicate != nil || rr.Vacuous() || len(rr.Impls) > rr.Role.Expected || len(rr.Novel) > 0
}

// computeCardinality evaluates every declared role against the extracted signatures.
// Pure: no I/O, no exit.
func computeCardinality(sigs []*code.FuncSig, roles []registry.RoleEntry) []roleResult {
	out := make([]roleResult, 0, len(roles))
	for _, role := range roles {
		rr := roleResult{Role: role}
		impls, err := code.Implementers(role.Predicate, sigs)
		if err != nil {
			rr.BadPredicate = err
			out = append(out, rr)
			continue
		}
		rr.Impls = impls
		if len(role.Baseline) > 0 {
			base := make(map[string]bool, len(role.Baseline))
			for _, k := range role.Baseline {
				base[k] = true
			}
			for _, f := range impls {
				if !base[f.Key()] {
					rr.Novel = append(rr.Novel, f.Key())
				}
			}
		}
		out = append(out, rr)
	}
	return out
}

func runCardinality(args []string) {
	fs := flag.NewFlagSet("cardinality", flag.ContinueOnError)
	repo := fs.String("repo", ".", "repo root to scan")
	exclude := fs.String("exclude", "", "comma-separated glob(s) to skip entirely (e.g. legacy/**,vendor/**)")
	regPath := fs.String("registry", ".calque/registry.md", "registry file (role declarations)")
	strict := fs.Bool("strict", false, "exit 1 if any declared role's cardinality is violated")
	if err := fs.Parse(args); err != nil {
		return
	}

	sigs, st, err := code.Extract(*repo, splitCSV(*exclude))
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque cardinality: walking %s: %v\n", *repo, err)
		os.Exit(1)
	}
	reg, err := registry.Load(joinRepo(*repo, *regPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque cardinality: reading registry: %v\n", err)
		os.Exit(1)
	}

	res := computeCardinality(sigs, reg.Roles)
	fmt.Print(renderCardinality(reg.Roles, res, st, *regPath))

	if *strict && countViolations(res) > 0 {
		os.Exit(1)
	}
}

func countViolations(res []roleResult) int {
	n := 0
	for _, rr := range res {
		if rr.Violation() {
			n++
		}
	}
	return n
}

func renderCardinality(roles []registry.RoleEntry, res []roleResult, st code.ScanStats, regPath string) string {
	var b strings.Builder
	b.WriteString("# calque — role cardinality\n\n")
	if len(roles) == 0 {
		fmt.Fprintf(&b, "no roles declared in %s.\n\n", regPath)
		b.WriteString("Declare one with a `- role:` / `- predicate:` / `- expected:` block (see DESIGN_NOTES §18):\n\n")
		b.WriteString("    ## role: input-constructor\n")
		b.WriteString("    - role: input-constructor\n")
		b.WriteString("    - predicate: name:/[Cc]onstruct/ calls:_dispatch\n")
		b.WriteString("    - expected: 1\n")
		return b.String()
	}
	fmt.Fprintf(&b, "scanned %d func(s) in %d file(s); %d role(s) declared.\n\n", st.Funcs, st.Files, len(roles))

	for _, rr := range res {
		if rr.BadPredicate != nil {
			fmt.Fprintf(&b, "## %s — BAD PREDICATE: %v\n\n", rr.Role.Name, rr.BadPredicate)
			continue
		}
		status := "ok ✓"
		switch {
		case rr.Vacuous():
			status = "VACUOUS"
		case rr.Violation():
			status = "VIOLATION"
		}
		fmt.Fprintf(&b, "## %s — %s (expected %d, found %d)\n", rr.Role.Name, status, rr.Role.Expected, len(rr.Impls))
		if rr.Vacuous() {
			b.WriteString("  predicate matched 0 implementers — stale declaration (renamed/deleted) or a typo'd predicate?\n")
		}
		novel := make(map[string]bool, len(rr.Novel))
		for _, k := range rr.Novel {
			novel[k] = true
		}
		ordered := append([]*code.FuncSig(nil), rr.Impls...)
		sort.Slice(ordered, func(i, j int) bool { return ordered[i].Key() < ordered[j].Key() })
		for _, f := range ordered {
			mark := " "
			if novel[f.Key()] {
				mark = "+" // outside the frozen baseline (the ratchet)
			}
			fmt.Fprintf(&b, "- [%s] `%s` (%s:%d)\n", mark, f.Qualname, f.File, f.Line)
		}
		if len(rr.Novel) > 0 {
			fmt.Fprintf(&b, "  ratchet: %d implementer(s) outside the frozen baseline.\n", len(rr.Novel))
		}
		b.WriteString("\n")
	}

	if n := countViolations(res); n == 0 {
		b.WriteString("all declared roles within their cardinality. ✓\n")
	} else {
		fmt.Fprintf(&b, "%d role(s) violate their declared cardinality. Over-count → collapse the extra\n", n)
		b.WriteString("implementations to a single path (or raise `- expected:` / record a `- baseline:` if a\n")
		b.WriteString("second path is genuinely required). Vacuous → fix or remove the stale declaration.\n")
	}
	return b.String()
}
