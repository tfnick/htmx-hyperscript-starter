package usecase

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	"github.com/tfnick/go-svelte-starter/api/framework/data/namelookup"
	"github.com/tfnick/go-svelte-starter/api/framework/events"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
	usecaseevents "github.com/tfnick/go-svelte-starter/api/usecase/events"
	"github.com/tfnick/go-svelte-starter/api/usecase/translate"
)

const (
	OrderSubscriptionStatusActive   = "active"
	OrderSubscriptionStatusCanceled = "canceled"
)

type CreateOrderCmd struct {
	UserID    string
	ProductID string
	Items     []CreateOrderItemCmd
}

type CreateOrderItemCmd struct {
	ProductID string
	Quantity  int
}

type UserOrdersQry struct {
	UserID   string
	Page     int
	PageSize int
}

type ListMyOrdersQry struct {
	Status   string
	Page     int
	PageSize int
}

type ListAdminOrdersQry struct {
	UserID    string
	UserQuery string
	Status    string
	StartTime string
	EndTime   string
	Page      int
	PageSize  int
}

type OrderDetailQry struct {
	OrderID string
}

type UpdateOrderStatusCmd struct {
	OrderID string
	Status  string
}

type PayOrderCmd struct {
	OrderID                  string
	ProviderCheckoutID       string
	ProviderOrderID          string
	ProviderCustomerID       string
	ProviderSubscriptionID   string
	ProviderProductID        string
	ProviderSubscriptionHint string
}

type ApplyOrderMembershipCmd struct {
	OrderID string
}

type ApplyOrderMembershipCo struct {
	UserID              string
	MembershipLevel     string
	MembershipExpiresAt string
}

type CancelOrderSubscriptionCmd struct {
	OrderID                 string
	ProviderSubscriptionID  string
	ProviderSubscriptionRef string
}

type OrderCo struct {
	ID                     string
	UserID                 string
	UserName               string
	ProductID              string
	ProductName            string
	Amount                 int64
	Status                 string
	ProviderCheckoutID     string
	ProviderOrderID        string
	ProviderCustomerID     string
	ProviderSubscriptionID string
	ProviderProductID      string
	SubscriptionStatus     string
	MembershipAppliedAt    string
	CreatedAt              string
}

type OrderItemCo struct {
	ID          string
	OrderID     string
	ProductID   string
	ProductName string
	Quantity    int
	Price       int64
}

type OrderDetailCo struct {
	Order OrderCo
	Items []OrderItemCo
}

type UserOrdersCo struct {
	Items      []OrderCo
	Pagination fwusecase.PageResult
}

func CreateOrder(ctx fwusecase.Context, cmd CreateOrderCmd) (OrderCo, error) {
	if strings.TrimSpace(cmd.UserID) == "" {
		return OrderCo{}, fwusecase.E(fwusecase.CodeValidation, "user ID is required", nil)
	}
	if strings.TrimSpace(cmd.ProductID) == "" {
		return OrderCo{}, fwusecase.E(fwusecase.CodeValidation, "product ID is required", nil)
	}

	product, err := loadCheckoutProduct(ctx, cmd.ProductID)
	if err != nil {
		return OrderCo{}, err
	}

	user, err := models.GetUserByID(ctx.Std(), cmd.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OrderCo{}, fwusecase.E(fwusecase.CodeValidation, "user not found", err)
		}
		return OrderCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load user", err)
	}
	if product.MembershipLevel != "" && product.MembershipLevel != MembershipLevelBasic {
		effectiveLevel, _ := EffectiveMembership(user)
		if effectiveLevel != MembershipLevelBasic && membershipLevelRank(effectiveLevel) >= membershipLevelRank(product.MembershipLevel) {
			return OrderCo{}, fwusecase.E(fwusecase.CodeConflict, "cannot purchase this product while current membership is active", nil)
		}
	}

	var order *models.Order
	err = fwusecase.WithAppTx(ctx, func(txCtx fwusecase.Context) error {
		createdOrder, err := models.InsertOrderWithProduct(txCtx.Std(), cmd.UserID, product.ID, 0)
		if err != nil {
			return err
		}
		order = createdOrder
		return nil
	})
	if err != nil {
		return OrderCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to create order", err)
	}

	names, err := resolveOrderNames(ctx, []models.Order{*order}, nil)
	if err != nil {
		return OrderCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load order display names", err)
	}

	return orderCoFromModel(order, names), nil
}

