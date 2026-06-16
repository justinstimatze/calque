package code

// Drift-confessing comments — a function's own source admitting it is one side of a
// twin ("mirrors the audit exactly", "keep in sync"). A literal self-witness: dirt
// cheap to grep, high precision (people rarely write "mirrors X" about code that
// isn't a deliberate parallel). Catches the twin class whose data-flow shape diverges
// enough that the reads / signature passes miss it — the comment is the recall signal.

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/justinstimatze/calque/internal/pairkey"
)

// confessRe matches drift-confessing comment phrases (case-insensitive).
var confessRe = regexp.MustCompile(`(?i)\b(mirror(s|ed|ing)?|keep[ -]in[ -]sync|kept[ -]in[ -]sync|keeps? (it )?in sync|stays? in sync|stayed in sync|kept consistent|must match|must agree|in lockstep|parallel to|duplicates? of|copy of|copied from|cross-checked|single[ -]source(d| of truth)?)\b`)

// wordRe pulls identifier-like words out of a comment line, so a named twin in the
// prose ("mirrors classifyThrough") can be resolved against the corpus.
var wordRe = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

// Confession is a drift-confessing comment found inside a function's source span.
type Confession struct {
	Func     *FuncSig
	Line     int    // absolute source line of the confession
	Phrase   string // the matched phrase (lowercased)
	Text     string // the trimmed source/comment line
	Register string // "line" (dedicated // or # comment — literal twin-flag) or
	// "prose" (docstring body / block-comment narrative — figurative). The reads
	// axis found "mirrors X" means different things by register: in a terse line
	// comment it's a literal maintenance warning (high precision), in a docstring
	// it's usually figurative ("telemetry mirrors the prior sites"). Used to gate
	// the directed candidates and to tag the Layer D matrix variety.
}

