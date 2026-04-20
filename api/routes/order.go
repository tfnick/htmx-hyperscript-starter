package routes

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/zachatrocity/htmx-hyperscript-starter/api/models"
)

// CreateOrderRequest 订单创建请求结构
type CreateOrderRequest struct {
	UserID string `json:"user_id"`
	Items  []struct {
		ProductID string `json:"product_id"`
		Quantity  int    `json:"quantity"`
	} `json:"items"`
}

// CreateOrder 创建订单处理函数（演示事务控制）
func CreateOrder(c echo.Context) error {
	var req CreateOrderRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "无效的请求数据",
		})
	}

	// 参数验证
	if req.UserID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "用户ID不能为空",
		})
	}
	if len(req.Items) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "订单项不能为空",
		})
	}

	// 转换订单项格式并获取产品信息
	orderItems := make([]models.OrderItem, 0, len(req.Items))
	for _, item := range req.Items {
		if item.Quantity <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "商品数量必须大于0",
			})
		}

		// 获取产品信息以获取价格
		product, err := models.GetProductByID(item.ProductID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "产品不存在: " + item.ProductID,
			})
		}

		orderItems = append(orderItems, models.OrderItem{
			ID:        uuid.New().String(),
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			Price:     product.Price,
		})
	}

	// 调用事务创建订单
	order, err := models.CreateOrder(req.UserID, orderItems)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"message": "订单创建成功",
		"order":   order,
	})
}

// GetUserOrders 获取用户的所有订单
func GetUserOrders(c echo.Context) error {
	userID := c.Param("user_id")
	if userID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "用户ID不能为空",
		})
	}

	orders, err := models.GetOrdersByUserID(userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, orders)
}

// GetOrderDetail 获取订单详情（包括订单项）
func GetOrderDetail(c echo.Context) error {
	orderID := c.Param("id")

	// 获取订单基本信息
	orders, err := models.GetOrdersByUserID("")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	var order *models.Order
	for _, o := range orders {
		if o.ID == orderID {
			order = &o
			break
		}
	}

	if order == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "订单不存在",
		})
	}

	// 获取订单项
	items, err := models.GetOrderItems(orderID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"order": order,
		"items": items,
	})
}

// UpdateOrderStatus 更新订单状态
func UpdateOrderStatus(c echo.Context) error {
	orderID := c.Param("id")
	var req struct {
		Status string `json:"status"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "无效的请求数据",
		})
	}

	validStatuses := map[string]bool{
		"pending":   true,
		"paid":      true,
		"shipped":   true,
		"completed": true,
		"cancelled": true,
	}
	if !validStatuses[req.Status] {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "无效的订单状态",
		})
	}

	if err := models.UpdateOrderStatus(orderID, req.Status); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "订单状态更新成功",
	})
}
