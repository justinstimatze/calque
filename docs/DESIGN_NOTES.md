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
embeddings, and clone-detectors all miss them. calque is the **high-recall GATE**
(the *recall* stage — "RECALL" is a description, not a sixth hybrid role) of a
hybrid heuristic→LLM-oracle→registry loop. It indexes the signals
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
| LENS (usually LLM) | **deterministic AST extract** — a *code-block lens* (deterministic because the substrate is already structured code) |
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

### 2026 survey (session 2) — where calque actually sits

A fresh fan-out survey (full source list logged with the session). Bottom line:
**calque is novel in framing + signal, not architecture.**

- **Nearest neighbor — HyClone** (arXiv 2508.01357, Aug 2025): two-stage Python
  Type-4 detector, *LLM screens → execution validates*. Same two-leg shape as
  calque but **inverted** — HyClone spends the LLM on the cheap screen; calque
  uses cheap *static effect signals* for the screen and reserves LLM + property
  testing for the verdict. And HyClone is built for *benchmark pair-classification*,
  not *drift-hunting in a live unlabeled repo*. Cite as closest prior art + the
  contrast that defines calque.
- **Verdict-leg backbone — differential fuzzing** (arXiv 2602.15761, Feb 2026):
  test-free equivalence by input-generation + cross-execution found **19–35% of
  LLM refactorings non-equivalent, ~21% slipping past existing tests.** This is
  calque's intended verdict leg, empirically validated, with a great motivating
  stat ("existing tests miss real drift" = exactly the stope `test_dual_path_parity`
  blind spots calque found).
- **LLM is not a sound oracle** — independently confirmed: empirical LLM-clone
  study (2511.01176), CETBench (2506.04019), the LLM-as-Judge-for-SE survey
  (2510.24367). Equivalence-judging isn't even a named LLM-judge task category yet.
  This *validates* demoting the LLM to triage (REASONER) and promoting differential
  testing to the verdict (ACTION).
- **Recall-validation datasets to adopt:** GPTCloneBench (Type-4 incl. Python),
  CETBench transform-pairs. Measure: of known Type-4 pairs, what fraction does
  calque's effect-signal jaccard rank as suspects? Tune for recall.
