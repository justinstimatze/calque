# Spec — the call-site context axis (`propose-context`)

Addresses the class named at the close of `DESIGN_NOTES.md` §22 and revisited in
`SPEC-reads-axis.md` §7's deferred "Phase 2": two independently-written functions that
compute the same real-world thing, share **zero lexical tokens** in their own bodies (no
common name, string, write, or callee), and have **no distinctive type signature** (either
untyped, or generically typed like `string→bool`). Every existing mechanism — the pairwise
scorer, `SharedDerivationCandidates`, `SignatureCandidates` — requires some signal *inside*
the candidate's own body or type to anchor on. This class is not low-recall for calque:
it is **zero-recall**, by construction.

## Why not the two mechanisms already considered

Both were designed, then rejected on evidence rather than a guess:

- **NL-summary embedding** (`SPEC-reads-axis.md` §7's deferred Phase 2): embed an
  LLM-generated behavioral summary instead of the raw source, so similarity tracks intent
  not syntax. *Functional Consistency of LLM Code Embeddings* (arXiv 2508.19558) found
  off-the-shelf code embeddings predominantly capture syntax, not function — closing this
  gap took a purpose-built contrastive fine-tuning pipeline, not a summarize-then-embed
  pass. *Semantic Code Clone Detection: Are We There Yet?* (2606.25272, Jun 2026) is the
  freshest and hardest evidence: 11 SOTA detectors — token-, tree-, and graph-based, the
  last being the closest category to embedding approaches — all degrade substantially
  under distribution-shifted Type-4 clones, relying on "shortcut learning based on lexical
  and structural cues rather than robust semantic understanding." Not disqualifying, but a
  real caution against expecting a cheap embedding pass to just work.
- **Execution-based confirmation** (HyClone, 2508.01357): run both candidates on
  LLM-synthesized inputs, compare outputs. Structurally biased *against* the exact bug this
  axis exists to catch: a drifted pair usually still agrees on most inputs — that is what
  drift looks like, mostly the same, diverged on one path or edge case — so a handful of
  test inputs will likely show agreement and falsely confirm equivalence on the pair that
  is actually broken. Testing can prove disagreement; it can never prove agreement. Every
  execution/differential-testing tool in calque's own prior-art sweep (`divergent-
  implementation-detection.md` lines 137-139) is already correctly scoped as a
  *confirmation* step for an already-surfaced candidate, never a recall mechanism — this
  spec does not change that division of labor.

## The actual gap: everything so far instruments the candidate itself

Source embedding, summary embedding, and execution testing all look *at* the candidate
function — its text, its description, or its output. None looks at where it's **called
from**. Two functions with zero shared tokens in their own bodies can still be invoked in
near-identical *shapes* of surrounding code: both called right before `db.Save()`, both
feeding an `if err != nil` gate, both taking a request object and producing something
compared against a boolean. That convergence is real signal — the code-graph analog of how
biology recognizes convergent evolution without shared ancestry (bird wings and bat wings
share no sequence homology, but occupy the same functional position in their respective
systems). This axis instruments the *caller*, not the candidate.

## 1. Data model — two new derived signals, one of them nearly free

`FuncSig` (`internal/code/funcsig.go`) gains one new interchange field:

```go
// CallResultShapes maps a callee's leaf name to the abstracted shape tags observed
// at each call site THIS function makes into it (ret-checked-nil, ret-passed-to-call,
// etc. — see §3). Populated during the normal per-function body walk, no new pass.
CallResultShapes map[string][]string `json:"call_result_shapes,omitempty"`
```

`CallerStems` is **not** a stored field — it is a derived, corpus-wide fold computed once
per scan from data every extractor already produces, in a new small helper
(`internal/code/callcontext.go`, `CallerStemIndex(sigs []*FuncSig) map[string][]string`):

```go
// CallerStemIndex inverts the corpus's existing Calls edges into a callee-leaf-name ->
// union-of-caller-name-stems map, entirely from data already extracted (FuncSig.Calls,
// FuncSig.stem via Prepare). No new AST walk, no new field, no new extraction code per
// language — this is the caller-role signal, and it's a post-processing fold.
func CallerStemIndex(sigs []*FuncSig) map[string]set {
	idx := map[string]set{}
	for _, f := range sigs {
		for _, callee := range f.Calls {
			if idx[callee] == nil {
				idx[callee] = set{}
			}
			for tok := range f.stem {
				idx[callee][tok] = struct{}{}
			}
		}
	}
	return idx
}
```

Backward compatible: an extractor that never populates `CallResultShapes` yields `nil`,
`CallerStemIndex` degrades to an empty set for that entry, no existing pass is affected.

## 2. Extraction rule — `CallResultShapes` (new, but bounded and single-function-local)

**Definition.** When a function's body-walk hits a call `g(...)`, classify what happens to
the call's result in the immediately enclosing statement/expression, and append the tag to
`CallResultShapes[g_leaf_name]`:

- `ret-nil-checked` — result compared with `nil` (`if x == nil` / `!= nil`)
- `ret-err-checked` — the `if err != nil` shape specifically (kept separate from
  `ret-nil-checked` because it is near-universal in idiomatic Go and needs its own
  down-weight, not lumped with a genuinely rarer nil-check)
- `ret-passed-to-call` — result passed directly as an argument to another call
- `ret-returned` — result returned as-is from the enclosing function (pass-through/
  delegation shape)
- `ret-assigned-field` — result assigned directly to a struct field or map key
- `ret-compared` — result compared with `==`/`<`/`>` against something other than `nil`
- `arg-count:N` — number of arguments passed at this call site (a weak but free signal —
  functions serving the same contract tend to take the same number of inputs even across
  independent implementations)

This is **not** a new corpus-wide pass. It is one more branch in the same per-function
visitor that already populates `Calls`/`Writes`/`Reads` (`goBody.Visit`,
`_BodyVisitor.visit_*`, `BodyVisitor.visit`, Rust's `Visit<'ast> for Body`) — the call
expression node is already being visited; this just also inspects its immediate parent
context. Per-language, this means: **Go** — in `goBody.Visit`'s existing `*ast.CallExpr`
handling, walk one level up via the parent statement (the visitor already tracks the
current statement context for write detection; reuse it) and classify against the tag
list above. **Python** — `_BodyVisitor.visit_Call`, inspect `ast.walk`'s parent via the
existing statement-context tracking. **TypeScript** — `CallExpression`'s `.parent` node,
switched on its `SyntaxKind`. **Rust** — `Expr::Call`/`Expr::MethodCall` in `visit_expr`,
classify against the enclosing `Stmt`/`Expr` already available in scope.

## 3. The recall pass — `CallContextCandidates` (boundary-free, no token or type required)

A new generator, mirroring `SignatureCandidates`'s shape (group-then-pair, no `scorePair`
gate to be invisible to) but keyed on the two new signals instead of a type signature:

