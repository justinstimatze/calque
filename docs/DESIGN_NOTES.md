# calque — design notes & handoff

> Status: **v0.0.1** (2026-06-05). Python MVP works and is validated against
> `lamina/stope`. This doc captures the full design conversation so a parallel
> session can pick up with complete context.
> Written for: someone continuing calque in a separate worktree/session.
>
> **Session 2 (2026-06-05) progress** — see git log for hashes (history was
> re-authored to `justin@justinstimatze.com`):
> - Roadmap #1 **done**: stope's real registry seeded — 30 suspects adjudicated
>   (`stope/.calque/registry.md`): **4 drift · 21 contracted-twin-ok · 5
>   false-alarm**, each with a `predicted:` score (seeds calibration, #2).
> - Roadmap #4 **done**: installed as a global skill (`~/.claude/skills/calque/`,
>   `/calque`) + the `calque` CLI is on PATH via `pipx install --editable`
>   (its own isolated venv). The tool now runs against any repo from any cwd.
> - stope wired to *act* on the registry: `stope/CLAUDE.md` Pitfall #1 + the
>   session-start reading list now point at `.calque/registry.md`. Drift fixes
>   themselves are left to a stope session.
> - Headline finding: `ScriptedGame` is **mostly an adapter, not a
>   reimplementation** — 21/30 flagged methods delegate to the real engine
>   (`self._engine.step(...)`) or a shared `turning.py`/`scheduler` fn, so they
>   can't behaviorally drift. Only 5 pairs genuinely reimplement; 4 of those
>   drifted — and 2 sit in blind spots of stope's hand-written
>   `test_dual_path_parity.py`. That's the recall-tool-beats-handwritten-tests
>   proof.

---

## 1. What calque is (one paragraph)

calque finds **dual-path code**: two implementations of one contract that are
*supposed* to behave identically but have silently drifted — a test harness that
reimplements production logic, a client that hardcodes a verb list the server
also owns, a `v2` path that diverged from `v1`. These are **Type-4 (behavioral)
clones**: dissimilar in syntax *by construction*, which is exactly why grep, LSP,
embeddings, and clone-detectors all miss them. calque is the **high-recall RECALL
stage** of a hybrid heuristic→LLM-oracle→registry loop. It indexes the signals
that stay invariant when a body is rewritten, ranks suspect pairs, and hands a
short list to an LLM/agent that makes the actual equivalence call. It never
*proves* equivalence (that's undecidable) — it's a nose, not a judge.

The name: a *calque* is a structural copy carried across languages ("skyscraper"
→ French "gratte-ciel"). The software version is two code paths sharing a contract
but diverging in surface.

---

## 2. The core insight (why this is hard, why indices miss it)

The undecidability is real but irrelevant to the user's need. The user's framing
(verbatim, worth preserving):

> "it's technically undecidable but really I just need clues for you to
> automatically go check and have your judgment call on what's too close to
> equivalent... somehow codebase indices don't seem to capture this kind of thing."

That reframe is the whole architecture. We don't build a *prover*; we build a
**recall-heavy clue generator** that routes an LLM oracle's attention. The hard
part (judgment) moves to the part that's good at judgment; the tool only has to be
*suspicious*, never *right*. A 30%-precision / 95%-recall ranker that yields 15
pairs to look at beats any sound analyzer here.

**Why codebase indices structurally can't do it:**
- grep / ctags / LSP index lexical identity + the call graph — but the two twins
  **don't call each other** (that's the definition of the bug), so there's no edge.
- embeddings / "semantic" search embed *how code is written* — and the twins are
  written nothing alike, so they embed *far apart*. The better the embedding gets
  at "looks like," the more reliably it misses this.
- clone detectors (`dupl`, jscpd) look for *similar* code — the opposite problem.

All index **representation**. Dual-path is a **role collision in behavior-space**,
and representation is the dimension along which the twins diverged. So you must
index the **contract invariants**, not the prose.

**The divergence-robust signals calque uses** (in `core.py`):
- emitted string literals — what the function *says* (surface output)
- attribute write-targets (`self.world.ruin`, `ctx.pending_confirm`) — what it
  *mutates* (effect signature)
