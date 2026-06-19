# Changelog

All notable changes to calque. The version string itself comes from the git tag
(`git describe`), not this file — see `Makefile` / `cmd/calque/main.go`.

## [Unreleased]

## [0.8.0] - 2026-06-19

### Added
- **Svelte (`.svelte`) extraction — `<script>` blocks are now first-class.** A
  `.svelte` file's `<script lang="ts">` / `<script module>` blocks are sliced out
  (everything outside them masked to whitespace, newlines preserved so line numbers
  stay exact) and run through the existing TypeScript extractor, so a Svelte
  component's functions get the full effect-footprint interchange
  (`writes`/`reads`/`strings`/`ret_keys`/`calls`/`consts`/`delegates`) at parity with
  a plain `.ts` file — including the cross-substrate symbol axis. This closes the gap
  where a SvelteKit boundary scan (`+page.svelte` × `+server.ts`) returned nothing
  because the `.svelte` side was never parsed. Template markup (`{#if}`/`{#each}` and
  mustache expressions) lives outside `<script>` and stays out of scope by design;
  intra-function and inline-assignment twins remain sub-function units calque does
  not score (guard those with differential tests). Needs the same `node` +
  `typescript` toolchain as the `.ts` path.

## [0.7.0] - 2026-06-18

### Added
- **Inline structural false-alarm hints on `scan`/`check` suspects.** A suspect pair
  that matches a usually-noise shape now carries an advisory tag on its output line:
  `· structural: same-receiver` (both functions are methods on one type, so sharing
  that type's fields is expected) or `· structural: field-copy` (both are
  projection / DTO mappers that write back the fields they read rather than deriving
  a value). `code.FalseAlarmHint` is deterministic, conservative, and purely
  advisory — it never gates a pair, it just surfaces the structure inline so an
  adjudicating agent or human triages faster (mapping the tag to the `SKILL.md`
  "reading the output" guide). Shared by the CLI and the `calque_check` MCP tool.
- **Asymmetric test-file awareness across every code pass — drop test↔test, keep test↔prod.**
  Test code is now a first-class signal (`FuncSig.Test`), set by file convention (`IsTestPath`:
  `*_test.go`, `tests.rs`, `test_*.py`, `*.test.ts`, a `tests/` dir, …) AND — the case no path
  rule can see — by the Rust extractor flagging `#[cfg(test)]` / `#[test]` functions that live
  inside an otherwise-production `.rs` file. Every recall pass (`scan`/`check` pairs + N-ary
  clusters, `propose-deriv`, `propose-roles`, `confess`) now gates a pair/cluster when *both*
  sides are test code — two test cases sharing a setup/mock fixture are the dominant false twin
  — while always keeping a **test↔production** pair: a test that reimplements production
  construction or recomputes a production quantity is real drift, not noise. `--include-tests`
  opts the test↔test pairs back in on each command. This replaces the old whole-file glob
  exclusion in `propose-deriv`/`propose-roles`/`confess` (one shared `testGlobs` list) with a
  single definition of "what is a test" (`IsTestPath`) consulted everywhere, so calque can't
  drift on its own test-handling — and, unlike the glob, it recovers the test↔prod twins the
  blunt exclusion silently dropped. Dogfood on calque's own (test-heavy) corpus: `check` new
  pairs fell 200 → 38 and new clusters 21 → 14, with the suppressed 162 all test↔test.
- **Const seam channel now gated on project-DECLARATION, not a hand-maintained stop-list.**
  A referenced SCREAMING_SNAKE constant only forms a cluster if the corpus actually *declares*
  it — the const analog of the project-defined gate already on the call channel. Std/library
  constants (`O_CREATE`, `RFC3339`, `JSON`, `os.O_*`) are referenced but never declared
  in-project, so they drop out structurally instead of being chased case-by-case in
  `commonConsts`. Extractors now emit each file's module-scope declared constants
  (`decl_consts`) across all four substrates; the touchpoint pass unions them and admits a
  const seam only on a hit. `commonConsts` is kept as a second filter for project-declared but
  generically-named values. Live A/B: two Go repos that previously clustered on std file/time
  constants went to zero const candidates with no recall loss, the sibling Rust codebase's real
  cross-crate geometry twins (`STRAIGHT_EPS`, `STRUCTURE_GROUP`) still surface, and every
  surviving Python candidate resolves to a real in-project declaration. Known limit: a constant
  declared in a file with no functions (a dedicated constants module) contributes no record, so
  its references won't cluster — a recall hole, not a precision one.
