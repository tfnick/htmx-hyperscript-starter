package db

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	_ "github.com/lib/pq"
	"github.com/tfnick/go-svelte-starter/api/framework/logging"
	"github.com/tfnick/sqlx"
	_ "modernc.org/sqlite"
	_ "modernc.org/sqlite/vec"
)

//go:embed migrations
var migrationsFS embed.FS

type namedDB struct {
	db     *sqlx.DB
	engine *sqlx.Engine
	driver Driver
	dsn    string
}

type DBManager struct {
	databases map[string]*namedDB
	mu        sync.RWMutex
}

var logger = logging.For("db")

func NewDBManager() *DBManager {
	// sqlx v1.4.4 enables global SQL stdout logging by default.
	// Keep the project's existing low-noise behavior unless we opt in explicitly.
	sqlx.Log.Enabled = false

	return &DBManager{
		databases: make(map[string]*namedDB),
	}
}

func sqlitePragmas() []struct{ sql, desc string } {
	return []struct{ sql, desc string }{
		{"PRAGMA foreign_keys = ON", "foreign key constraints"},
		{"PRAGMA journal_mode = WAL", "WAL mode"},
		{"PRAGMA synchronous = NORMAL", "sync mode"},
		{"PRAGMA cache_size = -64000", "64MB cache"},
		{"PRAGMA temp_store = MEMORY", "memory temp store"},
	}
}

func applyConfig(d *sqlx.DB, driver Driver) error {
	switch driver {
	case DriverSQLite:
		d.SetMaxOpenConns(1)
		d.SetMaxIdleConns(1)
		for _, p := range sqlitePragmas() {
			if _, err := d.StdDB().Exec(p.sql); err != nil {
				return fmt.Errorf("set %s failed: %w", p.desc, err)
			}
		}
	case DriverPostgres:
		d.SetMaxOpenConns(25)
		d.SetMaxIdleConns(5)
	default:
		return fmt.Errorf("unsupported database driver: %s", driver)
	}
	return nil
}

func openConfiguredDB(spec DatabaseSpec) (*sqlx.DB, error) {
	db, err := sqlx.Open(string(spec.Driver), spec.DSN)
	if err != nil {
		return nil, fmt.Errorf("open database %s failed: %w", spec.Name, err)
	}

	if err := applyConfig(db, spec.Driver); err != nil {
		db.Close()
		return nil, fmt.Errorf("configure database %s failed: %w", spec.Name, err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database %s failed: %w", spec.Name, err)
	}

	return db, nil
}

func (m *DBManager) Open(name, driver, path string) error {
	spec, err := NewDatabaseSpec(name, driver, path)
	if err != nil {
		return err
	}
	return m.OpenSpec(spec)
}

func (m *DBManager) OpenSpec(spec DatabaseSpec) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.databases[spec.Name]; exists {
		return fmt.Errorf("database already registered: %s", spec.Name)
	}

	db, err := openConfiguredDB(spec)
	if err != nil {
		return err
	}

	m.databases[spec.Name] = &namedDB{
		db:     db,
		engine: sqlx.NewEngine(db),
		driver: spec.Driver,
		dsn:    spec.DSN,
	}

	logger.Info().Str("database", spec.Name).Str("driver", string(spec.Driver)).Msg("database connected")
	return nil
}

func (m *DBManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for name, ndb := range m.databases {
		if err := ndb.db.Close(); err != nil {
			lastErr = fmt.Errorf("close database %s failed: %w", name, err)
		}
	}

	m.databases = make(map[string]*namedDB)
	return lastErr
}

func (m *DBManager) GetDB(name string) (*sqlx.DB, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ndb, ok := m.databases[name]
	if !ok {
		return nil, fmt.Errorf("database not found: %s", name)
	}
	return ndb.db, nil
}

func (m *DBManager) Driver(name string) (Driver, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ndb, ok := m.databases[name]
	if !ok {
		return "", fmt.Errorf("database not found: %s", name)
	}
	return ndb.driver, nil
}

func (m *DBManager) GetEngine(name string) (*sqlx.Engine, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ndb, ok := m.databases[name]
	if !ok {
		return nil, fmt.Errorf("database not found: %s", name)
	}
	return ndb.engine, nil
}

