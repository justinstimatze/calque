package main

// Layer D label store — the structured training rows behind `doctor --ablate`.
// A --judge run accumulates verdicts in two places: the judge DISK CACHE
// (content-hashed Verdict only — no provenance, keyed for free re-runs) and
// HERE, the label store, which tags each verdict with the detector that
// surfaced it plus the language/variety it came from. The cache makes re-runs
// free; the store makes them MEASURABLE. Global (colocated with the judge
// cache) so labels compound across repos, and so a --judge run against a
// read-only adopter repo writes nothing into that repo.

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/justinstimatze/calque/internal/code"
	"github.com/justinstimatze/calque/internal/llm"
)

// Label is one adjudicated candidate joined to its provenance: which detector
// fired, on what language, and (when the Sig carries it) what op-type variety.
// Distinct from a registry verdict (per-repo, human-curated, drives the gate)
// and from the judge cache (verdict only). Deduped on read by detector+a+b.
type Label struct {
	Ts         string `json:"ts"`
	Repo       string `json:"repo"`
	Detector   string `json:"detector"` // SigCandidate.Kind: read-set | signature | name-stem | confess | ...
	Lang       string `json:"lang"`
	Variety    string `json:"variety,omitempty"` // op-type pair parsed from the Sig, when present
	AKey       string `json:"a"`
	BKey       string `json:"b"`
	Verdict    string `json:"verdict"` // drift | contracted-twin-ok | false-alarm
	Confidence string `json:"confidence,omitempty"`
}

// labelStorePath is the global label log, colocated with the judge cache so a
// --judge run anywhere appends here — never into the scanned repo (adopters
// stay read-only). CALQUE_LABELS overrides.
func labelStorePath() string {
	if p := os.Getenv("CALQUE_LABELS"); p != "" {
		return p
	}
	if d, err := os.UserCacheDir(); err == nil {
		return filepath.Join(d, "calque", "labels.jsonl")
	}
	return filepath.Join(os.TempDir(), "calque-labels.jsonl")
}

// recordLabel appends one verdict to the global store. Best-effort: a logging
// failure must never break a --judge run. Duplicates on re-run are fine —
// loadLabels keeps the latest per detector+a+b.
func recordLabel(repo string, c code.SigCandidate, v llm.Verdict) {
	_ = appendJSONL(labelStorePath(), Label{
		Ts:         nowTs(),
		Repo:       filepath.Base(strings.TrimRight(repo, "/")),
		Detector:   c.Kind,
		Lang:       langOf(c.A.File),
		Variety:    varietyOf(c.Sig),
		AKey:       c.A.File + "::" + c.A.Qualname,
		BKey:       c.B.File + "::" + c.B.Qualname,
		Verdict:    v.Class,
		Confidence: v.Confidence,
	})
}

// langOf maps a file path to a coarse language tag by extension.
func langOf(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".ts", ".tsx", ".mts", ".cts":
		return "typescript"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "javascript"
	case ".rs":
		return "rust"
	default:
		return "other"
	}
}

// varietyOf pulls the op-type pair calque already prints in a derivation Sig
// (e.g. "reads≈0.83 [mutate/forward-map] {...}") as the candidate's variety —
// the finest free provenance available. "" when the Sig carries no [a/b] tag.
func varietyOf(sig string) string {
	i := strings.IndexByte(sig, '[')
	if i < 0 {
		return ""
	}
	rest := sig[i+1:]
	j := strings.IndexByte(rest, ']')
	if j < 0 {
		return ""
	}
	return rest[:j]
}
