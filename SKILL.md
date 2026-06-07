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

## Keeping it honest

- `calque hook install` writes a git pre-commit hook running `check` (warn-only
  by default; `--strict` to block; no-ops when calque isn't on PATH).
- `calque mcp` serves both gates over MCP (stdio JSON-RPC) — tools
  `calque_check` + `calque_vocab_check` — so an agent can ask "did my edit
  introduce new drift?" inline, without shelling out.
- `calque doctor` rolls up calibration (does the ranker discriminate real drift
  from false alarms?); `calque migrate-registry` converts a Python-era registry.

## Notes

- **Recall over precision.** Expect false alarms; that's the design. Pairs that
  only share generic callees are gated out, but name+surface coincidences slip
  through — your job is the filter.
- **Go + Python today; TS later.** `scan`/`check` parse Go natively (`go/ast`)
  and Python via an embedded `python3` extractor (needs `python3` on PATH). The
  signals are language-agnostic in concept; only the extractor is per-language.
- **Tune the boundary, not the threshold.** A well-chosen `--left/--right` (two
  things that genuinely should agree) gives far better signal than scanning a
  whole repo against itself.
