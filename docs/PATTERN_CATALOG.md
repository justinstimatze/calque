# calque — multi-path pattern catalog

The meta-bug calque exists to kill is **one contract, N implementations that drift.**
This catalogs the concrete *shapes* it takes, observed in real projects. It is the
bridge between loose findings and calque's signal profiles (§13 of DESIGN_NOTES): each
shape says where it hides, *why a local generator produces it*, the tell that catches
it, the fix, and whether calque's current nose catches it today.

Naming the shapes is the point. Unnamed, this is "AI slop" (a vibe you can't act on);
enumerated, it's a checklist a nose can hunt. The shapes recur across languages and
across human- and AI-authored code — AI just *industrializes* them (local generation +
no cross-session memory of existing primitives → re-implement instead of reuse).

Status legend: **[CAUGHT]** current AST nose ranks it · **[WEAK]** partial signal,
needs the LLM/semantic nose · **[MISS]** current nose blind, upgrade identified ·
**[SIBLING]** different lens (env / cross-language).

Instances are drawn from real codebases — primarily a mid-size Python project under
active development — generalized here, unless a public sibling is named.

---

## P1 — N-ary inlined shell duplication  [MISS → DESIGN_NOTES §15, roadmap 3a/3b]
**Shape.** ≥2 entry points ("shells") each *inline* the same multi-step sequence; N is
often >2 and the unit is a statement block, not a named function.
**Seen.** Three entry points (`step` on two adjacent classes, plus `run`) each inlined
`[parse → read/clear a shared canon → dispatch]`; `run()` drifted silently for months
(dropped the constructor-canon step).
**Why generated.** Each shell written at a different time; inlining the sequence is
locally complete — finding and routing through a shared primitive requires global
knowledge the generator doesn't hold.
**Tell.** A shared touch of *rare private symbols* (e.g. a private parse/canon helper)
across differently-named functions. Presence-based, name-agnostic, N-ary.
**Fix.** Extract the primitive, route all shells through it; pin with a registry-driven
structural antibody.
**Catchability.** Whole-function pairwise Jaccard drowns the block and the name signal
misleads → needs the private-symbol-touchpoint signal + N-ary clustering.

## P2 — Taxonomy / vocabulary in N literal sets  [WEAK]
**Shape.** One conceptual set restated as inline literals in many places.
**Seen.** A verb-synonym set ("examine/look/talk") restated in ≥5 spots: a canonical verb
config + an embedding copy (the only one with a lockstep test) + a scene verb-map + an
allowed-verbs list + an engine-side regex. Already drifted (two synonyms present in one
copy only).
**Why generated.** An inline literal set is locally legible; importing the canon needs
knowing it exists and where.
**Tell.** Same/overlapping string-literal cluster across files, or — more reliably — a
*semantic* match ("this is a verb-synonym set"). Pure AST struggles (the strings are
common); this is where the LLM nose earns its keep.
**Fix.** One table as data (`StrEnum` + alias map); each site applies a *whitelist*
(policy) over the shared resolver (mechanism). Policy is local data; mechanism is shared.
**Catchability.** Drives a "shared-literal-cluster" signal + the LLM-adjudicate idea.

## P3 — Magic constant redefined in N modules  [SIBLING + WEAK]  (cross-axis)
**Shape.** One tuning value as a module constant in several files, synced by a *comment*.
**Seen.** A `0.65` cosine threshold as a module constant in four modules; only one honors
the corresponding env override, so the override silently moves 1 of 4 sites — the
code-duplication axis (P1/P2) meeting the env-parity axis (P8) in one bug.
**Why generated.** Needed the number locally; re-typed it.
**Tell.** Identical numeric literal bound to similarly-named constants across files;
cross-referenced with env reads of the "same" knob.
**Fix.** One `config` field all sites import.
**Catchability.** A duplicated-constant signal + the env axis. Argues the two axes
should share a registry shape: "a value with N definition sites, some code, some env."

## P4 — Parallel parser in an un-enumerated subsystem  [MISS]
**Shape.** A subsystem rolls its own mini version of a core primitive the main antibody
doesn't know to check.
**Seen.** A subsystem's own `_parse_input` — own preposition strip, own verb map, *no
politeness strip* (re-opening the P1 bug inside a subsystem). The original antibody was
scoped to three named shells, so it couldn't see a fourth.
**Why generated.** A subsystem feels self-contained; the model builds it complete in
itself rather than reaching back to the shared normalizer.
**Tell.** A function whose body shape matches a core primitive (split + strip + map
verb) but lives outside the registered shell set.
**Fix.** Registry-driven antibody (a `Protocol` + registry so a *new* shell is
auto-covered); route through shared normalization, keep local policy.
**Catchability.** Name-scoped hand-written antibodies miss it; the structural/registry
antibody is the generalizing fix.

## P5 — Serialization twin (write/read halves)  [CAUGHT-able, audit-leg]
**Shape.** `to_dict`/`from_dict` (or save/load, encode/decode) — two halves that must
agree on a field set, edited independently.
**Seen.** Many such pairs in the surveyed code; **adjudicated clean** (disciplined
emit-if-non-default / read-with-default) — a calibration win: recall surfaced them,
adjudication cleared them. Latent twin nonetheless (a new field must touch both halves by hand).
**Why generated.** Both halves written at creation; later field additions touch one.
**Tell.** Paired functions sharing a field-name set; drift = asymmetric key set.
**Fix.** Dataclass + `asdict` / single field source; or a round-trip property test.
**Catchability.** A field-set differential is a clean `calque audit` target.

## P6 — Harness vs prod path (test double reimplements prod)  [CAUGHT]
**Shape.** A test/harness reimplements prod logic instead of driving it; the two drift.
**Seen.** A test harness vs the real engine — the canonical `engine* × testing` boundary,
where calque found **4 real drifts**; plus a large test-suite migration back onto the
prod path.
**Why generated.** Faking behavior in the harness is easier than wiring the real engine.
**Fix.** Harness *delegates* to prod (adapter, not reimpl) + a differential test.
**Catchability.** calque's home turf — name-stem + shared-calls + shared-strings.

## P7 — Same-contract twin functions (classic Type-4)  [CAUGHT]
**Shape.** Two functions implementing one contract that should converge.
**Seen.** `_refresh_location_refs` ≟ `_reveal_refs_for_location`, etc.
**Catchability.** The core nose. Calque was built for exactly this.

## P8 — Env / config parity (one path, N launcher configs)  [SIBLING → §14]
**Shape.** One code path booted with different effective config by different launchers.
**Seen.** One launcher left two LLM/embedding env flags unset versus the production
config. Dozens of scattered `os.environ.get` reads across many flags.
**Fix.** Typed single-source config + a boot-parity guard; static cross-launcher diff
(calque sibling check, planned).
**Catchability.** Different lens (parse shell/Make/TOML/conftest, not Python AST).

## P9 — Value computed two ways across subsystems  [WEAK]
**Shape.** A derived value computed by independent functions in different subsystems.
**Seen.** Two functions in different modules both answering "what do we call the exit
A→B" — independently derived, with no shared helper.
**Tell.** Functions sharing an arg signature + output concept across modules.
**Fix.** Shared helper or a differential.
**Catchability.** Partial via name + return-shape; medium.

## P10 — Client/server contract split (cross-language)  [SIBLING, open]
**Shape.** A classification/rule duplicated in a JS/TS client and a Python server.
**Seen.** A web client's verb-passthrough list duplicated server-side (since retired →
single path; the fix held — a negative result worth recording).
**Fix.** Shared spec/contract both sides map onto, or a cross-language differential.
**Catchability.** Not by a single-language AST nose (DESIGN_NOTES §10 open question).