- **`propose-roles --judge` — Layer D measurement for the touchpoint detector.** The N-ary
  cluster pass could surface clusters but had no way to *grade* them, so the touchpoint
  detector (and the const-set axis on it) was invisible to `doctor --ablate`. `--judge` now
  adjudicates each cluster with the oracle on its two representative members and records a
  Layer D label tagged `detector=touchpoint`, `variety=<seam channel>` — so the matrix gets
  distinct `touchpoint·consts` vs `touchpoint·calls` cells. `--channel calls|consts|emits`
  focuses a run on one slice; `--twins-only` prints only confirmed twins. Test files are now
  excluded by default (mirroring `propose-deriv`) — test-vs-impl pairs share helper seams and
  cluster as false twins, polluting the corpus. `commonConsts` grew to stop-list Go/JS std
  constants (`O_CREATE`, `RFC3339`, `JSON`) that masquerade as domain vocabulary.
- **Const-set recall axis — cluster functions sharing a domain constant (the "same
  computation, different access pattern" twin).** The N-ary touchpoint pass gains a fourth
  seam channel: referenced SCREAMING_SNAKE domain constants (`V_BELOW`, `STRAIGHT_EPS`). Two
  functions that derive one concept through *inverted* access patterns share no read-set,
  call-set, or write-set — the reads axis is blind to them by construction — but both reference
  the same named magic values, and that shared constant is the only positive signal linking
  them. Extracted across all four substrates (Go, Python, Rust, TypeScript); strongest on
  Rust/Python/TS where UPPER_SNAKE is the const convention (Go's MixedCaps makes its constants
  indistinguishable from types without type resolution, so the channel stays quiet there — an
  intentional asymmetry). A `commonConsts` stop-list drops universal std/library values
  (`TAU`/`NAN`/`MAX`) whose sharing is incidental, mirroring `commonIdents` on the call channel,
  and a first-class `consts:` predicate term lets `propose-roles` re-select const clusters
  exactly. Live on a sibling Rust codebase it cleanly surfaced the cross-crate geometry-split
  twin family (`pose_at` ≟ `pose_at` ≟ `easement_length`, keyed on `STRAIGHT_EPS`). Flows into
  `propose-roles`, `scan`, and `check` for free.
- **`calque confess --judge` — adjudicate the directed twin candidates.** The comment axis
  now has a precision half: `--judge` sends each *directed* candidate (a drift-confessing
  comment that names a resolvable function) to the LLM oracle, prints the verdict, and records
  it to the Layer D label store under a `confession` detector — so the comment axis earns its
  own column in the `doctor --ablate` matrix. Mirrors `propose-deriv`: registry dedup (the
  census still lists every confession), `--top` to bound cost, `--twins-only` to print only
  confirmed twins, and test files excluded by default (`--include-tests` opts back in). The
  bare `confess` (no `--judge`) is unchanged — generator-only, stdout, no exit code.
- **`confess` register discriminator — gate the figurative "prose" register.** A confession's
  source line is now classified `line` (a dedicated `//`/`///`/`//!`/`#` single-line comment —
  the terse register where "mirrors X" is a literal twin-flag) or `prose` (a docstring body or
  block/JSDoc continuation — where "mirrors" is usually the figurative English verb). Directed
  candidates keep the `line` register by default; `--include-prose` opts the figurative register
  back in. The register is tagged into the Layer D matrix variety (`[line]`/`[prose]`), and the
  census annotates each confession with its register. No extractor change — the raw source line
  carries the signal. The matrix this enables shows the real structure: `confession · go · line`
  pulls weight (~0.6) while Python confessions underperform in *both* registers, so the dominant
  axis is language, not register — the prose gate is a defensible noise-reducer (and a no-op on
  Go, which has no docstrings), not a fix for the language-level weakness.

### Fixed
- **Collapsed a real key-cleaning twin the new `confess --judge` caught on calque's own
  code.** `cmd/calque`'s registry pruner had its own byte-identical copy of the registry
  package's key-normalizer (trim whitespace + backticks), neither delegating — the "fix one,
  the twin still has the bug" shape. Exported it as `registry.CleanKey` (the one authority)
  and routed the pruner through it; the duplicate is gone.
- **Python/TS read-sets no longer count a call's leaf name as a field read.** The
  derivation read-set is meant to be the *domain-field footprint* a function reads, but the
  Python and TypeScript extractors recorded a method call's callee path (`road.compute` in
  `self.road.compute()`) as a read — so call vocabulary polluted the signal. The Go extractor
  already skipped this (`calleeSkip`); Python (`callee_skip` keyed by `id(node)`) and TS
  (a `calleeSkip` node set) now match it: the callee is dropped from reads while its receiver
  (`road`) and genuine field reads (`terrain.height`) are kept. Rust was already clean (method
  calls are a distinct AST node, never a field). On a 628-file Python repo this *un-diluted*
  the read-set — removing call-name overlap that was depressing jaccard surfaced 5 additional
  domain-field derivation candidates that sat below the 0.5 threshold (7 → 12) with zero churn
  to the existing pairs, so the fix is a recall gain on Python, not just noise removal.
