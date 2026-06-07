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
- **Spine: `hook`** — makes the gate ongoing/automated (the point of `check`).
  `calque hook install` writes a git pre-commit hook running `calque check`
  (warn-only by default — never blocks a commit — `--strict` to gate; no-ops when
  calque isn't on PATH; never clobbers an existing hook); `calque hook` prints the
  pre-commit + Claude Code Stop-hook snippets. Boundary flags (`--repo/--left/
  --right/--exclude`) are single-sourced via `addBoundaryFlags`, shared by
  scan/check/hook — itself a dedup the N-ary touchpoint pass flagged on calque's
  own new code (the flag block was duplicated verbatim across the three commands).
  Pinned by `cmd/calque/hook_test.go`.
- **Code axis: N-ary touchpoint clustering** (`internal/code/touchpoint.go`) — the
  recall upgrade for the case pairwise scoring structurally misses: a small shared
  block inlined into several large, differently-named functions (stope's #269
  triple-shell). Inverts the problem — an index of *private seam symbols*
  (leading-underscore / unexported call·write·getattr-string names, minus language
  builtins) → the functions touching each; a symbol touched by 2..K functions is a
  shared internal seam, scored by rarity (`1/fanout`, repo-size-independent,
  private-boosted). Emits a *cluster* `{members, shared seams}` — the N-ary unit a
  pairwise nose cannot express. Wired into `scan` (report section) and `check`
  (NEW-CLUSTER / known / STALE-CLUSTER); the registry parses `- cluster:` lines,
  keyed on the member SET (`pairkey.SetKey`). `scorePair` left untouched to
  preserve the parity-verified stope baseline. **Validated:** surfaced the exact
  #269 trio (`GameSession.step`/`GameEngine.run`/`GameEngine.step`) on stope; the
  self-scan caught the N-ary extent of calque's own taxonomy drift (DESIGN_NOTES
  §15). Pinned by `internal/code/touchpoint_test.go`.
- **Spine: `check`** — the registry-aware gate (the keystone for ongoing/hookable
  use). Scans, diffs against `.calque/registry.md`, and surfaces only NEW
  (un-adjudicated) pairs; suppresses known ones; reconciles STALE entries (pairs
  whose referenced code no longer exists — the dusty-registry problem, handled by
  liveness, not age-eviction). Warn-only; `--strict` exits 1 on new suspects
  (pre-commit / Stop-hook shaped). `internal/registry` parses the human-readable
  registry's `- pair:`/`- verdict:`/`- reviewed:` lines. calque now self-checks
  clean (0 new), and the self-scan caught a real dup (`unordered`/`unorderedKey`)
  → single-sourced into `internal/pairkey`.
- **`--exclude` glob(s)** on `scan`/`check` (e.g. `legacy/**`) — also the corpus-
  scoping knob the prose axis needs.
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
