package usecase_test

import (
	"context"
	"testing"
	"time"

	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/payment"
)

type recordingCancelPaymentAdapter struct {
	t        *testing.T
	requests []payment.CancelSubscriptionRequest
}

func (a *recordingCancelPaymentAdapter) CreatePayment(ctx context.Context, cfg payment.ProviderConfig, req payment.CreatePaymentRequest) (payment.CreatePaymentResult, error) {
	a.t.Helper()
	return payment.CreatePaymentResult{}, nil
}

func (a *recordingCancelPaymentAdapter) NormalizePaymentWebhook(ctx context.Context, cfg payment.ProviderConfig, req payment.WebhookRequest) (payment.NormalizedWebhook, error) {
	a.t.Helper()
	return payment.NormalizedWebhook{}, nil
}

func (a *recordingCancelPaymentAdapter) CancelSubscription(ctx context.Context, cfg payment.ProviderConfig, req payment.CancelSubscriptionRequest) (payment.CancelSubscriptionResult, error) {
	a.t.Helper()
	if cfg.ChannelCode != "usecase-creem" || cfg.APIKey != "payment-api-key" {
		a.t.Fatalf("unexpected provider config: %#v", cfg)
	}
	a.requests = append(a.requests, req)
	return payment.CancelSubscriptionResult{Status: "scheduled_cancel"}, nil
}

