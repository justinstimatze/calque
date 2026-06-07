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

Instances cited are from `lamina/poc/dense/stope` (Python), 2026-06-04→06, unless noted.

---

## P1 — N-ary inlined shell duplication  [MISS → DESIGN_NOTES §15, roadmap 3a/3b]
**Shape.** ≥2 entry points ("shells") each *inline* the same multi-step sequence; N is
often >2 and the unit is a statement block, not a named function.
**Seen.** #269: `GameEngine.step` / `GameSession.step` / `GameEngine.run` each inlined
`[_parse_action → read/clear _agent_canon → dispatch]`; `run()` drifted silently for
months (dropped the constructor canon).
**Why generated.** Each shell written at a different time; inlining the sequence is
locally complete — finding and routing through a shared primitive requires global
knowledge the generator doesn't hold.
**Tell.** A shared touch of *rare private symbols* (`_parse_action`, `_agent_canon`)
across differently-named functions. Presence-based, name-agnostic, N-ary.
**Fix.** Extract the primitive (`_resolve_clause`), route all shells through it; pin
with a registry-driven structural antibody.
**Catchability.** Whole-function pairwise Jaccard drowns the block and the name signal
misleads → needs the private-symbol-touchpoint signal + N-ary clustering.

## P2 — Taxonomy / vocabulary in N literal sets  [WEAK]
**Shape.** One conceptual set restated as inline literals in many places.
**Seen.** The "examine/look/talk" verb-synonym set in ≥5 spots: `input_llm._VERB_CONFIG`
(canon) + `affordances._EMBED_VERBS` (only copy with a lockstep test) +
`memory_scene._map_verb` + `reach_scene.ALLOWED_VERBS` + an `engine.py` regex. Already
drifted (`read`/`listen` each in one copy only).
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
**Seen.** `0.65` cosine threshold in `heard_set` / `affordances` / `noun_grounding` /
`noun_disambiguation`; only `noun_grounding` honors `STOPE_NOUN_EMBED_THRESHOLD`, so the
env override silently moves 1 of 4 sites — the code-duplication axis (P1/P2) meeting the
env-parity axis (P8) in one bug.
**Why generated.** Needed the number locally; re-typed it.
**Tell.** Identical numeric literal bound to similarly-named constants across files;
cross-referenced with env reads of the "same" knob.
**Fix.** One `config` field all sites import.
**Catchability.** A duplicated-constant signal + the env axis. Argues the two axes
should share a registry shape: "a value with N definition sites, some code, some env."

