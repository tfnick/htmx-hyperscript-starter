package db

import (
	"fmt"
	"regexp"
	"strings"
)

type MigrationDraftOptions struct {
	SourceName string
}

func DraftPostgresMigrationFromSQLite(sql string, opts MigrationDraftOptions) string {
	out := sql
	replacements := []struct {
		old string
		new string
	}{
		{"INSERT OR IGNORE INTO", "INSERT INTO"},
		{"DATETIME", "TIMESTAMPTZ"},
		{"datetime", "timestamptz"},
		{"BLOB", "BYTEA"},
		{"blob", "bytea"},
		{"CURRENT_TIMESTAMP", "now()"},
		{"current_timestamp", "now()"},
		{"INTEGER PRIMARY KEY AUTOINCREMENT", "BIGSERIAL PRIMARY KEY"},
		{"integer primary key autoincrement", "bigserial primary key"},
	}
	for _, replacement := range replacements {
		out = strings.ReplaceAll(out, replacement.old, replacement.new)
	}

	out = regexp.MustCompile(`(?i)\)\s+strict\s*;`).ReplaceAllString(out, ");")
	out = strings.ReplaceAll(out, "strftime('%Y-%m-%dT%H:%M:%fZ')", "now()")

	header := "-- Generated PostgreSQL migration draft. Review before use.\n"
	if source := strings.TrimSpace(opts.SourceName); source != "" {
		header += fmt.Sprintf("-- Source SQLite migration: %s\n", source)
	}
	header += "-- Check constraints, triggers, vector tables, conflict handling, and time/blob semantics manually.\n\n"
	return header + out
}
