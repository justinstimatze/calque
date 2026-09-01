# Divergent-implementation detection — terminology, prior art, and the gap calque fills

*Findings from a 2026 literature + SOTA sweep (academic Type-4/equivalence, industry
review/clone tooling, and the omission/deviant-behavior subfield), grounded on real
recurring bugs in a sibling Rust codebase.*

**Scope.** calque is general-purpose; this is a general code-maintenance phenomenon. The
prior art below is overwhelmingly *general systems code* (Linux-kernel bugs, API misuse,
language-agnostic clone evolution) — nothing here is domain-specific. Two large,
stateful, agent-edited game codebases are simply the **dogfood corpus** where the problem
(especially Shape B) shows up densely enough to study; both shapes occur in web backends,
serializers/parsers, harness-vs-prod, and any client/server split just as readily.

## The pain that started this

A single behavioral contract gets reimplemented in N places. Over time the copies
silently drift — someone fixes a bug in one, the twin still has it. The fix *looks*
done (the path you were staring at works) but isn't, because a second path was never
touched. This recurred roughly half a dozen times on one game project and ~a dozen on
another. It is the single biggest source of friction in large agent-assisted codebases.

Two concrete instances from the sibling codebase, which turn out to be **two
structurally different bug shapes**:

- **Shape A — divergent reimplementation.** *"Two divergent code paths computed spawn
  height: the launch path used raw terrain height while the cycler used
  `waypoints::compute`."* One contract, two bodies, drifted. Fixed by unifying both
  behind one function. This is calque's textbook target.

- **Shape B — precondition omission across sibling call sites.** A teleport path zeroed
  velocity but **forgot to reset** per-wheel ground-contact state, while the launch path
  gets fresh (NaN) state implicitly. The suspension then latched onto a stale surface and
  dropped the car in midair. *"Per-waypoint headless tests passed while interactive
  cycling failed."* There is **no duplicated body** here — the bug is an **omission**: one
  caller fails to establish a precondition the others satisfy.

Shape B is why "fix it again" keeps not sticking, and it is the one calque structurally
**cannot see today** (see Diagnosis).

## Terminology — there is no single agreed term, and the one we use is mispositioning us

The phenomenon is named differently in each subfield, and **the name you pick aligns you
with a neighborhood**. calque's README currently calls this a *"Type-4 (behavioral)
clone."* That is the **similarity** framing — and it points at the wrong neighbors. Clone
detection answers *"what is still the same?"*; its definition of success is that two copies
**are** alike. calque wants the **inverse**: code that was *supposed* to be the same and
**isn't**. A drifted near-miss is exactly what a clone detector is biased to drop from its
report.

| Term | Subfield / source | What it actually names | Fit for us |
|---|---|---|---|
| **Type-4 / semantic / behavioral clone** | clone detection (BigCloneBench, SemanticCloneBench) | code alike in *what it does*, not how it reads | **similarity framing — mispositions us toward the forward problem** |
| **Inconsistent clone** / **unpatched clone** | clone-genealogy (DECKARD context-inconsistencies, ICCheck 2025) | clones that diverged; the divergence is the smell | **closest academic term for Shape A** |
| **Late propagation** (esp. type **LP8**) | clone-evolution studies (Barbour/Khomh/Zou) | a clone diverged, one copy fixed, the twin never updated | **exact "bug fixed in one copy" framing** |
| **Update anomaly** | CodeQL `cpp/mostly-duplicate-function` (deprecated, removed CLI 2.8.3, 2022) | *"only one of several copies is updated to address a defect"* | **literally our use case — and abandoned** |
| **Divergent duplicate / parallel maintenance / shotgun surgery** | practitioner smell vocabulary | one change forces N edits; miss one and they diverge | **the everyday name; good for user-facing copy** |
| **Neglected condition / missing precondition** | NEGWeb, API-constraint mining | a call site omits a condition its peers establish | **the name for Shape B** |
| **API misuse** (missing-call / missing-condition) | MUDetect / MuBench | a usage graph missing a node/edge the pattern has | **Shape B as a graph deletion** |
| **Bugs as deviant behavior** | Engler et al., SOSP 2001 | majority defines the implied spec; the minority is the bug | **the recall-first posture itself** |
| **Use-before-initialization** | UBITect / LLift (OOPSLA 2024) | one path reaches a use without the init path having run | **Shape B, literally** |
| **Entity-/token-inconsistency bug** | WitheredLeaf, LineBreaker (2024) | an entity used wrong vs. its peers | **LLM framing covering both shapes** |
| **Drift detection** | infra/config (driftctl, Firefly, Spacelift) | declared state vs. live state | **right *word*, never applied to code logic — transplant is unclaimed** |

