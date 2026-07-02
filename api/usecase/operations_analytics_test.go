package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/tfnick/go-svelte-starter/api/framework/authz"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase"
	"github.com/tfnick/sqlx"
)

func TestGetOperationsAnalyticsReturnsGrowthGeoHoursAndComparisons(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	excludeSeedUsersFromOperationsAnalyticsPeriod(t, appDB)
	now := timefmt.NowUTC().Truncate(time.Second)
	seedOperationsAnalyticsFixture(t, appDB, now)

	result, err := usecase.GetOperationsAnalytics(operationsAnalyticsActorContext(), usecase.OperationsAnalyticsQry{Period: "7d"})
	if err != nil {
		t.Fatalf("get operations analytics: %v", err)
	}

	if result.Period.Key != "7d" || result.Period.StartAt == "" || result.Period.EndAt == "" {
		t.Fatalf("unexpected period: %#v", result.Period)
	}
	if result.Summary.NewUsers.Value != 3 {
		t.Fatalf("expected 3 new users, got %#v", result.Summary.NewUsers)
	}
	if result.Summary.NewUsers.MoMPercent == nil || *result.Summary.NewUsers.MoMPercent != 200 {
		t.Fatalf("expected new user MoM 200, got %#v", result.Summary.NewUsers.MoMPercent)
	}
	if result.Summary.NewPaidUsers.Value != 3 {
		t.Fatalf("expected 3 new paid users, got %#v", result.Summary.NewPaidUsers)
	}
	if result.Summary.NewOrders.Value != 4 || result.Summary.PaidOrders.Value != 3 || result.Summary.PaidRevenueAmount.Value != 9000 {
		t.Fatalf("unexpected order summary: %#v", result.Summary)
	}
	if result.Summary.CanceledSubscriptions.Value != 1 {
		t.Fatalf("expected 1 canceled subscription by cancellation time, got %#v", result.Summary.CanceledSubscriptions)
	}
	if result.Summary.SubscriptionCancelPercent.Value != 50 {
		t.Fatalf("expected cancel rate 50, got %#v", result.Summary.SubscriptionCancelPercent)
	}
	if len(result.Trends.UserGrowth) == 0 || len(result.Trends.OrderGrowth) == 0 || len(result.Trends.Subscription) == 0 {
		t.Fatalf("expected trend rows, got %#v", result.Trends)
	}
	if len(result.RegistrationHours) != 24 {
		t.Fatalf("expected 24 registration hour buckets, got %d", len(result.RegistrationHours))
	}
	hourTotal := 0
	nonZeroHours := 0
	for _, item := range result.RegistrationHours {
		hourTotal += item.Count
		if item.Count > 0 {
			nonZeroHours++
		}
	}
	if hourTotal != 3 || nonZeroHours == 0 {
		t.Fatalf("unexpected hour distribution: %#v", result.RegistrationHours)
	}
	if result.Geo.SelectedCountry != "" {
		t.Fatalf("expected no default selected country, got %#v", result.Geo)
	}
	if len(result.Geo.Countries) < 2 || result.Geo.Countries[0].Label != "China" || result.Geo.Countries[0].Count != 2 {
		t.Fatalf("unexpected countries: %#v", result.Geo.Countries)
	}
	if len(result.Geo.Regions) < 3 || result.Geo.Regions[0].Label == "" {
		t.Fatalf("expected all-region distribution when no country is selected, got %#v", result.Geo.Regions)
	}
	if !geoItemsContain(result.Geo.Regions, "California", 1) {
		t.Fatalf("expected all-region distribution to include California, got %#v", result.Geo.Regions)
	}
	if len(result.SourceDistribution) < 2 || result.SourceDistribution[0].Label != "xiaohongshu" || result.SourceDistribution[0].Count != 2 {
		t.Fatalf("expected source distribution with xiaohongshu count 2, got %#v", result.SourceDistribution)
	}
	if !geoItemsContain(result.SourceDistribution, "wechat", 1) {
		t.Fatalf("expected source distribution to include wechat, got %#v", result.SourceDistribution)
	}
}

func TestGetOperationsAnalyticsScopesOrganizationAndCountryRegion(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	excludeSeedUsersFromOperationsAnalyticsPeriod(t, appDB)
	now := timefmt.NowUTC().Truncate(time.Second)
	seedOperationsAnalyticsFixture(t, appDB, now)

	ctx := fwusecase.NewContext(context.Background(), fwusecase.SurfaceInternalAPI)
	ctx.Actor.Authenticated = true
	ctx.Actor.Role = authz.RoleOrgAdmin
	ctx.Actor.OrganizationID = "org-a"
	ctx.Actor.Permissions = authz.PermissionsForRole(authz.RoleOrgAdmin)

	result, err := usecase.GetOperationsAnalytics(ctx, usecase.OperationsAnalyticsQry{Period: "7d", Country: "China"})
	if err != nil {
		t.Fatalf("get org operations analytics: %v", err)
	}
	if result.Summary.NewUsers.Value != 2 {
		t.Fatalf("expected org scope to see 2 new users, got %#v", result.Summary.NewUsers)
	}
	for _, country := range result.Geo.Countries {
		if country.Label != "China" {
			t.Fatalf("expected org-a country distribution to include China only, got %#v", result.Geo.Countries)
		}
	}
	if len(result.Geo.Regions) == 0 || result.Geo.Regions[0].Label != "Shanghai" {
		t.Fatalf("expected China region distribution, got %#v", result.Geo.Regions)
	}
}

