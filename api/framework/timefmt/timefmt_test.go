package timefmt_test

import (
	"testing"
	"time"

	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
)

func TestSQLiteDateTimeFormatsUTC(t *testing.T) {
	local := time.Date(2026, 6, 7, 20, 30, 0, 0, time.FixedZone("CST", 8*60*60))

	if got := timefmt.SQLiteDateTime(local); got != "2026-06-07 12:30:00" {
		t.Fatalf("SQLiteDateTime() = %q, want UTC timestamp", got)
	}
}

func TestRFC3339FormatsUTC(t *testing.T) {
	local := time.Date(2026, 6, 7, 20, 30, 0, 0, time.FixedZone("CST", 8*60*60))

	if got := timefmt.RFC3339(local); got != "2026-06-07T12:30:00Z" {
		t.Fatalf("RFC3339() = %q, want UTC timestamp", got)
	}
}
