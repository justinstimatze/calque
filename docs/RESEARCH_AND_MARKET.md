# calque — prior art, competitive landscape, market (primary-sourced, 2026-06-06)

Extends DESIGN_NOTES §6 (which covers GPTCloneBench, CETBench, behavioral/Type-4
clones, HyClone, differential fuzzing). This doc adds: (1) calque's true intellectual
lineage; (2) a primary-source competitive scan; (3) verified market evidence; (4) the
go/no-go verdict. All claims below are from primary sources (product docs, papers,
funding pages).

---

## 1. Calque's real lineage — inconsistency bugs, NOT clone detection

The closest ancestry isn't clone detection; it's the "code that should agree but
doesn't" lineage. The prior survey missed this and it sharpens the framing.

- **Engler et al., "Bugs as Deviant Behavior" (SOSP 2001)** — infers implicit
  programmer beliefs from the corpus, ranks contradictions as bugs; no a-priori spec.
  *calque's founding stance, 25 years early.* calque differs: effect/private-symbol
  signatures + LLM oracle, not call-pair templates + statistics.
- **CP-Miner (OSDI 2004)** — frequent-subsequence mining of copy-paste; flags forgotten
  renames. Sub-function + bug-oriented like calque, but lexical → blind to Type-4.
- **DejaVu (OOPSLA 2010)** — N-ary groups of similar-but-not-identical fragments,
  recall-then-classify on 75M LOC. *Nearest architectural precedent for calque's N-ary
  clustering + adjudicate split.* But syntactic similarity + heuristic classifier.
- **FICS, "Finding bugs using your own code" (USENIX Security 2021)** — *unsupervised,
  single-codebase*: cluster functionally-similar fragments via data-flow features, flag
  the deviant one. **Strongest direct precedent for calque's whole design.** Contrasts
  calque can claim: FICS is PDG-based (heavy, C-centric), bug-hunting not drift-hunting,
  **no persistent memory** (re-runs from scratch), **no N-ary registry, no LLM oracle.**
- **SPA, "Detecting and Characterizing Semantic Inconsistencies in Ported Code" (ASE 2013)** — the v1↔v2 / client↔
  server case, but assumes a known port relationship; calque discovers it.
- **Clone-group co-evolution** (Bettenburg WCRE 2009; JSS 2017): empirical proof that
  clone *groups* drift via inconsistent edits — justifies calque's metabolism layer + the
  group-as-unit-of-memory. **ICCheck (arXiv 2504.04537, 2025)** keeps known clones in
  sync — the only precedent touching *persistence*, but research-only and operates on
  already-identified clones, not discovery.

**calque's defensible novelty = four moves none of the above combine:** static
effect/private-symbol signal · N-ary group unit · persistent adjudicated registry ·
generated antibodies — spanning **both code and config substrates** (§14+§15).

**Five gaps calque could own:** (1) Type-4 + N>2 + sub-function-block together on a live
unlabeled repo; (2) the rarity-weighted private-symbol-touchpoint signal (unclaimed);
(3) persistent adjudicated memory across runs; (4) registry→executable antibody;
(5) unifying code-drift + config/env-parity under one registry shape.

---

## 2. Competitive landscape (primary-sourced)

> **⚠ Correction (2026-06-08) — the "UNOCCUPIED" verdict below is STALE.** This section
> is the 2026-06-06 primary-sourced snapshot, kept as a record. Between then and going
> public the category was *named* and *occupied*: "**architecture drift**" / "**fitness
> functions**" is now established industry vocabulary with institutional backing
> (Thoughtworks Technology Radar lists *Architecture drift reduction with LLMs* as a
> technique), and a wave of tools moved in — most directly **`sauremilk/drift`** (MIT):
> AST-level near-duplicate detection ("mutant duplicates" / "pattern fragmentation"), 24
> cross-file signals, undeclared-dup discovery + declared boundaries, a feedback/
> calibration loop reweighted on git outcomes, and a CI gate — i.e. close to calque's
> whole spine, code-side, with a more mature calibration loop and partial TypeScript.
> Also in-category: CodeAnt AI, `@aiready/pattern-detect`, similarity-ts, CloneDR.
>
> **What still holds** (the defensible ground — see DESIGN_NOTES §6 for the full
> correction): calque is **not first-to-category** and must stop implying it. Its edge
> is (1) **Type-4-by-construction** — the drift wave indexes *structural* AST similarity
> (Type 1–3) and is blind to syntactically-dissimilar twins (a stub vs the real path;
> two different-API backends), which is calque's effect-footprint slice; (2)
> **role-cardinality / declare-and-gate** (DESIGN_NOTES §18) — a targeted search found
> *no* tool asserting "this role has one canonical implementation"; genuinely
> unoccupied, and field-validated; (3) **substrate-generality** (code + prose + planned
> axes; the competitors are code-only). This is a *positioning* correction, not a thesis
> change. The original 2026-06-06 analysis follows, unedited.

**Bottom line: the specific niche — recall-first discovery of N-ary behavioral (Type-4)
twins across a whole repo, with a persistent registry tracking drift over time — is
essentially UNOCCUPIED commercially.** But the surrounding territory is filling fast.

| Segment | Tools | Status vs calque |
|---|---|---|
| Token/structural dup (Type 1-3) | SonarQube (≥100 tokens / 10 statements, literals ignored), Codacy (= PMD CPD), CCFinder/NiCad/Deckard | SATURATED; stops at Type-3 by construction. NiCad authors state it *cannot* do Type-4. |
| Pattern/SAST | Semgrep | You hand-write the rule (inverse of discovery — must already know the contract). |
| AI PR review | **Greptile**, CodeRabbit, Qodo, cubic | Cross-file aware but **diff-gated + ephemeral**; no standing whole-repo twin scan, no drift ledger. |
| Exec "AI slop" metric | **Larridin** "AI Slop Index" | Overlapping drift signals (duplication, architectural divergence), but rolls up to a team/commit **score** — doesn't localize the pair/cluster to fix, no registry. |
| Type-4 behavioral discovery | HyClone, SEED, AST+PDG GNN papers | ACADEMIC ONLY — no commercial product. |

