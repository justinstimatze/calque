---
name: calque
description: Surface "one thing defined in N places that have silently drifted" — dual-path / behavioral-twin code (a test harness that reimplements production logic) and drifting prose compounds. Use when a codebase or doc set has parallel expressions of one contract/term meant to stay in sync, and you want to find where they diverged. Recall-only scanner + a registry of your verdicts; you are the precision half.
---

# calque — drift nose (code + prose)

**The problem.** Large, agent-edited bodies of code and prose grow *drift*: one
canonical thing expressed in N places that were supposed to stay in step and
didn't. Two implementations of one contract (a test harness reimplements an
engine method and forgets a step; a client hardcodes a verb list the server also
owns; a v2 path diverges from v1). Or one concept named with drifting compounds
across a doc set. The invariant is always the same:

> **one canonical thing → N expressions → independent drift.**

Code drift is a **Type-4 (behavioral) clone**: dissimilar in syntax *by
construction* — which is exactly why clone detectors, embeddings, grep, and LSP
miss it. The twins don't call each other and don't look alike; the only thing
they share is their *contract*.

**The approach — a hybrid loop.** calque is the cheap, high-recall **gate** (the
deterministic recall half; "recall" describes the tuning, not a separate role).
*You* are the precision half — the equivalence oracle. The registry is the
durable memory so neither of you re-litigates a cleared suspect across a context
reset.

```
calque scan / check (recall)  →  you adjudicate (judgment)  →  registry (memory)
```

It is undecidable to *prove* two functions equivalent, so don't try. calque only
has to be a good nose; you make the call.

## Install

```bash
go install github.com/justinstimatze/calque/cmd/calque@latest   # or `make install` from a clone
```

## The loop (code axis)

1. **Pick a boundary** — the two sides that should agree. Most common:
   `--left "engine*.py" --right "testing.py"` (harness vs prod). Others:
   client-glob vs server-glob, `*_v2.py` vs `*_v1.py`. A well-chosen boundary
   beats threshold-tuning every time.

