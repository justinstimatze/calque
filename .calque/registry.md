# calque registry — calque on calque

Durable, git-tracked memory of adjudicated dual-path pairs **for calque's own
source**. calque must eat its own dogfood (cf. the adit irony: a code-health tool
that contained the patterns it detects).

Run:  `calque scan --repo . --exclude 'legacy/**,**/*_test.go'`   (pairs + N-ary clusters)
Gate: `calque check --repo . --exclude 'legacy/**,**/*_test.go'`  (only NEW vs this registry)

(Exclude `legacy/**` — the Python port reference — and `**/*_test.go`, whose
synthetic fixtures plant seams on purpose. The registry tracks both `- pair:`
and `- cluster:` (N-ary, member-set keyed) verdicts.)

Each adjudicated pair carries machine lines (`- pair:` / `- verdict:` /
`- reviewed:`) that `check` parses; the surrounding prose is for humans.
Verdicts: **drift** (collapse) · **contracted-twin-ok** (pin, watch) ·
**false-alarm** (suppress). Recency is `reviewed` + liveness reconciliation,
never age-eviction (an old false-alarm is still a false-alarm).

A **drift** pair may also carry `- canonical:` (the path to keep) and
`- do-not-resync:` (the doomed path to collapse away). `check` surfaces every
drift pair whose both paths are still live as `DRIFT (unresolved)` with this
direction — warn-only — so a later agent collapses the right path instead of
re-syncing the doomed one (DESIGN_NOTES §18.7).

---

## Run — 2026-06-06 (Go rewrite; self-scan of the Go source)

52 funcs / 10 files. Two Python-era self-bugs resolved (symmetric output → fixed
via unordered-pair dedup; the relTo ≟ corpus.RelPath dup caught live during the
port → single-sourced, gone). Four standing pairs adjudicated below.

## Suspicion.Reason ≟ scorePair — DRIFT (open: calque's own taxonomy bug, again)
- pair: internal/code/score.go::Suspicion.Reason | internal/code/score.go::scorePair
- verdict: drift
- reviewed: 2026-06-06

The signal taxonomy `{strings,writes,name,calls,ret}` is listed in four places:
the `weights` map, the `sig` + `avail` maps in `scorePair`, and the `switch` in
`Reason`. The Python version had this (PATTERN_CATALOG P2/P3); the port reproduced
it. Registered as known so `check` doesn't re-flag it. **Fix:** make signals
table-driven (`[]signalDef{key,weight,sim,avail,render}`) so a new signal is one
entry.

## runSynonymReport ≟ runVocabReport — CONTRACTED-TWIN-OK (latent; watch)
- pair: cmd/calque/synonym_report.go::runSynonymReport | cmd/calque/vocab_report.go::runVocabReport
- verdict: contracted-twin-ok
- reviewed: 2026-06-06

Both prose commands share the walk→tally loop. They already single-source the
walk+strip via `internal/corpus`; move the per-token tally to a shared helper if a
third prose command appears. Pin, don't collapse yet.

## toSet ≟ setEqual — FALSE-ALARM
- pair: internal/code/funcsig.go::toSet | internal/code/score.go::setEqual
- verdict: false-alarm
- reviewed: 2026-06-06

Two tiny set helpers (name~set, one shared call). Coincidental overlap.

## texts ≟ TextsBatched — FALSE-ALARM (adapter)
- pair: internal/embed/embed.go::texts | internal/embed/embed.go::TextsBatched
- verdict: false-alarm
- reviewed: 2026-06-06

`TextsBatched` is the batching wrapper around `texts` — named after what it wraps.

---

## Cross-language extractor twins (intentional; the multi-language design)

The Go (`extract_go.go`, go/ast) and Python (`extract.py`) extractors deliberately
mirror the same extraction in each language — calque correctly flags them as twins.
These are **contracted-twin-ok**: they MUST stay in sync (if one drifts in what it
extracts, the code/Python axes disagree), so it's *good* that calque tracks them.
This is calque watching its own intentional parallel — the feature working on itself.

## attrPath ≟ _attr_path — contracted-twin-ok (dotted attr suffix, per language)
- pair: internal/code/extract_go.go::attrPath | internal/code/extract.py::_attr_path
- verdict: contracted-twin-ok
- reviewed: 2026-06-06

## goBody.recordTarget ≟ _BodyVisitor._record_target — contracted-twin-ok
- pair: internal/code/extract_go.go::goBody.recordTarget | internal/code/extract.py::_BodyVisitor._record_target
- verdict: contracted-twin-ok
- reviewed: 2026-06-06

## goBody.Visit ≟ _BodyVisitor.visit_Call — contracted-twin-ok
- pair: internal/code/extract_go.go::goBody.Visit | internal/code/extract.py::_BodyVisitor.visit_Call
- verdict: contracted-twin-ok
- reviewed: 2026-06-06

## Extract ≟ _extract — contracted-twin-ok (walk+extract entry, per language)
- pair: internal/code/scan.go::Extract | internal/code/extract.py::_extract
- verdict: contracted-twin-ok
- reviewed: 2026-06-06

## extractGoBatch ≟ ExtractGoFile — false-alarm (batch wraps the per-file extractor)
- pair: internal/code/extract_go.go::extractGoBatch | internal/code/extract_go.go::ExtractGoFile
- verdict: false-alarm
- reviewed: 2026-06-06

## main ≟ main — false-alarm (both entry points named main)
- pair: cmd/calque/main.go::main | internal/code/extract.py::main
- verdict: false-alarm
- reviewed: 2026-06-06

## runCheck ≟ runScan — contracted-twin-ok (shared scan pipeline; refactor candidate)
- pair: cmd/calque/check.go::runCheck | cmd/calque/scan.go::runScan
- verdict: contracted-twin-ok
- reviewed: 2026-06-06

Both run Extract→Filter→Rank then diverge (report vs gate). Shared setup could move
to a helper if it grows; pinned for now.

