package code

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/justinstimatze/calque/internal/glob"
)

// extractors maps a file extension to a BATCH extractor (all paths of that ext
// in one call). go/ast runs in-process; python3 runs once per scan as a
// subprocess (so a large scan spawns one interpreter, not one per file).
var extractors = map[string]func(paths []string, root string) ([]*FuncSig, error){
	".go":     extractGoBatch,
	".py":     extractPyBatch,
	".ts":     extractTSBatch,
	".tsx":    extractTSBatch,
	".js":     extractTSBatch, // the TS parser accepts plain JS (superset grammar)
	".jsx":    extractTSBatch, // extract_ts.mjs already selects ScriptKind.TSX for it
	".mjs":    extractTSBatch,
	".cjs":    extractTSBatch,
	".svelte": extractTSBatch, // <script lang="ts"> blocks, masked from the template
	".rs":     extractRustBatch,
}

// codeExts are extensions calque considers "code" — used to count files skipped
// for want of an extractor (so we don't report .md/.json as skipped code).
var codeExts = set{
	".go": {}, ".py": {}, ".ts": {}, ".tsx": {}, ".js": {}, ".jsx": {}, ".svelte": {},
	".rs": {}, ".java": {}, ".rb": {}, ".c": {}, ".cc": {}, ".cpp": {}, ".h": {},
}

var sourceSkipDirs = set{
	".git": {}, "node_modules": {}, "vendor": {}, "dist": {}, "build": {},
	"target": {}, ".calque": {}, ".venv": {}, "__pycache__": {}, "testdata": {},
}

// ScanStats summarizes what a walk covered.
type ScanStats struct {
	Files       int
	Funcs       int
	Skipped     int            // code files with no extractor yet
	SkippedExts map[string]int // by extension
	// CodeFiles is every code-file path the walk visited — both the files an
	// extractor handled AND the skipped-for-want-of-an-extractor ones — in the same
	// path form FuncSig.File uses. It lets a caller answer "did this boundary glob
	// match files on disk that simply didn't parse?" — the input to the
	// boundary-cannot-bite warning that turns a silent recall hole into a visible one.
	CodeFiles []string
}

// walkExtractable walks repo and groups source files by extension, applying the
// shared skip-dir/exclude rules. accept decides which extensions to collect; onSkip
// (may be nil) sees every other code-file extension and its path (Extract counts
// those as skipped-for-want-of-an-extractor and records the paths; ExtractSymbols
// ignores them). Single-sources the tree walk shared by the function and symbol
// extractors.
func walkExtractable(repo string, exclude []string, accept func(ext string) bool, onSkip func(ext, path string)) (map[string][]string, error) {
	exRe := glob.Compile(exclude)
	excluded := func(rel string) bool { return glob.MatchAny(exRe, rel) }
	byExt := map[string][]string{}
	err := filepath.WalkDir(repo, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(repo, p)
		if d.IsDir() {
			if p != repo && (sourceSkipDirs.has(d.Name()) || strings.HasPrefix(d.Name(), ".") || excluded(rel)) {
				return filepath.SkipDir
			}
			return nil
		}
		if excluded(rel) {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		if accept(ext) {
			byExt[ext] = append(byExt[ext], p)
		} else if onSkip != nil {
			onSkip(ext, p)
		}
		return nil
	})
	return byExt, err
}

// Extract walks repo, runs the matching extractor per supported source file, and
// returns every FuncSig (Prepared) plus coverage stats. Skips VCS/dep/build dirs
// and hidden dirs. legacy/ is intentionally NOT special-cased — once a .py
// extractor exists, exclude it via --left/--right if needed.
func Extract(repo string, exclude []string) ([]*FuncSig, ScanStats, error) {
	byExt, st, err := walkSources(repo, exclude)
	if err != nil {
		return nil, st, err
	}

	var all []*FuncSig
	for ext, paths := range byExt {
		sigs, exErr := extractors[ext](paths, repo)
		if exErr != nil {
			return nil, st, fmt.Errorf("%s extractor: %w", ext, exErr)
		}
		prepareSigs(sigs)
		all = append(all, sigs...)
		st.Files += len(paths)
		st.Funcs += len(sigs)
		st.CodeFiles = append(st.CodeFiles, paths...)
	}
	return all, st, nil
}

