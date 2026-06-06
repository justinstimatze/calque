# calque

**Find dual-path code — two implementations of one contract that have silently drifted.**

A *calque* is a structural copy carried across languages (English "skyscraper" →
French "gratte-ciel": same structure, different surface). This tool hunts the
software version: two code paths that share a contract but have diverged in shape
and, eventually, in behavior.

## Why existing tools miss this

Dual paths are **Type-4 (behavioral) clones** — dissimilar in syntax *by
construction*. So:

- **grep / ctags / LSP** index lexical identity and the call graph — but the two
  twins *don't call each other*, so there's no edge to follow.
- **embeddings / "semantic" code search** embed *how code is written* — and the
  twins are written nothing alike, so they embed far apart.
- **clone detectors** (`dupl`, jscpd, …) look for *similar* code — the opposite
  of the problem.

They all index **representation**. Dual-path is a **role collision in
behavior-space**, and representation is the dimension along which the twins
diverged. You have to index the contract, not the prose.

## How calque works

It's the recall half of a hybrid heuristic+LLM loop:

```
calque scan (cheap, high-recall)  →  LLM/human adjudicates  →  registry (memory)
```

It extracts the signals that stay **invariant when a body is rewritten** —
emitted string literals (what it *says*), attribute write-targets (what it
*mutates*), returned dict keys (what it *hands back*), callee names, and the
name-stem (the *role*) — then ranks cross-boundary pairs by overlap. It never
claims equivalence (that's undecidable); it hands you a short ranked list of
suspects to judge.

## Usage

```bash
python -m calque scan \
    --repo /path/to/repo \
    --left  "engine*.py" \
    --right "testing.py" \
    --out /tmp/calque.md
```

Then adjudicate each suspect (`drift` / `contracted-twin-ok` / `false-alarm`) and
record verdicts in `<repo>/.calque/registry.md`. See `SKILL.md` for the full loop
— calque is built to be driven by a coding agent as the equivalence oracle.

## Status

v0.0.1 — Python extractor only. Validated on a real codebase (re-surfaced the
known engine↔test-harness reimplementation family without being told where to
look). Go/TS extractors planned; the ranking core is language-agnostic.