## More cross-language visitor twins — contracted-twin-ok
- pair: internal/code/extract.py::_BodyVisitor.visit_Constant | internal/code/extract_go.go::goBody.Visit
- verdict: contracted-twin-ok
- pair: internal/code/extract.py::_BodyVisitor.visit_Assign | internal/code/extract_go.go::goBody.Visit
- verdict: contracted-twin-ok
- pair: internal/code/extract.py::_BodyVisitor.visit_Return | internal/code/extract_go.go::goBody.Visit
- verdict: contracted-twin-ok

## "*Key" name collisions — false-alarm (single shared name token "key")
- pair: internal/pairkey/pairkey.go::Key | cmd/calque/synonym_report.go::pairKey
- verdict: false-alarm
- pair: internal/pairkey/pairkey.go::Key | internal/code/extract_go.go::litKey
- verdict: false-alarm
- pair: internal/pairkey/pairkey.go::Key | cmd/calque/vocab_report.go::stemKey
- verdict: false-alarm

## scorePair ≟ _extract — false-alarm (coincidental shared signal-key strings)
- pair: internal/code/score.go::scorePair | internal/code/extract.py::_extract
- verdict: false-alarm
- reviewed: 2026-06-06

---

## Run — 2026-06-06 (N-ary touchpoint clustering added; self-dogfood of the new pass)

Added the private-symbol touchpoint signal + N-ary clustering (DESIGN_NOTES §15,
the triple-shell gap). Running the new feature on calque itself surfaced one
real N-ary drift, the intentional cluster-mirrors-pair API parallels, and the
usual key/set name-token coincidences. Adjudicated below; `check` is clean again.

## Signal taxonomy {name,strings,calls,writes,ret} in 4 sites — DRIFT (N-ary)

The new touchpoint pass caught the **N-ary extent** of the already-registered
`Suspicion.Reason ≟ scorePair` drift: the signal taxonomy is a contract shared by
both extractors (which emit those JSON field names) AND both scorers (which read
them as map keys). The pairwise entry saw 2 of the 4 sites; the cluster sees all
4. Same root, same fix (make signals table-driven — `[]signalDef{...}`); pinned so
`check` doesn't re-flag. This is the feature finding the bug it was built to find.
- cluster: internal/code/extract.py::_extract | internal/code/extract_go.go::goBody.Visit | internal/code/score.go::Suspicion.Reason | internal/code/score.go::scorePair
- verdict: drift
- reviewed: 2026-06-06

## Cluster API mirrors pair API — contracted-twin-ok (the N-ary registry, by design)

The cluster path deliberately parallels the pair path (HasCluster≟Has,
LookupCluster≟Lookup, Cluster.Reason≟Suspicion.Reason, SetKey≟Key). These MUST
stay in lockstep — calque correctly flags its own intentional parallel.
- pair: internal/registry/registry.go::Registry.HasCluster | internal/registry/registry.go::Registry.Has
- verdict: contracted-twin-ok
- reviewed: 2026-06-06
- pair: internal/registry/registry.go::Registry.LookupCluster | internal/registry/registry.go::Registry.Lookup
- verdict: contracted-twin-ok
- reviewed: 2026-06-06
- pair: internal/code/touchpoint.go::Cluster.Reason | internal/code/score.go::Suspicion.Reason
- verdict: contracted-twin-ok
- reviewed: 2026-06-06
- pair: internal/pairkey/pairkey.go::SetKey | internal/pairkey/pairkey.go::Key
- verdict: contracted-twin-ok
- reviewed: 2026-06-06

## More key/set name-token collisions — false-alarm (a module about keys and sets)

`keySet` (set of member keys, for subset checks) collides on the "key"/"set"
tokens with `SetKey`/`Key`/`toSet`. Coincidental shared name tokens, distinct
purposes — same class as the *Key collisions above.
- pair: internal/code/touchpoint.go::keySet | internal/pairkey/pairkey.go::SetKey
- verdict: false-alarm
- reviewed: 2026-06-06
- pair: internal/code/funcsig.go::toSet | internal/code/touchpoint.go::keySet
- verdict: false-alarm
- reviewed: 2026-06-06
- pair: internal/pairkey/pairkey.go::SetKey | internal/code/funcsig.go::toSet
- verdict: false-alarm
- reviewed: 2026-06-06
- pair: internal/pairkey/pairkey.go::Key | internal/code/touchpoint.go::keySet
- verdict: false-alarm
- reviewed: 2026-06-06

## runScan ≟ runCheck ≟ runHook ≟ runDoctor — contracted-twin-ok (shared spine)

The N-ary form of the registered runCheck≟runScan pair: all four code-axis command
handlers go through the single-sourced spine — `addBoundaryFlags` (flag taxonomy),
`codeAxis` (the extract→rank→cluster pipeline), `clusterOptsFrom`, `splitCSV`. This
is the dedup *working*: the touchpoint pass first flagged the verbatim flag block
(score 2.83 → factored `addBoundaryFlags`), then the duplicated pipeline (→ factored
`codeAxis`); what remains is intended shared infrastructure, not drift. Pin.
- cluster: cmd/calque/calib.go::runDoctor | cmd/calque/check.go::runCheck | cmd/calque/hook.go::runHook | cmd/calque/scan.go::runScan
- verdict: contracted-twin-ok
- reviewed: 2026-06-06

## calib.go commands (logFires/printDoctor/runDoctor/runMarkFire) — contracted-twin-ok

The calibration functions all resolve `.calque/` via `calqueDir`, append via
`appendJSONL`, and key suspects via `pairDisplayKey`/`clusterDisplayKey`. Intended
shared infra within the calibration leg.
- cluster: cmd/calque/calib.go::logFires | cmd/calque/calib.go::printDoctor | cmd/calque/calib.go::runDoctor | cmd/calque/calib.go::runMarkFire
- verdict: contracted-twin-ok
- reviewed: 2026-06-06

