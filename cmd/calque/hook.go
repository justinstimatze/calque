package main

// hook — make calque's gate ongoing/automated (the keystone of real use). Wires
// `calque check` into a git pre-commit hook (auto-installed, local, reversible)
// and prints the Claude Code Stop-hook snippet. Modelled on the slimemold/cupel
// hook shape: warn-only by default (never blocks a commit), --strict to gate.

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runHook(args []string) {
	// First positional selects the action: `install` writes the git hook;
	// anything else (or nothing) just shows the snippets.
	action := "show"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		action = args[0]
		args = args[1:]
	}

	fs := flag.NewFlagSet("hook", flag.ContinueOnError)
	b := addBoundaryFlags(fs)
	repo, left, right, exclude := b.repo, b.left, b.right, b.exclude
	strict := fs.Bool("strict", false, "make the gate BLOCK the commit on new suspects (default: warn-only)")
	postMerge := fs.Bool("post-merge", false, "also install a post-merge hook that scans pulled/merged code (closes the contributor-merge gap; always warn-only)")
	if err := fs.Parse(args); err != nil {
		return
	}

	checkCmd := buildCheckCmd(*exclude, *strict, *left, *right)

	switch action {
	case "install":
		installPreCommit(*repo, checkCmd)
		if *postMerge {
			// A post-merge hook can't block an already-completed merge, so it
			// runs warn-only regardless of --strict.
			installPostMerge(*repo, buildCheckCmd(*exclude, false, *left, *right))
		}
	case "show":
		showHookSnippets(*repo, checkCmd)
	default:
		fmt.Fprintf(os.Stderr, "calque hook: unknown action %q (use `install` or omit to show)\n", action)
		os.Exit(2)
	}
}

// buildCheckCmd assembles the `calque check ...` invocation the hook will run.
func buildCheckCmd(exclude string, strict bool, left, right string) string {
	parts := []string{"calque", "check"}
	if strict {
		parts = append(parts, "--strict")
	}
	if exclude != "" {
		parts = append(parts, "--exclude", shellQuote(exclude))
	}
	if left != "" {
		parts = append(parts, "--left", shellQuote(left))
	}
	if right != "" {
		parts = append(parts, "--right", shellQuote(right))
	}
	return strings.Join(parts, " ")
}

// preCommitScript renders the pre-commit hook body. It no-ops (exit 0) when the
// calque binary isn't on PATH, so a teammate without calque can still commit.
func preCommitScript(checkCmd string) string {
	return "#!/bin/sh\n" +
		"# calque drift gate — added by `calque hook install`.\n" +
		"# Surfaces NEW/drifted dual-path suspects vs .calque/registry.md.\n" +
		"# Remove this block (or the file) to uninstall.\n" +
		"command -v calque >/dev/null 2>&1 || { echo 'calque not on PATH; skipping drift gate'; exit 0; }\n" +
		checkCmd + "\n"
}

// postMergeScript renders the post-merge hook body. Git runs post-merge after a
// merge or pull updates the working tree — including fast-forward merges, which
// no pre-commit hook ever sees (no commit of yours happens). It scans the code
// that just landed for dual-path suspects vs .calque/registry.md, closing the
// contributor-merge gap the pre-commit gate can't reach. Always warn-only: the
// merge has already happened, so there is nothing to block — it only reports.
// No-ops (exit 0) when calque isn't on PATH so a teammate without it can pull.
func postMergeScript(checkCmd string) string {
	return "#!/bin/sh\n" +
		"# calque drift scan (post-merge) — added by `calque hook install --post-merge`.\n" +
		"# After a pull/merge (incl. fast-forward), surfaces NEW dual-path suspects\n" +
		"# the merged code introduced vs .calque/registry.md. Warn-only by nature\n" +
		"# (the merge already happened). Remove this file to uninstall.\n" +
		"command -v calque >/dev/null 2>&1 || { echo 'calque not on PATH; skipping drift scan'; exit 0; }\n" +
		"echo 'calque: scanning merged code for new dual-path drift…'\n" +
		checkCmd + "\n"
}

