# calque registry — calque on calque

Durable record of adjudicated dual-path pairs **for calque's own source**. calque
must eat its own dogfood (cf. the adit irony: a code-health tool that contained the
patterns it detects). Grep before assuming two paths are independent; update when a
pair is fixed/cleared/found.

Run: `calque scan --repo . --min-score 0.12`   (self-scan; all source × all source)

Verdicts: **drift** (same contract, behavior diverges → collapse) · **contracted-twin-ok**
(intentionally parallel, in sync → pin) · **false-alarm** (coincidental overlap → suppress).

---

## Run — 2026-06-06 (Go rewrite; first self-scan of the Go source)

52 funcs / 10 files. The Go port self-scanned during implementation; two prior
Python-era self-bugs are now resolved, one new dup was caught-and-fixed live, and the
old taxonomy bug reappeared in the port.

**Resolved from the Python era:**
- *Symmetric output not deduped* — FIXED: `Rank` now dedups on the unordered pair
  `{left,right}` (`unorderedKey`), so a self-scan no longer prints A≟B and B≟A.

**Caught live during implementation (the meta loop closing in real time):**

## relTo ≟ corpus.RelPath — DRIFT, FIXED same session
- was: internal/code/extract_go.go::relTo  ≟  internal/corpus/corpus.go::RelPath  (0.66)
- verdict: drift (I duplicated the filepath.Rel wrapper while writing the Go extractor).
- policy: collapse-to-single-path — DONE: deleted `relTo`, extract_go.go now calls
  `corpus.RelPath`. Re-scan confirms the pair is gone. Caught by calque's own scan.

## Suspicion.Reason ≟ scorePair — DRIFT (open; calque's own taxonomy bug, again)
- left:  internal/code/score.go::Suspicion.Reason
- right: internal/code/score.go::scorePair
- signal: shared-strings=4; shared-calls=1   (score 0.21)
- verdict: **drift** — the signal taxonomy `{strings,writes,name,calls,ret}` is listed in
  FOUR places: the `weights` map, the `sig` map + the `avail` map in `scorePair`, and the
  `switch` in `Reason`. The Python version had this (P2/P3); the port reproduced it.
- policy: collapse — make signals **table-driven** (one `[]signalDef{key,weight,sim,
  avail,render}`), so a new signal is one entry. [TODO — next]

## runSynonymReport ≟ runVocabReport — CONTRACTED-TWIN-OK (latent; watch)
- signal: shared-writes=[Count Locations]; shared-calls=21; name~0.33(report); shared-strings=5
- verdict: contracted-twin-ok — both prose commands share the walk→tally loop. Already
  single-source the walk+strip via internal/corpus; the per-token tally could move to a
  shared helper if a third prose command appears. Pin, don't collapse yet.

## toSet ≟ setEqual — FALSE-ALARM
- both are tiny set helpers (name~set, one shared call). Coincidental. Suppress.

## texts ≟ TextsBatched — FALSE-ALARM (adapter)
- `TextsBatched` is the batching wrapper around `texts`; named after what it wraps. Suppress.

Tally: **2 drift (1 fixed, 1 open) · 1 contracted-twin-ok · 2 false-alarm.**