---

## How this feeds calque

- **[CAUGHT]** (P6, P7) validate the current nose.
- **[MISS]** (P1, P4) are the P0 upgrades: private-symbol touchpoints, N-ary clusters,
  registry-driven antibodies.
- **[WEAK]** (P2, P3, P9) argue for an **LLM-driven adjudicate leg over candidates the
  cheap static nose surfaces** — the live-hunt method, productized.
- **[SIBLING]** (P8, P10) are separate axes/lenses (env parity; cross-language spec).

This catalog is **language-agnostic by design** — the next step is to confirm each shape
on TypeScript and Go siblings and note which transfer, which are language-specific,
and which a language's type system already neutralizes (roadmap items 11–12). Extend this
file as new shapes are found.

---

## Cross-repo evidence — a multi-repo LLM-as-nose survey

Surveyed several sibling repos (Python/Go/TS) with LLM fuzzy-hunts using this catalog as
the checklist. The meta-bug appeared in **every repo, every language.** Public siblings
are named with links; private ones are described by language and domain only.

**Confirmed LIVE drift (copies already disagree) — including real bugs, not just tidiness:**
- **A TS regulatory-data project** (correctness *is* the product): an identity-key
  canonicalizer was implemented 3 different ways across adapters (varying the allowed
  character class) → the same input produced different IDs → silent dedup misses (P7). A
  date parser was copy-pasted into several adapters (P4) — not live-broken (each matched
  its own source format) but a latent landmine: many copies, no range validation, and the
  next paste grabs the wrong variant. No shared ingest utility.