- returned dict keys — the shape of what it *hands back*
- callee names — what downstream it leans on
- name-stem (role tokens, role-prefixes like `handle_`/`resolve_` stripped) — the
  *role*; names track role even when bodies don't

None care that the bodies look nothing alike. That's the point.

---

## 3. Architecture — it's a "hybrid loop" (per justinstimatze/hybrid)

calque is a concrete instance of the **hybrid-loop** design pattern (the user's
own framework: `github.com/justinstimatze/hybrid` — "LLM judgment and
deterministic code in mutually-generative cycles"). Its catalog already names
calque's shape: a **Knowledge-base auditor** ("substrate is valid source code,
often Go AST; auditors run deterministic checks against the AST; an LLM proposes
edits") run as a **dev-time critique loop**.

Role mapping:

| hybrid role | calque |
|---|---|
| soft input | the codebase / a diff |
| LENS (usually LLM) | **deterministic AST extract** — collapsed into the gate |
| SUBSTRATE | `.calque/registry.md` (typed pair→verdict records) |
| GATE | `core.py` — signature overlap → rank suspects |
| REASONER | the agent — adjudicate drift / twin-ok / false-alarm |
| ACTION | write verdict + collapse-to-single-path |

**Notable variant:** calque is *gate-first with a deterministic lens*. The
canonical 5-role loop opens with an LLM lens because soft input is fuzzy
(transcripts). calque's soft input is **code** — already typed — so extraction
needs no LLM. Recognizable sub-shape: *the auditor whose lens is deterministic
because the substrate is already structured.*

**Two meta-layers calque v0.0.1 is MISSING** (the framework exposed these — they
are the v0.1 roadmap):
- **Calibration** — calque logs verdicts but not *predictions*. Record
  `(predicted score, actual verdict)` per suspect so we can measure whether the
  gate's ranking tracks real drift, and tune signal weights from data instead of
  a guess. The registry already holds the verdict half; add a score column.
- **Metabolism** — no periodic re-audit. A pair cleared `contracted-twin-ok` can
  *later* drift (one side gets edited). Need a substrate-wide sweep that
  re-checks cleared pairs. (`_sync_ruin_cap` in stope is this waiting to happen:
  today both delegate; tomorrow someone inlines one.)

---

## 4. Current implementation (file map)

```
~/Documents/calque/
  calque/
    core.py        # FuncSig extraction (AST) + jaccard scoring + rank(). The IP.
    __main__.py    # `python -m calque scan --repo --left --right [--out]`
    __init__.py
  tests/test_core.py   # 3 tests: stem collapse, planted-twin caught, delegating-pair scores lower
  SKILL.md         # the agent loop (scan → adjudicate → registry); drives the LLM as oracle
  README.md        # public-facing
  registry.template.md   # the `.calque/registry.md` schema
  examples/stope-engine-vs-testing.md   # saved validation run
  pyproject.toml   # stdlib-only; `calque` console script; hatchling
  docs/DESIGN_NOTES.md   # this file
```

**Scoring** (`core.py`): per-signal jaccard, weighted
(`strings .30, writes .30, name .22, calls .10, ret .08`), **renormalized over
available signals** (a pair isn't penalized for neither side emitting strings).
A noise gate requires a real anchor (name overlap ≥0.34 OR any
surface/effect/ret overlap) so generic-callee coincidences are dropped. Tiny
functions (<4 lines) and dunders excluded. Each left fn keeps its single best
match (dedup so one promiscuous fn doesn't dominate).

**CLI contract:** `--left`/`--right` globs are relative to `--repo`. Only
left×right pairs are scored (the testing-vs-prod boundary), not within-group.

---

## 5. Validation evidence (stope)

Run: `python -m calque scan --repo <stope> --left "engine*.py" --right "testing.py"`.
With **zero project knowledge**, it re-surfaced the engine↔testing
reimplementation family that stope's `CLAUDE.md` calls *"the #1 bug source"*:

- `#1 1.00` `GameEngine._sync_ruin_cap` ≟ `ScriptedGame._sync_ruin_cap`
- `#14` `_handle_move` ≟ `move` (caught by shared writes `world.current_loc`/`world.current_zone` + 10 shared calls)
- `#2` `_load_schedules`, `#3` `_find_physical_object`, plus the action-handler row
  (`mislead`/`tear`/`warn`/`betray`/`radio`/`dos`/`call` ≟ their `_handle_*`)

**Adjudication done (1 of 18):** `_sync_ruin_cap` → **`contracted-twin-ok`**. Both
copies are byte-identical and both delegate to `turning.sync_ruin_cap`. Not drift
— a duplicated thin wrapper. (This is the metabolism risk: safe today, could
inline-drift tomorrow.)

**Key property observed:** the pairs we collapsed earlier this session in stope
(#230 `submit_dialogue`, #232 `leave_town`) **dropped off the suspect list**,
because they now delegate and their bodies shrank. The suspect set shrinks as you
single-path — exactly the desired behavior. `test_core.py::test_collapsed_pair_scores_lower`
pins this.

---

## 6. Prior art / research (2026)

- **The problem is named and measured.** LLMs disproportionately produce Type-4
  clones (they re-derive behavior vs reuse it), and existing tools miss them:
  *More Code, Less Reuse* (arXiv 2601.21276); *Detecting Semantic Clones of Unseen
  Functionality* (arXiv 2510.04143, embeddings approach).
- **Verdict leg (per-language, off-the-shelf):** differential / model-based
  testing. Python = **Hypothesis** stateful testing (`RuleBasedStateMachine` +
  reference model — literally "compare optimized impl vs a simplified model").
  Go = **rapid** (`pgregory.net/rapid`, "aims to bring to Go the power Hypothesis
  brings to Python", has state-machine testing + shrinking); gopter is the older
  alt; `go test -fuzz` is coverage fuzzing (different tool). TS = **fast-check**.
  These confirm a flagged pair; they don't discover. (Full calque-vs-Hypothesis
  contrast in §12.)
- **Layered drift detection validated** in a 2026 paper, SysTradeBench (arXiv
  2604.04812): Layer 1 canonicalized-code hash + Layer 2 trace-edit-distance with
  thresholds — independently the same Tier-A/Tier-B shape.
- **Go clone tools won't help:** `dupl` (suffix-tree over ASTs, *ignores values*)
  catches Type 1–2 only — same blind spot.

The general "find any two things that should be identical and aren't" is program
equivalence = undecidable. Everyone converges on **fuzzy discovery + differential
verdict + an oracle**. The 2026 novelty is that the oracle (LLM) is now cheap and
good, which inverts the architecture toward recall-index → LLM-judgment.

---

## 7. Naming decision

Chose **calque** after collision checks. Killed: `twinscan` (ASML lithography +
bioinformatics), `mitosis` (Builder.io framework — same conceptual space!),
`samesame` (multiple existing tools), `lockstep` (fintech SDKs + a language),
`cleave` (PyPI + cleave.js). `cognate`/`homolog` have free namespaces (backups);
cognate sits next to `cognee` (an AI-memory Claude plugin). **calque**'s PyPI/npm/
GitHub namespace is free; metaphor is dead-on (cross-language structural copy).

---

## 8. Relationship to the user's other projects

- **`justinstimatze/hybrid`** (public) — the meta-framework calque instantiates.
  calque should be (a) added to `hybrid/BLOCK_GRAPHS.md` as the "dual-path /
  behavioral-twin auditor" exemplar under Knowledge-base auditor, and (b) ideally
  *authored using* the hybrid skill, which auto-triggers on exactly the prompt
  that started this ("a tool that watches X and flags when a pattern recurs").
- **`justinstimatze/defn`** (`~/Documents/defn`) — Go code-graph (definitions +
  call/ref/interface edges in Dolt). It indexes the *reference graph*, so it's
  structurally blind to dual-path (twins don't call each other) — but it's the
  ideal **Go-side recall backend**: calque's signals become SQL over defn's graph
  instead of a fresh `go/ast` parser. Philosophically the same project
  (externalize code knowledge that dies at context-close), different target.
- **stope** (`~/Documents/lamina/poc/dense/stope`) — the calibration target and
  first real customer; already cited in hybrid's catalog. Remaining dual-path work
  there (tasks #234, plus the ~17 unadjudicated calque suspects) is downstream.
- **`/home/gas6amus/Documents`** — another of the user's accounts (access granted
  via setfacl 2026-06-05). Scanned for overlap: **no prior dual-path / code-
  equivalence tool exists.** `nondual` is a puzzle game (name was a red herring);
  `loopback` is a closed-loop EEG/HRV→adaptive-music system (another hybrid-loop
  instance, different domain); `hybrid` is the framework itself. calque is the
  user's first solution to this problem — not a duplicate.

---

## 9. Roadmap (prioritized)

1. ~~**Seed stope's real registry**~~ — **DONE (session 2)**. All 30 suspects
   adjudicated into `stope/.calque/registry.md`. The 4 `drift` rows are the next
   single-path cleanups (left to a stope session). Note the boundary now spans 14
   `engine*.py` files (mixins), not just `engine.py`.
2. **Calibration layer** — add a `predicted_score` field to registry entries;
   a `calque calibrate` view that reports precision@k from logged verdicts; use it
   to tune `_WEIGHTS`.
3. **Metabolism layer** — `calque audit` re-scans and re-checks `contracted-twin-ok`
   pairs for fresh drift (re-extract both sides; if a side's signature changed
   materially since the recorded verdict, re-flag).
4. ~~**Install as a global Claude skill**~~ — **DONE (session 2)**.
   `~/.claude/skills/calque/SKILL.md` (aligned to hybrid's roles) + `calque` CLI
   on PATH via `pipx install --editable`. `/calque` works from any repo.
5. **Verdict leg** — wire Hypothesis `RuleBasedStateMachine` template for confirmed
   twins (this is stope's #234: ScriptedGame vs GameSession differential).
6. **Go + TS extractors** — TS (ts-morph) for undercity; Go as a query layer over
   defn. The `core.py` ranking is already language-agnostic; only extraction is
   per-language. Factor `extract_py.py` out of `core.py` behind a `FuncSig`
   producer interface.
7. **Boundary presets** — `--preset harness` (engine*×testing), `--preset client-server`,
   etc., so users don't hand-craft globs.

---

## 10. Open questions / decisions for a parallel session

- **Registry location.** Per-repo `<repo>/.calque/registry.md` (chosen) vs a
  central store. Per-repo wins for portability + git-tracked memory; confirm.
- **Cross-language pairs.** stope's real dual paths include JS-client ↔ Python-
  server (#233). calque (single-language AST) can't express those. Options: a
  shared spec/contract file both extractors map onto, or a differential at the
  API boundary. Out of scope for the AST core; note as a known gap.
- **Within-repo whole-scan mode.** Currently boundary-only (left×right). A
  whole-repo O(n²) mode needs name-stem prefiltering to be tractable; worth it for
  "find dual paths I didn't know to look for."
- **Should the LENS ever be an LLM?** For *prose-heavy* substrates (corpus files,
  config) a deterministic lens underperforms. Not relevant to code, but if calque
  ever audits non-code substrate, the lens flips back to LLM.

---

## 11. How to pick up

```bash
cd ~/Documents/calque
python -m pytest tests/ -q            # 5 green
calque scan --repo ~/Documents/lamina/poc/dense/stope \
    --left "engine*.py" --right "testing.py" --out /tmp/calque.md
# add --missing for the coverage-gap (missing-twin) sweep; see §12.
# then open /tmp/calque.md and adjudicate, writing verdicts into
# ~/Documents/lamina/poc/dense/stope/.calque/registry.md (it now exists — grep
# it first; see registry.template.md for the schema)
```

The loop is in `SKILL.md`. The IP is `core.py` (the signals + scoring). Everything
else is scaffolding. Tune the boundary (`--left/--right`), not the threshold.

---

## 12. Session-2 technical additions (2026-06-05)

### Delegation gate (precision fix for the 21/30 adapter problem)
stope's first run was 21/30 *adapters*, not reimplementations: harness methods
that just `return self._engine.step(...)` and repackage the result. They're
**named after** what they wrap, so name-stem matches the real method — a
guaranteed false-positive anchor. `core.py` now detects forwarding to a wrapped
impl (`_DELEGATION_ROOTS` = `_engine`/`_impl`/`_inner`/…) and sets `FuncSig.delegates`,
so a **name match alone can no longer anchor a delegating pair** (it must also
share real surface/effect). Pure delegators drop off; rich adapters that still
share emitted strings/calls (like stope's `radio`/`share`) remain — correctly,
since they own a sliver of glue logic that *can* drift. Pinned by
`test_pure_delegator_named_after_engine_not_flagged`.

### Missing-twins lift (recall fix, from maturity_check overlap)
Asked whether stope's `maturity_check.py` had dual-path logic worth lifting.
Finding: its `_check_dual_path_drift` (lines 474–531) is a **name-substring grep**
— for each engine `_handle_X`, check if `X` appears anywhere in `testing.py` +
`tests/*`. Strictly *weaker* than calque (no word boundaries → `move` matches
`remove`; no AST on the test side; **hardcoded 6-file engine list that silently
ignores 7 of the 13 `engine_*.py` mixins**). **Do not lift the grep.** But it
catches one thing calque's pair-ranker structurally can't: a contract that exists
on the left with **no twin on the right at all** (never written / deleted) →
produces zero pairs → invisible to `rank()`. Lifted as `missing_twins()` +
`--missing`, *generalized*: instead of hardcoding `_handle_`, it **learns which
role prefixes are twinned on this boundary** (those that produced a real match)
and reports only gaps within those roles, so engine-internal helpers don't flood.
On stope it surfaces 93 candidates — a broad triage sweep, not a clean list
(opt-in, off by default). The `_handle_*` subset (`_handle_pray`, `_handle_answer`,
the `_handle_*_ending` family) is the gold maturity_check was built for; the
`_get_*`/`_resolve_*`/`_maybe_*` rows are mostly expected-absent. (Sibling idea
not lifted: maturity_check's `_check_chokepoint_bypass`, lines 537–643 — a
"who may mutate this state" allowlist; a future calque precision signal if it
ever accepts per-repo chokepoint config.)

### calque vs Hypothesis (they're different legs, not competitors)
This came up early; pinning the distinction. **calque is discovery; Hypothesis is
verification.** They sit on opposite ends of the same loop.

| | **calque** | **Hypothesis** (`RuleBasedStateMachine`) |
|---|---|---|
| Question | *Which* pairs might be the same contract? | Does *this* known pair actually behave identically? |
| Method | static AST signal overlap, ranked | runs both, generates inputs, compares impl-vs-model/impl-vs-impl |
| Input | a whole boundary (`left×right`), zero setup | one pair + a hand-written model/state machine |
| Output | a ranked suspect list (recall) | a concrete counterexample, or "no counterexample in N tries" |
| Soundness | unsound both ways (heuristic; over-flags) | a failure is a *real* bug; passing is only probabilistic |
| Cost | ~instant on the repo | per-pair model authoring + runtime |

The key asymmetry: **Hypothesis cannot *discover* a dual path.** It has no notion
that `ScriptedGame.move` and `_handle_move` should agree unless *you already
wrote* the differential test driving both — at which point you already knew. It
won't tell you the harness drifted; it can only prove/disprove a hypothesis you
supplied. calque finds the drift you *didn't know to test*. Conversely calque
never *confirms* equivalence (undecidable; it's a nose, not a judge).

So they **compose** along the hybrid loop: calque = GATE/recall →
adjudicate = REASONER → for a `contracted-twin-ok` pair, **pin it with a
Hypothesis `RuleBasedStateMachine` that drives both impls and asserts equal
outputs** = the ACTION/verdict leg, recorded as `policy: differential-test` in
the registry. That differential test is exactly roadmap #5 (stope #234:
ScriptedGame vs GameSession). Analogy: calque is the smoke detector (cheap,
whole-house, points you to a room); Hypothesis is the lab assay (one sample,
near-definitive). You want both — and stope's existing `test_dual_path_parity.py`
is the hand-rolled, non-generative ancestor of that Hypothesis leg.
