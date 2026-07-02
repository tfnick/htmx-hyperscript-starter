package usecase_test

import (
	"testing"
	"time"

	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/queue"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

func TestGetQueueSummaryClassifiesMessageStates(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}

	now := time.Now().UTC()
	seedUsecaseQueueMessage(t, "ready-msg", queue.QueueHeavyTasks, now.Add(-10*time.Minute), now.Add(-1*time.Minute), 0)
	seedUsecaseQueueMessage(t, "delayed-msg", queue.QueueHeavyTasks, now.Add(-5*time.Minute), now.Add(5*time.Minute), 0)
	seedUsecaseQueueMessage(t, "inflight-msg", queue.QueueHeavyTasks, now.Add(-4*time.Minute), now.Add(4*time.Minute), 1)
	seedUsecaseQueueMessage(t, "dead-msg", queue.QueueHeavyTasks, now.Add(-3*time.Minute), now.Add(-2*time.Minute), queue.DefaultMaxReceive)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceSystem)
	summary, err := usecase.GetQueueSummary(ctx, usecase.QueueSummaryQry{Queue: queue.QueueHeavyTasks})
	if err != nil {
		t.Fatalf("get queue summary: %v", err)
	}
	if len(summary.Queues) != 1 {
		t.Fatalf("expected one queue summary, got %#v", summary.Queues)
	}
	item := summary.Queues[0]
	if item.Queue != queue.QueueHeavyTasks || item.Total != 4 || item.Ready != 1 || item.Delayed != 1 || item.InFlight != 1 || item.Exhausted != 1 {
		t.Fatalf("unexpected queue summary: %#v", item)
	}
	if item.Retrying != 1 {
		t.Fatalf("expected one retrying message, got %#v", item)
	}
	if item.MaxReceived != queue.DefaultMaxReceive {
		t.Fatalf("expected max received %d, got %d", queue.DefaultMaxReceive, item.MaxReceived)
	}
	if item.OldestReadyAgeSec <= 0 || item.OldestInFlightAgeSec <= 0 || item.OldestExhaustedAgeSec <= 0 {
		t.Fatalf("expected positive age metrics, got %#v", item)
	}
	if item.NextClaimAt == "" || item.NextClaimInSec <= 0 {
		t.Fatalf("expected next claim metrics, got %#v", item)
	}

	var count int
	if err := appDB.GetP(&count, `SELECT COUNT(*) FROM goqite WHERE queue = ?`, queue.QueueHeavyTasks); err != nil {
		t.Fatalf("count queue messages: %v", err)
	}
	if count != 4 {
		t.Fatalf("expected seeded queue rows to remain, got %d", count)
	}
}

func TestListMessagesAddsOperationalStateAndFilters(t *testing.T) {
	setupUsecaseOrderTxDB(t)

	now := time.Now().UTC()
	seedUsecaseQueueMessage(t, "list-ready-msg", queue.QueueHeavyTasks, now.Add(-10*time.Minute), now.Add(-1*time.Minute), 0)
	seedUsecaseQueueMessage(t, "list-delayed-msg", queue.QueueHeavyTasks, now.Add(-5*time.Minute), now.Add(5*time.Minute), 0)
	seedUsecaseQueueMessage(t, "list-inflight-msg", queue.QueueHeavyTasks, now.Add(-4*time.Minute), now.Add(4*time.Minute), 1)
	seedUsecaseQueueMessage(t, "list-dead-msg", queue.QueueHeavyTasks, now.Add(-3*time.Minute), now.Add(-2*time.Minute), queue.DefaultMaxReceive)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceSystem)
	messages, err := usecase.ListMessages(ctx, usecase.ListMessagesQry{Queue: queue.QueueHeavyTasks})
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 4 {
		t.Fatalf("expected four messages, got %#v", messages)
	}
	assertQueueMessageState(t, messages, "list-ready-msg", usecase.QueueMessageStateReady, queue.DefaultMaxReceive, "")
	assertQueueMessageState(t, messages, "list-delayed-msg", usecase.QueueMessageStateDelayed, queue.DefaultMaxReceive, "future")
	assertQueueMessageState(t, messages, "list-inflight-msg", usecase.QueueMessageStateInFlight, queue.DefaultMaxReceive, "future")
	assertQueueMessageState(t, messages, "list-dead-msg", usecase.QueueMessageStateExhausted, queue.DefaultMaxReceive, "")

	exhausted, err := usecase.ListMessages(ctx, usecase.ListMessagesQry{
		Queue: queue.QueueHeavyTasks,
		State: usecase.QueueMessageStateExhausted,
	})
	if err != nil {
		t.Fatalf("list exhausted messages: %v", err)
	}
	if len(exhausted) != 1 || exhausted[0].ID != "list-dead-msg" {
		t.Fatalf("expected only exhausted message, got %#v", exhausted)
	}

	if _, err := usecase.ListMessages(ctx, usecase.ListMessagesQry{State: "unknown"}); fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected validation error for invalid state, got %v", err)
	}
}