**Recommendation.** Retire *"Type-4 clone"* as the headline framing in user-facing copy
(keep it only as a "why clone tools miss this" contrast). Lead with **divergent
reimplementation** / **implementation drift** for Shape A and name **precondition
omission** as a distinct Shape B. Cite *inconsistent clone / late propagation / update
anomaly* as the academic lineage. The single sharpest positioning line the sweep produced:

### Canonical vocabulary for this project (align all docs/code/registry to this)

| Use this | For | Not this |
|---|---|---|
| **implementation drift** | the phenomenon / state | "the dual-path problem" (as the *formal* name) |
| **divergent reimplementation** | the structure / cause (Shape A) | "Type-4 clone", "behavioral clone" |
| **precondition omission** | the omission shape (Shape B) | (none — newly named) |
| **divergent twin** (N=2) / **divergent cluster** (N≥2) | an instance | "duplicate", "copy" |
| **dual-path / multi-path** | acceptable colloquial alias; prefer *multi-path* when N>2 | — |
| *Type-4 / semantic / behavioral clone* | **only** in the "why clone tools miss this" contrast | as a headline framing |

Encode these in calque's own `.calque/vocab-allowlist.txt` so `vocab-check` dogfoods the
consistency (the prose axis enforcing calque's own term discipline). Sweep targets where the
old framing dominates: `docs/DESIGN_NOTES.md` (dual-path ×17), `CHANGELOG.md`, `README.md`,
`SKILL.md`, and the `Type-4` code comments in `funcsig.go`/`sigcluster.go`/`judge.go`/
`scan.go`/`main.go`.

> The equivalence literature judges **actual** equivalence. Nobody judges **intended**
> equivalence — "were these *meant* to be the same?" — which is precisely the seam calque
> occupies.

## Why calque missed these

**Shape A (divergent reimplementation).** Two causes. (1) calque is **boundary-gated**
(`--left`/`--right`): if you don't point it at the two sides, the only un-gated mode is the
weaker whole-repo cluster pass. The "fix didn't stick" safety net **requires scanning where
you did *not* point** — exactly where a forgotten twin hides. (2) Type-4 hardness: the two
spawn-height paths share little effect-footprint (different callees — raw terrain vs.
`waypoints::compute` — different strings), so pairwise token overlap is low. The shared
*concept* ("spawn height") is not a shared *token*.

**Shape B (precondition omission).** calque indexes **commissions** — writes, calls,
strings, returned keys that are *present*. This bug is an **absence** (the missing
wheel-state reset). There is no positive signal to cluster on, so it is a **structural
blind spot**, not a tuning miss. Confirming evidence: calque only surfaced the *sibling*
half of this drift **after** the fix added the wheel-state write that made the token
clusterable. Pre-fix, the omission was invisible by construction.

**Meta-cause.** These are runtime, stateful bugs where per-path unit tests passed. calque
is a static effect-footprint nose; it has no notion of *"these two paths must leave the
system in the same state."*

## Has anyone solved it? No.

The **forward** problem (are two snippets similar/equivalent?) is mature and crowded. The
**inverse** problem calque occupies — surface code that was *supposed* to be the same and
silently drifted, recall-first, **without** a textual-clone prerequisite, **without** an
executable differential harness, **without** a written external spec — is owned only by a
**dormant 2007–2018 subfield** (inconsistent-clone / late-propagation) that is
textual-clone-gated and never got an LLM-era successor.

Closest live things, and how each misses:

- **Greptile** (agentic, whole-repo graph) — closest commercial; even markets the exact
  example. But **diff-gated** (fires on a PR, never a standing audit), no recall guarantee,
  no collapse workflow.
- **ICCheck** (2025) — closest published analogue; git-integrated "you changed a clone but
  not its twin." But **textual** (character-bigram) and single-substrate; misses
  low-textual-signal and cross-substrate twins.
- **CodeQL `mostly-duplicate-function`** — the one tool that *named* update anomalies.
  **Deprecated and removed in 2022**, no replacement.
- **CodeRabbit / Graphite / Cursor BugBot / Devin / Copilot review** — all **diff-anchored**:
  context is pulled *outward from the change to evaluate the change*, the exact inverse of
  recall-first whole-repo divergence surfacing.
- **jscpd / PMD-CPD / Simian / SonarQube / Semgrep / Moderne** — similarity engines or
  apply-side tools; divergence makes a finding **vanish** rather than appear. (Semgrep has
  no anti-join / NOT-exists operator, so "A present but twin B absent" is inexpressible.)
- **Equivalence provers / differential & metamorphic testing** (VeriEQL, PASDA, Mokav,
  DiffSpec, HyClone) — strong at *confirming* a divergence, but every one **assumes the pair
  is already handed to it**; none does the recall/discovery half.
- **Config/API "drift" tooling** — never touches code logic.

**Verdict: the niche is open.** No shipped product combines *detect-divergence* +
*standing whole-repo recall-first audit* + *cluster-to-canonical collapse* on code
behavior. Each market has at most two of the three.

A caveat to stay honest about: a 2025 audit found **~93% of BigCloneBench Type-4 labels are
false positives** ("How the Misuse of a Dataset Harmed Semantic Clone Detection"). Every
"Type-4 F1 in the 90s" headline is on sand — which both warns us off precision theater and
*justifies* calque's gating/calibration discipline. Recall-first on a genuinely unseen
contract caps ~70% F1; promise recall + adjudication, never prover-grade precision.

**Still true as of mid-2026, with fresher and harder evidence.** *Semantic Code Clone
Detection: Are We There Yet?* (arXiv 2606.25272, Jun 2026) tests 11 SOTA detectors —
token-, tree-, and graph-based, the last being the closest category to embedding/GNN
approaches — against newly-constructed, distribution-shifted-but-genuinely-Type-4 clones
rather than the same tainted benchmarks. Every approach shows "substantial performance
degradation," and the detectors "heavily rely on shortcut learning based on lexical and
structural cues rather than robust semantic understanding." Two months old at time of
writing — the field hasn't solved this, it's still gaming benchmark shortcuts. *TriFusion-LLM*
(2603.15004, Mar 2026) is a smaller, orthogonal data point: it independently arrives at
calque's own architecture — cheap structural/statistical signals first, LLM arbitration only
on the highest-uncertainty ~0.2% of cases — and finds that combination "substantially
outperforms blind reclassification," which validates the shape of `--judge` (cheap
deterministic recall, expensive LLM only on already-surfaced candidates), not a new
mechanism to adopt.

