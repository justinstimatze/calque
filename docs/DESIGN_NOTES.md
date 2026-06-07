# calque — design notes

> Architecture, rationale, and roadmap for calque, a substrate-general drift
> nose (Go). Companion docs: `PATTERN_CATALOG.md` (the concrete drift shapes),
> `RESEARCH_AND_MARKET.md` (prior art + competitive landscape).

---

## 1. What calque is (one paragraph)

calque finds **dual-path code**: two implementations of one contract that are
*supposed* to behave identically but have silently drifted — a test harness that
reimplements production logic, a client that hardcodes a verb list the server
also owns, a `v2` path that diverged from `v1`. These are **Type-4 (behavioral)
clones**: dissimilar in syntax *by construction*, which is exactly why grep, LSP,
embeddings, and clone-detectors all miss them. calque is the **high-recall GATE**
(the *recall* stage — "RECALL" describes the tuning, not a separate role) of a
hybrid heuristic → LLM-oracle → registry loop. It indexes the signals that stay
invariant when a body is rewritten, ranks suspect pairs, and hands a short list
to an LLM/agent that makes the actual equivalence call. It never *proves*
equivalence (that's undecidable) — it's a nose, not a judge.

The name: a *calque* is a structural copy carried across languages ("skyscraper"
→ French "gratte-ciel"). The software version is two code paths sharing a contract
but diverging in surface.

---

## 2. The core insight (why this is hard, why indices miss it)

The undecidability is real but irrelevant to the need. You don't want a *prover*;
you want **clues to automatically go check** — a recall-heavy generator that
routes an LLM oracle's attention to what's "too close to equivalent." Codebase
indices don't capture this. The hard part (judgment) moves to the part that's
good at judgment; the tool only has to be *suspicious*, never *right*. A
30%-precision / 95%-recall ranker that yields ~15 pairs to look at beats any
sound analyzer here.

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

**The divergence-robust signals calque uses** (`internal/code`):
- emitted string literals — what the function *says* (surface output)
- attribute write-targets — what it *mutates* (effect signature)
- returned dict keys — the shape of what it *hands back*
- callee names — what downstream it leans on
- name-stem (role tokens, role-prefixes like `handle_`/`resolve_` stripped) — the
  *role*; names track role even when bodies don't

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
                   #   python3 extractor) + jaccard scoring + N-ary touchpoint
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
targets parse via native `go/ast`; Python targets via an embedded `python3` AST
extractor (one subprocess per scan).

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

---

## 6. Prior art / research

> Fully developed in **`RESEARCH_AND_MARKET.md`** (primary-sourced): calque's true
> lineage is the *inconsistency-bug* line (Engler 2001 → CP-Miner → DejaVu → FICS
> 2021), not clone detection; a primary-source competitive scan (the niche is
> unoccupied; Greptile is diff-gated, Larridin is an exec score); verified market
> tailwind (DORA, the 304K-commit arXiv study, Stack-Overflow-2025's "66% almost
> right"); and the go/no-go verdict.

Highlights that shape the architecture:

- **The problem is named and measured.** LLMs disproportionately produce Type-4
  clones (they re-derive behavior instead of reusing it), and existing tools miss
  them (*More Code, Less Reuse*, arXiv 2601.21276; *Detecting Semantic Clones of
  Unseen Functionality*, arXiv 2510.04143).
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
- **The signal is the genuinely original move.** Static *effect/mutation*
  signatures (write-targets, emitted literals, returned keys) as a recall signal
  sit in an empty intersection: effect systems exist (for typing/optimization),
  behavioral-clone detection exists (but goes dynamic) — nobody uses cheap static
  effect footprints to *match twins*.

calque's unclaimed white space: **undeclared-contract drift-hunting in a live
repo, via static effect-signatures**, with a persistent adjudicated registry.

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
- Code axis (Go `go/ast` + embedded `python3`), parity-verified against the
  original Python implementation's scores.
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
7. **Go + TS extractors.** TS via ts-morph or tree-sitter; Go as a query layer
   over defn. Falls out of #1 once the producer interface exists.
8. **Boundary presets** (`--preset harness|client-server|v1-v2`) so users don't
   hand-craft globs.
9. **Cross-language dogfood.** Confirm the meta-bug + the private-symbol-touchpoint
   signal transfer to TypeScript and Go siblings (TS levers: discriminated unions,
   branded/opaque types, `as const` taxonomies; Go: pre-generics copy-paste,
   duplicated error handling, struct-tag drift). Goal is *generality*.

---

## 10. Open questions

- **Registry location.** Per-repo `<repo>/.calque/registry.md` (chosen) for
  portability + git-tracked memory, vs a central store. Confirm at scale (§17.5).
- **Cross-language pairs.** Real dual paths include JS-client ↔ Python-server. A
  single-language AST nose can't express those; the options are a shared
  spec/contract both extractors map onto, or a differential at the API boundary.
  Out of scope for the AST core; a known gap.
- **Within-repo whole-scan mode.** Currently boundary-only (left×right). A
  whole-repo O(n²) mode needs name-stem prefiltering to be tractable; worth it for
  "find dual paths I didn't know to look for."
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

## 13. The generalization challenge (honest assessment)

**A lot of calque's signal is tuned to one project's effectful-OOP conventions.**
Generalizing to *any* large project is the central open problem (roadmap P0).
Being honest about what's specific vs general is the prerequisite.

### What's project-shaped (the signals encode assumptions)
| signal | assumes | weak/absent on |
|---|---|---|
| **returned dict keys** | functions return `dict` results | objects/dataclasses/tuples/`None` |
| **attribute write-targets** | mutable OOP self-state; mutation = effect | functional / immutable / pure code |
| **emitted string literals** | functions emit user-facing text (game/IF/CLI) | libs where strings are log lines / keys |

And the default constants encode one project's naming/architecture (role-prefixes,
delegation roots, dispatchers, the `engine*×testing` boundary) — all now
flag-overridable, but the *defaults* are project-shaped.

### What's genuinely general (keep)
- The **hybrid loop**: recall → adjudicate → registry → verdict. The consensus
  2026 architecture (§6).
- The **ranker**: per-signal jaccard, weighted + renormalized, noise-gate,
  best-match dedup. Nothing in it is project-specific — it scores whatever signals
  it's handed.
- The **delegation down-weight**, **calibration**, **metabolism**, and the "a nose,
  not a judge" stance.
- Per-repo `.calque/registry.md` as portable, git-tracked substrate.

### The design path: pluggable signal *profiles*, not more flags
Because there is **no universal behavioral signal**, any detector must pick
*domain-appropriate invariants*. So: keep the ranker; make **extraction** a
pluggable `FuncSig`-producer profile (`effectful-oop` today; `functional`,
`api-contract`, `data-pipeline` next). Configurability (`--role-prefixes` etc.)
was step one — it removes *hardcoding* but not the *signal-shape* assumption.
Profiles are step two; the recall benchmark (P0 #2) is how you'd *know* a profile
generalizes rather than just believe it.

**Bottom line.** calque is a validated *instance* of a general *shape*. The shape
is SOTA-aligned; the instance is one profile. Becoming "2026 SOTA for this problem"
is precisely: factor the profile boundary, add 2–3 profiles, and prove recall on
GPTCloneBench/CETBench.

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
keeper**; the prose project becomes a *consumer*. License: the prose source is MIT,
calque Apache-2.0 — compatible; MIT attribution is preserved for the lifted
portions.

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