func GetUserOrders(ctx fwusecase.Context, qry UserOrdersQry) (UserOrdersCo, error) {
	userID := strings.TrimSpace(qry.UserID)
	if userID == "" {
		return UserOrdersCo{}, fwusecase.E(fwusecase.CodeValidation, "user ID is required", nil)
	}
	if !ctx.Actor.Authenticated {
		return UserOrdersCo{}, fwusecase.E(fwusecase.CodeUnauthorized, "not logged in", nil)
	}
	if !ctx.Actor.IsAdmin && ctx.Actor.UserID != userID {
		return UserOrdersCo{}, fwusecase.E(fwusecase.CodeForbidden, "cannot view another user's orders", nil)
	}

	return listOrders(ctx, models.OrderQuery{UserID: userID}, qry.Page, qry.PageSize)
}

func ListMyOrders(ctx fwusecase.Context, qry ListMyOrdersQry) (UserOrdersCo, error) {
	if !ctx.Actor.Authenticated || strings.TrimSpace(ctx.Actor.UserID) == "" {
		return UserOrdersCo{}, fwusecase.E(fwusecase.CodeUnauthorized, "not logged in", nil)
	}

	return listOrders(ctx, models.OrderQuery{
		UserID: strings.TrimSpace(ctx.Actor.UserID),
		Status: strings.TrimSpace(qry.Status),
	}, qry.Page, qry.PageSize)
}

func ListAdminOrders(ctx fwusecase.Context, qry ListAdminOrdersQry) (UserOrdersCo, error) {
	if !ctx.Actor.Authenticated {
		return UserOrdersCo{}, fwusecase.E(fwusecase.CodeUnauthorized, "not logged in", nil)
	}
	if !ctx.Actor.IsAdmin {
		return UserOrdersCo{}, fwusecase.E(fwusecase.CodeForbidden, "admin access is required", nil)
	}

	query, err := adminOrderQueryFromQry(qry)
	if err != nil {
		return UserOrdersCo{}, err
	}

	return listOrders(ctx, query, qry.Page, qry.PageSize)
}

func adminOrderQueryFromQry(qry ListAdminOrdersQry) (models.OrderQuery, error) {
	startTime, err := normalizeUserListTime(qry.StartTime, "start_time")
	if err != nil {
		return models.OrderQuery{}, err
	}
	endTime, err := normalizeUserListTime(qry.EndTime, "end_time")
	if err != nil {
		return models.OrderQuery{}, err
	}
	if startTime != "" && endTime != "" && startTime > endTime {
		return models.OrderQuery{}, fwusecase.E(fwusecase.CodeValidation, "start_time must be before end_time", nil)
	}
	return models.OrderQuery{
		UserID:    strings.TrimSpace(qry.UserID),
		UserQuery: normalizeLikeQuery(qry.UserQuery),
		Status:    strings.TrimSpace(qry.Status),
		StartAt:   startTime,
		EndAt:     endTime,
	}, nil
}

type analyticsPeriod struct {
	Key           string
	Label         string
	CurrentStart  time.Time
	CurrentEnd    time.Time
	PreviousStart time.Time
	PreviousEnd   time.Time
	YoYStart      time.Time
	YoYEnd        time.Time
}

