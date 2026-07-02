package db_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	appdb "github.com/tfnick/go-svelte-starter/api/db"
)

const benchmarkRowCount = 10000

type benchmarkUserRow struct {
	ID     string `db:"id"`
	Email  string `db:"email"`
	Name   string `db:"name"`
	Status string `db:"status"`
}

type benchmarkOrderUserRow struct {
	OrderID string `db:"order_id"`
	UserID  string `db:"user_id"`
	Email   string `db:"email"`
	Amount  int    `db:"amount"`
	Status  string `db:"status"`
}

func BenchmarkDBLayerSingleTableGet(b *testing.B) {
	manager := setupBenchmarkDB(b)
	ctx := context.Background()
	ids := benchmarkIDs("user", benchmarkRowCount)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := benchmarkSingleTableGet(ctx, ids[i%len(ids)]); err != nil {
			b.Fatalf("single-table get: %v", err)
		}
	}
	b.StopTimer()
	reportBenchmarkQPS(b)
	_ = manager.Close()
}

func BenchmarkDBLayerTwoTableJoinGet(b *testing.B) {
	manager := setupBenchmarkDB(b)
	ctx := context.Background()
	orderIDs := benchmarkIDs("order", benchmarkRowCount)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := benchmarkTwoTableJoinGet(ctx, orderIDs[i%len(orderIDs)]); err != nil {
			b.Fatalf("two-table join get: %v", err)
		}
	}
	b.StopTimer()
	reportBenchmarkQPS(b)
	_ = manager.Close()
}

func benchmarkSingleTableGet(ctx context.Context, id string) (benchmarkUserRow, error) {
	eng, err := appdb.EngineFor(ctx, "app")
	if err != nil {
		return benchmarkUserRow{}, err
	}
	var row benchmarkUserRow
	if err := eng.GetP(&row, `
		SELECT id, email, name, status
		FROM bench_users
		WHERE id = ?
	`, id); err != nil {
		return benchmarkUserRow{}, err
	}
	return row, nil
}

func benchmarkTwoTableJoinGet(ctx context.Context, orderID string) (benchmarkOrderUserRow, error) {
	eng, err := appdb.EngineFor(ctx, "app")
	if err != nil {
		return benchmarkOrderUserRow{}, err
	}
	var row benchmarkOrderUserRow
	if err := eng.GetP(&row, `
		SELECT o.id AS order_id, o.user_id, u.email, o.amount, o.status
		FROM bench_orders o
		INNER JOIN bench_users u ON u.id = o.user_id
		WHERE o.id = ?
	`, orderID); err != nil {
		return benchmarkOrderUserRow{}, err
	}
	return row, nil
}

func setupBenchmarkDB(b *testing.B) *appdb.DBManager {
	b.Helper()
	previous := appdb.DefaultManager
	manager := appdb.NewDBManager()
	appdb.DefaultManager = manager
	b.Cleanup(func() {
		_ = manager.Close()
		appdb.DefaultManager = previous
	})

	dbPath := filepath.Join(b.TempDir(), "bench.db")
	if err := manager.Open("app", "sqlite", dbPath); err != nil {
		b.Fatalf("open benchmark db: %v", err)
	}
	eng, err := manager.GetEngine("app")
	if err != nil {
		b.Fatalf("get benchmark engine: %v", err)
	}
	if _, err := eng.ExecP(`
		CREATE TABLE bench_users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL,
			name TEXT NOT NULL,
			status TEXT NOT NULL
		);
		CREATE INDEX idx_bench_users_status ON bench_users(status);
		CREATE TABLE bench_orders (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			status TEXT NOT NULL,
			amount INTEGER NOT NULL,
			FOREIGN KEY (user_id) REFERENCES bench_users(id)
		);
		CREATE INDEX idx_bench_orders_user_id ON bench_orders(user_id);
		CREATE INDEX idx_bench_orders_status ON bench_orders(status);
	`); err != nil {
		b.Fatalf("create benchmark schema: %v", err)
	}

	err = manager.WithTx(context.Background(), "app", func(ctx context.Context) error {
		eng, err := manager.Engine(ctx, "app")
		if err != nil {
			return err
		}
		for i := 0; i < benchmarkRowCount; i++ {
			userID := benchmarkID("user", i)
			if _, err := eng.ExecP(`INSERT INTO bench_users (id, email, name, status) VALUES (?, ?, ?, ?)`,
				userID, fmt.Sprintf("user-%05d@example.com", i), fmt.Sprintf("User %05d", i), benchmarkStatus(i),
			); err != nil {
				return fmt.Errorf("seed user %d: %w", i, err)
			}
			if _, err := eng.ExecP(`INSERT INTO bench_orders (id, user_id, status, amount) VALUES (?, ?, ?, ?)`,
				benchmarkID("order", i), userID, benchmarkStatus(i), i*10,
			); err != nil {
				return fmt.Errorf("seed order %d: %w", i, err)
			}
		}
		return nil
	})
	if err != nil {
		b.Fatalf("seed benchmark data: %v", err)
	}
	return manager
}

func benchmarkIDs(prefix string, count int) []string {
	ids := make([]string, count)
	for i := 0; i < count; i++ {
		ids[i] = benchmarkID(prefix, i)
	}
	return ids
}

func benchmarkID(prefix string, i int) string {
	return fmt.Sprintf("%s-%05d", prefix, i)
}

func benchmarkStatus(i int) string {
	if i%2 == 0 {
		return "active"
	}
	return "inactive"
}

func reportBenchmarkQPS(b *testing.B) {
	if b.Elapsed() <= 0 {
		return
	}
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "qps")
}
