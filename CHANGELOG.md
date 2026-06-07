# Changelog

All notable changes to calque. The version string itself comes from the git tag
(`git describe`), not this file — see `Makefile` / `cmd/calque/main.go`.

## [Unreleased]

### Changed
- **Rewrite in Go, underway.** calque becomes a substrate-general drift engine
  (code · prose · planned config) sharing one spine (recall → registry → check →
  calibrate). Decision + rationale in `docs/DESIGN_NOTES.md` §16. Scaffolded the
  Go module (`cmd/calque`, hindcast-style git-tag versioning); subcommands land
  one axis at a time. The prose axis, calibration, and hook are consolidated from
  the sibling project `cupel` (MIT → Apache-2.0, attribution preserved); the code
  axis is ported from the original Python nose.

### Added
- **Code axis: `scan`** (ported from the Python nose to Go) — ranks dual-path /
  behavioral-twin (Type-4) suspects across a boundary. `internal/code` holds the
  language-agnostic scorer (FuncSig + jaccard + delegation gate + **unordered-pair
  dedup**, fixing the Python era's symmetric-output self-bug) and a `go/ast`
  extractor (no deps). Default is a self-scan (all source × all source). Python
  targets (the stope use case) land next via a `python3`-subprocess extractor.
  Self-scan caught a live dup during implementation (`relTo`/`RelPath`) — fixed.
- **Code axis: Python targets** via an embedded `python3` AST extractor
  (`internal/code/extract.py`, reused from `legacy/core.py`) run as one
  subprocess per scan; extraction is batched per language. Verified on stope
  (10,673 funcs / 718 files): the Go scorer reproduces the original Python
  calque's seed-run scores exactly on unchanged functions (1.00/0.79/0.52/0.45).
  (The legacy Python nose no longer even runs on Python 3.14 — a point for the port.)
- **Prose axis: `vocab-report`** (ported + generalized from cupel) — read-only
  frequency surface of hyphenated compounds across any prose repo, with `--stems`
  clustering (the synonym-drift signature). Shared, generic corpus walker +
  markdown stripper in `internal/corpus` (single-sourced for the prose commands).
- **Prose axis: `synonym-report`** (ported from cupel) — embedding-based
  near-synonym surfacing (the harder word-level drift: people/person/human,
  want/wanted/desire). Local ollama client in `internal/embed`
  (`CALQUE_OLLAMA_URL`/`CALQUE_EMBED_MODEL`). A recall surface, not a gate.
  Validated on cupel: surfaces real concept clusters.
- `.calque/registry.md` is now git-tracked for calque's own self-dogfood
  (`.gitignore`: `.calque/*` + `!.calque/registry.md`).
