package db

import (
	"fmt"
	"os"
	"strings"
)

type Driver string

const (
	DriverSQLite   Driver = "sqlite"
	DriverPostgres Driver = "postgres"
)

const (
	EnvAppDBDriver    = "APP_DB_DRIVER"
	EnvAppDBDSN       = "APP_DB_DSN"
	EnvSharedDBDriver = "SHARED_DB_DRIVER"
	EnvSharedDBDSN    = "SHARED_DB_DSN"
)

type DatabaseSpec struct {
	Name   string
	Driver Driver
	DSN    string
}

type RuntimeDatabases struct {
	App    DatabaseSpec
	Shared DatabaseSpec
}

type RuntimeConfigInput struct {
	AppDriver        string
	AppDSN           string
	SharedDriver     string
	SharedDSN        string
	LegacyAppPath    string
	LegacySharedPath string
}

func LoadRuntimeDatabases(input RuntimeConfigInput) (RuntimeDatabases, error) {
	appDriver := firstNonEmpty(input.AppDriver, os.Getenv(EnvAppDBDriver), string(DriverSQLite))
	appDSN := firstNonEmpty(input.AppDSN, os.Getenv(EnvAppDBDSN), input.LegacyAppPath, "data/app.db")
	sharedDriver := firstNonEmpty(input.SharedDriver, os.Getenv(EnvSharedDBDriver), string(DriverSQLite))
	sharedDSN := firstNonEmpty(input.SharedDSN, os.Getenv(EnvSharedDBDSN), input.LegacySharedPath, "data/shared.db")

	app, err := NewDatabaseSpec("app", appDriver, appDSN)
	if err != nil {
		return RuntimeDatabases{}, err
	}
	shared, err := NewDatabaseSpec("shared", sharedDriver, sharedDSN)
	if err != nil {
		return RuntimeDatabases{}, err
	}
	return RuntimeDatabases{App: app, Shared: shared}, nil
}

func NewDatabaseSpec(name string, driver string, dsn string) (DatabaseSpec, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return DatabaseSpec{}, fmt.Errorf("database name is required")
	}
	parsedDriver, err := ParseDriver(driver)
	if err != nil {
		return DatabaseSpec{}, err
	}
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return DatabaseSpec{}, fmt.Errorf("database %s dsn is required", name)
	}
	return DatabaseSpec{Name: name, Driver: parsedDriver, DSN: dsn}, nil
}

func ParseDriver(value string) (Driver, error) {
	switch Driver(strings.ToLower(strings.TrimSpace(value))) {
	case DriverSQLite:
		return DriverSQLite, nil
	case DriverPostgres:
		return DriverPostgres, nil
	default:
		return "", fmt.Errorf("unsupported database driver: %s", value)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
