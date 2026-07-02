package usecase_test

import (
	"context"
	"testing"

	"github.com/tfnick/go-svelte-starter/api/db"
	fwevents "github.com/tfnick/go-svelte-starter/api/framework/events"
	"github.com/tfnick/go-svelte-starter/api/framework/queue"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/usecase"
	usecaseevents "github.com/tfnick/go-svelte-starter/api/usecase/events"
	"github.com/tfnick/sqlx"
)

func TestPayOrderQueuesPointsAwardAndSubscriberIsIdempotent(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, orderID := seedPayableOrder(t, manager)
	registerUsecaseEventHandlers(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceSystem)
	order, err := usecase.PayOrder(ctx, usecase.PayOrderCmd{OrderID: orderID})
	if err != nil {
		t.Fatalf("pay order: %v", err)
	}
	if order.Status != "paid" {
		t.Fatalf("expected paid order, got %q", order.Status)
	}

	points, err := usecase.GetUserPoints(ctx, usecase.PointsBalanceQry{UserID: "u1"})
	if err != nil {
		t.Fatalf("get points: %v", err)
	}
	if points.Balance != 0 {
		t.Fatalf("expected points to be awarded asynchronously, got %d", points.Balance)
	}

	if deliveryCount := countRows(t, appDB, `SELECT COUNT(*) FROM domain_event_deliveries WHERE subscriber = ?`, usecaseevents.OrderPaidSubscriber); deliveryCount != 1 {
		t.Fatalf("expected one order paid delivery, got %d", deliveryCount)
	}
	if queueCount := countRows(t, appDB, `SELECT COUNT(*) FROM goqite WHERE queue = ?`, queue.QueueDomainEvents); queueCount != 2 {
		t.Fatalf("expected two domain event queue messages, got %d", queueCount)
	}

	handleDomainEventMessageForSubscriber(t, appDB, usecaseevents.OrderPaidSubscriber)
	points, err = usecase.GetUserPoints(ctx, usecase.PointsBalanceQry{UserID: "u1"})
	if err != nil {
		t.Fatalf("get points after handling event: %v", err)
	}
	if points.Balance != usecaseevents.OrderPaidPoints {
		t.Fatalf("expected %d points after handling event, got %d", usecaseevents.OrderPaidPoints, points.Balance)
	}

	if _, err := usecase.PayOrder(ctx, usecase.PayOrderCmd{OrderID: orderID}); err != nil {
		t.Fatalf("pay order again: %v", err)
	}
	points, err = usecase.GetUserPoints(ctx, usecase.PointsBalanceQry{UserID: "u1"})
	if err != nil {
		t.Fatalf("get points after duplicate pay: %v", err)
	}
	if points.Balance != usecaseevents.OrderPaidPoints {
		t.Fatalf("expected duplicate payment to keep %d points, got %d", usecaseevents.OrderPaidPoints, points.Balance)
	}
	if eventCount := countRows(t, appDB, `SELECT COUNT(*) FROM domain_events WHERE topic = ?`, usecaseevents.OrderPaidTopic); eventCount != 1 {
		t.Fatalf("expected duplicate payment not to publish another event, got %d", eventCount)
	}

	handleOrderPaidEventAgain(t, appDB, orderID)
	points, err = usecase.GetUserPoints(ctx, usecase.PointsBalanceQry{UserID: "u1"})
	if err != nil {
		t.Fatalf("get points after duplicate event handling: %v", err)
	}
	if points.Balance != usecaseevents.OrderPaidPoints {
		t.Fatalf("expected duplicate event handling to keep %d points, got %d", usecaseevents.OrderPaidPoints, points.Balance)
	}

	var txCount int
	if err := appDB.Get(&txCount, `SELECT COUNT(*) FROM point_transactions WHERE order_id = ?`, orderID); err != nil {
		t.Fatalf("count point transactions: %v", err)
	}
	if txCount != 1 {
		t.Fatalf("expected one point transaction, got %d", txCount)
	}
}