func TestGetOperationsAnalyticsGeoUsesUserRegistrationProfileJoinAndExplicitCountryOnly(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	excludeSeedUsersFromOperationsAnalyticsPeriod(t, appDB)
	now := timefmt.NowUTC().Truncate(time.Second)

	users := []struct {
		id      string
		country string
		region  string
	}{
		{"geo-missing-a", "", ""},
		{"geo-missing-b", "", ""},
		{"geo-us", "United States", "California"},
	}
	for _, user := range users {
		createdAt := sqliteTime(now.AddDate(0, 0, -1))
		if _, err := appDB.ExecP(`
			INSERT INTO users (id, name, email, password_hash, email_verified, is_active, created_at, updated_at)
			VALUES (?, ?, ?, '', 1, 1, ?, ?)
		`, user.id, user.id, user.id+"@example.com", createdAt, createdAt); err != nil {
			t.Fatalf("insert geo user: %v", err)
		}
		if user.country == "" && user.region == "" {
			continue
		}
		if _, err := appDB.ExecP(`
			INSERT INTO user_registration_profiles (id, user_id, registration_ip, registration_country, registration_region, registration_user_agent, created_at, updated_at)
			VALUES (?, ?, '127.0.0.1', ?, ?, 'test-agent', ?, ?)
		`, user.id+"-profile", user.id, user.country, user.region, createdAt, createdAt); err != nil {
			t.Fatalf("insert geo profile: %v", err)
		}
	}

	result, err := usecase.GetOperationsAnalytics(operationsAnalyticsActorContext(), usecase.OperationsAnalyticsQry{Period: "7d"})
	if err != nil {
		t.Fatalf("get operations analytics: %v", err)
	}

	if result.Geo.SelectedCountry != "" {
		t.Fatalf("expected selected country to stay empty without query country, got %#v", result.Geo)
	}
	if !geoItemsContain(result.Geo.Countries, "Unknown", 2) || !geoItemsContain(result.Geo.Countries, "United States", 1) {
		t.Fatalf("expected country distribution from users left join profiles, got %#v", result.Geo.Countries)
	}
	if !geoItemsContain(result.Geo.Regions, "Unknown", 2) || !geoItemsContain(result.Geo.Regions, "California", 1) {
		t.Fatalf("expected unfiltered region distribution without selected country, got %#v", result.Geo.Regions)
	}

	result, err = usecase.GetOperationsAnalytics(operationsAnalyticsActorContext(), usecase.OperationsAnalyticsQry{Period: "7d", Country: "United States"})
	if err != nil {
		t.Fatalf("get operations analytics with country: %v", err)
	}
	if result.Geo.SelectedCountry != "United States" {
		t.Fatalf("expected explicit selected country, got %#v", result.Geo)
	}
	if len(result.Geo.Regions) != 1 || result.Geo.Regions[0].Label != "California" || result.Geo.Regions[0].Count != 1 {
		t.Fatalf("expected regions filtered by explicit country, got %#v", result.Geo.Regions)
	}
}

func TestGetOperationsAnalyticsRejectsRegularUserAndInvalidPeriod(t *testing.T) {
	ctx := fwusecase.NewContext(context.Background(), fwusecase.SurfaceInternalAPI)
	ctx.Actor.Authenticated = true
	ctx.Actor.Permissions = authz.PermissionsForRole(authz.RoleUser)

	if _, err := usecase.GetOperationsAnalytics(ctx, usecase.OperationsAnalyticsQry{}); fwusecase.CodeOf(err) != fwusecase.CodeForbidden {
		t.Fatalf("expected forbidden for regular user, got %v", err)
	}

	ctx = operationsAnalyticsActorContext()
	if _, err := usecase.GetOperationsAnalytics(ctx, usecase.OperationsAnalyticsQry{Period: "custom"}); fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected validation for invalid period, got %v", err)
	}
}

func operationsAnalyticsActorContext() fwusecase.Context {
	ctx := fwusecase.NewContext(context.Background(), fwusecase.SurfaceInternalAPI)
	ctx.Actor.Authenticated = true
	ctx.Actor.Role = authz.RolePlatformAdmin
	ctx.Actor.Permissions = authz.PermissionsForRole(authz.RolePlatformAdmin)
	return ctx
}

func geoItemsContain(items []usecase.OperationsAnalyticsGeoItemCo, label string, count int) bool {
	for _, item := range items {
		if item.Label == label && item.Count == count {
			return true
		}
	}
	return false
}