## logFires ≟ runDoctor ≟ runCheck — contracted-twin-ok (suspect-id helpers)

All three id suspects via the single-sourced `pairID`/`clusterID` so the fire log,
the gate output, and doctor agree on a suspect's identity. Intended.
- cluster: cmd/calque/calib.go::logFires | cmd/calque/calib.go::runDoctor | cmd/calque/check.go::runCheck
- verdict: contracted-twin-ok
- reviewed: 2026-06-06

---

## Run — 2026-06-06 (prose gate `vocab-check` added)

The prose-axis gate (cupel vocab-audit equivalent, generalized). The compound
walk→tally is now single-sourced as `tallyCompounds` — the shared helper the
registered runSynonymReport≟runVocabReport note predicted "once a third prose
command appears." The remaining overlaps are the intended prose-axis family and
the prose-gate-mirrors-code-gate parallel.

## Prose command family + shared flags (tally/walk/--exclude) — contracted-twin-ok

The three prose commands share `tallyCompounds`/`locs`; the `--exclude` flag (added
for prose scoping, mirroring the code axis) links them to `addBoundaryFlags` too.
Single-sourced infra (the walk is one `tallyCompounds`; the glob match is one
`internal/glob`). Intended, not drift.
- cluster: cmd/calque/scan.go::addBoundaryFlags | cmd/calque/synonym_report.go::runSynonymReport | cmd/calque/vocab_check.go::runVocabCheck | cmd/calque/vocab_report.go::runVocabReport
- verdict: contracted-twin-ok
- reviewed: 2026-06-07
- pair: cmd/calque/synonym_report.go::runSynonymReport | cmd/calque/vocab_report.go::tallyCompounds
- verdict: contracted-twin-ok
- reviewed: 2026-06-06
- pair: cmd/calque/vocab_check.go::runVocabCheck | cmd/calque/vocab_report.go::runVocabReport
- verdict: contracted-twin-ok
- reviewed: 2026-06-06

## runVocabCheck ≟ runCheck — contracted-twin-ok (prose gate mirrors code gate)

Both are warn-only→`--strict` gates; the prose gate (compounds vs allow-list)
deliberately parallels the code gate (pairs/clusters vs registry). Same spine
shape, different substrate — the §16 unification working, not drift.
- pair: cmd/calque/vocab_check.go::runVocabCheck | cmd/calque/check.go::runCheck
- verdict: contracted-twin-ok
- reviewed: 2026-06-06

## Borderline name-stem collisions among the CLI handlers — false-alarm

`run*`/`build*` handlers share role stems at the 0.18–0.21 noise floor (the gate vs.
the hook's command string; doctor's run/print split). Coincidental name tokens.
- pair: cmd/calque/check.go::runCheck | cmd/calque/hook.go::buildCheckCmd
- verdict: false-alarm
- reviewed: 2026-06-06
- pair: cmd/calque/calib.go::runDoctor | cmd/calque/calib.go::printDoctor
- verdict: false-alarm
- reviewed: 2026-06-06
- pair: cmd/calque/calib.go::runDoctor | cmd/calque/check.go::runCheck
- verdict: contracted-twin-ok
- reviewed: 2026-06-06

## MCP server + compute/render extraction (2026-06-07) — contracted-twin-ok

The `mcp` command shares the gates' cores with the CLI by design: each gate now
splits into compute*/render* (pure core) + run* (CLI wrapper) + mcp* (MCP
wrapper) so the CLI and MCP path CANNOT drift. The parallelism below is the §16
unification made literal, not duplication to collapse.

- pair: cmd/calque/check.go::runCheck | cmd/calque/check.go::computeCheck
- verdict: contracted-twin-ok
- reviewed: 2026-06-07
- pair: cmd/calque/check.go::renderCheck | cmd/calque/check.go::runCheck
- verdict: contracted-twin-ok
- reviewed: 2026-06-07
- pair: cmd/calque/vocab_check.go::computeVocabCheck | cmd/calque/vocab_check.go::runVocabCheck
- verdict: contracted-twin-ok
- reviewed: 2026-06-07
- pair: cmd/calque/vocab_check.go::runVocabCheck | cmd/calque/vocab_check.go::renderVocabCheck
- verdict: contracted-twin-ok
- reviewed: 2026-06-07
- pair: cmd/calque/vocab_check.go::renderVocabCheck | cmd/calque/vocab_check.go::computeVocabCheck
- verdict: contracted-twin-ok
- reviewed: 2026-06-07
- pair: cmd/calque/vocab_check.go::renderVocabCheck | cmd/calque/check.go::renderCheck
- verdict: contracted-twin-ok
- reviewed: 2026-06-07
- pair: cmd/calque/mcp.go::checkToolDefinition | cmd/calque/mcp.go::vocabCheckToolDefinition
- verdict: contracted-twin-ok
- reviewed: 2026-06-07
- pair: cmd/calque/mcp.go::mcpVocabCheck | cmd/calque/mcp.go::mcpCheck
- verdict: contracted-twin-ok
- reviewed: 2026-06-07
- pair: cmd/calque/mcp.go::handleMCP | cmd/calque/mcp.go::runMCP
- verdict: contracted-twin-ok
- reviewed: 2026-06-07
- pair: cmd/calque/mcp.go::vocabCheckToolDefinition | cmd/calque/vocab_check.go::runVocabCheck
- verdict: false-alarm
- reviewed: 2026-06-07

## Tests mirror their subject (2026-06-07) — contracted-twin-ok

A test function structurally echoes the function under test; that is the test
doing its job, not drift. Sibling test cases sharing test scaffolding are
coincidental-token false-alarms.

