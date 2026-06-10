// Package llm is calque's behavioral-equivalence oracle — the precision half of
// the Type-4 loop. The signature-recall pass (internal/code) is high-recall but
// low-precision: it proposes function pairs that SHARE a type signature, many of
// which do different jobs. This package adjudicates each candidate with an LLM:
// "are these two functions the same contract?" — the judgment a human equivalence
// oracle would make, automated.
//
// Deliberately stdlib-only (calque is a zero-dependency single binary; it shells
// out rather than embed). One POST to /v1/messages, parsed defensively. Results
// are content-hash cached to disk so re-runs over unchanged code are free (the
// API-cost discipline: never pay twice for the same pair).
package llm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Verdict is the oracle's judgment on one candidate pair, classified into calque's
// registry taxonomy so the output is directly actionable.
type Verdict struct {
	Class      string `json:"class"`      // drift | contracted-twin-ok | false-alarm
	Confidence string `json:"confidence"` // low | medium | high
	Reason     string `json:"reason"`
	Cached     bool   `json:"-"` // true if served from disk, not the API
}

// IsTwin reports whether the pair shares a contract at all (drift OR an intentional
// twin) — i.e. anything but a false-alarm.
func (v Verdict) IsTwin() bool { return v.Class == "drift" || v.Class == "contracted-twin-ok" }

// IsDrift reports the actionable case: two independent impls that can diverge.
func (v Verdict) IsDrift() bool { return v.Class == "drift" }

// PairInput is the two functions to judge.
type PairInput struct {
	AKey, ASource string
	BKey, BSource string
	Sig           string // the shared signature that made them candidates
}

// doer is the HTTP surface (so tests inject a fake without a network or key).
type doer interface {
	Do(*http.Request) (*http.Response, error)
}

// Judge holds the model + credentials + cache for a run.
type Judge struct {
	model    string
	apiKey   string
	cacheDir string
	http     doer
}

const defaultModel = "claude-opus-4-8"

// NewJudge builds a judge from the environment. CALQUE_API_KEY or ANTHROPIC_API_KEY
// is required (the oracle makes paid API calls — fail loudly if absent rather than
// silently degrade). CALQUE_JUDGE_MODEL overrides the model (set it to
// claude-haiku-4-5 for cheap bulk judging). CALQUE_JUDGE_CACHE overrides the cache
// dir (default: user cache dir / calque / judge).
func NewJudge() (*Judge, error) {
	key := os.Getenv("CALQUE_API_KEY")
	if key == "" {
		key = os.Getenv("ANTHROPIC_API_KEY")
	}
	if key == "" {
		return nil, fmt.Errorf("no API key — set ANTHROPIC_API_KEY (or CALQUE_API_KEY) to use the LLM judge")
	}
	model := os.Getenv("CALQUE_JUDGE_MODEL")
	if model == "" {
		model = defaultModel
	}
	cache := os.Getenv("CALQUE_JUDGE_CACHE")
	if cache == "" {
		if d, err := os.UserCacheDir(); err == nil {
			cache = filepath.Join(d, "calque", "judge")
		} else {
			cache = filepath.Join(os.TempDir(), "calque-judge-cache")
		}
	}
	return &Judge{
		model:    model,
		apiKey:   key,
		cacheDir: cache,
		http:     &http.Client{Timeout: 120 * time.Second},
	}, nil
}

// Model reports the model in use (for the report header).
func (j *Judge) Model() string { return j.model }

// schemaVersion is bumped whenever the prompt or verdict shape changes, so old
// cached verdicts (a different schema) are not read back.
const schemaVersion = "v2-taxonomy"

// cacheKey hashes everything that affects the verdict: schema + model + both
// sources. A body edit, model change, or prompt/schema bump re-judges; an
// unchanged pair is a free disk read.
func (j *Judge) cacheKey(in PairInput) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00%s\x00%s", schemaVersion, j.model, in.AKey, in.ASource, in.BKey, in.BSource)
	return hex.EncodeToString(h.Sum(nil))
}

// JudgePair returns the oracle's verdict, from cache when possible.
func (j *Judge) JudgePair(ctx context.Context, in PairInput) (Verdict, error) {
	key := j.cacheKey(in)
	if v, ok := j.readCache(key); ok {
		v.Cached = true
		return v, nil
	}
	v, err := j.callAPI(ctx, in)
	if err != nil {
		return Verdict{}, err
	}
	j.writeCache(key, v)
	return v, nil
}

