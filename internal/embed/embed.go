// Package embed is a thin local-ollama embedding client for calque's prose
// axis: it turns words/phrases into L2-normalized vectors so cosine similarity
// is a plain dot product. Used by synonym-report (near-synonym surfacing) and
// available to any future embedding-based recall signal.
//
// Ported from cupel (MIT) cmd/cupel/client.go (the ollama /api/embed half;
// cupel's Anthropic "lens" client is left behind). Env vars renamed CUPEL_*→CALQUE_*.
package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

// URL returns the ollama base URL (CALQUE_OLLAMA_URL, default localhost:11434).
func URL() string {
	if u := os.Getenv("CALQUE_OLLAMA_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	return "http://localhost:11434"
}

// Model returns the embedding model (CALQUE_EMBED_MODEL, default nomic-embed-text
// — fast and cleanly separating per cupel's probe).
func Model() string {
	if m := os.Getenv("CALQUE_EMBED_MODEL"); m != "" {
		return m
	}
	return "nomic-embed-text"
}

type embedReq struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedResp struct {
	Embeddings [][]float64 `json:"embeddings"`
}

// texts embeds one batch in a single round trip, returning one L2-normalized
// vector per input.
func texts(ctx context.Context, in []string) ([][]float64, error) {
	body, err := json.Marshal(embedReq{Model: Model(), Input: in})
	if err != nil {
		return nil, fmt.Errorf("ollama embed: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, URL()+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama embed: status %d (is ollama running at %s?)", resp.StatusCode, URL())
	}
	var er embedResp
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		return nil, err
	}
	if len(er.Embeddings) != len(in) {
		return nil, fmt.Errorf("ollama embed: got %d vectors for %d inputs", len(er.Embeddings), len(in))
	}
	for i := range er.Embeddings {
		normalize(er.Embeddings[i])
	}
	return er.Embeddings, nil
}

// TextsBatched embeds many texts in fixed-size chunks, each on its own generous
// (90s) timeout, so a large corpus never exceeds a single request budget.
func TextsBatched(in []string, chunk int) ([][]float64, error) {
	if chunk <= 0 {
		chunk = 64
	}
	all := make([][]float64, 0, len(in))
	for i := 0; i < len(in); i += chunk {
		end := i + chunk
		if end > len(in) {
			end = len(in)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		vs, err := texts(ctx, in[i:end])
		cancel()
		if err != nil {
			return nil, fmt.Errorf("embed chunk %d-%d: %w", i, end, err)
		}
		all = append(all, vs...)
	}
	return all, nil
}

func normalize(v []float64) {
	var n float64
	for _, x := range v {
		n += x * x
	}
	n = math.Sqrt(n)
	if n == 0 {
		return
	}
	for i := range v {
		v[i] /= n
	}
}

// Cosine of two already-normalized vectors (a plain dot product).
func Cosine(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot float64
	for i := range a {
		dot += a[i] * b[i]
	}
	return dot
}