- pair: cmd/calque/calib_test.go::TestVerdictLabel | cmd/calque/calib.go::verdictLabel
- verdict: contracted-twin-ok
- reviewed: 2026-06-07
- pair: cmd/calque/hook.go::buildCheckCmd | cmd/calque/hook_test.go::TestBuildCheckCmd
- verdict: contracted-twin-ok
- reviewed: 2026-06-07
- pair: cmd/calque/hook_test.go::TestShellQuote | cmd/calque/hook.go::shellQuote
- verdict: contracted-twin-ok
- reviewed: 2026-06-07
- pair: cmd/calque/vocab_check_test.go::TestLoadAllowlist | cmd/calque/vocab_check.go::loadAllowlist
- verdict: contracted-twin-ok
- reviewed: 2026-06-07
- pair: cmd/calque/vocab_check_test.go::TestCompoundViolations | cmd/calque/vocab_check.go::compoundViolations
- verdict: contracted-twin-ok
- reviewed: 2026-06-07
- pair: cmd/calque/hook.go::preCommitScript | cmd/calque/hook_test.go::TestPreCommitScriptGracefulWhenMissing
- verdict: contracted-twin-ok
- reviewed: 2026-06-07
- pair: cmd/calque/vocab_check_test.go::TestRunSeedCmd | cmd/calque/vocab_check_test.go::TestRunSeedCmdError
- verdict: false-alarm
- reviewed: 2026-06-07
- pair: cmd/calque/vocab_check_test.go::TestLoadAllowlistMissing | cmd/calque/vocab_check_test.go::TestLoadAllowlist
- verdict: false-alarm
- reviewed: 2026-06-07
- pair: cmd/calque/calib_test.go::TestVerdictLabel | cmd/calque/calib_test.go::TestFireTagRoundTrip
- verdict: false-alarm
- reviewed: 2026-06-07
- pair: internal/code/touchpoint_test.go::TestTripleShellClustered | internal/code/touchpoint_test.go::makeShell
- verdict: false-alarm
- reviewed: 2026-06-07

## N-ary clusters from the MCP/extraction pass (2026-06-07)

Shared-helper reuse (single-sourced `codeAxis`/`tallyCompounds`/id helpers) and
shared schema/flag/verdict vocabulary — intended reuse, not inlined-seam drift.
Test-fixture and version-string clusters are coincidental-token false-alarms.

- cluster: cmd/calque/mcp.go::checkToolDefinition | cmd/calque/mcp.go::vocabCheckToolDefinition | cmd/calque/scan.go::addBoundaryFlags | cmd/calque/synonym_report.go::runSynonymReport | cmd/calque/vocab_check.go::runVocabCheck | cmd/calque/vocab_report.go::runVocabReport
- verdict: contracted-twin-ok
- reviewed: 2026-06-07
- cluster: cmd/calque/mcp.go::checkToolDefinition | cmd/calque/mcp.go::handleMCP | cmd/calque/mcp.go::vocabCheckToolDefinition | internal/code/extract.py::_extract | internal/code/score.go::Suspicion.Reason | internal/code/score.go::scorePair
- verdict: false-alarm
- reviewed: 2026-06-07
- cluster: internal/code/touchpoint_test.go::TestPublicSymbolNotSeam | internal/code/touchpoint_test.go::TestTripleShellClustered | internal/code/touchpoint_test.go::makeShell
- verdict: false-alarm
- reviewed: 2026-06-07
- cluster: cmd/calque/hook.go::buildCheckCmd | cmd/calque/hook.go::installPreCommit | cmd/calque/main.go::main | cmd/calque/mcp.go::handleMCP
- verdict: false-alarm
- reviewed: 2026-06-07
- cluster: cmd/calque/scan.go::codeAxis | cmd/calque/synonym_report.go::runSynonymReport | cmd/calque/vocab_check.go::computeVocabCheck | cmd/calque/vocab_check.go::runVocabCheck | cmd/calque/vocab_report.go::runVocabReport
- verdict: contracted-twin-ok
- reviewed: 2026-06-07
- cluster: cmd/calque/calib.go::printDoctor | cmd/calque/calib.go::verdictLabel | cmd/calque/calib_test.go::TestFireTagRoundTrip | cmd/calque/calib_test.go::TestVerdictLabel
- verdict: false-alarm
- reviewed: 2026-06-07
- cluster: cmd/calque/calib.go::logFires | cmd/calque/calib.go::runDoctor | cmd/calque/calib_test.go::TestFireIDStable
- verdict: contracted-twin-ok
- reviewed: 2026-06-07
- cluster: cmd/calque/calib.go::logFires | cmd/calque/calib.go::runDoctor | cmd/calque/check.go::renderCheck
- verdict: contracted-twin-ok
- reviewed: 2026-06-07
- cluster: cmd/calque/calib.go::runDoctor | cmd/calque/check.go::computeCheck | cmd/calque/scan.go::runScan
- verdict: contracted-twin-ok
- reviewed: 2026-06-07

## mcp_test.go scaffolding (2026-06-07) — false-alarm

Sibling MCP test cases and clusters sharing protocol/version tokens
(serverInfo, version, calque_check) — test scaffolding, not code drift.

- pair: cmd/calque/mcp_test.go::TestMCPToolsList | cmd/calque/mcp_test.go::TestMCPInitialize
- verdict: false-alarm
- reviewed: 2026-06-07
- pair: cmd/calque/mcp_test.go::TestMCPInitialize | cmd/calque/mcp_test.go::TestMCPUnknownTool
- verdict: false-alarm
- reviewed: 2026-06-07
- pair: cmd/calque/mcp_test.go::TestMCPUnknownTool | cmd/calque/mcp_test.go::TestMCPToolsList
- verdict: false-alarm
- reviewed: 2026-06-07
- cluster: cmd/calque/hook.go::buildCheckCmd | cmd/calque/hook.go::installPreCommit | cmd/calque/main.go::main | cmd/calque/mcp.go::handleMCP | cmd/calque/mcp_test.go::TestMCPInitialize
- verdict: false-alarm
- reviewed: 2026-06-07
- cluster: cmd/calque/mcp.go::checkToolDefinition | cmd/calque/mcp.go::handleMCP | cmd/calque/mcp_test.go::TestMCPToolsList
- verdict: false-alarm
- reviewed: 2026-06-07