2. **Scan** (one-shot recall) or **check** (only what's new vs the registry):
   ```
   calque scan  --repo <path> --left "<glob>" --right "<glob>"
   calque check --repo <path> --left "<glob>" --right "<glob>"   # diffs the registry
   ```
   Output is a ranked list of suspect pairs — and N-ary **clusters** (a shared
   private seam inlined across several functions, which pairwise scoring dilutes)
   — each with its firing signal (`shared-writes=[...]`, `shared-strings=N`,
   `name~0.x`).

3. **Adjudicate each suspect** (this is you). Open the functions and classify:
   - **`drift`** — same contract, behavior diverges. The bug. Fix by *collapsing
     to a single path* (extract shared logic, make one delegate to the other) —
     far better than keeping both and adding a test.
   - **`contracted-twin-ok`** — intentionally parallel and currently in sync
     (e.g. both delegate to one shared function). Record it so it's not
     re-flagged, and ideally pin it with a differential test so it *stays* in
     sync. (Hypothesis `RuleBasedStateMachine` for Python; rapid for Go;
     fast-check for TS — drive both, assert equal outputs.)
   - **`false-alarm`** — unrelated; the signal was coincidental. Record to
     suppress.

   The firing signal tells you where to look: `shared-writes=['world.ruin']`
   means "both mutate ruin — compare how." High `name~` with low surface/effect
   overlap often means a real twin one side fakes.

4. **Record the verdict** in `<repo>/.calque/registry.md` as a `- pair: A | B`
   (or `- cluster: A | B | …`) line with `- verdict:` and `- reviewed:`. The
   registry is the externalized "these must agree" memory that survives every
   context reset, in any repo. After fixes, re-`check`: a collapsed pair drops
   off automatically; stale entries (referenced code gone) are flagged for
   pruning.

## Prose axis

Same loop, different recall extractor — drifting hyphenated compounds vs an
allow-list (`.calque/vocab-allowlist.txt`, the prose registry):

```
calque vocab-report                          # frequency surface (recall)
calque vocab-check                           # gate: compounds not in the allow-list
calque vocab-check --bootstrap               # seed the allow-list from the current tail
calque vocab-check --seed-cmd '<proj seeder>'  # merge a project's own slug list (prints slugs to stdout)
calque synonym-report                        # embedding near-synonyms (needs local ollama)
```

## Boundary-free audit (no `--left/--right`)

Standing whole-repo generators for the drift the boundary scan can't frame. All
print to stdout, write nothing, never exit non-zero, and make **zero network
calls** unless you pass `--judge` — pure static analysis, so running any of
them costs nothing and is safe on any repo (~0.5s on calque's own 620-function,
4-language corpus; Rust pays a one-time ~20s toolchain build on the very first
`.rs` scan anywhere, cached afterward):

```
calque propose-deriv --repo <path>   # value-derivation twins: the same quantity derived
                                     # two ways from the same input field-set, no shared
                                     # authority ("fix one path, the twin still has the bug")
calque confess       --repo <path>   # functions whose own comment confesses a twin
                                     # ("mirrors X", "keep in sync with Y", "copy of")
calque propose-roles --repo <path>   # N-ary seam clusters → paste-ready cardinality roles
calque propose-deep  --repo <path>   # Type-4 twins sharing a rare type signature, no shared
                                     # tokens needed at all — the one channel that isn't
                                     # effect-footprint overlap. All five languages.
calque propose-context --repo <path> # call-site context axis: zero shared tokens AND no
                                     # distinctive signature — anchors on caller name-stem +
                                     # call-result shape instead. Go-only.
calque propose-cross --repo <path>   # non-function entities (tables, schemas, corpus shapes)
calque propose-branches --repo <path> # intra-function dual paths: if/else arms, switch/select
                                     # cases that drifted apart — below function granularity
calque propose-values --repo <path>  # scattered literal values (a maxRetries-style constant)
                                     # repeated across sites with no shared symbol
```

**Enabling `--judge`.** Add it to any command above (or to `confess`/
`propose-branches`/`propose-cross`/`propose-deriv`/`propose-values`/
`propose-roles`) to run the LLM equivalence oracle as the precision half
automatically. To enable it: export `ANTHROPIC_API_KEY` (or `CALQUE_API_KEY`)
in the environment calque runs in, then pass `--judge` on the command line —
both are required; a key alone does nothing, and there is no config file or
auto-detection that turns it on for you. `--twins-only` then prints only the
confirmed twins.

**Why it's opt-in, not automatic:** each candidate costs one real, billed
Anthropic API call (`CALQUE_JUDGE_MODEL` picks the model; results are cached
on disk per-pair, so a re-run only pays for genuinely new candidates) — a
whole-repo generator can return dozens of candidates in one pass, so this is a
real, scaling cost, not a rounding error. **When to reach for it:** a handful
of candidates (roughly under ten) are usually faster to read by eye than to
wait on API round-trips for; past that, `--judge` is the practical way to
actually process the output rather than eyeballing every row. **Without
`--judge`**, you (or an agent operating calque on someone's behalf) adjudicate
by hand exactly as in the loop above, recording verdicts in the registry —
realistic for small candidate counts, not for the full output of a whole-repo
sweep.

## Keeping it honest

- `calque hook install` writes a git pre-commit hook running `check` (warn-only
  by default; `--strict` to block; no-ops when calque isn't on PATH). Add
  `--post-merge` to also install a post-merge hook that scans incoming code: it
  fires after every `git pull`/merge — including fast-forward, which a
  pre-commit hook never sees because no commit of yours happens — so a
  contributor's merged code is checked for new dual-path drift before you build
  on it. Always warn-only (the merge has already happened; it reports, never
  blocks).
- `calque review` is the CI/pull-request surface: the same gate as `check`,
  emitted as GitHub Actions annotations (`::warning file=…,line=…::`) so drift
  shows up inline on the PR diff. Advisory by default (exit 0 — never fails the
  build); `--strict` makes it a hard check. No hosted service; BYOK for the
  `--judge` precision half. Seed `.calque/registry.md` before turning it on
  (recall-first → a fresh repo's first run is noisy; the registry decays the
  noise as the team adjudicates). README has a ready-to-paste workflow.
- `calque mcp` serves both gates over MCP (stdio JSON-RPC) — tools
  `calque_check` + `calque_vocab_check` — so an agent can ask "did my edit
  introduce new drift?" inline, without shelling out.
- `calque doctor` rolls up calibration (does the ranker discriminate real drift
  from false alarms?); `calque migrate-registry` converts a Python-era registry.

## Reading the output — signal vs. the common false alarms

calque is recall-first: a ranked list to adjudicate, not a verdict. A handful of
patterns recur as false alarms — recognize them so you mark `false-alarm` fast
(record it in the registry so it doesn't re-surface) and don't "fix" a non-bug:

- **Test fixtures (now auto-handled).** Two *test* functions sharing a setup/mock
  read-set or helper seam are the single most common false twin. By default
  calque **gates test↔test pairs and all-test clusters** across
  `scan`/`check`/`propose-deriv`/`propose-roles`/`confess`. It deliberately
  **keeps test↔production** pairs — a test that reimplements production
  construction or recomputes a production quantity is *real* drift, not noise.
  Pass `--include-tests` to any of those commands to see the test↔test pairs
  anyway. (Test = `*_test.go`, `tests.rs`, `test_*.py`, `*.test.ts`, a `tests/`
  dir, or a Rust `#[cfg(test)]` / `#[test]` function — including inline test
  modules that sit in a production `.rs` file.)
- **Field projections / conversions.** A DTO mapper or projection reads fields
  off struct A and writes the same-named fields onto struct B. It shares a field
  set with its siblings but isn't *deriving* anything — `false-alarm`.
- **Same fields, different operation.** Functions over the same numeric struct
  (`x`/`y`/`width`/…) that do *different* arithmetic share a read-set by
  coincidence, not by contract. Lean on `--judge`, or mark `false-alarm`.
- **Same-receiver methods.** Two methods on one type naturally share that type's
  fields; that overlap is expected, not drift.
- **SvelteKit route handlers.** Two framework exports (`GET`/`POST`/`load`/
  `actions`/…) in `+server.ts` / `+page.server.ts` / `+layout.server.ts` modules on
  *different* routes share the verb name and the request/locals/params shape by
  construction — a route-handler shape, not a contract twin.

`scan` and `check` flag the structural shapes **inline**: a suspect line ending in
`· structural: same-receiver`, `· structural: sveltekit-handler`, or
`· structural: field-copy` is calque telling you it matched one of those
usually-noise shapes. Advisory only — it never drops the pair; it just speeds your
triage.

**Watch for a "boundary cannot bite" warning.** If a `--left`/`--right` glob
matched files on disk but parsed **zero** functions from them — an unsupported
language on that side, or a stale binary pointed at a newer repo — `scan`/`check`
print `⚠ boundary cannot bite: … 0 parsed … Result is NOT a clean bill.` *before*
the suspect count. A zero-suspect result under that warning is a **false clean**
(nothing parsed, so nothing could fire), not a real all-clear — fix the glob or the
toolchain and re-run.

What to **trust** over raw field-set or name overlap: shared **emitted strings**,
shared **state writes**, and shared **domain-specific callees** — the
effect-footprint signals. A pair that shares real behavioral machinery (the same
emitted markers, the same mutations, the same validation calls) is a far stronger
twin than one that merely touches the same field set, even when the latter ranks
higher on name similarity. When in doubt, `--judge` is the precision half.

## Notes

- **Recall over precision.** Expect false alarms; that's the design. Pairs that
  only share generic callees are gated out, but name+surface coincidences slip
  through — your job is the filter.
- **Five languages today.** `scan`/`check` parse Go natively (`go/ast`), Python
  via an embedded `python3` extractor, TypeScript/TSX via `node` + the TypeScript
  compiler, Svelte by slicing the `<script lang="ts">` block (template masked out)
  and running it through the same TS extractor, and Rust via an embedded `syn` helper
  (built once and cached on the first `.rs` scan). Each language needs its own
  toolchain present for that language's targets (`.svelte` uses the `.ts` toolchain);
  the signals are language-agnostic in concept, only the extractor is per-language.
  On SvelteKit, use `**/` globs (`--left "**/*.svelte" --right "**/*.ts"`); Svelte
  template `{#if}`/`{#each}` branches and inline non-function assignments stay out of
  scope (sub-function units) — guard those with differential tests.
- **Tune the boundary, not the threshold.** A well-chosen `--left/--right` (two
  things that genuinely should agree) gives far better signal than scanning a
  whole repo against itself.
