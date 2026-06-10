package code

import "fmt"

// symbolExtractors maps an extension to a NON-FUNCTION entity extractor: module-
// level tables (.py, via the embedded extractor's "symbols" mode) and JSON object
// field-sets (.json, pure Go). These feed ONLY the generator-only cross-substrate
// axis (propose-cross) — never the scoring gate — so the function corpus that
// Extract -> Rank -> check -> --strict sees is provably unchanged.
var symbolExtractors = map[string]func(paths []string, root string) ([]*FuncSig, error){
	".py":   extractPySymbols,
	".json": extractJSONBatch,
	".go":   extractGoSymbols,
}

// extractPySymbols extracts module-level TABLES from .py paths (the cross-substrate
// axis) — dict/set/list constants, their key set in RetKeys. Shares the extractor
// process setup with extractPyBatch via runPyExtractor.
func extractPySymbols(paths []string, root string) ([]*FuncSig, error) {
	return runPyExtractor(paths, root, "symbols")
}

// ExtractSymbols walks repo and extracts NON-FUNCTION entities (module-level
// tables, JSON object field-sets) as FuncSig-like records — the cross-substrate
// axis's corpus. Reuses walkExtractable (the same tree walk Extract uses) but
// dispatches to symbolExtractors, and is kept SEPARATE from Extract so the scoring
// gate never sees these entities. Returns Prepared entities + coverage stats
// (Funcs counts entities here).
func ExtractSymbols(repo string, exclude []string) ([]*FuncSig, ScanStats, error) {
	st := ScanStats{SkippedExts: map[string]int{}}
	byExt, err := walkExtractable(repo, exclude,
		func(ext string) bool { _, ok := symbolExtractors[ext]; return ok }, nil)
	if err != nil {
		return nil, st, err
	}
	var all []*FuncSig
	for ext, paths := range byExt {
		sigs, exErr := symbolExtractors[ext](paths, repo)
		if exErr != nil {
			return nil, st, fmt.Errorf("%s symbol extractor: %w", ext, exErr)
		}
		for _, s := range sigs {
			s.Prepare()
		}
		all = append(all, sigs...)
		st.Files += len(paths)
		st.Funcs += len(sigs)
	}

	// SQL schemas — CREATE TABLE in .sql files or embedded in source. A separate
	// pass because a schema can live in a file the per-ext symbol extractors also
	// handle (db.py carries both module dicts AND CREATE TABLE strings); each pass
	// extracts a different entity from it.
	sqlByExt, err := walkExtractable(repo, exclude,
		func(ext string) bool { return sqlBearingExts.has(ext) }, nil)
	if err != nil {
		return nil, st, err
	}
	var sqlPaths []string
	for _, paths := range sqlByExt {
		sqlPaths = append(sqlPaths, paths...)
	}
	schemas, err := extractSQLBatch(sqlPaths, repo)
	if err != nil {
		return nil, st, fmt.Errorf("SQL schema extractor: %w", err)
	}
	for _, s := range schemas {
		s.Prepare()
	}
	all = append(all, schemas...)
	st.Funcs += len(schemas)

	return all, st, nil
}
