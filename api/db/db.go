package db

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

var DB *sqlx.DB

// InitDB 初始化 SQLite 数据库连接
func InitDB(dataSourceName string) error {
	var err error
	DB, err = sqlx.Open("sqlite", dataSourceName)
	if err != nil {
		return fmt.Errorf("无法打开数据库: %w", err)
	}

	// SQLite 推荐设置
	DB.SetMaxOpenConns(1)
	DB.SetMaxIdleConns(1)

	// SQLite 优化配置
	pragmas := []struct {
		sql  string
		desc string
	}{
		{"PRAGMA foreign_keys = ON", "外键约束"},
		{"PRAGMA journal_mode = WAL", "WAL 模式"},
		{"PRAGMA synchronous = NORMAL", "同步模式"},
		{"PRAGMA cache_size = -64000", "缓存大小 64MB"},
		{"PRAGMA temp_store = MEMORY", "临时表存储在内存"},
	}

	for _, p := range pragmas {
		if _, err := DB.Exec(p.sql); err != nil {
			return fmt.Errorf("设置 %s 失败: %w", p.desc, err)
		}
	}

	// 检查连接
	err = DB.Ping()
	if err != nil {
		return fmt.Errorf("数据库连接失败: %w", err)
	}

	fmt.Println("✅ SQLite 数据库连接成功 (WAL 模式已启用)")
	return nil
}

// CloseDB 关闭数据库连接
func CloseDB() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}

// EnsureDataDir 确保数据目录存在
func EnsureDataDir() error {
	if _, err := os.Stat("data"); os.IsNotExist(err) {
		return os.MkdirAll("data", 0755)
	}
	return nil
}

// AutoMigrate 从 SQL 文件执行数据库迁移（带版本控制，防止重复执行）
func AutoMigrate() error {
	// 1. 创建版本记录表（如果不存在）
	_, err := DB.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name       TEXT PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("创建迁移记录表失败: %w", err)
	}

	// 2. 获取已执行的迁移
	var applied []string
	if err := DB.Select(&applied, "SELECT name FROM schema_migrations"); err != nil {
		return fmt.Errorf("查询已执行迁移失败: %w", err)
	}
	appliedMap := make(map[string]bool)
	for _, v := range applied {
		appliedMap[v] = true
	}

	// 3. 获取迁移文件列表
	files, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("读取迁移目录失败: %w", err)
	}

	// 按文件名排序确保执行顺序
	var sortedFiles []string
	for _, f := range files {
		if !f.IsDir() && filepath.Ext(f.Name()) == ".sql" {
			sortedFiles = append(sortedFiles, f.Name())
		}
	}
	sort.Strings(sortedFiles)

	// 4. 只执行未应用的迁移
	migrated := false
	for _, filename := range sortedFiles {
		if appliedMap[filename] {
			fmt.Printf("⏭️  跳过已执行: %s\n", filename)
			continue
		}

		sqlContent, err := migrationsFS.ReadFile("migrations/" + filename)
		if err != nil {
			return fmt.Errorf("读取迁移文件 %s 失败: %w", filename, err)
		}

		if _, err := DB.Exec(string(sqlContent)); err != nil {
			return fmt.Errorf("执行迁移 %s 失败: %w", filename, err)
		}

		// 记录本次迁移
		if _, err := DB.Exec("INSERT INTO schema_migrations (name) VALUES (?)", filename); err != nil {
			return fmt.Errorf("记录迁移 %s 失败: %w", filename, err)
		}

		fmt.Printf("✅ 执行迁移: %s\n", filename)
		migrated = true
	}

	if migrated {
		fmt.Println("✅ 数据库迁移完成")
	} else {
		fmt.Println("✅ 数据库已是最新版本")
	}
	return nil
}

// RunMigrationsFromFile 从指定目录执行迁移（用于开发调试，带版本控制）
func RunMigrationsFromFile(dir string) error {
	// 1. 创建版本记录表（如果不存在）
	_, err := DB.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name       TEXT PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("创建迁移记录表失败: %w", err)
	}

	// 2. 获取已执行的迁移
	var applied []string
	if err := DB.Select(&applied, "SELECT name FROM schema_migrations"); err != nil {
		return fmt.Errorf("查询已执行迁移失败: %w", err)
	}
	appliedMap := make(map[string]bool)
	for _, v := range applied {
		appliedMap[v] = true
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("读取迁移目录失败: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".sql" {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	// 3. 只执行未应用的迁移
	for _, filename := range files {
		if appliedMap[filename] {
			fmt.Printf("⏭️  跳过已执行: %s\n", filename)
			continue
		}

		content, err := os.ReadFile(filepath.Join(dir, filename))
		if err != nil {
			return fmt.Errorf("读取文件失败: %w", err)
		}

		if _, err := DB.Exec(string(content)); err != nil {
			return fmt.Errorf("执行迁移失败: %w", err)
		}

		// 记录本次迁移
		if _, err := DB.Exec("INSERT INTO schema_migrations (name) VALUES (?)", filename); err != nil {
			return fmt.Errorf("记录迁移 %s 失败: %w", filename, err)
		}

		fmt.Printf("✅ 执行迁移: %s\n", filename)
	}

	return nil
}

// Transaction 事务执行函数类型
type Transaction func(*sqlx.Tx) error

// WithTransaction 封装事务处理，自动处理提交/回滚
func WithTransaction(fn Transaction) error {
	tx, err := DB.Beginx()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("事务回滚失败 (原始错误: %v): %w", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("事务提交失败: %w", err)
	}

	return nil
}

// GetDB 返回数据库实例
func GetDB() *sqlx.DB {
	return DB
}

// SqlxDB interface for dependency injection
type SqlxDB interface {
	BindNamed(query string, arg interface{}) (string, []interface{}, error)
	Close() error
	DBStats() sql.DBStats
	DriverName() string
	Exec(query string, args ...interface{}) (sql.Result, error)
	Get(dest interface{}, query string, args ...interface{}) error
	MustBegin() *sqlx.Tx
	MustExec(query string, args ...interface{}) sql.Result
	Ping() error
	Prepare(query string) (*sql.Stmt, error)
	Preparex(query string) (*sqlx.Stmt, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Queryx(query string, args ...interface{}) (*sqlx.Rows, error)
	Rebind(query string) string
	Select(dest interface{}, query string, args ...interface{}) error
	SetMaxIdleConns(n int)
	SetMaxOpenConns(n int)
}
