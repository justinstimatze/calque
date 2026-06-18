# calque

**calque finds multi-path drift — one contract reimplemented in N places that
silently diverged — so you can collapse the copies back to a single source.**
Recall first; you (or an LLM) adjudicate; a registry remembers.

> Status — early, in progress. Today it's a multi-signal *code* nose (effect-footprint
> pairs, N-ary seam clusters, value-derivation reads, and drift-confessing comments —
> validated on real Python, Go, and Rust codebases) plus a prose vocab-drift checker,
> on a shared registry → check → calibrate spine. The broader multi-axis vision in
> `docs/DESIGN_NOTES.md` §16–17 is roadmap, not built yet. This is shared as a work in
> progress, not a finished tool.

A *calque* is a structural copy carried across surfaces (English "skyscraper" →
French "gratte-ciel": same structure, different words). This tool hunts the
software version of that: a single contract, concept, or value expressed in
several places that were supposed to stay in step and didn't.

These copies are rarely intentional — they're **quiet cruft**: an LLM (or a
hurried human) reimplements behavior without consulting the path that already
exists, so two now-divergent implementations of one contract drift apart in
silence. calque's main value is to **surface those multi-paths so you can collapse
them to a single source** wherever possible — extract the shared logic, make one
delegate to the other; one definition can't drift from itself. Occasionally a
second path is genuinely unavoidable, and then you keep the two in lockstep (a
differential test pins them) — but that's the fallback, not the goal. The registry
records each verdict so the gate can stop *new* copies from creeping in.

## Why existing tools miss the code axis

Dual paths are **Type-4 (behavioral) clones** — the hardest of the four clone
types. Where Types 1–3 are progressively looser *textual* copies (identical →
renamed → edited-with-gaps), a Type-4 clone is alike only in *what it does*, not
how it reads. It's dissimilar in syntax *by construction*, which is exactly why
the usual tools whiff:

- **grep / ctags / LSP** index lexical identity and the call graph — but the
  twins *don't call each other*, so there's no edge to follow.
- **embeddings / "semantic" search** embed *how code is written* — the twins are
  written nothing alike, so they embed far apart.
- **clone detectors** (`dupl`, jscpd, …) look for *similar* code — the opposite
  of the problem.

They all index **representation**. Dual-path drift is a **role collision in
behavior-space**, and representation is the very dimension along which the twins
diverged. You have to index the contract, not the prose. calque extracts the
signals that stay invariant when a body is rewritten and ranks cross-boundary
pairs by overlap — plus an N-ary cluster pass that catches a shared private seam
inlined across several differently-named functions (the case pairwise scoring
structurally dilutes).

## The invariant

Every drift it catches is the same shape:

> **one canonical thing → N expressions → independent drift.**

A test harness reimplements an engine method and forgets a step. A doc and the
code it describes disagree after a refactor. Two prose passages name the same
concept with drifting compounds (`single-source` vs `single-sourced` vs
`single sourced`). The canonical thing is singular; the expressions multiply;
nothing keeps them honest. calque is the thing that keeps them honest.

## The spine

Each *axis* plugs one recall extractor into a shared pipeline:

```
recall (cheap, high-recall scan)  →  registry (adjudicated memory)  →  check (gate)  →  calibrate
```

- **recall** extracts signals and ranks suspects — tuned for recall, not precision.
- **registry** (`.calque/registry.md`) is the durable verdict memory, so neither
  you nor an agent re-litigates a cleared suspect across context resets.
- **check** diffs a fresh scan against the registry and surfaces only what's
  *new* — the hookable gate.
- **calibrate** (`doctor`) rolls up whether the ranker actually discriminates
  real drift from false alarms.

Only the recall extractor is axis-specific. Two axes ship today:

| axis | canonical unit | what it indexes |
|------|----------------|-----------------|
| **code** | a contract / value two paths implement | behavior-invariant signals (emitted strings, state writes, returned keys, callees, name-stem, **input field-sets read**) + N-ary private-seam clusters (call/string/write/domain-constant) + drift-confessing comments |
| **prose** | a term/compound | hyphenated-compound frequency vs an allow-list; embedding near-synonyms |

## Install

```bash
go install github.com/justinstimatze/calque/cmd/calque@latest
# or, from a clone (bakes the version from the git tag):
make install
```

The version string comes from the git tag (`git describe`), not a hand-edited
constant — `calque version` self-describes.

## Quickstart

**Code axis** — scan a boundary, then gate against the registry:

```bash
calque scan  --left "engine*.py" --right "testing.py"   # rank suspects
calque check --left "engine*.py" --right "testing.py"   # only what's new vs the registry
calque check --strict ...                               # exit 1 on new suspects (for hooks)
```

**Prose axis** — flag drifting compounds against an allow-list:

```bash
calque vocab-report                       # frequency surface (recall)
calque vocab-check                        # gate: compounds not in .calque/vocab-allowlist.txt
calque vocab-check --bootstrap            # seed the allow-list from the current tail
calque vocab-check --seed-cmd '<proj seeder>'   # merge a project's own slug list
```

**Standing audit (boundary-free)** — whole-repo generators that need no `--left/--right`:

```bash
calque propose-deriv --repo .          # value-derivation twins (same field-set, no shared authority)
calque confess       --repo .          # functions whose own comments confess a twin
calque propose-roles --repo .          # N-ary seam clusters → paste-ready cardinality roles
calque propose-deriv --repo . --judge  # add --judge to adjudicate with the LLM oracle (needs ANTHROPIC_API_KEY)
```

These are generators — they print to stdout, never write or gate — so they're safe
to run against any repo.

`scan`/`check` work on Go (native `go/ast`), Python (embedded `python3`),
TypeScript/TSX (embedded `node` + the TypeScript compiler), and Rust (an embedded
`syn` helper, built once and cached on the first `.rs` scan). Each language needs its
own toolchain present for that language's targets — `python3` for `.py`,
`node`+`typescript` for `.ts`, `cargo` for `.rs` (the same toolchain you already build
that code with; `CALQUE_RUST_EXTRACTOR` can point at a prebuilt binary to skip the
build).

## The loop

1. **Pick a boundary** — the two sides that should agree (`--left`/`--right`
   globs). A well-chosen boundary beats threshold-tuning every time.
2. **Scan / check** — get a ranked suspect list, each with its firing signal.
3. **Adjudicate** each suspect and record the verdict in `.calque/registry.md`:
   - **`drift`** — same contract, behavior diverged. The bug. Fix by collapsing
     to a single path (extract the shared logic) — far better than keeping both
     and bolting on a test.
   - **`contracted-twin-ok`** — intentionally parallel and currently in sync;
     record it so it's not re-flagged (and ideally pin it with a differential
     test).
   - **`false-alarm`** — coincidental signal; record to suppress.
4. **Keep it honest** — `calque hook install` wires `check` into a git
   pre-commit hook; `calque mcp` serves both gates over MCP (stdio JSON-RPC,
   tools `calque_check` + `calque_vocab_check`) so an agent can ask "did my edit
   introduce new drift?" inline.

calque is built to be driven by a coding agent as the equivalence oracle — see
`SKILL.md` for the full agent-facing loop.

## What it's good at — and how to read the output

calque is a **recall-first nose, not a prover.** It surfaces *candidates* that
smell like the same contract — via cheap effect-footprint heuristics (shared
emitted strings, mutated field-paths, returned keys, callees, name-role, and shared
rare private seams) — and hands them to you to judge. It does **not** prove two
functions equivalent; that's undecidable. Read the output with that in mind:

- **Expect false positives — that's the design.** The job is *recall*: a short
  ranked list you (or an LLM) adjudicate into the registry as `drift` /
  `contracted-twin-ok` / `false-alarm`. A run that's ~30% real but misses almost
  nothing beats a "sound" analyzer that finds nothing actionable.