func excludeSeedUsersFromOperationsAnalyticsPeriod(t *testing.T, appDB *sqlx.Engine) {
	t.Helper()

	if _, err := appDB.ExecP(`
		UPDATE users
		SET created_at = ?, updated_at = ?
	`, "2020-01-01 00:00:00", "2020-01-01 00:00:00"); err != nil {
		t.Fatalf("move seed users outside operations analytics period: %v", err)
	}
}

func seedOperationsAnalyticsFixture(t *testing.T, appDB *sqlx.Engine, now time.Time) {
	t.Helper()

	users := []struct {
		id        string
		name      string
		email     string
		org       string
		active    int
		createdAt string
		country   string
		region    string
		utmSource string
	}{
		{"ops-user-a", "Ada", "ada@example.com", "org-a", 1, sqliteTime(now.AddDate(0, 0, -1).Add(-3 * time.Hour)), "China", "Shanghai", "xiaohongshu"},
		{"ops-user-b", "Bea", "bea@example.com", "org-a", 1, sqliteTime(now.AddDate(0, 0, -2).Add(10 * time.Hour)), "China", "Zhejiang", "wechat"},
		{"ops-user-c", "Cid", "cid@example.com", "org-b", 0, sqliteTime(now.AddDate(0, 0, -3).Add(-3 * time.Hour)), "United States", "California", "xiaohongshu"},
		{"ops-user-prev", "Pre", "pre@example.com", "org-a", 1, sqliteTime(now.AddDate(0, 0, -10)), "China", "Shanghai", "wechat"},
		{"ops-user-yoy", "Yoy", "yoy@example.com", "org-a", 1, sqliteTime(now.AddDate(-1, 0, -1)), "China", "Shanghai", "xiaohongshu"},
	}
	for _, user := range users {
		if _, err := appDB.ExecP(`
			INSERT INTO users (id, name, email, password_hash, email_verified, is_active, organization_id, created_at, updated_at)
			VALUES (?, ?, ?, '', 1, ?, ?, ?, ?)
		`, user.id, user.name, user.email, user.active, user.org, user.createdAt, user.createdAt); err != nil {
			t.Fatalf("insert operations user: %v", err)
		}
		if _, err := appDB.ExecP(`
			INSERT INTO user_registration_profiles (id, user_id, registration_ip, registration_country, registration_region, registration_user_agent, utm_source, created_at, updated_at)
			VALUES (?, ?, '127.0.0.1', ?, ?, 'test-agent', ?, ?, ?)
		`, user.id+"-profile", user.id, user.country, user.region, user.utmSource, user.createdAt, user.createdAt); err != nil {
			t.Fatalf("insert operations registration profile: %v", err)
		}
	}

	products := []struct {
		id      string
		billing string
	}{
		{"ops-sub-product", "subscription"},
		{"ops-one-product", "one_time"},
	}
	for _, product := range products {
		if _, err := appDB.ExecP(`
			INSERT INTO products (
				id, name, description, price, currency, stock, enabled, creem_product_id,
				billing_type, membership_level, subscription_interval, created_at, updated_at
			) VALUES (?, ?, '', 3000, 'USD', 0, 1, ?, ?, 'premium', 'month', '2026-01-01 00:00:00', '2026-01-01 00:00:00')
		`, product.id, product.id, product.id+"_creem", product.billing); err != nil {
			t.Fatalf("insert operations product: %v", err)
		}
	}

	orders := []struct {
		id                 string
		userID             string
		productID          string
		amount             int64
		status             string
		subscriptionStatus string
		createdAt          string
		canceledAt         string
	}{
		{"ops-order-a", "ops-user-a", "ops-sub-product", 3000, "paid", "active", sqliteTime(now.AddDate(0, 0, -1)), ""},
		{"ops-order-b", "ops-user-b", "ops-sub-product", 3000, "paid", "canceled", sqliteTime(now.AddDate(0, 0, -2)), sqliteTime(now.AddDate(0, 0, -1).Add(time.Hour))},
		{"ops-order-c", "ops-user-c", "ops-one-product", 3000, "paid", "", sqliteTime(now.AddDate(0, 0, -3)), ""},
		{"ops-order-d", "ops-user-c", "ops-one-product", 500, "pending", "", sqliteTime(now.AddDate(0, 0, -3).Add(time.Hour)), ""},
		{"ops-order-prev", "ops-user-prev", "ops-one-product", 1000, "paid", "", sqliteTime(now.AddDate(0, 0, -10)), ""},
		{"ops-order-yoy", "ops-user-yoy", "ops-one-product", 1000, "paid", "", sqliteTime(now.AddDate(-1, 0, -1)), ""},
	}
	for _, order := range orders {
		if _, err := appDB.ExecP(`
			INSERT INTO orders (
				id, user_id, product_id, amount, status, subscription_status, subscription_canceled_at, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, order.id, order.userID, order.productID, order.amount, order.status, order.subscriptionStatus, order.canceledAt, order.createdAt); err != nil {
			t.Fatalf("insert operations order: %v", err)
		}
	}
}