func TestGetQueueMessageCorrelationFindsProjectReferences(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}

	messageID := "corr-message"
	if _, err := appDB.ExecP(`
		INSERT INTO domain_events (
			id, topic, aggregate_type, aggregate_id, payload_json, metadata_json, occurred_at, created_at
		) VALUES ('corr-event', 'order.paid', 'order', 'corr-order', '{}', '{}', '2026-01-01T00:00:00Z', '2026-01-01 00:00:00')
	`); err != nil {
		t.Fatalf("insert domain event: %v", err)
	}
	if _, err := appDB.ExecP(`
		INSERT INTO domain_event_deliveries (
			id, event_id, subscriber, message_id, status
		) VALUES ('corr-delivery', 'corr-event', 'corr.subscriber', ?, 'queued')
	`, messageID); err != nil {
		t.Fatalf("insert domain event delivery: %v", err)
	}
	if _, err := appDB.ExecP(`
		INSERT INTO async_tasks (
			id, user_id, task_type, status, payload_json, message_id
		) VALUES ('corr-task', 'u1', 'test_export', 'queued', '{}', ?)
	`, messageID); err != nil {
		t.Fatalf("insert async task: %v", err)
	}
	if _, err := appDB.ExecP(`
		INSERT INTO scheduled_tasks (
			id, name, job_name, schedule_type, schedule_value, payload_json, enabled, created_at, updated_at
		) VALUES ('corr-scheduled', 'Correlation task', 'scheduler.noop', 'cron', '*/5 * * * *', '{}', 1, '2026-01-01 00:00:00', '2026-01-01 00:00:00')
	`); err != nil {
		t.Fatalf("insert scheduled task: %v", err)
	}
	if _, err := appDB.ExecP(`
		INSERT INTO scheduled_task_executions (
			id, task_id, job_name, message_id, status, scheduled_at
		) VALUES ('corr-execution', 'corr-scheduled', 'scheduler.noop', ?, 'queued', '2026-01-01 00:00:00')
	`, messageID); err != nil {
		t.Fatalf("insert scheduled execution: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceSystem)
	correlation, err := usecase.GetQueueMessageCorrelation(ctx, usecase.QueueMessageCorrelationQry{MessageID: messageID})
	if err != nil {
		t.Fatalf("get queue message correlation: %v", err)
	}
	if len(correlation.References) != 3 {
		t.Fatalf("expected three references, got %#v", correlation.References)
	}
	assertUsecaseReference(t, correlation.References, "async_task", "corr-task", "queued", "task_type", "test_export")
	assertUsecaseReference(t, correlation.References, "domain_event_delivery", "corr-delivery", "queued", "domain_event", "corr-event")
	assertUsecaseReference(t, correlation.References, "scheduled_task_execution", "corr-execution", "queued", "scheduled_task", "corr-scheduled")
}

func TestRetryQueueMessageNowRetriesOnlyExhaustedMessages(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	queueManager, err := queue.NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}
	previousQueue := usecase.DefaultQueueManager
	usecase.DefaultQueueManager = queueManager
	t.Cleanup(func() {
		usecase.DefaultQueueManager = previousQueue
	})

	now := time.Now().UTC()
	seedUsecaseQueueMessage(t, "retry-usecase-dead", queue.QueueHeavyTasks, now.Add(-10*time.Minute), now.Add(-1*time.Minute), queue.DefaultMaxReceive)
	seedUsecaseQueueMessage(t, "retry-usecase-ready", queue.QueueHeavyTasks, now.Add(-10*time.Minute), now.Add(-1*time.Minute), 0)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceSystem)
	if err := usecase.RetryQueueMessageNow(ctx, usecase.QueueMessageActionCmd{
		MessageID: "retry-usecase-dead",
		Queue:     queue.QueueHeavyTasks,
	}); err != nil {
		t.Fatalf("retry exhausted message: %v", err)
	}

	var received int
	if err := appDB.GetP(&received, `SELECT received FROM goqite WHERE id = ?`, "retry-usecase-dead"); err != nil {
		t.Fatalf("load retried message: %v", err)
	}
	if received != 0 {
		t.Fatalf("expected received reset to 0, got %d", received)
	}

	if err := usecase.RetryQueueMessageNow(ctx, usecase.QueueMessageActionCmd{
		MessageID: "retry-usecase-ready",
		Queue:     queue.QueueHeavyTasks,
	}); fwusecase.CodeOf(err) != fwusecase.CodeConflict {
		t.Fatalf("expected conflict for non-exhausted message, got %v", err)
	}

	if err := usecase.RetryQueueMessageNow(ctx, usecase.QueueMessageActionCmd{
		MessageID: "missing-message",
		Queue:     queue.QueueHeavyTasks,
	}); fwusecase.CodeOf(err) != fwusecase.CodeNotFound {
		t.Fatalf("expected not found for missing message, got %v", err)
	}
}

