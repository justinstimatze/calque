# Changelog

All notable changes to calque. The version string itself comes from the git tag
(`git describe`), not this file — see `Makefile` / `cmd/calque/main.go`.

## [Unreleased]

### Added
- **Cross-substrate axis (`propose-cross`) — pair non-function entities across
  files and substrates.** The code axis only sees functions; the hardest drifts
  in a content-driven codebase live between *non-function* entities — a
  module-level table mirrored in another file, or an authored corpus shape
  (`*.json`) mirrored by a code table/schema. Those share no surface tokens, so
  the jaccard gate is structurally blind to them. The new axis extracts
  non-function "symbols" — module-level dict/set/list constants (`.py`, via the
  embedded extractor's `symbols` mode) and JSON object field-sets (`.json`, pure
  Go) — as `FuncSig`-like entities whose KEY SET is their footprint, then pairs
  them by shared key set (`KeySetCandidates`, jaccard ≥ `--key-jaccard`, default
  0.5) and adjudicates with the same LLM oracle (`--judge`). Like
  `propose-roles`/`propose-deep` it is a **generator, not a gate**: stdout only,
  no registry writes, no exit code, never part of `--strict` — non-function
  entities use a separate `ExtractSymbols` path and never enter the scoring gate,
  so the self-clean `check` is provably undisturbed. Identical-shape corpus
  objects collapse to one representative (so a common column key can't blow past
  the fanout cap and prune the real cross pair). Flags: `--min-keys`,
  `--key-jaccard`, `--max-fanout`, `--top`, `--judge`, `--twins-only`.
  **Validated read-only on a private content-driven codebase** (1346 entities:
  543 module-level tables + 803 corpus shapes → 261 candidates, 188 of them
  cross-substrate): the oracle confirmed copy-paste constant tables and two
  separately-maintained canonical-fact registries as drift (one had *already
  diverged* on a single entry — a live latent bug no function-axis tool can see),
  flagged a corpus-JSON ↔ code-table pair as a contracted twin needing a
  differential pin, and correctly rejected coincidental same-key tables (verb
  dispatch vs verb config, etc.) as false alarms.

### Changed
- **Collapsed two dual paths the self-scan flagged in the new code** (dogfooding
  the thesis): the Python function/symbol extractors now share one
  `runPyExtractor` (mode arg), and `Extract` / `ExtractSymbols` share one
  `walkExtractable` tree walk.

## [0.2.0] - 2026-06-10

The Type-4 release: a representation-independent behavioral-twin detector
(candidate generation → LLM equivalence oracle), plus adaptive weights, registry
GC, and a TypeScript extractor. All additive — no breaking changes to the
existing gates.

### Added
- **Name-stem recall pass (`propose-deep`) — broadens Type-4 candidate
  generation and makes it language-agnostic.** Signature recall only catches
  twins that share a type signature; a rigorous measurement found it missed a
  real twin whose params differed (`formatRemainingTime` ≟ `formatTimeRemaining`
  — same role, different signature). The name-stem pass pairs functions whose
  name-token SETS are near-identical (jaccard ≥ `--name-jaccard`, default 0.6)
  regardless of token order or a differing word — recovering that class (the
  verified-label recall went 2/3 → 3/3). Crucially, **name stems exist in every
  language**, so this extends Type-4 candidate generation to Go and Python
  (signature recall is TS-only) — `propose-deep` now produces candidates on
  Go/Python repos for the first time. An inverted stem index keeps it
  near-linear; a fanout cap (`--name-max-fanout`) skips ultra-common stems. The
  two passes are unioned and ranked by **gate-invisibility** (lowest jaccard
  score first = the pairs the existing scan/check gate is most blind to =
  highest unique Type-4 value), so the best of both kinds interleave.
  `--no-name-stem` disables it. The LLM judge remains the precision filter over
  the (looser, higher-recall) union.
- **LLM equivalence oracle (`propose-deep --judge`) — the precision half of the
  Type-4 loop.** Signature recall is high-recall / low-precision (many
  same-signature functions do different jobs); the oracle adjudicates each
  candidate pair — "are these two functions the same contract?" — the judgment a
  human equivalence oracle would make, automated. It reads both function bodies,
  asks the model for a `{same_contract, confidence, reason}` verdict (parsed
  defensively), and prints each candidate annotated with the verdict;
  `--twins-only` filters to confirmed twins. Deliberately **stdlib-only** (one
  `net/http` POST to `/v1/messages`) to keep calque a zero-dependency single
  binary. Results are content-hash **cached to disk** (model + both sources in
  the key), so re-runs over unchanged code are free. Defaults to
  `claude-opus-4-8`; `CALQUE_JUDGE_MODEL` (e.g. `claude-haiku-4-5`) trades
  quality for cost. Needs `ANTHROPIC_API_KEY` (or `CALQUE_API_KEY`) and fails
  loudly without one. Bounded concurrency (4 workers). Still a generator —
  stdout only, no writes, no exit code — so it can't touch the deterministic
  `check` gate. The pure request/parse/cache logic is unit-tested with a mock
  HTTP transport (no key or network).
