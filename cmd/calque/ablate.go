package main

// Layer D — `doctor --ablate`. Rolls the global label store (labels.go) into a
// per-detector × language × variety matrix and asks the only question that
// matters: does each detector PULL ITS WEIGHT? A detector that surfaces mostly
// false-alarms on a given language is noise there and should be gated down or
// dropped. "Real twin" = drift OR contracted-twin-ok (both are genuine shared
// contracts the detector correctly found); false-alarm is the miss. Precision =
// real / total. The verdict needs support — a cell with n<minSupport is
// reported INSUFFICIENT, never PRUNE, so a thin corpus can't kill a detector.

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	ablateMinSupport  = 5    // below this, a cell can't earn a keep/prune verdict
	ablatePullsWeight = 0.50 // precision at/above this (with support) ⇒ pulls weight
)

type ablateCell struct {
	drift, twinOK, falseAlarm int
}

func (c ablateCell) total() int { return c.drift + c.twinOK + c.falseAlarm }
func (c ablateCell) real() int  { return c.drift + c.twinOK }
func (c ablateCell) precision() float64 {
	if c.total() == 0 {
		return 0
	}
	return float64(c.real()) / float64(c.total())
}

func (c *ablateCell) add(verdict string) {
	switch verdict {
	case "drift":
		c.drift++
	case "contracted-twin-ok":
		c.twinOK++
	default:
		c.falseAlarm++
	}
}

// loadLabels reads the global store, keeping the latest verdict per
// detector+a+b (re-runs append; latest wins, matching loadFireTags).
func loadLabels(path string) []Label {
	latest := map[string]Label{}
	var order []string
	scanJSONL(path, func(line []byte) {
		var l Label
		if json.Unmarshal(line, &l) != nil || l.Detector == "" {
			return
		}
		k := l.Detector + "\x00" + l.AKey + "\x00" + l.BKey
		if _, seen := latest[k]; !seen {
			order = append(order, k)
		}
		latest[k] = l
	})
	out := make([]Label, 0, len(order))
	for _, k := range order {
		out = append(out, latest[k])
	}
	return out
}

func ablateVerdict(c ablateCell) string {
	switch {
	case c.total() < ablateMinSupport:
		return "insufficient"
	case c.precision() >= ablatePullsWeight:
		return "pulls-weight"
	default:
		return "prune?"
	}
}

func runAblate() {
	labels := loadLabels(labelStorePath())
	if len(labels) == 0 {
		fmt.Println("calque doctor --ablate — no labels yet.")
		fmt.Printf("store: %s\n", labelStorePath())
		fmt.Println("grow it: run `calque propose-deriv --judge` (or confess --judge) on a repo.")
		return
	}

	// Aggregate three views: per detector×lang×variety (the matrix), per
	// detector×lang (the row a maintainer acts on), and per detector (the call).
	type key struct{ detector, lang, variety string }
	matrix := map[key]*ablateCell{}
	byDetLang := map[string]*ablateCell{}
	byDet := map[string]*ablateCell{}
	getCell := func(m map[string]*ablateCell, k string) *ablateCell {
		if m[k] == nil {
			m[k] = &ablateCell{}
		}
		return m[k]
	}
	for _, l := range labels {
		mk := key{l.Detector, l.Lang, l.Variety}
		if matrix[mk] == nil {
			matrix[mk] = &ablateCell{}
		}
		matrix[mk].add(l.Verdict)
		getCell(byDetLang, l.Detector+" · "+l.Lang).add(l.Verdict)
		getCell(byDet, l.Detector).add(l.Verdict)
	}

	fmt.Println("calque doctor --ablate — per-detector label matrix (Layer D)")
	fmt.Println(strings.Repeat("─", 64))
	fmt.Printf("store: %s  ·  %d labelled pair(s)\n", labelStorePath(), len(labels))
	fmt.Printf("real twin = drift+twin-ok · precision = real/total · support≥%d to rule\n\n", ablateMinSupport)

	// Per detector × language, the actionable row, with the full matrix nested.
	fmt.Printf("%-12s %-11s %-9s %5s %5s %5s %5s  %-13s\n",
		"detector", "lang", "variety", "n", "drft", "twin", "fa", "verdict")
	dls := sortedCellKeys(byDetLang)
	for _, dl := range dls {
		cell := byDetLang[dl]
		parts := strings.SplitN(dl, " · ", 2)
		det, lang := parts[0], parts[1]
		fmt.Printf("%-12s %-11s %-9s %5d %5d %5d %5d  %-13s\n",
			det, lang, "—", cell.total(), cell.drift, cell.twinOK, cell.falseAlarm, ablateVerdict(*cell))
		// Nested varieties for this detector×lang, when any are tagged.
		var vks []key
		for k := range matrix {
			if k.detector == det && k.lang == lang && k.variety != "" {
				vks = append(vks, k)
			}
		}
		sort.Slice(vks, func(i, j int) bool { return vks[i].variety < vks[j].variety })
		for _, k := range vks {
			c := matrix[k]
			fmt.Printf("%-12s %-11s %-9s %5d %5d %5d %5d  %-13s\n",
				"", "", k.variety, c.total(), c.drift, c.twinOK, c.falseAlarm, ablateVerdict(*c))
		}
	}

	// Per-detector call: keep, prune, or buy more labels.
	fmt.Println("\nper-detector verdict (across all languages):")
	for _, det := range sortedCellKeys(byDet) {
		c := byDet[det]
		fmt.Printf("  %-12s n=%-3d precision=%.2f  → %s\n", det, c.total(), c.precision(), ablateVerdict(*c))
	}
	fmt.Println("\n  pulls-weight: precision ≥ 0.50 with support ≥ 5")
	fmt.Println("  prune?:       precision < 0.50 with support ≥ 5 — gate harder or drop on this slice")
	fmt.Println("  insufficient: n < 5 — buy more labels before ruling (propose-deriv/confess --judge)")
}

func sortedCellKeys(m map[string]*ablateCell) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
