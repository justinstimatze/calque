# Changelog

All notable changes to calque. The version string itself comes from the git tag
(`git describe`), not this file — see `Makefile` / `cmd/calque/main.go`.

## [Unreleased]

### Added
- **Role-cardinality axis (`calque cardinality`)** — calque's declare-and-gate axis
  (DESIGN_NOTES §18), the differentiator no similarity-based competitor occupies.
  Declare `- role:` / `- predicate:` / `- expected:` / `- baseline:` in the registry;
  the gate enumerates each role's implementers across the repo (an AND-composed
  predicate over `FuncSig` fields — `name:`/`qual:`/`file:`/`calls:`/`writes:`/`emits:`/
  `returns:`, delegating wrappers excluded) and flags any role over its expected count,
  or any implementer past a frozen baseline (the ratchet). `--strict` exits 1. Catches
  the dual paths pairwise similarity misses by construction (no shared footprint) and
  recurrence. Dogfooded on calque's own source.

## [0.1.0] - 2026-06-07

First tagged release. The Go rewrite is complete: a substrate-general drift nose
with a finished spine (recall → registry → check → calibrate) across two axes
(code · prose), git + MCP hooks, and calibration — dogfooded clean on its own
source and validated on a real Python codebase.

### Changed
- **Rewrite in Go — complete.** calque is a substrate-general drift engine
  (code · prose · planned config) sharing one spine (recall → registry → check →
  calibrate). Decision + rationale in `docs/DESIGN_NOTES.md` §16. Go module
  (`cmd/calque`, hindcast-style git-tag versioning); each subcommand is one leg
  of the spine or one axis. The prose axis, calibration, and hook are
  consolidated from the sibling project `cupel` (MIT → Apache-2.0, attribution
  preserved); the code axis is ported from the original Python nose.

### Removed
- **The `legacy/` Python tree.** It held the original Python calque as a port
  reference; the Go rewrite reached and exceeded parity (the live Python AST
  extractor it depended on now lives standalone at `internal/code/extract.py`),
  so the reference is retired. README/SKILL no longer document `python -m calque`.

### Added
- **`check` warns on a zero-parse registry** — if a registry file exists and has
  real content but parses to zero entries (almost always a format/path mismatch),
  `check` now prints `⚠ registry … has content but parsed 0 entries …` instead of
  silently treating the whole repo as new. Detects the Python-era format and names
  `migrate-registry`. Closes the silent-cry-wolf failure mode seen in practice
  (a 30-entry registry read as 0 → 26k false "new"). Pinned by
  `cmd/calque/check_test.go`.
- **`migrate-registry`** — one-time converter for Python-era registries. The
  Python calque wrote `## id — verdict` + `- left:`/`- right:` blocks; the Go
  parser keys on `- pair: <left> | <right>`, so an un-migrated registry parses to
  ZERO entries and `check` falsely flags the whole repo as new (hit in practice: a
  30-entry registry read as 0 known → 26k "new"). Conservative: preserves all
  human prose, skips ``` fences (the registry's own template), inserts the
  `- pair:` line, and normalizes verdict (`contracted-twin-ok (collapsed) — was
  drift` → `contracted-twin-ok`) and reviewed-date. `--write` overwrites in place
  after a `.bak` backup; default is a dry run to stdout.
- **Spine: `mcp`** — serve the gates over MCP (stdio JSON-RPC 2.0) so an agent
  editing code or prose can ask "did my change introduce new drift?" inline,
  without shelling out, and get the same report the CLI prints. Two tools — the
  two gates: `calque_check` (code axis: scan + registry diff → new/known/stale)
  and `calque_vocab_check` (prose axis: compounds vs the allow-list, with
  `seed_cmd` support). Read-only (no fire log, no exit). To keep CLI and MCP from
  drifting, each gate now splits into a pure core (`computeCheck`/`renderCheck`,
  `computeVocabCheck`/`renderVocabCheck`) shared by both paths — the dogfood loop
  flagged the resulting parallelism and it was adjudicated contracted-twin-ok.
  The JSON-RPC framing is lifted from the sibling Go project hindcast
  (`cmd/hindcast/cmd_mcp.go`), a zero-dependency stdlib implementation, so calque
  stays dependency-free. Answers "calque as MCP or CLI" — now both. Pinned by
  `cmd/calque/mcp_test.go`.
- **Prose gate: `vocab-check`** — the prose-axis analog of `check` (the last piece
  blocking cupel from retiring its own `vocab-audit`). Flags hyphenated compounds
  at freq ≥ threshold not in an allow-list (`.calque/vocab-allowlist.txt` — the
  *prose registry*, now git-tracked alongside `registry.md`). Warn-only by default,
  `--strict` to gate, `--bootstrap` to seed the allow-list from the current
  compound tail. Substrate-general: unlike cupel's vocab-audit there's no auto-seed
  from project catalogs (engines/clusters/glossary) — that domain logic stays in
  cupel; the *gate* is shared. The compound walk→tally is single-sourced as
  `tallyCompounds` across vocab-report and vocab-check (the dedup the registry's
  runSynonymReport≟runVocabReport note predicted for "a third prose command").
  Validated on cupel (551 files): 2438 violations → `--bootstrap` → clean.
  Ported from cupel `cmd/cupel/vocab_audit.go` (MIT, attribution preserved). Pinned
  by `cmd/calque/vocab_check_test.go`.
- **`vocab-check --seed-cmd`** — the seeder plugin point. Runs a project's own
  command (cwd = `--dir`) and merges its stdout into the allow-list under the
  *seeder contract* (one slug per line, `#` comments). Lets a project feed bespoke
  catalog→slug logic in without calque knowing the catalog shape — e.g.
  `calque vocab-check --seed-cmd 'cupel vocab-seed'` collapses cupel's
  seed-then-check two-step into one atomic call. Best-effort: a seed failure warns
  but doesn't wedge the gate (the file allow-list still applies). The answer to
  "how do future projects write their own seed easily" — they print slugs to
  stdout; calque does the rest.
