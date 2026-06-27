package code

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// sigKeys returns the sorted Keys of a sig slice — an order-independent identity
// for the set, since ExtractCached may return per-file groups in a different
// order than Extract.
func sigKeys(sigs []*FuncSig) []string {
	ks := make([]string, len(sigs))
	for i, s := range sigs {
		ks[i] = s.Key()
	}
	sort.Strings(ks)
	return ks
}

func writeFile(t *testing.T, dir, rel, body string) string {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const goFileA = `package x

func Alpha() int {
	total := 0
	for i := 0; i < 10; i++ {
		total += i
	}
	return total
}
`

const goFileB = `package x

func Beta(s string) string {
	out := ""
	for _, c := range s {
		out += string(c)
	}
	return out
}
`

// TestExtractCachedEquivalence: a cold ExtractCached (no cache yet) must return
// the same sig set as plain Extract, and a warm second run (everything cached)
// must still return that same set — proving the cache is transparent.
func TestExtractCachedEquivalence(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, repo, "a.go", goFileA)
	writeFile(t, repo, "sub/b.go", goFileB)

	plain, _, err := Extract(repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := sigKeys(plain)
	if len(want) == 0 {
		t.Fatal("fixture produced no sigs; extractor unavailable?")
	}

	cold, _, err := ExtractCached(repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := sigKeys(cold); !equalStrs(got, want) {
		t.Errorf("cold ExtractCached set != Extract set\n got=%v\nwant=%v", got, want)
	}
	if _, err := os.Stat(IndexCachePath(repo)); err != nil {
		t.Errorf("cache file not written: %v", err)
	}

	warm, _, err := ExtractCached(repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := sigKeys(warm); !equalStrs(got, want) {
		t.Errorf("warm ExtractCached set != Extract set\n got=%v\nwant=%v", got, want)
	}
}

// TestExtractCachedPicksUpChange: after a file is modified, ExtractCached must
// reflect the new content (not serve a stale cached sig). Touch the mtime
// forward to defeat same-second-write coarseness.
func TestExtractCachedPicksUpChange(t *testing.T) {
	repo := t.TempDir()
	a := writeFile(t, repo, "a.go", goFileA)

	if _, _, err := ExtractCached(repo, nil); err != nil {
		t.Fatal(err)
	}

	// Replace Alpha with a differently-named function and bump mtime.
	writeFile(t, repo, "a.go", "package x\n\nfunc Gamma() int { return 1 }\n")
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(a, future, future); err != nil {
		t.Fatal(err)
	}

	got, _, err := ExtractCached(repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, s := range got {
		names[s.Name] = true
	}
	if names["Alpha"] {
		t.Error("ExtractCached served a stale sig for a changed file (Alpha still present)")
	}
	if !names["Gamma"] {
		t.Error("ExtractCached missed the new function Gamma after the file changed")
	}
}

// TestExtractCachedCorruptCacheRebuilds: a garbage cache file must not crash or
// poison results — it degrades to a full rebuild.
func TestExtractCachedCorruptCacheRebuilds(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, repo, "a.go", goFileA)
	if err := os.MkdirAll(filepath.Join(repo, ".calque"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(IndexCachePath(repo), []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, _, err := ExtractCached(repo, nil)
	if err != nil {
		t.Fatalf("corrupt cache should rebuild, not error: %v", err)
	}
	if len(got) == 0 {
		t.Error("corrupt cache rebuild produced no sigs")
	}
}

func equalStrs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