- **The signal is the genuinely original move.** Static *effect/mutation*
  signatures (write-targets, emitted literals, returned keys) as a recall signal
  sit in an empty intersection: effect systems exist (for typing/optimization),
  behavioral-clone detection exists (but goes dynamic) — nobody uses cheap static
  effect footprints to *match twins*. (NSF 10113743 is the formal "no purely static
  method detects all behavioral clones" — grounds "a nose, not a judge.")
- **Commercial near-miss:** SMART TS XL / "Mirror Code" sells the *divergence*
  narrative across systems but finds duplicates without judging drift and has no
  open recall→oracle pipeline. API-drift tools (oasdiff etc.) need a *declared*
  contract; calque's niche is the *undeclared, implicit* contract.

The general "find any two things that should be identical and aren't" is program
equivalence = undecidable. Everyone converges on **fuzzy discovery + differential
verdict + an oracle**. The 2026 novelty is that the oracle (LLM) is now cheap and
good, which inverts the architecture toward recall-index → LLM-judgment. calque's
unclaimed white space: **undeclared-contract drift-hunting in a live repo, via
static effect-signatures.**

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

## 9. Roadmap — path to 2026 SOTA

> **Current direction (end of session 2): dogfood on stope first.** Decision is to
> keep calque pointed at stope for a while before generalizing — real usage there
> tells us which P0 profile work actually matters, instead of building profiles
> speculatively. **Treat P0 below as planned, not started; do not begin the
> profile refactor without that signal.** Near-term value = running scans on stope
> and adjudicating into `stope/.calque/registry.md`.

Reframed in session 2 around the survey (§6) and the central problem: **calque
works on stope; making it work on *any* large project is the real challenge**
(§13). The architecture is SOTA-shaped (recall→adjudicate→verdict, the consensus
2026 shape); to *be* SOTA, calque must (a) generalize its signal beyond stope's
conventions, (b) close the loop with a real verdict leg, and (c) prove recall on
a public benchmark. Prioritized to that end:

**Done (session 2)**
- ~~Seed stope's registry~~ — 30 suspects adjudicated; loop closed end-to-end
  (calque found `_check_artifact_flashback`, a real prod bug; stope fixed it).
- ~~Global skill + CLI~~ — `~/.claude/skills/calque/` + `pipx` editable `calque`.
- ~~Delegation down-weight~~ — adapters that forward to `self._engine` no longer
  name-anchor (killed the 21/30 false positives). [commit bc91dca]
- ~~Missing-twins + reachability gate~~ — coverage-gap sweep, dispatcher-aware
  gate suppresses verbs driven via `step("x")`. Opt-in; still coarse (§13).
- ~~Configurability~~ — `--role-prefixes`, `--delegation-roots`, `--dispatchers`
  let other repos override the stope-shaped defaults.

**P0 — Generalization (the SOTA-blocker; see §13)**
1. **Pluggable signal profiles.** Factor extraction behind a `FuncSig`-producer
   interface; ship profiles beyond stope's *effectful-OOP* one: *functional*
   (return-shape, arg/param names, raised exceptions), *API/contract* (routes,
   params, status codes), *data-pipeline* (input/output schema). The ranker
   (jaccard + dedup + gate) is already domain-agnostic — only the signal set is
   stope-shaped. Per the survey: there is **no universal behavioral signal**, so
   profile-per-domain is the correct architecture, not a hack.
2. **Recall benchmark.** Validate on GPTCloneBench + CETBench (§6): measure what
   fraction of known Type-4 pairs calque ranks as suspects. This is how SOTA is
   *claimed*, not asserted. Guards against the "generalization cliff" (per-repo
   tuning that doesn't transfer).
3. **Auto-learn the conventions** (stretch): infer role-prefixes, delegation
   roots, and dispatcher names from a repo's own naming distribution instead of
   flags. Risky (can strip meaningful tokens) — gate behind measurement from #2.
3a. **Rare private-symbol touchpoint signal + N-ary recall (from stope #269; see §15).**
   Highest-leverage code-axis upgrade the evidence points to. Inverted index of private
   symbols (leading-underscore call/string/write) → functions touching them; a symbol
   touched by 2..K functions is a shared internal seam (weight by rarity, TF-IDF). Emit
   **clusters** `{members, shared-symbols}`, not just pairs, and key the registry on a
   set. This catches sub-function inlined-block duplication across differently-named
   functions (the #269 triple shell: `step`/`step`/`run` all touch `_parse_action` +
   `_agent_canon`) that whole-function pairwise Jaccard drowns. Pairs with #6 (audit)
   and the antibody generator below. **Planned, not started.**
3b. **Antibody generator (registry → executable guard).** When a cluster is adjudicated,
   emit stope's hand-written shape (`inspect.getsource`: assert every member routes
   through the shared primitive, none call the forbidden bare primitive — cf.
   `test_session_shell_unification.py`). Makes the verdict leg (#4) concrete for the
   collapse-to-single-path case. **Planned, not started.**

**P1 — Second axis: env/config parity (from stope usage; see §14)**
3.5. **Env-parity sibling check.** A second lens for the *same* meta-bug ("one value
   defined in N places that drift") in config rather than code: diff each launcher's
   effective boot env (`make dev`, `roleplay-server`, conftest, …) against a canonical
   prod profile (`fly.toml [env]`), minus a declared dev-override whitelist; flag any
   parity-critical flag that differs or is unset. Same recall→adjudicate→registry
   loop, registry keyed on `(flag, launchers, prod value)`. Different lens (shell/
   Make/TOML/conftest parsing, not Python AST), so a **sibling check, not a profile**.
   **Planned, not started** — stope's boot-time parity assertion covers its instance
   today; build the static cross-launcher check when that proves insufficient.

**P1 — Close the loop (recall is half a tool)**
4. **Verdict leg.** Wire a Hypothesis `RuleBasedStateMachine` template that drives
   both sides of a `contracted-twin-ok` pair and asserts equal outputs (stope #234:
   ScriptedGame vs GameSession). The survey's strongest result (2602.15761) is that
   *differential fuzzing* is the right oracle — generalize the template to
   rapid (Go) / fast-check (TS).
5. **Calibration layer.** `calque calibrate` reports precision@k from the
   registry's `predicted:` scores vs recorded verdicts; tune `_WEIGHTS` from data,
   per-profile. (Registry already carries the `predicted:` half.)
6. **Metabolism layer.** `calque audit` re-checks `contracted-twin-ok` pairs for
   fresh drift (re-extract; re-flag if a side's signature changed since the
   verdict). `_sync_ruin_cap` is the canonical risk (both delegate today; an
   inline tomorrow re-drifts).

**P2 — Reach & ergonomics**
7. **Go + TS extractors.** TS via ts-morph; Go as a query layer over `defn`.
   Falls out of #1 (profiles) once the producer interface exists.
8. **Boundary presets.** `--preset harness` (engine*×testing), `--preset
   client-server`, `--preset v1-v2` so users don't hand-craft globs.
9. **min-score calibration.** Feedback flagged the engine×testing default could be
   ~0.25 (post-delegation-downweight, a 0.19–0.21 noise tail appears). Make it
   boundary-relative rather than a hardcoded constant — but only after #2/#5 give
   data; don't bake a tuned threshold (generalization-cliff risk).
10. **Add calque to hybrid's catalog** — under "Knowledge-base auditor" in
    `hybrid/skills/hybrid-loops/references/BLOCK_GRAPHS.md`, beside `defn`.
    **Deferred until calque is stabilized + public** (per Justin). Entry drafted.

**TODO — Cross-language dogfood targets (next, per Justin 2026-06-06)**
11. **Run the LLM-as-nose fuzzy hunt on `undercity` (TypeScript).** Dusty (475
    TS/TSX + 209 JS/JSX files, last commit 2026-03-21), so likely carries the same
    meta-bug stope did, but in TS. Apply the §15 live-hunt method: enumerate shells/
    entry points, find taxonomies defined in N literal places, parallel parsers,
    serialization twins, magic constants, the env/config tail. Produce a TS-flavored
    language-design feedback doc (TS levers: discriminated unions + exhaustive `switch`
    with `never`, **branded/opaque types** for raw-vs-canonical, `as const` + derived
    union for single-source taxonomies, `satisfies`, eslint custom rules / dependency-
    cruiser as the import-linter-equivalent antibody). Validates the cross-language
    claim and feeds the TS extractor (item 7, ts-morph).
12. **Poke at a Go repo for the same pattern** — `gemot` is big enough (163 `.go`
    files; `adit` 44, `defn` 33 as backups). Hypothesis (Justin): Go is *slightly* more
    robust to this (no inheritance → less template-method drift; implicit/structural
    interface satisfaction). But check its own flavors: pre-generics copy-paste,
    duplicated `error` handling, struct-tag drift, two impls of one interface, `const`
    blocks redefined. Goal is *generality* — confirm whether the meta-bug + the
    private-symbol-touchpoint signal transfer to Go, to make calque broadly capable.

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
python -m pytest tests/ -q            # 6 green
calque scan --repo ~/Documents/lamina/poc/dense/stope \
    --left "engine*.py" --right "testing.py" --out /tmp/calque.md
# coverage-gap sweep with the reachability gate (see §12):
#   calque scan --repo <stope> --left "engine*.py" --right "testing.py" \
#       --missing --missing-corpus "testing.py" "tests/*.py"
# other-convention repos: --role-prefixes / --delegation-roots / --dispatchers
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

**Reachability gate (`--missing-corpus`).** "No dedicated twin method" massively
over-flags any harness built around a single command dispatcher: a verb driven by
`game.step("pray")` is fully tested with no `def pray()`. So `missing_twins` takes
`reachable_terms` — the verb vocabulary of *dispatcher-call* string args in a
usage/test corpus (`extract_command_terms`, scoped to `_DEFAULT_DISPATCHERS` =
step/do/run/…, configurable via `--dispatchers`). A candidate whose role-stripped
stem is covered by those terms is suppressed. This is the disciplined form of
maturity_check's grep: **word-boundary, string-literals-in-dispatcher-calls only,
used to suppress not flag.** It's deliberately dispatcher-scoped so a verb word in
an *assertion* or a *fact-name* string (`'count_unstable'`) doesn't create false
reachability — that scoping is what keeps the genuinely-untested `pray`/`sing`/
`count` flagged while suppressing `eat`/`pet`/etc.

**Honest precision caveat.** Even gated, stope still shows ~82 (down from 116):
the reachability gate fixes the *dispatcher-reachable* false positives, but the
bulk that remain are `_get_*`/`_check_*`/`_resolve_*`/`_maybe_*` **engine
internals** whose role-prefix was "twinned" by a couple of coincidental matches.
No structural signal knows those aren't *meant* to have a harness twin — that's
domain knowledge. So `missing_twins` is a **coarse, opt-in recall aid, not a clean
report**; the actionable yield on stope is the ~6 player-verb gaps (`pray`, `sing`,
`count`, `follow`, `inventory`, `wait`). This is a concrete instance of the
generalization challenge in §13. (Sibling idea not lifted: maturity_check's
`_check_chokepoint_bypass`, lines 537–643 — a "who may mutate this state"
allowlist; a future calque precision signal if it ever accepts per-repo config.)

### Configurability (first generalization step)
The stope-shaped constants are now CLI-overridable (extend the defaults):
`--role-prefixes`, `--delegation-roots`, `--dispatchers`. This removes the
*hardcoding* hazard the survey warned about (naming-convention-dependence) but
not the *signal*-shape assumption (§13) — that needs profiles, not flags.

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

---

## 13. The generalization challenge (honest assessment, session 2)

Observed bluntly in session 2: **a lot of calque is specific to stope and its
effectful-OOP interactive-fiction engine conventions.** Generalizing to *any*
large project is the central open problem (roadmap P0). Being honest about what's
specific vs general is the prerequisite.

### What's stope-shaped (the signals encode assumptions)
The three behavioral signals are not domain-neutral — each assumes a coding style:

| signal | assumes | weak/absent on |
|---|---|---|
| **returned dict keys** | functions return `dict` results | objects/dataclasses/tuples/`None` returns |
| **attribute write-targets** (`self.world.x`) | mutable OOP self-state; mutation = effect | functional / immutable / pure code |
| **emitted string literals** | functions emit user-facing text (game/IF/CLI) | libs where strings are log lines / dict keys (noisy) |

And the constants encode stope's naming/architecture (now flag-overridable, but
the *defaults* are stope): `_ROLE_PREFIXES` (verb_noun handlers), `_DELEGATION_ROOTS`
(harness-wraps-`self._engine`), `_DEFAULT_DISPATCHERS` (a `step("verb")` command
loop), and the canonical `engine*×testing` boundary (harness-vs-prod drift). The
`missing_twins` reachability gate is the sharpest example: it only makes sense
because stope routes everything through one dispatcher.

### What's genuinely general (keep)
- The **hybrid loop**: recall (gate) → adjudicate (reasoner) → registry
  (substrate) → verdict (action). This is the consensus 2026 architecture (§6).
- The **ranker**: per-signal jaccard, weighted + renormalized over available
  signals, noise-gate, best-match dedup. Nothing in `rank()`/`score_pair` is
  stope-specific — it scores whatever signals it's handed.
- The **delegation down-weight** (adapters can't drift), **calibration**,
  **metabolism**, and the "a nose, not a judge" stance.
- Per-repo `.calque/registry.md` as portable, git-tracked substrate.

### The design path: pluggable signal *profiles*, not more flags
The survey's load-bearing finding: **there is no universal behavioral signal** —
behavior is unknowable statically, so any detector must pick *domain-appropriate
invariants* (NSF 10113743). Therefore the right architecture is:

> Keep the ranker; make **extraction** a pluggable `FuncSig`-producer profile.

- **`effectful-oop`** (today): emitted strings + attr-writes + ret-dict-keys +
  callees + name-stem. Fits stope, IF engines, CLIs, stateful services.
- **`functional`**: return-value shape/type, parameter names, raised exception
  types, called names. Fits pure/transform-heavy code.
- **`api-contract`**: routes, HTTP verbs, param names, status codes, payload
  keys. Fits the client-server drift case (stope #233, JS↔Python) — also the
  cross-language gap, since both sides map onto the *same* contract vocabulary.
- **`data-pipeline`**: input/output column/schema names, dtypes.

Configurability (`--role-prefixes` etc.) was step one — it removes *hardcoding*
but not the *signal-shape* assumption. Profiles are step two. Validation
(roadmap P0 #2) is how we'd know a profile generalizes rather than just believe
it — the survey's "generalization cliff" warning is that per-repo intuition does
*not* transfer; only benchmark recall numbers do.

**Bottom line.** calque is a validated *instance* (stope) of a general *shape*.
The shape is SOTA-aligned; the instance is one profile. The work to "become 2026
SOTA for this kind of problem" is precisely: factor the profile boundary, add 2–3
profiles, and prove recall on GPTCloneBench/CETBench. Until then, calque is
honestly described as "a dual-path finder tuned for effectful-OOP Python," not "a
universal one."

---

## 14. The env/config-parity axis (a second axis, from stope usage 2026-06-05)

Real-usage feedback from the stope side (`/tmp/calque_feedback_env_parity.md`,
folded in here so it's durable) surfaced a dual-path divergence calque's code axis
**cannot** see — and shouldn't be bent to. Worth recording as a distinct axis.

**What happened.** A live playtest booted the app via `make roleplay-server`, which
left `STOPE_INPUT_LLM` / `STOPE_NOUN_EMBED` **unset** while prod
(`deploy/fly.toml [env]`) sets both. So the session exercised a non-prod input path
without knowing it — at least the 6th time a harness/prod flag divergence has bitten,
despite a standing rule (`feedback_harness_mirror_prod_flags`).

**Why calque misses it.** calque finds Type-4 *code* clones — two code paths that
should converge. This is the dual: **one** code path fed **different config by two
launchers**. The divergence lives in the *environment a process boots with*, not in
duplicated code. calque watches code; it has no notion of "these two run targets
should boot the same effective config."

**Same meta-bug, different substrate.** The deeper finding is squarely calque's
thesis: config has no single source — ~65 scattered `os.environ.get("STOPE_*", default)`
reads across ~38 flags, each with its own inline default, plus a hand-maintained
`PROD_PLAYER_FLAGS` Make var mirroring only 3. "Same value defined in N independent
places that drift" is *exactly* the meta-bug calque kills, just in config not code.

**Shape of an env-parity check** (sibling to the code axis — same recall→adjudicate→
registry loop, different lens):
1. **Inputs / run profiles.** `deploy/fly.toml [env]` as canonical prod, plus every
   launcher that boots the app (`make dev`, `dev-no-reload`, `roleplay-server`,
   `panel_driver.sh`, CI/test conftest).
2. **Check.** For each launcher, diff its effective exported env against the prod
   profile, minus a declared whitelist of intentional dev overrides (e.g.
   `STOPE_API_DEV`, `STOPE_DB_PATH`). Flag any parity-critical flag that differs or
   is unset. This is a *static, across-all-launchers, pre-boot* check.
3. **Registry.** Keyed on `(flag, [launchers that set it], prod value)` — a launcher
   omitting a parity-critical flag becomes a registered divergence, not a surprise.

**Relation to the app-level fix.** stope is fixing its instance with a typed config
layer (`stope/config.py`) + a boot-time parity assertion (server refuses to start if
effective parity-critical config ≠ the fly.toml prod profile). That boot guard is
runtime enforcement for one process; a calque env-parity check is strictly broader —
it catches the gap statically across *all* launchers calque can read, before boot.

**Scope call (don't over-rotate).** This is a genuinely different lens (parse shell/
Make/TOML/conftest env exports, not Python AST signatures), so it's a *sibling
check*, not a profile of the code extractor. It is **planned, not started** (same
dogfood-first discipline as P0). Recorded here and in the roadmap (§9, P1) so it
isn't lost; build it only when stope usage shows the boot-guard isn't enough.


## 15. The triple-shell finding — granularity + N-ary recall (from stope #269, 2026-06-06)

Two days of stope work (2026-06-04→06) were almost entirely one activity: finding
and collapsing parallel implementations of a single contract. The meta-bug recurred
in **four substrates** in 48h — talk paths (#228/#230), constructor-vs-legacy
(#224/#234), harness-vs-prod tests (the migration waves), env/config (#240, §14) —
and the dominant one was a **triple**, not a dual. This is the most important recall
evidence calque has gathered since the seed run, because it shows where the current
nose is blind.

**The #269 triple session-shell.** Three orchestrations turned a raw player line into
a dispatched command, each independently inlining the same
`[_parse_action → read/clear _agent_canon → dispatch]` block:
`GameEngine.step` (programmatic), `GameSession.step` (web/prod), and
`GameEngine.run` (interactive CLI loop). The third drifted **silently for months**:
`run()` called `_parse_action` directly and ignored `_agent_canon`, so it dispatched
the model's raw line instead of the #128 constructor's canonical command. Cure:
extract `_resolve_clause` + `_prepare_command_line`, route all three through them.

**What calque caught vs missed.** It caught the harness-vs-prod boundary
(`engine*×testing`, the seed run — 4 drifts, those are *name twins*). It would have
**missed the #269 trio**, even though the fingerprint is fully present in today's
signals: `self._parse_action(...)` is recorded as call `_parse_action`;
`getattr(self, "_agent_canon", None)` as string `_agent_canon`;
`self._agent_canon = None` as write `_agent_canon`. Three concrete reasons it misses:

1. **Whole-function granularity dilutes it.** The duplicated unit is a 5-line *block*,
   but `step`/`step`/`run` are large methods. The seam's few tokens are swamped, so
   pairwise Jaccard (`score_pair`) scores below threshold. The signal is real but
   drowned.
2. **The name signal misleads.** `step` vs `run` share no stem; name weight (.22) plus
   everything else is diluted. The thing that *should* pair them — a shared private
   seam — isn't a signal at all today.
3. **Pairwise, not N-ary.** calque only emits pairs. Even if `step ≟ step` surfaced,
   `run()` pairs with neither, so the *triple* is structurally invisible.

**The three upgrades this argues for** (all P0-adjacent — they strengthen the existing
code axis, they are not a new sibling like §14):

1. **Rare private-symbol touchpoint signal.** Inverted index: each private symbol
   (leading-underscore call/string/write) → set of functions touching it. A symbol
   touched by 2..K functions (K small) is a *shared internal seam*; weight by rarity
   (TF-IDF — touched-by-50 is plumbing/noise, touched-by-2-4 is signal). `_parse_action`
   is touched by exactly the 3 shells → instant triple, **name- and size-agnostic.**
   This is *presence*-based, so it survives the dilution that defeats Jaccard, and it
   needs no naming convention — strictly more robust than the name-stem signal.
2. **N-ary clustering + N-ary registry.** Emit a *cluster* `{members, shared-rare-symbols}`,
   not just a pair. The registry currently keys on `left/right`; key it on a **set**.
   This is the *same shape* as the env-parity axis (a set of launchers) — both unify
   under "a set of sites that should share one seam."
3. **Emit the antibody.** stope hand-wrote `tests/test_session_shell_unification.py`:
   `inspect.getsource` over the three shells, asserting each contains `_resolve_clause`
   and none contain bare `_parse_action(`. That is literally a manual, instance-specific
   calque output. Once a cluster is adjudicated, calque should generate exactly that
   structural guard — making "close the loop" (§9, P1) concrete: registry → executable
   antibody. stope's own phrase: *detection, not vigilance.*

**General lessons (for the profile/generalization work, §13).**
- **N is usually >2 and the unit is usually sub-function.** Pairwise named-function
  similarity sees the easy third (name twins) and misses the expensive part (triples,
  inlined blocks).
- **The cheap universal tell of a parallel path is a shared touch of a *private*
  symbol.** Public touchpoints are noise (everyone calls them); private ones mean two
  sites do the same internal job. Cross-language, convention-free.
- **Enumerate the project's "shells"** — every entry point that turns the same input
  into the same effect (CLI loop, programmatic API, web handler, test harness, cron,
  launcher env). The audit: do all shells share the core primitive (code) and boot the
  same effective config (env)? Same question, two substrates.
- **The fix's closing move is a structural antibody,** generated from the registry, so
  a collapse can't silently re-divide.

**Status: planned, not started** — same dogfood-first discipline. Recorded so the
next build pass starts from this evidence. The private-symbol-touchpoint signal (#1)
is the single highest-leverage code-axis upgrade the evidence points to.

**Live-hunt corroboration (2026-06-06, LLM-as-nose).** Acting as the nose by hand
(fuzzy-matching the "right spots" rather than running the AST scanner) over stope
surfaced parallel-path drift the current static nose *and* stope's hand-written #269
antibody both miss — confirming the §15 thesis with concrete instances:
- **A taxonomy defined in ≥5 literal sets.** The verb-synonym table
  (examine/look/talk/…) lives in `input_llm._VERB_CONFIG` (canon) + `affordances._EMBED_VERBS`
  (the only copy with a lockstep test) + `memory_scene._map_verb` + `reach_scene.ALLOWED_VERBS`
  + an `engine.py` regex — already drifted (`read`/`listen` exist in only one copy each).
  Invisible to the AST nose: these are *set/regex literals inside differently-named
  functions*, not function twins; the tell is the *pattern* ("a literal set of
  verb-synonyms"), which is an LLM/fuzzy match, not a `FuncSig` signature.
- **A magic constant straddling both axes.** `0.65` (cosine threshold) is a module
  constant in 4 files, synced by a *comment*, and only one honors the
  `STOPE_NOUN_EMBED_THRESHOLD` env override — so the code-duplication axis (§15) and the
  env-parity axis (§14) meet in one bug. Argues the two axes should share a registry
  shape: "a value with N definition sites, some code, some env."
- **A 4th input parser in a different subsystem.** `MemoryScene._parse_input` rolls its
  own prep-strip with no politeness strip (re-opening #269 brick-3 in the scenes), and
  the #269 antibody — scoped to three named shells — can't see it. Argues for the
  registry-driven antibody (a Protocol + registry so a *new* shell is auto-covered)
  over hard-coded shell lists.
- **Calibration win:** the `to_dict`/`from_dict` pairs *looked* like drift to a crude
  scan but adjudication found them clean (disciplined emit-if/get-with-default). The
  recall→adjudicate split earned its keep — same honesty as the seed run's
  4-drift/21-ok/5-false-alarm tally.

Net: an LLM fuzzy-hunt is a viable *nose* for exactly the cases the static heuristic
can't rank (sub-function, cross-subsystem, data-literal, cross-axis). The build
implication isn't "drop the static nose" — it's that calque's adjudicate leg could be
LLM-driven over LLM-surfaced candidates, with the static nose as the cheap first pass.
Fed back to stope as `reference/calque-language-design-feedback.md` (Python levers:
parse-once into a typed Command, `NewType` raw-vs-canon, single-source taxonomies as
data, registry-driven antibody, import-linter contracts). See [[calque-triple-shell-recall]].
