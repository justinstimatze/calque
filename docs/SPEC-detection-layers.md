# Spec — detection layers v2 (precision gate + two recall axes + ablation harness)

The next increment after the `reads` axis (`docs/SPEC-reads-axis.md`). Adds three cheap,
deterministic layers and the quantitative harness to decide whether each earns its place.
Grounded in the **first live validation** (camber, 2026-06-16): `propose-deriv` surfaced 5
candidates → **2 real / 3 false**, then the collapse of the real one (#5) surfaced a **third
copy the reads axis missed**. Each layer below is justified by a specific one of those data
points, and the empirical target for each is stated so it can be measured, not asserted.

Vocabulary unchanged: *implementation drift* / *divergent reimplementation* (Shape A);
*divergent twin / cluster* (instance). See `divergent-implementation-detection.md`.

Hybrid-loop framing: **typed substrate → cheap recall → deterministic gates → LLM judge →
behavioral confirm → calibration that prunes.** These layers are the cheap-recall and
deterministic-gate tiers; the judge/confirm tiers already exist (`--judge`).

---

## Layer A — operation-type discriminator (precision; the top-ranked layer)

**Why:** all 3 false alarms shared a read-set but did *opposite* operations — `continuity`
*measures* a residual vs `spiral_ending_at` *constructs* a clothoid; `RoadPiece.sample`
*forward-evaluates* (s→state) vs `RoadPiece.project` *inverse-searches* (world→s). Method
**stereotypes** (Dragan/Collard/Maletic, ICSM 2006/2011 — computable by light static
analysis, shipped as Stereocode, *never applied to clone detection*) are exactly this axis.

**The classifier** — `opType(f *FuncSig) string` in Go, derived from signals `FuncSig`
already carries (no extractor changes). Returns one of:

| op-type | cheap signal |
|---|---|
| `construct` | non-empty `RetKeys` (returns a composite/struct literal) |
| `measure` | empty `Writes` + empty `RetKeys` + returns a scalar/bool; name-stem ∈ {distance, residual, error, deviation, continuity, count, area, score, …} |
| `forward-map` | name-stem ∈ {sample, eval, at, interpolate, transform, apply, map, project_forward} |
| `inverse-search` | name-stem ∈ {project, search, find, nearest, closest, locate, bisect, lookup, invert, solve} |
| `mutate` | non-empty `Writes`, no `RetKeys` |
| `other` | default (unknown — never gates) |

**The gate is NARROW and surgical — suppress only provably-DUAL pairs.** This is the
load-bearing design decision: a hard whole-function op-type match would wrongly kill #5 (a
*real* twin whose two sides are `measure` (audit) vs `mutate/predicate` (classify) but share
an inner predicate). So suppress a derivation candidate **only** when the pair is one of the
two *opposed/dual* combinations — where there is provably no shared sub-computation:

- `forward-map` ↔ `inverse-search` (dual operations)
- `construct` ↔ `measure` (build vs quantify)

Everything else (same type, or any pairing with `mutate`/`predicate`/`other`) passes. Worked
against the live data:

| pair | op-types | opposed? | outcome | correct? |
|---|---|---|---|---|
| #1 sample ≟ sample | forward / forward | no | keep | ✅ real |
| #2 continuity ≟ spiral_ending_at | measure / construct | **yes** | suppress | ✅ false |
| #3 project ≟ sample | inverse / forward | **yes** | suppress | ✅ false |
| #4 sample ≟ project | forward / inverse | **yes** | suppress | ✅ false |
| #5 building_audit ≟ auto_classify | measure / mutate | no | keep | ✅ real |

**Empirical target: 5 candidates → 2 (both real). Precision 40% → 100%, recall on the real
twins unchanged.** Wire as a filter inside `SharedDerivationCandidates` (drop opposed pairs)
*and* surface the op-type pair in the candidate `Sig` so the judge sees it. Name-stem lists
live as small `set`s next to `delegationRoots` in `funcsig.go`. **Ablatable** — if `doctor
--ablate` shows it never flips a verdict on the corpus, cut it.

---

## Layer B — shared-named-constants recall axis (covers reads' blind spot)

**Why:** the third building-span copy (`trim_buildings_off_carriageways`) shares the *derived
concept* — the `V_BELOW`/`V_ROOF` constants + base/top span math — but has an **inverted
broadphase** (segment-grid vs building-grid), so its read-set differs and `reads` could not
pair it. Functions referencing the **same domain constants** are likely twins even when their
data-flow shape diverges. Orthogonal to reads.

- **Extract** a new `Consts []string` on `FuncSig` (+ `sConst set`, one line in `Prepare`):
  identifier references that look like named constants — `SCREAMING_SNAKE_CASE` idents in the
  body (cheapest cross-substrate proxy; a const-resolution pass is a later refinement). Mirror
  the `reads` collector site in each extractor; emit `consts` in the JSON. Fanout cap drops
  ubiquitous constants (`MAX`, `PI`).
