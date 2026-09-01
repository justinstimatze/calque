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

**Be precise about what that pairwise scorer actually needs, though.** It's a
weighted overlap across five channels (emitted strings, write-targets, return
keys, callees, name-stem) — tolerant of heavy rewriting, but it still requires
*at least one* of those five to overlap before a pair is even considered a
candidate. Two functions sharing zero tokens across all five — no common name,
no common string, no common write, no common callee, no common return key — are
invisible to it, full stop. That's real Type 1–3 territory (renamed, restructured,
edited-with-gaps), not the textbook zero-footprint Type-4 case.

The one mechanism that needs **no shared token at all** is `propose-deep` — a
separate, opt-in generator (`calque propose-deep --repo .`, listed under
Quickstart's "Standing audit" below), not part of the default `scan`/`check`
loop (Quickstart's "Code axis," also below) and not something you need to
reach for to get value from calque.
It groups functions by a *rare, domain-typed signature* — two functions sharing
`(UserRecord)=>ValidationResult` and nothing else pair up regardless of how
differently their bodies are written. That's genuinely representation-independent,
but it depends on the language exposing static types calque can read, which
covers all five languages today (Go/Python/Rust included). And even
`propose-deep` needs a **distinctive** signature to anchor on — a generic one
(`string→bool`) is too common to mean anything. Two independently-written twins
sharing neither tokens nor a distinctive signature sit outside every axis
calque has today; see `docs/DESIGN_NOTES.md` §22 for the full breakdown.

`propose-context` covers part of that remaining gap for Go: instead of a type
signature, it anchors on *call-site context* — two functions with zero shared
tokens still tend to be called from similarly-named driver functions and
return similarly-shaped results (both null-checked, both error-checked, …).
Both signals have to agree (either alone is too noisy). A pair where one side
directly calls the other is dropped too: the call graph already accounts for
it, so it reads as a pipeline stage, outside the zero-shared-token twin case
this axis targets. See `docs/SPEC-callsite-context-axis.md`.

`calque scan` also *runs* `jscpd`/`dupl` as a belt-and-suspenders companion pass
if either is already on `$PATH`, so one invocation surfaces both axes instead of
requiring two separate tools in your pipeline. Best-effort and purely additive: a
tool absent from `$PATH` is skipped with an install hint (never fetched), and
neither tool's output touches calque's own scoring or registry. `--no-companions`
to skip it.

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
calque propose-deriv   --repo .        # value-derivation twins (same field-set, no shared authority)
calque confess         --repo .        # functions whose own comments confess a twin
calque propose-roles   --repo .        # N-ary seam clusters → paste-ready cardinality roles
calque propose-deep    --repo .        # Type-4 twins sharing a rare type signature, no shared tokens
calque propose-context --repo .        # call-site context axis: no shared tokens AND no distinctive signature (Go-only)
calque propose-cross   --repo .        # non-function entities (tables, schemas, corpus shapes)
calque propose-branches --repo .       # intra-function dual paths (if/else arms, switch/select cases)
calque propose-values  --repo .        # scattered literal values (a maxRetries-style constant, no shared symbol)
calque propose-deriv   --repo . --judge  # add --judge to adjudicate with the LLM oracle (needs ANTHROPIC_API_KEY)
```

These are generators — they print to stdout, never write or gate, and make **zero
network calls** on their own, so they're free and safe to run against any repo
(~0.5s on this repo's own 620-function, 4-language corpus).

**To turn on `--judge`:** export `ANTHROPIC_API_KEY` (or `CALQUE_API_KEY`) in the
environment, then pass `--judge` on the command line — both steps are required;
setting the key alone does nothing, calque never calls an LLM unless you also pass
the flag. It's off by default on every command that has it (`scan`/`check` don't
have the flag at all). Each judged candidate is one real, billed Anthropic call
(cached per-pair on disk, so a re-run only pays for new candidates) — a real,
scaling cost, not a rounding error. Without it you're reading raw candidate lists
by eye — fine for a handful, not realistic for the dozens a whole-repo generator
can return (a self-scan of this repo's own Go code returned 63) — so `--judge` is
what most people will actually want past a handful of candidates.

`scan`/`check` work on Go (native `go/ast`), Python (embedded `python3`),
TypeScript/TSX (embedded `node` + the TypeScript compiler), Svelte (the
`<script lang="ts">` block, masked out of the template and run through the same TS
extractor), and Rust (an embedded `syn` helper, built once and cached on the first
`.rs` scan). Each language needs its own toolchain present for that language's
targets — `python3` for `.py`, `node`+`typescript` for `.ts`/`.svelte`, `cargo` for
`.rs` (the same toolchain you already build that code with; `CALQUE_RUST_EXTRACTOR`
can point at a prebuilt binary to skip the build). On a SvelteKit repo, use `**/`
globs (`--left "**/*.svelte" --right "**/*.ts"`); template `{#if}`/`{#each}` branches
and inline (non-function) assignments are out of scope — calque's unit is the named
function, so guard sub-function twins with differential tests.

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
   pre-commit hook; add `--post-merge` to also install a post-merge hook that
   scans the code a contributor just merged (fires on `git pull` and
   fast-forward merges, which a pre-commit hook never sees — warn-only, it
   reports without blocking). `calque mcp` serves both gates over MCP (stdio
   JSON-RPC, tools `calque_check` + `calque_vocab_check`) so an agent can ask
   "did my edit introduce new drift?" inline.

calque is built to be driven by a coding agent as the equivalence oracle — see
`SKILL.md` for the full agent-facing loop.

## Review incoming code (git hook or GitHub Actions)

The pre-commit gate only sees *your* commits. To catch drift a contributor
introduces, calque scans incoming code two ways:

- **Locally** — `calque hook install --post-merge` also installs a git
  `post-merge` hook that runs after every `git pull`/merge (including
  fast-forward, which no pre-commit hook sees). Warn-only: it reports the new
  dual-path suspects the merged code introduced without blocking.

- **On pull requests** — `calque review` runs the same gate as `check` but emits
  each new suspect as a GitHub Actions annotation (`::warning file=…,line=…::`),
  so drift shows up **inline on the PR diff**, and writes an at-a-glance markdown
  table to the job summary (`$GITHUB_STEP_SUMMARY`) so the run's Checks tab shows
  the full suspect list at a glance. It's advisory by default (exit 0 —
  annotations never fail the build); pass `--strict` to make it a hard check.
  No hosted service, no third party: the code never leaves your CI. The
  deterministic pass needs no API key; `--judge` (available on the generators —
  `propose-deriv`/`confess`/`propose-roles`/`propose-deep`/`propose-context`/
  `propose-cross`/`propose-branches`/`propose-values` — not on `check`/`review`
  itself) uses
  *your own* key as a CI secret if you choose to run one of them as a separate
  step.

  Drop this in `.github/workflows/calque.yml`:

  ```yaml
  name: calque drift review
  on: pull_request
  jobs:
    calque:
      runs-on: ubuntu-latest
      steps:
        - uses: actions/checkout@v4
          with: { fetch-depth: 0 }
        - uses: actions/setup-go@v5
          with: { go-version: stable }
        - run: go install github.com/justinstimatze/calque/cmd/calque@latest
        - run: calque review --exclude '**/*_test.go'   # advisory; never fails the build
  ```

  Commit `.calque/registry.md` and seed it once (`calque check`, adjudicate the
  initial suspects) **before** turning the PR check on — calque is recall-first,
  so a fresh repo's first run is noisy by design; the registry is the memory that
  makes every later PR quieter as the team clears suspects. For non-Go targets,
  add the matching toolchain to the workflow (`python3` for `.py`,
  `node`+`typescript` for `.ts`/`.svelte`, `cargo` for `.rs`).

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
  type's fields; two SvelteKit route handlers (`GET`/`POST`/`load`/… in
  `+server.ts`/`+page.server.ts`) sharing the framework's request shape. `scan`/`check`
  flag the structural shapes inline (`· structural: same-receiver` /
  `sveltekit-handler` / `field-copy`) — advisory, never gated. Trust shared **emitted
  strings / state writes / domain callees** (effect-footprint) over raw field-set
  or name overlap — `SKILL.md` has the full adjudication guide.
- **A zero-suspect run isn't always a clean bill.** If a `--left`/`--right` glob
  matched files but parsed **zero** functions (unsupported language on that side, or
  a stale binary on a newer repo), `scan`/`check` emit `⚠ boundary cannot bite … 0
  parsed … Result is NOT a clean bill.` before the suspect count — a false clean to
  fix, not an all-clear.
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
- **A boundary is `--left` × `--right`, not `--left` × `--left`.** Two near-identical
  twins that both land on the *same* side of a scoped boundary are never compared
  to each other — by design (a client-vs-server split shouldn't cross-pair two
  server files), not a scorer miss. Verified: a synthetic pair matching a
  real-world "same shape, both `.ts`, zero fires" report scored 1.00 the moment it
  was forced onto opposite sides (or scanned with no boundary at all, the
  self-scan default, which always covers same-side twins). If a boundary run comes
  back suspiciously clean, re-run once with no `--left`/`--right` to rule this out.

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
- **signature-rarity (`propose-deep`)** — the one channel that needs **no shared
  token at all**: it groups functions by a rare, domain-typed signature (declared
  param/return types, e.g. `(UserRecord)=>ValidationResult`), so two functions can
  pair up sharing zero strings/writes/calls/names. This is calque's actual
  zero-footprint Type-4 mechanism — every other axis, including the core pairwise
  scorer above, requires at least one overlapping token to anchor a candidate.
  All five languages, but recall quality isn't uniform across them — it tracks
  each language's own type-annotation guarantees, not a calque setting
  (`docs/DESIGN_NOTES.md` §22 breaks it down per language).
- **cross-substrate (`propose-cross`)** — non-function entities (module-level
  tables, JSON corpus shapes) extracted and scored the same way, for drift that
  lives outside a function body entirely (`docs/DESIGN_NOTES.md` §19).
- **sub-function branches (`propose-branches`)** and **scattered values
  (`propose-values`)** — duplication living *below* function granularity: two
  conditional arms that drifted apart, or a literal value (a `maxRetries`
  constant) repeated across sites with no shared symbol (`docs/DESIGN_NOTES.md`
  §21).

A `--judge` flag on the generators (`propose-deriv`/`confess`/`propose-roles`/
`propose-deep`/`propose-cross`/`propose-branches`/`propose-values`) runs an LLM
equivalence oracle as the precision half,
and `calque doctor --ablate` rolls every judged verdict into a per-detector × language
× variety matrix so each detector has to earn its keep. Further axes (config/env,
catalog, narrative) are roadmapped in `docs/DESIGN_NOTES.md` §16–18. Apache-2.0; the
prose axis, calibration, and
hook are consolidated from the sibling project `cupel` (MIT, attribution
preserved) — which now consumes calque back: cupel retired its own vocabulary
tooling and runs `calque vocab-check` as its pre-commit prose gate.
