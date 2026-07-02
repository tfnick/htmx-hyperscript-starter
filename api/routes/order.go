package routes

import (
	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	"github.com/tfnick/go-svelte-starter/api/framework/http/middleware"
	fwrequest "github.com/tfnick/go-svelte-starter/api/framework/http/request"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

type CreateOrderRequest struct {
	UserID    string                   `json:"user_id"`
	ProductID string                   `json:"product_id"`
	Items     []CreateOrderItemRequest `json:"items"`
}

type CreateOrderItemRequest struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

type UpdateOrderStatusRequest struct {
	Status string `json:"status"`
}

type OrderResponse struct {
	ID                     string `json:"id"`
	UserID                 string `json:"user_id"`
	UserName               string `json:"user_name"`
	ProductID              string `json:"product_id"`
	ProductName            string `json:"product_name"`
	Amount                 int64  `json:"amount"`
	Status                 string `json:"status"`
	ProviderCheckoutID     string `json:"provider_checkout_id,omitempty"`
	ProviderOrderID        string `json:"provider_order_id,omitempty"`
	ProviderCustomerID     string `json:"provider_customer_id,omitempty"`
	ProviderSubscriptionID string `json:"provider_subscription_id,omitempty"`
	ProviderProductID      string `json:"provider_product_id,omitempty"`
	SubscriptionStatus     string `json:"subscription_status,omitempty"`
	MembershipAppliedAt    string `json:"membership_applied_at,omitempty"`
	CreatedAt              string `json:"created_at,omitempty"`
}

type OrderItemResponse struct {
	ID          string `json:"id"`
	OrderID     string `json:"order_id"`
	ProductID   string `json:"product_id"`
	ProductName string `json:"product_name"`
	Quantity    int    `json:"quantity"`
	Price       int64  `json:"price"`
}

type CreateOrderResponse struct {
	Message string        `json:"message"`
	Order   OrderResponse `json:"order"`
}

type PayOrderResponse struct {
	Message string        `json:"message"`
	Order   OrderResponse `json:"order"`
}

type EnqueueOrderExportResponse struct {
	TaskID  string `json:"task_id"`
	Message string `json:"message"`
}

type OrderDetailResponse struct {
	Order OrderResponse       `json:"order"`
	Items []OrderItemResponse `json:"items"`
}

type PaginationResponse struct {
	Page        int  `json:"page"`
	PageSize    int  `json:"page_size"`
	TotalItems  int  `json:"total_items"`
	TotalPages  int  `json:"total_pages"`
	HasPrevious bool `json:"has_previous"`
	HasNext     bool `json:"has_next"`
}

type UserOrdersResponse struct {
	Items      []OrderResponse    `json:"items"`
	Pagination PaginationResponse `json:"pagination"`
}

func ToOrderResponse(order usecase.OrderCo) OrderResponse {
	return OrderResponse{
		ID:                     order.ID,
		UserID:                 order.UserID,
		UserName:               order.UserName,
		ProductID:              order.ProductID,
		ProductName:            order.ProductName,
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

func ToOrderResponses(orders []usecase.OrderCo) []OrderResponse {
	responses := make([]OrderResponse, 0, len(orders))
	for i := range orders {
		responses = append(responses, ToOrderResponse(orders[i]))
	}
	return responses
}

func ToOrderItemResponse(item usecase.OrderItemCo) OrderItemResponse {
	return OrderItemResponse{
		ID:          item.ID,
		OrderID:     item.OrderID,
		ProductID:   item.ProductID,
		ProductName: item.ProductName,
		Quantity:    item.Quantity,
		Price:       item.Price,
	}
}

func ToOrderItemResponses(items []usecase.OrderItemCo) []OrderItemResponse {
	responses := make([]OrderItemResponse, 0, len(items))
	for i := range items {
		responses = append(responses, ToOrderItemResponse(items[i]))
	}
	return responses
}

func ToOrderDetailResponse(detail usecase.OrderDetailCo) OrderDetailResponse {
	return OrderDetailResponse{
		Order: ToOrderResponse(detail.Order),
		Items: ToOrderItemResponses(detail.Items),
	}
}

func ToPaginationResponse(page fwusecase.PageResult) PaginationResponse {
	return PaginationResponse{
		Page:        page.Page,
		PageSize:    page.PageSize,
		TotalItems:  page.TotalItems,
		TotalPages:  page.TotalPages,
		HasPrevious: page.HasPrevious,
		HasNext:     page.HasNext,
	}
}

func ToUserOrdersResponse(orders usecase.UserOrdersCo) UserOrdersResponse {
	return UserOrdersResponse{
		Items:      ToOrderResponses(orders.Items),
		Pagination: ToPaginationResponse(orders.Pagination),
	}
}

// CreateOrder creates a pending local order ledger for provider checkout.
func CreateOrder(c echo.Context) error {
	var req CreateOrderRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return httpresponse.Unauthorized(c, "not logged in")
	}
	if currentUser.IsAdmin != 1 && currentUser.ID != req.UserID {
		return httpresponse.Forbidden(c, "cannot create order for another user")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	order, err := usecase.CreateOrder(ctx, usecase.CreateOrderCmd{
		UserID:    req.UserID,
		ProductID: req.ProductID,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.Created(c, CreateOrderResponse{
		Message: "order created",
		Order:   ToOrderResponse(order),
	})
}

// CreateMyOrder creates a pending order for the current authenticated user.
func CreateMyOrder(c echo.Context) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return httpresponse.Unauthorized(c, "not logged in")
	}

	var req CreateOrderRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	order, err := usecase.CreateOrder(ctx, usecase.CreateOrderCmd{
		UserID:    currentUser.ID,
		ProductID: req.ProductID,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.Created(c, CreateOrderResponse{
		Message: "order created",
		Order:   ToOrderResponse(order),
	})
}

// PayOrder marks an order as paid and runs payment side effects in the same transaction.
func PayOrder(c echo.Context) error {
	orderID := c.Param("id")

	ctx := fwcontext.InternalUsecaseContext(c)
	order, err := usecase.PayOrder(ctx, usecase.PayOrderCmd{OrderID: orderID})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OK(c, PayOrderResponse{
		Message: "order paid",
		Order:   ToOrderResponse(order),
	})
}

// GetUserOrders returns paginated orders for a user.
func GetUserOrders(c echo.Context) error {
	userID := c.Param("user_id")
	page := fwrequest.PageQuery(c)
	ctx := fwcontext.InternalUsecaseContext(c)
	orders, err := usecase.GetUserOrders(ctx, usecase.UserOrdersQry{
		UserID:   userID,
		Page:     page.Page,
		PageSize: page.PageSize,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OK(c, ToUserOrdersResponse(orders))
}

// ListMyOrders returns paginated orders owned by the current authenticated user.
func ListMyOrders(c echo.Context) error {
	page := fwrequest.PageQuery(c)
	ctx := fwcontext.InternalUsecaseContext(c)
	orders, err := usecase.ListMyOrders(ctx, usecase.ListMyOrdersQry{
		Status:   c.QueryParam("status"),
		Page:     page.Page,
		PageSize: page.PageSize,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OK(c, ToUserOrdersResponse(orders))
}

// ListAdminOrders returns paginated orders across users for admin operators.
func ListAdminOrders(c echo.Context) error {
	page := fwrequest.PageQuery(c)
	ctx := fwcontext.InternalUsecaseContext(c)
	orders, err := usecase.ListAdminOrders(ctx, usecase.ListAdminOrdersQry{
		UserID:    c.QueryParam("user_id"),
		UserQuery: c.QueryParam("user_query"),
		Status:    c.QueryParam("status"),
		StartTime: c.QueryParam("start_time"),
		EndTime:   c.QueryParam("end_time"),
		Page:      page.Page,
		PageSize:  page.PageSize,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OK(c, ToUserOrdersResponse(orders))
}

// GetOrderAnalytics keeps the legacy path as an order-list compatibility alias.
func GetOrderAnalytics(c echo.Context) error {
	page := fwrequest.PageQuery(c)
	ctx := fwcontext.InternalUsecaseContext(c)
	orders, err := usecase.ListAdminOrders(ctx, usecase.ListAdminOrdersQry{
		UserID:    c.QueryParam("user_id"),
		UserQuery: c.QueryParam("user_query"),
		Status:    c.QueryParam("status"),
		StartTime: c.QueryParam("start_time"),
		EndTime:   c.QueryParam("end_time"),
		Page:      page.Page,
		PageSize:  page.PageSize,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OK(c, ToUserOrdersResponse(orders))
}

// EnqueueMyOrdersExport queues an Excel export for the current user's own orders.
func EnqueueMyOrdersExport(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	result, err := usecase.EnqueueMyOrdersExcelExport(ctx, usecase.EnqueueMyOrdersExcelExportQry{
		Status: c.QueryParam("status"),
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.Created(c, EnqueueOrderExportResponse{
		TaskID:  result.TaskID,
		Message: "order export task enqueued",
	})
}

// EnqueueAdminOrdersExport queues an Excel export for the admin order view.
func EnqueueAdminOrdersExport(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	result, err := usecase.EnqueueAdminOrdersExcelExport(ctx, usecase.EnqueueAdminOrdersExcelExportQry{
		UserID: c.QueryParam("user_id"),
		Status: c.QueryParam("status"),
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.Created(c, EnqueueOrderExportResponse{
		TaskID:  result.TaskID,
		Message: "order export task enqueued",
	})
}

// RequireLegacyUserOrdersAccess keeps the old user-id path safe during migration.
func RequireLegacyUserOrdersAccess(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		currentUser := middleware.GetCurrentUser(c)
		if currentUser == nil {
			return httpresponse.Unauthorized(c, "not logged in")
		}
		if currentUser.IsAdmin != 1 && currentUser.ID != c.Param("user_id") {
			return httpresponse.Forbidden(c, "cannot view another user's orders")
		}
		return next(c)
	}
}

// GetOrderDetail returns an order and its items.
func GetOrderDetail(c echo.Context) error {
	orderID := c.Param("id")

	ctx := fwcontext.InternalUsecaseContext(c)
	detail, err := usecase.GetOrderDetail(ctx, usecase.OrderDetailQry{OrderID: orderID})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OK(c, ToOrderDetailResponse(detail))
}

// UpdateOrderStatus updates an order status.
func UpdateOrderStatus(c echo.Context) error {
	orderID := c.Param("id")
	var req UpdateOrderStatusRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	if err := usecase.UpdateOrderStatus(ctx, usecase.UpdateOrderStatusCmd{
		OrderID: orderID,
		Status:  req.Status,
	}); err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OKMessage(c, "order status updated")
}