- **[effigy](https://github.com/justinstimatze/effigy)** (Py): a `compression_ratio`
  metric computed with **reversed operands** in two modules — two same-named metrics
  measuring inverse things; cross-referencing them corrupts analytics (P7).
- **A TS agent tool**: an objective-length limit defined as two different values in the
  schema vs the handler (P3); a state-dir path literal repeated across ~20 files (P3, latent).
- **[gemot](https://github.com/justinstimatze/gemot)** (Go): a retry budget restated as
  several different values across files (P3); an API-secret resolution that drifts across
  scripts — one has a legacy-var + `.env` fallback the others lack (P8).
- **[defn](https://github.com/justinstimatze/defn)** (Go): a branch-checkout sequence
  inlined several times (P1/P8); a `Kind` taxonomy restated in multiple places (P2); a
  default port literal duplicated (P3).
- **[adit-code](https://github.com/justinstimatze/adit-code)** (Go, *itself a code-health
  scanner*): a session-JSONL parse loop inlined several times with already-differing
  filters (P1); serialization boilerplate repeated many times (P1, latent); two divergent
  complexity taxonomies in adjacent files (P2). **The tool has the disease it diagnoses.**
- **calque** (Py, self-scan — see `.calque/registry.md`): its own signal taxonomy
  `{strings,writes,name,calls,ret}` was hand-written in 4 places (`_WEIGHTS` + `sig` +
  `avail` + `reason`) — calque's *own* P2/P3 bug, caught by its own nose.

**Six findings the survey proves:**
1. **P3 (magic constants) and P8 (env/config scatter) dominate every repo, every
   language.** They are the literal connective tissue between the code axis (§15) and the
   env-parity axis (§14) → strong evidence for ONE registry keyed on "a value with N
   definition sites, code *or* env."
2. **Go is robust but narrowly** (the Go repos): the compiler kills *signature* drift
   (compile-time `var _ Interface = (*T)(nil)`) but does nothing for config-scatter,
   inlined orchestration, or *behavioral* drift between two impls. Slop sources are
   inheritance-independent.
3. **The unit of memory is the group, not the pair** — empirically, the same value or set
   recurred at many sites at once (date parsers, API-secret resolution, verb-sets,
   thresholds, state-dir literals). N-ary registry validated in the wild.
4. **Tool-irony + calque-self-bug:** code-tools (a health scanner; calque itself) carry
   the bug they detect. It is *invisible from inside* — the strongest argument for an
   external standing nose.
5. **Real LIVE data-corrupting drifts exist** (identity-key/date drift, inverted metrics,
   disagreeing limits) — this is a bug class, not a neatness preference.
6. **Calibration earned its keep:** crude scans false-flagged clean serialization pairs;
   adjudication cleared them. Recall surfaces, adjudicate decides — the core loop,
   repeatedly validated.
