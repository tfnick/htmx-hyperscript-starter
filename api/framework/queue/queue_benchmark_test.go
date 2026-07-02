package queue

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/tfnick/go-svelte-starter/api/db"
)

func BenchmarkQueueJSONRunnerImmediateWake(b *testing.B) {
	manager := setupQueueBenchmarkDB(b)
	queueManager, err := NewManager()
	if err != nil {
		b.Fatalf("new queue manager: %v", err)
	}
	_ = manager

	runner, err := queueManager.NewJSONRunner(QueueDomainEvents, 1, time.Hour)
	if err != nil {
		b.Fatalf("new json runner: %v", err)
	}

	handled := make(chan time.Time, 1)
	runner.Register(func(context.Context, []byte) error {
		handled <- time.Now()
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		runner.Start(ctx)
	}()
	b.Cleanup(func() {
		cancel()
		<-done
	})

	latencies := make([]int64, 0, b.N)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		startedAt := time.Now()
		if _, err := queueManager.SendJSON(context.Background(), SendOptions{Queue: QueueDomainEvents}, map[string]int{"i": i}); err != nil {
			b.Fatalf("send json: %v", err)
		}
		handledAt := waitQueueBenchmarkSignal(b, handled)
		latencies = append(latencies, handledAt.Sub(startedAt).Nanoseconds())
	}
	b.StopTimer()

	reportLatencyPercentiles(b, latencies)
	reportQueueBenchmarkRate(b, 1, "messages/s")
}

func BenchmarkQueueNextClaimAt(b *testing.B) {
	for _, rowCount := range []int{0, 1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("rows_%d", rowCount), func(b *testing.B) {
			manager := setupQueueBenchmarkDB(b)
			if rowCount > 0 {
				seedQueueBenchmarkMessages(b, manager, rowCount)
			}

			queueManager, err := NewManager()
			if err != nil {
				b.Fatalf("new queue manager: %v", err)
			}
			runner, err := queueManager.NewJSONRunner(QueueDomainEvents, 1, 10*time.Second)
			if err != nil {
				b.Fatalf("new json runner: %v", err)
			}

			ctx := context.Background()
			b.ReportAllocs()
			b.ReportMetric(float64(rowCount), "seed_rows")
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, _, err := runner.nextClaimAt(ctx); err != nil {
					b.Fatalf("next claim at: %v", err)
				}
			}
			b.StopTimer()
			reportQueueBenchmarkRate(b, 1, "queries/s")
		})
	}
}

func setupQueueBenchmarkDB(b *testing.B) *db.DBManager {
	b.Helper()

	previous := db.DefaultManager
	manager := db.NewDBManager()
	db.DefaultManager = manager
	dir := b.TempDir()
	b.Cleanup(func() {
		_ = manager.Close()
		db.DefaultManager = previous
	})

	if err := manager.Open("app", "sqlite", filepath.Join(dir, "app.db")); err != nil {
		b.Fatalf("open app db: %v", err)
	}
	if err := manager.AutoMigrate("app"); err != nil {
		b.Fatalf("migrate app db: %v", err)
	}
	return manager
}

func seedQueueBenchmarkMessages(b *testing.B, manager *db.DBManager, count int) {
	b.Helper()

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := manager.WithTx(context.Background(), "app", func(ctx context.Context) error {
		eng, err := manager.Engine(ctx, "app")
		if err != nil {
			return err
		}
		for i := 0; i < count; i++ {
			createdAt := base.Add(time.Duration(i) * time.Millisecond).Format(goqiteTimeFormat)
			timeoutAt := base.Add(time.Duration(i%3600) * time.Second).Format(goqiteTimeFormat)
			if _, err := eng.ExecP(`
				INSERT INTO goqite (id, queue, body, created, updated, timeout, received, priority)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			`,
				fmt.Sprintf("bench-message-%06d", i),
				QueueDomainEvents,
				[]byte(`{"benchmark":true}`),
				createdAt,
				createdAt,
				timeoutAt,
				i%DefaultMaxReceive,
				i%10,
			); err != nil {
				return fmt.Errorf("insert queue benchmark message %d: %w", i, err)
			}
		}
		return nil
	}); err != nil {
		b.Fatalf("seed queue benchmark messages: %v", err)
	}
}

func waitQueueBenchmarkSignal(b *testing.B, ch <-chan time.Time) time.Time {
	b.Helper()

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case value := <-ch:
		return value
	case <-timer.C:
		b.Fatal("json runner did not handle message")
		return time.Time{}
	}
}

func reportLatencyPercentiles(b *testing.B, values []int64) {
	b.Helper()

	if len(values) == 0 {
		return
	}
	sort.Slice(values, func(i, j int) bool {
		return values[i] < values[j]
	})
	b.ReportMetric(float64(percentileNanos(values, 50))/float64(time.Microsecond), "p50_us")
	b.ReportMetric(float64(percentileNanos(values, 95))/float64(time.Microsecond), "p95_us")
	b.ReportMetric(float64(percentileNanos(values, 99))/float64(time.Microsecond), "p99_us")
}

func percentileNanos(values []int64, percentile int) int64 {
	if len(values) == 0 {
		return 0
	}
	index := (len(values)*percentile + 99) / 100
	if index <= 0 {
		index = 1
	}
	if index > len(values) {
		index = len(values)
	}
	return values[index-1]
}

func reportQueueBenchmarkRate(b *testing.B, unitsPerOp int, metric string) {
	b.Helper()

	if b.Elapsed() <= 0 || unitsPerOp <= 0 {
		return
	}
	b.ReportMetric(float64(b.N*unitsPerOp)/b.Elapsed().Seconds(), metric)
}
