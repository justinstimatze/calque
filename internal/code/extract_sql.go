package code

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// SQL-schema extractor — the third cross-substrate emitter (after .py tables and
// .json corpus shapes). A CREATE TABLE statement's COLUMN SET is its footprint
// (-> RetKeys), so a SQL schema — in a .sql file or embedded as a string in source
// — can pair against the corpus shape / code table it mirrors (e.g. a db
// `temporal_markers` schema's columns vs the authored corpus temporal_markers
// fields, the case the .py/.json extractors alone couldn't bridge). Pure Go: a
// tolerant CREATE TABLE scanner (no SQL engine), good enough for column sets.

// sqlBearingExts are scanned for CREATE TABLE — standalone .sql plus source files
// that commonly embed schema strings (db.py, a Go migration, etc.). A file matched
// here may ALSO be handled by a symbolExtractor (db.py has both module dicts AND
// CREATE TABLE strings); the two passes extract different entities from it.
var sqlBearingExts = set{
	".sql": {}, ".py": {}, ".go": {}, ".ts": {}, ".tsx": {}, ".js": {}, ".rb": {},
}

var createTableRe = regexp.MustCompile(
	"(?is)CREATE\\s+TABLE\\s+(?:IF\\s+NOT\\s+EXISTS\\s+)?[`\"']?([A-Za-z_][A-Za-z0-9_.]*)[`\"']?\\s*\\(")

var sqlLineComment = regexp.MustCompile(`--[^\n]*`)
var sqlBlockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)

// sqlConstraintLeader: a body fragment starting with one of these is a table-level
// constraint, not a column definition.
var sqlConstraintLeader = set{
	"PRIMARY": {}, "FOREIGN": {}, "UNIQUE": {}, "CHECK": {}, "CONSTRAINT": {},
	"KEY": {}, "INDEX": {}, "EXCLUDE": {},
}

const sqlMaxCols = 200

func extractSQLBatch(paths []string, root string) ([]*FuncSig, error) {
	var out []*FuncSig
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue // best-effort
		}
		text := string(data)
		if !strings.Contains(strings.ToUpper(text), "CREATE TABLE") {
			continue // fast reject — most files carry no schema
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			rel = p
		}
		for _, t := range parseCreateTables(text) {
			if len(t.cols) < 2 {
				continue // a 0-1 column table isn't a discriminating shape
			}
			out = append(out, &FuncSig{
				File: rel, Qualname: t.name, Name: t.name, Kind: "sql-table",
				Line: t.line, NLines: 1, RetKeys: t.cols, Source: t.src,
			})
		}
	}
	return out, nil
}

type sqlTable struct {
	name string
	cols []string
	line int
	src  string
}

func parseCreateTables(text string) []sqlTable {
	var out []sqlTable
	for _, loc := range createTableRe.FindAllStringSubmatchIndex(text, -1) {
		name := text[loc[2]:loc[3]]
		open := loc[1] - 1 // index of the '(' the regex ended on
		body, end := balancedParen(text, open)
		if body == "" {
			continue
		}
		cols := sqlColumns(body)
		if len(cols) == 0 {
			continue
		}
		srcEnd := end + 1
		if srcEnd > len(text) {
			srcEnd = len(text)
		}
		src := text[loc[0]:srcEnd]
		if len(src) > 4000 {
			src = src[:4000]
		}
		out = append(out, sqlTable{
			name: name,
			cols: cols,
			line: 1 + strings.Count(text[:loc[0]], "\n"),
			src:  src,
		})
	}
	return out
}

// balancedParen returns the contents between the '(' at openIdx and its matching
// ')' (exclusive), plus the index of that ')'.
func balancedParen(text string, openIdx int) (string, int) {
	depth := 0
	for i := openIdx; i < len(text); i++ {
		switch text[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return text[openIdx+1 : i], i
			}
		}
	}
	return "", openIdx
}

// sqlColumns splits a CREATE TABLE body on top-level commas and returns the column
// names — the first identifier of each fragment that isn't a table-level constraint.
// Comments are stripped first so an inline `-- ...` can't swallow the next column.
func sqlColumns(body string) []string {
	body = sqlBlockComment.ReplaceAllString(body, "")
	body = sqlLineComment.ReplaceAllString(body, "")
	var cols []string
	for _, frag := range splitTopComma(body) {
		first := firstIdent(frag)
		if first == "" || sqlConstraintLeader.has(strings.ToUpper(first)) {
			continue
		}
		cols = append(cols, first)
		if len(cols) >= sqlMaxCols {
			break
		}
	}
	cols = dedupStrings(cols)
	sort.Strings(cols)
	return cols
}

// splitTopComma splits on commas that are not nested inside parentheses.
func splitTopComma(s string) []string {
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, s[start:])
}

// firstIdent returns the leading identifier of a column fragment (after trimming
// whitespace and a leading quote/backtick).
func firstIdent(frag string) string {
	frag = strings.TrimLeft(strings.TrimSpace(frag), "`\"'")
	i := 0
	for i < len(frag) && isIdentByte(frag[i]) {
		i++
	}
	return frag[:i]
}

func isIdentByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

func dedupStrings(in []string) []string {
	seen := map[string]bool{}
	out := in[:0:0]
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
