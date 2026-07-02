package db

import "testing"

func TestLoadRuntimeDatabasesDefaultsToSQLite(t *testing.T) {
	t.Setenv(EnvAppDBDriver, "")
	t.Setenv(EnvAppDBDSN, "")
	t.Setenv(EnvSharedDBDriver, "")
	t.Setenv(EnvSharedDBDSN, "")

	got, err := LoadRuntimeDatabases(RuntimeConfigInput{
		LegacyAppPath:    "data/custom-app.db",
		LegacySharedPath: "data/custom-shared.db",
	})
	if err != nil {
		t.Fatalf("LoadRuntimeDatabases() error = %v", err)
	}
	if got.App.Driver != DriverSQLite || got.App.DSN != "data/custom-app.db" {
		t.Fatalf("unexpected app database config: %#v", got.App)
	}
	if got.Shared.Driver != DriverSQLite || got.Shared.DSN != "data/custom-shared.db" {
		t.Fatalf("unexpected shared database config: %#v", got.Shared)
	}
}

func TestLoadRuntimeDatabasesKeepsSharedSQLiteWhenAppUsesPostgres(t *testing.T) {
	t.Setenv(EnvAppDBDriver, "postgres")
	t.Setenv(EnvAppDBDSN, "postgres://app")
	t.Setenv(EnvSharedDBDriver, "")
	t.Setenv(EnvSharedDBDSN, "")

	got, err := LoadRuntimeDatabases(RuntimeConfigInput{
		LegacySharedPath: "data/shared.db",
	})
	if err != nil {
		t.Fatalf("LoadRuntimeDatabases() error = %v", err)
	}
	if got.App.Driver != DriverPostgres || got.App.DSN != "postgres://app" {
		t.Fatalf("unexpected app database config: %#v", got.App)
	}
	if got.Shared.Driver != DriverSQLite || got.Shared.DSN != "data/shared.db" {
		t.Fatalf("unexpected shared database config: %#v", got.Shared)
	}
}

func TestLoadRuntimeDatabasesAllowsExplicitSharedPostgres(t *testing.T) {
	t.Setenv(EnvAppDBDriver, "postgres")
	t.Setenv(EnvAppDBDSN, "postgres://app")
	t.Setenv(EnvSharedDBDriver, "postgres")
	t.Setenv(EnvSharedDBDSN, "postgres://shared")

	got, err := LoadRuntimeDatabases(RuntimeConfigInput{})
	if err != nil {
		t.Fatalf("LoadRuntimeDatabases() error = %v", err)
	}
	if got.Shared.Driver != DriverPostgres || got.Shared.DSN != "postgres://shared" {
		t.Fatalf("unexpected shared database config: %#v", got.Shared)
	}
}

func TestMigrationDirUsesPostgresLayoutOnlyForPostgres(t *testing.T) {
	if got := migrationDir(DriverSQLite, "app"); got != "migrations/sqlite/app" {
		t.Fatalf("sqlite migrationDir() = %q", got)
	}
	if got := migrationDir(DriverPostgres, "app"); got != "migrations/postgres/app" {
		t.Fatalf("postgres migrationDir() = %q", got)
	}
}
