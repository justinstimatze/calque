package main

// calib — the calibration leg of the spine (DESIGN_NOTES §3/§9 #5). A "fire" is a
// suspect the gate surfaces; `mark-fire` tags it useful|mixed|not-useful, and
// `doctor` rolls up whether the SCORE actually tracks real drift — the only way to
// know the ranker earns its keep (and, later, to tune weights from data).
//
// calque's registry verdicts double as calibration labels (drift→useful,
// false-alarm→not-useful, contracted-twin-ok→mixed), so doctor needs no manual
// tagging for already-adjudicated suspects; fire-tags.jsonl supplements the rest.
//
// Shape ported from cupel cmd/cupel/calib.go (MIT, attribution preserved); the
// fire/verdict model is the same, the recall substrate (code suspects vs prose
// hook events) differs.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/justinstimatze/calque/internal/code"
	"github.com/justinstimatze/calque/internal/llm"
	"github.com/justinstimatze/calque/internal/pairkey"
	"github.com/justinstimatze/calque/internal/registry"
)

type fireTag struct {
	Ts      string `json:"ts"`
	ID      string `json:"id"`
	Verdict string `json:"verdict"`
	Notes   string `json:"notes,omitempty"`
}

// fireRecord is one logged gate fire (a NEW suspect at the time check ran).
type fireRecord struct {
	Ts    string  `json:"ts"`
	ID    string  `json:"id"`
	Kind  string  `json:"kind"` // pair | cluster
	Key   string  `json:"key"`  // human-readable member key(s)
	Score float64 `json:"score"`
}

var validVerdicts = map[string]bool{"useful": true, "mixed": true, "not-useful": true}

func calqueDir(repo string) string { return filepath.Join(repo, ".calque") }
func nowTs() string                { return time.Now().UTC().Format(time.RFC3339) }

// fireID is a stable id for a suspect, derived from its kind + canonical key, so
// the same suspect gets the same id across runs — `mark-fire <id>` sticks even as
// check re-fires.
func fireID(kind, key string) string {
	sum := sha256.Sum256([]byte(kind + "\x00" + key))
	return hex.EncodeToString(sum[:])[:10]
}

func pairID(s code.Suspicion) string {
	return fireID("pair", pairkey.Key(s.Left.Key(), s.Right.Key()))
}
func clusterID(c code.Cluster) string { return fireID("cluster", c.Key()) }

func pairDisplayKey(s code.Suspicion) string  { return s.Left.Key() + " | " + s.Right.Key() }
func clusterDisplayKey(c code.Cluster) string { return strings.Join(c.MemberKeys(), " | ") }

// verdictLabel maps a registry verdict class to a calibration label.
func verdictLabel(vclass string) string {
	switch vclass {
	case llm.ClassDrift:
		return "useful"
	case llm.ClassFalseAlarm:
		return "not-useful"
	case llm.ClassContractedTwinOK:
		return "mixed"
	default:
		return ""
	}
}

// resolveLabel joins one suspect to its calibration label: a registry verdict
// wins, then a manual fire-tag, else untagged. Shared by doctor (reporting) and
// calibrate (training-row construction) so the precedence lives in one place.
func resolveLabel(vclass, id string, tagOf map[string]string) string {
	if l := verdictLabel(vclass); l != "" {
		return l
	}
	if v, ok := tagOf[id]; ok {
		return v
	}
	return "untagged"
}

