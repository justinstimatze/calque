package code

import (
	"os"
	"path/filepath"
	"testing"
)

// Embedded SQL in source (the db.py case, here a .go fixture so the test needs no
// python3): inline `--` comments, a column-level constraint, and a table-level
// PRIMARY KEY line must all resolve to the right column set.
func TestExtractSQLSchemas(t *testing.T) {
	repo := t.TempDir()
	src := "package x\n\nconst schema = `\n" +
		"CREATE TABLE IF NOT EXISTS temporal_markers (\n" +
		"    id          INTEGER PRIMARY KEY,\n" +
		"    source_era  TEXT    NOT NULL,   -- which era bleeds through\n" +
		"    description TEXT,\n" +
		"    trigger     TEXT,\n" +
		"    intensity   INTEGER DEFAULT 1,\n" +
		"    discoverable_fact TEXT,\n" +
		"    PRIMARY KEY (id, source_era)\n" +
		");\n" +
		"`\n"
	if err := os.WriteFile(filepath.Join(repo, "schema.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	ents, _, err := ExtractSymbols(repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	var sch *FuncSig
	for _, e := range ents {
		if e.Kind == "sql-table" && e.Name == "temporal_markers" {
			sch = e
		}
	}
	if sch == nil {
		t.Fatalf("temporal_markers schema not extracted; got %v", ents)
	}
	want := map[string]bool{
		"id": true, "source_era": true, "description": true,
		"trigger": true, "intensity": true, "discoverable_fact": true,
	}
	for _, c := range sch.RetKeys {
		if !want[c] {
			t.Errorf("unexpected column %q (a constraint line leaked?)", c)
		}
		delete(want, c)
	}
	if len(want) != 0 {
		t.Errorf("missing columns: %v (got %v)", want, sch.RetKeys)
	}
}

// A standalone .sql file is scanned too; block comments and nested type parens
// (DECIMAL(10,2)) must not break top-level comma splitting.
func TestExtractSQLStandaloneFile(t *testing.T) {
	repo := t.TempDir()
	sql := "/* migration 007 */\n" +
		"CREATE TABLE accounts (\n" +
		"  account_id   TEXT,\n" +
		"  balance      DECIMAL(10,2),\n" +
		"  opened_at    TIMESTAMP,\n" +
		"  UNIQUE (account_id)\n" +
		");\n"
	if err := os.WriteFile(filepath.Join(repo, "007.sql"), []byte(sql), 0o644); err != nil {
		t.Fatal(err)
	}
	ents, _, err := ExtractSymbols(repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	var sch *FuncSig
	for _, e := range ents {
		if e.Kind == "sql-table" {
			sch = e
		}
	}
	if sch == nil {
		t.Fatal("accounts schema not extracted")
	}
	want := map[string]bool{"account_id": true, "balance": true, "opened_at": true}
	for _, c := range sch.RetKeys {
		if !want[c] {
			t.Errorf("unexpected column %q (UNIQUE leaked or DECIMAL paren split wrong?)", c)
		}
		delete(want, c)
	}
	if len(want) != 0 {
		t.Errorf("missing columns: %v (got %v)", want, sch.RetKeys)
	}
}
