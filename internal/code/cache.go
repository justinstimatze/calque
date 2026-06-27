package code

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// indexCacheVersion bumps when the cache layout or the FuncSig interchange
// changes in a way that would invalidate stored sigs. A version mismatch forces
// a full rebuild rather than trusting a stale shape.
const indexCacheVersion = 1

// IndexCachePath is where ExtractCached persists its per-file extraction cache —
// alongside the registry under .calque/, so a project's calque state lives in one
// place.
func IndexCachePath(repo string) string {
	return filepath.Join(repo, ".calque", "index.json")
}

// cacheKey maps a walked file path to the form FuncSig.File uses (repo-relative),
// so cache entries key on the same string the extractor stamps into each sig.
// Keying on the raw walk path breaks when repo is absolute (walk paths are
// absolute, FuncSig.File is relative) — the two only coincide under --repo ".".
func cacheKey(repo, p string) string {
	if rel, err := filepath.Rel(repo, p); err == nil {
		return rel
	}
	return p
}

// cachedFile is one file's extraction result plus the stat fingerprint that
// validates it. mtime+size unchanged ⇒ the stored Sigs are still good.
type cachedFile struct {
	ModNano int64      `json:"mtime"`
	Size    int64      `json:"size"`
	Sigs    []*FuncSig `json:"sigs"`
}

// indexCache is the on-disk map from FuncSig.File path to that file's cached
// extraction.
type indexCache struct {
	Version int                   `json:"version"`
	Files   map[string]cachedFile `json:"files"`
}

func loadIndexCache(path string) indexCache {
	fresh := indexCache{Version: indexCacheVersion, Files: map[string]cachedFile{}}
	b, err := os.ReadFile(path)
	if err != nil {
		return fresh
	}
	var got indexCache
	// Unreadable, wrong schema version, or nil map ⇒ cold rebuild, never a crash.
	if json.Unmarshal(b, &got) != nil || got.Version != indexCacheVersion || got.Files == nil {
		return fresh
	}
	return got
}

// saveIndexCache writes atomically (tmp + rename) so a crash mid-write can't
// leave a half-written cache that loadIndexCache would reject anyway.
func saveIndexCache(path string, c indexCache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return atomicWrite(path, b, 0o644)
}

// atomicWrite writes data to path via a sibling temp file + rename, so a reader
// never observes a half-written file. Single authority for calque's
// write-tmp-then-rename idiom (saveIndexCache and installBinary).
func atomicWrite(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ExtractCached is Extract with a persistent per-file cache: a file whose mtime
// and size are unchanged since the last run reuses its stored FuncSigs; only new
// or modified files are re-parsed. This makes author-time / pre-write queries
// affordable — re-extracting a whole repo per edit blows a PreToolUse latency
// budget, but re-parsing only the touched file does not.
//
// The result is set-equivalent to Extract (the recall passes sort + dedup, so
// per-file ordering doesn't matter). A missing, corrupt, or stale-version cache
// degrades to a full Extract-equivalent rebuild. A cache WRITE failure is
// non-fatal — the sigs are still returned — so a read-only .calque never breaks
// a scan; the next run simply rebuilds.
func ExtractCached(repo string, exclude []string) ([]*FuncSig, ScanStats, error) {
	byExt, st, err := walkSources(repo, exclude)
	if err != nil {
		return nil, st, err
	}

	cachePath := IndexCachePath(repo)
	cache := loadIndexCache(cachePath)
	next := indexCache{Version: indexCacheVersion, Files: map[string]cachedFile{}}

	var all []*FuncSig
	for ext, paths := range byExt {
		st.CodeFiles = append(st.CodeFiles, paths...)

		// stale collects paths needing re-extraction; meta remembers their stat
		// fingerprint so we don't stat twice.
		var stale []string
		meta := map[string][2]int64{}
		for _, p := range paths {
			key := cacheKey(repo, p)
			fi, statErr := os.Stat(p)
			if statErr != nil {
				stale = append(stale, p) // unstatable ⇒ force re-extract
				continue
			}
			mod, size := fi.ModTime().UnixNano(), fi.Size()
			if ent, ok := cache.Files[key]; ok && ent.ModNano == mod && ent.Size == size {
				for _, s := range ent.Sigs {
					s.Prepare() // derived sets are unexported — rebuild after JSON load
				}
				next.Files[key] = ent
				all = append(all, ent.Sigs...)
				st.Files++
				st.Funcs += len(ent.Sigs)
				continue
			}
			stale = append(stale, p)
			meta[p] = [2]int64{mod, size}
		}
		if len(stale) == 0 {
			continue
		}

		sigs, exErr := extractors[ext](stale, repo)
		if exErr != nil {
			return nil, st, fmt.Errorf("%s extractor: %w", ext, exErr)
		}
		// Bucket fresh sigs back per file so each gets its own cache entry. A stale
		// file yielding zero sigs still gets an (empty) entry, so it isn't re-parsed
		// every run.
		prepareSigs(sigs)
		buckets := map[string][]*FuncSig{}
		for _, s := range sigs {
			buckets[s.File] = append(buckets[s.File], s)
		}
		for _, p := range stale {
			key := cacheKey(repo, p)
			m := meta[p] // zero-value {0,0} if it was unstatable — next run re-extracts
			next.Files[key] = cachedFile{ModNano: m[0], Size: m[1], Sigs: buckets[key]}
		}
		all = append(all, sigs...)
		st.Files += len(stale)
		st.Funcs += len(sigs)
	}

	// Best-effort persist: a write failure (read-only .calque, full disk) must not
	// fail an otherwise-successful scan.
	_ = saveIndexCache(cachePath, next)
	return all, st, nil
}
