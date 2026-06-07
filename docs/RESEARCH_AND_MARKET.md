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
- **SPA, "Semantic Inconsistencies in Ported Code" (ASE 2013)** — the v1↔v2 / client↔
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

**Bottom line: the specific niche — recall-first discovery of N-ary behavioral (Type-4)
twins across a whole repo, with a persistent registry tracking drift over time — is
essentially UNOCCUPIED commercially.** But the surrounding territory is filling fast.

| Segment | Tools | Status vs calque |
|---|---|---|
| Token/structural dup (Type 1-3) | SonarQube (≥100 tokens / 10 statements, literals ignored), Codacy (= PMD CPD), CCFinder/NiCad/Deckard | SATURATED; stops at Type-3 by construction. NiCad authors state it *cannot* do Type-4. |
| Pattern/SAST | Semgrep | You hand-write the rule (inverse of discovery — must already know the contract). |
| AI PR review | **Greptile**, CodeRabbit, Qodo, cubic | Cross-file aware but **diff-gated + ephemeral**; no standing whole-repo twin scan, no drift ledger. |
| Exec "AI slop" metric | **Larridin** "AI Slop Index" | Repo-level **score**, not a code locator; not in shipping changelog. |
| Type-4 behavioral discovery | HyClone, SEED, AST+PDG GNN papers | ACADEMIC ONLY — no commercial product. |

**Greptile** (YC W24; $25M Series A, Benchmark; Brex/Substack/PostHog) — closest
*capability*. v3 (Nov 2025) genuinely catches cross-path drift: "searches the entire
codebase for similar logic," textbook stale-`applyProration` example. **But every catch
is diff-anchored** (only when a PR edits one twin); no standing repo-wide twin
enumeration, no persistent ledger of unresolved divergences. Persists a graph index +
learned *style* rules, not drift findings. $30/seat/mo; free for OSS. *The clean
differentiator is trigger model: diff-gated vs standing.*

**Larridin** (founded 2024; $17M seed, a16z) — closest *positioning* ("AI Slop Index").
But it's an enterprise **AI-adoption/spend-telemetry** platform; the Slop Index is a
**management composite score** (per-commit/dev/team rollup) that *explicitly does not
locate the offending code* ("which teams, which codebases, which time periods"), and it
is **absent from the shipping changelog** (28 features, none touch source) — pre-product
marketing. Sales-led, ~$50–500K/yr. Different product, different buyer (execs not devs).

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
- **arXiv 2603.28592 (Mar 2026)** — 304,362 AI commits / 6,275 repos / 5 tools: **>15%
  of AI commits introduce ≥1 issue** (17.3% Copilot → 28.7% Gemini); code smells = 89.1%.
  Caveat (their own): no human baseline.
- **He et al. (Cursor DiD)** — ~**+41% complexity**, +30% static warnings; transient
  velocity gain, persistent debt. *Most causal-leaning design.*
- **Stack Overflow 2025** (33,662 devs) — **66% frustrated by "AI solutions almost right,
  but not quite"**; 84% adopt but only ~33% trust accuracy. *The single best on-thesis
  one-liner — that's the Type-4 silent-drift failure mode, independently measured.*
- **GitClear 2025** (211M LOC) — copy-paste 8.3%→12.3%, "moved"/refactored <10% for the
  first time, dup blocks 4–8×. *Punchy but vendor + correlational + version-inconsistent
  → cite paired with the independents.*
- **"slop" = Merriam-Webster 2025 Word of the Year** (Dec 15, 2025) — cultural marker,
  but about *content* not code; use as flavor, not evidence.

---

## 4. Go/no-go verdict

The competitive question isn't "can we beat Greptile" — it's **"is there a tool to adopt
instead of building?"** Answer: **no.** The only close thing (Greptile) is closed-source,
diff-gated, per-seat SaaS that doesn't do standing whole-repo N-ary twin discovery with a
registry. Nothing off-the-shelf does what the 6-repo survey just proved is needed.

**GO — build for own-dogfooding + open-source (Apache-2.0), keep it sharp.** Do *not*
turn it into an AI-review product (crowded, well-funded, and not for sale anyway). Build
only the part nobody else does and that the survey + lit-review converge on:
1. **Standing whole-repo recall** (not diff-gated) — the Greptile gap.
2. **Persistent adjudicated registry** — zero commercial instances; the durable moat.
3. **In-the-agent-build-loop integration** — attacks the root cause (memoryless local
   generation) at generation time. The real differentiator; also the answer to "how
   should calque run automatically."

Risk (downhill from three directions) only matters for a product play, which this isn't.