func normalizeAnalyticsPeriod(value string, monthInput string, now time.Time) (analyticsPeriod, error) {
	key := strings.TrimSpace(value)
	if key == "" {
		key = "7d"
	}
	now = now.UTC().Truncate(time.Second)

	if key == "month" {
		monthStr := strings.TrimSpace(monthInput)
		if monthStr == "" {
			return analyticsPeriod{}, fwusecase.E(fwusecase.CodeValidation, "month parameter is required when period is 'month'", nil)
		}
		parsed, err := time.ParseInLocation("2006-01", monthStr, time.UTC)
		if err != nil {
			return analyticsPeriod{}, fwusecase.E(fwusecase.CodeValidation, "month parameter must be in YYYY-MM format", nil)
		}
		monthStart := parsed
		monthEnd := monthStart.AddDate(0, 1, 0).Add(-time.Second)
		label := monthStart.Format("January 2006")

		// MoM: previous calendar month
		previousMonthStart := monthStart.AddDate(0, -1, 0)
		previousMonthEnd := previousMonthStart.AddDate(0, 1, 0).Add(-time.Second)

		// YoY: same month previous year
		yoyMonthStart := monthStart.AddDate(-1, 0, 0)
		yoyMonthEnd := yoyMonthStart.AddDate(0, 1, 0).Add(-time.Second)

		return analyticsPeriod{
			Key:           key,
			Label:         label,
			CurrentStart:  monthStart,
			CurrentEnd:    monthEnd,
			PreviousStart: previousMonthStart,
			PreviousEnd:   previousMonthEnd,
			YoYStart:      yoyMonthStart,
			YoYEnd:        yoyMonthEnd,
		}, nil
	}

	var start time.Time
	var previousStart time.Time
	var label string
	switch key {
	case "7d":
		label = "Past 7 days"
		start = now.AddDate(0, 0, -7)
		previousStart = start.AddDate(0, 0, -7)
	case "1m":
		label = "Past 1 month"
		start = addDateClamped(now, 0, -1, 0)
		previousStart = addDateClamped(start, 0, -1, 0)
	case "3m":
		label = "Past 3 months"
		start = addDateClamped(now, 0, -3, 0)
		previousStart = addDateClamped(start, 0, -3, 0)
	default:
		return analyticsPeriod{}, fwusecase.E(fwusecase.CodeValidation, "analytics period is invalid", nil)
	}
	return analyticsPeriod{
		Key:           key,
		Label:         label,
		CurrentStart:  start,
		CurrentEnd:    now,
		PreviousStart: previousStart,
		PreviousEnd:   start.Add(-time.Second),
		YoYStart:      start.AddDate(-1, 0, 0),
		YoYEnd:        now.AddDate(-1, 0, 0),
	}, nil
}

func percentChangeFloat(current float64, previous float64) *float64 {
	if previous == 0 {
		if current == 0 {
			value := 0.0
			return &value
		}
		return nil
	}
	value := (current - previous) * 100 / previous
	return &value
}

func listOrders(ctx fwusecase.Context, query models.OrderQuery, page int, pageSize int) (UserOrdersCo, error) {
	if query.Status != "" && !isValidOrderStatus(query.Status) {
		return UserOrdersCo{}, fwusecase.E(fwusecase.CodeValidation, "invalid order status", nil)
	}

	pageQuery, err := fwusecase.NormalizePageQuery(fwusecase.PageQuery{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return UserOrdersCo{}, err
	}
	query.Limit = pageQuery.Limit()
	query.Offset = pageQuery.Offset()

	total, err := models.CountOrders(ctx.Std(), query)
	if err != nil {
		return UserOrdersCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to count orders", err)
	}

	orders, err := models.ListOrders(ctx.Std(), query)
	if err != nil {
		return UserOrdersCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load orders", err)
	}

	names, err := resolveOrderNames(ctx, orders, nil)
	if err != nil {
		return UserOrdersCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load order display names", err)
	}

	responses := make([]OrderCo, 0, len(orders))
	for i := range orders {
		responses = append(responses, orderCoFromModel(&orders[i], names))
	}
	return UserOrdersCo{
		Items:      responses,
		Pagination: fwusecase.NewPageResult(pageQuery, total),
	}, nil
}

func normalizeLikeQuery(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `%`, `\%`)
	escaped = strings.ReplaceAll(escaped, `_`, `\_`)
	return "%" + escaped + "%"
}

func GetOrderDetail(ctx fwusecase.Context, qry OrderDetailQry) (OrderDetailCo, error) {
	if qry.OrderID == "" {
		return OrderDetailCo{}, fwusecase.E(fwusecase.CodeValidation, "order ID is required", nil)
	}

	order, err := models.GetOrderByID(ctx.Std(), qry.OrderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OrderDetailCo{}, fwusecase.E(fwusecase.CodeNotFound, "order not found", err)
		}
		return OrderDetailCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load order", err)
	}

	if err := requireOrderAccess(ctx, order.UserID, "cannot view another user's order"); err != nil {
		return OrderDetailCo{}, err
	}

	items, err := models.GetOrderItems(ctx.Std(), qry.OrderID)
	if err != nil {
		return OrderDetailCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load order items", err)
	}

	names, err := resolveOrderNames(ctx, []models.Order{*order}, items)
	if err != nil {
		return OrderDetailCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load order display names", err)
	}

	itemResponses := make([]OrderItemCo, 0, len(items))
	for i := range items {
		itemResponses = append(itemResponses, orderItemCoFromModel(&items[i], names))
	}
	return OrderDetailCo{
		Order: orderCoFromModel(order, names),
		Items: itemResponses,
	}, nil
}