## migrate-registry (2026-06-07)

The old-registry migrator. wrapper/core split is intended; it shares the registry
grammar with the canonical parser (must stay in step). The `normalize*`/`verdict`
overlaps are coincidental name tokens with unrelated purposes.

- pair: cmd/calque/migrate.go::migrateRegistry | cmd/calque/migrate.go::runMigrateRegistry
- verdict: contracted-twin-ok
- reviewed: 2026-06-07
- pair: cmd/calque/migrate.go::migrateRegistry | internal/registry/registry.go::Load
- verdict: contracted-twin-ok
- reviewed: 2026-06-07
- pair: cmd/calque/calib.go::verdictLabel | cmd/calque/migrate.go::normalizeVerdict
- verdict: false-alarm
- reviewed: 2026-06-07
- pair: internal/embed/embed.go::normalize | cmd/calque/migrate.go::normalizeReviewed
- verdict: false-alarm
- reviewed: 2026-06-07
- pair: cmd/calque/calib_test.go::TestVerdictLabel | cmd/calque/migrate.go::normalizeVerdict
- verdict: false-alarm
- reviewed: 2026-06-07

## registry zero-parse warning (2026-06-07)

registryParseWarning and migrateRegistry both key on the old-format markers
(`- left:`/`- right:`) — they share that grammar knowledge by design and should
stay in step. Test mirrors its subject.

- pair: cmd/calque/check.go::registryParseWarning | cmd/calque/check_test.go::TestRegistryParseWarning
- verdict: contracted-twin-ok
- reviewed: 2026-06-07
- pair: cmd/calque/check.go::registryParseWarning | cmd/calque/migrate.go::migrateRegistry
- verdict: contracted-twin-ok
- reviewed: 2026-06-07

---

## Role declarations (the cardinality axis — DESIGN_NOTES §18)

Forward invariants for `calque cardinality --repo .`: "role R has exactly N
implementations." Unlike the pairs/clusters above (backward verdicts on discovered
suspects), these are declared up front and the gate enforces them — catching dual
paths that share no footprint, and recurrence (a re-added implementation).

## role: verdict-class-producer
- role: verdict-class-producer
- predicate: qual:/^verdictClass$/
- expected: 1
- reviewed: 2026-06-09

---

## Run — 2026-06-09 (role-cardinality axis added; self-scan churn)

Adding `internal/code/role.go` + `cmd/calque/cardinality.go` introduced new functions;
the self-scan surfaced coincidental name-stem pairs (Match ≟ MatchAny; the deliberate
`compute*`/`run*` core-vs-orchestrator split) and shared-utility clusters (everyone
calls `splitCSV`; predicate-kind literals `name`/`qual`/`file`/… collide with MCP
JSON-schema field strings; command runners share flag-name strings). None is real drift
— all false-alarm, recorded so the self-check stays clean.

- pair: internal/code/role.go::Predicate.Matches | internal/code/role.go::predTerm.matches
- verdict: false-alarm
- reviewed: 2026-06-09
- pair: internal/code/role.go::Match | internal/glob/glob.go::MatchAny
- verdict: false-alarm
- reviewed: 2026-06-09
- pair: cmd/calque/cardinality.go::computeCardinality | cmd/calque/cardinality.go::runCardinality
- verdict: false-alarm
- reviewed: 2026-06-09
- pair: cmd/calque/cardinality.go::runCardinality | cmd/calque/cardinality.go::renderCardinality
- verdict: false-alarm
- reviewed: 2026-06-09
- pair: internal/code/role.go::ParsePredicate | internal/code/role.go::predTerm.matches
- verdict: false-alarm
- reviewed: 2026-06-09
- cluster: cmd/calque/mcp.go::checkToolDefinition | cmd/calque/mcp.go::handleMCP | cmd/calque/mcp.go::vocabCheckToolDefinition | internal/code/extract.py::_extract | internal/code/role.go::ParsePredicate | internal/code/role.go::predTerm.matches | internal/code/score.go::Suspicion.Reason | internal/code/score.go::scorePair
- verdict: false-alarm
- reviewed: 2026-06-09
- cluster: cmd/calque/cardinality.go::runCardinality | cmd/calque/mcp.go::checkToolDefinition | cmd/calque/mcp.go::vocabCheckToolDefinition | cmd/calque/scan.go::addBoundaryFlags | cmd/calque/synonym_report.go::runSynonymReport | cmd/calque/vocab_check.go::runVocabCheck | cmd/calque/vocab_report.go::runVocabReport
- verdict: false-alarm
- reviewed: 2026-06-09
- cluster: cmd/calque/cardinality.go::runCardinality | cmd/calque/scan.go::codeAxis | cmd/calque/synonym_report.go::runSynonymReport | cmd/calque/vocab_check.go::computeVocabCheck | cmd/calque/vocab_check.go::runVocabCheck | cmd/calque/vocab_report.go::runVocabReport
- verdict: false-alarm
- reviewed: 2026-06-09

---

## Run — 2026-06-09 (propose-roles added; self-scan churn)

