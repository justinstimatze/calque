---
name: calque
description: Find dual-path / behavioral-twin code — two implementations of one contract that have silently drifted (e.g. a test harness that reimplements production logic). Use when a codebase has parallel paths meant to behave identically (testing vs prod, client vs server, legacy vs new) and you want to surface where they've diverged. Recall-only scanner + your judgment.
---

# calque — dual-path / behavioral-twin finder

**The problem.** Large codebases (especially agent-edited ones) grow *dual paths*:
two implementations of the same contract that are supposed to behave identically
but silently drift. A test harness reimplements an engine method and forgets a
step; a client hardcodes a verb list the server also owns; a v2 path diverges
from v1. These are **Type-4 (behavioral) clones**: dissimilar in syntax *by
construction* — which is exactly why clone detectors, embeddings, grep, and LSP
miss them. The two twins don't call each other and don't look alike; the only
thing they share is their *contract*.

**The approach — a hybrid loop.** calque is the cheap, high-recall **gate** (the
deterministic recall half; "recall" describes the tuning, not a separate hybrid
role). It indexes the signals that stay invariant when a body is rewritten
(emitted strings, state writes, returned keys, callees, name-stem) and ranks
suspect pairs. *You* are the precision half — the equivalence oracle. The registry is the
durable memory so neither of you re-litigates a cleared pair.

```
calque scan (recall)  →  you adjudicate (judgment)  →  registry (memory)
```

It is undecidable to *prove* two functions equivalent, so don't try. calque only
has to be a good nose; you make the call.

## The loop

1. **Pick a boundary** — the two sides that should agree. Most common:
   `--left "engine*.py" --right "testing.py"` (harness vs prod). Others:
   client-glob vs server-glob, `*_v2.py` vs `*_v1.py`.

2. **Scan:**
   ```
   python -m calque scan --repo <path> --left "<glob>" --right "<glob>" --out /tmp/calque.md
   ```
   Output is a ranked list of suspect pairs, each with the *firing signal*
   (`shared-writes=[...]`, `shared-strings=N`, `name~0.x(tokens)`).

3. **Adjudicate each suspect** (this is you). Open both functions and classify:
   - **`drift`** — same contract, behavior diverges. The bug. Fix by *collapsing
     to single-path* (extract shared logic, make one delegate to the other) —
     that's the durable fix, far better than keeping both and adding a test.
   - **`contracted-twin-ok`** — intentionally parallel and currently in sync
     (e.g. both delegate to one shared function). Record it so it's not
     re-flagged, and ideally pin it with a differential test so it *stays* in
     sync. (Hypothesis `RuleBasedStateMachine` for Python; rapid for Go;
     fast-check for TS — drive both, assert equal outputs.)
   - **`false-alarm`** — unrelated; the signal was coincidental. Record to
     suppress.

   The firing signal tells you where to look: `shared-writes=['world.ruin']`
   means "both mutate ruin — compare how." High `name~` with low surface/effect
   overlap often means a real twin that one side fakes.

4. **Record the verdict** in `<repo>/.calque/registry.md` (see the template).
   The registry is the artifact that beats the partial-context problem: it's the
   externalized "these must agree" memory that survives every context reset, in
   any repo.

5. **Re-scan after fixes.** A collapsed pair drops off the list automatically
   (its body shrank to a delegation). The suspect list shrinks as you single-path.

## Notes

- **Recall over precision.** Expect false alarms; that's the design. A pair that
  only shares generic callees is gated out, but name+surface coincidences slip
  through — your job is the filter.
- **Python today; Go/TS later.** The signals (`core.py`) are language-agnostic in
  concept; only the AST extractor is per-language. A Go port can compute the same
  signals as SQL over a code-graph (e.g. `defn`) instead of re-parsing.
- **Tune the boundary, not the threshold.** A well-chosen `--left/--right` (two
  things that genuinely should agree) gives far better signal than scanning a
  whole repo against itself.