- **Cluster-seam detector no longer treats std / extern-crate methods as private seams.** The
  N-ary touchpoint detector (`propose-roles`, `cardinality`) pooled any lower-first call name as a
  candidate private seam — but a Rust/Python snake_case stdlib method (`read_to_end`, `parent`,
  `lerp`, `from_xyz`, `sort_unstable`) is not a *shared private* symbol, so unrelated functions
  that merely call the same library method clustered as false roles. A *non-underscore* call name
  now counts as a seam only if it resolves to a project-defined function; leading-underscore names
  (unambiguously project-private — std uses none) still pass unconditionally, and the
  string/write seam channels are unchanged. On a Rust codebase this nearly halved proposed roles
  (38 → 19), and every dropped seam was a genuine std/Bevy/glam method.

### Changed
- **`propose-deriv` gates same-op bare-mutator (`mutate/mutate`) pairs by default.** Two
  functions that both only *write* a shared field-set — neither constructing, measuring, nor
  searching — are co-mutating the same state incidentally, not deriving one value two ways.
  A 7-repo judged corpus localized the read-set's noise to exactly this variety: `mutate/mutate`
  ran 0.36 precision (n=50, the dominant false-twin generator) while `construct/construct` held
  0.50. The pairwise op-type gate now drops `mutate/mutate` candidates unless `--include-mutators`
  opts them back in — the read-set analog of the `confess` prose gate. On a large mixed corpus this
  cut default candidates ~90% (97 → 7), surfacing only the stronger derivation varieties.
- **`propose-deriv` excludes test files by default.** Two test cases intentionally
  exercise the same code and share a read-set (often a reused mock/setup fixture), so they
  cluster as false "twins" — the `doctor --ablate` matrix surfaced a degenerate Python
  test-vs-test cluster that dragged read-set precision to 0.32. The pass now skips the four
  substrates' test conventions by default (`*_test.go`; `test_*.py`/`*_test.py`;
  `*.test.*`/`*.spec.*`; and `tests/`/`test/`/`__tests__/` dirs); `--include-tests` opts back
  in. On a large Go+Python corpus this cut candidates ~55% (215 → 97) and lifted read-set
  precision from 0.32 to 0.81.
