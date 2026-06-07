package code

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
)

// extractors maps a file extension to its FuncSig extractor. go/ast is
// in-process; .py lands next as a python3-subprocess extractor (the stope target).
var extractors = map[string]func(path, root string) []*FuncSig{
	".go": ExtractGoFile,
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
func Extract(repo string) ([]*FuncSig, ScanStats, error) {
	var all []*FuncSig
	st := ScanStats{SkippedExts: map[string]int{}}
	err := filepath.WalkDir(repo, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != repo && (sourceSkipDirs.has(d.Name()) || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		ex, ok := extractors[ext]
		if !ok {
			if codeExts.has(ext) {
				st.Skipped++
				st.SkippedExts[ext]++
			}
			return nil
		}
		sigs := ex(p, repo)
		all = append(all, sigs...)
		st.Files++
		st.Funcs += len(sigs)
		return nil
	})
	return all, st, err
}

// Filter returns the FuncSigs whose File matches any of the comma-separated glob
// patterns (supporting **, *, ?). An empty pattern set returns all (the default,
// self-scan-everything). Invalid globs are skipped.
func Filter(sigs []*FuncSig, globsCSV string) []*FuncSig {
	globsCSV = strings.TrimSpace(globsCSV)
	if globsCSV == "" {
		return sigs
	}
	var res []*regexp.Regexp
	for _, g := range strings.Split(globsCSV, ",") {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		if re, err := globToRegexp(g); err == nil {
			res = append(res, re)
		}
	}
	if len(res) == 0 {
		return sigs
	}
	var out []*FuncSig
	for _, f := range sigs {
		for _, re := range res {
			if re.MatchString(f.File) {
				out = append(out, f)
				break
			}
		}
	}
	return out
}

// globToRegexp compiles a path glob into an anchored regexp. `**/` matches zero
// or more leading directories; `**` matches across separators; `*` and `?` do
// not cross `/`.
func globToRegexp(g string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(g); i++ {
		c := g[i]
		switch c {
		case '*':
			if i+1 < len(g) && g[i+1] == '*' {
				i++ // consume the second '*'
				if i+1 < len(g) && g[i+1] == '/' {
					i++ // consume the '/'
					b.WriteString("(?:.*/)?")
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '[', ']', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

// SupportedExts returns the extensions calque can currently extract (for help/UX).
func SupportedExts() []string {
	out := make([]string, 0, len(extractors))
	for e := range extractors {
		out = append(out, e)
	}
	return out
}
