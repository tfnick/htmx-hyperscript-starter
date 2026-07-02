package timefmt

import "time"

const SQLiteDateTimeLayout = "2006-01-02 15:04:05"

func NowUTC() time.Time {
	return time.Now().UTC()
}

func SQLiteDateTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(SQLiteDateTimeLayout)
}

func NowSQLiteDateTime() string {
	return SQLiteDateTime(NowUTC())
}

func RFC3339(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func RFC3339Nano(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
