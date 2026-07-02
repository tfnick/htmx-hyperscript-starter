package db

import (
	"strings"
	"testing"
)

func TestDraftPostgresMigrationFromSQLiteConvertsCommonSyntax(t *testing.T) {
	input := `
CREATE TABLE example (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  body BLOB NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
) STRICT;

INSERT OR IGNORE INTO example (body) VALUES ('x');
`

	got := DraftPostgresMigrationFromSQLite(input, MigrationDraftOptions{SourceName: "001_example.sql"})
	for _, want := range []string{
		"Generated PostgreSQL migration draft",
		"Source SQLite migration: 001_example.sql",
		"id BIGSERIAL PRIMARY KEY",
		"body BYTEA NOT NULL",
		"created_at TIMESTAMPTZ DEFAULT now()",
		")",
		"INSERT INTO example",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("draft missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(strings.ToUpper(got), "STRICT;") {
		t.Fatalf("draft should remove SQLite STRICT table option:\n%s", got)
	}
}
