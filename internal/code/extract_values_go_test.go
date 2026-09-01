package code

import (
	"os"
	"path/filepath"
	"testing"
)

// writeValuesFixture writes src to a temp .go file and extracts its
// Kind:"value-site" fragments via extractGoValuesBatch.
func writeValuesFixture(t *testing.T, src string) []*FuncSig {
	t.Helper()
	dir := t.TempDir()
	f := filepath.Join(dir, "values.go")
	if err := os.WriteFile(f, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	sigs, err := extractGoValuesBatch([]string{f}, dir)
	if err != nil {
		t.Fatalf("extractGoValuesBatch: %v", err)
	}
	return sigs
}

// TestExtractGoValuesFindsMatchingNameStemPair is the core claim behind
// Feature D: the same literal bound to a similarly-named identifier at two
// independent, unrelated call sites (no shared const backing it) extracts as
// two Kind:"value-site" fragments that ValueSiteCandidates pairs.
func TestExtractGoValuesFindsMatchingNameStemPair(t *testing.T) {
	src := `package p

func dialWithRetry() {
	maxRetries := 3
	_ = maxRetries
}

func fetchWithRetry() {
	maxRetries := 3
	_ = maxRetries
}
`
	sites := writeValuesFixture(t, src)
	if len(sites) != 2 {
		t.Fatalf("expected 2 value-sites, got %d", len(sites))
	}
	for _, s := range sites {
		if s.Kind != "value-site" {
			t.Errorf("expected Kind=value-site, got %q", s.Kind)
		}
		if s.Value != "3" {
			t.Errorf("expected Value=3, got %q", s.Value)
		}
		if s.Name != "maxRetries" {
			t.Errorf("expected Name=maxRetries, got %q", s.Name)
		}
	}
	if sites[0].Key() == sites[1].Key() {
		t.Fatalf("both sites produced the same Key() — collision: %s", sites[0].Key())
	}

	for _, s := range sites {
		s.Prepare()
	}
	cands := ValueSiteCandidates(sites, 0.01, 8)
	if len(cands) != 1 {
		t.Fatalf("expected 1 candidate pair, got %d", len(cands))
	}
	if cands[0].A.Value != "3" || cands[0].B.Value != "3" {
		t.Errorf("expected both sides of the pair to carry Value=3")
	}
}

// TestExtractGoValuesSkipsTrivialValues: 0, 1, -1, true, false, and empty
// string are excluded — the standard magic-number-linter exclude list.
func TestExtractGoValuesSkipsTrivialValues(t *testing.T) {
	src := `package p

func trivials() {
	a := 0
	b := 1
	c := -1
	d := true
	e := false
	f := ""
	_ = a
	_ = b
	_ = c
	_ = d
	_ = e
	_ = f
}
`
	sites := writeValuesFixture(t, src)
	if len(sites) != 0 {
		t.Fatalf("expected 0 value-sites (all trivial), got %d: %v", len(sites), qualnames(sites))
	}
}

// TestExtractGoValuesNegativeNumberKept: a non-trivial negative number (-3)
// must still be captured — only -1 (and 0/1) are excluded.
func TestExtractGoValuesNegativeNumberKept(t *testing.T) {
	src := `package p

func offset() {
	yOffset := -3
	_ = yOffset
}
`
	sites := writeValuesFixture(t, src)
	if len(sites) != 1 {
		t.Fatalf("expected 1 value-site, got %d", len(sites))
	}
	if sites[0].Value != "-3" {
		t.Errorf("expected Value=-3, got %q", sites[0].Value)
	}
}

// TestExtractGoValuesPackageScopeConstDecl: a package-level const/var
// declaration is a value-site too, with no enclosing function (package
// scope) — Qualname has no dotted function prefix.
func TestExtractGoValuesPackageScopeConstDecl(t *testing.T) {
	src := `package p

const maxRetries = 5
`
	sites := writeValuesFixture(t, src)
	if len(sites) != 1 {
		t.Fatalf("expected 1 value-site, got %d", len(sites))
	}
	if sites[0].Value != "5" || sites[0].Name != "maxRetries" {
		t.Errorf("expected Value=5 Name=maxRetries, got Value=%q Name=%q", sites[0].Value, sites[0].Name)
	}
}

// TestExtractGoValuesCompositeLiteralKey: a struct-literal field is a
// value-site keyed by the field name.
func TestExtractGoValuesCompositeLiteralKey(t *testing.T) {
	src := `package p

type Config struct{ MaxRetries int }

func build() Config {
	return Config{MaxRetries: 7}
}
`
	sites := writeValuesFixture(t, src)
	if len(sites) != 1 {
		t.Fatalf("expected 1 value-site, got %d", len(sites))
	}
	if sites[0].Value != "7" || sites[0].Name != "MaxRetries" {
		t.Errorf("expected Value=7 Name=MaxRetries, got Value=%q Name=%q", sites[0].Value, sites[0].Name)
	}
}

// TestValueSiteCandidatesRequiresNameStemOverlap: same value, UNRELATED
// names, should not pair at the default (positive) name-jaccard — a
// coincidental shared literal like 3 recurs constantly for unrelated
// reasons — but should pair once name-jaccard is loosened to 0 (accept any
// same-value pair regardless of name, the low-confidence residual tail).
func TestValueSiteCandidatesRequiresNameStemOverlap(t *testing.T) {
	src := `package p

func retryOp() {
	maxRetries := 3
	_ = maxRetries
}

func layoutGrid() {
	gridSize := 3
	_ = gridSize
}
`
	sites := writeValuesFixture(t, src)
	if len(sites) != 2 {
		t.Fatalf("expected 2 value-sites, got %d", len(sites))
	}
	for _, s := range sites {
		s.Prepare()
	}

	anchored := ValueSiteCandidates(sites, 0.01, 8)
	if len(anchored) != 0 {
		t.Fatalf("expected 0 candidates with name-stem anchoring required (maxRetries vs gridSize share no token), got %d", len(anchored))
	}

	residual := ValueSiteCandidates(sites, 0, 8)
	if len(residual) != 1 {
		t.Fatalf("expected 1 residual candidate with name-jaccard=0 (value-match alone), got %d", len(residual))
	}
}