func TestDeleteQueueMessageRemovesGoqiteRow(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	queueManager, err := queue.NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}
	previousQueue := usecase.DefaultQueueManager
	usecase.DefaultQueueManager = queueManager
	t.Cleanup(func() {
		usecase.DefaultQueueManager = previousQueue
	})

	now := time.Now().UTC()
	seedUsecaseQueueMessage(t, "delete-usecase-message", queue.QueueHeavyTasks, now.Add(-10*time.Minute), now.Add(-1*time.Minute), queue.DefaultMaxReceive)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceSystem)
	if err := usecase.DeleteQueueMessage(ctx, usecase.QueueMessageActionCmd{
		MessageID: "delete-usecase-message",
		Queue:     queue.QueueHeavyTasks,
	}); err != nil {
		t.Fatalf("delete queue message: %v", err)
	}

	var count int
	if err := appDB.GetP(&count, `SELECT COUNT(*) FROM goqite WHERE id = ?`, "delete-usecase-message"); err != nil {
		t.Fatalf("count queue message: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected queue message deleted, got %d", count)
	}

	if err := usecase.DeleteQueueMessage(ctx, usecase.QueueMessageActionCmd{
		MessageID: "delete-usecase-message",
		Queue:     queue.QueueHeavyTasks,
	}); fwusecase.CodeOf(err) != fwusecase.CodeNotFound {
		t.Fatalf("expected not found after delete, got %v", err)
	}
}

func assertQueueMessageState(t *testing.T, messages []usecase.QueueMessageCo, id string, state string, maxReceive int, nextClaim string) {
	t.Helper()

	for _, message := range messages {
		if message.ID != id {
			continue
		}
		if message.State != state {
			t.Fatalf("expected %s state %q, got %#v", id, state, message)
		}
		if message.MaxReceive != maxReceive {
			t.Fatalf("expected %s max receive %d, got %#v", id, maxReceive, message)
		}
		if message.AgeSec <= 0 {
			t.Fatalf("expected %s positive age, got %#v", id, message)
		}
		if nextClaim == "future" {
			if message.NextClaimAt == "" || message.AvailableInSec <= 0 {
				t.Fatalf("expected %s future next claim, got %#v", id, message)
			}
		} else if message.NextClaimAt != "" || message.AvailableInSec != 0 {
			t.Fatalf("expected %s no future next claim, got %#v", id, message)
		}
		return
	}
	t.Fatalf("missing queue message %s in %#v", id, messages)
}

func seedUsecaseQueueMessage(t *testing.T, id string, queueName string, created time.Time, timeoutAt time.Time, received int) {
	t.Helper()

	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	if _, err := appDB.ExecP(`
		INSERT INTO goqite (id, queue, body, created, updated, timeout, received, priority)
		VALUES (?, ?, CAST(? AS BLOB), ?, ?, ?, ?, 0)
	`, id, queueName, `{"id":"`+id+`"}`, formatGoqiteTestTime(created), formatGoqiteTestTime(created), formatGoqiteTestTime(timeoutAt), received); err != nil {
		t.Fatalf("insert goqite message %s: %v", id, err)
	}
}

func formatGoqiteTestTime(value time.Time) string {
	return value.UTC().Format("2006-01-02T15:04:05.000Z07:00")
}

func assertUsecaseReference(t *testing.T, refs []usecase.QueueMessageReferenceCo, refType string, id string, status string, relatedType string, relatedID string) {
	t.Helper()

	for _, ref := range refs {
		if ref.Type == refType && ref.ID == id {
			if ref.Status != status || ref.RelatedType != relatedType || ref.RelatedID != relatedID {
				t.Fatalf("unexpected reference for %s/%s: %#v", refType, id, ref)
			}
			return
		}
	}
	t.Fatalf("missing reference %s/%s in %#v", refType, id, refs)
}
