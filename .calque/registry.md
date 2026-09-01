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

## Suspicion.Reason ≟ scorePair — RESOLVED 2026-06-16 (taxonomy collapsed to one table)
- pair: internal/code/score.go::Suspicion.Reason | internal/code/score.go::scorePair
- verdict: false-alarm
- reviewed: 2026-06-16

WAS drift (reviewed 2026-06-06, PATTERN_CATALOG P2/P3 inherited from the Python
port): the signal taxonomy `{strings,writes,name,calls,ret}` lived in FOUR places —
the `weights` map, scorePair's `sig`+`avail` maps, and Reason's `switch` — so adding
a channel meant editing all four in lockstep. COLLAPSED into one `signals`
`[]signalDef{key,weight,sim,avail,render}` table: scorePair (weighted sum), Reason
(evidence render), `weights`, and `channelOrder` now all DERIVE from it, so a channel
is a single entry and the sites can't drift. The pair still fires only because both
range the shared `signals` table (shared infrastructure, no shared behavioral
contract) → false-alarm. Static prior values and all scores are unchanged — the
calibrate/score/block test suites pass untouched.

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

---

## Run — 2026-06-10 (Go module-level table extraction for the cross-substrate axis)

`extractGoSymbolsFile` (go/ast package-level map/slice tables → the cross-substrate
axis's Go entity source — the analogue of the Python `symbols` mode) is intentionally
parallel to the two existing per-file extractors: `ExtractGoFile` (Go functions) and
`_extract_symbols` (the Python table extractor, same job, different language). Same
parse-then-emit shape, different decl kind — contracted twins, not collapsible (the
shared loop already collapsed into `goBatch`).

- pair: internal/code/extract_go.go::ExtractGoFile | internal/code/extract_go.go::extractGoSymbolsFile
- verdict: contracted-twin-ok
- reviewed: 2026-06-10
- pair: internal/code/extract_go.go::extractGoSymbolsFile | internal/code/extract.py::_extract_symbols
- verdict: contracted-twin-ok
- reviewed: 2026-06-10

---

## Run — 2026-06-14 (Rust function extraction; calque now self-scans its own `syn` helper)

Adding `.rs` extraction makes calque scan its OWN embedded helper crate
(`internal/code/rust-extractor/src/main.rs`) — which deliberately mirrors `extract.py`
and `extract_go.go` so the four substrates emit identical `FuncSig` JSON. That
intentional cross-language parity is what these pairs are; it's *asserted* by
`extract_rust_test.go`, so it's pinned (`contracted-twin-ok`), not collapsible — the
shared core already lives in `runJSONExtractor` on the Go side. (This only fires on
calque scanning its own repo; a user's repo never contains the crate — it's materialized
to a cache dir outside their tree.) The remaining matches are incidental name-stem or
Rust-stdlib-idiom (`.collect()`/`.last()`/`.to_string()`) coincidences → `false-alarm`.

Intentional extractor parity (pinned):

- pair: internal/code/extract.py::_BodyVisitor._record_target | internal/code/rust-extractor/src/main.rs::Body.record_target
- verdict: contracted-twin-ok
- reviewed: 2026-06-14
- pair: internal/code/extract.py::_BodyVisitor.visit_Call | internal/code/rust-extractor/src/main.rs::Body.visit_expr
- verdict: contracted-twin-ok
- reviewed: 2026-06-14
- pair: internal/code/extract_go.go::recvTypeName | internal/code/rust-extractor/src/main.rs::type_name
- verdict: contracted-twin-ok
- reviewed: 2026-06-14
- pair: internal/code/extract_go.go::goBody.recordTarget | internal/code/rust-extractor/src/main.rs::Body.record_target
- verdict: contracted-twin-ok
- reviewed: 2026-06-14
- pair: internal/code/extract_go.go::goBody.Visit | internal/code/rust-extractor/src/main.rs::Body.visit_expr
- verdict: contracted-twin-ok
- reviewed: 2026-06-14

Incidental coincidences (suppressed):

- pair: internal/corpus/corpus.go::RelPath | internal/code/rust-extractor/src/main.rs::rel_path
- verdict: false-alarm
- reviewed: 2026-06-14
- pair: cmd/calque/main.go::main | internal/code/rust-extractor/src/main.rs::main
- verdict: false-alarm
- reviewed: 2026-06-14
- pair: internal/code/extract.py::main | internal/code/rust-extractor/src/main.rs::main
- verdict: false-alarm
- reviewed: 2026-06-14
- pair: internal/corpus/corpus.go::Walk | internal/code/rust-extractor/src/main.rs::walk_items
- verdict: false-alarm
- reviewed: 2026-06-14
- pair: internal/code/extract_json.go::jsonCollector.walk | internal/code/rust-extractor/src/main.rs::walk_items
- verdict: false-alarm
- reviewed: 2026-06-14
- pair: internal/code/extract_rust.go::rustExtractorBin | internal/code/extract_rust.go::buildRustExtractor
- verdict: false-alarm
- reviewed: 2026-06-14
- cluster: internal/code/rust-extractor/src/main.rs::Body.visit_expr | internal/code/rust-extractor/src/main.rs::emit_fn | internal/code/rust-extractor/src/main.rs::main | internal/code/rust-extractor/src/main.rs::member_str | internal/code/rust-extractor/src/main.rs::rel_path | internal/code/rust-extractor/src/main.rs::type_name
- verdict: false-alarm
- reviewed: 2026-06-14
- cluster: cmd/calque/hook.go::buildCheckCmd | cmd/calque/hook.go::installPreCommit | cmd/calque/main.go::main | cmd/calque/mcp.go::handleMCP | internal/code/extract_rust.go::rustExtractorBin | internal/llm/judge.go::NewJudge
- verdict: false-alarm
- reviewed: 2026-06-14
- cluster: internal/code/rust-extractor/src/main.rs::Body.record_target | internal/code/rust-extractor/src/main.rs::Body.visit_expr | internal/code/rust-extractor/src/main.rs::field_members
- verdict: false-alarm
- reviewed: 2026-06-14

## Run — 2026-06-15 (reads axis — value-derivation drift detection)

Adding the `reads` signal + the `SharedDerivationCandidates` pass + `propose-deriv` extends
the four extractors (a `reads` collector mirroring `writes`) and adds one CLI command. The
new self-scan clusters are the same intentional-parity / shared-helper shapes already
recorded above, now with the new members joined:

- The `propose-*` CLI-flag-parsing cluster gains `runProposeDeriv` (same boilerplate as
  `runProposeDeep`/`runProposeCross`) → `contracted-twin-ok` (matches the existing verdict).
- The Python extractor visitors share the `_record_target` / `_attr_path` HELPERS — that
  shared seam IS the single authority (the opposite of drift); my `visit_Attribute` /
  `visit_AugAssign` edits joined those clusters → `false-alarm`.
- The Rust extractor's own `read_set`/`emit_fn`/`main` share Rust-stdlib idioms
  (`.collect()`/`.cloned()`) → `false-alarm` (same as the prior main.rs clusters).

CLI-command parity (pinned):

- cluster: cmd/calque/calib.go::runMarkFire | cmd/calque/cardinality.go::runCardinality | cmd/calque/mcp.go::checkToolDefinition | cmd/calque/propose.go::runProposeRoles | cmd/calque/propose_cross.go::runProposeCross | cmd/calque/propose_deep.go::runProposeDeep | cmd/calque/propose_deriv.go::runProposeDeriv | cmd/calque/scan.go::addBoundaryFlags
- verdict: contracted-twin-ok
- reviewed: 2026-06-15

Shared-helper usage + stdlib-idiom coincidences (suppressed):

- cluster: internal/code/rust-extractor/src/main.rs::Body.read_set | internal/code/rust-extractor/src/main.rs::emit_fn | internal/code/rust-extractor/src/main.rs::main
- verdict: false-alarm
- reviewed: 2026-06-15
- cluster: internal/code/extract.py::_BodyVisitor._record_target | internal/code/extract.py::_BodyVisitor.visit_Assign | internal/code/extract.py::_BodyVisitor.visit_AugAssign
- verdict: false-alarm
- reviewed: 2026-06-15
- cluster: internal/code/extract.py::_BodyVisitor._record_target | internal/code/extract.py::_BodyVisitor.visit_Attribute | internal/code/extract.py::_BodyVisitor.visit_AugAssign | internal/code/extract.py::_BodyVisitor.visit_Call
- verdict: false-alarm
- reviewed: 2026-06-15

Pairwise findings from the new code (one was a REAL dual path I introduced and collapsed —
`sharedKeysLabel`/`sharedReadsLabel` are now one `sharedSetLabel(as, bs set)`; the rest are
intentional CLI parity or incidental coincidence):

- pair: cmd/calque/propose_cross.go::runProposeCross | cmd/calque/propose_deriv.go::runProposeDeriv
- verdict: contracted-twin-ok
- reviewed: 2026-06-15
- pair: cmd/calque/propose_deep.go::runProposeDeep | cmd/calque/propose_deriv.go::runProposeDeriv
- verdict: contracted-twin-ok
- reviewed: 2026-06-15
- pair: internal/code/extract.py::_BodyVisitor.visit_Assign | internal/code/extract.py::_BodyVisitor.visit_AugAssign
- verdict: false-alarm
- reviewed: 2026-06-15
- pair: internal/code/funcsig.go::toSet | internal/code/rust-extractor/src/main.rs::Body.read_set
- verdict: false-alarm
- reviewed: 2026-06-15
- cluster: internal/code/score.go::scorePair | internal/code/sigcluster.go::KeySetCandidates | internal/code/sigcluster.go::NameStemCandidates | internal/code/sigcluster.go::SharedDerivationCandidates
- verdict: contracted-twin-ok
- reviewed: 2026-06-15

## Run — 2026-06-16 (confession axis + operation-type gate)

The confession axis (`confess.go`) adds a line-reader; the op-type gate (`optype.go`)
adds a classifier; the `confess` command joins the propose-* CLI family. New self-scan
pairs are file-read boilerplate (no behavioral contract that can drift → false-alarm) or
the CLI-command parity already recorded for the propose-* runners.

- pair: cmd/calque/propose_deep.go::readFuncSource | internal/code/confess.go::readSourceLines
- verdict: false-alarm
- reviewed: 2026-06-16
- pair: cmd/calque/propose_cross.go::readEntitySource | internal/code/confess.go::readSourceLines
- verdict: false-alarm
- reviewed: 2026-06-16
- pair: internal/code/optype.go::opType | internal/code/optype.go::opposedOps
- verdict: false-alarm
- reviewed: 2026-06-16
- cluster: cmd/calque/confess.go::runConfess | cmd/calque/propose_cross.go::runProposeCross | cmd/calque/propose_deep.go::runJudge | cmd/calque/propose_deep.go::runProposeDeep | cmd/calque/propose_deriv.go::runProposeDeriv
- verdict: contracted-twin-ok
- reviewed: 2026-06-16

## Run — 2026-06-16 (Layer D: label store + doctor --ablate)

`labels.go` (recordLabel/labelStorePath) and `ablate.go` surface five new self-scan
fires. All false-alarm: incidental shared vocabulary/helpers or one feature's own
cohesive internals — no independent contract that can drift.

- `ablateCell.add` ≟ `normalizeVerdict` share the verdict-class string vocabulary
  ("drift"/"contracted-twin-ok"/"false-alarm") but do different jobs (tally vs free-form
  normalize). NOTE: that vocabulary is now hardcoded in ~4 sites (here, normalizeVerdict,
  verdictLabel, the judge parser) — a genuine Layer B shared-const candidate, tracked.
- `ablateVerdict` ≟ `runAblate` and the precision/total cluster are one feature's helper
  + caller (a layering relationship), glued by the `ablate`/`precision`/`total` stems.
- The buildVersion/version/`calque` 7-member cluster is glued by the `"calque"` cache-dir
  literal + version symbols across unrelated funcs (labelStorePath just joined it).
- logFires/runMarkFire/recordLabel all DELEGATE to the shared `appendJSONL`+`nowTs`
  helpers, each writing its own JSONL schema — shared infrastructure, not duplicated logic.

- pair: cmd/calque/ablate.go::ablateCell.add | cmd/calque/migrate.go::normalizeVerdict
- verdict: false-alarm
- reviewed: 2026-06-16
- pair: cmd/calque/ablate.go::ablateVerdict | cmd/calque/ablate.go::runAblate
- verdict: false-alarm
- reviewed: 2026-06-16
- cluster: cmd/calque/hook.go::buildCheckCmd | cmd/calque/hook.go::installPreCommit | cmd/calque/labels.go::labelStorePath | cmd/calque/main.go::main | cmd/calque/mcp.go::handleMCP | internal/code/extract_rust.go::rustExtractorBin | internal/llm/judge.go::NewJudge
- verdict: false-alarm
- reviewed: 2026-06-16
- cluster: cmd/calque/ablate.go::ablateCell.precision | cmd/calque/ablate.go::ablateVerdict | cmd/calque/ablate.go::runAblate
- verdict: false-alarm
- reviewed: 2026-06-16
- cluster: cmd/calque/calib.go::logFires | cmd/calque/calib.go::runMarkFire | cmd/calque/labels.go::recordLabel
- verdict: false-alarm
- reviewed: 2026-06-16

## Run — 2026-06-16 (table-driven scorer refactor fallout)

Collapsing the signal taxonomy into `signals` (extracting `nameSim`) shifts three
self-scan fires, all false-alarm:

- `normalizeVerdict` ≟ `embed.normalize` share only the `normalize` name stem — one
  reduces a free-form verdict to its token, the other L2-normalizes a vector.
- The 8-member MCP/extract/role/score cluster is the recurring vocabulary cluster
  (glued by FuncSig field names: `qualname`/`ret_keys`/`strings`/…); scorePair's
  membership shifted when its `sig`/`avail` map literals collapsed into the table.
- `nameSim` joins KeySet/NameStem/SharedDerivation candidates via the shared
  `jaccard`/`sharedSetLabel` helpers — shared infrastructure, not a shared contract.

- pair: cmd/calque/migrate.go::normalizeVerdict | internal/embed/embed.go::normalize
- verdict: false-alarm
- reviewed: 2026-06-16
- cluster: cmd/calque/mcp.go::checkToolDefinition | cmd/calque/mcp.go::handleMCP | cmd/calque/mcp.go::vocabCheckToolDefinition | internal/code/extract.py::_extract | internal/code/extract.py::_extract_symbols | internal/code/role.go::ParsePredicate | internal/code/role.go::predTerm.matches | internal/code/score.go::scorePair
- verdict: false-alarm
- reviewed: 2026-06-16
- cluster: internal/code/score.go::nameSim | internal/code/sigcluster.go::KeySetCandidates | internal/code/sigcluster.go::NameStemCandidates | internal/code/sigcluster.go::SharedDerivationCandidates
- verdict: false-alarm
- reviewed: 2026-06-16

## Run — 2026-06-16 (confess --judge — comment axis precision half)

Wiring `--judge`/`--top`/`--twins-only`/`--include-tests` + registry-dedup into
`runConfess` makes it structurally mirror its sibling `run*`-`--judge` subcommands.
All three fires are intentional parity, not drift — every CLI subcommand parses flags,
excludes tests, dedups vs the registry, and routes through the ONE shared `runJudge`
authority (delegation, the opposite of duplicated logic):

- `runConfess` ≟ `runProposeCross` / `runProposeDeep` — siblings share the flag-parse +
  judge-dispatch shape; the judging logic lives once in `runJudge`, called by all.
- The 5-member cluster (the four `run*` commands + `NewJudge`) is the shared-judge seam:
  every subcommand delegating to one oracle constructor is correct shared infrastructure.

- pair: cmd/calque/confess.go::runConfess | cmd/calque/propose_cross.go::runProposeCross
- verdict: false-alarm
- reviewed: 2026-06-16
- pair: cmd/calque/confess.go::runConfess | cmd/calque/propose_deep.go::runProposeDeep
- verdict: false-alarm
- reviewed: 2026-06-16
- cluster: cmd/calque/confess.go::runConfess | cmd/calque/propose_cross.go::runProposeCross | cmd/calque/propose_deep.go::runProposeDeep | cmd/calque/propose_deriv.go::runProposeDeriv | internal/llm/judge.go::NewJudge
- verdict: false-alarm
- reviewed: 2026-06-16

## Run — 2026-06-16 (confess register discriminator)

Adding confessionRegister (line vs prose register classifier) next to
ConfessionCandidates makes them share the "confession" name stem — name-adjacency,
not a contract. One builds directed twin candidates; the other classifies a single
comment line's register. False-alarm.

- pair: internal/code/confess.go::ConfessionCandidates | internal/code/confess.go::confessionRegister
- verdict: false-alarm
- reviewed: 2026-06-16

## Run — 2026-06-16 (const-set axis / item 13)

Adding the `consts` seam channel (referenced SCREAMING_SNAKE domain constants) means
the field-name string literals "consts" and "calls" now appear across the extractor
plumbing — ParsePredicate/predTerm.matches list them as predicate kinds; goBody.Visit
and the Python extractors emit a "consts" key. The five functions cluster on those
incidental literals, not on a shared computation. False-alarm (channel-plumbing token
coincidence, the same shape as prior extractor-mirror false-alarms).

- cluster: internal/code/extract.py::_extract | internal/code/extract.py::_extract_symbols | internal/code/extract_go.go::goBody.Visit | internal/code/role.go::ParsePredicate | internal/code/role.go::predTerm.matches
- verdict: false-alarm
- reviewed: 2026-06-16

## Run — 2026-06-16 (const declaration gate / item 16)

Gating the const seam channel on project-declaration (`decl_consts`) makes
`_is_const` a shared call seam across the three Python functions that validate
const-shape: `visit_Attribute` and `visit_Name` (reference-side capture) and
`_extract` (the new module-scope decl collection). They share a validation helper,
not a derivation — channel-plumbing token coincidence, the same shape as the prior
extractor-mirror false-alarms. The second cluster is the standing judge-wiring family:
six command entrypoints share `runJudge`/`NewJudge` because they all wire up the LLM
oracle — shared infrastructure, not a contract. Both false-alarm.

Two name-noise pairs come with it: `goDeclConsts ≟ collect_decl_consts` is the
deliberate cross-substrate extractor mirror (Go's go/ast vs Rust's syn collecting the
same file-scope const declarations — zero shared footprint, name-only, test-pinned per
substrate), and `judgeClusters ≟ runJudge` shares the judge-invocation plumbing over
different candidate shapes (N-ary cluster reps vs pairs). Both false-alarm.

- cluster: internal/code/extract.py::_BodyVisitor.visit_Attribute | internal/code/extract.py::_BodyVisitor.visit_Name | internal/code/extract.py::_extract
- verdict: false-alarm
- reviewed: 2026-06-16
- cluster: cmd/calque/confess.go::runConfess | cmd/calque/propose.go::runProposeRoles | cmd/calque/propose_cross.go::runProposeCross | cmd/calque/propose_deep.go::runProposeDeep | cmd/calque/propose_deriv.go::runProposeDeriv | internal/llm/judge.go::NewJudge
- verdict: false-alarm
- reviewed: 2026-06-16
- pair: internal/code/extract_go.go::goDeclConsts | internal/code/rust-extractor/src/main.rs::collect_decl_consts
- verdict: false-alarm
- reviewed: 2026-06-16
- pair: cmd/calque/propose.go::judgeClusters | cmd/calque/propose_deep.go::runJudge
- verdict: false-alarm
- reviewed: 2026-06-16

## Run — 2026-06-18 (test-awareness + false-alarm hints)

Two name-stem coincidences from this session's new code, both false-alarm.
`sameReceiver ≟ receiverOf` (falsealarm.go): sameReceiver merely CALLS receiverOf to
extract the type prefix — a helper call, not a parallel contract. `has_cfg_test ≟
has_test_attr` (rust-extractor): both are attribute predicates sharing a has/test
stem and iter/any plumbing, but answer DIFFERENT questions (a #[cfg(test)] module vs
a #[test]-family attribute on the fn) — no shared invariant to drift.

- pair: internal/code/falsealarm.go::sameReceiver | internal/code/falsealarm.go::receiverOf
- verdict: false-alarm
- reviewed: 2026-06-18
- pair: internal/code/rust-extractor/src/main.rs::has_cfg_test | internal/code/rust-extractor/src/main.rs::has_test_attr
- verdict: false-alarm
- reviewed: 2026-06-18

## Run — 2026-06-19 (boundary-bite warning + sveltekit-handler tag)

Three incidental coincidences from this session's new code (the adopter-requested
zero-parse boundary warning + the SvelteKit route-handler structural tag), all
false-alarm. `HasExtractor ≟ Registry.Has`: a one-line map-membership predicate
sharing only the "has" stem with the registry's pair lookup — unrelated maps, no
contract. `Match ≟ MatchGlob`: role.Match (does a FuncSig fit a cardinality role)
vs MatchGlob (which file paths match a glob) share the "match" stem and nothing
else. The cluster `checkToolDefinition | addBoundaryFlags | boundaryBiteWarnings`
fires only because all three name the boundary-flag params `left`/`right` — shared
plumbing vocabulary, not a shared seam.

- pair: internal/code/scan.go::HasExtractor | internal/registry/registry.go::Registry.Has
- verdict: false-alarm
- reviewed: 2026-06-19
- pair: internal/code/role.go::Match | internal/code/scan.go::MatchGlob
- verdict: false-alarm
- reviewed: 2026-06-19
- cluster: cmd/calque/mcp.go::checkToolDefinition | cmd/calque/scan.go::addBoundaryFlags | cmd/calque/scan.go::boundaryBiteWarnings
- verdict: false-alarm
- reviewed: 2026-06-19

## Run — 2026-06-24 (post-merge hook install)

One incidental cluster from the post-merge hook work. The new `installGitHook`
helper (shared by the pre-commit and post-merge installers) joined a coincidental
seam keyed on the literal string `"calque"` plus `version`/`buildVersion` — every
member just emits a user-facing `"calque …"` message or touches the version
string. `buildCheckCmd`, `main`, `handleMCP`, `labelStorePath`, `rustExtractorBin`,
and `NewJudge` share no contract with the hook installer or each other; it's the
CLI's own brand string surfacing as a seam, not a dual path.

- cluster: cmd/calque/hook.go::buildCheckCmd | cmd/calque/hook.go::installGitHook | cmd/calque/labels.go::labelStorePath | cmd/calque/main.go::main | cmd/calque/mcp.go::handleMCP | internal/code/extract_rust.go::rustExtractorBin | internal/llm/judge.go::NewJudge
- verdict: false-alarm
- reviewed: 2026-06-24

## Run — 2026-06-24 (review emitter + shared addCheckFlags collapse)

Adding `calque review` (the CI/PR GitHub-annotations surface) duplicated check's
flag surface; calque flagged `runCheck ≟ runReview` on its own source, so the
shared knobs were collapsed into one `addCheckFlags` helper both call (the direct
pair then dropped off). The remainder below are all false-alarm: coincidental
shared idioms/strings, or — better — functions correctly funnelling through a
single shared authority (computeCheck / ghAnnotation), which is the opposite of
drift.

- pair: cmd/calque/review.go::ghEscapeData | cmd/calque/review.go::ghEscapeProp
- verdict: false-alarm
- reviewed: 2026-06-24
  (ghEscapeProp delegates to ghEscapeData then adds two replacements — composition,
   not two implementations of one contract.)
- pair: cmd/calque/review.go::emitPairAnnotations | cmd/calque/review.go::emitClusterAnnotations
- verdict: false-alarm
- reviewed: 2026-06-24
  (parallel emitters that both call ghAnnotation; the annotation FORMAT is
   single-sourced in ghAnnotation, so there is no contract to drift.)
- pair: cmd/calque/calib.go::runDoctor | cmd/calque/check.go::addCheckFlags
- verdict: false-alarm
- reviewed: 2026-06-24
  (shared generic flag-help strings; unrelated functions.)
- pair: cmd/calque/check.go::addCheckFlags | cmd/calque/scan.go::addBoundaryFlags
- verdict: false-alarm
- reviewed: 2026-06-24
  (same "add a group of flags to a FlagSet" idiom over DISJOINT flags — no shared
   contract to drift.)
- cluster: cmd/calque/calib.go::runDoctor | cmd/calque/calibrate.go::runCalibrate | cmd/calque/check.go::runCheck | cmd/calque/hook.go::runHook | cmd/calque/prune.go::runPrune | cmd/calque/review.go::runReview | cmd/calque/scan.go::runScan
- verdict: false-alarm
- reviewed: 2026-06-24
  (top-level subcommand handlers sharing CLI scaffolding — addBoundaryFlags,
   applyCalibratedWeights; parallel entry points, not a drifting contract.)
- cluster: cmd/calque/check.go::runCheck | cmd/calque/mcp.go::mcpCheck | cmd/calque/prune.go::runPrune | cmd/calque/review.go::runReview
- verdict: false-alarm
- reviewed: 2026-06-24
  (all consumers of the single computeCheck/renderCheck core — correct
   single-sourcing, the intended shape, not duplication.)
- cluster: cmd/calque/review.go::emitClusterAnnotations | cmd/calque/review.go::emitPairAnnotations | cmd/calque/review.go::runReview | cmd/calque/vocab_check.go::runVocabCheck
- verdict: false-alarm
- reviewed: 2026-06-24
  (the review emitters cluster around the shared ghAnnotation authority;
   runVocabCheck joins coincidentally on the generic string "warning".)

## Run — 2026-06-24 (review step-summary panel)

- pair: cmd/calque/review.go::writeStepSummary | cmd/calque/review.go::renderStepSummary
- verdict: false-alarm
- reviewed: 2026-06-24
  (writeStepSummary does the env-check + file append around the pure
   renderStepSummary; render/write split for testability — single authority for
   the panel content is renderStepSummary, nothing to drift.)

## Run — 2026-06-27 (Nearest author-time core)

- pair: internal/code/score.go::Rank | internal/code/score.go::scoreAndRank
- verdict: false-alarm
- reviewed: 2026-06-27
  (scoreAndRank was extracted FROM Rank as the shared score→gate→sort→dedup→top
   spine; Rank now calls it. The flag fires on the name stem "rank" + the shared
   call — but the logic lives in ONE place (scoreAndRank), so there is no contract
   to drift. This is the collapse, not the duplication — exactly the resolution
   the author-time Nearest path is meant to drive callers toward.)

## Run — 2026-06-27 (ExtractCached index-cache)

- pair: internal/code/cache.go::ExtractCached | internal/code/scan.go::Extract
- verdict: false-alarm
- reviewed: 2026-06-27
  (ExtractCached is the cached variant of Extract. The genuinely shared logic was
   COLLAPSED: the tree-walk + skip-stat accounting into walkSources, the sig
   finalize + test-attribution into prepareSigs, the atomic write into atomicWrite.
   What remains (0.73) is the irreducible "loop byExt → run extractor → accumulate
   Files/Funcs/CodeFiles" skeleton common to any repo extractor — structure, not a
   contract that can drift. Extract must NOT delegate to ExtractCached: that would
   force cache writes on every uncached caller.)
- pair: internal/code/cache.go::cacheKey | internal/llm/judge.go::Judge.cacheKey
- verdict: false-alarm
- reviewed: 2026-06-27
  (name-only collision across packages; never share a file. code.cacheKey maps a
   walk path to the FuncSig.File-relative form; Judge.cacheKey builds an LLM
   judge-cache key. Disjoint domains.)
- pair: internal/code/cache.go::cacheKey | internal/pairkey/pairkey.go::Key
- verdict: false-alarm
- reviewed: 2026-06-27
  (name stem "key" only; pairkey.Key canonicalizes an unordered pair, cacheKey
   relativizes a path. No shared logic.)
- pair: internal/code/cache.go::loadIndexCache | internal/code/cache.go::saveIndexCache
- verdict: false-alarm
- reviewed: 2026-06-27
  (deliberate load/save inverses sharing the "index cache" name stem; opposite
   operations over the same on-disk format — the format authority is the indexCache
   type, nothing to drift.)
- pair: internal/code/cache.go::ExtractCached | internal/code/symbols.go::ExtractSymbols
- verdict: false-alarm
- reviewed: 2026-06-27
  (both consume the INTENDED single-source walkExtractable — that is its whole
   purpose — plus share the "extract" stem; different outputs, FuncSigs vs symbols.)
- pair: internal/code/scan.go::walkSources | internal/corpus/corpus.go::Walk
- verdict: false-alarm
- reviewed: 2026-06-27
  (name stem "walk" only; corpus.Walk walks a prose corpus, walkSources groups code
   files by extension. Different package, different domain.)

## Run — 2026-06-27 (nearest author-time command)

- pair: cmd/calque/nearest.go::runNearest | internal/code/nearest.go::Nearest
- verdict: false-alarm
- reviewed: 2026-06-27
  (runNearest is the CLI wrapper — flag parsing, PreToolUse-payload decode, pending
   compose, output formatting — that CALLS the pure recall core code.Nearest. The
   flag fires on the shared "nearest" name + that one call; command-wraps-core is
   the correct layering, no duplicated logic.)
- cluster: internal/code/cache.go::ExtractCached | internal/code/scan.go::Extract | internal/code/scan.go::ExtractPending
- verdict: false-alarm
- reviewed: 2026-06-27
  (the three extraction entry points cluster around the INTENDED single-source
   helpers walkSources/prepareSigs — the very collapse this session performed.
   Sharing the authority is correct single-sourcing, the opposite of drift.)
- cluster: cmd/calque/hook.go::runHook | cmd/calque/main.go::main | cmd/calque/nearest.go::runNearest
- verdict: false-alarm
- reviewed: 2026-06-27
  (main is the command dispatcher that calls both runHook and runNearest; they
   cluster on the shared "hook"/"nearest" command tokens + the dispatch edge —
   parallel CLI entry points around the single main switch, same shape as the
   already-adjudicated subcommand-handler cluster. No shared contract to drift.)

## Run — 2026-07-02 (jscpd/dupl companion pass)

- pair: internal/code/extract_shell.go::runJSONExtractor | internal/companion/companion.go::runTool
- verdict: false-alarm
- reviewed: 2026-07-02
  (both are generic "run a subprocess, capture output" plumbing — same shape by
   necessity for anything that shells out — but serve unrelated contracts:
   runJSONExtractor parses a per-extractor JSON protocol (node/python), runTool
   passes through an external clone-detector's raw report verbatim. Nothing to
   keep in sync between them.)

## Run — 2026-07-06 (distance-decay score boost + AST-node-count size gate)

Adding `NodeCount` (one counter increment riding the existing per-node body-walk
in all four extractors) retriggers the cross-language visitor-twin category
already established below (`goBody.Visit ≟ _BodyVisitor.visit_Call`, "More
cross-language visitor twins"): Python's `_BodyVisitor` gains a `visit`
dispatcher override (the single choke point `ast.NodeVisitor` calls for every
node) and Rust's `Body` gains a `visit_stmt` override alongside its existing
`visit_expr` — both join the same already-adjudicated shape, not new drift.

- pair: internal/code/extract.py::_BodyVisitor.visit | internal/code/rust-extractor/src/main.rs::Body.visit_expr
- verdict: contracted-twin-ok
- reviewed: 2026-07-06
- pair: internal/code/extract.py::_BodyVisitor.visit | internal/code/rust-extractor/src/main.rs::Body.visit_stmt
- verdict: contracted-twin-ok
- reviewed: 2026-07-06
- pair: internal/code/extract.py::_BodyVisitor.visit | internal/code/extract_ts.mjs::BodyVisitor.visit
- verdict: contracted-twin-ok
- reviewed: 2026-07-06
- pair: internal/code/extract.py::_BodyVisitor.visit | internal/code/extract_ts.mjs::BodyVisitor.visitBody
- verdict: contracted-twin-ok
- reviewed: 2026-07-06
- pair: internal/code/extract_go.go::goBody.Visit | internal/code/extract.py::_BodyVisitor.visit
- verdict: contracted-twin-ok
- reviewed: 2026-07-06
  (five-way cross-language visitor parity — Python's new single-dispatcher
   `visit` override joins the same "same interchange, different language"
   category as the existing goBody.Visit/BodyVisitor.visit_Call twins above.
   Intentional: a future field added to one visitor's counting logic should be
   mirrored in the others, same as every other channel they already share.)
- pair: internal/code/rust-extractor/src/main.rs::Body.visit_expr | internal/code/rust-extractor/src/main.rs::Body.visit_stmt
- verdict: false-alarm
- reviewed: 2026-07-06
  (same file, same `Body` visitor struct — syn's `Visit` trait requires separate
   callbacks for expression vs statement nodes; both do the same one-line
   "increment then delegate to the default traversal" and are maintained
   together by construction, not two implementations that could drift apart.)
- pair: internal/code/distance.go::dirHops | internal/code/distance_test.go::TestDirHops
- verdict: false-alarm
- reviewed: 2026-07-06
  (a function and its own dedicated unit test share name/call signal by
   construction — the general function-vs-its-test shape, not drift.)
- cluster: internal/code/extract_py.go::runPyExtractor | internal/code/extract_py_test.go::TestExtractPyConsts | internal/code/extract_py_test.go::TestExtractPyReadsSkipsCallee | internal/code/nodecount_test.go::TestExtractPyNodeCountDiscriminatesFromNLines | internal/code/symbols_test.go::TestExtractSymbolsPyTables
- verdict: false-alarm
- reviewed: 2026-07-06
  (the new NodeCount parity test shares the `extractPyBatch`/`pythonBin` seam
   with the other Python-extractor tests — same generic-subprocess-plumbing
   category as the existing rust-extractor clusters above, just on the Python
   side; joining it is expected, not a new shared contract to drift.)
- cluster: internal/code/block_test.go::naiveRank | internal/code/extract_ts.mjs::maskSvelteScript | internal/code/nearest.go::Nearest | internal/code/score.go::Rank | internal/code/sigcluster.go::NameStemCandidates | internal/code/sigcluster.go::SignatureCandidates | internal/code/touchpoint.go::ClusterByTouchpoint
- verdict: false-alarm
- reviewed: 2026-07-06
  (the SizeGate refactor collapsed four near-identical `f.NLines >= minLines &&
   !strings.HasPrefix(f.Name, "__")` inline filters into one `SizeGate.keep`
   method, now called from Rank/Nearest/NameStemCandidates/SignatureCandidates/
   ClusterByTouchpoint — the cluster grew because the shared seam is now
   literally the SAME function (single-sourced), not five drifting copies. The
   best-case outcome of this refactor, not a new risk.)

## Run — 2026-07-06 (--distance-boost dogfood, one-time opt-in adjudication)

Enabling `--distance-boost` on this repo before deciding whether to adopt it,
per the design's own default-off rationale. Isolated the effect precisely by
diffing `check --distance-boost` against a plain `check` run: exactly 20 pairs
are surfaced by the boost alone (73 → 93 new), all landing at score 0.18–0.20 —
right at the `--min-score` edge, confirming the boost does what it's designed to
do (nudge marginal pairs), not manufacture unrelated ones (`hasAnchor` still
gates first). On THIS codebase the marginal band the boost pulls across is
dominated by single-shared-token name-stem coincidences (`visit`, `key`, `has`,
`all`, `label`, `judge`, `nearest`, `shell`, `alarm+false`) between functions in
unrelated cross-directory files — exactly the failure mode a name-stem-heavy
scorer risks once distance stops discriminating against it. Two pairs are real:
the cross-language visitor-twin category (§ above) gains its TS↔Rust and Go↔Rust
legs now that `visit`/`visit_stmt` exist on both sides.

Cross-language visitor twins (extends the existing category above):

- pair: internal/code/extract_ts.mjs::BodyVisitor.visit | internal/code/rust-extractor/src/main.rs::Body.visit_stmt
- verdict: contracted-twin-ok
- reviewed: 2026-07-06
- pair: internal/code/extract_go.go::goBody.Visit | internal/code/rust-extractor/src/main.rs::Body.visit_stmt
- verdict: contracted-twin-ok
- reviewed: 2026-07-06

Same-class dispatch siblings (Python's new `visit` override vs its own
pre-existing `visit_X` handlers — same file, same receiver, flagged
`same-receiver` by the structural hint already):

- pair: internal/code/extract.py::_BodyVisitor.visit | internal/code/extract.py::_BodyVisitor.visit_Assign
- verdict: false-alarm
- reviewed: 2026-07-06
- pair: internal/code/extract.py::_BodyVisitor.visit | internal/code/extract.py::_BodyVisitor.visit_Call
- verdict: false-alarm
- reviewed: 2026-07-06
- pair: internal/code/extract.py::_BodyVisitor.visit | internal/code/extract.py::_BodyVisitor.visit_Return
- verdict: false-alarm
- reviewed: 2026-07-06

Same-file script-generator + its own installer (hook.go) — same category as
the already-adjudicated `buildCheckCmd | installPreCommit | ...` cluster
(2026-06-14 run above), just the pre-commit/post-merge sibling pair:

- pair: cmd/calque/hook.go::preCommitScript | cmd/calque/hook.go::installPreCommit
- verdict: false-alarm
- reviewed: 2026-07-06
- pair: cmd/calque/hook.go::postMergeScript | cmd/calque/hook.go::installPostMerge
- verdict: false-alarm
- reviewed: 2026-07-06

Shared generic idiom, unrelated purpose (verified by reading both sides — the
shared signal is a common Go idiom, not a duplicated CONTRACT):

- pair: cmd/calque/labels.go::labelStorePath | internal/llm/judge.go::NewJudge
- verdict: false-alarm
- reviewed: 2026-07-06
  (both resolve a path via env-override → os.UserCacheDir() → os.TempDir()
   fallback chain — labelStorePath for the label store, NewJudge for the judge
   cache — the same "resolve an app data path" idiom, applied to two different
   paths with no shared contract to drift.)
- pair: cmd/calque/vocab_check.go::runSeedCmd | internal/code/extract_rust.go::buildRustExtractor
- verdict: false-alarm
- reviewed: 2026-07-06
  (both set `cmd.Dir` before running an `exec.Command` — the standard
   os/exec idiom for "run a subprocess in a specific directory" — one runs a
   user-configured seed shell command, the other builds the Rust extractor
   via cargo; unrelated purposes.)
- pair: cmd/calque/propose.go::runProposeRoles | internal/code/testfile_test.go::TestAsymmetricTestGateCluster
- verdict: false-alarm
- reviewed: 2026-07-06
  (both merely SET the same `ClusterOptions.IncludeTests` field — production
   code reading a flag, a test fixture hardcoding `true` — coincidental via a
   shared struct field name, not a duplicated behavior.)

Single-shared-token name-stem coincidences (no other channel fired; the
functions are unrelated beyond sharing one word):

- pair: internal/code/extract_ts_test.go::assertHas | internal/registry/registry.go::Registry.Has
- verdict: false-alarm
- reviewed: 2026-07-06
- pair: cmd/calque/scan.go::falseAlarmSuffix | internal/code/falsealarm.go::FalseAlarmHint
- verdict: false-alarm
- reviewed: 2026-07-06
  (falseAlarmSuffix is the thin CLI-rendering wrapper that CALLS
   code.FalseAlarmHint — command-wraps-core, the same layering already
   established for runNearest ≟ Nearest above — not two implementations.)
- pair: cmd/calque/scan.go::falseAlarmSuffix | internal/code/falsealarm_test.go::TestFalseAlarmHint
- verdict: false-alarm
- reviewed: 2026-07-06
  (the shared-calls signal is both sides calling FalseAlarmHint — the wrapper,
   and FalseAlarmHint's own test — not each other.)
- pair: cmd/calque/scan.go::orAll | internal/code/testfile_test.go::TestAllTest
- verdict: false-alarm
- reviewed: 2026-07-06
- pair: cmd/calque/scan.go::orAll | internal/code/testfile.go::allTest
- verdict: false-alarm
- reviewed: 2026-07-06
  (orAll formats a glob-or-"all" display string; allTest checks whether every
   member of a cluster is test code — unrelated domains sharing the token
   "all".)
- pair: cmd/calque/calib.go::resolveLabel | internal/code/optype.go::opLabel
- verdict: false-alarm
- reviewed: 2026-07-06
- pair: cmd/calque/propose_deep.go::runJudge | internal/llm/judge.go::NewJudge
- verdict: false-alarm
- reviewed: 2026-07-06
  (runJudge orchestrates judging candidates and constructs its Judge via
   NewJudge internally — consumer calls constructor, not a twin.)
- pair: cmd/calque/nearest.go::runNearest | internal/code/nearest_test.go::nearestHas
- verdict: false-alarm
- reviewed: 2026-07-06
  (nearestHas is a test-assertion helper for a DIFFERENT test file; unrelated
   to the CLI entry point beyond the shared "nearest" stem.)
- pair: cmd/calque/hook.go::shellQuote | internal/code/touchpoint_test.go::makeShell
- verdict: false-alarm
- reviewed: 2026-07-06
  ("shell" means two different things: a POSIX shell-escaping helper vs a test
   fixture constructor for a fake "Shell" struct.)
- pair: internal/llm/judge.go::Judge.cacheKey | internal/pairkey/pairkey.go::Key
- verdict: false-alarm
- reviewed: 2026-07-06
  (Judge.cacheKey sha256-hashes a verdict cache key; pairkey.Key normalizes an
   unordered string pair — different algorithms, coincidental "key" stem.)

**Conclusion:** `--distance-boost` works exactly as designed (a bounded,
gate-respecting nudge on marginal pairs), but on this repo's current size and
vocabulary the marginal band is dominated by name-stem noise rather than real
drift — 18 of 20 false-alarm, 2 confirming an already-known twin category, 0
new drift found. Recommendation: keep it off by default (already the ship
default) and reach for it selectively on a boundary-scoped run (`--left`/
`--right`) where cross-directory pairs are inherently more meaningful than a
whole-repo self-scan, rather than as a blanket self-scan flag.

## Run — 2026-07-09 (sub-function granularity: branch + value-site axes)

New `propose-branches`/`propose-values` generators (see docs/DESIGN_NOTES.md
§21) added `internal/code/extract_branches_go.go`, `extract_values_go.go`,
`cmd/calque/propose_branches.go`, `propose_values.go` — new functions that
`check --strict`'s own self-scan now sees as ordinary Go functions (they were
never wired into `--strict`; this is `check` reacting to their PRESENCE in the
corpus, not the new axes' own candidates). Isolated the delta the same way the
distance-boost session did: a clean `git worktree` baseline at `411156e`
(v0.13.0) vs the current tree, order-independent pair-key diff. Note: the
SAME baseline binary, re-run 3x with zero code changes, already shows 1-2
pairs flickering in/out (confirmed pre-existing map-iteration-order
non-determinism in `check`'s candidate report, unrelated to this session —
worth its own investigation later, not in scope here) — so this diff is
read as "the new pairs this session's code caused," not a claim of literal
byte-for-byte stability.

Cross-language/cross-file "Visit" method twins (the established visitor-twin
category from the NodeCount/distance-boost rollouts — each language's
extractor needs its own dispatcher over the same node-kind space, so the
shape recurs by necessity, not by copying):

- pair: internal/code/extract_branches_go.go::branchFinder.Visit | internal/code/extract_go.go::goBody.Visit
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract_branches_go.go::branchFinder.Visit | internal/code/extract_ts.mjs::BodyVisitor.visit
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract_branches_go.go::branchFinder.Visit | internal/code/extract_ts.mjs::BodyVisitor.visitBody
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract_branches_go.go::branchFinder.Visit | internal/code/extract.py::_BodyVisitor.visit
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract_branches_go.go::branchFinder.Visit | internal/code/extract.py::_BodyVisitor.visit_Assign
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract_branches_go.go::branchFinder.Visit | internal/code/extract.py::_BodyVisitor.visit_Attribute
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract_branches_go.go::branchFinder.Visit | internal/code/extract.py::_BodyVisitor.visit_Constant
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract_branches_go.go::branchFinder.Visit | internal/code/extract.py::_BodyVisitor.visit_Name
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract_branches_go.go::branchFinder.Visit | internal/code/extract.py::_BodyVisitor.visit_Return
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract_branches_go.go::branchFinder.Visit | internal/code/extract_values_go.go::valueFinder.Visit
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
  (the two NEW finders this session added, mirroring each other by design —
   see docs/DESIGN_NOTES.md §21.2.)
- pair: internal/code/extract_go.go::goBody.Visit | internal/code/extract_values_go.go::valueFinder.Visit
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract_ts.mjs::BodyVisitor.visit | internal/code/extract_values_go.go::valueFinder.Visit
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract_ts.mjs::BodyVisitor.visitBody | internal/code/extract_values_go.go::valueFinder.Visit
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract.py::_BodyVisitor.visit | internal/code/extract_values_go.go::valueFinder.Visit
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract.py::_BodyVisitor.visit_Assign | internal/code/extract_values_go.go::valueFinder.Visit
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract.py::_BodyVisitor.visit_Attribute | internal/code/extract_values_go.go::valueFinder.Visit
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract.py::_BodyVisitor.visit_Constant | internal/code/extract_values_go.go::valueFinder.Visit
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract.py::_BodyVisitor.visit_Name | internal/code/extract_values_go.go::valueFinder.Visit
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract.py::_BodyVisitor.visit_Return | internal/code/extract_values_go.go::valueFinder.Visit
- verdict: contracted-twin-ok
- reviewed: 2026-07-09

Extraction-entry-point twins (a third/fourth `ExtractXxx(repo, exclude) →
([]*FuncSig, ScanStats, error)` shape, mirroring `Extract`/`ExtractSymbols` by
design — see docs/DESIGN_NOTES.md §21):

- pair: internal/code/extract_branches_go.go::ExtractBranches | internal/code/extract_values_go.go::ExtractValueSites
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract_branches_go.go::ExtractBranches | internal/code/scan.go::Extract
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract_branches_go.go::ExtractBranches | internal/code/symbols.go::ExtractSymbols
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract_values_go.go::ExtractValueSites | internal/code/scan.go::Extract
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract_values_go.go::ExtractValueSites | internal/code/symbols.go::ExtractSymbols
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/cache.go::ExtractCached | internal/code/extract_branches_go.go::ExtractBranches
- verdict: false-alarm
- reviewed: 2026-07-09
  (checked ExtractCached directly: it's a caching layer with stat-based
   staleness detection, meaningfully more than the walk+extract shape it
   superficially shares with ExtractBranches.)
- pair: internal/code/cache.go::ExtractCached | internal/code/extract_values_go.go::ExtractValueSites
- verdict: false-alarm
- reviewed: 2026-07-09
  (same reasoning as the ExtractCached/ExtractBranches pair above.)
- pair: internal/code/extract_branches_go.go::extractGoBranchesFile | internal/code/extract_go.go::ExtractGoFile
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract_branches_go.go::extractGoBranchesFile | internal/code/extract_go.go::extractGoSymbolsFile
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract_branches_go.go::extractGoBranchesFile | internal/code/extract_values_go.go::extractGoValuesFile
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract_go.go::ExtractGoFile | internal/code/extract_values_go.go::extractGoValuesFile
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract_branches_go.go::goFuncSigFromStmts | internal/code/extract_go.go::goFuncSigFromBody
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
  (deliberate siblings by design — goFuncSigFromStmts is goFuncSigFromBody's
   sibling for a bare statement list; see extract_branches_go.go's own doc
   comment.)

`propose-*` CLI command twins (every generator shares the same flag-parse →
extract → registry-load → dedupe → print/--judge shape by necessity — the
established "generic CLI plumbing, same shape by necessity" category):

- pair: cmd/calque/confess.go::runConfess | cmd/calque/propose_branches.go::runProposeBranches
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: cmd/calque/confess.go::runConfess | cmd/calque/propose_values.go::runProposeValues
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: cmd/calque/propose_branches.go::runProposeBranches | cmd/calque/propose.go::runProposeRoles
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: cmd/calque/propose_branches.go::runProposeBranches | cmd/calque/propose_cross.go::runProposeCross
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: cmd/calque/propose_branches.go::runProposeBranches | cmd/calque/propose_deep.go::runProposeDeep
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: cmd/calque/propose_branches.go::runProposeBranches | cmd/calque/propose_deriv.go::runProposeDeriv
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: cmd/calque/propose_branches.go::runProposeBranches | cmd/calque/propose_values.go::runProposeValues
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
  (the two NEW commands this session added — same CLI shape by design.)
- pair: cmd/calque/propose_cross.go::runProposeCross | cmd/calque/propose_values.go::runProposeValues
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: cmd/calque/propose_deep.go::runProposeDeep | cmd/calque/propose_values.go::runProposeValues
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: cmd/calque/propose_deriv.go::runProposeDeriv | cmd/calque/propose_values.go::runProposeValues
- verdict: contracted-twin-ok
- reviewed: 2026-07-09

Small helper-method twins (deliberate structural mirroring within/across the
two new finders, same role in each — not accidental duplication):

- pair: internal/code/extract_branches_go.go::branchFinder.addArm | internal/code/extract_branches_go.go::branchFinder.addStmts
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract_branches_go.go::branchFinder.addArm | internal/code/extract_values_go.go::valueFinder.add
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract_branches_go.go::branchFinder.addStmts | internal/code/extract_values_go.go::valueFinder.add
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: internal/code/extract_values_go.go::valueFinder.fromKeyValue | internal/code/extract_values_go.go::valueFinder.fromValueSpec
- verdict: contracted-twin-ok
- reviewed: 2026-07-09

Coincidental short-name collision (checked the actual source — genuinely
unrelated):

- pair: cmd/calque/ablate.go::ablateCell.add | internal/code/extract_values_go.go::valueFinder.add
- verdict: false-alarm
- reviewed: 2026-07-09
  (ablateCell.add tallies a verdict class into a counter; valueFinder.add
   builds and appends a FuncSig — unrelated beyond the generic verb "add".)

**Conclusion:** all 46 pairs this session's own new code caused in `check
--strict`'s self-scan are exactly the expected shape (visitor/extraction-
entry-point/CLI-command twins already-established categories) — 44
contracted-twin-ok, 2 false-alarm (both individually verified against actual
source, not assumed). Zero represent lost detection or real drift. This
confirms the plan's "neither axis is wired into --strict" claim held in
practice, not just by construction: `check`'s own candidate SET is unchanged
except for this session's own new code entering the corpus as ordinary
functions, which is unavoidable and expected of ANY new .go file, new axis
or not.

### `propose-branches` dogfood (the axis's OWN candidates, not the check-axis
self-detection above)

`calque propose-branches --repo .` at defaults (`--min-lines 4`) surfaces 73
candidates (1462 branch fragments scanned). Read real source for the top 20
(by score) rather than rubber-stamping the summary line:

- **`rustExtractorBin`'s 4 arms** (extract_rust.go:80/85/95/99 — env-override
  check, cache-hit check, build-and-cache) — all 6 pairwise combinations
  scored jac=1.00 driven ENTIRELY by shared name-stem (no shared-calls/
  strings/writes fired at all). Read the source: these are sequential
  GUARD-CLAUSE arms (`if <condition> { set path; return }`) doing three
  DIFFERENT things — same shape by necessity (the guard-clause idiom), not
  duplicated logic.
- pair: internal/code/extract_rust.go::rustExtractorBin#branch@80.1 | internal/code/extract_rust.go::rustExtractorBin#branch@85.2
- verdict: false-alarm
- reviewed: 2026-07-09
- pair: internal/code/extract_rust.go::rustExtractorBin#branch@80.1 | internal/code/extract_rust.go::rustExtractorBin#branch@95.4
- verdict: false-alarm
- reviewed: 2026-07-09
- pair: internal/code/extract_rust.go::rustExtractorBin#branch@80.1 | internal/code/extract_rust.go::rustExtractorBin#branch@99.5
- verdict: false-alarm
- reviewed: 2026-07-09
- pair: internal/code/extract_rust.go::rustExtractorBin#branch@85.2 | internal/code/extract_rust.go::rustExtractorBin#branch@95.4
- verdict: false-alarm
- reviewed: 2026-07-09
- pair: internal/code/extract_rust.go::rustExtractorBin#branch@85.2 | internal/code/extract_rust.go::rustExtractorBin#branch@99.5
- verdict: false-alarm
- reviewed: 2026-07-09
- pair: internal/code/extract_rust.go::rustExtractorBin#branch@95.4 | internal/code/extract_rust.go::rustExtractorBin#branch@99.5
- verdict: false-alarm
- reviewed: 2026-07-09

- **`CalibrateWeights`'s 4 arms** (calibrate.go:59/64/90/101 — single-class
  early return, per-channel discrimination loop, normalize block, …) — same
  pattern: all 6 pairwise combos score jac=1.00 on shared name-stem alone.
  Read the source: four DIFFERENT statistics steps within one function, not
  duplicated logic.
- pair: internal/code/calibrate.go::CalibrateWeights#branch@59.2 | internal/code/calibrate.go::CalibrateWeights#branch@64.3
- verdict: false-alarm
- reviewed: 2026-07-09
- pair: internal/code/calibrate.go::CalibrateWeights#branch@59.2 | internal/code/calibrate.go::CalibrateWeights#branch@90.5
- verdict: false-alarm
- reviewed: 2026-07-09
- pair: internal/code/calibrate.go::CalibrateWeights#branch@59.2 | internal/code/calibrate.go::CalibrateWeights#branch@101.7
- verdict: false-alarm
- reviewed: 2026-07-09
- pair: internal/code/calibrate.go::CalibrateWeights#branch@64.3 | internal/code/calibrate.go::CalibrateWeights#branch@90.5
- verdict: false-alarm
- reviewed: 2026-07-09
- pair: internal/code/calibrate.go::CalibrateWeights#branch@64.3 | internal/code/calibrate.go::CalibrateWeights#branch@101.7
- verdict: false-alarm
- reviewed: 2026-07-09
- pair: internal/code/calibrate.go::CalibrateWeights#branch@90.5 | internal/code/calibrate.go::CalibrateWeights#branch@101.7
- verdict: false-alarm
- reviewed: 2026-07-09

- **Same-shape-by-necessity switch/case arms** (checked each directly): a
  parser handling structurally-different cases with the same accumulation
  idiom.
- pair: cmd/calque/migrate.go::migrateRegistry#branch@98.2 | cmd/calque/migrate.go::migrateRegistry#branch@103.3
- verdict: false-alarm
- reviewed: 2026-07-09
  (two `- left:`/`- right:` prefix-parsing cases in a markdown-line scanner —
   same idiom, different fields.)
- pair: internal/code/extract_go.go::goBody.recordTarget#branch@330.1 | internal/code/extract_go.go::goBody.recordTarget#branch@334.2
- verdict: false-alarm
- reviewed: 2026-07-09
  (SelectorExpr vs IndexExpr cases both record a write target — same
   operation, different AST node shape.)
- pair: internal/code/extract_go.go::goLiteralKeys#branch@177.1 | internal/code/extract_go.go::goLiteralKeys#branch@190.2
- verdict: false-alarm
- reviewed: 2026-07-09
  (MapType vs ArrayType cases share an append-to-keys idiom but the map case
   does meaningfully more — same shape, not duplicated logic.)
- pair: internal/code/extract_json.go::jsonCollector.walk#branch@75.2 | internal/code/extract_json.go::jsonCollector.walk#branch@80.3
- verdict: false-alarm
- reviewed: 2026-07-09
  (map[string]any vs []any recursion cases in a JSON tree walker.)

- **The `extractGoBranchesFile ≟ ExtractGoFile` candidate this session's OWN
  code produced (jac 0.83) was a REAL finding, not noise**: the receiver-
  qualname-construction snippet (`if fd.Recv != nil && len(fd.Recv.List) > 0
  { if rt := recvTypeName(...); rt != "" { qual = rt + "." + name } }`) was
  copy-pasted verbatim into `ExtractGoFile`, `extractGoBranchesFile`, AND
  `extractGoValuesFile` while writing this session's own code — genuine
  intra-function-shape duplication, exactly the class of bug this axis
  targets. Fixed (not adjudicated as a pair — it no longer exists): extracted
  `qualNameFor(name string, recv *ast.FieldList) string` into extract_go.go,
  all three call sites now call it. Re-ran `propose-branches` after the fix —
  the pair is gone.

- **Left un-adjudicated, flagged for the maintainer** (not confidently a
  false-alarm, not confidently drift — genuinely needs a human call, not a
  forced verdict):
  `cmd/calque/vocab_check.go::runVocabCheck#branch@77.6` ≟
  `cmd/calque/vocab_check.go::renderVocabCheck#branch@159.2` (jac 0.77) — both
  arms print/build the IDENTICAL "clean across %d file(s) — %d allow-listed
  compound(s); threshold freq >= %d" message via two SEPARATE format-string
  literals rather than one delegating to the other. Could drift (edit one
  message, forget the other) or could be a deliberate CLI-vs-render split
  (unclear without more context on renderVocabCheck's caller). Not fixed this
  pass (pre-existing code, unrelated to this session's own changes — out of
  scope to touch here) — worth a look, not force-adjudicated.

- **The `propose-*` CLI family's shared tail idiom** (`if *judge {
  runJudge(...); return }; for i, c := range fresh { printCandidate(...) }`) —
  same established convention already adjudicated above for the check-axis
  pairs, confirmed again here for two more instances:
- pair: cmd/calque/propose_deep.go::runProposeDeep#branch@122.9 | cmd/calque/propose_values.go::runProposeValues#branch@81.6
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: cmd/calque/propose_branches.go::runProposeBranches#branch@92.7 | cmd/calque/propose_deriv.go::runProposeDeriv#branch@100.9
- verdict: contracted-twin-ok
- reviewed: 2026-07-09

**`propose-branches` conclusion:** the mechanism works — it correctly
extracts and scores sub-function fragments, and even caught a real, if minor,
self-introduced duplication this session's own code created (fixed above).
But the dominant noise pattern at defaults is exactly what the design section
anticipated: sibling arms of ONE function trivially clear the anchor gate via
shared name-stem (same enclosing function name), and for functions with
several structurally-parallel-but-semantically-distinct branches (guard
clauses, switch/case dispatch), this produces a HIGH nominal score (often
1.00) driven by name alone, with zero real content overlap — 18 of the 20
reviewed were exactly this shape. Recommendation: before considering
`--strict` graduation, add a same-function discount (or a `Reason()` hint
analogous to `FalseAlarmHint`, "same-function siblings scored on name alone")
so a human scanning the report can immediately deprioritize the dominant
noise class, the same way `falseAlarmSuffix` already helps for the
whole-function axis. Real drift signal exists (the qualNameFor case proves
it) but is currently buried under name-tautology noise at these defaults.

### `propose-values` dogfood

`calque propose-values --repo .` at defaults (`--name-jaccard 0.01
--max-fanout 8`) surfaces 105 candidates (386 value-sites scanned). Read real
source for the top 20:

- **Verdict-vocabulary strings** ("drift"/"false-alarm" assigned to fields
  named `Verdict` in two different test files) — calque's own fixed verdict
  vocabulary (`llm.ClassDrift`/`llm.ClassFalseAlarm` etc.) reused correctly in
  independent test fixtures, not an accidental shared magic value at risk of
  drifting — there's no "one true value" to keep in sync, both sides
  correctly reference the SAME established vocabulary term.
- pair: cmd/calque/ablate_test.go::TestLoadLabelsDedupLatestWins.Verdict#value@63.19 | cmd/calque/check_test.go::TestUnresolvedDrift.Verdict#value@55.9
- verdict: false-alarm
- reviewed: 2026-07-09
- pair: cmd/calque/ablate_test.go::TestLoadLabelsDedupLatestWins.Verdict#value@64.23 | cmd/calque/check_test.go::TestUnresolvedDrift.Verdict#value@53.3
- verdict: false-alarm
- reviewed: 2026-07-09
- pair: cmd/calque/ablate_test.go::TestLoadLabelsDedupLatestWins.Verdict#value@64.23 | cmd/calque/check_test.go::TestUnresolvedDrift.Verdict#value@54.6
- verdict: false-alarm
- reviewed: 2026-07-09
- pair: cmd/calque/ablate_test.go::TestLoadLabelsDedupLatestWins.Verdict#value@65.27 | cmd/calque/check_test.go::TestUnresolvedDrift.Verdict#value@53.3
- verdict: false-alarm
- reviewed: 2026-07-09
- pair: cmd/calque/ablate_test.go::TestLoadLabelsDedupLatestWins.Verdict#value@65.27 | cmd/calque/check_test.go::TestUnresolvedDrift.Verdict#value@54.6
- verdict: false-alarm
- reviewed: 2026-07-09

- **`JSONRPC = "2.0"` — a REAL instance of the target pattern** (production
  `cmd/calque/mcp.go` and its own test file `mcp_test.go`, 4 distinct
  construction sites, no shared const backing the literal). Verified: this is
  genuinely the maxRetries-style shape (a protocol-version literal repeated
  across independent sites). Classified contracted-twin-ok rather than drift
  because JSON-RPC's wire version is an external, effectively-immutable
  protocol constant — not presently at risk of silent divergence — but it IS
  a legitimate hygiene opportunity (extract `const jsonRPCVersion = "2.0"`)
  even though nothing is currently broken.
- pair: cmd/calque/mcp.go::errResp.JSONRPC#value@195.7 | cmd/calque/mcp_test.go::TestMCPNotificationSilent.JSONRPC#value@81.3
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: cmd/calque/mcp.go::errResp.JSONRPC#value@195.7 | cmd/calque/mcp_test.go::drive.JSONRPC#value@23.1
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: cmd/calque/mcp.go::handleMCP.JSONRPC#value@68.1 | cmd/calque/mcp_test.go::TestMCPNotificationSilent.JSONRPC#value@81.3
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: cmd/calque/mcp.go::handleMCP.JSONRPC#value@68.1 | cmd/calque/mcp_test.go::drive.JSONRPC#value@23.1
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: cmd/calque/mcp.go::handleMCP.JSONRPC#value@86.4 | cmd/calque/mcp_test.go::TestMCPNotificationSilent.JSONRPC#value@81.3
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: cmd/calque/mcp.go::handleMCP.JSONRPC#value@86.4 | cmd/calque/mcp_test.go::drive.JSONRPC#value@23.1
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: cmd/calque/mcp.go::toolText.JSONRPC#value@187.5 | cmd/calque/mcp_test.go::TestMCPNotificationSilent.JSONRPC#value@81.3
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: cmd/calque/mcp.go::toolText.JSONRPC#value@187.5 | cmd/calque/mcp_test.go::drive.JSONRPC#value@23.1
- verdict: contracted-twin-ok
- reviewed: 2026-07-09

- **`type"/"Type" = "text"`** — checked both sites directly: `mcp.go`'s
  `toolText` builds an MCP `{"type":"text","text":...}` content block;
  `judge_test.go`'s `apiContentText{Type:"text",...}` fixtures model Claude
  API response content blocks. Both independently model a similar
  external-protocol content-block shape for UNRELATED integration points
  (outbound MCP response vs. inbound Claude API parsing) — coincidental
  shared-idiom collision, not drift (they're not meant to stay in sync).
- pair: cmd/calque/mcp.go::toolText.type#value@189.6 | internal/llm/judge_test.go::TestParseVerdictNoJSON.Type#value@105.24
- verdict: false-alarm
- reviewed: 2026-07-09
- pair: cmd/calque/mcp.go::toolText.type#value@189.6 | internal/llm/judge_test.go::TestParseVerdictToleratesProse.Type#value@82.20
- verdict: false-alarm
- reviewed: 2026-07-09
- pair: cmd/calque/mcp.go::toolText.type#value@189.6 | internal/llm/judge_test.go::TestParseVerdictUnknownClass.Type#value@95.22
- verdict: false-alarm
- reviewed: 2026-07-09

- **The `propose-*` CLI family's shared `testNote` message convention** — the
  SAME `--include-tests` explanatory string, deliberately mirrored across
  every propose-* command (confirmed: this session's own propose_branches.go
  was written by deliberately copying propose_deriv.go's exact convention).
- pair: cmd/calque/propose_branches.go::runProposeBranches.testNote#value@72.2 | cmd/calque/propose_deriv.go::runProposeDeriv.testNote#value@74.1
- verdict: contracted-twin-ok
- reviewed: 2026-07-09
- pair: cmd/calque/propose_branches.go::runProposeBranches.testNote#value@74.3 | cmd/calque/propose_deriv.go::runProposeDeriv.testNote#value@76.2
- verdict: contracted-twin-ok
- reviewed: 2026-07-09

- **Coincidental test-fixture placeholder values** (two unrelated test
  functions independently choosing the same generic filler value):
- pair: internal/code/block_test.go::TestCandidatePairsEmptyChannels.File#value@148.4 | internal/code/touchpoint_test.go::TestClusterKeyOrderIndependent.File#value@272.11
- verdict: false-alarm
- reviewed: 2026-07-09
  ("b.go" as a generic placeholder filename in two unrelated fixture tables.)
- pair: internal/code/block_test.go::TestCandidatePairsEmptyChannels.NLines#value@148.5 | internal/code/touchpoint_test.go::TestExternalCallNotSeam.NLines#value@120.5
- verdict: false-alarm
- reviewed: 2026-07-09
  ("10" as a generic placeholder line count in two unrelated fixture tables.)

**`propose-values` conclusion:** one genuine instance of the target pattern
found in the top 20 (JSONRPC="2.0" — real, if low-risk since it's an external
protocol constant), zero drift, the rest split between calque's own fixed
verdict-vocabulary reuse, a coincidental cross-protocol idiom collision, an
established CLI-convention string, and plain test-fixture-placeholder
coincidence. The `--name-jaccard`/`--max-fanout` anchoring works as intended
(no bare-value-only noise made the top 20), but a FIXED-VOCABULARY exclude
list (calque's own verdict classes, common test-fixture placeholders like
short filenames/line counts) would meaningfully raise precision — a natural
follow-up once more corpora are calibrated, not implemented this pass.

**Overall recommendation for both axes:** ship as `propose-*` generators
(already the design — not wired into `--strict`), keep dogfooding on
external repos before considering `--strict` graduation for either, and
treat the same-function-name-tautology (branches) and fixed-vocabulary
(values) noise patterns identified above as the concrete, evidence-based
next calibration targets — not guessed ones.

**External validation (read-only — this is calque's OWN registry; nothing
was written to the external repo):** ran both generators against `~/Documents/
stope` (a large, unrelated Go repo, 1176 files) per the plan's dogfood step.
`propose-values` surfaced, at rank #1, `MaxTokens = 128` (`cmd/spike-vcr/
main.go`) ≟ `maxTokens = 128` (`internal/inputllm/inputllm.go`) — an
unprompted, real-world instance of EXACTLY the `maxRetries`-shaped bug this
axis was built to catch (an LLM-call config constant duplicated across two
files, different capitalization, no shared symbol). Also surfaced `asciiEscape`'s
`"0123456789abcdef"` hex-digit-alphabet string copy-pasted verbatim across 4
different files' helper functions — a real, if minor, "same utility
reimplemented instead of shared" pattern. `propose-branches` reproduced the
same same-function-name-tautology noise pattern at scale (1938 candidates on
1176 files) but also surfaced a genuinely interesting cross-struct candidate
(`GameEngine.findPreauthored` ≟ `Game.findPreauthored`, cross-file) worth a
maintainer's look. Both axes generalize to unfamiliar code, not just this
repo's own idioms.