func (m *DBManager) AutoMigrate(name string) error {
	db, err := m.GetDB(name)
	if err != nil {
		return err
	}
	engine, err := m.GetEngine(name)
	if err != nil {
		return err
	}

	driver, err := m.Driver(name)
	if err != nil {
		return err
	}

	if name != "app" && name != "shared" {
		return fmt.Errorf("unknown database: %s", name)
	}

	_, err = engine.ExecP(schemaMigrationsSQL(driver))
	if err != nil {
		return fmt.Errorf("create schema_migrations failed: %w", err)
	}

	var applied []string
	if err := engine.SelectP(&applied, "SELECT name FROM schema_migrations"); err != nil {
		return fmt.Errorf("query applied migrations failed: %w", err)
	}

	appliedMap := make(map[string]bool, len(applied))
	for _, v := range applied {
		appliedMap[v] = true
	}

	dir := migrationDir(driver, name)
	files, err := migrationsFS.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations dir %s failed: %w", dir, err)
	}
	if err := validateMigrationSet(driver, name, files); err != nil {
		return err
	}

	var sortedFiles []string
	for _, f := range files {
		if !f.IsDir() && filepath.Ext(f.Name()) == ".sql" {
			sortedFiles = append(sortedFiles, f.Name())
		}
	}
	sort.Strings(sortedFiles)

	migrated := false
	for _, filename := range sortedFiles {
		if appliedMap[filename] {
			logger.Info().Str("database", name).Str("migration", filename).Msg("skip applied migration")
			continue
		}

		sqlContent, err := migrationsFS.ReadFile(dir + "/" + filename)
		if err != nil {
			return fmt.Errorf("read migration %s failed: %w", filename, err)
		}

		// Use the standard-library DB for migration files because sqlx v1.4.4's
		// DB.Exec wrapper may call RowsAffected on driver results that do not
		// support it for DDL / migration-style statements.
		if _, err := db.StdDB().Exec(string(sqlContent)); err != nil {
			return fmt.Errorf("execute migration %s failed: %w", filename, err)
		}

		if _, err := engine.ExecP(`INSERT INTO schema_migrations (name) VALUES (?)`, filename); err != nil {
			return fmt.Errorf("record migration %s failed: %w", filename, err)
		}

		logger.Info().Str("database", name).Str("migration", filename).Msg("applied migration")
		migrated = true
	}

	if migrated {
		logger.Info().Str("database", name).Msg("database migrations complete")
	} else {
		logger.Info().Str("database", name).Msg("database already up to date")
	}

	return nil
}

func schemaMigrationsSQL(driver Driver) string {
	if driver == DriverPostgres {
		return `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name       TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ DEFAULT now()
		)
	`
	}
	return `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name       TEXT PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`
}

func migrationDir(driver Driver, name string) string {
	switch driver {
	case DriverPostgres:
		return "migrations/postgres/" + name
	case DriverSQLite:
		return "migrations/sqlite/" + name
	default:
		return "migrations/" + string(driver) + "/" + name
	}
}

func validateMigrationSet(driver Driver, name string, files []os.DirEntry) error {
	if driver != DriverPostgres {
		return nil
	}
	for _, file := range files {
		if !file.IsDir() && file.Name() == "001_schema.sql" {
			return nil
		}
	}
	return fmt.Errorf("postgres migrations for %s are not ready: missing 001_schema.sql", name)
}

func (m *DBManager) Reopen(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ndb, ok := m.databases[name]
	if !ok {
		return fmt.Errorf("database not found: %s", name)
	}

	db, err := openConfiguredDB(DatabaseSpec{Name: name, Driver: ndb.driver, DSN: ndb.dsn})
	if err != nil {
		return err
	}

	oldDB := ndb.db
	ndb.db = db
	ndb.engine = sqlx.NewEngine(db)
	if oldDB != nil {
		if err := oldDB.Close(); err != nil {
			ndb.db = oldDB
			ndb.engine = sqlx.NewEngine(oldDB)
			db.Close()
			return fmt.Errorf("close old database connection failed: %w", err)
		}
	}

	logger.Info().Str("database", name).Msg("database reloaded")
	return nil
}

func EnsureDataDir() error {
	if _, err := os.Stat("data"); os.IsNotExist(err) {
		return os.MkdirAll("data", 0755)
	}
	return nil
}

var DefaultManager *DBManager

func GetDB(name string) (*sqlx.DB, error) {
	return DefaultManager.GetDB(name)
}

func GetEngine(name string) (*sqlx.Engine, error) {
	return DefaultManager.GetEngine(name)
}

func DriverFor(name string) (Driver, error) {
	return DefaultManager.Driver(name)
}

func AutoMigrateDB(name string) error {
	return DefaultManager.AutoMigrate(name)
}

func ReopenDB(name string) error {
	return DefaultManager.Reopen(name)
}
