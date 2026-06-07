package code

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/justinstimatze/calque/internal/glob"
)

// extractors maps a file extension to a BATCH extractor (all paths of that ext
// in one call). go/ast runs in-process; python3 runs once per scan as a
// subprocess (so a large scan spawns one interpreter, not one per file).
var extractors = map[string]func(paths []string, root string) ([]*FuncSig, error){
	".go": extractGoBatch,
	".py": extractPyBatch,
}

// codeExts are extensions calque considers "code" — used to count files skipped
// for want of an extractor (so we don't report .md/.json as skipped code).
var codeExts = set{
	".go": {}, ".py": {}, ".ts": {}, ".tsx": {}, ".js": {}, ".jsx": {},
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
}

// Extract walks repo, runs the matching extractor per supported source file, and
// returns every FuncSig (Prepared) plus coverage stats. Skips VCS/dep/build dirs
// and hidden dirs. legacy/ is intentionally NOT special-cased — once a .py
// extractor exists, exclude it via --left/--right if needed.
func Extract(repo string, exclude []string) ([]*FuncSig, ScanStats, error) {
	st := ScanStats{SkippedExts: map[string]int{}}
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
		if _, ok := extractors[ext]; ok {
			byExt[ext] = append(byExt[ext], p)
		} else if codeExts.has(ext) {
			st.Skipped++
			st.SkippedExts[ext]++
		}
		return nil
	})
	if err != nil {
		return nil, st, err
	}

	var all []*FuncSig
	for ext, paths := range byExt {
		sigs, exErr := extractors[ext](paths, repo)
		if exErr != nil {
			return nil, st, fmt.Errorf("%s extractor: %w", ext, exErr)
		}
		for _, s := range sigs {
			s.Prepare()
		}
		all = append(all, sigs...)
		st.Files += len(paths)
		st.Funcs += len(sigs)
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

// SupportedExts returns the extensions calque can currently extract (for help/UX).
func SupportedExts() []string {
	out := make([]string, 0, len(extractors))
	for e := range extractors {
		out = append(out, e)
	}
	return out
}
