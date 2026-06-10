package llm

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type fakeDoer struct {
	status int
	body   string
	calls  int
}

func (f *fakeDoer) Do(req *http.Request) (*http.Response, error) {
	f.calls++
	// Verify the request is shaped right while we're here.
	if req.Header.Get("x-api-key") == "" {
		return nil, io.ErrUnexpectedEOF
	}
	return &http.Response{
		StatusCode: f.status,
		Body:       io.NopCloser(strings.NewReader(f.body)),
		Header:     http.Header{},
	}, nil
}

func newTestJudge(t *testing.T, fd *fakeDoer) *Judge {
	t.Helper()
	return &Judge{model: "claude-opus-4-8", apiKey: "test-key", cacheDir: t.TempDir(), http: fd}
}

const okBody = `{"content":[{"type":"text","text":"{\"same_contract\": true, \"confidence\": \"high\", \"reason\": \"Both resolve a session's worktree.\"}"}]}`

func TestJudgePairParsesVerdict(t *testing.T) {
	fd := &fakeDoer{status: 200, body: okBody}
	j := newTestJudge(t, fd)
	v, err := j.JudgePair(context.Background(), PairInput{AKey: "a.ts::f", ASource: "x", BKey: "b.ts::g", BSource: "y"})
	if err != nil {
		t.Fatalf("JudgePair: %v", err)
	}
	if !v.SameContract || v.Confidence != "high" {
		t.Errorf("unexpected verdict: %+v", v)
	}
	if v.Cached {
		t.Error("first call should not be cached")
	}
}

func TestJudgePairCaches(t *testing.T) {
	fd := &fakeDoer{status: 200, body: okBody}
	j := newTestJudge(t, fd)
	in := PairInput{AKey: "a.ts::f", ASource: "x", BKey: "b.ts::g", BSource: "y"}
	if _, err := j.JudgePair(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	v2, err := j.JudgePair(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if fd.calls != 1 {
		t.Errorf("expected 1 API call (second served from cache), got %d", fd.calls)
	}
	if !v2.Cached {
		t.Error("second call should be cache-served")
	}
}

func TestJudgePairAPIError(t *testing.T) {
	fd := &fakeDoer{status: 401, body: `{"error":{"type":"authentication_error","message":"invalid x-api-key"}}`}
	j := newTestJudge(t, fd)
	_, err := j.JudgePair(context.Background(), PairInput{AKey: "a", ASource: "x", BKey: "b", BSource: "y"})
	if err == nil || !strings.Contains(err.Error(), "invalid x-api-key") {
		t.Errorf("expected surfaced API error, got %v", err)
	}
}

func TestParseVerdictToleratesProse(t *testing.T) {
	blocks := []apiContentText{
		{Type: "text", Text: "Here is my judgment:\n{\"same_contract\": false, \"confidence\": \"medium\", \"reason\": \"insert vs update.\"}\nDone."},
	}
	v, err := parseVerdict(blocks)
	if err != nil {
		t.Fatal(err)
	}
	if v.SameContract || v.Confidence != "medium" {
		t.Errorf("unexpected: %+v", v)
	}
}

func TestParseVerdictNoJSON(t *testing.T) {
	if _, err := parseVerdict([]apiContentText{{Type: "text", Text: "I cannot decide."}}); err == nil {
		t.Error("expected error when no JSON object present")
	}
}

func TestCacheKeyVariesByModelAndSource(t *testing.T) {
	j := &Judge{model: "claude-opus-4-8"}
	base := PairInput{AKey: "a", ASource: "x", BKey: "b", BSource: "y"}
	k1 := j.cacheKey(base)
	j2 := &Judge{model: "claude-haiku-4-5"}
	if j2.cacheKey(base) == k1 {
		t.Error("model change should change cache key")
	}
	changed := base
	changed.ASource = "x2"
	if j.cacheKey(changed) == k1 {
		t.Error("source change should change cache key")
	}
}