// runMarkFire tags a fire (by id) with a verdict — the manual calibration signal
// for suspects not (yet) in the registry. Registry-adjudicated suspects are
// labelled automatically by doctor; this is for the rest.
func runMarkFire(args []string) {
	fs := flag.NewFlagSet("mark-fire", flag.ContinueOnError)
	repo := fs.String("repo", ".", "repo whose .calque/ holds the fire log")
	notes := fs.String("notes", "", "optional note")
	if err := fs.Parse(args); err != nil {
		return
	}
	pos := fs.Args()
	if len(pos) < 2 || !validVerdicts[pos[1]] {
		fmt.Fprintln(os.Stderr, "usage: calque mark-fire <id> <verdict> [--notes \"...\"] [--repo .]\n  verdict: useful | mixed | not-useful")
		os.Exit(2)
	}
	path := filepath.Join(calqueDir(*repo), "fire-tags.jsonl")
	if err := appendJSONL(path, fireTag{Ts: nowTs(), ID: pos[0], Verdict: pos[1], Notes: *notes}); err != nil {
		fmt.Fprintf(os.Stderr, "calque mark-fire: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("tagged fire %s as %s\n", pos[0], pos[1])
}

// logFires appends NEW suspects to .calque/fires.jsonl (gitignored telemetry),
// stable-id'd so re-fires dedup in doctor. Best-effort: a logging failure must
// never break the gate (check runs in commit/Stop hooks).
func logFires(repo string, pairs []code.Suspicion, clusters []code.Cluster) {
	path := filepath.Join(calqueDir(repo), "fires.jsonl")
	ts := nowTs()
	for _, s := range pairs {
		_ = appendJSONL(path, fireRecord{Ts: ts, ID: pairID(s), Kind: "pair", Key: pairDisplayKey(s), Score: s.Score})
	}
	for _, c := range clusters {
		_ = appendJSONL(path, fireRecord{Ts: ts, ID: clusterID(c), Kind: "cluster", Key: clusterDisplayKey(c), Score: c.Score})
	}
}

// labeled is a suspect joined to its calibration label + score.
type labeled struct {
	id, kind, key, label string
	score                float64
}

func runDoctor(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	b := addBoundaryFlags(fs)
	repo, left, right, exclude := b.repo, b.left, b.right, b.exclude
	minScore := fs.Float64("min-score", 0.18, "minimum suspicion score to consider")
	minLines := fs.Int("min-lines", 4, "ignore functions shorter than this many lines")
	regPath := fs.String("registry", ".calque/registry.md", "registry file (adjudicated verdicts)")
	clusterMinMembers := fs.Int("cluster-min-members", 3, "smallest N-ary cluster to consider")
	clusterMaxFanout := fs.Int("cluster-max-fanout", 8, "private-symbol fanout ceiling for a seam")
	noCalib := fs.Bool("no-calibrated-weights", false, "ignore .calque/weights.json; score on the static prior")
	ablate := fs.Bool("ablate", false, "Layer D: report the per-detector × language × variety label matrix from the global label store (cross-repo; ignores --repo)")
	if err := fs.Parse(args); err != nil {
		return
	}

	if *ablate {
		runAblate()
		return
	}

	if applyCalibratedWeights(*repo, *noCalib) {
		fmt.Fprintln(os.Stderr, "calque: calibrated weights active (.calque/weights.json)")
	}
	copts := clusterOptsFrom(*minLines, *clusterMinMembers, *clusterMaxFanout, 1<<30)
	r, err := codeAxis(*repo, *left, *right, *exclude, *minScore, *minLines, 1<<30, copts, true, false) // calibrate the gate's real behavior: test↔test excluded
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque doctor: %v\n", err)
		os.Exit(1)
	}
	reg, err := registry.Load(joinRepo(*repo, *regPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "calque doctor: reading registry: %v\n", err)
		os.Exit(1)
	}
	tagOf := loadFireTags(filepath.Join(calqueDir(*repo), "fire-tags.jsonl"))

	// Join every current suspect to a calibration label: registry verdict first,
	// then a manual fire-tag, else untagged (resolveLabel, shared with calibrate).
	var rows []labeled
	for _, s := range r.Pairs {
		id := pairID(s)
		vclass := ""
		if e, ok := reg.Lookup(s.Left.Key(), s.Right.Key()); ok {
			vclass = e.VerdictClass()
		}
		rows = append(rows, labeled{id, "pair", pairDisplayKey(s), resolveLabel(vclass, id, tagOf), s.Score})
	}
	for _, c := range r.Clusters {
		id := clusterID(c)
		vclass := ""
		if e, ok := reg.LookupCluster(c.MemberKeys()); ok {
			vclass = e.VerdictClass()
		}
		rows = append(rows, labeled{id, "cluster", clusterDisplayKey(c), resolveLabel(vclass, id, tagOf), c.Score})
	}

	printDoctor(*repo, r, rows, len(reg.Entries)+len(reg.Clusters))
}

func printDoctor(repo string, r codeAxisResult, rows []labeled, regSize int) {
	fmt.Println("calque doctor — recall calibration")
	fmt.Println(strings.Repeat("─", 44))
	fmt.Printf("scanned %d func(s) in %d file(s); registry: %d adjudicated\n", r.Stats.Funcs, r.Stats.Files, regSize)
	fmt.Printf("current suspects: %d pair(s) · %d cluster(s)\n", len(r.Pairs), len(r.Clusters))
	if len(rows) == 0 {
		fmt.Println("no suspects at this threshold — nothing to calibrate yet.")
		return
	}

	// Verdict tally + mean score per label (the discrimination signal).
	tally := map[string]int{}
	sum := map[string]float64{}
	for _, x := range rows {
		tally[x.label]++
		sum[x.label] += x.score
	}
	fmt.Printf("labels: useful=%d mixed=%d not-useful=%d untagged=%d\n",
		tally["useful"], tally["mixed"], tally["not-useful"], tally["untagged"])

	mean := func(l string) float64 {
		if tally[l] == 0 {
			return 0
		}
		return sum[l] / float64(tally[l])
	}
	if tally["useful"] > 0 || tally["not-useful"] > 0 {
		mu, mn := mean("useful"), mean("not-useful")
		fmt.Printf("mean score: useful=%.2f  not-useful=%.2f\n", mu, mn)
		switch {
		case tally["useful"] == 0 || tally["not-useful"] == 0:
			fmt.Println("  (need both useful and not-useful labels to judge discrimination)")
		case mu > mn:
			fmt.Printf("  ✓ ranker discriminates — drift outscores false-alarms by %.2f\n", mu-mn)
		default:
			fmt.Printf("  ⚠ ranker NOT discriminating — false-alarms score ≥ drift (tune weights)\n")
		}
	}

	// precision@k over all suspects by score (real = useful|mixed; among labelled).
	sort.Slice(rows, func(i, j int) bool { return rows[i].score > rows[j].score })
	for _, k := range []int{5, 10} {
		if k > len(rows) {
			continue
		}
		real, labelled := 0, 0
		for _, x := range rows[:k] {
			if x.label == "untagged" {
				continue
			}
			labelled++
			if x.label == "useful" || x.label == "mixed" {
				real++
			}
		}
		if labelled > 0 {
			fmt.Printf("precision@%d: %d/%d genuine twins among labelled top-%d\n", k, real, labelled, k)
		}
	}

	// Fire log summary (telemetry over time), if present.
	if fires := loadFires(filepath.Join(calqueDir(repo), "fires.jsonl")); len(fires) > 0 {
		byKind := map[string]int{}
		for _, f := range fires {
			byKind[f.Kind]++
		}
		fmt.Printf("fire log: %d distinct fires (%s)\n", len(fires), sortedCounts(byKind))
	}

	fmt.Println("\ntop suspects (score · label · id):")
	for i, x := range rows {
		if i >= 8 {
			break
		}
		fmt.Printf("  %.2f  %-10s %-9s [%s]  %s\n", x.score, x.label, x.kind, x.id, x.key)
	}
	fmt.Println("tag an untagged fire:  calque mark-fire <id> <useful|mixed|not-useful>")
}

// --- jsonl + tag/fire IO ---

func appendJSONL(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

func scanJSONL(path string, fn func([]byte)) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			fn([]byte(line))
		}
	}
}

// loadFireTags returns the latest verdict per fire id.
func loadFireTags(path string) map[string]string {
	out := map[string]string{}
	scanJSONL(path, func(line []byte) {
		var t fireTag
		if json.Unmarshal(line, &t) == nil && t.ID != "" && validVerdicts[t.Verdict] {
			out[t.ID] = t.Verdict
		}
	})
	return out
}

// loadFires returns the latest record per fire id (dedup over re-fires).
func loadFires(path string) map[string]fireRecord {
	out := map[string]fireRecord{}
	scanJSONL(path, func(line []byte) {
		var f fireRecord
		if json.Unmarshal(line, &f) == nil && f.ID != "" {
			out[f.ID] = f
		}
	})
	return out
}

func sortedCounts(m map[string]int) string {
	type kv struct {
		k string
		v int
	}
	var s []kv
	for k, v := range m {
		s = append(s, kv{k, v})
	}
	sort.Slice(s, func(i, j int) bool { return s[i].v > s[j].v })
	var parts []string
	for _, e := range s {
		parts = append(parts, fmt.Sprintf("%s=%d", e.k, e.v))
	}
	return strings.Join(parts, " ")
}
