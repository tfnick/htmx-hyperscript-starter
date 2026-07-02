package usecase_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/queue"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

func BenchmarkScheduledDueScanAndEnqueue(b *testing.B) {
	for _, taskCount := range []int{100, 1_000, 10_000} {
		b.Run(fmt.Sprintf("tasks_%d", taskCount), func(b *testing.B) {
			manager := setupUsecaseOrderTxDB(b)
			queueManager, err := queue.NewManager()
			if err != nil {
				b.Fatalf("new queue manager: %v", err)
			}
			previousQueue := usecase.DefaultQueueManager
			usecase.DefaultQueueManager = queueManager
			b.Cleanup(func() {
				usecase.DefaultQueueManager = previousQueue
			})

			now := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
			seedScheduledBenchmarkTasks(b, manager, taskCount, now.Add(-time.Minute))
			ctx := fwusecase.NewContext(context.Background(), fwusecase.SurfaceSystem)

			b.ReportAllocs()
			b.ReportMetric(float64(taskCount), "tasks/op")
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				resetScheduledBenchmarkDueState(b, manager, now.Add(-time.Minute))
				b.StartTimer()

				enqueued, err := usecase.EnqueueDueScheduledTasks(ctx, usecase.EnqueueDueScheduledTasksCmd{Now: now})
				if err != nil {
					b.Fatalf("enqueue due scheduled tasks: %v", err)
				}
				if enqueued != taskCount {
					b.Fatalf("enqueued %d tasks, want %d", enqueued, taskCount)
				}
			}
			b.StopTimer()
			reportScheduledBenchmarkRate(b, taskCount)
		})
	}
}

func seedScheduledBenchmarkTasks(b *testing.B, manager *db.DBManager, count int, dueAt time.Time) {
	b.Helper()

	if err := manager.WithTx(context.Background(), "app", func(ctx context.Context) error {
		eng, err := manager.Engine(ctx, "app")
		if err != nil {
			return err
		}
		for i := 0; i < count; i++ {
			if _, err := eng.ExecP(`
				INSERT INTO scheduled_tasks (
					id, name, job_name, schedule_type, schedule_value, payload_json,
					enabled, next_run_at, created_at, updated_at
				) VALUES (?, ?, ?, 'once_at', ?, '{}', 1, ?, ?, ?)
			`,
				scheduledBenchmarkTaskID(i),
				fmt.Sprintf("Benchmark task %06d", i),
				usecase.BuiltInScheduledTaskJob,
				dueAt.Format(time.RFC3339),
				dueAt.Format(time.RFC3339),
				dueAt.Format(time.RFC3339),
				dueAt.Format(time.RFC3339),
			); err != nil {
				return fmt.Errorf("insert scheduled benchmark task %d: %w", i, err)
			}
		}
		return nil
	}); err != nil {
		b.Fatalf("seed scheduled benchmark tasks: %v", err)
	}
}

func resetScheduledBenchmarkDueState(b *testing.B, manager *db.DBManager, dueAt time.Time) {
	b.Helper()

	eng, err := manager.GetEngine("app")
	if err != nil {
		b.Fatalf("get app engine: %v", err)
	}
	if _, err := eng.ExecP(`DELETE FROM scheduled_task_executions`); err != nil {
		b.Fatalf("clear scheduled task executions: %v", err)
	}
	if _, err := eng.ExecP(`DELETE FROM goqite WHERE queue = ?`, queue.QueueScheduledTasks); err != nil {
		b.Fatalf("clear scheduled queue messages: %v", err)
	}
	if _, err := eng.ExecP(`
		UPDATE scheduled_tasks
		SET enabled = 1, next_run_at = ?, last_run_at = NULL, claim_token = '', claim_expires_at = NULL
	`, dueAt.Format(time.RFC3339)); err != nil {
		b.Fatalf("reset scheduled tasks due state: %v", err)
	}
}

func scheduledBenchmarkTaskID(i int) string {
	return fmt.Sprintf("bench-scheduled-task-%06d", i)
}

func reportScheduledBenchmarkRate(b *testing.B, tasksPerOp int) {
	b.Helper()

	if b.Elapsed() <= 0 || tasksPerOp <= 0 {
		return
	}
	b.ReportMetric(float64(b.N*tasksPerOp)/b.Elapsed().Seconds(), "tasks/s")
}
