# Spec — the `reads` axis (value-derivation drift detection)

Implements roadmap item #0 of `divergent-implementation-detection.md`. Grounded in an
adopter's own dual-path ledger, whose every entry is one shape: **the same physical
quantity derived independently in ≥2 places.** The invariant tying the twins is *the set of
domain input field-paths they both READ to derive an output* — a signal calque does not
currently extract. The adopter hand-wrote the detection rule; this spec implements it.

> *flag any two functions that both read the same `Road`/`Terrain`/`Junction` field set
> to derive a position/height/offset, where **neither delegates to a shared function**.*

Vocabulary (per project alignment): the phenomenon is **implementation drift**; the
structure is a **divergent reimplementation**; an instance is a **divergent twin** (N=2) or
**divergent cluster** (N≥2). "dual-path"/"multi-path" are colloquial aliases for the same
thing. Do **not** call it a "Type-4 clone" outside the "why clone tools miss this" contrast.

## 1. Data model — add `reads` to the extractor interchange

`FuncSig` (`internal/code/funcsig.go`) gains one interchange field and one derived set:

```go
Reads []string `json:"reads"` // dotted field/attr READ paths consumed to derive output
// …
sStr, sWrite, sRet, sCall, sRead, stem set   // add sRead
```

`Prepare` (`funcsig.go`) gains one line:

```go
f.sRead = toSet(f.Reads)
```

`reads` is the symmetric twin of `writes`: same dotted-path convention (root object dropped,
index access → `[]` suffix), same sorted+deduped set. Backward compatible — an extractor that
omits `reads` yields `nil`, all existing passes unaffected.

## 2. Extraction rule (uniform across all four substrates)

**Definition.** `reads` = every dotted field-path that appears in a **value position**,
where a value position is anywhere *except* the immediate LHS of a plain `=` assignment.
Compound assignments (`x += y`) and inc/dec are value positions (read-modify-write), so
their target IS a read. Call **receivers** are reads (`self.road.width()` reads `road`).
Conditions, index expressions, and RHS field accesses are reads. Bare locals (no field
access, base not a plain identifier) are not recorded — same filter `writes` already applies.

Equivalently: `reads := {all dotted field-paths in the body} − {paths that are exclusively
plain-'=' assignment targets}`. Mirror each extractor's existing `writes` collector:

- **Go** (`extract_go.go`, `goBody.Visit`): add an `*ast.SelectorExpr` read case recording
  `attrPath(sel)`; collect the LHS selectors of plain `*ast.AssignStmt` with `token.ASSIGN`
  into a `pureWrites` set and subtract at emit. `attrPath`/`recordTarget` already exist and
  are reused verbatim. Call receivers: `fn.X` of a `*ast.SelectorExpr` call already routes
  through Visit, so its field-path is captured for free.
- **Python** (`extract_py.go` embedded script): cleanest — Python's `ast.Attribute` carries
  `ctx`. `reads` = `_attr_path(node)` for every `ast.Attribute` with `isinstance(node.ctx,
  ast.Load)`; `Store`/`Del` ctx is the write side (already handled). No subtraction needed.
- **TypeScript** (`extract_ts.mjs`): record `PropertyAccessExpression` paths; exclude the
  `.left` of a `BinaryExpression` whose operator is `=` (`EqualsToken`); keep `+=` etc.
- **Rust** (`rust-extractor/src/main.rs`): add `reads: BTreeSet<String>` to `Body`; in
  `visit_expr`, on `Expr::Field` insert `field_members(node)?.join(".")`; collect plain
  `Expr::Assign` LHS paths into a `pure_writes` set and subtract before emit. (`Expr::Binary`
  compound-assign targets stay, since `is_assign_op` ones are read-modify-write.) Add `reads`
  to `Record` + the `emit_fn` projection. Bump the embedded-source hash → the build-once
  cache rebuilds the helper automatically (no manual cache bust).

**Parity test fixture** (one per substrate, reuse the `extract_*_test.go` harness): a method
that reads `self.road.width_m` + `self.road.pieces` and returns a value must emit
`reads ⊇ {road.width_m, road.pieces}`; a plain `self.x = 1` must NOT put `x` in reads;
`self.n += 1` MUST put `n` in reads.

## 3. The recall pass — `SharedDerivationCandidates` (boundary-free, whole-repo)

New function in `sigcluster.go`, structurally a clone of `KeySetCandidates` (same inverted
index + fanout cap + descending-jaccard rank), but over `sRead` and gated for the
derivation shape. **This is the whole-repo batch-cleanup feature** — it runs over the entire
corpus with no `--left`/`--right` boundary.

```go
// SharedDerivationCandidates surfaces VALUE-DERIVATION DRIFT: functions that derive an
// output from the SAME input field-set without routing through a shared authority — the
// dual-path shape where one physical quantity is computed independently in ≥2 places.
// Boundary-free (whole-corpus), so it serves the standing-audit / batch-cleanup use case
// the gated pairwise scorer cannot. High recall, low precision; the judge is the filter.
func SharedDerivationCandidates(sigs []*FuncSig, minReads int, minJaccard float64, maxFanout int) []SigCandidate
```