func UpdateOrderStatus(ctx fwusecase.Context, cmd UpdateOrderStatusCmd) error {
	if cmd.OrderID == "" {
		return fwusecase.E(fwusecase.CodeValidation, "order ID is required", nil)
	}
	if !isValidOrderStatus(cmd.Status) {
		return fwusecase.E(fwusecase.CodeValidation, "invalid order status", nil)
	}
	if cmd.Status == "paid" {
		return fwusecase.E(fwusecase.CodeValidation, "use pay order endpoint to mark order paid", nil)
	}

	if err := models.UpdateOrderStatus(ctx.Std(), cmd.OrderID, cmd.Status); err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return fwusecase.E(fwusecase.CodeNotFound, "order not found", err)
		}
		return fwusecase.E(fwusecase.CodeInternal, "failed to update order status", err)
	}
	return nil
}

func PayOrder(ctx fwusecase.Context, cmd PayOrderCmd) (OrderCo, error) {
	if strings.TrimSpace(cmd.OrderID) == "" {
		return OrderCo{}, fwusecase.E(fwusecase.CodeValidation, "order ID is required", nil)
	}
	if err := requireSystemOrAdmin(ctx, "payment confirmation is restricted"); err != nil {
		return OrderCo{}, err
	}
	if len(events.Subscribers(usecaseevents.OrderPaidTopic)) == 0 {
		return OrderCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to pay order", usecaseevents.ErrOrderPaidPointsSubscriberMissing)
	}

	var paidOrder *models.Order
	err := fwusecase.WithAppTx(ctx, func(txCtx fwusecase.Context) error {
		order, err := models.GetOrderByID(txCtx.Std(), cmd.OrderID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fwusecase.E(fwusecase.CodeNotFound, "order not found", err)
			}
			return fwusecase.E(fwusecase.CodeInternal, "failed to load order", err)
		}

		refs := orderProviderRefsFromPayCmd(cmd)
		if refs.SubscriptionStatus == "" && refs.ProviderSubscriptionID != "" {
			refs.SubscriptionStatus = OrderSubscriptionStatusActive
		}
		if hasOrderProviderRefs(refs) {
			if err := models.UpdateOrderProviderRefs(txCtx.Std(), order.ID, refs); err != nil {
				return fwusecase.E(fwusecase.CodeInternal, "failed to update order payment references", err)
			}
			mergeOrderProviderRefs(order, refs)
		}

		if order.Status == "paid" {
			paidOrder = order
			return nil
		}
		if order.Status != "pending" {
			return fwusecase.E(fwusecase.CodeConflict, "only pending orders can be paid", nil)
		}

		if err := models.UpdateOrderStatus(txCtx.Std(), order.ID, "paid"); err != nil {
			if errors.Is(err, modelerror.ErrNotFound) {
				return fwusecase.E(fwusecase.CodeNotFound, "order not found", err)
			}
			return fwusecase.E(fwusecase.CodeInternal, "failed to update order status", err)
		}

		order.Status = "paid"
		paidOrder = order

		event, err := usecaseevents.NewOrderPaidEvent(order)
		if err != nil {
			return fwusecase.E(fwusecase.CodeInternal, "failed to build order paid event", err)
		}
		if err := events.Publish(txCtx, event); err != nil {
			return fwusecase.E(fwusecase.CodeInternal, "failed to queue payment side effects", err)
		}
		return nil
	})
	if err != nil {
		return OrderCo{}, err
	}
	if paidOrder == nil {
		return OrderCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to pay order", fmt.Errorf("paid order result is empty"))
	}

	names, err := resolveOrderNames(ctx, []models.Order{*paidOrder}, nil)
	if err != nil {
		return OrderCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load order display names", err)
	}
	return orderCoFromModel(paidOrder, names), nil
}

func requireOrderAccess(ctx fwusecase.Context, ownerUserID string, forbiddenMessage string) error {
	if ctx.Surface == fwusecase.SurfaceSystem {
		return nil
	}
	if !ctx.Actor.Authenticated || strings.TrimSpace(ctx.Actor.UserID) == "" {
		return fwusecase.E(fwusecase.CodeUnauthorized, "not logged in", nil)
	}
	if ctx.Actor.IsAdmin || strings.TrimSpace(ctx.Actor.UserID) == strings.TrimSpace(ownerUserID) {
		return nil
	}
	return fwusecase.E(fwusecase.CodeForbidden, forbiddenMessage, nil)
}