## The architecture everyone converges on

Independently, all three sweeps (academic, industry, omission-subfield) land on the **same
loop** — and it is the same loop for both bug shapes:

> **Enumerate the siblings → learn the modal contract/setup → flag the outlier →
> LLM-adjudicate the hard cases → behaviorally confirm the consequential ones.**

- **Shape A:** siblings = the N implementations of a contract; modal = the canonical
  behavior; outlier = the drifted copy.
- **Shape B:** siblings = the N call sites into a shared subsystem; modal = the pre-entry
  setup-set; outlier = the site whose setup-set is a **strict subset** of its peers'.

Lineage for this exact loop: Engler "deviant behavior" (2001) → PR-Miner (2005) → APISan
(2016) → MUDetect (2019), re-validated by 2024–2026 LLM work: **WitheredLeaf**
(entity-inconsistency, small-LLM filter → GPT-4 adjudicate), **LLift** (OOPSLA 2024 —
static candidate-gen → LLM resolves the undecided modal-vs-outlier paths), **"One Bug,
Hundreds Behind"** (2025 — seed from one bug, LLM learns the pattern, finds siblings
including "most sites do X, outliers don't"), and **MINES** (2026 — LLM infers *executable*
precondition predicates, flags the site that violates one, and the predicate *is* the
explanation).

## Roadmap for calque (ranked by leverage on the actual pain)

0. **Extract a `reads` signal — the missing invariant for value-derivation drift.** The
   adopter's own dual-path ledger (every fixed + open instance: spawn height, ground
   height, carriageway width, centerline, junction rim) is one shape: *the same physical
   quantity derived independently in two places.* The invariant tying the twins is **the set
   of domain fields they both READ** to compute their output (`road.width_m`, `road.pieces`,
   `terrain.heights`/carve…). calque's `FuncSig` has `writes`/`calls`/`strings`/`ret_keys`/
   `delegates` but **no `reads`** — so for this dominant shape there is no shared positive
   signal to cluster on, by construction. Add a `reads` key (field-paths consumed to derive
   a value) and a cluster rule, **including the negative discriminator the ledger supplies:**
   *N functions read the same canonical field-set to derive a position/height/offset/width,
   and **none delegates to a shared authority** → dual-path candidate.* calque already
   extracts `delegates`; the false twins in the ledger are "NOT diverging" precisely because
   both route through one shared function — that delegation check is the precision signal.
   Fully general (no game/crate assumption): "same read-set, no shared delegation, anywhere."
   **Full implementation spec: `docs/SPEC-reads-axis.md`.** SHIPPED 2026-06-15; first live
   validation on a sibling Rust codebase (5 candidates, 2 real / 3 false, zero noise). The
   next increment — an **operation-type precision gate** + **shared-constants** and
   **drift-confessing-comment** recall axes + an **ablation harness** to prune dead layers —
   is specced in `docs/SPEC-detection-layers.md`, grounded in that validation + a 2026
   coding-LLM-failure-mode sweep (the niche is documented-but-untooled: SlopCodeBench measures
   accumulated redundancy but pinpoints nothing; SWE-Refactor proves multi-site propagation
   fails at 39.4% but no tool audits k-of-N sites).