Adding `cmd/calque/propose.go` (the cluster→role-candidate proposer, task #15) added a
new command runner that joins the same parallel-orchestrator and shared-utility patterns
the prior runs already adjudicated: another `compute*`/`render*`/`run*` triplet (the pure
core / render / orchestrator split — by design, not drift), and `runProposeRoles` joining
the shared-`splitCSV` / shared-`clusterOptsFrom` / shared-flag-string clusters every
command runner sits in. None is real drift — all false-alarm, recorded so the self-check
stays clean. (Test fixtures live in `**/*_test.go`, excluded by the canonical gate.)

- pair: cmd/calque/cardinality.go::runCardinality | cmd/calque/propose.go::runProposeRoles
- verdict: false-alarm
- reviewed: 2026-06-09
- pair: cmd/calque/propose.go::computeProposals | cmd/calque/propose.go::renderProposals
- verdict: false-alarm
- reviewed: 2026-06-09
- pair: cmd/calque/cardinality.go::renderCardinality | cmd/calque/propose.go::renderProposals
- verdict: false-alarm
- reviewed: 2026-06-09
- cluster: cmd/calque/cardinality.go::runCardinality | cmd/calque/mcp.go::checkToolDefinition | cmd/calque/mcp.go::vocabCheckToolDefinition | cmd/calque/propose.go::runProposeRoles | cmd/calque/scan.go::addBoundaryFlags | cmd/calque/synonym_report.go::runSynonymReport | cmd/calque/vocab_check.go::runVocabCheck | cmd/calque/vocab_report.go::runVocabReport
- verdict: false-alarm
- reviewed: 2026-06-09
- cluster: cmd/calque/cardinality.go::runCardinality | cmd/calque/propose.go::runProposeRoles | cmd/calque/scan.go::codeAxis | cmd/calque/synonym_report.go::runSynonymReport | cmd/calque/vocab_check.go::computeVocabCheck | cmd/calque/vocab_check.go::runVocabCheck | cmd/calque/vocab_report.go::runVocabReport
- verdict: false-alarm
- reviewed: 2026-06-09
- cluster: cmd/calque/calib.go::runDoctor | cmd/calque/check.go::computeCheck | cmd/calque/propose.go::runProposeRoles | cmd/calque/scan.go::runScan
- verdict: false-alarm
- reviewed: 2026-06-09

---

## Run — 2026-06-09 (blocking index added; task #16)

The blocking index (`internal/code/block.go`) added a `FuncSig.tokens` method that
gathers a function's tokens across its index channels. Its name stem ("tokens")
coincides with the name-normalization helpers `stemTokens` / `normTokens`, so the
name signal alone fires — but the bodies are unrelated (one walks a `FuncSig`'s
channel sets; the others split a *name* string into role tokens). Name-stem
coincidence, not a dual path. Both false-alarm. (Slice 1 of #16 is output-identical
to the pre-blocking `Rank` — see `TestRankBlockingEquivalence` — so it introduces no
new *pairs*; only the new `tokens` method itself is new surface.)

- pair: internal/code/block.go::FuncSig.tokens | internal/code/funcsig.go::stemTokens
- verdict: false-alarm
- reviewed: 2026-06-09
- pair: internal/code/block.go::FuncSig.tokens | internal/code/funcsig.go::normTokens
- verdict: false-alarm
- reviewed: 2026-06-09

---

## Run — 2026-06-10 (adaptive-weights `calibrate` command added; task #1)

The adaptive-weights leg (`internal/code/calibrate.go` + `cmd/calque/calibrate.go`)
adds a `calibrate` command that reweights the signal channels from adjudicated
registry labels, plus the `applyCalibratedWeights` loader the gate calls. As with
every prior command handler, `runCalibrate` goes through the single-sourced spine
(`addBoundaryFlags`, `codeAxis`, `clusterOptsFrom`, `calqueDir`, `appendJSONL`,
`pairID`/`clusterID`, `loadFireTags`, `resolveLabel`) — so it joins the existing
pinned command-handler families (the N-ary shared-spine clusters at "runScan ≟
runCheck ≟ runHook ≟ runDoctor" and "calib.go commands"). The membership shift
re-keys those clusters, so they re-fire under new set-keys; all are the same
intended shared infrastructure, not drift — pin. The two new pairs are coincidental
name-stem collisions (`verdictClassFor` the cmd-side registry lookup vs the registry
method `verdictClass`; `normalizeWeights` over channel weights vs `normalize` over an
embedding vector) — false-alarm.

- pair: cmd/calque/calibrate.go::verdictClassFor | internal/registry/registry.go::verdictClass
- verdict: false-alarm
- reviewed: 2026-06-10
- pair: internal/code/calibrate.go::normalizeWeights | internal/embed/embed.go::normalize
- verdict: false-alarm
- reviewed: 2026-06-10
- cluster: cmd/calque/calib.go::logFires | cmd/calque/calib.go::printDoctor | cmd/calque/calib.go::runDoctor | cmd/calque/calib.go::runMarkFire | cmd/calque/calibrate.go::runCalibrate | cmd/calque/calibrate.go::writeWeights
- verdict: contracted-twin-ok
- reviewed: 2026-06-10
- cluster: cmd/calque/calib.go::runDoctor | cmd/calque/calibrate.go::runCalibrate | cmd/calque/cardinality.go::runCardinality | cmd/calque/check.go::computeCheck | cmd/calque/propose.go::runProposeRoles | cmd/calque/vocab_check.go::computeVocabCheck | cmd/calque/vocab_check.go::runVocabCheck
- verdict: contracted-twin-ok
- reviewed: 2026-06-10
- cluster: cmd/calque/calib.go::printDoctor | cmd/calque/calib.go::verdictLabel | cmd/calque/calibrate.go::runCalibrate
- verdict: contracted-twin-ok
- reviewed: 2026-06-10
- cluster: cmd/calque/calib.go::logFires | cmd/calque/calib.go::runDoctor | cmd/calque/calibrate.go::runCalibrate | cmd/calque/check.go::renderCheck
- verdict: contracted-twin-ok
- reviewed: 2026-06-10
- cluster: cmd/calque/calib.go::runDoctor | cmd/calque/calibrate.go::runCalibrate | cmd/calque/check.go::runCheck | cmd/calque/hook.go::runHook | cmd/calque/scan.go::runScan
- verdict: contracted-twin-ok
- reviewed: 2026-06-10
- cluster: cmd/calque/calib.go::runDoctor | cmd/calque/calibrate.go::runCalibrate | cmd/calque/check.go::computeCheck | cmd/calque/propose.go::runProposeRoles | cmd/calque/scan.go::runScan
- verdict: contracted-twin-ok
- reviewed: 2026-06-10

---

## Run — 2026-06-10 (registry-GC `prune` command added; post-#1 leverage task)

`calque prune` (`cmd/calque/prune.go`) is the remediation for the staleness `check`
already detects: it re-runs liveness reconciliation and surgically removes dead
`- pair:`/`- cluster:` entries from the registry (dry-run default, `--write` +.bak).
Surfaced by a 2026-06-10 adopter dogfood (38/40 entries stale, no tool to act). As
with every command handler, `runPrune` goes through the shared spine (`addBoundaryFlags`,
`computeCheck`, `joinRepo`, the compute/render split), so it joins the existing pinned
command-handler families — re-keying those N-ary clusters. All contracted-twin-ok, same
intended infrastructure. `runPrune ≟ pruneRegistry` is the deliberate CLI-wrapper ↔
pure-core split (like `runCheck ≟ computeCheck`). `migrateRegistry ≟ pruneRegistry` is a
coincidental name-stem + both-walk-markdown-lines collision at the 0.28 floor — they share
no helper and do opposite transforms (format-convert vs stale-removal) — false-alarm.

- pair: cmd/calque/prune.go::runPrune | cmd/calque/prune.go::pruneRegistry
- verdict: contracted-twin-ok
- reviewed: 2026-06-10
- pair: cmd/calque/migrate.go::migrateRegistry | cmd/calque/prune.go::pruneRegistry
- verdict: false-alarm
- reviewed: 2026-06-10
- cluster: cmd/calque/calib.go::runDoctor | cmd/calque/calibrate.go::runCalibrate | cmd/calque/cardinality.go::runCardinality | cmd/calque/check.go::computeCheck | cmd/calque/propose.go::runProposeRoles | cmd/calque/prune.go::runPrune | cmd/calque/vocab_check.go::computeVocabCheck | cmd/calque/vocab_check.go::runVocabCheck
- verdict: contracted-twin-ok
- reviewed: 2026-06-10
- cluster: cmd/calque/check.go::runCheck | cmd/calque/mcp.go::mcpCheck | cmd/calque/prune.go::runPrune
- verdict: contracted-twin-ok
- reviewed: 2026-06-10
- cluster: cmd/calque/calib.go::runDoctor | cmd/calque/calibrate.go::runCalibrate | cmd/calque/check.go::runCheck | cmd/calque/hook.go::runHook | cmd/calque/prune.go::runPrune | cmd/calque/scan.go::runScan
- verdict: contracted-twin-ok
- reviewed: 2026-06-10

---

## Run — 2026-06-10 (propose-deep Type-4 signature generator added)

`calque propose-deep` (`internal/code/sigcluster.go` + `cmd/calque/propose_deep.go`) is
the representation-independent Type-4 candidate generator — groups functions by rare
domain-typed signature, the contract the jaccard gate is blind to. Validated on an
unseen TypeScript repo (surfaced a `getWorktreeForSession`≟`getWorktreeInfo`
already-drifted bug at jaccard 0.00).
Generator, not gate. New helpers collide on name-stems with existing code (false-alarm) and
runProposeDeep joins the command-handler spine families (contracted-twin-ok).

- pair: internal/code/sigcluster.go::buildOpposed | internal/code/sigcluster.go::opposed
- verdict: false-alarm
- reviewed: 2026-06-10
- pair: internal/code/block.go::FuncSig.tokens | internal/code/sigcluster.go::diffTokens
- verdict: false-alarm
- reviewed: 2026-06-10
- pair: internal/code/funcsig.go::toSet | internal/code/sigcluster.go::tokenSet
- verdict: false-alarm
- reviewed: 2026-06-10
- pair: cmd/calque/propose.go::runProposeRoles | cmd/calque/propose_deep.go::runProposeDeep
- verdict: contracted-twin-ok
- reviewed: 2026-06-10
- cluster: cmd/calque/cardinality.go::runCardinality | cmd/calque/propose.go::runProposeRoles | cmd/calque/propose_deep.go::runProposeDeep | cmd/calque/scan.go::codeAxis | cmd/calque/synonym_report.go::runSynonymReport | cmd/calque/vocab_check.go::computeVocabCheck | cmd/calque/vocab_check.go::runVocabCheck | cmd/calque/vocab_report.go::runVocabReport
- verdict: contracted-twin-ok
- reviewed: 2026-06-10
- cluster: cmd/calque/calib.go::runMarkFire | cmd/calque/cardinality.go::runCardinality | cmd/calque/mcp.go::checkToolDefinition | cmd/calque/propose.go::runProposeRoles | cmd/calque/propose_deep.go::runProposeDeep | cmd/calque/scan.go::addBoundaryFlags
- verdict: contracted-twin-ok
- reviewed: 2026-06-10
- cluster: cmd/calque/calib.go::runDoctor | cmd/calque/calibrate.go::runCalibrate | cmd/calque/cardinality.go::runCardinality | cmd/calque/check.go::runCheck | cmd/calque/mcp.go::checkToolDefinition | cmd/calque/propose.go::runProposeRoles | cmd/calque/propose_deep.go::runProposeDeep | cmd/calque/prune.go::runPrune
- verdict: contracted-twin-ok
- reviewed: 2026-06-10
- cluster: cmd/calque/mcp.go::vocabCheckToolDefinition | cmd/calque/synonym_report.go::runSynonymReport | cmd/calque/vocab_check.go::runVocabCheck | cmd/calque/vocab_report.go::runVocabReport
- verdict: contracted-twin-ok
- reviewed: 2026-06-10

---

## Run — 2026-06-10 (LLM judge / propose-deep --judge added)

The behavioral-equivalence oracle (`internal/llm/judge.go`) — the precision half of the
Type-4 loop. `readCache ≟ writeCache` is the intended read/write pair of one disk cache
(mirror by design). `NewJudge` joins the loose env-reading/constructor cluster (main,
handleMCP, …) via os.Getenv scaffolding — intended, not drift.

- pair: internal/llm/judge.go::Judge.readCache | internal/llm/judge.go::Judge.writeCache
- verdict: contracted-twin-ok
- reviewed: 2026-06-10
- cluster: cmd/calque/hook.go::buildCheckCmd | cmd/calque/hook.go::installPreCommit | cmd/calque/main.go::main | cmd/calque/mcp.go::handleMCP | internal/llm/judge.go::NewJudge
- verdict: contracted-twin-ok
- reviewed: 2026-06-10

---

## Run — 2026-06-10 (judge 3-way taxonomy: drift/contracted-twin-ok/false-alarm)

`parseVerdict` now validates the judge's class against calque's registry verdict
vocabulary, so it switches on the same {drift, contracted-twin-ok, false-alarm} strings
as `normalizeVerdict` (registry-input normalizer) and `verdictLabel` (calibration-label
mapper). Shared ENUM, different jobs — coincidental, not collapsible.

- pair: cmd/calque/migrate.go::normalizeVerdict | internal/llm/judge.go::parseVerdict
- verdict: false-alarm
- reviewed: 2026-06-10
- pair: cmd/calque/calib.go::verdictLabel | internal/llm/judge.go::parseVerdict
- verdict: false-alarm
- reviewed: 2026-06-10

---

## Run — 2026-06-10 (cross-substrate axis: propose-cross + ExtractSymbols)

The cross-substrate axis (v0.3.0): non-function entity extraction (module-level tables
`.py`, JSON corpus shapes `.json`) + the `KeySetCandidates` pass + the generator-only
`propose-cross` command. These entities never enter the scoring gate (separate
`ExtractSymbols` path), so all churn here is from the NEW Go functions colliding with
existing code on name-stems (false-alarm) or joining intentional parallel families
(contracted-twin-ok). Two genuine dual paths the self-scan flagged WERE collapsed, not
adjudicated: `extractPyBatch≟extractPySymbols` → `runPyExtractor`, and the `Extract` /
`ExtractSymbols` walk → `walkExtractable`.

- pair: internal/code/extract_json.go::jsonCollector.walk | internal/corpus/corpus.go::Walk
- verdict: false-alarm
- reviewed: 2026-06-10
- pair: internal/code/scan.go::walkExtractable | internal/corpus/corpus.go::Walk
- verdict: false-alarm
- reviewed: 2026-06-10
- pair: internal/code/extract_json.go::jsonCollector.walk | internal/code/scan.go::walkExtractable
- verdict: false-alarm
- reviewed: 2026-06-10
- pair: internal/code/sigcluster.go::KeySetCandidates | internal/code/touchpoint.go::keySet
- verdict: false-alarm
- reviewed: 2026-06-10
- pair: internal/code/sigcluster.go::KeySetCandidates | internal/pairkey/pairkey.go::SetKey
- verdict: false-alarm
- reviewed: 2026-06-10
- pair: internal/code/symbols.go::ExtractSymbols | internal/code/extract.py::_extract_symbols
- verdict: false-alarm
- reviewed: 2026-06-10
- pair: internal/code/extract_json.go::extractJSONBatch | internal/code/extract.py::_extract
- verdict: false-alarm
- reviewed: 2026-06-10
- pair: internal/code/extract_sql.go::extractSQLBatch | internal/code/extract.py::_extract
- verdict: false-alarm
- reviewed: 2026-06-10
- pair: cmd/calque/propose_cross.go::readEntitySource | cmd/calque/propose_deep.go::readFuncSource
- verdict: false-alarm
- reviewed: 2026-06-10
- pair: internal/code/extract.py::_extract | internal/code/extract.py::_extract_symbols
- verdict: contracted-twin-ok
- reviewed: 2026-06-10
- pair: internal/code/scan.go::Extract | internal/code/symbols.go::ExtractSymbols
- verdict: contracted-twin-ok
- reviewed: 2026-06-10
- pair: cmd/calque/propose_cross.go::runProposeCross | cmd/calque/propose_deep.go::runProposeDeep
- verdict: contracted-twin-ok
- reviewed: 2026-06-10
- pair: cmd/calque/propose.go::runProposeRoles | cmd/calque/propose_cross.go::runProposeCross
- verdict: contracted-twin-ok
- reviewed: 2026-06-10
- cluster: internal/code/extract.py::_extract | internal/code/extract.py::_extract_symbols | internal/code/extract_go.go::goBody.Visit | internal/code/role.go::ParsePredicate | internal/code/role.go::predTerm.matches | internal/code/score.go::Suspicion.Reason | internal/code/score.go::scorePair
- verdict: false-alarm
- reviewed: 2026-06-10
- cluster: cmd/calque/mcp.go::checkToolDefinition | cmd/calque/mcp.go::toolText | cmd/calque/mcp.go::vocabCheckToolDefinition
- verdict: contracted-twin-ok
- reviewed: 2026-06-10
- cluster: cmd/calque/calib.go::runMarkFire | cmd/calque/cardinality.go::runCardinality | cmd/calque/mcp.go::checkToolDefinition | cmd/calque/propose.go::runProposeRoles | cmd/calque/propose_cross.go::runProposeCross | cmd/calque/propose_deep.go::runProposeDeep | cmd/calque/scan.go::addBoundaryFlags
- verdict: contracted-twin-ok
- reviewed: 2026-06-10
- cluster: cmd/calque/vocab_check.go::computeVocabCheck | cmd/calque/vocab_check.go::runVocabCheck | cmd/calque/vocab_report.go::runVocabReport
- verdict: contracted-twin-ok
- reviewed: 2026-06-10
