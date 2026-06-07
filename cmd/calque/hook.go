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
	if err := fs.Parse(args); err != nil {
		return
	}

	checkCmd := buildCheckCmd(*exclude, *strict, *left, *right)

	switch action {
	case "install":
		installPreCommit(*repo, checkCmd)
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

func installPreCommit(repo, checkCmd string) {
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
	hookPath := filepath.Join(hooksDir, "pre-commit")

	// Never clobber an existing hook — that's someone else's logic.
	if existing, err := os.ReadFile(hookPath); err == nil {
		if strings.Contains(string(existing), "calque") {
			fmt.Printf("calque hook: already installed at %s — nothing to do.\n", hookPath)
			return
		}
		fmt.Printf("calque hook: a pre-commit hook already exists at %s and is left untouched.\n", hookPath)
		fmt.Printf("Add this line to it to enable the gate:\n\n  %s\n\n", checkCmd)
		return
	}

	body := preCommitScript(checkCmd)
	if err := os.WriteFile(hookPath, []byte(body), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "calque hook: writing %s: %v\n", hookPath, err)
		os.Exit(1)
	}
	fmt.Printf("calque hook: installed pre-commit gate at %s\n\n%s\n", hookPath, body)
	fmt.Println("It runs:", checkCmd)
	fmt.Println("Warn-only unless you passed --strict. Uninstall: rm", hookPath)
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