// installGitHook writes body to <gitdir>/hooks/<name>, never clobbering an
// existing non-calque hook. Shared by the pre-commit gate and the post-merge
// scan so the two installers can't drift apart — the tool eats its own dog
// food. Returns the hook path and whether a fresh hook was written (false when
// it already existed or a foreign hook was left untouched).
func installGitHook(repo, name, body, checkCmd, kindLabel string) (string, bool) {
	gitDir, err := absoluteGitDir(repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque hook: %s is not a git repo (%v)\n", repo, err)
		os.Exit(1)
	}
	hooksDir := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "calque hook: creating %s: %v\n", hooksDir, err)
		os.Exit(1)
	}
	hookPath := filepath.Join(hooksDir, name)

	// Never clobber an existing hook — that's someone else's logic.
	if existing, err := os.ReadFile(hookPath); err == nil {
		if strings.Contains(string(existing), "calque") {
			fmt.Printf("calque hook: %s already installed at %s — nothing to do.\n", name, hookPath)
			return hookPath, false
		}
		fmt.Printf("calque hook: a %s hook already exists at %s and is left untouched.\n", name, hookPath)
		fmt.Printf("Add this line to it to enable the %s:\n\n  %s\n\n", kindLabel, checkCmd)
		return hookPath, false
	}

	if err := os.WriteFile(hookPath, []byte(body), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "calque hook: writing %s: %v\n", hookPath, err)
		os.Exit(1)
	}
	fmt.Printf("calque hook: installed %s at %s\n\n%s\n", kindLabel, hookPath, body)
	return hookPath, true
}

func installPreCommit(repo, checkCmd string) {
	hookPath, wrote := installGitHook(repo, "pre-commit", preCommitScript(checkCmd), checkCmd, "pre-commit gate")
	if wrote {
		fmt.Println("It runs:", checkCmd)
		fmt.Println("Warn-only unless you passed --strict. Uninstall: rm", hookPath)
	}
}

func installPostMerge(repo, checkCmd string) {
	hookPath, wrote := installGitHook(repo, "post-merge", postMergeScript(checkCmd), checkCmd, "post-merge scan")
	if wrote {
		fmt.Println("It runs after every pull/merge (incl. fast-forward):", checkCmd)
		fmt.Println("Warn-only — it reports, never blocks. Uninstall: rm", hookPath)
	}
}

func showHookSnippets(repo, checkCmd string) {
	abs, _ := filepath.Abs(repo)
	fmt.Println("# calque hook — make the gate ongoing")
	fmt.Println()
	fmt.Println("## git pre-commit (auto-install)")
	fmt.Println()
	fmt.Println("    calque hook install --repo", repo, "[--exclude '…'] [--strict]")
	fmt.Println()
	fmt.Println("Installs", filepath.Join(".git", "hooks", "pre-commit"), "running:")
	fmt.Println()
	fmt.Println("   ", checkCmd)
	fmt.Println()
	fmt.Println("## git post-merge (scan incoming code — add --post-merge)")
	fmt.Println()
	fmt.Println("    calque hook install --post-merge --repo", repo, "[--exclude '…']")
	fmt.Println()
	fmt.Println("Also installs", filepath.Join(".git", "hooks", "post-merge")+",", "which runs after every")
	fmt.Println("pull/merge — including fast-forward, which no pre-commit hook sees. Scans the")
	fmt.Println("code a contributor just merged for new dual-path drift. Always warn-only.")
	fmt.Println()
	fmt.Println("## Claude Code Stop hook (.claude/settings.json)")
	fmt.Println()
	fmt.Println("Runs the gate at the end of each agent turn (warn-only is best here):")
	fmt.Println()
	fmt.Print(stopHookSnippet(abs, checkCmd))
}

// stopHookSnippet renders the Claude Code settings.json block for a Stop hook.
func stopHookSnippet(absRepo, checkCmd string) string {
	// In a Stop hook the cwd may differ, so pin --repo to the absolute path.
	cmd := strings.Replace(checkCmd, "calque check", "calque check --repo "+shellQuote(absRepo), 1)
	return `    {
      "hooks": {
        "Stop": [
          { "matcher": "", "hooks": [
            { "type": "command", "command": ` + jsonQuote(cmd+" 2>&1 | tail -8") + ` }
          ] }
        ]
      }
    }
`
}

// absoluteGitDir returns the absolute .git directory for repo, resolving
// worktrees and submodules correctly (via git itself).
func absoluteGitDir(repo string) (string, error) {
	cmd := exec.Command("git", "-C", repo, "rev-parse", "--absolute-git-dir")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// shellQuote single-quotes a value for a /bin/sh command line (the values here
// are globs, never containing single quotes).
func shellQuote(s string) string {
	if s == "" || strings.ContainsAny(s, " *?,") {
		return "'" + s + "'"
	}
	return s
}

// jsonQuote renders s as a JSON string literal (for the settings.json snippet).
func jsonQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