func requireSystemOrAdmin(ctx fwusecase.Context, forbiddenMessage string) error {
	if ctx.Surface == fwusecase.SurfaceSystem {
		return nil
	}
	if !ctx.Actor.Authenticated || strings.TrimSpace(ctx.Actor.UserID) == "" {
		return fwusecase.E(fwusecase.CodeUnauthorized, "not logged in", nil)
	}
	if ctx.Actor.IsAdmin {
		return nil
	}
	return fwusecase.E(fwusecase.CodeForbidden, forbiddenMessage, nil)
}

func ApplyOrderMembership(ctx fwusecase.Context, cmd ApplyOrderMembershipCmd) (ApplyOrderMembershipCo, bool, error) {
	if strings.TrimSpace(cmd.OrderID) == "" {
		return ApplyOrderMembershipCo{}, false, fwusecase.E(fwusecase.CodeValidation, "order ID is required", nil)
	}

	var result ApplyOrderMembershipCo
	applied := false
	err := fwusecase.WithAppTx(ctx, func(txCtx fwusecase.Context) error {
		order, err := models.GetOrderByID(txCtx.Std(), cmd.OrderID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fwusecase.E(fwusecase.CodeNotFound, "order not found", err)
			}
			return fwusecase.E(fwusecase.CodeInternal, "failed to load order", err)
		}
		if order.Status != "paid" {
			return fwusecase.E(fwusecase.CodeConflict, "only paid orders can grant membership", nil)
		}
		if strings.TrimSpace(order.MembershipAppliedAt) != "" {
			return nil
		}
		if strings.TrimSpace(order.ProductID) == "" {
			return fwusecase.E(fwusecase.CodeValidation, "order product is required", nil)
		}

		product, err := models.GetProductByID(txCtx.Std(), order.ProductID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fwusecase.E(fwusecase.CodeNotFound, "product not found", err)
			}
			return fwusecase.E(fwusecase.CodeInternal, "failed to load order product", err)
		}
		expiresAt, err := membershipExpiresAtForProduct(product)
		if err != nil {
			return err
		}
		if err := models.UpdateUserMembership(txCtx.Std(), order.UserID, product.MembershipLevel, expiresAt); err != nil {
			return fwusecase.E(fwusecase.CodeInternal, "failed to update user membership", err)
		}

		if product.MembershipLevel != "" && product.MembershipLevel != MembershipLevelBasic {
			oldSubscriptions, err := models.GetActiveSubscriptionOrdersByUserID(txCtx.Std(), order.UserID, order.ID)
			if err != nil {
				return fwusecase.E(fwusecase.CodeInternal, "failed to find old subscriptions", err)
			}
			for i := range oldSubscriptions {
				sub := &oldSubscriptions[i]
				if err := models.UpdateOrderSubscriptionStatus(txCtx.Std(), sub.ID, OrderSubscriptionStatusCanceled); err != nil {
					return fwusecase.E(fwusecase.CodeInternal, "failed to cancel old subscription", err)
				}
				if sub.ProviderSubscriptionID != "" {
					subID := sub.ProviderSubscriptionID
					if err := fwusecase.RegisterAfterCommit(txCtx, func(runCtx context.Context) {
						_ = cancelCreemSubscriptionByID(runCtx, subID)
					}); err != nil {
						return fwusecase.E(fwusecase.CodeInternal, "failed to register subscription cancellation", err)
					}
				}
			}
		}

		appliedAt := timefmt.NowSQLiteDateTime()
		if err := models.MarkOrderMembershipApplied(txCtx.Std(), order.ID, appliedAt); err != nil {
			return fwusecase.E(fwusecase.CodeInternal, "failed to mark membership fulfillment", err)
		}

		result = ApplyOrderMembershipCo{
			UserID:              order.UserID,
			MembershipLevel:     product.MembershipLevel,
			MembershipExpiresAt: expiresAt,
		}
		applied = true
		return nil
	})
	if err != nil {
		return ApplyOrderMembershipCo{}, false, err
	}
	return result, applied, nil
}

