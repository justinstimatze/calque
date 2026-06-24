package main

import (
	"strings"
	"testing"
)

func TestBuildCheckCmd(t *testing.T) {
	cases := []struct {
		name                       string
		exclude, left, right       string
		strict                     bool
		wantContains, wantExcludes []string
	}{
		{
			name:         "warn-only default has no --strict",
			wantContains: []string{"calque check"},
			wantExcludes: []string{"--strict", "--exclude", "--left", "--right"},
		},
		{
			name:         "strict gate",
			strict:       true,
			wantContains: []string{"calque check", "--strict"},
		},
		{
			name:         "exclude is shell-quoted",
			exclude:      "legacy/**,**/*_test.go",
			wantContains: []string{"--exclude 'legacy/**,**/*_test.go'"},
		},
		{
			name:         "boundary globs",
			left:         "engine*.py",
			right:        "testing.py",
			wantContains: []string{"--left 'engine*.py'", "--right testing.py"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := buildCheckCmd(c.exclude, c.strict, c.left, c.right)
			for _, w := range c.wantContains {
				if !strings.Contains(got, w) {
					t.Errorf("buildCheckCmd = %q, want substring %q", got, w)
				}
			}
			for _, w := range c.wantExcludes {
				if strings.Contains(got, w) {
					t.Errorf("buildCheckCmd = %q, should not contain %q", got, w)
				}
			}
		})
	}
}

// The generated hook must no-op (not block commits) when calque isn't on PATH.
func TestPreCommitScriptGracefulWhenMissing(t *testing.T) {
	s := preCommitScript("calque check --strict")
	if !strings.HasPrefix(s, "#!/bin/sh\n") {
		t.Errorf("hook must start with a shebang, got %q", s)
	}
	if !strings.Contains(s, "command -v calque") || !strings.Contains(s, "exit 0") {
		t.Errorf("hook must skip gracefully when calque is absent, got:\n%s", s)
	}
	if !strings.Contains(s, "calque check --strict") {
		t.Errorf("hook must run the gate command, got:\n%s", s)
	}
}

// The post-merge scan must no-op when calque is absent, run the gate, and never
// carry --strict (a merge is already done — there is nothing to block).
func TestPostMergeScriptGracefulWhenMissing(t *testing.T) {
	s := postMergeScript("calque check --exclude 'legacy/**'")
	if !strings.HasPrefix(s, "#!/bin/sh\n") {
		t.Errorf("hook must start with a shebang, got %q", s)
	}
	if !strings.Contains(s, "command -v calque") || !strings.Contains(s, "exit 0") {
		t.Errorf("hook must skip gracefully when calque is absent, got:\n%s", s)
	}
	if !strings.Contains(s, "calque check --exclude 'legacy/**'") {
		t.Errorf("hook must run the scan command, got:\n%s", s)
	}
	if strings.Contains(s, "--strict") {
		t.Errorf("post-merge scan must be warn-only (no --strict), got:\n%s", s)
	}
}

func TestShellQuote(t *testing.T) {
	if got := shellQuote("legacy"); got != "legacy" {
		t.Errorf("plain word should be unquoted, got %q", got)
	}
	if got := shellQuote("legacy/**,**/*_test.go"); got != "'legacy/**,**/*_test.go'" {
		t.Errorf("glob with metachars should be quoted, got %q", got)
	}
}

func TestJSONQuote(t *testing.T) {
	if got := jsonQuote(`a "b" \c`); got != `"a \"b\" \\c"` {
		t.Errorf("jsonQuote escaping wrong, got %q", got)
	}
}