```go
// CallContextCandidates pairs functions with no shared body token and no distinctive
// type signature, purely on how and where they're invoked: overlapping caller-role
// stems (near-synonym callers, e.g. canBypassRateLimit / skipThrottleCheck) AND
// overlapping call-result shapes (both get nil-checked, both feed a downstream call).
// Both conditions required — either alone is the generic-shape/valence-guard trap
// (see §5): shared caller vocabulary alone catches unrelated functions serving one
// popular caller; shared result-shape alone catches "gets its error checked," which
// is nearly every function in idiomatic Go.
func CallContextCandidates(sigs []*FuncSig, minCallerJaccard, minShapeJaccard float64, maxFanout int) []SigCandidate
```

Requiring **both** signals to clear their own threshold is the anchor this axis needs in
place of a shared token — a pair that only shares one dimension doesn't enter the pool.
`Rank`'s pairwise scorer is untouched; this stays a standalone generator like
`propose-deep`/`propose-cross`, not a `score.go` channel, because `score.go`'s channels
only ever get evaluated for a pair that already passed the existing `hasAnchor` gate — a
genuinely zero-token pair never reaches `scorePair` at all, so the anchor has to live in a
dedicated recall pass, not a corroborating weight.

## 4. CLI wiring

**New subcommand `propose-context`** (working name, trivially renameable — matches
`propose-deep`/`propose-cross`'s brevity), modeled on `propose_deep.go`: extract `[]*FuncSig`
across the repo, build the `CallerStemIndex`, run `CallContextCandidates`, dedup vs the
registry, hand survivors to `printCandidate`/`runJudge` (reused verbatim — its candidates
are `[]SigCandidate`, same shape every other generator emits). Flags: `--min-caller-jaccard`,
`--min-shape-jaccard`, `--max-fanout`, `--repo`, `--exclude`, `--judge`, `--twins-only`
(matching the existing flag vocabulary across every `propose-*` command).

## 5. The valence-guard problem — this needs its own calibrated stoplist

`ret-err-checked` and `arg-count:1` will fire on a huge fraction of any Go corpus — a
classifier calibrated on a surface cluster this generic won't generalize (Cronbach-Meehl's
Pd-scale problem: the same signal elevation shows up across unrelated groups that share a
surface pattern but not the actual construct). This axis needs its own `builtinGeneric`-
style exclude list for shape tags too common to mean anything, built from real adjudication
data exactly like the signature axis's stoplist was — not guessed up front. Expect the
requiring-both-signals design in §3 to carry most of the precision burden until that
calibration exists.

