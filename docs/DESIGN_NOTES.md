# calque — design notes

> Architecture, rationale, and roadmap for calque, a substrate-general drift
> nose (Go). Companion docs: `PATTERN_CATALOG.md` (the concrete drift shapes),
> `RESEARCH_AND_MARKET.md` (prior art + competitive landscape).

---

## 1. What calque is

> Overview, quickstart, the name, and the collapse-to-single-path framing live in
> the **README**. This section adds only what the design needs.

calque finds **dual-path code**: two implementations of one contract that should
behave identically but have silently drifted. These are **Type-4 (behavioral)
clones** — the hardest clone class (Types 1–3 are progressively looser *textual*
copies; a Type-4 is alike only in *what it does*), which is why grep, LSP,
embeddings, and clone-detectors all miss them (§2). calque is the **high-recall
GATE** (the *recall* stage — not a separate hybrid role) of a heuristic →
LLM-oracle → registry loop: it indexes the signals that stay invariant when a body
is rewritten, ranks suspect pairs, and hands a short list to an LLM/agent for the
equivalence call. It never *proves* equivalence (undecidable) — a nose, not a
judge. The goal is to **collapse** the copies to one source; pinning two paths in
sync is the fallback.

---

## 2. The core insight (why indices miss it)

The undecidability is real but irrelevant to the need. You don't want a *prover*;
you want **clues to automatically go check** — a recall-heavy generator that routes
an LLM oracle's attention to what's "too close to equivalent." The hard part
(judgment) moves to the part good at judgment; the tool only has to be *suspicious*,
never *right*. A 30%-precision / 95%-recall ranker that yields ~15 pairs to look at
beats any sound analyzer here.

The README walks through *why* grep/LSP, embeddings, and clone-detectors each miss
this; the design consequence is one sentence: they all index **representation**,
but dual-path is a **role collision in behavior-space**, and representation is the
very dimension along which the twins diverged. So calque indexes the **contract
invariants**, not the prose:

**The divergence-robust signals** (`internal/code`):
- emitted string literals — what the function *says* (surface output)
- attribute write-targets — what it *mutates* (effect signature)
- returned dict keys — the shape of what it *hands back*
- callee names — what downstream it leans on
- name-stem (role-prefixes like `handle_`/`resolve_` stripped) — the *role*

None care that the bodies look nothing alike. That's the point.

---

## 3. Architecture — a "hybrid loop"

calque is a concrete instance of the **hybrid-loop** design pattern
([`github.com/justinstimatze/hybrid`](https://github.com/justinstimatze/hybrid) —
"LLM judgment and deterministic code in mutually-generative cycles"). Its catalog
names calque's shape: a **knowledge-base auditor** (substrate is structured
source; deterministic checks run against it; an LLM proposes edits) run as a
**dev-time critique loop**.

Role mapping:

| hybrid role | calque |
|---|---|
| soft input | the codebase / a diff |
| LENS (usually LLM) | **deterministic AST extract** — a *code-block lens* (deterministic because the substrate is already structured code) |
| SUBSTRATE | `.calque/registry.md` (typed pair/cluster → verdict records) |
| GATE | the scorer — signature overlap → rank suspects |
| REASONER | the agent — adjudicate drift / twin-ok / false-alarm |
| ACTION | write verdict + collapse-to-single-path |

**Notable variant:** calque is *gate-first with a deterministic lens*. The
canonical 5-role loop opens with an LLM lens because soft input is fuzzy. calque's
soft input is **code** — already typed — so extraction needs no LLM. (Keep this
prose consistent with hybrid's vocabulary: RECALL is *not* a role; it is the gate
tuned for recall.)

---

## 4. Implementation (file map)

```
cmd/calque/        # the CLI — one file per spine leg / axis
  main.go          # subcommand dispatch + buildVersion()
  scan.go          # code axis: rank dual-path suspects + N-ary clusters
  check.go         # the registry-aware gate (new vs known vs stale)
  vocab_report.go / synonym_report.go / vocab_check.go   # prose axis
  calib.go         # doctor + mark-fire (calibration)
  hook.go          # git pre-commit / Stop-hook installer
  mcp.go           # MCP stdio server (both gates as tools)
  migrate.go       # one-time registry-format converter
internal/
  code/            # the code-axis core: FuncSig extract (go/ast + embedded
                   #   python3 + node/TypeScript extractors) + jaccard scoring + N-ary touchpoint
                   #   clustering (touchpoint.go)
  corpus/ embed/   # prose-axis corpus walker + ollama embedding client
  registry/ pairkey/ glob/   # registry parse, set keys, glob matching
Makefile           # git-tag versioning via -ldflags
```

