# calque registry — calque on calque

Durable record of adjudicated dual-path pairs **for calque's own source**. calque
should eat its own dogfood (cf. the adit irony: a code-health tool that contained the
patterns it detects — calque must not). Grep this before assuming two paths are
independent; update it when a pair is fixed/cleared/found.

Run: `calque scan --repo . --left "calque/*.py" --right "calque/*.py" --min-score 0.12`

Verdicts: **drift** (same contract, behavior diverges → collapse) · **contracted-twin-ok**
(intentionally parallel, in sync → pin) · **false-alarm** (coincidental overlap → suppress).

---

## Run — 2026-06-06 (first self-scan; calque was NOT previously dogfooded on itself)

656 LOC, 3 files. 3 unique suspect pairs (calque reported each twice — see known-bug
below). Tally: **1 drift · 1 contracted-twin-ok · 1 false-alarm.**

---

## score_pair ≟ Suspicion.reason — DRIFT (real; calque's own P2/P3 bug)
- left:  calque/core.py::score_pair (268)
- right: calque/core.py::Suspicion.reason (243)
- signal: shared-strings=4; shared-calls=1   (predicted 0.23)
- verdict: **drift** — the signal taxonomy `{strings, writes, name, calls, ret}` is
  hand-written in FOUR places: `_WEIGHTS` (line 233, the canon), the `sig` dict and the
  `avail` dict inside `score_pair` (275–290), and the `elif` branches in
  `Suspicion.reason` (250–264). Add or rename a signal → must edit all four by hand.
  This is exactly PATTERN_CATALOG P2 (taxonomy in N literal sets) / P3 (constant set
  redefined), in calque's own scorer. calque's nose caught it via the shared signal-key
  string literals.
- policy: collapse-to-single-path — drive `sig`/`avail`/`reason` off `_WEIGHTS` keys
  (one taxonomy: the keys of `_WEIGHTS`), so a new signal is one edit. [TODO]
- reviewed: 2026-06-06

## _report ≟ _missing_report — CONTRACTED-TWIN-OK (latent twin; pin)
- left:  calque/__main__.py::_report (32)
- right: calque/__main__.py::_missing_report (55)
- signal: name~0.50(report); shared-calls=2   (predicted 0.23)
- verdict: contracted-twin-ok — two markdown-report builders with deliberately
  different bodies (suspects vs missing-twins). They share header/format scaffolding;
  if the report format changes, both must move together (latent drift). Acceptable
  today; pin with a shared header helper if a third report appears.
- policy: none (watch)
- reviewed: 2026-06-06

## extract_file ≟ extract_command_terms — FALSE-ALARM
- left:  calque/core.py::extract_file (184)
- right: calque/core.py::extract_command_terms (435)
- signal: shared-strings=2; name~0.25(extract); shared-calls=5   (predicted 0.29)
- verdict: false-alarm — both are AST walkers (hence shared ast.* calls) and both named
  `extract_*`, but they do unrelated jobs (file→FuncSigs vs a command-corpus term set).
  Coincidental signal overlap; suppress.
- policy: none
- reviewed: 2026-06-06

---

# Known calque self-bug (found by running calque on calque)

**Symmetric output not deduped.** The scan printed all 3 pairs TWICE — once as `A ≟ B`
and once as `B ≟ A` (6 rows for 3 pairs). The ranker dedups, but not by unordered pair,
so direction-flipped duplicates survive into the report. Minor, but ironic for a tool
about duplication. Fix: key dedup on `frozenset({a.key, b.key})`. [TODO]