**Greptile** (YC W24; $25M Series A, Benchmark; Brex/Substack/PostHog) — closest
*capability*. v3 (Sep 2025, shipped with the Series A) genuinely catches cross-path drift: "searches the entire
codebase for similar logic," textbook stale-`applyProration` example. **But every catch
is diff-anchored** (only when a PR edits one twin); no standing repo-wide twin
enumeration, no persistent ledger of unresolved divergences. Persists a graph index +
learned *style* rules, not drift findings. $30/seat/mo; free for OSS. *The clean
differentiator is trigger model: diff-gated vs standing.*

**Larridin** (founded 2024; $17M seed led by a16z) — closest *positioning* ("AI Slop
Index"), and the conceptual overlap is real: the Slop Index scores repos on **semantic
duplication, architectural coherence/divergence, revert-churn, and whether tests mirror
implementation** — the same drift signals calque hunts. The difference is **granularity
and output**: Larridin rolls those signals up into a per-commit / per-developer / per-team
score and weekly/monthly trends, to guide *management* intervention ("which teams, which
codebases, which periods"); it does **not localize to the specific file, function, or pair
to fix**, and carries no adjudicated registry. calque's output is the inverse — a located,
adjudicable pair/cluster a developer collapses to a single source, remembered across runs.
Enterprise, sales-led; different buyer (execs vs devs).

**CodeRabbit** (Multi-Repo, Feb 2026) — cross-repo *dependency/API* drift on the PR diff;
PR-scoped, not behavioral-twin discovery, no registry. **Qodo** — "duplication +
architectural drift" but marketing-grade, PR-shaped. **cubic.dev** — blast-radius
(one source → consumers), the *inverse* of twin discovery.

**Two genuine moats remain:** (1) N-ary behavioral-twin discovery as the primary product
output (a located, adjudicable pair/set) vs everyone's PR-bug-finding or exec dashboard;
(2) the **persistent registry tracking drift over time** — zero commercial instances.
**Honest risk:** "open today but downhill from three well-funded directions" — Greptile /
CodeRabbit / Qodo have the graph + LLM-equivalence substrate; behavioral-twin detection
is "a feature away, not a research program." Defensibility is the *workflow* (recall +
registry + build-loop), not "we can spot a twin."

---

## 3. Market tailwind — real and citable (lead with independents, not GitClear)

- **DORA 2024 (Google)** — 25% AI-adoption increase → **−7.2% delivery stability**,
  −1.5% throughput (also +3.4% code quality, +7.5% docs — nuanced). Root cause framed as
  larger batch sizes. *Strongest independent source.* dora.dev/research/2024.
- **arXiv 2603.28592 (Mar 2026), *Debt Behind the AI Boom*** — ~302.6k AI-authored
  commits / 6,299 repos / 5 tools: **>15% of commits from every AI assistant introduce
  ≥1 issue**; code smells in 89.3%.
  Caveat (their own): no human baseline.
- **Cursor difference-in-differences study** (arXiv 2511.04427, *Does AI-Assisted
  Coding Deliver?*) — ~**+41% complexity**, +30% static warnings; transient velocity
  gain, persistent debt. *Most causal-leaning design.*
- **Stack Overflow 2025** (33,662 devs) — **66% frustrated by "AI solutions almost right,
  but not quite"**; 84% adopt but only ~33% trust accuracy. *The single best on-thesis
  one-liner — that's the Type-4 silent-drift failure mode, independently measured.*
- **GitClear 2025** (211M LOC) — copy-paste 8.3%→12.3%, "moved"/refactored <10% for the
  first time, dup blocks 4–8×. *Punchy but vendor + correlational + version-inconsistent
  → cite paired with the independents.*
- **"slop" = Merriam-Webster 2025 Word of the Year** (December 2025) — cultural marker,
  but about *content* not code; use as flavor, not evidence.

---

## 4. Go/no-go verdict

The competitive question isn't "can we beat Greptile" — it's **"is there a tool to adopt
instead of building?"** Answer (2026-06-06): no. *Updated 2026-06-08:* `sauremilk/drift`
(MIT) now covers much of the similarity-based code axis off-the-shelf, so the honest
answer is **"partly"** — for AST-near-duplicate / pattern-fragmentation detection there
is now a real OSS tool to adopt or learn from. What still has no off-the-shelf option is
calque's distinctive bet: Type-4-by-construction recall (effect footprints, not AST
shape) and the **role-cardinality declare-and-gate** axis (§2 correction; DESIGN_NOTES
§18). Build *that*, and treat `drift` as prior art for the similarity axis.

**GO — build for own-dogfooding + open-source (Apache-2.0), keep it sharp.** Do *not*
turn it into an AI-review product (crowded, well-funded, and not for sale anyway). Build
only the part nobody else does and that the survey + lit-review converge on:
1. **Standing whole-repo recall** (not diff-gated) — the Greptile gap.
2. **Persistent adjudicated registry** — zero commercial instances; the durable moat.
3. **In-the-agent-build-loop integration** — attacks the root cause (memoryless local
   generation) at generation time. The real differentiator; also the answer to "how
   should calque run automatically."

Risk (downhill from three directions) only matters for a product play, which this isn't.