func (j *Judge) readCache(key string) (Verdict, bool) {
	b, err := os.ReadFile(filepath.Join(j.cacheDir, key+".json"))
	if err != nil {
		return Verdict{}, false
	}
	var v Verdict
	if json.Unmarshal(b, &v) != nil {
		return Verdict{}, false
	}
	return v, true
}

func (j *Judge) writeCache(key string, v Verdict) {
	if os.MkdirAll(j.cacheDir, 0o755) != nil {
		return // cache is best-effort; a write failure must not break the run
	}
	if b, err := json.Marshal(v); err == nil {
		_ = os.WriteFile(filepath.Join(j.cacheDir, key+".json"), b, 0o644)
	}
}

const judgeSystem = `You are a code-equivalence judge for calque, a dual-path drift detector.
Two functions share a CONTRACT when, given equivalent inputs, a caller expects equivalent
observable results — the same role and responsibility — regardless of how differently they
are written. Classify the pair into exactly one class:

- "drift": two INDEPENDENT implementations of the same contract that can diverge (or already
  have). Neither calls the other; each could be edited without the other following. This is
  the dangerous case — a maintainer should collapse them to one source of truth. (Example:
  two functions that both resolve a session's worktree, one reading a JSON file and one
  rebuilding from git, whose fields have already drifted apart.)

- "contracted-twin-ok": the same contract, but the parallelism is INTENTIONAL and safe — a
  thin wrapper/adapter that delegates to the other, or a deliberately mirrored pair. Collapsing
  would remove intended indirection; the right action is to pin them with a differential test,
  not merge them.

- "false-alarm": NOT the same contract. They merely share a type signature but do different
  jobs (insert vs update, find-by-X vs find-by-Y, escalate vs cap), OR one is a higher-level
  dispatcher that delegates to the other while adding real logic of its own (a layering
  relationship, not a duplicate). A shared shape is not a shared contract.

Respond with ONLY a JSON object, no prose around it:
{"class": "drift"|"contracted-twin-ok"|"false-alarm", "confidence": "low"|"medium"|"high", "reason": "<one sentence>"}`

// apiRequest / apiResponse are the minimal /v1/messages shapes calque needs.
type apiContentText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (j *Judge) callAPI(ctx context.Context, in PairInput) (Verdict, error) {
	user := fmt.Sprintf("Shared signature: %s\n\nFunction A — %s:\n```\n%s\n```\n\nFunction B — %s:\n```\n%s\n```\n\nAre these behavioral twins (the same contract)?",
		in.Sig, in.AKey, in.ASource, in.BKey, in.BSource)

	body := map[string]any{
		"model":      j.model,
		"max_tokens": 2048,
		"system":     judgeSystem,
		"messages": []map[string]any{
			{"role": "user", "content": user},
		},
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return Verdict{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(buf))
	if err != nil {
		return Verdict{}, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", j.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := j.http.Do(req)
	if err != nil {
		return Verdict{}, err
	}
	defer resp.Body.Close()
	var raw struct {
		Content []apiContentText `json:"content"`
		Error   *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&raw); err != nil {
		return Verdict{}, fmt.Errorf("judge: decoding API response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("HTTP %d", resp.StatusCode)
		if raw.Error != nil {
			msg = raw.Error.Message
		}
		return Verdict{}, fmt.Errorf("judge: API error: %s", msg)
	}
	return parseVerdict(raw.Content)
}

var jsonObjRe = regexp.MustCompile(`(?s)\{.*\}`)

// parseVerdict pulls the verdict JSON out of the model's text blocks. Defensive:
// it scans for the first {...} object across all text blocks, tolerating any
// stray prose or thinking the model emitted around it.
func parseVerdict(blocks []apiContentText) (Verdict, error) {
	var text strings.Builder
	for _, b := range blocks {
		if b.Type == "text" {
			text.WriteString(b.Text)
		}
	}
	m := jsonObjRe.FindString(text.String())
	if m == "" {
		return Verdict{}, fmt.Errorf("judge: no JSON object in model output: %q", text.String())
	}
	var v Verdict
	if err := json.Unmarshal([]byte(m), &v); err != nil {
		return Verdict{}, fmt.Errorf("judge: malformed verdict JSON %q: %w", m, err)
	}
	if v.Confidence == "" {
		v.Confidence = "low"
	}
	// Conservative default: an unrecognized class is treated as not-a-twin rather
	// than risk a false "drift" on a malformed verdict.
	switch v.Class {
	case "drift", "contracted-twin-ok", "false-alarm":
	default:
		v.Class = "false-alarm"
	}
	return v, nil
}