**Cross-repo data confirms the AND-gate, not a single-signal cut (stope, 2026-09-01,
30 pairs adjudicated by an agent reading source directly, not via `--judge`,
from a 1785-file/9727-function run).** calque's own 25-pair
self-scan had both real hits at shape≈1.00 (caller≈ 1.00 and 0.14), suggesting shape
might be the load-bearing signal and caller-stem the noisy one. stope's sample inverted
that: 4 of 6 real hits had caller≈1.00 exactly, two of those on shape≈ only 0.50 — the
*caller* signal carried them, not shape. Only one hit had caller≈ below 1.00 (0.89) while
shape≈ stayed at 1.00. Across the two corpora neither signal is consistently the stronger
one — sometimes caller-stem rescues a weak shape score, sometimes shape rescues a weak
caller score. Read this as validating the §3 design (both signals required) rather than
as license to drop or down-weight either one; a stoplist that discounts "the weak signal"
would have suppressed different real hits in each repo.

**A new mechanical false-alarm pattern, not seen in the calque self-scan: direct
caller→callee pairs.** `marshalReport`/`SaveReport` (the former was "pulled out of" the
latter) and `PanelReport.AvgScores`/`.ScoreKeys` (the former calls the latter directly)
are two functions in a genuine call relationship — trivially sharing caller vocabulary
and often result shape, but a pipeline, not a twin. **Shipped:** `callsEachOther`
(`internal/code/callcontext.go`) skips a pair when either's `FuncSig.Calls` names the
other. Unlike the tag stoplist this needed no adjudication data — it's a direct read of
the call graph already captured in extraction. Verified against real data, not just the
unit fixture: rebuilding and re-running against stope removed exactly the pairs this
check should remove (`marshalReport`/`SaveReport`, `PanelReport.AvgScores`/`.ScoreKeys`,
plus two more of the same shape found outside the original 30-pair sample —
`HottestMoment` calling `ScoreAll`, `CapturePromptGold` calling `SaveGold`) and nothing
else — 171 → 163 candidates, all four removals confirmed by reading the actual call site.