- **`propose-deriv` gates whole-struct field shares (structural co-access) by default.** When the
  fields two functions SHARE are a small whole-object field set — a `Pose`'s bare `{x,z,hdg}`, a
  `RoadPiece`'s `{geom,elevation,super}` — the overlap is structural (both merely touch the same
  struct), not value-derivation drift. With no type info, the proxy is: the shared (intersection)
  read-set is small (≤3) and every member is a whole-object field token (a bare leaf, or a TS
  `this.`-prefixed leaf), vs a dotted domain path (`road.width`) which names a specific quantity
  and is kept. Dogfood on a sibling Rust codebase found this single filter killed 3/3 read-set
  false alarms with no loss to the real twins; `--include-structural` opts back in.

## [0.6.0] - 2026-06-16

### Added
- **Operation-type gate (Layer A) — suppress provably-dual derivation candidates.** A coarse
  method-stereotype classifier (`construct`/`measure`/`forward-map`/`inverse-search`/`mutate`,
  from the function's name-role stem + writes/ret shape) gates the derivation pass: two
  functions that read the same fields but perform *opposed* operations — a forward map vs its
  inverse search, or a constructor vs a measure — are not twins, so the pair is dropped. Only
  ever **suppresses** a provably-dual pair (never asserts a twin) and never fires when either
  side is unclassified, so a weak signal can't drop a real twin. Lineage: method stereotypes
  (Dragan/Collard/Maletic), pointed at twin discrimination. Ablatable — Layer D decides if it
  earns its keep.
- **Layer D — `calque doctor --ablate` + a global label store.** A `--judge` run now appends
  each verdict to a global label store (`~/.cache/calque/labels.jsonl`, colocated with the
  judge cache — never written into the scanned repo, so adopters stay read-only), tagging it
  with the detector that surfaced it, the language, and the op-type variety. `doctor --ablate`
  rolls that store into a **per-detector × language × variety matrix** and asks the one
  question that matters: *does each detector pull its weight?* A cell is `pulls-weight`
  (precision ≥ 0.50 with support ≥ 5), `prune?` (below threshold with support — gate harder or
  drop on that slice), or `insufficient` (n < 5 — buy more labels before ruling). This turns
  "which strategies are working" from a hunch into a measurement; re-runs are free (judge disk
  cache), so growing the corpus costs API only on genuinely new pairs.

### Changed
- **Signal taxonomy is table-driven.** The five scoring channels
  (`strings`/`writes`/`name`/`calls`/`ret`) lived in four parallel places — the weight map,
  `scorePair`'s similarity/availability maps, and `Reason`'s render switch — so a new channel
  meant editing all four and `Reason` could silently skip one. They now derive from a single
  `signals` table (`[]signalDef{key,weight,sim,avail,render}`): a channel is one entry.
  Behavior-preserving — the static prior, the fixed (determinism-critical) summation order, and
  every score are unchanged, with the calibration/score/blocking test suites passing untouched.
  Resolves calque's longest-standing self-flagged drift (`Reason ≟ scorePair`).
- **Verdict-class vocabulary named once.** The registry taxonomy
  `drift`/`contracted-twin-ok`/`false-alarm` is now `llm.ClassDrift` / `ClassContractedTwinOK` /
  `ClassFalseAlarm`, referenced at every comparison site instead of bare literals (calque's own
  `doctor --ablate` self-scan flagged the duplication). The judge system prompt and on-disk
  registry strings stay literal — they're the wire format the consts mirror.

## [0.5.0] - 2026-06-15

### Added
- **Value-derivation drift detection — the `reads` axis.** calque now extracts a
  `reads` signal per function (the dotted field-paths a function consumes to derive its
  output), mirroring `writes` on the read side across all four substrates (Go, Python,
  TypeScript, Rust). This closes the dominant real-world miss: *the same physical
  quantity (a height, width, offset, centerline) derived independently in ≥2 places that
  silently diverge* — "fix one path, the twin still has the bug." The invariant tying the
  twins is the **input field-set they both read**, which prior signals
  (`writes`/`calls`/`strings`/`ret_keys`) didn't capture. Grounded in a 2026 prior-art
  sweep (`docs/divergent-implementation-detection.md`) and an adopter's dual-path ledger.
- **`calque propose-deriv`** — a boundary-free, whole-repo generator that surfaces
  functions deriving a value from the same input field-set **without routing through a
  shared authority** (the `SharedDerivationCandidates` pass). Three precision gates: a
  function must read ≥ `--min-reads` fields, must **not delegate** (a twin that forwards
  to a shared authority is the fix, not the drift — `delegates` already encodes that), and
  must actually derive a value (writes or returns something). High recall / low precision;
  `--judge` runs the LLM oracle, or adjudicate by hand. Each candidate carries a
  **collapse-vs-pin lean** (same package → collapse to one authority; cross-package → lean
  to a differential test). This is the standing-audit / batch-cleanup surface the
  boundary-gated `scan`/`check` can't be.
- **Go reads precision: call-callee selectors are excluded.** A package- or method-call
  callee (`exec.LookPath`, `fmt.Errorf`) is a call, not a field read, so it no longer
  pollutes the derivation footprint — only genuine field reads (and a method call's
  *receiver* field) count. (Python/TS have the same call-name-in-reads noise; refining
  them is tracked in the precision backlog. Rust is already clean — its calls are
  structurally distinct from field access.)

- **Confession axis (Layer C) — drift-confessing comments as a twin signal.** Some twins
  announce themselves: a comment saying *"mirrors X"*, *"keep in sync with Y"*, *"must match
  Z"*, *"copy of"*, *"cross-checked against"*. `calque confess` scans each function's body
  **and its doc-comment block** for these self-witness phrases; a directed pair fires only
  when the confessed name is an `identifierLike` token (has `_` or camelCase) that exactly
  names another function — so prose like "drift" or "engine" can't trigger it. A census plus
  directed pairs; cheap, deterministic, and complementary to the read-set recall (it catches
  the copy a maintainer already flagged in a comment but never wired a test to).
- **Windows portability for the Rust extractor.** The cached Rust helper binary is now
  `.exe`-suffixed on Windows (`exeSuffix()` via `runtime.GOOS`), so `.rs` scans work on a
  Windows box, and the `Makefile` gains `windows` (→ `bin/calque.exe`) and `cross` targets.

### Notes
- The `reads` *score channel* (folding reads into the gated pairwise scorer) was
  prototyped and **deferred**: without type resolution, package-constant reads
  (`flag.ContinueOnError`) are indistinguishable from field reads, so it added
  low-score false pairs to the `--strict` gate on every Go repo. The dedicated
  `propose-deriv` pass is immune (high jaccard floor + fanout cap), so `reads` ships
  powering that pass only; the score channel can return later behind a calibration pass.

## [0.4.0] - 2026-06-15

### Changed
- **`check` STALE no longer cries wolf on non-function / excluded keys.** A registry
  entry was flagged STALE whenever a referenced symbol was absent from the extracted
  *function* corpus — but under the canonical `**/*_test.go` exclude that wrongly
  flagged every test-referencing entry, and it would equally mis-flag module-level
  tables and cross-substrate keys calque doesn't extract as functions (calque's own
  scan showed "20 stale" that were all this false class). STALE now requires a key to
  be **provably dead** — its source file is gone (`confidentlyDead`, the same
  soundness `prune` uses for destructive removal). Entries whose symbol is merely
  absent-but-file-present are counted and surfaced as a soft one-line note
  (`StaleAmbig`), not flagged STALE. calque's self-scan went 20 → 0 STALE.

