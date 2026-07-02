package main

import (
	"fmt"
	"strings"

	"github.com/justinstimatze/calque/internal/companion"
)

// renderCompanions formats calque's belt-and-suspenders pass: nothing here
// touches scorePair, the registry, or any gate — it's a second, independent
// axis (Type-1/2 textual clones) that calque's own Type-4 engine explicitly
// does not cover. A tool absent from $PATH is listed with an install hint,
// never fetched. Pure/testable, mirroring renderCheck's compute→render split.
func renderCompanions(sections []companion.Section) string {
	var b strings.Builder
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "# companion pass — Type-1/2 (textual) clones, not calque's own axis")
	fmt.Fprintln(&b)
	for _, s := range sections {
		if !s.Ran {
			fmt.Fprintf(&b, "- %s: %s\n", s.Tool, s.Skip)
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", s.Tool)
		if s.Output == "" {
			fmt.Fprintln(&b, "(no duplicates reported)")
		} else {
			fmt.Fprintln(&b, s.Output)
		}
		fmt.Fprintln(&b)
	}
	return b.String()
}

// printCompanions runs the companion pass against repo and prints its report.
func printCompanions(repo string) {
	fmt.Print(renderCompanions(companion.Run(repo)))
}