**A closely related but distinct pattern this filter does NOT catch, verified by reading
source, not assumed:** `shaftWrong`/`shaftAdvance`, `Tokenizer.wordPiece`/`.preTokenize`,
and `BuildJobs`/`ParseArgs` were mischaracterized in this doc's first pass as more
caller→callee examples — reading the actual bodies shows neither calls the other in any
of the three; a third function calls both in sequence and (in the shaft/tokenizer cases)
threads one's return into the other's argument. That's structurally the same
same-driver-function-siblings shape §5 already names above, just with a data dependency
between the siblings — `callsEachOther` correctly leaves these alone, and they remain
open for the frequency-based stoplist rather than this mechanical check. Don't conflate
"one function's result feeds another, both called by a third" with "one function calls
the other" — they look similar in a doc-comment summary and are different in the call
graph.

## 6. Precision budget & calibration

Expect the same recall-first profile every boundary-free generator in this codebase
ships with: high recall, low precision by nature, a `propose-*` command that never gates,
calibrated from real registry adjudications before any `--strict` graduation is even
considered (not in scope for this pass). Keep the judge prompt terse, same overcorrection
finding `SPEC-reads-axis.md` §6 already names. This axis's dominant real-world use case is
a **single-codebase self-scan** (the `sessionId→WorktreeInfo` motivating case: two
divergent paths in the *same* project) rather than cross-project comparison — caller-stem
overlap leans on shared naming/logging conventions across one wider codebase, which two
genuinely unrelated projects would not share. Expect materially weaker recall if ever
pointed at a `--left`/`--right` cross-project boundary instead of a whole-repo self-scan.

## 7. Relationship to the deferred NL-summary embedding (Phase 2)

This is not a replacement for `SPEC-reads-axis.md` §7's deferred embedding idea — it is a
cheaper, complementary axis that ships without an embedding model, without an LLM call, and
without the syntax-vs-function risk the 2508.19558/2606.25272 findings raise, because it
never looks at the candidate's own text at all. If `internal/embed`'s existing ollama client
is ever reused here, the natural extension is embedding *caller name-stems* (a much smaller,
more tractable target than a full behavioral summary) for near-synonym caller matching
instead of exact stem-token overlap — a later enhancement, not required for a first pass.

## 8. Self-gate

Adding this axis makes calque self-scan its own call-context clusters. Build, run
`propose-context --repo .`, and adjudicate the results in `.calque/registry.md` the same way
every other boundary-free generator's first self-scan was adjudicated — expect heavy
`false-alarm` volume until the §5 stoplist is calibrated from that data. `gofmt -l .` clean;
`go test ./internal/code/ ./cmd/calque/` green.

## Build order

1. `FuncSig.CallResultShapes` + per-language extraction (Go first, matching house
   convention of shipping one language before generalizing — see `SPEC-reads-axis.md`'s
   own Go-first precedent for `propose-branches`/`propose-values`).
2. `CallerStemIndex` (pure post-processing, no extraction dependency — can ship
   independently of #1 and be tested in isolation).
3. `CallContextCandidates` requiring both signals + `sigcluster_test.go`-style cases
   (the recall pass).
4. `propose-context` CLI wiring (the surface).
5. Self-gate adjudication; §5's stoplist calibration is a follow-up once real
   adjudication data exists, not part of this build.

## Sources

Design rationale cross-checked against lexicon atoms this session: `lex-s22hf` (same
effects across disparate domains warrant a shared cause — the call-site-shape argument),
`lex-uq4xw` (an object cannot change itself; add the instrument and the coupling field —
naming why every prior idea instrumented the candidate instead of its context), `lex-ah2es`
(structural match needs a valence guard — the §5 stoplist requirement), `lex-zfbef`
(additional detail on existing variables raises confidence without raising accuracy — the
case against "improve the summarization prompt" instead of adding a genuinely orthogonal
signal), `lex-pp9c8` (Feynman on extending concepts past where they've been checked — why
this signal doesn't need to be individually strong to be worth adding to `Suspicion`'s
existing weighted-combination architecture). Literature: see this spec's own citations
above, and `divergent-implementation-detection.md`'s Sources section for the fuller sweep.
