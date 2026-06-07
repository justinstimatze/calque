# calque registry — calque on calque

Durable, git-tracked memory of adjudicated dual-path pairs **for calque's own
source**. calque must eat its own dogfood (cf. the adit irony: a code-health tool
that contained the patterns it detects).

Run:  `calque scan --repo . --exclude 'legacy/**,**/*_test.go'`   (pairs + N-ary clusters)
Gate: `calque check --repo . --exclude 'legacy/**,**/*_test.go'`  (only NEW vs this registry)

(Exclude `legacy/**` — the Python port reference — and `**/*_test.go`, whose
synthetic fixtures plant seams on purpose. The registry tracks both `- pair:`
and `- cluster:` (N-ary, member-set keyed) verdicts.)

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

---

## Run — 2026-06-06 (N-ary touchpoint clustering added; self-dogfood of the new pass)

Added the private-symbol touchpoint signal + N-ary clustering (DESIGN_NOTES §15,
the #269 triple-shell gap). Running the new feature on calque itself surfaced one
real N-ary drift, the intentional cluster-mirrors-pair API parallels, and the
usual key/set name-token coincidences. Adjudicated below; `check` is clean again.

## Signal taxonomy {name,strings,calls,writes,ret} in 4 sites — DRIFT (N-ary)

The new touchpoint pass caught the **N-ary extent** of the already-registered
`Suspicion.Reason ≟ scorePair` drift: the signal taxonomy is a contract shared by
both extractors (which emit those JSON field names) AND both scorers (which read
them as map keys). The pairwise entry saw 2 of the 4 sites; the cluster sees all
4. Same root, same fix (make signals table-driven — `[]signalDef{...}`); pinned so
`check` doesn't re-flag. This is the feature finding the bug it was built to find.
- cluster: internal/code/extract.py::_extract | internal/code/extract_go.go::goBody.Visit | internal/code/score.go::Suspicion.Reason | internal/code/score.go::scorePair
- verdict: drift
- reviewed: 2026-06-06

## Cluster API mirrors pair API — contracted-twin-ok (the N-ary registry, by design)

The cluster path deliberately parallels the pair path (HasCluster≟Has,
LookupCluster≟Lookup, Cluster.Reason≟Suspicion.Reason, SetKey≟Key). These MUST
stay in lockstep — calque correctly flags its own intentional parallel.
- pair: internal/registry/registry.go::Registry.HasCluster | internal/registry/registry.go::Registry.Has
- verdict: contracted-twin-ok
- reviewed: 2026-06-06
- pair: internal/registry/registry.go::Registry.LookupCluster | internal/registry/registry.go::Registry.Lookup
- verdict: contracted-twin-ok
- reviewed: 2026-06-06
- pair: internal/code/touchpoint.go::Cluster.Reason | internal/code/score.go::Suspicion.Reason
- verdict: contracted-twin-ok
- reviewed: 2026-06-06
- pair: internal/pairkey/pairkey.go::SetKey | internal/pairkey/pairkey.go::Key
- verdict: contracted-twin-ok
- reviewed: 2026-06-06

## More key/set name-token collisions — false-alarm (a module about keys and sets)

`keySet` (set of member keys, for subset checks) collides on the "key"/"set"
tokens with `SetKey`/`Key`/`toSet`. Coincidental shared name tokens, distinct
purposes — same class as the *Key collisions above.
- pair: internal/code/touchpoint.go::keySet | internal/pairkey/pairkey.go::SetKey
- verdict: false-alarm
- reviewed: 2026-06-06
- pair: internal/code/funcsig.go::toSet | internal/code/touchpoint.go::keySet
- verdict: false-alarm
- reviewed: 2026-06-06
- pair: internal/pairkey/pairkey.go::SetKey | internal/code/funcsig.go::toSet
- verdict: false-alarm
- reviewed: 2026-06-06
- pair: internal/pairkey/pairkey.go::Key | internal/code/touchpoint.go::keySet
- verdict: false-alarm
- reviewed: 2026-06-06

## runScan ≟ runCheck ≟ runHook ≟ runDoctor — contracted-twin-ok (shared spine)

The N-ary form of the registered runCheck≟runScan pair: all four code-axis command
handlers go through the single-sourced spine — `addBoundaryFlags` (flag taxonomy),
`codeAxis` (the extract→rank→cluster pipeline), `clusterOptsFrom`, `splitCSV`. This
is the dedup *working*: the touchpoint pass first flagged the verbatim flag block
(score 2.83 → factored `addBoundaryFlags`), then the duplicated pipeline (→ factored
`codeAxis`); what remains is intended shared infrastructure, not drift. Pin.
- cluster: cmd/calque/calib.go::runDoctor | cmd/calque/check.go::runCheck | cmd/calque/hook.go::runHook | cmd/calque/scan.go::runScan
- verdict: contracted-twin-ok
- reviewed: 2026-06-06

## calib.go commands (logFires/printDoctor/runDoctor/runMarkFire) — contracted-twin-ok

The calibration functions all resolve `.calque/` via `calqueDir`, append via
`appendJSONL`, and key suspects via `pairDisplayKey`/`clusterDisplayKey`. Intended
shared infra within the calibration leg.
- cluster: cmd/calque/calib.go::logFires | cmd/calque/calib.go::printDoctor | cmd/calque/calib.go::runDoctor | cmd/calque/calib.go::runMarkFire
- verdict: contracted-twin-ok
- reviewed: 2026-06-06

## logFires ≟ runDoctor ≟ runCheck — contracted-twin-ok (suspect-id helpers)

All three id suspects via the single-sourced `pairID`/`clusterID` so the fire log,
the gate output, and doctor agree on a suspect's identity. Intended.
- cluster: cmd/calque/calib.go::logFires | cmd/calque/calib.go::runDoctor | cmd/calque/check.go::runCheck
- verdict: contracted-twin-ok
- reviewed: 2026-06-06

---

## Run — 2026-06-06 (prose gate `vocab-check` added)

The prose-axis gate (cupel vocab-audit equivalent, generalized). The compound
walk→tally is now single-sourced as `tallyCompounds` — the shared helper the
registered runSynonymReport≟runVocabReport note predicted "once a third prose
command appears." The remaining overlaps are the intended prose-axis family and
the prose-gate-mirrors-code-gate parallel.

## Prose command family (report/check/synonym share the tally+walk) — contracted-twin-ok
- cluster: cmd/calque/synonym_report.go::runSynonymReport | cmd/calque/vocab_check.go::runVocabCheck | cmd/calque/vocab_report.go::runVocabReport
- verdict: contracted-twin-ok
- reviewed: 2026-06-06
- pair: cmd/calque/synonym_report.go::runSynonymReport | cmd/calque/vocab_report.go::tallyCompounds
- verdict: contracted-twin-ok
- reviewed: 2026-06-06
- pair: cmd/calque/vocab_check.go::runVocabCheck | cmd/calque/vocab_report.go::runVocabReport
- verdict: contracted-twin-ok
- reviewed: 2026-06-06

## runVocabCheck ≟ runCheck — contracted-twin-ok (prose gate mirrors code gate)

Both are warn-only→`--strict` gates; the prose gate (compounds vs allow-list)
deliberately parallels the code gate (pairs/clusters vs registry). Same spine
shape, different substrate — the §16 unification working, not drift.
- pair: cmd/calque/vocab_check.go::runVocabCheck | cmd/calque/check.go::runCheck
- verdict: contracted-twin-ok
- reviewed: 2026-06-06

## Borderline name-stem collisions among the CLI handlers — false-alarm

`run*`/`build*` handlers share role stems at the 0.18–0.21 noise floor (the gate vs.
the hook's command string; doctor's run/print split). Coincidental name tokens.
- pair: cmd/calque/check.go::runCheck | cmd/calque/hook.go::buildCheckCmd
- verdict: false-alarm
- reviewed: 2026-06-06
- pair: cmd/calque/calib.go::runDoctor | cmd/calque/calib.go::printDoctor
- verdict: false-alarm
- reviewed: 2026-06-06
- pair: cmd/calque/calib.go::runDoctor | cmd/calque/check.go::runCheck
- verdict: contracted-twin-ok
- reviewed: 2026-06-06
