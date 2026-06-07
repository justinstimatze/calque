# calque registry — calque on calque

Durable, git-tracked memory of adjudicated dual-path pairs **for calque's own
source**. calque must eat its own dogfood (cf. the adit irony: a code-health tool
that contained the patterns it detects).

Run:  `calque scan --repo . --min-score 0.12`   (exploratory recall, all × all)
Gate: `calque check --repo .`                    (only NEW vs this registry)

Each adjudicated pair carries machine lines (`- pair:` / `- verdict:` /
`- reviewed:`) that `check` parses; the surrounding prose is for humans.
Verdicts: **drift** (collapse) · **contracted-twin-ok** (pin, watch) ·
**false-alarm** (suppress). Recency is `reviewed` + liveness reconciliation,
never age-eviction (an old false-alarm is still a false-alarm).

---

## Run — 2026-06-06 (Go rewrite; self-scan of the Go source)

52 funcs / 10 files. Two Python-era self-bugs resolved (symmetric output → fixed
via unordered-pair dedup; the relTo ≟ corpus.RelPath dup caught live during the
port → single-sourced, gone). Four standing pairs adjudicated below.

## Suspicion.Reason ≟ scorePair — DRIFT (open: calque's own taxonomy bug, again)
- pair: internal/code/score.go::Suspicion.Reason | internal/code/score.go::scorePair
- verdict: drift
- reviewed: 2026-06-06

The signal taxonomy `{strings,writes,name,calls,ret}` is listed in four places:
the `weights` map, the `sig` + `avail` maps in `scorePair`, and the `switch` in
`Reason`. The Python version had this (PATTERN_CATALOG P2/P3); the port reproduced
it. Registered as known so `check` doesn't re-flag it. **Fix:** make signals
table-driven (`[]signalDef{key,weight,sim,avail,render}`) so a new signal is one
entry. [TODO]

## runSynonymReport ≟ runVocabReport — CONTRACTED-TWIN-OK (latent; watch)
- pair: cmd/calque/synonym_report.go::runSynonymReport | cmd/calque/vocab_report.go::runVocabReport
- verdict: contracted-twin-ok
- reviewed: 2026-06-06

Both prose commands share the walk→tally loop. They already single-source the
walk+strip via `internal/corpus`; move the per-token tally to a shared helper if a
third prose command appears. Pin, don't collapse yet.

## toSet ≟ setEqual — FALSE-ALARM
- pair: internal/code/funcsig.go::toSet | internal/code/score.go::setEqual
- verdict: false-alarm
- reviewed: 2026-06-06

Two tiny set helpers (name~set, one shared call). Coincidental overlap.

## texts ≟ TextsBatched — FALSE-ALARM (adapter)
- pair: internal/embed/embed.go::texts | internal/embed/embed.go::TextsBatched
- verdict: false-alarm
- reviewed: 2026-06-06

`TextsBatched` is the batching wrapper around `texts` — named after what it wraps.

---

## Cross-language extractor twins (intentional; the multi-language design)

The Go (`extract_go.go`, go/ast) and Python (`extract.py`) extractors deliberately
mirror the same extraction in each language — calque correctly flags them as twins.
These are **contracted-twin-ok**: they MUST stay in sync (if one drifts in what it
extracts, the code/Python axes disagree), so it's *good* that calque tracks them.
This is calque watching its own intentional parallel — the feature working on itself.

## attrPath ≟ _attr_path — contracted-twin-ok (dotted attr suffix, per language)
- pair: internal/code/extract_go.go::attrPath | internal/code/extract.py::_attr_path
- verdict: contracted-twin-ok
- reviewed: 2026-06-06

## goBody.recordTarget ≟ _BodyVisitor._record_target — contracted-twin-ok
- pair: internal/code/extract_go.go::goBody.recordTarget | internal/code/extract.py::_BodyVisitor._record_target
- verdict: contracted-twin-ok
- reviewed: 2026-06-06

## goBody.Visit ≟ _BodyVisitor.visit_Call — contracted-twin-ok
- pair: internal/code/extract_go.go::goBody.Visit | internal/code/extract.py::_BodyVisitor.visit_Call
- verdict: contracted-twin-ok
- reviewed: 2026-06-06

## Extract ≟ _extract — contracted-twin-ok (walk+extract entry, per language)
- pair: internal/code/scan.go::Extract | internal/code/extract.py::_extract
- verdict: contracted-twin-ok
- reviewed: 2026-06-06

## extractGoBatch ≟ ExtractGoFile — false-alarm (batch wraps the per-file extractor)
- pair: internal/code/extract_go.go::extractGoBatch | internal/code/extract_go.go::ExtractGoFile
- verdict: false-alarm
- reviewed: 2026-06-06

## main ≟ main — false-alarm (both entry points named main)
- pair: cmd/calque/main.go::main | internal/code/extract.py::main
- verdict: false-alarm
- reviewed: 2026-06-06

## runCheck ≟ runScan — contracted-twin-ok (shared scan pipeline; refactor candidate)
- pair: cmd/calque/check.go::runCheck | cmd/calque/scan.go::runScan
- verdict: contracted-twin-ok
- reviewed: 2026-06-06

Both run Extract→Filter→Rank then diverge (report vs gate). Shared setup could move
to a helper if it grows; pinned for now.

## More cross-language visitor twins — contracted-twin-ok
- pair: internal/code/extract.py::_BodyVisitor.visit_Constant | internal/code/extract_go.go::goBody.Visit
- verdict: contracted-twin-ok
- pair: internal/code/extract.py::_BodyVisitor.visit_Assign | internal/code/extract_go.go::goBody.Visit
- verdict: contracted-twin-ok
- pair: internal/code/extract.py::_BodyVisitor.visit_Return | internal/code/extract_go.go::goBody.Visit
- verdict: contracted-twin-ok

## "*Key" name collisions — false-alarm (single shared name token "key")
- pair: internal/pairkey/pairkey.go::Key | cmd/calque/synonym_report.go::pairKey
- verdict: false-alarm
- pair: internal/pairkey/pairkey.go::Key | internal/code/extract_go.go::litKey
- verdict: false-alarm
- pair: internal/pairkey/pairkey.go::Key | cmd/calque/vocab_report.go::stemKey
- verdict: false-alarm

## scorePair ≟ _extract — false-alarm (coincidental shared signal-key strings)
- pair: internal/code/score.go::scorePair | internal/code/extract.py::_extract
- verdict: false-alarm
- reviewed: 2026-06-06
