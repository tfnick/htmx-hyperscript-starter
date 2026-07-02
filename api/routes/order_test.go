package routes_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/tfnick/go-svelte-starter/api/db"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/routes"
	"github.com/tfnick/sqlx"
)

func TestGetUserOrdersReturnsPaginatedEnvelope(t *testing.T) {
	setupRouteTestDBs(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	const seedUserID = "019ea0c1-0001-7000-8000-000000000001"
	seedRouteUserOrdersForPagination(t, appDB, seedUserID, 5)

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/orders/user/"+seedUserID+"?page=2&page_size=2", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	c.SetParamNames("user_id")
	c.SetParamValues(seedUserID)
	fwcontext.SetCurrentUser(c, &models.User{ID: seedUserID, Name: "Ada", IsAdmin: 0})

	if err := routes.GetUserOrders(c); err != nil {
		t.Fatalf("get user orders: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var envelope struct {
		Success bool                      `json:"success"`
		Data    routes.UserOrdersResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success {
		t.Fatalf("expected success envelope, got %s", rec.Body.String())
	}
	if len(envelope.Data.Items) != 2 {
		t.Fatalf("expected two order items, got %#v", envelope.Data.Items)
	}
	if envelope.Data.Items[0].ID != "route-order-03" || envelope.Data.Items[1].ID != "route-order-02" {
		t.Fatalf("expected stable page items, got %#v", envelope.Data.Items)
	}
	if envelope.Data.Pagination.TotalItems != 5 || envelope.Data.Pagination.TotalPages != 3 {
		t.Fatalf("unexpected pagination metadata: %#v", envelope.Data.Pagination)
	}
}

func TestListMyOrdersUsesCurrentUser(t *testing.T) {
	setupRouteTestDBs(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	const currentUserID = "019ea0c1-0001-7000-8000-000000000001"
	const otherUserID = "019ea0c1-0002-7000-8000-000000000002"
	ensureRouteTestUser(t, appDB, otherUserID)
	seedRouteUserOrdersForPagination(t, appDB, currentUserID, 2)
	seedRouteUserOrdersForPaginationWithPrefix(t, appDB, otherUserID, "route-other-order", 1)

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/user/orders?page=1&page_size=10", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	fwcontext.SetCurrentUser(c, &models.User{ID: currentUserID, Name: "Ada", IsAdmin: 0})

	if err := routes.ListMyOrders(c); err != nil {
		t.Fatalf("list my orders: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool                      `json:"success"`
		Data    routes.UserOrdersResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(envelope.Data.Items) != 2 {
		t.Fatalf("expected current-user orders only, got %#v", envelope.Data.Items)
	}
	for _, order := range envelope.Data.Items {
		if order.UserID != currentUserID {
			t.Fatalf("expected current-user order, got %#v", order)
		}
	}
}

func TestListAdminOrdersAllowsUserAndStatusFilters(t *testing.T) {
	setupRouteTestDBs(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	const seedUserID = "019ea0c1-0001-7000-8000-000000000001"
	if _, err := appDB.ExecP(`UPDATE users SET name = ?, email = ? WHERE id = ?`, "Ada Admin", "ada@example.com", seedUserID); err != nil {
		t.Fatalf("update admin user: %v", err)
	}
	seedRouteUserOrdersForPagination(t, appDB, seedUserID, 3)
	if _, err := appDB.ExecP(`UPDATE orders SET status = ? WHERE id = ?`, "paid", "route-order-02"); err != nil {
		t.Fatalf("mark paid order: %v", err)
	}

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/orders?user_query=ada%40example.com&status=paid&start_time=2026-01-01T00%3A00%3A02Z&end_time=2026-01-01T00%3A00%3A02Z&page=1&page_size=10", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	fwcontext.SetCurrentUser(c, &models.User{ID: seedUserID, Name: "Admin", IsAdmin: 1})

	if err := routes.ListAdminOrders(c); err != nil {
		t.Fatalf("list admin orders: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool                      `json:"success"`
		Data    routes.UserOrdersResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(envelope.Data.Items) != 1 || envelope.Data.Items[0].ID != "route-order-02" {
		t.Fatalf("expected filtered paid order, got %#v", envelope.Data.Items)
	}
}

func TestGetOrderAnalyticsLegacyPathReturnsOrderListOnly(t *testing.T) {
	setupRouteTestDBs(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	const seedUserID = "019ea0c1-0001-7000-8000-000000000001"
	if _, err := appDB.ExecP(`UPDATE users SET name = ?, email = ? WHERE id = ?`, "Ada Admin", "ada@example.com", seedUserID); err != nil {
		t.Fatalf("update analytics user: %v", err)
	}
	seedRouteCheckoutProduct(t, appDB, "route-analytics-product", "prod_route_analytics")
	if _, err := appDB.ExecP(`
		INSERT INTO orders (id, user_id, product_id, amount, status, subscription_status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "route-analytics-order", seedUserID, "route-analytics-product", 9900, "paid", "active", timefmt.SQLiteDateTime(timefmt.NowUTC().AddDate(0, 0, -1))); err != nil {
		t.Fatalf("insert analytics order: %v", err)
	}

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/order-analytics?period=7d&user_query=ada&status=paid&page=1&page_size=10", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	fwcontext.SetCurrentUser(c, &models.User{
		ID:      seedUserID,
		Name:    "Admin",
		IsAdmin: 1,
		Role:    "platform_admin",
	})

	if err := routes.GetOrderAnalytics(c); err != nil {
		t.Fatalf("get order analytics: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool                      `json:"success"`
		Data    routes.UserOrdersResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success {
		t.Fatalf("expected success envelope, got %s", rec.Body.String())
	}
	if len(envelope.Data.Items) != 1 || envelope.Data.Items[0].ID != "route-analytics-order" {
		t.Fatalf("unexpected order response: %#v", envelope.Data.Items)
	}
	if envelope.Data.Pagination.TotalItems != 1 {
		t.Fatalf("unexpected pagination: %#v", envelope.Data.Pagination)
	}
	var raw struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw response: %v", err)
	}
	if _, ok := raw.Data["summary"]; ok {
		t.Fatalf("legacy order analytics path must not return summary: %s", rec.Body.String())
	}
	if _, ok := raw.Data["period"]; ok {
		t.Fatalf("legacy order analytics path must not return period: %s", rec.Body.String())
	}
}

func TestGetOperationsAnalyticsReturnsGrowthEnvelope(t *testing.T) {
	setupRouteTestDBs(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	now := timefmt.NowUTC().Truncate(time.Second)
	const seedUserID = "019ea0c1-0001-7000-8000-000000000001"
	if _, err := appDB.ExecP(`
		UPDATE users
		SET created_at = '2020-01-01 00:00:00', updated_at = '2020-01-01 00:00:00'
	`); err != nil {
		t.Fatalf("move seed users outside operations analytics period: %v", err)
	}
	if _, err := appDB.ExecP(`
		UPDATE users
		SET name = 'Ops Admin', email = 'ops@example.com', role = 'platform_admin', created_at = ?, updated_at = ?
		WHERE id = ?
	`, timefmt.SQLiteDateTime(now.AddDate(0, 0, -1)), timefmt.SQLiteDateTime(now.AddDate(0, 0, -1)), seedUserID); err != nil {
		t.Fatalf("update operations user: %v", err)
	}
	if _, err := appDB.ExecP(`
		INSERT INTO user_registration_profiles (id, user_id, registration_ip, registration_country, registration_region, registration_user_agent)
		VALUES ('route-ops-profile', ?, '127.0.0.1', 'China', 'Shanghai', 'agent')
	`, seedUserID); err != nil {
		t.Fatalf("insert registration profile: %v", err)
	}
	seedRouteCheckoutProduct(t, appDB, "route-ops-product", "prod_route_ops")
	if _, err := appDB.ExecP(`
		INSERT INTO orders (
			id, user_id, product_id, amount, status, subscription_status, subscription_canceled_at, created_at
		) VALUES (?, ?, ?, 9900, 'paid', 'canceled', ?, ?)
	`, "route-ops-order", seedUserID, "route-ops-product", timefmt.SQLiteDateTime(now.Add(-time.Hour)), timefmt.SQLiteDateTime(now.AddDate(0, 0, -1))); err != nil {
		t.Fatalf("insert operations order: %v", err)
	}

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/analytics/operations?period=7d&country=China", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	fwcontext.SetCurrentUser(c, &models.User{
		ID:      seedUserID,
		Name:    "Admin",
		IsAdmin: 1,
		Role:    "platform_admin",
	})

	if err := routes.GetOperationsAnalytics(c); err != nil {
		t.Fatalf("get operations analytics: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool                               `json:"success"`
		Data    routes.OperationsAnalyticsResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success {
		t.Fatalf("expected success envelope, got %s", rec.Body.String())
	}
	if envelope.Data.Summary.NewUsers.Value != 1 || envelope.Data.Summary.PaidRevenueAmount.Value != 9900 {
		t.Fatalf("unexpected operations summary: %#v", envelope.Data.Summary)
	}
	if envelope.Data.Geo.SelectedCountry != "China" || len(envelope.Data.Geo.Countries) == 0 || len(envelope.Data.Geo.Regions) == 0 {
		t.Fatalf("unexpected geo response: %#v", envelope.Data.Geo)
	}
	if len(envelope.Data.RegistrationHours) != 24 {
		t.Fatalf("expected 24 hour buckets, got %#v", envelope.Data.RegistrationHours)
	}
}

func TestLegacyUserOrdersAccessRejectsCrossUser(t *testing.T) {
	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/orders/user/user-2", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	c.SetParamNames("user_id")
	c.SetParamValues("user-2")
	fwcontext.SetCurrentUser(c, &models.User{ID: "user-1", Name: "Ada", IsAdmin: 0})

	called := false
	err := routes.RequireLegacyUserOrdersAccess(func(c echo.Context) error {
		called = true
		return nil
	})(c)
	if err != nil {
		t.Fatalf("legacy owner guard: %v", err)
	}
	if called {
		t.Fatalf("expected cross-user legacy request to be blocked")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}
}

func TestGetUserOrdersRejectsInvalidPageQuery(t *testing.T) {
	setupRouteTestDBs(t)

	const seedUserID = "019ea0c1-0001-7000-8000-000000000001"
	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/orders/user/"+seedUserID+"?page=0&page_size=10", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	c.SetParamNames("user_id")
	c.SetParamValues(seedUserID)
	fwcontext.SetCurrentUser(c, &models.User{ID: seedUserID, Name: "Ada", IsAdmin: 0})

	if err := routes.GetUserOrders(c); err != nil {
		t.Fatalf("get user orders: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	body := strings.TrimSpace(rec.Body.String())
	if !strings.Contains(body, `"success":false`) || !strings.Contains(body, `"code":"validation"`) {
		t.Fatalf("expected validation envelope, got %s", body)
	}
}

func TestCreateOrderAcceptsSelectedProductLedgerRequest(t *testing.T) {
	setupRouteTestDBs(t)

	const seedUserID = "019ea0c1-0001-7000-8000-000000000001"
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedRouteCheckoutProduct(t, appDB, "route-product", "prod_route")

	router := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/orders", strings.NewReader(`{"user_id":"`+seedUserID+`","product_id":"route-product"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	fwcontext.SetCurrentUser(c, &models.User{ID: seedUserID, Name: "Ada", IsAdmin: 0})

	if err := routes.CreateOrder(c); err != nil {
		t.Fatalf("create order: %v", err)
	}

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool                       `json:"success"`
		Data    routes.CreateOrderResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success {
		t.Fatalf("expected success envelope, got %s", rec.Body.String())
	}
	if envelope.Data.Order.ID == "" {
		t.Fatalf("expected created order id")
	}
	if envelope.Data.Order.Amount != 0 || envelope.Data.Order.Status != "pending" {
		t.Fatalf("unexpected created order: %#v", envelope.Data.Order)
	}
	if envelope.Data.Order.ProductID != "route-product" || envelope.Data.Order.ProductName != "Route Product" {
		t.Fatalf("expected selected product in order response, got %#v", envelope.Data.Order)
	}
}

func TestCreateOrderRejectsCrossUserForNonAdmin(t *testing.T) {
	setupRouteTestDBs(t)

	const currentUserID = "019ea0c1-0001-7000-8000-000000000001"
	const requestUserID = "019ea0c1-0002-7000-8000-000000000002"
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	ensureRouteTestUser(t, appDB, requestUserID)
	seedRouteCheckoutProduct(t, appDB, "route-product", "prod_route")

	router := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/orders", strings.NewReader(`{"user_id":"`+requestUserID+`","product_id":"route-product"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	fwcontext.SetCurrentUser(c, &models.User{ID: currentUserID, Name: "Ada", IsAdmin: 0})

	if err := routes.CreateOrder(c); err != nil {
		t.Fatalf("create order: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}
}

func TestCreateMyOrderUsesCurrentUserInsteadOfRequestUserID(t *testing.T) {
	setupRouteTestDBs(t)

	const currentUserID = "019ea0c1-0001-7000-8000-000000000001"
	const requestUserID = "019ea0c1-0002-7000-8000-000000000002"
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	ensureRouteTestUser(t, appDB, requestUserID)
	seedRouteCheckoutProduct(t, appDB, "route-product", "prod_route")

	router := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", strings.NewReader(`{"user_id":"`+requestUserID+`","product_id":"route-product"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	fwcontext.SetCurrentUser(c, &models.User{ID: currentUserID, Name: "Ada", IsAdmin: 0})

	if err := routes.CreateMyOrder(c); err != nil {
		t.Fatalf("create my order: %v", err)
	}

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool                       `json:"success"`
		Data    routes.CreateOrderResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.Order.UserID != currentUserID {
		t.Fatalf("expected order owner to come from current user, got %#v", envelope.Data.Order)
	}
}

func TestGetOrderDetailRejectsCrossUserRoute(t *testing.T) {
	setupRouteTestDBs(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	const ownerUserID = "019ea0c1-0001-7000-8000-000000000001"
	const otherUserID = "019ea0c1-0002-7000-8000-000000000002"
	ensureRouteTestUser(t, appDB, ownerUserID)
	ensureRouteTestUser(t, appDB, otherUserID)
	seedRouteCheckoutProduct(t, appDB, "route-sec-product", "prod_route_sec")
	seedRouteSecurityOrder(t, appDB, "route-sec-order", ownerUserID, "route-sec-product")

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/orders/route-sec-order", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("route-sec-order")
	fwcontext.SetCurrentUser(c, &models.User{ID: otherUserID, Name: "Mallory", IsAdmin: 0})

	if err := routes.GetOrderDetail(c); err != nil {
		t.Fatalf("get order detail: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}
}

func TestPayOrderRejectsUserRoute(t *testing.T) {
	setupRouteTestDBs(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	const ownerUserID = "019ea0c1-0001-7000-8000-000000000001"
	ensureRouteTestUser(t, appDB, ownerUserID)
	seedRouteCheckoutProduct(t, appDB, "route-pay-product", "prod_route_pay")
	seedRouteSecurityOrder(t, appDB, "route-pay-order", ownerUserID, "route-pay-product")

	router := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/orders/route-pay-order/pay", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("route-pay-order")
	fwcontext.SetCurrentUser(c, &models.User{ID: ownerUserID, Name: "Ada", IsAdmin: 0})

	if err := routes.PayOrder(c); err != nil {
		t.Fatalf("pay order: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
	}

	var status string
	if err := appDB.GetP(&status, `SELECT status FROM orders WHERE id = ?`, "route-pay-order"); err != nil {
		t.Fatalf("load order status: %v", err)
	}
	if status != "pending" {
		t.Fatalf("expected order to remain pending, got %q", status)
	}
}

func seedRouteUserOrdersForPagination(t *testing.T, appDB *sqlx.Engine, userID string, count int) {
	seedRouteUserOrdersForPaginationWithPrefix(t, appDB, userID, "route-order", count)
}

func seedRouteUserOrdersForPaginationWithPrefix(t *testing.T, appDB *sqlx.Engine, userID string, idPrefix string, count int) {
	t.Helper()

	query := `INSERT INTO orders (id, user_id, amount, status, created_at) VALUES (?, ?, ?, ?, ?)`
	for i := 1; i <= count; i++ {
		_, err := appDB.ExecP(query,
			fmt.Sprintf("%s-%02d", idPrefix, i),
			userID,
			int64(i*100),
			"pending",
			fmt.Sprintf("2026-01-01 00:00:%02d", i),
		)
		if err != nil {
			t.Fatalf("insert order %d: %v", i, err)
		}
	}
}

func seedRouteCheckoutProduct(t *testing.T, appDB *sqlx.Engine, productID string, creemProductID string) {
	t.Helper()

	if _, err := appDB.ExecP(`
		INSERT INTO products (
			id, name, description, price, currency, stock, enabled, creem_product_id,
			billing_type, membership_level, subscription_interval, created_at, updated_at
		) VALUES (?, 'Route Product', 'Route checkout product', 1000, 'USD', 0, 1, ?, 'subscription', 'premium', 'month', '2026-01-01 00:00:00', '2026-01-01 00:00:00')
	`, productID, creemProductID); err != nil {
		t.Fatalf("insert route product: %v", err)
	}
}

func ensureRouteTestUser(t *testing.T, appDB *sqlx.Engine, userID string) {
	t.Helper()

	if _, err := appDB.ExecP(`
		INSERT OR IGNORE INTO users (id, name, email, password_hash, email_verified, is_active)
		VALUES (?, ?, ?, '', 1, 1)
	`, userID, "Route Test User", userID+"@example.com"); err != nil {
		t.Fatalf("insert route test user: %v", err)
	}
}

func seedRouteSecurityOrder(t *testing.T, appDB *sqlx.Engine, orderID string, userID string, productID string) {
	t.Helper()

	if _, err := appDB.ExecP(`
		INSERT INTO orders (id, user_id, product_id, amount, status, created_at)
		VALUES (?, ?, ?, 1000, 'pending', '2026-01-01 00:00:00')
	`, orderID, userID, productID); err != nil {
		t.Fatalf("insert route security order: %v", err)
	}
}