1. **Ungate from the boundary into a standing whole-repo recall audit.** Greptile's fatal
   limitation is diff-gating; calque's analog is `--left`/`--right` gating. The "fix didn't
   stick" safety net **must** scan where you didn't point — and the `reads`-cluster pass (0)
   is where it fires un-gated. (Industry sweep, recall-first requirement.)

2. **Add the omission axis (Shape B) — the open gap nobody occupies.** Extract, per
   call-site of a shared subsystem, the **pre-entry setup-set** as a first-class key, then
   run calque's *existing* modal/outlier judge, flagging the site whose key-set is a strict
   subset of its siblings'. This converts the commission-index into an omission-detector
   **without a new mechanism — just a new key.** Make the precondition *executable* and the
   flag *explainable* (emit the predicate, MINES-style), not a bare score.

3. **Behavioral confirm step for surfaced twins.** Have the judge synthesize a
   *distinguishing input* / differential test (HyClone / Mokav / DiffSpec) — grounds a
   drift flag instead of vibes, and is decisive for runtime/stateful bugs that static
   footprints miss.

4. **Calibrate the judge: terse prompt, expect over-flagging of real twins.** EquiBench +
   overcorrection findings: LLM equivalence judges are biased toward calling
   structurally-different-but-equivalent code **inequivalent** (the false-drift direction),
   and **verbose "explain the difference" prompts make it worse**. Use a terse judge; keep
   the LLM as a recall/filter stage, never the sole oracle (BugStone hit 92% precision only
   because the LLM judged *within* tight pre-filtered candidates).

5. **Metamorphic / differential relation for the "two paths must agree" case.** State one
   invariant — *"reaching state S by any path must leave the system in the same equivalence
   class"* — and Shape B becomes a test failure at runtime. General to any digital-twin or
   client/server split (a game repositioning path, a serializer↔parser round-trip, a
   handler↔job that must compute the same result). The sweep found **no game-physics
   metamorphic-testing prior art** specifically — one open lane among many.

## Sources

Academic / equivalence: ICCheck (arXiv 2504.04537), late-propagation (Barbour/Khomh/Zou),
DECKARD context-inconsistencies, HyClone (2508.01357), DiffSpec (2410.04249), Mokav
(2406.10375), PASDA (2311.08071), EquiBench (2502.12466), BugStone (2510.14036), the
BigCloneBench-misuse audit (2505.04311), Functional Consistency of LLM Code Embeddings
(2508.19558 — off-the-shelf code embeddings predominantly capture syntax, not function;
getting real functional separation took purpose-built contrastive fine-tuning, a caution
against assuming a plain embed-a-summary pass would cleanly separate Type-4 twins), An
Empirical Study of LLM-Based Code Clone Detection (2511.01176 — LLM clone-judgment F1
swings from 0.94 on one benchmark to markedly worse on another depending on clone style;
LLM-based judgment needs real per-corpus calibration, not an assumed accuracy), Semantic
Code Clone Detection: Are We There Yet? (2606.25272, Jun 2026 — 11 SOTA detectors across
token/tree/graph paradigms all degrade substantially under distribution-shifted Type-4
clones; shortcut learning, not semantic understanding), TriFusion-LLM (2603.15004, Mar
2026 — cheap structural signals + selective LLM arbitration on ~0.2% of high-uncertainty
cases beats blind reclassification, validating calque's own recall-then-judge shape).
Industry: Greptile v3 agentic review, CodeQL CLI
2.8.3 changelog (mostly-duplicate-function removal), Semgrep join-mode docs, Engler "Bugs as
Deviant Behavior" (SOSP 2001). Omission/deviant: PR-Miner (FSE 2005), NEGWeb, APISan
(USENIX Security 2016), MUDetect/MuBench (TSE 2019 / MSR 2016), UBITect+LLift (OOPSLA 2024),
WitheredLeaf (2405.01668), "One Bug, Hundreds Behind" (2510.14036), MINES (2512.06906).