**Scoring** (`internal/code`): per-signal jaccard, weighted (`strings .30,
writes .30, name .22, calls .10, ret .08`), **renormalized over available
signals** (a pair isn't penalized for neither side emitting strings). A noise
gate requires a real anchor (name overlap ≥ 0.34 OR any surface/effect/ret
overlap) so generic-callee coincidences are dropped. Tiny functions (< 4 lines)
and dunders excluded. Each left fn keeps its single best match.

**CLI contract:** `--left`/`--right` globs are relative to `--repo`. Only
left×right pairs are scored (the testing-vs-prod boundary), not within-group. Go
targets parse via native `go/ast`; Python (`.py`) via an embedded `python3` AST
extractor; TypeScript (`.ts`/`.tsx`) via an embedded `node` script driving the
TypeScript compiler API (one subprocess per scan each). The two subprocess
extractors share one `runJSONExtractor` core so the stdin/JSON/error handling
can't drift. The node + `typescript` runtime dependency for `.ts` mirrors the
python3 one already accepted for `.py`; calque resolves `typescript` from the
scanned repo's `node_modules`, a global install, or `CALQUE_TS`. `.js`/`.jsx`
are recognized as code but not yet extracted (no type surface).

---

## 5. Validation evidence

Run `calque scan --repo <repo> --left "engine*.py" --right "testing.py"`. With
**zero project knowledge**, on a real mid-size Python codebase calque re-surfaced
the engine↔testing reimplementation family that the project itself called its #1
bug source — including a row caught purely by **shared write-targets**
(`world.current_loc`/`world.current_zone`) plus shared calls, and an exact
name-role match on a capped-value sync that both sides delegate to one helper
(a `contracted-twin-ok`, the canonical metabolism risk: safe today, could
inline-drift tomorrow). See `examples/engine-vs-test-harness.md`.

**Key property observed:** pairs that get collapsed to a single path **drop off
the suspect list** automatically, because the delegating body shrinks. The suspect
set shrinks as you single-path — exactly the desired behavior; a regression test
pins it.

**The N-ary win.** On the same target, the touchpoint-cluster pass surfaced a
three-shell cluster (two `step` methods + a `run` loop) sharing private parse/
canon primitives — a sub-function inlined-block triple that whole-function
pairwise scoring structurally cannot express (§15). And in live dogfooding on a
sibling's web/engine boundary, a calque cluster led directly to a real
player-visible bug: an "examinable details" enumeration computed three different
ways (dict-filter + lowercase + mutations vs a raw key-set vs a truncated copy),
so the UI offered a detail the engine refused to resolve. The fix was a
single-path collapse onto the canonical enumerator, locked by a prod-path test
asserting the three agree. Cluster → real bug → single-path fix: the loop, end to
end.

**Quantified seed run (a private sibling engine, 2026-06-05).** A scan over an
`engine*.py × testing.py` boundary adjudicated **30 suspects → 4 genuine drifts, 21
delegating adapters, 5 false alarms**; one of the four was a *shipped prod bug* an
inline eligibility gate had introduced, caught by no test. All four were collapsed to
single-path. This is the clean positive; §18.7 records it alongside the honest limit —
a later, larger dual-path campaign in the *same* repo that calque **missed** because
it lived outside the one boundary the scan was pointed at, which is what motivates the
boundary-blind/role-predicate work (§10, §18.7).

---

## 6. Prior art / research

> Fully developed in **`RESEARCH_AND_MARKET.md`** (primary-sourced): calque's true
> lineage is the *inconsistency-bug* line (Engler 2001 → CP-Miner → DejaVu → FICS
> 2021), not clone detection; a primary-source competitive scan (the niche is
> unoccupied; Greptile is diff-gated, Larridin is an exec score); verified market
> tailwind (DORA, the 304K-commit arXiv study, Stack-Overflow-2025's "66% almost
> right"); and the go/no-go verdict.

Highlights that shape the architecture:

- **The problem is named and measured.** LLMs under-reuse existing code — they
  re-derive behavior instead of calling what's already there (*More Code, Less
  Reuse*, arXiv 2601.21276) — and the resulting Type-4 (behavioral) clones evade
  existing detectors (*Detecting Semantic Clones of Unseen Functionality*, arXiv
  2510.04143).
- **Verdict leg (per-language, off-the-shelf):** differential / model-based
  testing — **Hypothesis** (`RuleBasedStateMachine`) for Python, **rapid** for Go,
  **fast-check** for TS. These *confirm* a flagged pair; they don't discover it
  (full contrast in §12).
- **Differential fuzzing** (arXiv 2602.15761) found ~19–35% of LLM refactorings
  non-equivalent, ~21% slipping past existing tests — calque's intended verdict
  leg, empirically validated, and exactly the "existing tests miss real drift"
  failure mode calque exists to catch.
- **LLM is not a sound oracle** (CETBench 2506.04019; the LLM-as-Judge survey
  2510.24367) — which validates demoting the LLM to triage (REASONER) and
  promoting differential testing to the verdict (ACTION).
- **The effect-footprint signal is still uncommon.** Static *effect/mutation*
  signatures (write-targets, emitted literals, returned keys) as a recall signal sit
  in a sparse intersection: effect systems exist (for typing/optimization),
  behavioral-clone detection exists (but goes dynamic), and the new wave of
  architecture-drift tools indexes *structural* AST similarity — but cheap static
  effect footprints to *match twins* remain rare. This is a narrower claim than
  "original"; see the competitive correction below.

> **Competitive correction (2026-06-08) — the "unclaimed white space" claim was
> stale.** The earlier scan (and the `RESEARCH_AND_MARKET.md` blockquote above,
> which still needs the same fix) concluded the niche was unoccupied. It is **not**.
> Between the initial research and going public, "**architecture drift**" + "**fitness
> functions**" became named industry vocabulary with institutional backing
> (Thoughtworks Technology Radar lists *Architecture drift reduction with LLMs* as a
> technique), and a wave of tools moved into the space — most directly
> **`sauremilk/drift`** (MIT): AST-level near-duplicate detection ("mutant
> duplicates", "pattern fragmentation"), 24 cross-file signals, undeclared-dup
> discovery + declared boundaries, a feedback/calibration loop reweighted on git
> outcomes, and a CI gate. That is close to calque's recall→registry→check→calibrate
> spine, code-side, with a more mature calibration loop and partial TypeScript. Also
> in-category: CodeAnt AI, `@aiready/pattern-detect`, similarity-ts, CloneDR.
>
> **What survives the scan as genuinely differentiated** (these sharpen rather than
> dissolve under the competition):
> 1. **Type-4-by-construction.** The drift wave indexes *structural* AST similarity —
>    Type 1–3 (renamed / edited-with-gaps). It is blind, by definition, to twins that
>    are *syntactically dissimilar* (the stub vs the real path; two different-API
>    backends). calque's effect-footprint recall aims at that slice; AST similarity
>    cannot reach it.
> 2. **Role-cardinality / declare-and-gate (§18).** A targeted search found *no* tool
>    asserting "this role should have exactly one canonical implementation." Fitness
>    functions enforce dependency/import/API/perf-budget constraints, not singularity.
>    "How many implement role R" subsumes the similarity question and is unoccupied —
>    and a private sibling just field-validated it (§18.7). **Lead positioning here.**
> 3. **Substrate-generality.** The recall→registry→check→calibrate spine is
>    substrate-agnostic (code + prose live; config/catalog/narrative roadmapped);
>    the competitors are code-only.
>
> Net: this is a *positioning* miss, not a *thesis* miss. calque is not first-to-
> category and must stop implying it. Its defensible ground is Type-4 + cardinality +
> substrate-generality, not first-mover. `drift` deserves a close, fair read — it is
> the sharpest competitor and, as a fellow open-source (MIT) project, prior art worth
> learning from and crediting; that study is queued.

### 6.1 `drift` teardown (2026-06-09, primary-source-verified)

The queued study, done — every claim below checked against the live repo, not
recalled. **Repo path resolved:** the canonical, current repo is
**`github.com/sauremilk/drift`** (README tagline "24 signals · deterministic, no
LLM"; PyPI `drift-analyzer`; CLI `drift`; Python 3.11+). `mick-gsk/drift` serves the
*same* `docs-site/` content (a prior handle/transfer, not a divergent fork) — cite
`sauremilk/drift`. License **MIT**, confirmed.

**Verified specifics (the 2026-06-08 block above held up; these sharpen it):**

| Claim (as written 06-08) | Primary-source verdict (06-09) |
|---|---|
| "24 cross-file signals" | ✅ tagline "24 signals"; named: PFS (pattern-fragmentation), MDS (mutant-duplicate), AVS (architecture-violation), BAT (bypass-accumulation), TPD (test-polarity-deficit), +19. (Deep-dive doc says "25" once — a benign internal inconsistency; 24 is the headline + the TS denominator.) |
| "feedback/calibration loop reweighted on git outcomes" | ✅ **and more mature than implied**: "precision-weighted linear interpolation across three evidence sources — explicit `drift feedback mark`, git-outcome correlation, and GitHub issue/PR-label correlation"; "observed signal precision gradually overrides default weights." CLI: `drift feedback` / `calibrate` / `precision`. |
| "partial TypeScript" | ✅ **17/24 signals** on TS via tree-sitter (`pip install 'drift-analyzer[typescript]'`); Python is primary. |
| "declared boundaries" | ✅ but it's **layer/import boundaries only** (`drift.yaml`, architecture-violation rules) — **NOT cardinality.** Confirmed: "no mention of specifying that a function/role must have precisely one or multiple implementations." calque's role-cardinality axis stays unoccupied *even against the sharpest competitor.* |

**Two learnings worth acting on:**

1. **The differentiator is now algorithmically precise, not hand-wavy.** drift's
   duplicate detection is **AST-structural with no data-flow/effect analysis**
   (verbatim: "Drift's duplicate detection is AST-level, not text-level"; no write-set
   / return-value / side-effect signal anywhere in the pipeline). MDS = bucket by LOC
   ±10% → Jaccard ≥ 0.80 on **3-grams of AST node types, normalizing away identifiers
   and literals**. That construction is *provably blind* to calque's slice: a stub vs
   the real path lands in a different LOC bucket with a disjoint n-gram multiset
   (Jaccard ≪ 0.80); two different-API backends share no AST shape. calque's
   effect-footprint anchors (strings/writes/ret/name-stem — `internal/code/score.go`)
   match on *what the code does*, which survives exactly the rewrites that move a pair
   below drift's 0.80. They are complementary axes, not rivals — and the gap is a
   theorem about drift's normalization, not a marketing line.
2. **The calibration steal (actionable, and §13-legal).** drift reweights signal
   precision from **adjudicated git outcomes + issue/PR labels**, not from a self-scan.
   calque's `weights` map is hand-tuned and static (strings .30 / writes .30 / name
   .22 / calls .10 / ret .08); `doctor` only *reports* precision@k against registry
   verdicts — it never feeds them back. The §13 prohibition is on tuning to *self-scan
   noise*; reweighting from **adjudicated registry labels** (drift = useful,
   false-alarm = not-useful — the labels `doctor` already joins) is the principled,
   §13-clean path to non-static weights. This is the one place drift is genuinely
   ahead, and the upgrade is well-scoped: `doctor` already computes the join; the
   missing piece is emitting a calibrated weight vector the gate can load.

   **Shipped (2026-06-10), task #1.** `calque calibrate` does exactly this. The
   estimator (`internal/code/calibrate.go::CalibrateWeights`, pure + table-tested)
   measures, per channel, `mean(signal | useful) − mean(signal | not-useful)` over
   adjudicated pairs, normalizes the positive discriminations into observed weights,
   and shrinks them toward the static prior by `λ = n/(n+priorStrength)` — so a
   handful of labels can't overfit. `--write` emits a git-tracked `.calque/weights.json`;
   the gate (`scan`/`check`/`doctor`) loads it via `applyCalibratedWeights` and scores
   with it, with `--no-calibrated-weights` to fall back to the prior. Two deliberate
   §13 guards: (a) the gate never auto-writes — calibration is an explicit act; (b)
   `calibrate` always trains on the static prior (`ResetWeights` up front), never on
   its own prior output, so it can't chase its tail. Crucially, `hasAnchor` in
   `scorePair` is weight-independent, so a calibrated vector only **re-ranks** anchored
   pairs — it can never surface or suppress an anchor, leaving the #16 blocking-index
   superset invariant intact. The honest limitation surfaced on first run: calque's own
   registry has only **one** useful-labeled pair, so its calibrated vector overfits that
   single positive (a thin-minority-class warning fires, and calque's repo ships no
   weights.json — it stays on the prior). The estimator is sound; the labels are thin —
   which is the §13-correct place to stop, not a reason to tune harder.

Net unchanged: positioning miss, not thesis miss. The teardown *strengthens* the
three differentiators by grounding #1 in drift's own algorithm and confirming #2 is
unoccupied against the strongest in-category tool.

> **⚠ Validation update (2026-06-10) — differentiator #1 (Type-4-by-construction)
> is DEMOTED by measurement; see RESEARCH_AND_MARKET §5 for the full evidence.** A
> falsification-first probe (confirmed twins from the dogfood corpus + an adopter's
> 4-day field tally) found scan recall on genuine no-shared-footprint Type-4 twins is
> ~0 *even on the repo the signals were tuned to* — the misses are exactly the
> no-footprint cases; the catches are name/footprint (Type 1–3). What *is* validated:
> the persistent registry as durable memory (the adopter's most-used surface), the
> N-ary/architecture-granularity cluster pass, and **cross-substrate** drift (code ↔
> config/JSON/sqlite/prose) as the real unoccupied frontier — the adopter's hardest
> hand-found drifts lived there, with a ready 4-case validation set.
>
> **Update (same day) — the Type-4 detector was then BUILT and measured; the demotion
> applied to *jaccard* recall, not Type-4 detection.** A representation-independent
> pipeline now exists: candidate generation by rare domain-typed type signature
> (`propose-deep`) + near-identical name-stem token sets (language-agnostic), unioned and
> ranked by gate-invisibility, with an LLM judge (`--judge`) classifying each into
> drift / contracted-twin-ok / false-alarm. Measured un-circularly: candidate recall 3/3
> and judge precision 3/3 on human-verified labels, judge recall 8/8 on
> by-construction synthetic twins (held de-confounded with split rewriter/judge models).
> Differentiator #1 is **re-earned** — see RESEARCH_AND_MARKET §5.1 for the full numbers,
> bounds, and the honest n=4-small caveat.

---

## 7. Naming

Chose **calque** after collision checks. Rejected: `twinscan` (ASML lithography +
bioinformatics), `mitosis` (Builder.io framework), `samesame` / `lockstep` /
`cleave` (existing tools/SDKs). The PyPI/npm/GitHub namespaces were free; the
metaphor is dead-on (a cross-language structural copy).

---

## 8. Relationship to sibling projects

- **[hybrid](https://github.com/justinstimatze/hybrid)** — the meta-framework
  calque instantiates; calque is its "knowledge-base auditor" exemplar.
- **[defn](https://github.com/justinstimatze/defn)** — a Go code-graph
  (definitions + call/ref/interface edges). It indexes the *reference graph*, so
  it's structurally blind to dual-path (twins don't call each other) — but it's an
  ideal **Go-side recall backend**: calque's signals can become queries over
  defn's graph instead of a fresh parse. Same philosophy (externalize code
  knowledge that dies at context-close), different target.
- A private interactive-fiction engine is calque's **calibration target and first
  real customer** — the source of the calibration findings cited throughout this doc.
- The prose axis was consolidated from a sibling prose-criticism project (MIT; see
  §16).

A survey of the author's projects found **no prior dual-path / code-equivalence
tool** — calque is the first solution to this problem in the portfolio, not a
duplicate.

---

## 9. Roadmap

The architecture is the consensus 2026 shape (recall → adjudicate → verdict). To
*be* state-of-the-art, calque must (a) generalize its signal beyond one project's
conventions, (b) close the loop with a real verdict leg, and (c) prove recall on a
public benchmark.

**Done**
- Code axis (Go `go/ast` + embedded `python3` + embedded `node`/TypeScript),
  parity-verified against the original Python implementation's scores.
- The registry-aware `check` gate; warn-only + `--strict`.
- **N-ary private-symbol touchpoint clustering** (§15) — the recall upgrade for
  inlined sub-function seams; validated on the real target.
- **Calibration** (`doctor` + `mark-fire`): discrimination signal (mean score of
  useful vs not-useful suspects) + precision@k.
- **Hook** (`hook install` — git pre-commit / Stop-hook) and **MCP** (`mcp` —
  both gates over stdio JSON-RPC).
- Prose axis (`vocab-report` / `synonym-report` / `vocab-check`), consolidated
  from a sibling project (§16).
- Git-tag versioning (hindcast pattern); `migrate-registry` for the old format.

**P0 — Role-cardinality invariants (MVP SHIPPED — `calque cardinality`; see §18)**
0. **Assert a declared implementation-count for role R.** *Status: the
   explicit-declaration MVP is built* — `calque cardinality` reads `- role:` /
   `- predicate:` / `- expected:` / `- baseline:` declarations from the registry,
   enumerates each role's implementers across the repo via the existing extractor
   (delegating wrappers excluded, §12), and gates on count + frozen-baseline membership
   with `--strict`. Dogfooded on calque's own source. The remaining slices —
   cluster-proposed role candidates (§18.5 option 2), the boundary-blind predicate scan
   (§10), `doctor` non-vacuity calibration, and collapse-direction registry fields — are
   tracked but not yet built. Original derivation below. The strongest signal
   dogfooding has produced (two verified drift instances in a private sibling engine,
   2026-06-07) is that the next primitive is **not another pairwise signal** — it
   is a *cardinality invariant on a declared role*: "role R should have one
   implementation (the usual collapse target); flag whenever it has **two or
   more**." The violation is N≥2 and N is unbounded — the multi-path case (the
   triple-shell case in §15 was N=3) is the general form. Both real bugs — a stub path that
   short-circuits the real one, and two independent record/replay backends for a
   single constructor — defeat pairwise similarity *by construction* (the twins
   share little footprint, or use different APIs) and need N=1 enforcement
   instead. This inverts calque's mechanism — *declare-and-gate* rather than
   *discover-similar* — and is the only form that catches **recurrence** (a second
   implementation reappearing because nothing forbids it), which no pairwise scan
   does. It also cures the failure mode the sibling kept naming ("memory rules
   don't stick"): a declared cardinality is code, not a remembered convention. See
   §18 for the full derivation. A second private sibling (2026-06-08) then
   **shipped a production reference implementation** — an AST-predicate role registry
   gated as a standing test, with a frozen-baseline ratchet that doubles as a
   collapse-progress ledger — and independently named the primitive ("assert exactly
   one constructor replay backend"). Build the MVP against that shape: a role is an
   **AST predicate**, and the gate is **set-membership against a declared baseline
   that ratchets toward the target count** (not a bare `count == 1`). See §18.7 for
   the field evidence and the full list of adoptions.

   Also derived from that sibling's field evidence (§18.7), to build alongside the
   cardinality MVP:
   - **Non-vacuity.** *Gate-level guard SHIPPED:* `cardinality` flags a role whose
     predicate matches zero implementers while expecting ≥1 (`VACUOUS`) — a stale or
     typo'd declaration can no longer pass silently as a green gate. The deeper
     `doctor` mutation-verification (does the detector flag a known member when its
     allow-list is emptied) remains as a follow-on.
   - **Collapse-direction in the registry schema** — *SHIPPED (task #18):* first-class
     `- canonical:` / `- do-not-resync:` fields on a `drift` pair, surfaced by `check`
     as `DRIFT (unresolved)` (warn-only) so a later agent collapses the doomed path
     instead of re-syncing it (maintaining the dual path). The delegation
     gate (§12) is separately field-validated by the same campaign (21/30 seed
     suspects were delegating adapters) — keep it, no work needed.

**P0 — Generalization (the SOTA-blocker; see §13)**
1. **Pluggable signal profiles.** Factor extraction behind a `FuncSig`-producer
   interface; ship profiles beyond the current *effectful-OOP* one: *functional*
   (return-shape, params, raised exceptions), *API/contract* (routes, params,
   status codes), *data-pipeline* (input/output schema). The ranker is already
   domain-agnostic — only the signal set is project-shaped. There is **no
   universal behavioral signal**, so profile-per-domain is the correct
   architecture, not a hack.
2. **Recall benchmark.** Validate on GPTCloneBench + CETBench: what fraction of
   known Type-4 pairs does calque rank as suspects? This is how SOTA is *claimed*,
   not asserted — and it guards against the per-repo "generalization cliff."
3. **Antibody generator (registry → executable guard).** When a cluster is
   adjudicated, emit a structural test (assert every member routes through the
   shared primitive, none call the forbidden bare primitive). Makes the verdict
   leg concrete for the collapse-to-single-path case. *Planned.*

**P1 — Second axes**
4. **Env/config parity** (§14) — one code path booted with different effective
   config by different launchers; the same meta-bug in config rather than code.
   A *sibling check*, not a code profile (it parses shell/Make/TOML/CI, not ASTs).
5. **Verdict leg.** A `RuleBasedStateMachine`/`rapid`/`fast-check` template that
   drives both sides of a `contracted-twin-ok` pair and asserts equal outputs.
6. **Metabolism.** Re-check `contracted-twin-ok` pairs for fresh drift (re-extract;
   re-flag if a side's signature changed since the verdict).

**P2 — Reach & ergonomics**
7. **TS extractor.** ✅ **Shipped.** Go (`go/ast`), Python (embedded `python3`),
   and TypeScript/TSX (embedded `node` driving the TS compiler API) all extract
   natively; the two subprocess extractors share one `runJSONExtractor` core.
   Validated read-only on a 302-file TS repo (`.js`/`.jsx` still deferred — no
   type surface).
8. **Boundary presets** (`--preset harness|client-server|v1-v2`) so users don't
   hand-craft globs.
9. **Cross-language dogfood — TS.** The meta-bug + the private-symbol-touchpoint
   signal are already confirmed on Python and Go siblings (§13); TypeScript is the
   open one (TS levers: discriminated unions, branded/opaque types, `as const`
   taxonomies).

---

## 10. Open questions

- **Registry location.** Per-repo `<repo>/.calque/registry.md` (chosen) for
  portability + git-tracked memory, vs a central store. Confirm at scale (§17.5).
- **Cross-language pairs.** Real dual paths include JS-client ↔ Python-server. A
  single-language AST nose can't express those; the options are a shared
  spec/contract both extractors map onto, or a differential at the API boundary.
  Out of scope for the AST core; a known gap.
- **Within-repo whole-scan mode.** The boundary-blind default already ships (an empty
  `--left/--right` scans all×all); task #16 slice 1 made it *tractable* — `code.Rank` now
  blocks via an inverted index over the four anchor channels (`internal/code/block.go`)
  instead of the O(n²) double loop, **output-identical** (proven by
  `TestRankBlockingEquivalence`) while scoring ~3% of pairs on calque's own source. The
  index degrades gracefully to O(n²) only on a token shared by nearly every function
  (kept exact on purpose — pruning would drop genuinely-anchored pairs). *Still open
  (slice 2, measurement-gated):* the **no-shared-footprint class** — pairs that share no
  anchor-channel token at all, so blocking correctly never generates them and the gate
  never saw them. That needs a separate, lossy discovery signal (name-stem / call-target /
  return-shape), kept only if it earns its precision against the adjudicated registry
  (§13). *Slice-2 measurement ran (2026-06-09) and stopped at the precondition:* a registry
  records only what the gate already surfaced (every entry has shared footprint by
  construction), and the adopter seed is likewise gate-found pairs — so the labeled
  no-footprint set is n≈1 (the external two-backend anecdote), too thin to discriminate
  three candidate signals without fitting to noise. Deferred, not abandoned: `channelSet`
  in `internal/code/block.go` is already parameterized to drive a looser-channel discovery
  pass the day a labeled corpus of this class exists. Forcing a signal against n≈1 would be
  the exact self-scan-tuning §13 prohibits. *Concrete evidence (a private sibling, §18):* the
  two-replay-backend bug lived in test-infra×test-infra — a boundary nobody thinks
  to name — so the cluster pass never saw it. You cannot pick the boundary you
  didn't know existed; boundary-blind scanning is the recall fix. *Field-confirmed
  (a private sibling, 2026-06-08, §18.7):* the sibling's whole input-path campaign lived outside
  the one boundary calque was pointed at, and it compensated by shipping in-repo
  AST-predicate antibodies. The lesson for this mode: drive it by a **role
  predicate**, not a left/right glob — enumerate every implementer of a role
  repo-wide and gate the set, which is the same primitive as the cardinality axis
  (§18.7 adoption 1).
- **Should the LENS ever be an LLM?** For *prose-heavy* substrates a deterministic
  lens underperforms — which is exactly why the prose axis (§16) carries embedding
  recall.

---

## 11. How to pick up

```bash
go install github.com/justinstimatze/calque/cmd/calque@latest   # or `make install`
calque scan  --repo <repo> --left "engine*.py" --right "testing.py"
calque check --repo <repo> --left "engine*.py" --right "testing.py"   # gate vs registry
# then adjudicate each suspect and record the verdict in <repo>/.calque/registry.md
# (see registry.template.md for the schema). Tune the boundary, not the threshold.
```

The agent-facing loop is in `SKILL.md`. The core is `internal/code` (the signals +
scoring). Everything else is the shared spine.

---

## 12. The delegation gate, missing-twins, and calque vs Hypothesis

### Delegation gate (precision)
The first run on a harness-heavy target was dominated by *adapters*, not
reimplementations: harness methods that just forward to a wrapped engine and
repackage the result. They're **named after** what they wrap, so the name-stem
matches the real method — a guaranteed false-positive anchor. The scorer detects
forwarding to a wrapped impl (`_engine`/`_impl`/`_inner`/…) and sets a `delegates`
flag, so a **name match alone can no longer anchor a delegating pair** — it must
also share real surface/effect. Pure delegators drop off; rich adapters that still
share emitted strings/calls remain, correctly, since they own glue logic that
*can* drift.

### Missing-twins + reachability gate (recall)
The pair-ranker is structurally blind to a contract that exists on the left with
**no twin on the right at all** (never written / deleted) → zero pairs → invisible.
`missing_twins` (`--missing`) covers that: it **learns which role prefixes are
twinned on this boundary** and reports only gaps within those roles. To avoid
over-flagging a harness built around one command dispatcher (a verb driven by
`step("pray")` is fully tested with no `def pray()`), it takes a `--missing-corpus`
**reachability gate** — the verb vocabulary of *dispatcher-call* string args in a
usage/test corpus — and suppresses any candidate whose role-stripped stem is
covered. It remains a **coarse, opt-in recall aid, not a clean report**: the
residue is engine internals whose role-prefix was twinned by coincidence, which no
structural signal can rule out (that's domain knowledge).

### calque vs Hypothesis (different legs, not competitors)
**calque is discovery; Hypothesis is verification** — opposite ends of the same loop.

| | **calque** | **Hypothesis** (`RuleBasedStateMachine`) |
|---|---|---|
| Question | *Which* pairs might be the same contract? | Does *this* known pair behave identically? |
| Method | static AST signal overlap, ranked | runs both, generates inputs, compares |
| Input | a whole boundary, zero setup | one pair + a hand-written model |
| Output | a ranked suspect list (recall) | a concrete counterexample, or none in N tries |
| Soundness | unsound (heuristic; over-flags) | a failure is a *real* bug |

The asymmetry: **Hypothesis cannot *discover* a dual path** — it can only
prove/disprove a hypothesis you already supplied (at which point you already knew).
calque finds the drift you *didn't know to test*; conversely it never *confirms*
equivalence. So they **compose**: calque (GATE/recall) → adjudicate (REASONER) →
pin a `contracted-twin-ok` pair with a differential test (ACTION/verdict leg).
calque is the smoke detector; Hypothesis is the lab assay. You want both.

---

## 13. Generalization — what's proven, what's still tuned

An earlier draft framed this as "calque is tuned to one project's effectful-OOP
conventions, not universal." Cross-project dogfooding has made that too pessimistic.
The honest move is to be precise about which *parts* generalize — because some are
already validated well beyond one project and one language, and one is genuinely
still style-dependent.

### What's validated (Python solid; Go promising-but-unconfirmed)
The rock-solid validation is **Python**. On a real effectful-OOP interactive-fiction
engine, calque surfaced a triple-shell input path and a live api-vs-engine "examinable
details" divergence (the UI offered a detail the engine then refused to resolve). Those
fixes landed in *that project's own git history*, with calque credited in the commit
messages — independent of any claim made here.

**Go is promising but not yet confirmed.** calque's own (small) Go source is
clean-validated — it caught its own signal-taxonomy duplicated across four sites — and on
other Go projects it surfaces plausible candidates. But the one candidate we hand-checked
(an API-schema cluster in a CLI) turned out to be a **false positive**: `input_schema`
(Anthropic API) and `inputSchema` (MCP) are both correct for their respective protocols;
the cluster conflated two protocols that share generic JSON-schema vocabulary. So Go today
is "produces promising output that still needs adjudication," not "validated." It does run
natively on Go (`go/ast`) and Python.

(The earlier `PATTERN_CATALOG.md` cross-repo survey was an LLM-as-nose *hand*-hunt, not
calque-the-tool — evidence the meta-bug is widespread, not that calque catches it
automatically.)

### What generalizes cleanly (the engine)
- The **N-ary touchpoint/cluster signal** is the most general piece: it keys on the
  *presence* of shared rare private symbols, needs **no naming convention** and no
  language-specific assumption, and is validated on both Python and Go. It carries
  the load precisely where the pairwise effect-signals are thin.
- The **spine** — recall → registry → check → calibrate — is fully
  substrate-general (it already drives the prose axis too).
- The **ranker** (weighted jaccard, renormalized, noise-gated, deduped) scores
  whatever signals it's handed; nothing in it is project-specific.

### What's still tuned (the pairwise effect-weighting)
The genuine residual: the *pairwise* scorer leans on three effect-signals whose
richness depends on coding style —

| signal | richest on | thin on |
|---|---|---|
| **attribute write-targets** | mutable/stateful OOP (mutation = effect) | functional / immutable / pure code |
| **emitted string literals** | code that emits text (games, CLIs, services) | libs where strings are log lines / keys |
| **returned dict/struct keys** | dict/record-returning code | tuple / `None` / scalar returns |

On pure-functional, value-returning code these go sparse and the pairwise pass
leans on name-stem + callee overlap — weaker (the cluster pass partly offsets this,
since it doesn't depend on them). The default role-prefixes / delegation roots are
likewise conventional, though flag-overridable.

### The design path: pluggable signal *profiles*
Because there is **no universal behavioral signal**, the architecture keeps the
ranker and makes **extraction** a pluggable `FuncSig`-producer profile
(`effectful-oop` today; `functional` — return-shape / params / raised-exceptions —
then `api-contract`, `data-pipeline`). That rounds out the pairwise pass for the
styles where it's currently thin. A public recall benchmark (P0 #2) is how you'd
*prove* a profile generalizes rather than believe it.

**Bottom line.** calque is more than "one Python-tuned profile," but less than
"general." Its engine — the convention-free seam-cluster signal plus the
recall→registry→check→calibrate spine — has one rock-solid validation (a real Python
project, fixes in that project's git crediting calque) and runs natively on Go, where it
so far produces promising-but-unadjudicated candidates. What remains genuinely
style-dependent is the *pairwise* effect-weighting, richest on effectful/stateful code.
The honest one-liner: a working dual-path nose with a confirmed win on effectful Python,
a real spine, and a vision (more axes, more languages, profiles + a public recall
benchmark) that is still mostly roadmap.

### Explored and rejected — don't redo these without new evidence

Three plausible "make the pairwise pass more general" ideas were prototyped against
our own adjudicated registries (the only ground truth we trust) and the evidence
said **no**. Recorded here so they aren't re-attempted from scratch.

1. **`constructs` / `retType` pairwise signals** (give value-returning code a footprint
   beyond name+calls). A density probe confirmed the gap is real — ~69% of functional
   Go funcs give the scorer only name+calls — but the proposed signals would fire on
   only ~⅓ of those, and *none of the drift we've actually adjudicated* (calque's
   signal-taxonomy duplication; the input-path shells) is the value-returning-utility
   kind they target. The remaining weak funcs are tiny utilities (set ops, `jaccard`)
   where drift doesn't occur and no cheap static signal exists — the real "no universal
   behavioral signal" floor, not laziness.

2. **Literal-seam clustering** (extend the touchpoint pass to shared rare *literals*,
   e.g. a magic `0.65`). A miner over a large Python repo showed shared numeric
   literals are **noise**: the same value (`14`, `999`, `2.5`) co-occurs across
   unrelated functions by coincidence — numbers are low-entropy, unlike private
   identifiers. The one real literal-drift in our registry (calque's *identifier*-shaped
   taxonomy keys) is already caught because those literals pass `isSeam`. The canonical
   magic-constant case is a *module-level* constant, a granularity the function-scoped
   model doesn't address anyway.

3. **Cohesion-weighted cluster ranking** (dampen subset-shared seams so a bushy cluster
   can't out-score a tight one). Implemented and measured via `doctor`: it did **not**
   improve discrimination on calque (drift still ranked below the intentional-parallelism
   clusters; precision@5 slipped 3/5→2/5) and was reverted.

**Why `doctor` reads "not discriminating" on calque — and why that's mostly fine.**
calque's own codebase is unusually parallel *by design* (compute/render twins, the
CLI-command family, the MCP tool-defs); those score high and are correctly adjudicated
`contracted-twin-ok`. The cluster score ranks *how parallel/suspect* a set is — **not**
*drift vs intentional-twin*; separating those is the adjudication step's job, by design.
So a low drift-vs-twin-ok separation on a heavily-intentional-twin codebase is expected,
not a ranker bug. The score's real job — separating real parallel structure from
coincidence — it does (≈60% genuine in top-10). The lever for precision is **the
boundary and the registry**, not the cluster-score formula. Don't tune the formula
against calque's self-scan; it's a pathological sample.

---

## 14. The env/config-parity axis (a second axis)

Real usage surfaced a dual-path divergence the code axis **cannot** see — and
shouldn't be bent to. A live run booted an app via one launcher that left two
LLM/embedding env flags **unset** while production sets both, so the session
exercised a non-prod input path without knowing it.

**Why calque misses it.** calque finds Type-4 *code* clones — two code paths that
should converge. This is the dual: **one** code path fed **different config by two
launchers**. The divergence lives in the *environment a process boots with*, not
in duplicated code.

**Same meta-bug, different substrate.** Config has no single source — dozens of
scattered `os.environ.get("…", default)` reads, each with its own inline default,
plus a hand-maintained mirror of a few in a Make var. "Same value defined in N
independent places that drift" is *exactly* the meta-bug calque kills, in config
not code.

**Shape of an env-parity check** (sibling to the code axis — same recall →
adjudicate → registry loop, different lens):
1. **Run profiles.** A canonical prod profile (e.g. `fly.toml [env]`), plus every
   launcher that boots the app (`make` targets, dev scripts, CI/test conftest).
2. **Check.** For each launcher, diff its effective exported env against the prod
   profile, minus a declared whitelist of intentional dev overrides; flag any
   parity-critical flag that differs or is unset — a *static, across-all-launchers,
   pre-boot* check.
3. **Registry.** Keyed on `(flag, [launchers that set it], prod value)`.

This is a genuinely different lens (parse shell/Make/TOML/conftest, not Python
AST), so it's a **sibling check, not a profile** of the code extractor. *Planned*
— build it when an app-level boot-parity guard proves insufficient.

---

## 15. The triple-shell finding — granularity + N-ary recall

The dominant drift shape observed in real use was a **triple**, not a dual — and
it shows where whole-function pairwise scoring is blind.

**The triple session-shell.** Three orchestrations turned a raw input line into a
dispatched command, each independently inlining the same `[parse → read/clear a
canon → dispatch]` block: two `step` methods (programmatic + web) and a `run` loop
(interactive). The third drifted **silently for months** — it parsed directly and
ignored the canon, dispatching the raw line instead of the canonical command. The
cure: extract the shared primitives and route all three through them.

**Why the old nose would miss it** — even though the fingerprint is fully present
in the signals:
1. **Whole-function granularity dilutes it.** The duplicated unit is a ~5-line
   *block*, but the methods are large; the seam's few tokens are swamped, so
   pairwise jaccard scores below threshold.
2. **The name signal misleads.** `step` vs `run` share no stem; the thing that
   *should* pair them — a shared private seam — wasn't a signal at all.
3. **Pairwise, not N-ary.** Even if two of the three surfaced, the third pairs
   with neither, so the *triple* is structurally invisible.

**The upgrades this argued for (now implemented):**
1. **Rare private-symbol touchpoint signal.** An inverted index: each private
   symbol (leading-underscore / unexported call·write·getattr-string) → the set of
   functions touching it. A symbol touched by 2..K functions is a *shared internal
   seam*, weighted by rarity (`1/fanout`, repo-size-independent, private-boosted —
   touched-by-50 is plumbing, touched-by-2–4 is signal). *Presence*-based, so it
   survives the dilution that defeats jaccard, and it needs no naming convention.
2. **N-ary clustering + N-ary registry.** Emit a *cluster* `{members, shared
   seams}`, not just a pair; key the registry on a **set**. Same shape as the
   env-parity axis (a set of launchers) — both unify under "a set of sites that
   should share one seam."
3. **Emit the antibody.** Once a cluster is adjudicated, generate the structural
   guard (assert each member routes through the shared primitive, none call the
   bare one) — registry → executable antibody. *Still planned.*

**Status: #1 + #2 implemented.** `internal/code/touchpoint.go`
(`ClusterByTouchpoint`) wires into `scan` (an "N-ary clusters" report) and `check`
(NEW-CLUSTER / known / STALE-CLUSTER, keyed on the member set via `pairkey.SetKey`;
the registry parses `- cluster:` lines). `scorePair` is deliberately left untouched
(folding a seam signal into pairwise scoring would shift the parity-verified
baseline; deferred to the calibration leg). **Validated on the real target:** the
pass surfaced the exact three-shell cluster the case demonstrates, and self-dogfood
caught the N-ary extent of calque's own signal-taxonomy drift (a cluster the
pairwise registry entry only saw two of). The open follow-up is *ranking* (the
cluster score sums seam rarities, so bushy multi-seam subsystems outrank the tight
triple — a calibration tuning question, not a correctness one) and the antibody
generator.

**General lessons (for the profile/generalization work, §13).**
- **N is usually > 2 and the unit is usually sub-function.** Pairwise named-function
  similarity sees the easy name-twins and misses the expensive part.
- **The cheap universal tell of a parallel path is a shared touch of a *private*
  symbol.** Public touchpoints are noise; private ones mean two sites do the same
  internal job. Cross-language, convention-free.
- **Enumerate the project's "shells"** — every entry point that turns the same
  input into the same effect (CLI loop, programmatic API, web handler, test
  harness, launcher env). The audit: do all shells share the core primitive (code)
  and boot the same effective config (env)? Same question, two substrates.

---

## 16. calque as a substrate-general drift engine — the prose convergence

**The third substrate: prose.** A sibling project (MIT) — a large
literary-criticism corpus — fought exactly calque's meta-bug ("one concept
expressed N ways that drift") in *prose*, where there are **no native guardrails
at all** (no compiler, types, or tests). It independently **reinvented calque's
entire loop**: a read-only frequency surface of hyphenated compounds (the recall
nose), embedding near-synonyms (recall for the [WEAK] vocabulary-drift case), a
warn-only → strict gate against an allowlist (the registry-aware check), a
calibration rollup, and a pre-commit hook.

So the meta-bug is **substrate-independent**: code (§15), config/env (§14), and
prose (§16) — three axes of "one value/contract/concept defined in N places that
drift," each a sibling instantiation of the same recall → registry → check →
calibrate loop, converged independently. That is strong thesis validation.

**Consolidation: the two toolchains were themselves an instance of the meta-bug**
(two implementations of one loop), so they were collapsed to one. **calque is the
keeper**; the prose project becomes a *consumer* — and verifiably is one now: cupel
deleted its own vocab-audit / vocab-report / synonym tooling, keeping only a thin
`vocab-seed` that feeds calque's allow-list, and its pre-commit runs `calque
vocab-check` on every commit. License: the prose source is MIT, calque Apache-2.0 —
compatible; MIT attribution is preserved for the lifted portions.

**Broad demand for the prose axis.** Several corpora in the portfolio need it (a
forthcoming pattern/lexicon substrate especially), so the prose/vocab axis is a
general capability, not a one-off — it must run easily on arbitrary prose repos.

**The decision: rewrite calque in Go.** The mature, *calibrated* spine
(registry/audit/calibrate/hook/embeddings) already existed in the prose project's
Go; a **single static binary** is the killer requirement (calque must drop into
many repos + CI + git hooks + agent loops with zero runtime deps, where a Python
install is friction at scale); multi-language parsing goes through tree-sitter
regardless of host language, so Python's only real edge (native `ast`) is moot
beyond Python itself. Cost: port the small Python AST nose to Go (the signals —
string literals, call-leaf names, assignment targets, name stems, private-symbol
touchpoints — are shallow/syntactic and map cleanly), and redo the install
(`go install`/binary) and the `/calque` skill.

**Architecture implication.** §13's "pluggable signal profiles" generalizes to
**substrate axes sharing one spine**:
- **recall** (substrate-specific extractor: AST FuncSig for code · compound/
  embedding surface for prose · launcher env-diff for config) →
- **registry** (substrate-general: a set of adjudicated entries; allowlist =
  registry) →
- **check** (substrate-general: warn-only → strict gate, flag new/drifted) →
- **calibrate** (substrate-general: log fires + verdicts → hit-rate → tune signals).

The recall extractor is the only per-substrate part; registry/check/calibrate/hook
are shared. The Go scaffold + git-tag versioning follow the
[hindcast](https://github.com/justinstimatze/hindcast) pattern.

## 17. The axis roadmap — how far "meta" the drift goes

The thesis of §16 generalizes further than code+prose. This is the durable roadmap
(a roadmap, not a build queue — most rows are reasoned from the invariant, not yet
proven on a real substrate; prove one extractor at a time, the way code was).

### 17.1 The invariant and the axis template

The meta-bug is one shape: **one canonical thing → N expressions → independent
drift.** An *axis* is just a pair `(canonical unit, recall extractor)` bolted onto
the shared spine (recall → registry → check/`mcp` → calibrate). The spine is
substrate-general; **the recall extractor is the only per-substrate part.** So "is
there more slop like this?" reduces to "what other recall extractors are worth
writing?" — and adding an axis is "write an extractor"; the registry, the gate, the
MCP tool, and calibration come free.

### 17.2 The axis map

| Axis | Canonical unit | Drift it catches | Status |
|---|---|---|---|
| code | a function's behavior | dual-path twins; N-ary inlined seams | **live** (validated) |
| prose-vocab | a hyphenated compound | invented noun-stacks | **live** |
| prose-synonym | a word | people/person/human word drift | **live** (recall-only) |
| config-env parity | an env var / config key | same key drifts across launchers | planned §14 |
| pattern-pattern (catalog) | a *catalog atom* (named pattern/term) | two atoms = the same move under different names | gap — nearest neighbor of synonym (reuses `internal/embed`) |
| value/constant | a magic value | same threshold/URL/port hardcoded in N places | gap (cheap, deterministic) |
| schema/shape | a data shape | struct ↔ JSON ↔ OpenAPI ↔ migration ↔ TS diverge | gap (heavier) |
| interface-doc | a declared flag/route | code says `--bar`, docs say `--foo` | gap |
| dependency/version | a pinned version | same dep pinned differently across manifests | gap |
| narrative | a world-fact / state | see §17.4 | gap (rich; partly reuses §12) |

Suggested sequence after code+prose: **config-env** (already scoped, deterministic)
→ **pattern-pattern/catalog** (smallest delta) → **value/constant** (cheap win).
Schema/doc/version are real but heavier and lower immediate demand.

### 17.3 The meta-ladder — pattern, pattern-pattern, pattern-pattern-pattern

The axes stack by level of abstraction:

- **L0 — instance.** A single function, paragraph, config value. The raw material.
- **L1 — pattern.** "One contract expressed in N sites that drift." Code twins,
  vocab compounds, config keys. *calque's current floor.*
- **L2 — pattern-pattern (the catalog/lexicon level).** The units are *named
  patterns themselves*; drift is two catalog atoms that are the same underlying
  move under different names. Substrates: a pattern lexicon, a project's engine
  catalog, any glossary. Mechanically this is prose-synonym promoted one level —
  cluster *entries* (name+description) by embedding instead of *words*.
- **L3 — pattern-pattern-pattern (cross-project).** The units are L2 shapes that
  recur *across the portfolio*. Independent projects grow the same architectural
  pattern-patterns — a recall→registry→gate→calibrate loop, a seed/contract plugin
  point, git-tag versioning, a catalog→render build step, an MCP stdio framing.
  Drift at L3 = the same shape reimplemented divergently in N repos. **calque is
  itself an L3 drift-fix:** the §16 convergence (two projects grew the same drift
  loop → consolidate into one substrate-general engine) was exactly an L3
  deduplication. An L3 recall extractor would read *across repos* for recurring
  architectural shapes and flag where they've diverged (e.g. three projects'
  versioning schemes that should be identical but aren't). This is where projects
  overlap and is the most leveraged — and least built — level.

  **L3 already has a named substrate:
  [hybrid](https://github.com/justinstimatze/hybrid).** hybrid is the framework/
  vocabulary for recurring LLM-and-code shapes (RAG, ReAct, knowledge-base auditor,
  the canonical 5-role lens→substrate→gate→reasoner→action loop); its shape catalog
  *is* the L3 registry, and calque / defn are cited instances in it (calque is the
  "knowledge-base auditor" shape). So L3 is not hypothetical — the registry exists
  by hand; the gap is the *automated* cross-repo recall extractor that flags where
  two instances of one hybrid shape have drifted apart.

The ladder is the same invariant at each rung; only the canonical unit changes
(instance → contract → named pattern → architectural shape).

### 17.4 Narrative as a substrate — drift beyond code/vocab/config

A sprawling high-agency narrative (characters, plot, player choice) has drift axes
of its own. Several map directly onto machinery calque already has:

- **Continuity / canon-fact drift.** A character's established fact (backstory,
  what they know, relationship state) asserted in multiple scenes that contradict.
  The prose axis lifted from *word* to *entity-fact*: a character bible vs the
  actual prose.
- **State-vs-narrative drift (the high-agency special).** The game *state*
  (flags/variables) vs the *narrative text* shown. Branch explosion lets a path
  describe a world inconsistent with the state machine (text greets a guard the
  state says you killed). The purest instance of the invariant for games — one
  world, two representations (engine state + prose) drifting — and unique to
  interactive fiction.
- **Voice / register drift per character** — prose-synonym scoped per-speaker.
- **Rule / affordance drift** — a game rule stated/implemented across code + config
  + tutorial text that diverge (the config-parity axis applied to game rules).
- **Dangling setup/payoff (Chekhov drift).** A foreshadowed event that never
  resolves, or a payoff with no setup — a *left with no right*: calque's
  **missing-twins** recall (§12) pointed at narrative.
- **Orphan / unreachable scene.** Dead narrative content is dead code — the
  **reachability/coverage-gap gate** (§12).

The genuinely-new narrative extractors are the first two (canon-fact and
state-vs-narrative); the rest reuse existing calque signals with a narrative
extractor in front.

### 17.5 Scaling the registry

The human-readable `.calque/registry.md` is right for dogfood-scale repos but will
not scale to large projects — appending and re-parsing a growing markdown file is
O(n) per check and gets unwieldy to hand-edit. Roadmap: move the *storage* to a
structured/indexed store (SQLite) while keeping a human-readable projection for
review and git history. Not urgent — defer until a real consumer hits the wall;
the markdown stays the source of truth until then.

---

## 18. The role-cardinality axis — declare-and-gate, not discover-similar

This is the **next thing to build** (roadmap §9, P0 item 0). It came directly out
of dogfooding: in one 2026-06-07 session a private sibling engine found two real
dual-path bugs by hand, and *both* defeated calque's existing mechanism in the same
way — which told us, by two independent instances, what the next primitive has to
be.

### 18.1 The two instances

1. **Stub short-circuit (green-tests-unplayable).** A `model="stub"` branch
   short-circuited the LLM call site and returned a plausible-but-fake value before
   reaching the real chokepoint. Tests went green; the game was unplayable. There
   is **no static pair of similar functions** here — it is one call site with a
   runtime mode flag swapping a fake. Pairwise touchpoint scoring has nothing to
   compare. calque could not have found it.
2. **Two record/replay backends.** The test layer grew *two* independent mechanisms
   for replaying one constructor's LLM calls — a bespoke content-addressed cassette
   and an off-the-shelf record/replay wrapper — both gating the same test-mode
   check. This *is* the canonical meta-bug (one canonical thing → N expressions), but
   it lived in **test-infra × test-infra**, a boundary nobody thinks to point a scan
   at, and the two backends use **different APIs**, so even a repo-wide pairwise pass
   would dilute rather than rank them.

These two happened to be N=2, but the general shape is **N≥2** — the multi-path
problem. calque has already seen N=3 (the triple-shell case, §15): one canonical
input path re-expressed across three shells. The primitive must count, not pair.

### 18.2 Why pairwise can't catch either

calque today does *similarity recall at a chosen boundary*: "which cross-boundary
pairs (or N-ary clusters) look like the same contract?" That requires two things
both instances violate:

- **You must pick the boundary.** Bug 2 hid in a boundary you would never name. You
  cannot pick the boundary you didn't know existed (§10, whole-scan open question).
- **The twins must share indexable footprint.** Bug 1's stub shares almost nothing
  with the real path; Bug 2's two backends use different APIs. Similarity is the
  wrong axis when the whole point is that the redundant implementations look
  nothing alike.

### 18.3 The primitive both need

> **Role-cardinality:** declare role *R*'s expected number of implementations
> (the collapse target is usually **one**), and flag whenever the *actual* count is
> **two or more**. The violation is N≥2, and N is unbounded — the multi-path case
> (3, 4, … redundant implementations) is the general form, not an edge case. The
> triple-shell case (§15; 3 shells → 1 canon) was already an N=3 instance.
> Gate every implementation beyond the declared count.

This inverts the mechanism: from *discover-similar-after-the-fact* to
*declare-the-expected-count-and-enforce-it*. Its properties are exactly the ones
the bugs demand:

- **No boundary to pick** — the invariant is repo-wide for the named role.
- **No similarity required** — it counts implementations of a role, not their
  textual/footprint overlap. The stub vs real, and the N backends, all *count*
  even though none *resemble*.
- **N-ary by nature** — "how many implement role R" is intrinsically a set
  question, not a pairwise one, so it consumes the existing N-ary cluster pass
  (§15) directly: a cluster of N members claiming one role *is* the violation.
- **Catches recurrence** — the sibling's own sharpest point: "a second layer
  reappeared because nothing forbids one." A cardinality gate is the only form that
  stops the next implementation from reappearing; a pairwise scan re-finds it each
  time at best.
- **Cures rule-forgetting** — the sibling kept losing the "only one of these"
  convention to context resets. A declared cardinality is *code*, enforced at the
  gate, not a memory.

### 18.4 Relationship to what already exists

- It is the **`check` gate generalized from suspects to roles.** Today the registry
  records a backward verdict on a *discovered* pair (`drift` / `contracted-twin-ok`
  / `false-alarm`). Role-cardinality is the registry as a *forward declaration*:
  "role R = 1 impl" (or a stated N for the rare legitimate multi-path), enforced
  before a violating implementation exists.
- It composes with the **N-ary cluster pass** (§15) as a *candidate proposer*:
  clustering can surface "these N functions all smell like the replay backend — is
  that intended?", and the cardinality declaration answers "expected one." Because
  the cluster pass is already N-ary (it found the triple-shell cluster), it natively handles
  the multi-path case; cardinality just adds the declared expected count and the
  gate.
- It is a cousin of the **antibody generator** (§9, P0 item 3) but earlier in the
  lifecycle: the antibody emits a test *after* adjudication; cardinality is a
  standing invariant declared *up front*.

### 18.5 The honest limit

Cardinality is not free magic: *something* must declare role *R* and what counts as
"an implementation" of it. Two honest options:

1. **Explicit declaration** (low-magic, reliable): the registry/a config gains
   `role: constructor-replay → impls: 1`, with the impl sites named or matched by a
   predicate. The author states the invariant once; calque enforces it. §18.7
   documents a *production reference implementation* of this form — an AST-predicate
   role registry the sibling shipped as a standing test — which establishes
   the right shape: the declaration is a **predicate** (calque discovers the
   members), and the gate is **set-membership against a baseline that ratchets toward
   the target count**, not a bare `count == 1`.
2. **Cluster-proposed** (higher-magic): the existing clustering proposes role
   candidates and their current cardinality; the author confirms the expected count.
   **SHIPPED — `calque propose-roles` (task #15).** It feeds each N-ary private-seam
   cluster through a predicate synthesizer (the cluster's strongest shared seam →
   `calls:`/`emits:` term) and **self-verifies** the synthesized predicate by running
   it back through the matcher (§18 option-1 machinery), emitting paste-ready `- role:`
   blocks with `expected: 1` + a frozen `- baseline:` and a verify line showing whether
   the predicate re-selects the cluster exactly. It is a *generator, not a gate* — prints
   to stdout, writes nothing, never exits non-zero — and dedups against already-declared
   roles and adjudicated `- cluster:` verdicts so settled ground isn't re-proposed. This
   removes the option-1 authoring friction (you no longer have to know the seam to write
   the predicate); the human still adjudicates which candidates are real.
   *Prior art / honest scoping:* the discover-then-propose-a-declaration loop itself is
   **not** novel — `sauremilk/drift` already pairs undeclared-dup discovery with declared
   boundaries (§6), and DejaVu/FICS are the academic "recall-then-classify" precedent
   (`RESEARCH_AND_MARKET.md` §1). What is unoccupied is the *cardinality* gate the
   proposal targets — declaring a role's expected implementation count and enforcing
   singularity — not the act of proposing a declaration from a cluster.

Neither auto-finds the bugs with zero input — but both convert a convention the
author keeps forgetting into a code-enforced invariant, which is the whole win. The
first is the MVP; the second is the recall sugar on top.

### 18.6 Why this outranks the prior P0

The previous lead P0 (pluggable signal *profiles*, §13/§9) is *more pairwise
signal* — better recall on Type-4 *similar* pairs. Role-cardinality is orthogonal
and, on the evidence, more valuable: the two real bugs that actually shipped were
**not** caught by any amount of pairwise signal, because they were not similarity
problems. Profiles remain real roadmap; they are simply re-ranked behind the
primitive that two verified instances asked for by name.

> Note on bias, for the record: an earlier read of these instances mapped them onto
> the env/config-parity axis (§14). That was fitting the evidence to a pre-existing
> pet axis. The two *verified* bugs point at cardinality, not parity — parity is
> "same value, N places, drifted"; cardinality is "this role should have one
> implementation, and it has two or more." Different invariant; cardinality is the
> one the instances support.

### 18.7 Field evidence: a sibling's collapse campaign (2026-06-08)

A day after the two-instance derivation above, a private sibling engine ran a
deliberate, multi-front dual-path **collapse campaign**. It is the most direct field
evidence we have of the exact problem class calque exists to correct — and, read
honestly, it shows both where calque already earned its keep and where its current
scanner is structurally blind.

**What calque's scanner caught (harness × engine, 2026-06-05 seed run).** A real scan
over an `engine*.py × testing.py` boundary adjudicated 30 suspects into **4 genuine
drifts, 21 delegating adapters, 5 false alarms** — and one of the four was a *shipped
prod bug*: an inline eligibility gate omitted a suppression the harness path applied,
so production could re-fire a one-shot event inside its own cooldown window, and no
test caught it. That is calque doing exactly what it claims — a behavioral twin that
diverged where no test looked. All four were collapsed to single-path with regression
tests, recorded in the sibling's `.calque/registry.md`.

**What calque's scanner missed (a later input-path campaign).** The bigger, later
campaign — collapse an input dual path so a single constructor is the one path,
collapse two record/replay backends to one, and rip a no-LLM test driver so it runs
the real production path — calque **did not catch.** The sibling's own registry says
why (a "boundary expansion" note): the seed run scanned only `engine × testing`, and
these duplication classes *live outside that boundary* (an input-parser cluster;
test-infra × test-infra). Nobody pointed calque at those boundaries, so it never
fired. The sibling found them through broken integration runs and reasoning, then
**built its own in-repo antibodies** to hold the line.

**Why this is direct evidence of our problem class, not an indictment.** Both halves
confirm the thesis. The caught half is a clean positive: one canonical thing, two
expressions, silent divergence, real bug. The missed half proves the class is
*larger than the current scanner reaches* — the failures were boundary selection and
recurrence-after-rename (a delegated agent re-created a banned path under a new name
and kept the disallowed mode flag), which is precisely what the §10 whole-scan gap
and the §18 cardinality primitive are for. The sibling **independently re-derived
role-cardinality** and named the primitive in its own notes: *"assert exactly one
constructor replay backend."* Two projects converging on the same primitive from
opposite directions is the strongest signal we have that it is the right next build.

**What to adopt — production-validated mechanisms.**

1. **A role is an AST predicate; gate the discovered set** (sharpens §18.5 option 1
   and the §10 whole-scan). The sibling shipped a role-registry test that enumerates
   *every* implementer of a role by AST predicate (every raw-input parser, every
   caller of the canonical resolver, every shell driver) and fails if a match is not
   registered + classified. A *new* implementer turns the build red until its author
   registers it. This makes the explicit-declaration MVP concrete: the declaration is
   **a predicate, not a hand-listed set of sites**, so calque discovers the role's
   members itself and gates set-membership — which is also how the boundary-blind scan
   (§10, task #16) should be *driven*: by a role predicate, not a left/right glob.

2. **Frozen-baseline ratchet as a progress ledger** (sharpens §18.3 enforcement). The
   sibling's single-path antibody test asserts **exact equality to a frozen baseline
   set**, not `count == 1`. A new member turns the build red; so does deleting a known
   offender without shrinking the baseline. This is the form that *survives a
   mid-flight N→1 collapse*: while you still have N implementations you cannot assert
   `== 1`, but you can assert `== {the known N}, and the baseline only ever shrinks
   toward the target`. The gate doubles as a visible collapse-progress ledger (done
   when the baseline reaches the declared count). Adopt this as the cardinality gate's
   enforcement model — **`set == declared-baseline, monotone toward the target
   count`**, not a bare integer compare.

3. **Non-vacuity by mutation** (new `doctor` primitive). The sibling mutation-checks
   its detector: with an empty allow-list the guard must flag the canonical
   implementation itself, proving it isn't vacuously green. calque's `doctor` should
   mutation-verify any cardinality/role detector the same way before trusting a clean
   run.

4. **Collapse-direction in the registry schema** (new schema field). The sibling's
   drift entries carry more than a `drift` verdict: a policy of `collapse-to-single
   (NOT differential — the legacy side is slated for deletion, do NOT re-sync)` plus
   which path is canonical. Without those fields a later agent "fixes" a drift by
   re-syncing the doomed path — i.e. *maintaining the dual path*, the exact anti-goal.
   Make **canonical-path** and **do-not-resync** first-class registry fields, not
   prose in a note. **SHIPPED (task #18):** a `drift` `- pair:` now carries optional
   `- canonical:` / `- do-not-resync:` lines, and `check` surfaces every drift pair
   whose both paths are still live as `DRIFT (unresolved)` with that direction (or a
   prompt to record it) — warn-only, never gating `--strict`, so an in-progress
   collapse doesn't fail the gate. A collapsed drift (one side gone) falls through to
   the existing STALE pass to be pruned.

5. **The delegation gate is field-validated, not new work.** 21 of the 30 seed
   suspects were *adapters that delegate to the real implementation* and "can't
   behaviorally drift — the name-stem signal fires because the wrapper is named after
   what it wraps." That is exactly the precision failure calque's **delegation gate
   (§12)** already suppresses. The campaign is independent confirmation that the gate
   targets a real, dominant false-positive class; treat 21/30 as the field measurement
   of why it matters.

Adoptions 1–2 sharpen tasks #14/#16; 3–4 are cheap, high-value `doctor`/schema
additions; 5 confirms an existing mechanism. None require abandoning the pairwise
engine — they extend the *declare-and-gate* half the field evidence keeps demanding.

## 19. The cross-substrate axis — non-function entities (v0.3.0)

Every axis above the function level shares a structural blind spot: the code axis
extracts **functions** and scores their effect footprints, but a content-driven
codebase carries most of its "one concept in N places" drift in *non-function*
entities — module-level lookup tables, enum/registry constants, and the authored
corpus (`*.json`) those tables mirror. Two such entities that must agree share no
surface tokens a jaccard gate can anchor on (a verb registry in one module vs the
verb-template table in another; a corpus shape vs the schema/table that ingests it),
so the function axis is *structurally* blind to them — not low-recall, blind.

### 19.1 The mechanism — reuse the engine, change only the extractor

The spine is unchanged. A non-function entity is represented as a `FuncSig` with a
`Kind` tag (`table` | `corpus-field`) and its **key set in `RetKeys`** — which
already means "the key set this entity defines," so `Prepare`/`jaccard` need no
change. Two extractors feed it: the embedded Python extractor gains a `symbols` mode
(module-level `dict`/`set`/`list` constants → `table`), and a pure-Go JSON extractor
turns each corpus object's field-name set into a `corpus-field` entity (carrying a
bounded `Source` blob for the oracle, since a JSON object has no clean line span).
`KeySetCandidates` pairs entities by shared key set (an inverted key index; a fanout
cap drops ubiquitous keys as JOIN paths only, never from the jaccard itself), and the
existing LLM oracle adjudicates. The pairing the function axis can't make is now a
cheap key-set overlap + a precision judge.

### 19.2 Isolation — the axis cannot disturb the gate

Non-function entities never enter `Extract` (which feeds `Rank` → `check` →
`--strict`); they have a separate `ExtractSymbols` path consumed only by
`propose-cross`, which — like `propose-roles`/`propose-deep` — is a **generator, not
a gate**: stdout only, no registry writes, no exit code. So the cross-substrate axis
adds zero risk to the self-clean gate, and the corpus the scorer sees is byte-for-byte
what it saw before. (One dedup detail: identical-shape corpus objects collapse to one
representative, or a column key shared across 50 location files would blow past the
fanout cap and prune the one cross-substrate pair that matters.)

### 19.3 What the field run showed — and the honest boundaries

Read-only on a private content-driven codebase: **1346 entities (543 tables + 803
corpus shapes) → 261 candidates, 188 of them cross-substrate.** A 14-candidate judge
slice returned 4 drift + 1 contracted-twin-ok + 9 false-alarm — the oracle confirmed
two copy-paste constant tables and two separately-maintained canonical-fact registries
as drift (one had **already diverged on a single entry** — a live latent bug invisible
to any function-axis or AST-shape tool), flagged a **corpus-JSON ↔ code-table** pair as
a contracted twin needing a differential pin (the headline cross-substrate case), and
correctly rejected coincidental same-key tables (verb-dispatch vs verb-config,
beat→arc vs beat→budget) as false alarms. Precision rests on the judge; the key-set
pass is deliberately high-recall.

Three boundaries, recorded honestly (§13 discipline — don't claim what didn't surface):

1. **SQL-schema side — ADDRESSED (`extract_sql.go`).** A corpus field-set whose code
   mirror is a *SQL* table (a `CREATE TABLE` in a `.sql` file or embedded as a string
   in source) used to surface only on the corpus side. A tolerant pure-Go
   `CREATE TABLE` scanner now extracts each table's column set as a `sql-table` entity,
   so the corpus ↔ db pair joins: the `temporal_markers` corpus shape ↔ the `db.py`
   schema now surfaces (jaccard ~0.62 — the schema carries extra operational columns
   the authored corpus doesn't). Remaining nuance: the scanner is regex/paren-based,
   not a SQL engine — good for column *sets*, not for type/constraint fidelity.
2. **Different key spaces stay invisible to key-set pairing.** A `{char_id: descriptor}`
   table and a corpus record with a `descriptor` *field* are the same concept but share
   no keys (ids vs field names), so key-set overlap can't pair them. This needs a
   value-shape or semantic signal (judge-only), and is deferred — measured, not forced.
3. **Mechanism is substrate-general; the extractor is per-substrate — now Python, Go,
   AND TypeScript.** Module-level tables are extracted for Python (`symbols` mode), Go
   (`extract_go.go`: package-level `var X = map[…]{…}` / `[]string{…}` via `go/ast`),
   and TypeScript/TSX (`extract_ts.mjs` `symbols` mode: module-level `const/let/var X =
   {…}` / `[…]` via the TS compiler API); the same entity shape applies to any language
   with literal tables. So the axis is not Python-bound — a Go or TS codebase's
   registries pair against corpus shapes and SQL schemas the same way, and tables pair
   *across* languages (a TS `HANDLERS` ↔ a Python `_VERB_TEMPLATES` share a key set
   regardless of source language).

### 19.4 Second validation — a different language (TypeScript), real drifts

The first field run (§19.3) was a Python codebase. A second, independent run on an
unseen **TypeScript** repo (~300 source files) makes the validation plural across
languages — and exercises the TS table extractor at scale: **127 entities (74 module-
level TS tables + 31 corpus shapes) → 44 candidates.** A 14-candidate judge slice
returned **2 drift + 11 false-alarm + 1 judge error**. Both drifts were hand-verified
(positive-control discipline) and both are function-axis-invisible:

- A **duplicated SQL schema** — one logical table's `CREATE TABLE` written verbatim in
  two modules (a setup helper and a migration); identical today, free to diverge the
  moment one gains a column. Surfaced by the SQL extractor + cross-file key-set pairing.
- A **duplicated config literal** — the same typed baseline constant defined in a test
  file and a build-config file; re-tune one and the test asserts a stale target.
  Surfaced by the **TS table extractor** (a module-level `const X = {…}`, invisible to
  the function axis). This is the entity the TS leg was built to see.

The judge again held high precision on the false-alarm class — it rejected coincidental
key-shape matches (an agent-type enum used as a list vs as a map's keys; distinct
threshold presets that intentionally hold different values; differently-sized test
fixtures) rather than crying drift on shared shape alone.

One honest wrinkle: **1 of 14 judge calls returned malformed verdict JSON** — the model
emitted a short reasoning preamble and *then* a corrected JSON object, and the parser
took the first object and choked on the trailing prose. The verdict was lost (counted as
errored, not silently mis-classified). This is concrete motivation for the fenced-oracle
hardening (§13 / the stull select-don't-invent pattern): constrain the judge to *select*
a vetted verdict token under a grammar that fails safe, so a chatty completion can't
corrupt the parse.