func CancelOrderSubscription(ctx fwusecase.Context, cmd CancelOrderSubscriptionCmd) error {
	orderID := strings.TrimSpace(cmd.OrderID)
	subscriptionID := firstNonEmptyString(cmd.ProviderSubscriptionID, cmd.ProviderSubscriptionRef)
	if orderID == "" && subscriptionID == "" {
		return fwusecase.E(fwusecase.CodeValidation, "subscription reference is required", nil)
	}

	err := fwusecase.WithAppTx(ctx, func(txCtx fwusecase.Context) error {
		if subscriptionID != "" {
			err := models.UpdateOrderSubscriptionStatusByProviderSubscriptionID(txCtx.Std(), subscriptionID, OrderSubscriptionStatusCanceled)
			if err == nil {
				return nil
			}
			if !errors.Is(err, modelerror.ErrNotFound) || orderID == "" {
				return err
			}
		}
		return models.UpdateOrderSubscriptionStatus(txCtx.Std(), orderID, OrderSubscriptionStatusCanceled)
	})
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return fwusecase.E(fwusecase.CodeNotFound, "order subscription not found", err)
		}
		return fwusecase.E(fwusecase.CodeInternal, "failed to update order subscription", err)
	}
	return nil
}

func loadCheckoutProduct(ctx fwusecase.Context, productID string) (*models.Product, error) {
	product, err := models.GetProductByID(ctx.Std(), productID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fwusecase.E(fwusecase.CodeValidation, "product not found", err)
		}
		return nil, fwusecase.E(fwusecase.CodeInternal, "failed to load product", err)
	}
	if product.Enabled != 1 {
		return nil, fwusecase.E(fwusecase.CodeValidation, "product is disabled", nil)
	}
	if strings.TrimSpace(product.CreemProductID) == "" {
		return nil, fwusecase.E(fwusecase.CodeValidation, "product Creem ID is required", nil)
	}
	return product, nil
}

func membershipLevelRank(level string) int {
	switch level {
	case MembershipLevelSuper:
		return 3
	case MembershipLevelPremium:
		return 2
	default:
		return 1
	}
}

func EffectiveMembership(user *models.User) (level string, expiresAt string) {
	if expires, err := parseSQLiteTime(user.MembershipExpiresAt); err == nil && expires.Before(time.Now().UTC()) {
		return MembershipLevelBasic, PermanentMembershipExpiresAt
	}
	return user.MembershipLevel, user.MembershipExpiresAt
}

func membershipExpiresAtForProduct(product *models.Product) (string, error) {
	if product.BillingType == ProductBillingTypeOneTime {
		return PermanentMembershipExpiresAt, nil
	}
	if product.BillingType != ProductBillingTypeSubscription {
		return "", fwusecase.E(fwusecase.CodeValidation, "invalid product billing type", nil)
	}

	base := timefmt.NowUTC()

	switch product.SubscriptionInterval {
	case SubscriptionIntervalMonth:
		return timefmt.SQLiteDateTime(addDateClamped(base, 0, 1, 0)), nil
	case SubscriptionIntervalThreeMonths:
		return timefmt.SQLiteDateTime(addDateClamped(base, 0, 3, 0)), nil
	case SubscriptionIntervalSixMonths:
		return timefmt.SQLiteDateTime(addDateClamped(base, 0, 6, 0)), nil
	case SubscriptionIntervalYear:
		return timefmt.SQLiteDateTime(addDateClamped(base, 1, 0, 0)), nil
	default:
		return "", fwusecase.E(fwusecase.CodeValidation, "invalid subscription interval", nil)
	}
}