func TestPayOrderRollsBackOrderStatusWhenEventQueueFails(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, orderID := seedPayableOrder(t, manager)
	registerUsecaseEventHandlers(t)

	if _, err := appDB.Exec(`DROP TABLE goqite`); err != nil {
		t.Fatalf("drop goqite to force queue failure: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceSystem)
	_, err := usecase.PayOrder(ctx, usecase.PayOrderCmd{OrderID: orderID})
	if err == nil {
		t.Fatalf("expected payment failure")
	}
	if fwusecase.CodeOf(err) != fwusecase.CodeInternal {
		t.Fatalf("expected internal error, got %q: %v", fwusecase.CodeOf(err), err)
	}

	var status string
	if err := appDB.Get(&status, `SELECT status FROM orders WHERE id = ?`, orderID); err != nil {
		t.Fatalf("get order status: %v", err)
	}
	if status != "pending" {
		t.Fatalf("expected order status rollback to pending, got %q", status)
	}

	if eventCount := countRows(t, appDB, `SELECT COUNT(*) FROM domain_events WHERE topic = ?`, usecaseevents.OrderPaidTopic); eventCount != 0 {
		t.Fatalf("expected event insert rollback, got %d events", eventCount)
	}
}

func TestOrderPaidSubscriberFailureDoesNotRollBackPaidOrder(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, orderID := seedPayableOrder(t, manager)
	registerUsecaseEventHandlers(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceSystem)
	if _, err := usecase.PayOrder(ctx, usecase.PayOrderCmd{OrderID: orderID}); err != nil {
		t.Fatalf("pay order: %v", err)
	}

	if _, err := appDB.Exec(`DROP TABLE point_transactions`); err != nil {
		t.Fatalf("drop point_transactions: %v", err)
	}

	body := domainEventMessageBodyForSubscriber(t, appDB, usecaseevents.OrderPaidSubscriber)
	err := fwevents.HandleMessage(t.Context(), []byte(body))
	if err == nil {
		t.Fatalf("expected subscriber failure")
	}

	var status string
	if err := appDB.Get(&status, `SELECT status FROM orders WHERE id = ?`, orderID); err != nil {
		t.Fatalf("get order status: %v", err)
	}
	if status != "paid" {
		t.Fatalf("expected paid order to remain paid after subscriber failure, got %q", status)
	}

	var deliveryStatus string
	if err := appDB.Get(&deliveryStatus, `SELECT status FROM domain_event_deliveries WHERE subscriber = ?`, usecaseevents.OrderPaidSubscriber); err != nil {
		t.Fatalf("get delivery status: %v", err)
	}
	if deliveryStatus != models.DomainEventDeliveryStatusFailed {
		t.Fatalf("expected failed delivery, got %q", deliveryStatus)
	}
}

func TestUpdateOrderStatusCannotMarkOrderPaid(t *testing.T) {
	setupUsecaseOrderTxDB(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	err := usecase.UpdateOrderStatus(ctx, usecase.UpdateOrderStatusCmd{
		OrderID: "o1",
		Status:  "paid",
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected validation code, got %q", fwusecase.CodeOf(err))
	}
}

func TestPayOrderRejectsNonAdminActor(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	_, orderID := seedPayableOrder(t, manager)
	registerUsecaseEventHandlers(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	ctx.Actor = fwusecase.ActorContext{Authenticated: true, UserID: "u1"}
	_, err := usecase.PayOrder(ctx, usecase.PayOrderCmd{OrderID: orderID})
	if fwusecase.CodeOf(err) != fwusecase.CodeForbidden {
		t.Fatalf("expected forbidden for user direct payment confirmation, got %v", err)
	}
}

func TestPayOrderAllowsAdminActor(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	_, orderID := seedPayableOrder(t, manager)
	registerUsecaseEventHandlers(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	ctx.Actor = fwusecase.ActorContext{Authenticated: true, UserID: "admin-user", IsAdmin: true}
	order, err := usecase.PayOrder(ctx, usecase.PayOrderCmd{OrderID: orderID})
	if err != nil {
		t.Fatalf("admin pay order: %v", err)
	}
	if order.Status != "paid" {
		t.Fatalf("expected paid order, got %q", order.Status)
	}
}

func registerUsecaseEventHandlers(t *testing.T) {
	t.Helper()

	queueManager, err := queue.NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}
	fwevents.Configure(usecaseevents.DurableStore{}, queueManager)
	if err := usecaseevents.RegisterEventHandlers(func(ctx fwusecase.Context, cmd usecaseevents.AwardOrderPaidPointsCmd) (usecaseevents.PointsResult, bool, error) {
		points, awarded, err := usecase.AwardOrderPaidPoints(ctx, usecase.AwardOrderPaidPointsCmd{
			UserID:  cmd.UserID,
			OrderID: cmd.OrderID,
			Points:  cmd.Points,
		})
		return usecaseevents.PointsResult{
			UserID:  points.UserID,
			Balance: points.Balance,
		}, awarded, err
	}, sendRealtimeNotificationForUsecaseTest); err != nil {
		t.Fatalf("register event handlers: %v", err)
	}
	if err := usecaseevents.RegisterMembershipEventHandlers(func(ctx fwusecase.Context, cmd usecaseevents.ApplyOrderMembershipCmd) (usecaseevents.MembershipResult, bool, error) {
		membership, applied, err := usecase.ApplyOrderMembership(ctx, usecase.ApplyOrderMembershipCmd{
			OrderID: cmd.OrderID,
		})
		return usecaseevents.MembershipResult{
			UserID:              membership.UserID,
			MembershipLevel:     membership.MembershipLevel,
			MembershipExpiresAt: membership.MembershipExpiresAt,
		}, applied, err
	}); err != nil {
		t.Fatalf("register membership event handlers: %v", err)
	}
}

func handleNextDomainEventMessage(t *testing.T, appDB *sqlx.DB) {
	t.Helper()

	body := nextDomainEventMessageBody(t, appDB)
	if err := fwevents.HandleMessage(t.Context(), []byte(body)); err != nil {
		t.Fatalf("handle domain event message: %v", err)
	}
	if _, err := appDB.Exec(`DELETE FROM goqite WHERE queue = ?`, queue.QueueDomainEvents); err != nil {
		t.Fatalf("delete handled queue message: %v", err)
	}
}

func handleDomainEventMessageForSubscriber(t *testing.T, appDB *sqlx.DB, subscriber string) {
	t.Helper()

	body := domainEventMessageBodyForSubscriber(t, appDB, subscriber)
	if err := fwevents.HandleMessage(t.Context(), []byte(body)); err != nil {
		t.Fatalf("handle domain event message for %s: %v", subscriber, err)
	}
	if _, err := appDB.Exec(`DELETE FROM goqite WHERE queue = ? AND CAST(body AS TEXT) LIKE ?`, queue.QueueDomainEvents, `%"subscriber":"`+subscriber+`"%`); err != nil {
		t.Fatalf("delete handled queue message: %v", err)
	}
}

func handleOrderPaidEventAgain(t *testing.T, appDB *sqlx.DB, orderID string) {
	t.Helper()

	var eventID string
	if err := appDB.Get(&eventID, `SELECT id FROM domain_events WHERE aggregate_id = ? AND topic = ?`, orderID, usecaseevents.OrderPaidTopic); err != nil {
		t.Fatalf("get order paid event id: %v", err)
	}
	message := `{"event_id":"` + eventID + `","subscriber":"` + usecaseevents.OrderPaidSubscriber + `","topic":"` + usecaseevents.OrderPaidTopic + `"}`
	if err := fwevents.HandleMessage(context.Background(), []byte(message)); err != nil {
		t.Fatalf("handle order paid event again: %v", err)
	}
}

func nextDomainEventMessageBody(t *testing.T, appDB *sqlx.DB) string {
	t.Helper()

	var body string
	query := `SELECT CAST(body AS TEXT) FROM goqite WHERE queue = ? ORDER BY created LIMIT 1`
	if err := appDB.Get(&body, query, queue.QueueDomainEvents); err != nil {
		t.Fatalf("get domain event message body: %v", err)
	}
	return body
}

func domainEventMessageBodyForSubscriber(t *testing.T, appDB *sqlx.DB, subscriber string) string {
	t.Helper()

	var body string
	query := `SELECT CAST(body AS TEXT) FROM goqite WHERE queue = ? AND CAST(body AS TEXT) LIKE ? ORDER BY created LIMIT 1`
	if err := appDB.Get(&body, query, queue.QueueDomainEvents, `%"subscriber":"`+subscriber+`"%`); err != nil {
		t.Fatalf("get domain event message body for %s: %v", subscriber, err)
	}
	return body
}

func countRows(t *testing.T, appDB *sqlx.DB, query string, args ...interface{}) int {
	t.Helper()

	var count int
	if err := appDB.Get(&count, query, args...); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return count
}

func seedPayableOrder(t *testing.T, manager *db.DBManager) (*sqlx.DB, string) {
	t.Helper()

	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	if _, err := appDB.Exec(`INSERT INTO users (id, name, email, password_hash, email_verified, is_active) VALUES ('u1', 'Ada', 'ada@example.com', '', 1, 1)`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	seedUsecaseCheckoutProduct(t, appDB, "p1", "prod_usecase")
	if _, err := appDB.Exec(`INSERT INTO orders (id, user_id, product_id, amount, status) VALUES ('o1', 'u1', 'p1', 1000, 'pending')`); err != nil {
		t.Fatalf("insert order: %v", err)
	}
	return appDB, "o1"
}

func seedUsecaseCheckoutProduct(t *testing.T, appDB *sqlx.DB, productID string, creemProductID string) {
	t.Helper()

	if _, err := appDB.Exec(`
		INSERT INTO products (
			id, name, description, price, currency, stock, enabled, creem_product_id,
			billing_type, membership_level, subscription_interval, created_at, updated_at
		) VALUES (?, 'Premium Month', 'Membership test product', 1000, 'USD', 0, 1, ?, 'subscription', 'premium', 'month', '2026-01-01 00:00:00', '2026-01-01 00:00:00')
	`, productID, creemProductID); err != nil {
		t.Fatalf("insert checkout product: %v", err)
	}
}