// FindConfessions scans every function's source span for drift-confessing comment
// phrases. Files are read once and functions attributed by line span. Best-effort:
// an unreadable file is skipped. Tables/corpus entities (Kind != "") are ignored.
func FindConfessions(sigs []*FuncSig, repo string) []Confession {
	byFile := map[string][]*FuncSig{}
	for _, f := range sigs {
		if f.Kind != "" || f.NLines <= 0 {
			continue
		}
		byFile[f.File] = append(byFile[f.File], f)
	}
	var out []Confession
	for file, fns := range byFile {
		lines := readSourceLines(filepath.Join(repo, file))
		if lines == nil {
			continue
		}
		for _, f := range fns {
			// Scan the doc-comment block directly above the function too — that's
			// where "mirrors X" / "keep in sync" usually live (Go/Rust/TS leading
			// comments sit outside [Line, Line+NLines); Python docstrings are inside).
			for ln := docBlockStart(lines, f.Line); ln < f.Line+f.NLines; ln++ {
				if ln-1 < 0 || ln-1 >= len(lines) {
					continue
				}
				text := lines[ln-1]
				if m := confessRe.FindString(text); m != "" {
					out = append(out, Confession{
						Func: f, Line: ln,
						Phrase:   strings.ToLower(strings.TrimSpace(m)),
						Text:     strings.TrimSpace(text),
						Register: confessionRegister(text),
					})
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Func.File != out[j].Func.File {
			return out[i].Func.File < out[j].Func.File
		}
		return out[i].Line < out[j].Line
	})
	return out
}

// docBlockStart walks up from a function's first line over a contiguous block of
// comment lines, returning the first line of that doc block (or the function line
// itself if none) — so a "mirrors X" doc comment is attributed to the function it
// documents.
func docBlockStart(lines []string, funcLine int) int {
	ln := funcLine - 1
	for ln >= 1 && ln-1 < len(lines) && isCommentLine(lines[ln-1]) {
		ln--
	}
	return ln + 1
}

func isCommentLine(s string) bool {
	t := strings.TrimSpace(s)
	return strings.HasPrefix(t, "//") || strings.HasPrefix(t, "#") ||
		strings.HasPrefix(t, "/*") || strings.HasPrefix(t, "*")
}

func readSourceLines(path string) []string {
	fh, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer fh.Close()
	var lines []string
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines
}

// ConfessionCandidates turns confessions into DIRECTED twin candidates: when a
// confession line names a token that resolves to another function's role-stem, emit
// that pair (the code told us its twin). High precision — the comment IS the recall
// trigger; the judge/human confirms. Confessions whose prose names no resolvable
// function surface only in the `confess` census, not here.
//
// includeProse keeps the figurative "prose" register (docstring bodies / block-comment
// narrative); by default only the literal "line"-comment register is emitted — the
// register discriminator (see confessionRegister): on a mixed corpus the prose register
// was ~all false alarms. Each candidate's register is tagged into Sig as `[line]`/`[prose]`
// so the Layer D matrix slices precision by register.
func ConfessionCandidates(confs []Confession, sigs []*FuncSig, includeProse bool) []SigCandidate {
	// Index by lowercased simple name and by qualname leaf (Type.method → method).
	// Directed pairing fires ONLY on an identifier-like word that exactly NAMES a
	// function — prose words ("drift", "engine", "match") are not matched, keeping
	// precision high. The census backstops recall (a confession whose prose doesn't
	// spell the twin's name still appears there for a human to resolve).
	byName := map[string][]*FuncSig{}
	add := func(k string, f *FuncSig) {
		if k != "" {
			byName[strings.ToLower(k)] = append(byName[strings.ToLower(k)], f)
		}
	}
	for _, f := range sigs {
		if f.Kind != "" {
			continue
		}
		add(f.Name, f)
		if i := strings.LastIndex(f.Qualname, "."); i >= 0 {
			add(f.Qualname[i+1:], f)
		}
	}
	var out []SigCandidate
	seen := map[string]bool{}
	for _, c := range confs {
		if c.Register == "prose" && !includeProse {
			continue // figurative register — gated by default (see confessionRegister)
		}
		for _, word := range wordRe.FindAllString(stripCommentLeader(c.Text), -1) {
			if !identifierLike(word) {
				continue
			}
			matches := byName[strings.ToLower(word)]
			if len(matches) == 0 || len(matches) > 4 {
				continue // unresolved, or too-common a name to be a specific twin
			}
			for _, b := range matches {
				if b.Key() == c.Func.Key() {
					continue
				}
				pk := pairkey.Key(c.Func.Key(), b.Key())
				if seen[pk] {
					continue
				}
				seen[pk] = true
				out = append(out, SigCandidate{
					A: c.Func, B: b, Kind: "confession",
					Sig:       "comment: " + c.Phrase + " [" + c.Register + "]",
					GroupSize: 2, CrossFile: c.Func.File != b.File,
				})
			}
		}
	}
	return out
}

// identifierLike reports whether a comment word looks like a code identifier (has
// an underscore or an internal camelCase boundary) — i.e. someone naming a specific
// function, not a plain English word.
func identifierLike(w string) bool {
	if strings.Contains(w, "_") {
		return true
	}
	for i := 1; i < len(w); i++ {
		if w[i] >= 'A' && w[i] <= 'Z' && w[i-1] >= 'a' && w[i-1] <= 'z' {
			return true
		}
	}
	return false
}

// stripCommentLeader removes common comment leaders so the named-token scan sees prose.
func stripCommentLeader(s string) string {
	s = strings.TrimSpace(s)
	for _, p := range []string{"///", "//!", "//", "#", "/*", "*/", "*"} {
		s = strings.TrimSpace(strings.TrimPrefix(s, p))
	}
	return s
}

// confessionRegister classifies a confession's source line by comment register:
// "line" if it's a dedicated single-line comment (//, ///, //!, #) — the terse
// maintenance register where "mirrors X" is a literal twin-flag; "prose" otherwise
// (a docstring body or block/JSDoc continuation, where "mirrors" is usually the
// figurative English verb). This is the register discriminator: on a mixed corpus
// the prose register's confessions were ~all false alarms while the line register's
// carried the real drift (Layer D matrix, 2026-06-16), so the directed pass keeps
// the line register by default. Substrate-general: every line-comment convention has
// a leader, while docstring prose (Python """…""") is leaderless.
func confessionRegister(text string) string {
	t := strings.TrimSpace(text)
	for _, p := range []string{"///", "//!", "//", "#"} {
		if strings.HasPrefix(t, p) {
			return "line"
		}
	}
	return "prose"
}