- **`calque propose-deep` — representation-independent Type-4 candidate
  generator.** The jaccard `scan`/`check` gate scores surface tokens, so it is
  structurally blind to behavioral twins that share a *contract* but no token —
  the textbook Type-4 case (two impls of `sessionId → WorktreeInfo`, one reading
  JSON, one rebuilding from git). This pass groups functions by a rare,
  **domain-typed** signature (`(paramTypes…)=>returnType`, via the new
  `FuncSig.Sig`) — the shared contract — and emits the pairs as twin candidates,
  each tagged with the jaccard score that proves how gate-invisible it is.
  Precision boosters: rarity window, an opposed-verb filter (drops
  `insertTask≟deleteTask`, `taskStart≟taskComplete` — same shape, opposite job),
  a domain-type requirement (a signature whose only named types are stdlib
  generics like `Promise<string[]>` is too common to anchor a twin), and
  cross-file ranking. **Generator, not gate**: stdout only, no registry writes,
  no exit code — cannot disturb `check --strict`. Signatures are extracted for
  TS/TSX today (Go/Python is a planned extension). Validated on a 1,987-function
  TS repo: surfaced a real, already-drifted Type-4 twin at jaccard 0.00 — two
  `sessionId → WorktreeInfo|null` impls where one fabricates the
  `createdAt`/`isActive` fields the other stores — that the token scorer never
  sees. The motivation + measurement is recorded in DESIGN_NOTES /
  RESEARCH_AND_MARKET §5.
- **TypeScript / TSX extractor** — calque's code axis now covers `.ts`/`.tsx`,
  not just Go and Python. An embedded node script (`extract_ts.mjs`) drives the
  TypeScript compiler API and emits the same `FuncSig` JSON the go/ast and
  python3 extractors produce, so the language-agnostic scorer/cluster passes
  work unchanged. Extracts the full signal set (string literals ≥4 chars incl.
  templates, `this.x`/`a.b[]` write targets, returned object-literal keys, call
  leaf names, delegation through wrapper fields) from function declarations,
  class methods, and const-assigned arrow/function expressions. The node +
  `typescript` runtime dependency mirrors the already-accepted python3 one for
  `.py`; calque resolves `typescript` from the scanned repo's `node_modules`, a
  global install, or `CALQUE_TS` (clear error if absent). `CALQUE_NODE`
  overrides the interpreter. `.js`/`.jsx` stay counted-as-skipped (no type
  surface yet). Validated read-only on a 302-file / 2,122-function TS repo.
  Tests skip cleanly when no node/typescript toolchain is present, so the
  pure-Go build/CI never requires node. (As a side effect the new
  `extractTSBatch` and `extractPyBatch` were collapsed onto a shared
  `runJSONExtractor` — calque flagged its own dual path and it was eliminated.)
- **Registry liveness GC (`calque prune`)** — the remediation for staleness
  `check` already detects. `check` flags adjudicated pairs/clusters whose
  referenced code is gone (the dusty-over-months problem) but offered no way to
  act on it except hand-editing the markdown; `prune` re-runs liveness
  reconciliation and surgically removes the dead entries' machine lines (the
  `- pair:`/`- cluster:` anchor + its attached attributes), preserving freeform
  prose and `##` headers. Dry-run by default; `--write` removes in place after a
  `.bak` backup. Refuses to run on an empty corpus (a wrong `--repo`/over-broad
  `--exclude` would otherwise mark everything stale and wipe the registry), and
  warns when run under an `--exclude` (which can hide live code). Surfaced by a
  real dogfood run (2026-06-10): the audited repo deleted the file the whole
  registry axis pointed at, leaving 38/40 entries stale — `prune` clears them in
  one pass. Pure core (`pruneRegistry`) unit-tested for surgical removal + prose
  preservation + adjacent-entry boundaries.
- **Adaptive signal weights (`calque calibrate`)** — the calibration-leg upgrade
  that makes calque's per-channel signal weights
  (`strings`/`writes`/`name`/`calls`/`ret`) learn from adjudicated registry
  labels instead of staying hand-tuned (DESIGN_NOTES §6.1, the one steal from
  `sauremilk/drift`). The pure estimator (`internal/code.CalibrateWeights`)
  scores each channel by how well it separates useful from not-useful pairs
  (`mean(useful) − mean(not-useful)`), normalizes the positive discriminations,
  and shrinks them toward the static prior by `λ = n/(n+priorStrength)` so few
  labels can't overfit. `--write` emits a git-tracked `.calque/weights.json`;
  `scan`/`check`/`doctor` load it and score with it (`--no-calibrated-weights`
  reverts to the prior). §13-clean by construction: trains only on adjudicated
  labels, never auto-writes, and always trains on the static prior (not its own
  output). `hasAnchor` is weight-independent, so a calibrated vector only
  re-ranks anchored pairs — it can't surface or suppress an anchor, preserving
  the #16 blocking-index invariant. Warns when a label class is too thin to
  trust (calque's own repo has one useful-labeled pair, so it ships no
  weights.json and stays on the prior). Table-tested
  (`internal/code/calibrate_test.go`).