### Added
- **Rust function extraction — the code (scoring) axis now works on `.rs` codebases.**
  Adds Rust as the fourth function-axis substrate (after Go/Python/TypeScript). Unlike
  `.py`/`.ts` (run a script via an always-present interpreter), Rust needs a compiled
  parser, so calque embeds a tiny `syn`-based helper crate
  (`internal/code/rust-extractor/`) and **builds it once, then caches** the binary under
  the user cache dir keyed by a hash of the embedded source — the first `.rs` scan runs
  `cargo build --release --locked` (pinned `syn` via a committed `Cargo.lock`), every
  scan after reuses the cached binary. A real AST (not a brace-scanner) keeps the
  scoring gate's inputs precise: the helper emits the same `FuncSig` JSON the go/ast and
  python3 extractors produce (so `runJSONExtractor` handles all four identically), with
  field semantics mirroring `extract.py` — `writes` strip the `self.` root
  (`self.speed += x` → `speed`), `delegates` detects forwarding through a delegation-root
  field (`self.inner.f()` / `self._engine.step()`), `ret_keys` come from a returned
  struct literal. `cargo` is present wherever Rust is actively developed (you can't build
  the crate without it), so the build-time dependency mirrors the existing
  `node`+`typescript` requirement for the TS leg. `CALQUE_RUST_EXTRACTOR` overrides with
  a prebuilt binary (CI / no-toolchain); the test skips when `cargo` is absent so the
  pure-Go build/CI is unaffected. Cross-substrate Rust tables (const/static/`phf!` maps)
  remain a follow-up.