// Filter returns the FuncSigs whose File matches any of the comma-separated glob
// patterns (supporting **, *, ?). An empty pattern set returns all (the default,
// self-scan-everything). Invalid globs are skipped.
func Filter(sigs []*FuncSig, globsCSV string) []*FuncSig {
	globsCSV = strings.TrimSpace(globsCSV)
	if globsCSV == "" {
		return sigs
	}
	res := glob.Compile(strings.Split(globsCSV, ","))
	if len(res) == 0 {
		return sigs
	}
	var out []*FuncSig
	for _, f := range sigs {
		if glob.MatchAny(res, f.File) {
			out = append(out, f)
		}
	}
	return out
}

// MatchGlob returns the subset of file paths matching the glob CSV — the SAME
// matcher Filter uses over FuncSig.File, so a boundary glob selects the same files
// here as it does among parsed functions. An empty/whitespace CSV returns nil:
// the whole-repo default (no boundary) can never under-bite, so callers skip the
// warning for it.
func MatchGlob(files []string, globsCSV string) []string {
	globsCSV = strings.TrimSpace(globsCSV)
	if globsCSV == "" {
		return nil
	}
	res := glob.Compile(strings.Split(globsCSV, ","))
	if len(res) == 0 {
		return nil
	}
	var out []string
	for _, f := range files {
		if glob.MatchAny(res, f) {
			out = append(out, f)
		}
	}
	return out
}

// HasExtractor reports whether calque has a function-axis extractor for ext
// (lowercased, leading dot). Used to explain a zero-bite boundary: a matched file
// whose extension has no extractor is why the side parsed nothing.
func HasExtractor(ext string) bool {
	_, ok := extractors[ext]
	return ok
}

// SupportedExts returns the extensions calque can currently extract (for help/UX).
func SupportedExts() []string {
	out := make([]string, 0, len(extractors))
	for e := range extractors {
		out = append(out, e)
	}
	return out
}

// walkSources runs the shared tree-walk for the function extractors: it groups
// supported source files by extension and pre-fills the skip stats (code files
// with no extractor yet). Extract and ExtractCached share this — they differ only
// in whether the per-file extraction is cached — so the walk + skip-accounting
// lives in one place.
func walkSources(repo string, exclude []string) (map[string][]string, ScanStats, error) {
	st := ScanStats{SkippedExts: map[string]int{}}
	byExt, err := walkExtractable(repo, exclude,
		func(ext string) bool { _, ok := extractors[ext]; return ok },
		func(ext, path string) {
			if codeExts.has(ext) {
				st.Skipped++
				st.SkippedExts[ext]++
				st.CodeFiles = append(st.CodeFiles, path)
			}
		})
	return byExt, st, err
}

// prepareSigs finalizes a batch of freshly-extracted sigs: it builds each sig's
// derived sets (Prepare) and attributes test files — unioning the file-path
// convention (IsTestPath) with whatever the extractor already flagged inline
// (Rust #[cfg(test)] / #[test]). Shared by Extract and ExtractCached so the
// finalize step can't drift between the cached and uncached paths.
func prepareSigs(sigs []*FuncSig) {
	for _, s := range sigs {
		s.Prepare()
		if IsTestPath(s.File) {
			s.Test = true
		}
	}
}

// ExtractPending extracts FuncSigs from an in-memory source buffer — the pending
// post-edit content of one file — by materializing it to a temp file the existing
// per-extension extractor can parse. The caller (the author-time `nearest` path)
// composes the FULL post-edit file before calling, so the buffer is a complete,
// parseable compilation unit, not a bare diff fragment that would fail to parse.
// Each returned sig's File is set to relPath (the buffer's real repo-relative
// path) so author-time self-exclusion — Nearest skipping the query's own prior
// version already in the corpus — and the surfaced location both line up.
func ExtractPending(content, ext, relPath string) ([]*FuncSig, error) {
	ex, ok := extractors[ext]
	if !ok {
		return nil, fmt.Errorf("no extractor for %q", ext)
	}
	dir, err := os.MkdirTemp("", "calque-nearest-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	tmp := filepath.Join(dir, "pending"+ext)
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return nil, err
	}
	sigs, err := ex([]string{tmp}, dir)
	if err != nil {
		return nil, err
	}
	for _, s := range sigs {
		s.File = relPath
	}
	prepareSigs(sigs)
	return sigs, nil
}