- **Role-cardinality axis (`calque cardinality`)** — calque's declare-and-gate
  axis (DESIGN_NOTES §18), the differentiator no similarity-based competitor
  occupies. Declare `- role:` / `- predicate:` / `- expected:` / `- baseline:`
  in the registry; the gate enumerates each role's implementers across the repo
  (an AND-composed predicate over `FuncSig` fields —
  `name:`/`qual:`/`file:`/`calls:`/`writes:`/`emits:`/ `returns:`, delegating
  wrappers excluded) and flags any role over its expected count, or any
  implementer past a frozen baseline (the ratchet). `--strict` exits 1. Catches
  the dual paths pairwise similarity misses by construction (no shared
  footprint) and recurrence. Dogfooded on calque's own source.
  - **Vacuity guard**: a role whose predicate matches zero implementers while
    expecting ≥1 is flagged `VACUOUS` (a stale/typo'd declaration), so the gate
    cannot pass silently while checking nothing. A `- expected: 0` ban that
    matches nothing stays correct.
- **`calque propose-roles`** — turns the N-ary private-seam cluster pass into a
  role-candidate proposer (DESIGN_NOTES §18.5 option 2), removing the
  hand-authoring friction of the cardinality MVP. For each suspect cluster it
  synthesizes a predicate from the cluster's strongest shared seam
  (`calls:`/`emits:`), **self-verifies** it by running it back through the
  matcher (reporting whether it re-selects the cluster exactly, too broadly, or
  too narrowly), and emits a paste-ready `- role:` block with `expected: 1` + a
  frozen `- baseline:`. A generator, not a gate: prints to stdout, writes
  nothing, never exits non-zero — so it cannot disturb a repo's `check --strict`
  state. Dedups against already-declared roles and adjudicated `- cluster:`
  verdicts. The discover-then-declare loop is adjacent prior art (drift's
  declared boundaries; DejaVu/FICS recall-then-classify); the *cardinality* gate
  it targets is the unoccupied part. Dogfooded on calque's own source. Pinned by
  `cmd/calque/propose_test.go`.
- **Collapse-direction registry fields + unresolved-drift surfacing** — a
  `drift`-verdict `- pair:` may now carry `- canonical:` (the path to keep) and
  `- do-not-resync:` (the doomed path), recording which way to collapse a known
  dual path (DESIGN_NOTES §18.7 steal #4). `check` surfaces every known-drift
  pair whose **both** paths are still live as `DRIFT (unresolved)` with its
  recorded collapse direction (or a prompt to add one) — warn-only, never
  affects `--strict` exit, so an in-progress collapse doesn't gate. This closes
  the failure mode where a later agent "fixes" drift by re-syncing the doomed
  path, maintaining the dual path instead of collapsing it. Pinned by
  `cmd/calque/check_test.go` (`TestUnresolvedDrift`) +
  `internal/registry/registry_test.go` (`TestLoadEntryCollapseFields`).
- **Blocking index for the pairwise scan (task #16, slice 1)** — `code.Rank` no
  longer visits the full L×R product. A new inverted index
  (`internal/code/block.go`) over the four channels `scorePair` can anchor on
  (name-stem, strings, writes, ret) generates only the candidate pairs that
  could possibly survive the gate, then scores those. This is
  **output-identical** to the old double loop — proven by
  `TestRankBlockingEquivalence` (naive vs blocked, fixtures anchored
  independently per channel + calque's own corpus) — while scoring ~3% of pairs
  on calque's source (96 of 3249). The boundary-blind whole-repo scan it makes
  tractable was already the default (empty `--left/--right` → all×all); this is
  the scaling half.

### Fixed
- **Non-reproducible suspicion score.** `scorePair` summed its weighted signals
  by ranging a Go map; because float addition is non-associative and map
  iteration is randomized, a pair sitting near the `min-score` threshold could
  be included on one run and dropped on the next. Scores are now summed in a
  fixed channel order (`channelOrder`), making the gate deterministic — a
  code-health tool must not carry the irreproducibility it exists to flag.
  Surfaced by the blocking-index equality test.

### Docs
- **Primary-source teardown of `sauremilk/drift`** (the sharpest in-category
  competitor) in DESIGN_NOTES §6.1, with RESEARCH_AND_MARKET §2/§4 stamped
  verified. Resolves the repo-path question (canonical `sauremilk/drift`;
  `mick-gsk/drift` is the same content under a prior handle), confirms the
  2026-06-08 correction held, and sharpens it: drift is **AST-structural with no
  data-flow/effect signal** (so its 0.80-Jaccard gate is provably blind to
  calque's effect-footprint slice), declares **layer boundaries not
  implementation cardinality** (calque's cardinality axis stays unoccupied),
  supports **17/24 signals on TypeScript**, and reweights from **adjudicated git
  outcomes** — the §13-clean path to making calque's static weights adaptive,
  queued as a calibration-leg upgrade.

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