- **TypeScript module-level table extraction — the cross-substrate axis now works on a
  TS/TSX codebase too.** Mirrors the Go and Python table extractors: the embedded
  `extract_ts.mjs` gains a `symbols` mode that walks module-level `const/let/var X =
  {…}` / `[…]` literals (via the TS compiler API) and emits `table` entities whose
  property names / string elements are their `RetKeys`. Same noise control as the
  Python leg (UPPER-cased name or ≥3 keys), and the function and symbol extractors share
  the temp-script + process setup (`runTSExtractor`, the analogue of `runPyExtractor`).
  The cross-substrate axis now covers Python, Go, and TypeScript, and tables pair
  *across* languages — a TS `HANDLERS` joins a Python `_VERB_TEMPLATES` on their shared
  key set regardless of source language.
- **Go module-level table extraction — the cross-substrate axis now works on a Go
  codebase.** The Python `symbols` mode extracts module-level dict/set/list tables;
  this adds the Go analogue via `go/ast` — package-level `var X = map[…]{…}` /
  `[]string{…}` literals become `table` entities whose keys/elements are their
  `RetKeys` (`extract_go.go`). So a Go project's registries pair against corpus
  shapes and SQL schemas exactly as Python's do (the SQL side already scanned `.go`;
  the corpus `.json` side is language-agnostic — this closes the gap). Mirrors the
  Python noise control (exported-name or ≥3 keys) and shares the per-file batch loop
  with the function extractor (`goBatch`). Verified on a Go repo: `propose-cross`
  now surfaces module-level Go tables that were previously invisible.
- **SQL-schema extractor — the third cross-substrate emitter, closing the
  corpus ↔ database boundary.** The v0.3.0 axis surfaced authored corpus shapes
  and code tables, but a corpus field-set whose code mirror is a *SQL* table (a
  `CREATE TABLE` in a `.sql` file or embedded as a string in source, not a Python
  literal) had no code-side entity to pair against. A tolerant pure-Go
  `CREATE TABLE` scanner (`extract_sql.go`) now extracts each table's COLUMN SET
  as a `sql-table` entity's `RetKeys`, scanning `.sql` plus SQL-bearing source
  (`.py`/`.go`/`.ts`/…) — a file may yield both module-dict and schema entities.
  It strips SQL comments and respects nested type/constraint parens so
  `DECIMAL(10,2)` and `PRIMARY KEY (…)` don't corrupt the column split.
  **Validated read-only**: the corpus-JSON ↔ `db.py` `temporal_markers` pair —
  the documented v0.3.0 boundary — now surfaces (corpus field-set ↔ the SQL
  schema's columns, jaccard 0.62, the schema carrying extra operational columns),
  along with 31 schema-involving candidates total.

## [0.3.0] - 2026-06-10

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