- **Recall pass** `SharedConstCandidates(sigs, minConsts, minJaccard, maxFanout)` — a clone of
  `SharedDerivationCandidates` over `sConst`, same `!Delegates` gate, **no** writes/ret gate
  (a const-sharer need not write). Emit `SigCandidate{Kind:"const-set"}`.
- Feed it into `propose-deriv` (union with the read-set candidates, dedup vs registry) and let
  the op-type gate (Layer A) prune it too.

**Empirical target: surfaces the third building-span copy** (`trim` ≟ the other two on
`{V_BELOW, V_ROOF}`) that the read-set pass missed. Measure its *marginal* recall (candidates
only this axis finds) on the corpus; if it adds nothing beyond reads, cut it.

---

## Layer C — drift-confessing-comment recall axis (best precision/cost in the stack)

**Why:** *"mirrors the audit exactly"* appeared in **2 of the 3** building-span copies — a
literal self-confession of a twin. The 2026 lit review found **nobody** using self-witness
comments as a clone signal. Dirt-cheap (regex over source), high-precision, novel.

- **No extractor change.** A Go-side pass `ConfessionCandidates(sigs, repo)` scans each
  `FuncSig`'s source span (reuse `File`+`Line`+`NLines`, same line-read as `readFuncSource`)
  for confession phrases: `mirror(s)?`, `same as`, `keep in sync`, `kept consistent`,
  `must match`, `must agree`, `in lockstep`, `parallel to`, `duplicate of`, `copy of`,
  `cross-check(ed)?`, `single source`. (Compile one `regexp` alternation; case-insensitive.)
- A confessing function is a candidate on its own. Two modes:
  1. **Directed** — if the comment names a symbol that resolves to another `FuncSig`, emit
     that exact pair `SigCandidate{Kind:"confession", Sig:"comment: <phrase>"}` (highest
     precision — the code told us its twin).
  2. **Undirected** — otherwise surface the confessor as a single-sided candidate (the judge
     / human finds the other side), or pair it with same-stem / shared-const functions.
- This is also a standing **audit** in its own right: `calque confess` could list every
  drift-confessing comment in a repo (a census of self-declared twins to verify in sync) —
  independent of the derivation machinery.

**Empirical target: 2 of the 3 building-span copies self-confess** → near-zero-cost recall of
a known-hard cluster. Precision should be very high (people rarely write "mirrors X" falsely);
measure false-positive rate on the corpus.

---

## Layer D — the ablation harness (`doctor --ablate`) — so we drop dead layers

The whole point of v2 is to **decide layers on numbers, not vibes.** The adjudicated registry
IS the labeled corpus (`drift` / `contracted-twin-ok` = true positives; `false-alarm` = true
negatives). `doctor` already rolls up fire-rate / hit-rate and `calibrate` reweights from
labels; extend it with per-layer ablation.

- **`calque doctor --ablate`**: for each recall axis (reads / const-set / confession /
  name-stem / signature) and the op-type gate, run the candidate pipeline **with the layer on
  vs off** and report against the registry labels:
  - **candidates generated**, **precision** (fraction matching a non-`false-alarm` label),
  - **marginal recall** — true-positive pairs surfaced *only* by this layer (its unique value),
  - **decision flips** — verdicts that change when the layer is toggled. **A layer with zero
    decision flips is dead weight → drop it**, regardless of how clever it is.
- Seed corpus = camber's 5 verdicts + the third copy + calque's self-scan registry; grow it as
  `propose-deriv --judge` runs accumulate verdicts. ~50–100 labeled pairs for stable per-layer
  precision; log the n so a small-sample number isn't over-trusted.
- First ablation result is already known from camber's 5 and belongs in the harness as the
  regression baseline: **reads-alone = 40% precision / 100% recall-on-real; + op-type gate =
  100% / 100%.** That single row justifies Layer A and is the template for every later row.

---

## Build order
1. **Layer A** (op-type classifier + opposed-pair suppression in `SharedDerivationCandidates`
   + op-type in the `Sig`). Pure Go, no extractor changes. Highest rank, cheapest, biggest
   measured lift. Ship first.
2. **Layer C** (confession pass — Go-only source scan; the directed-pair mode). Cheap, novel,
   high precision; no extractor changes.
3. **Layer D** (`doctor --ablate`) — stand it up early so Layers B/E are decided on numbers.
4. **Layer B** (const extraction across 4 substrates + `SharedConstCandidates`) — the one with
   extractor changes; gate its keep/cut on the harness.
5. Re-validate on camber read-only: target 5→2 (Layer A), + the third copy surfaced (Layers
   B/C), all real twins kept.

## Out of scope (later, gated on the harness plateauing)
Drift-robust embedding recall (CodeSage/CodeBERT local — the ML recall layer); PDG/data-flow
GNN; perplexity-as-similarity; behavioral differential probe (auto-generated from the shared
read-set — strongest confirm where code is executable). These are the heavier tiers; the cheap
deterministic layers above are measured first, and only the gaps they leave justify the ML.