func addDateClamped(value time.Time, years int, months int, days int) time.Time {
	targetYear := value.Year() + years
	targetMonth := int(value.Month()) + months
	for targetMonth > 12 {
		targetYear++
		targetMonth -= 12
	}
	for targetMonth < 1 {
		targetYear--
		targetMonth += 12
	}

	lastDay := daysInMonth(targetYear, time.Month(targetMonth))
	targetDay := value.Day()
	if targetDay > lastDay {
		targetDay = lastDay
	}
	return time.Date(
		targetYear,
		time.Month(targetMonth),
		targetDay,
		value.Hour(),
		value.Minute(),
		value.Second(),
		value.Nanosecond(),
		value.Location(),
	).AddDate(0, 0, days)
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func parseSQLiteTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	if parsed, err := time.ParseInLocation(timefmt.SQLiteDateTimeLayout, value, time.UTC); err == nil {
		return parsed, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC(), nil
	}
	return time.Parse(time.RFC3339Nano, value)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func orderProviderRefsFromPayCmd(cmd PayOrderCmd) models.OrderProviderRefs {
	return models.OrderProviderRefs{
		ProviderCheckoutID:     strings.TrimSpace(cmd.ProviderCheckoutID),
		ProviderOrderID:        strings.TrimSpace(cmd.ProviderOrderID),
		ProviderCustomerID:     strings.TrimSpace(cmd.ProviderCustomerID),
		ProviderSubscriptionID: strings.TrimSpace(cmd.ProviderSubscriptionID),
		ProviderProductID:      strings.TrimSpace(cmd.ProviderProductID),
		SubscriptionStatus:     strings.TrimSpace(cmd.ProviderSubscriptionHint),
	}
}

func hasOrderProviderRefs(refs models.OrderProviderRefs) bool {
	return refs.ProviderCheckoutID != "" ||
		refs.ProviderOrderID != "" ||
		refs.ProviderCustomerID != "" ||
		refs.ProviderSubscriptionID != "" ||
		refs.ProviderProductID != "" ||
		refs.SubscriptionStatus != ""
}

func mergeOrderProviderRefs(order *models.Order, refs models.OrderProviderRefs) {
	if refs.ProviderCheckoutID != "" {
		order.ProviderCheckoutID = refs.ProviderCheckoutID
	}
	if refs.ProviderOrderID != "" {
		order.ProviderOrderID = refs.ProviderOrderID
	}
	if refs.ProviderCustomerID != "" {
		order.ProviderCustomerID = refs.ProviderCustomerID
	}
	if refs.ProviderSubscriptionID != "" {
		order.ProviderSubscriptionID = refs.ProviderSubscriptionID
	}
	if refs.ProviderProductID != "" {
		order.ProviderProductID = refs.ProviderProductID
	}
	if refs.SubscriptionStatus != "" {
		order.SubscriptionStatus = refs.SubscriptionStatus
	}
}

func isValidOrderStatus(status string) bool {
	switch status {
	case "pending", "paid", "shipped", "completed", "cancelled":
		return true
	default:
		return false
	}
}

func resolveOrderNames(ctx fwusecase.Context, orders []models.Order, items []models.OrderItem) (namelookup.Result, error) {
	return translate.Resolve(ctx.Std(), func(batch *namelookup.Batch) {
		namelookup.Collect(batch, translate.UserDisplayName, orders, func(order models.Order) string {
			return order.UserID
		})
		namelookup.Collect(batch, translate.ProductDisplayName, orders, func(order models.Order) string {
			return order.ProductID
		})
		namelookup.Collect(batch, translate.ProductDisplayName, items, func(item models.OrderItem) string {
			return item.ProductID
		})
	})
}

func orderCoFromModel(order *models.Order, names namelookup.Result) OrderCo {
	if order == nil {
		return OrderCo{}
	}

	return OrderCo{
		ID:                     order.ID,
		UserID:                 order.UserID,
		UserName:               names.Name(translate.UserDisplayName, order.UserID),
		ProductID:              order.ProductID,
		ProductName:            names.Name(translate.ProductDisplayName, order.ProductID),
		Amount:                 order.Amount,
		Status:                 order.Status,
		ProviderCheckoutID:     order.ProviderCheckoutID,
		ProviderOrderID:        order.ProviderOrderID,
		ProviderCustomerID:     order.ProviderCustomerID,
		ProviderSubscriptionID: order.ProviderSubscriptionID,
		ProviderProductID:      order.ProviderProductID,
		SubscriptionStatus:     order.SubscriptionStatus,
		MembershipAppliedAt:    order.MembershipAppliedAt,
		CreatedAt:              order.CreatedAt,
	}
}

func orderItemCoFromModel(item *models.OrderItem, names namelookup.Result) OrderItemCo {
	if item == nil {
		return OrderItemCo{}
	}

	return OrderItemCo{
		ID:          item.ID,
		OrderID:     item.OrderID,
		ProductID:   item.ProductID,
		ProductName: names.Name(translate.ProductDisplayName, item.ProductID),
		Quantity:    item.Quantity,
		Price:       item.Price,
	}
}