- **Know the recurring false alarms.** A few patterns are noise, not drift:
  two *test* functions sharing a setup/mock fixture (calque **gates test↔test by
  default** and keeps test↔prod; `--include-tests` to override); DTO/projection
  mappers that copy the same field set between structs; functions over one numeric
  struct doing *different* arithmetic; two methods on the same type sharing that
  type's fields. `scan`/`check` flag the last two inline (`· structural:
  same-receiver` / `field-copy`) — advisory, never gated. Trust shared **emitted
  strings / state writes / domain callees** (effect-footprint) over raw field-set
  or name overlap — `SKILL.md` has the full adjudication guide.
- **It's richest on effectful / stateful / text-emitting code** — game engines,
  CLIs, services, agent tooling: functions that mutate state, emit strings, and
  return records. On pure-functional, value-returning libraries the per-pair
  effect-signals thin out, and the N-ary **cluster pass** (functions sharing a rare
  private symbol — convention-free and language-agnostic) carries more of the load.
- **Validated on real Python, Go, and Rust codebases** (and calque's own source),
  where it has surfaced concrete, shipped-bug-class drift — on a real Rust codebase its
  first run surfaced three genuine dual-path collapses and correctly pinned a deliberate
  twin whose drift-guard had no backing test. TypeScript/TSX extraction also ships.
- **Tune the boundary, not the threshold.** A well-chosen `--left/--right` (two
  things that genuinely should agree — harness vs prod, client vs server, v2 vs v1)
  beats scanning a whole repo against itself every time.

`docs/DESIGN_NOTES.md` §13 has the honest breakdown of what generalizes (the
convention-free engine) versus what's still style-tuned (the pairwise
effect-weighting).

## Migrating an old registry

A registry written by the original Python calque (`- left:`/`- right:` blocks)
parses to zero entries under the Go parser, so `check` would flag the whole repo
as new (it warns when it detects this). Convert it once:

```bash
calque migrate-registry --in .calque/registry.md --write   # .bak backup first
```

## Status

The spine is complete and dogfooded (calque scans its own source and stays
clean) — code + prose axes, registry gate, git/MCP hooks, calibration. The code
axis now carries several recall extractors beyond the original pairwise
effect-footprint scorer:

- **value-derivation (`reads`)** — the same quantity derived independently in ≥2
  places that silently diverge ("fix one path, the twin still has the bug"), keyed
  on the input field-set both sides read. The boundary-free `propose-deriv` is the
  standing-audit surface.
- **confession (comments)** — `calque confess` surfaces a function's own
  self-witnessing comment ("mirrors X", "keep in sync with Y") that names another
  function — the twin a maintainer already flagged but never pinned with a test.
- **N-ary touchpoint clusters** — functions sharing a rare private seam (a call,
  string, write, or domain constant) that pairwise scoring dilutes; `propose-roles`
  turns a cluster into a paste-ready cardinality role.
- **role-cardinality** (`calque cardinality`) — declare "this role should have one
  implementation; flag whenever it has two or more" and gate it; counting needs no
  resemblance (`docs/DESIGN_NOTES.md` §18).

A `--judge` flag on the generators (`propose-deriv`/`confess`/`propose-roles`/
`propose-deep`/`propose-cross`) runs an LLM equivalence oracle as the precision half,
and `calque doctor --ablate` rolls every judged verdict into a per-detector × language
× variety matrix so each detector has to earn its keep. Further axes (config/env,
catalog, narrative) are roadmapped in `docs/DESIGN_NOTES.md` §16–18. Apache-2.0; the
prose axis, calibration, and
hook are consolidated from the sibling project `cupel` (MIT, attribution
preserved) — which now consumes calque back: cupel retired its own vocabulary
tooling and runs `calque vocab-check` as its pre-commit prose gate.