## P4 — Parallel parser in an un-enumerated subsystem  [MISS]
**Shape.** A subsystem rolls its own mini version of a core primitive the main antibody
doesn't know to check.
**Seen.** `MemoryScene._parse_input` — own preposition strip, own verb map, *no
politeness strip* (re-opening #269 brick-3 inside the scenes). The #269 antibody is
scoped to three named shells, so it can't see a fourth.
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
**Seen.** Many pairs in stope; **adjudicated clean here** (disciplined emit-if-non-
default / read-with-default) — a calibration win: recall surfaced them, adjudication
cleared them. Latent twin nonetheless (a new field must touch both halves by hand).
**Why generated.** Both halves written at creation; later field additions touch one.
**Tell.** Paired functions sharing a field-name set; drift = asymmetric key set.
**Fix.** Dataclass + `asdict` / single field source; or a round-trip property test.
**Catchability.** A field-set differential is a clean `calque audit` target.

## P6 — Harness vs prod path (test double reimplements prod)  [CAUGHT]
**Shape.** A test/harness reimplements prod logic instead of driving it; the two drift.
**Seen.** `ScriptedGame` vs the real engine — the calque seed-run boundary
(`engine*×testing`), where calque found **4 real drifts**; plus the 323-test migration
back onto the prod path.
**Why generated.** Faking behavior in the harness is easier than wiring the real engine.
**Fix.** Harness *delegates* to prod (adapter, not reimpl) + a differential test.
**Catchability.** calque's home turf — name-stem + shared-calls + shared-strings.

## P7 — Same-contract twin functions (classic Type-4)  [CAUGHT]
**Shape.** Two functions implementing one contract that should converge.
**Seen.** `_refresh_location_refs` ≟ `_reveal_refs_for_location`, etc.
**Catchability.** The core nose. Calque was built for exactly this.

## P8 — Env / config parity (one path, N launcher configs)  [SIBLING → §14]
**Shape.** One code path booted with different effective config by different launchers.
**Seen.** `make roleplay-server` left `STOPE_INPUT_LLM`/`STOPE_NOUN_EMBED` unset vs prod
`fly.toml`. ~65 scattered `os.environ.get` across ~38 flags.
**Fix.** Typed single-source config + boot parity guard (stope #240); static
cross-launcher diff (calque sibling check, planned).
**Catchability.** Different lens (parse shell/Make/TOML/conftest, not Python AST).

## P9 — Value computed two ways across subsystems  [WEAK]
**Shape.** A derived value computed by independent functions in different subsystems.
**Seen.** `engine_explore._exit_label_for` vs `scene_state._connection_exit_label` —
both answer "what do we call the exit A→B."
**Tell.** Functions sharing an arg signature + output concept across modules.
**Fix.** Shared helper or a differential.
**Catchability.** Partial via name + return-shape; medium.

## P10 — Client/server contract split (cross-language)  [SIBLING, open]
**Shape.** A classification/rule duplicated in a JS/TS client and a Python server.
**Seen.** #233 web `TALK_MODE_PASSTHROUGH_VERBS` (since retired → single path; the fix
held — a negative result worth recording).
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
on `undercity` (TS) and `gemot` (Go) and note which transfer, which are language-specific,
and which a language's type system already neutralizes (roadmap items 11–12). Extend this
file as new shapes are found.

---

## Cross-repo evidence — 6-repo LLM-as-nose survey (2026-06-06)

Surveyed 6 of Justin's non-fork repos (Python/Go/TS) with LLM fuzzy-hunts using this
catalog as the checklist. The meta-bug is present in **every repo, every language.**

**Confirmed LIVE drift (copies already disagree) — including real bugs, not just tidiness:**
- **publicrecord** (TS, regulatory data — correctness IS the product): `sanitizeCaseNumber`
  canonicalizes an *identity key* 3 different ways across adapters (`[^a-z0-9]+` vs
  `[^a-z0-9-]` vs `[^a-z0-9.-]`) → same case → different IDs → silent dedup misses (P7).
  `normalizeDate` in 5 adapters (P4) — *not* live-broken (each parser matches its own
  source: US `MM/DD`, EU `DD/MM`+`DD.MM`) but a latent landmine: 5 copies, no range
  validation, next copy-paste grabs the wrong variant. No shared `src/ingest/utils.ts`.
- **effigy** (Py): `compression_ratio` computed with **reversed operands** —
  `metrics.py` (`json/effigy`) vs `discovery.py` (`baseline/dossier`) — two same-named
  metrics measuring inverse things; cross-referencing them corrupts analytics (P7).
- **undercity** (TS): `MAX_OBJECTIVE_LENGTH` = 500 (`pm-schemas.ts`) vs 2000
  (`server.ts`) — validation vs handler disagree (P3). `.undercity` state-dir literal in
  19+ files (P3, latent).
- **gemot** (Go): retry budget 3/5/10 across files (P3); `GEMOT_API_SECRET` resolution
  drifts across 8 scripts — diplomacy has a legacy-var + `.env` fallback others lack (P8).
- **defn** (Go): `DEFN_BRANCH` checkout inlined 3× (P1/P8); `Kind` taxonomy in 4 places
  (P2); `DEFN_PORT` "3307" default twice (P3).
- **adit** (Go, *itself a code-health scanner*): session-JSONL parse loop inlined 3× with
  already-differing filters (P1); `json.MarshalIndent` boilerplate ×7 (P1, latent); two
  different complexity taxonomies — `nesting.go` (36 node kinds) vs `branching.go` (subset)
  (P2). **The tool has the disease it diagnoses.**
- **calque** (Py, self-scan — see `.calque/registry.md`): its own signal taxonomy
  `{strings,writes,name,calls,ret}` hand-written in 4 places (`_WEIGHTS` + `sig` + `avail`
  + `reason`) — calque's *own* P2/P3 bug, caught by its own nose.

**Six findings the survey proves:**
1. **P3 (magic constants) and P8 (env/config scatter) dominate every repo, every
   language.** They are the literal connective tissue between the code axis (§15) and the
   env-parity axis (§14) → strong evidence for ONE registry keyed on "a value with N
   definition sites, code *or* env."
2. **Go is robust but narrowly** (gemot + defn): kills *signature* drift (compile-time
   `var _ Interface = (*T)(nil)`) but does nothing for config-scatter, inlined
   orchestration, or *behavioral* drift between two impls. Slop sources are
   inheritance-independent.
3. **The unit of memory is the group, not the pair** — empirically: 5× `normalizeDate`,
   8× `GEMOT_API_SECRET`, 5× verb-sets (stope), 4× `0.65` (stope), 19× `.undercity`.
   N-ary registry validated in the wild.
4. **adit-irony + calque-self-bug:** code-tools (a health scanner; calque itself) carry
   the bug they detect. It is *invisible from inside* — the strongest argument for an
   external standing nose.
5. **Real LIVE data-corrupting drifts exist** (publicrecord IDs/dates, effigy metrics,
   undercity limits) — this is a bug class, not a neatness preference.
6. **Calibration earned its keep:** crude scans false-flagged clean serialization pairs
   (stope `to_dict`/`from_dict`, dos/phone); adjudication cleared them. Recall surfaces,
   adjudicate decides — the core loop, repeatedly validated.
