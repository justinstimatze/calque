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