Gates (the precision levers — all three are load-bearing):

1. **Eligibility:** keep a `FuncSig` only if `len(f.sRead) >= minReads` **and** `!f.Delegates`
   **and** `(len(f.sWrite) > 0 || len(f.sRet) > 0)`. The `!Delegates` clause IS the adopter's
   discriminator — their non-diverging items are non-diverging *precisely because* both route
   through one shared `Poly3`/`height_at`; `delegates` already encodes that. The
   `writes||ret` clause requires the function to actually *derive a value* (a pure reader that
   logs `road.width` is not a derivation).
2. **Fanout cap:** a read-path shared by `> maxFanout` functions (`id`, `name`, `x`, `z`) is
   plumbing — drop it as a JOIN path (but compute jaccard over the FULL read-set, so a real
   twin still surfaces through a rarer shared field). Identical to `KeySetCandidates`.
3. **Jaccard floor:** `jaccard(a.sRead, b.sRead) >= minJaccard`.

Emit `SigCandidate{Kind:"read-set", Sig: fmt.Sprintf("reads≈%.2f {%s}", j, sharedReadsLabel(a,b)), Jaccard:j, CrossFile:…}`.
Rank descending by jaccard (match strength), then cross-file, then deterministic pairkey —
copy `KeySetCandidates`'s sort verbatim. Add a `sharedReadsLabel` mirroring `sharedKeysLabel`.

## 4. Pairwise scorer — `reads` as a corroborating (non-anchoring) channel

In `score.go`, add `reads` to `weights`, `channelOrder`, the `sig`/`avail` maps in
`scorePair`, and `Reason`. **Critical:** reads are noisier than writes (every method reads
many fields), so `reads` must **corroborate, never anchor** — do NOT add it to the
`hasAnchor` disjunction. Two functions sharing only read-paths and nothing else stay out of
the gated pass (they are caught, correctly, by the dedicated `SharedDerivationCandidates`
pass with its derivation gates). Suggested prior weight `reads: 0.12`, rebalanced so the
vector still sums sensibly (e.g. trim `calls` to `0.08`); `CalibrateWeights` tunes from there.
Keep the deterministic fixed-`channelOrder` summation — same reproducibility invariant.

## 5. CLI wiring

- **New subcommand `propose-deriv`** (`cmd/calque/propose_deriv.go`), modeled on
  `propose_cross.go`: extract all function `FuncSig`s across the repo, call
  `SharedDerivationCandidates`, dedup vs the registry, hand survivors to the judge, print
  ranked divergent-derivation clusters. Flags: `--min-reads`, `--read-jac`, `--max-fanout`,
  `--repo`, `--exclude` (match the existing flag vocabulary).
- **Ungated `scan` (roadmap #1):** when `scan`/`check` are run with no `--left`/`--right`,
  run `SharedDerivationCandidates` (and the other boundary-free passes) over the whole
  corpus. This is the standing-audit / "fix didn't stick" safety net — it scans where you
  did not point. Gate `--strict` on it like the others; registry verdicts unchanged
  (`drift` / `contracted-twin-ok` / `false-alarm`).

## 6. Precision budget & calibration

Expect the same recall-first profile as `KeySetCandidates` (~30% real, misses little). The
judge is the precision stage — keep its prompt **terse** (the 2026 overcorrection finding:
verbose "explain the difference" prompts make an LLM equivalence judge *worse*; it already
over-flags real twins as divergent). The `!Delegates` + `writes||ret` gates do most of the
structural filtering before the judge ever runs.

## 7. Phase 2 (separate change) — NL-summary embedding recall

The read-set pass catches twins that share *field names*. It misses twins that derive the
same quantity from *differently-named* fields (cross-substrate, or post-rename). To close
that: generate a one-line NL summary per function, embed it, cluster by cosine — a second
recall source feeding the same judge. (Concept independently reimplemented from public
Greptile engineering posts on function-granular NL-summary embeddings; no code or example
fixtures lifted — IP read: clean, no patents.) calque already has an embedding client
(`internal/embed`); this reuses it. Deferred — the deterministic read-set pass ships first.

## 8. Self-gate

Adding `reads` makes calque self-scan its own derivation clusters. Build, run
`calque check --repo . --exclude 'legacy/**,**/*_test.go' --strict`, and adjudicate any new
read-set clusters in `.calque/registry.md` (expect `false-alarm` for incidental shared-field
coincidences across the extractors, `contracted-twin-ok` for genuine cross-substrate parity).
`gofmt -l .` clean; `go test ./internal/code/ ./cmd/calque/` green; the Rust fixture skips
when `cargo` is absent (pure-Go CI stays green).

## Build order
1. `FuncSig.Reads` + `Prepare` + per-substrate extraction + parity fixtures (the data).
2. `SharedDerivationCandidates` + `sharedReadsLabel` + `sigcluster_test.go` cases (the recall).
3. `propose-deriv` + ungated-`scan` wiring (the surface — batch cleanup).
4. `reads` corroborating channel in `score.go` (the gated-pass lift).
5. Self-gate adjudication. Phase 2 (NL embeddings) is a later, separate change.