- **`--exclude` on the prose axis** (vocab-report/synonym-report/vocab-check) —
  path globs skipped during the corpus walk (e.g. `refs/**,theory/working/**`),
  the prose analog of the code axis's `--exclude`. The glob→regexp matcher is now
  single-sourced in `internal/glob` (was duplicated in `internal/code`; the dedup
  calque flagged on itself). Lets a consumer scope the gate to authored prose —
  e.g. cupel excludes its reference corpus, taking its gate from 357 → 4 warnings.
- **Spine: `doctor` + `mark-fire`** — the calibration leg (was stubbed). A "fire"
  is a suspect the gate surfaces; `doctor` does a live scan, joins each suspect to
  a calibration label (registry verdicts double as labels — drift→useful,
  false-alarm→not-useful, contracted-twin-ok→mixed; manual `mark-fire <id>
  <verdict>` fills the rest), and reports the **discrimination signal**: mean score
  of useful vs not-useful suspects + precision@k. On calque itself it reads
  *✓ ranker discriminates — drift outscores false-alarms by 0.17* — the first
  data-grounded evidence the score tracks real drift (DESIGN_NOTES §9 #5). `check`
  appends NEW suspects to `.calque/fires.jsonl` (stable-id'd, deduped, gitignored;
  `--no-fire-log` to disable) and prints each fire's id for `mark-fire`. Shape
  ported from cupel `cmd/cupel/calib.go` (MIT, attribution preserved). The
  extract→rank→cluster pipeline is single-sourced as `codeAxis` across
  scan/check/doctor (another dedup the touchpoint pass drove on calque's own code).
  Pinned by `cmd/calque/calib_test.go`.
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
  block inlined into several large, differently-named functions (the triple-shell
  input-path case). Inverts the problem — an index of *private seam symbols*
  (leading-underscore / unexported call·write·getattr-string names, minus language
  builtins) → the functions touching each; a symbol touched by 2..K functions is a
  shared internal seam, scored by rarity (`1/fanout`, repo-size-independent,
  private-boosted). Emits a *cluster* `{members, shared seams}` — the N-ary unit a
  pairwise nose cannot express. Wired into `scan` (report section) and `check`
  (NEW-CLUSTER / known / STALE-CLUSTER); the registry parses `- cluster:` lines,
  keyed on the member SET (`pairkey.SetKey`). `scorePair` left untouched to
  preserve the parity-verified baseline. **Validated:** surfaced the exact
  three-shell input-path cluster on the calibration target; the
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
  targets (the primary use case) land next via a `python3`-subprocess extractor.
  Self-scan caught a live dup during implementation (`relTo`/`RelPath`) — fixed.
- **Code axis: Python targets** via an embedded `python3` AST extractor
  (`internal/code/extract.py`, ported from the original Python) run as one
  subprocess per scan; extraction is batched per language. Verified on a real
  codebase (10,673 funcs / 718 files): the Go scorer reproduces the original Python
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