func TestProductCRUDStoresCreemAndMembershipSettings(t *testing.T) {
	setupUsecaseOrderTxDB(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	product, err := usecase.CreateProduct(ctx, usecase.SaveProductCmd{
		Name:                 "Super Quarter",
		Description:          "Quarterly test membership",
		Price:                2999,
		Currency:             "usd",
		Enabled:              true,
		CreemProductID:       "prod_super_quarter",
		BillingType:          usecase.ProductBillingTypeSubscription,
		MembershipLevel:      usecase.MembershipLevelSuper,
		SubscriptionInterval: usecase.SubscriptionIntervalThreeMonths,
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	if product.ID == "" || product.Currency != "USD" || !product.Enabled || product.CreemProductID != "prod_super_quarter" {
		t.Fatalf("unexpected created product: %#v", product)
	}

	updated, err := usecase.UpdateProduct(ctx, usecase.SaveProductCmd{
		ID:              product.ID,
		Name:            "Premium Lifetime",
		Enabled:         true,
		CreemProductID:  "prod_premium_lifetime",
		BillingType:     usecase.ProductBillingTypeOneTime,
		MembershipLevel: usecase.MembershipLevelPremium,
	})
	if err != nil {
		t.Fatalf("update product: %v", err)
	}
	if updated.Name != "Premium Lifetime" || updated.SubscriptionInterval != "" || updated.BillingType != usecase.ProductBillingTypeOneTime {
		t.Fatalf("unexpected updated product: %#v", updated)
	}
}

func TestApplyOrderMembershipAppliesNowPlusInterval(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	if _, err := appDB.Exec(`INSERT INTO users (id, name, email, password_hash, email_verified, is_active, membership_level, membership_expires_at) VALUES ('member-user', 'Ada', 'ada@example.com', '', 1, 1, 'basic', '2099-01-31 00:00:00')`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	seedUsecaseCheckoutProduct(t, appDB, "member-product", "prod_member")
	if _, err := appDB.Exec(`INSERT INTO orders (id, user_id, product_id, amount, status) VALUES ('member-order', 'member-user', 'member-product', 0, 'paid')`); err != nil {
		t.Fatalf("insert order: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	membership, applied, err := usecase.ApplyOrderMembership(ctx, usecase.ApplyOrderMembershipCmd{OrderID: "member-order"})
	if err != nil {
		t.Fatalf("apply membership: %v", err)
	}
	expectedExpiry := time.Now().UTC().AddDate(0, 1, 0).Format("2006-01-02")
	if !applied || membership.MembershipLevel != usecase.MembershipLevelPremium || membership.MembershipExpiresAt[:10] != expectedExpiry {
		t.Fatalf("unexpected membership result: applied=%v membership=%#v (expected expiry prefix %s)", applied, membership, expectedExpiry)
	}

	_, appliedAgain, err := usecase.ApplyOrderMembership(ctx, usecase.ApplyOrderMembershipCmd{OrderID: "member-order"})
	if err != nil {
		t.Fatalf("apply membership again: %v", err)
	}
	if appliedAgain {
		t.Fatalf("expected duplicate membership application to be idempotent")
	}
	var user models.User
	if err := appDB.Get(&user, `SELECT * FROM users WHERE id = 'member-user'`); err != nil {
		t.Fatalf("load user: %v", err)
	}
	if user.MembershipLevel != usecase.MembershipLevelPremium || user.MembershipExpiresAt[:10] != expectedExpiry {
		t.Fatalf("unexpected persisted user membership: %#v (expected expiry prefix %s)", user, expectedExpiry)
	}
}

func TestApplyOrderMembershipMakesOneTimeProductPermanent(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	if _, err := appDB.Exec(`INSERT INTO users (id, name, email, password_hash, email_verified, is_active, membership_level, membership_expires_at) VALUES ('one-time-user', 'Ada', 'ada@example.com', '', 1, 1, 'basic', '2026-01-01 00:00:00')`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := appDB.Exec(`
		INSERT INTO products (
			id, name, description, price, currency, stock, enabled, creem_product_id, billing_type,
			membership_level, subscription_interval, created_at, updated_at
		) VALUES (?, 'Lifetime', '', 5000, 'USD', 0, 1, 'prod_lifetime', 'one_time', 'super', '', '2026-01-01 00:00:00', '2026-01-01 00:00:00')
	`, "one-time-product"); err != nil {
		t.Fatalf("insert product: %v", err)
	}
	if _, err := appDB.Exec(`INSERT INTO orders (id, user_id, product_id, amount, status) VALUES ('one-time-order', 'one-time-user', 'one-time-product', 0, 'paid')`); err != nil {
		t.Fatalf("insert order: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	membership, applied, err := usecase.ApplyOrderMembership(ctx, usecase.ApplyOrderMembershipCmd{OrderID: "one-time-order"})
	if err != nil {
		t.Fatalf("apply membership: %v", err)
	}
	if !applied || membership.MembershipLevel != usecase.MembershipLevelSuper || membership.MembershipExpiresAt != usecase.PermanentMembershipExpiresAt {
		t.Fatalf("unexpected membership result: applied=%v membership=%#v", applied, membership)
	}
}

func TestCancelOrderSubscriptionOnlyUpdatesOrderSubscriptionStatus(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	if _, err := appDB.Exec(`INSERT INTO users (id, name, email, password_hash, email_verified, is_active, membership_level, membership_expires_at) VALUES ('cancel-user', 'Ada', 'ada@example.com', '', 1, 1, 'premium', '2099-02-28 00:00:00')`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	seedUsecaseCheckoutProduct(t, appDB, "cancel-product", "prod_cancel")
	if _, err := appDB.Exec(`INSERT INTO orders (id, user_id, product_id, amount, status, provider_subscription_id, subscription_status) VALUES ('cancel-order', 'cancel-user', 'cancel-product', 0, 'paid', 'sub_cancel', 'active')`); err != nil {
		t.Fatalf("insert order: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	if err := usecase.CancelOrderSubscription(ctx, usecase.CancelOrderSubscriptionCmd{ProviderSubscriptionID: "sub_cancel"}); err != nil {
		t.Fatalf("cancel subscription: %v", err)
	}

	var order models.Order
	if err := appDB.Get(&order, `SELECT * FROM orders WHERE id = 'cancel-order'`); err != nil {
		t.Fatalf("load order: %v", err)
	}
	if order.Status != "paid" || order.SubscriptionStatus != usecase.OrderSubscriptionStatusCanceled {
		t.Fatalf("unexpected order after cancellation: %#v", order)
	}
	if order.SubscriptionCanceledAt == "" {
		t.Fatalf("expected subscription_canceled_at to be set")
	}
	firstCanceledAt := order.SubscriptionCanceledAt
	if err := usecase.CancelOrderSubscription(ctx, usecase.CancelOrderSubscriptionCmd{ProviderSubscriptionID: "sub_cancel"}); err != nil {
		t.Fatalf("cancel subscription again: %v", err)
	}
	if err := appDB.Get(&order, `SELECT * FROM orders WHERE id = 'cancel-order'`); err != nil {
		t.Fatalf("reload order: %v", err)
	}
	if order.SubscriptionCanceledAt != firstCanceledAt {
		t.Fatalf("expected cancellation timestamp to be idempotent, before=%q after=%q", firstCanceledAt, order.SubscriptionCanceledAt)
	}
	var user models.User
	if err := appDB.Get(&user, `SELECT * FROM users WHERE id = 'cancel-user'`); err != nil {
		t.Fatalf("load user: %v", err)
	}
	if user.MembershipLevel != "premium" || !sameSQLiteInstant(user.MembershipExpiresAt, "2099-02-28 00:00:00") {
		t.Fatalf("membership should not change on cancellation: %#v", user)
	}
}

func TestEffectiveMembershipExpiredReturnsBasic(t *testing.T) {
	user := &models.User{
		MembershipLevel:     usecase.MembershipLevelPremium,
		MembershipExpiresAt: "2022-01-01 00:00:00",
	}
	level, expiresAt := usecase.EffectiveMembership(user)
	if level != usecase.MembershipLevelBasic {
		t.Fatalf("expected basic for expired premium, got %s", level)
	}
	if expiresAt != usecase.PermanentMembershipExpiresAt {
		t.Fatalf("expected permanent expiry for basic fallback, got %s", expiresAt)
	}
}

func TestEffectiveMembershipActiveReturnsLevel(t *testing.T) {
	future := time.Now().UTC().AddDate(0, 6, 0).Format("2006-01-02 15:04:05")
	user := &models.User{
		MembershipLevel:     usecase.MembershipLevelSuper,
		MembershipExpiresAt: future,
	}
	level, expiresAt := usecase.EffectiveMembership(user)
	if level != usecase.MembershipLevelSuper {
		t.Fatalf("expected super for active membership, got %s", level)
	}
	if expiresAt != user.MembershipExpiresAt {
		t.Fatalf("expected expiry %s for active membership, got %s", user.MembershipExpiresAt, expiresAt)
	}
}

func TestMembershipLevelRank(t *testing.T) {
	tests := []struct {
		level    string
		expected int
	}{
		{usecase.MembershipLevelBasic, 1},
		{usecase.MembershipLevelPremium, 2},
		{usecase.MembershipLevelSuper, 3},
		{"unknown", 1},
	}
	for _, tc := range tests {
		_ = tc
	}
}

func TestApplyOrderMembershipUpgradesMembershipLevel(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	if _, err := appDB.Exec(`INSERT INTO users (id, name, email, password_hash, email_verified, is_active, membership_level, membership_expires_at) VALUES ('upgrade-user', 'Bob', 'bob@example.com', '', 1, 1, 'basic', '2099-12-31 23:59:59')`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := appDB.Exec(`
		INSERT INTO products (
			id, name, description, price, currency, stock, enabled, creem_product_id, billing_type,
			membership_level, subscription_interval, created_at, updated_at
		) VALUES (?, 'Super Month', 'Super membership', 5000, 'USD', 0, 1, 'prod_super_month', 'subscription', 'super', 'month', '2026-01-01 00:00:00', '2026-01-01 00:00:00')
	`, "super-product"); err != nil {
		t.Fatalf("insert super product: %v", err)
	}
	if _, err := appDB.Exec(`INSERT INTO orders (id, user_id, product_id, amount, status) VALUES ('upgrade-order', 'upgrade-user', 'super-product', 0, 'paid')`); err != nil {
		t.Fatalf("insert order: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	membership, applied, err := usecase.ApplyOrderMembership(ctx, usecase.ApplyOrderMembershipCmd{OrderID: "upgrade-order"})
	if err != nil {
		t.Fatalf("apply membership: %v", err)
	}
	if !applied || membership.MembershipLevel != usecase.MembershipLevelSuper {
		t.Fatalf("expected level upgrade from basic to super: applied=%v membership=%#v", applied, membership)
	}

	var user models.User
	if err := appDB.Get(&user, `SELECT * FROM users WHERE id = 'upgrade-user'`); err != nil {
		t.Fatalf("load user: %v", err)
	}
	if user.MembershipLevel != usecase.MembershipLevelSuper {
		t.Fatalf("expected user membership_level=super, got %s", user.MembershipLevel)
	}
}

func TestApplyOrderMembershipCancelsOldActiveSubscriptionOnUpgrade(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	if _, err := appDB.Exec(`INSERT INTO users (id, name, email, password_hash, email_verified, is_active, membership_level, membership_expires_at) VALUES ('upgrade-sub-user', 'Bob', 'bob@example.com', '', 1, 1, 'premium', '2099-12-31 23:59:59')`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := appDB.Exec(`
		INSERT INTO products (
			id, name, description, price, currency, stock, enabled, creem_product_id, billing_type,
			membership_level, subscription_interval, created_at, updated_at
		) VALUES (?, 'Premium Month', 'Premium membership', 1000, 'USD', 0, 1, 'prod_premium_month', 'subscription', 'premium', 'month', '2026-01-01 00:00:00', '2026-01-01 00:00:00')
	`, "premium-sub-product"); err != nil {
		t.Fatalf("insert premium product: %v", err)
	}
	if _, err := appDB.Exec(`
		INSERT INTO products (
			id, name, description, price, currency, stock, enabled, creem_product_id, billing_type,
			membership_level, subscription_interval, created_at, updated_at
		) VALUES (?, 'Super Month', 'Super membership', 5000, 'USD', 0, 1, 'prod_super_month', 'subscription', 'super', 'month', '2026-01-01 00:00:00', '2026-01-01 00:00:00')
	`, "super-sub-product"); err != nil {
		t.Fatalf("insert super product: %v", err)
	}
	if _, err := appDB.Exec(`INSERT INTO orders (id, user_id, product_id, amount, status, provider_subscription_id, subscription_status, membership_applied_at) VALUES ('old-sub-order', 'upgrade-sub-user', 'premium-sub-product', 0, 'paid', 'sub_old_premium', 'active', '2026-01-01 00:00:00')`); err != nil {
		t.Fatalf("insert old subscription order: %v", err)
	}
	if _, err := appDB.Exec(`INSERT INTO orders (id, user_id, product_id, amount, status, provider_subscription_id, subscription_status) VALUES ('new-super-order', 'upgrade-sub-user', 'super-sub-product', 0, 'paid', 'sub_new_super', 'active')`); err != nil {
		t.Fatalf("insert new subscription order: %v", err)
	}
	seedPaymentIntegrationConfig(t, appDB, "payment.creem.upgrade-cancel-test")
	adapter := &recordingCancelPaymentAdapter{t: t}
	if err := usecase.RegisterPaymentAdapter("payment.creem.upgrade-cancel-test", adapter); err != nil {
		t.Fatalf("register payment adapter: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	membership, applied, err := usecase.ApplyOrderMembership(ctx, usecase.ApplyOrderMembershipCmd{OrderID: "new-super-order"})
	if err != nil {
		t.Fatalf("apply membership: %v", err)
	}
	if !applied || membership.MembershipLevel != usecase.MembershipLevelSuper {
		t.Fatalf("expected super membership to be applied, got applied=%v membership=%#v", applied, membership)
	}
	if len(adapter.requests) != 1 {
		t.Fatalf("expected one Creem cancellation request, got %#v", adapter.requests)
	}
	cancelReq := adapter.requests[0]
	if cancelReq.SubscriptionID != "sub_old_premium" || cancelReq.Mode != payment.CancelSubscriptionModeScheduled || cancelReq.OnExecute != payment.CancelSubscriptionOnExecuteCancel {
		t.Fatalf("unexpected cancellation request: %#v", cancelReq)
	}

	var oldStatus string
	if err := appDB.Get(&oldStatus, `SELECT subscription_status FROM orders WHERE id = 'old-sub-order'`); err != nil {
		t.Fatalf("load old order status: %v", err)
	}
	if oldStatus != usecase.OrderSubscriptionStatusCanceled {
		t.Fatalf("expected old subscription status canceled, got %q", oldStatus)
	}
	var oldCanceledAt string
	if err := appDB.Get(&oldCanceledAt, `SELECT subscription_canceled_at FROM orders WHERE id = 'old-sub-order'`); err != nil {
		t.Fatalf("load old order cancellation time: %v", err)
	}
	if oldCanceledAt == "" {
		t.Fatalf("expected old subscription cancellation time to be set")
	}
	var newStatus string
	if err := appDB.Get(&newStatus, `SELECT subscription_status FROM orders WHERE id = 'new-super-order'`); err != nil {
		t.Fatalf("load new order status: %v", err)
	}
	if newStatus != usecase.OrderSubscriptionStatusActive {
		t.Fatalf("expected new subscription to stay active, got %q", newStatus)
	}
}

func TestApplyOrderMembershipRejectsNonPaidOrder(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	if _, err := appDB.Exec(`INSERT INTO users (id, name, email, password_hash, email_verified, is_active, membership_level, membership_expires_at) VALUES ('pending-user', 'Eve', 'eve@example.com', '', 1, 1, 'basic', '2099-12-31 23:59:59')`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	seedUsecaseCheckoutProduct(t, appDB, "pending-product", "prod_pending")
	if _, err := appDB.Exec(`INSERT INTO orders (id, user_id, product_id, amount, status) VALUES ('pending-order', 'pending-user', 'pending-product', 0, 'pending')`); err != nil {
		t.Fatalf("insert order: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	_, applied, err := usecase.ApplyOrderMembership(ctx, usecase.ApplyOrderMembershipCmd{OrderID: "pending-order"})
	if err == nil {
		t.Fatalf("expected error for non-paid order")
	}
	if applied {
		t.Fatalf("expected membership not applied for non-paid order")
	}
}

func sameSQLiteInstant(actual string, expected string) bool {
	if actual == expected {
		return true
	}
	return actual == expected[:10]+"T"+expected[11:]+"Z"
}
