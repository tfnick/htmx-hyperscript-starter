package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/zachatrocity/htmx-hyperscript-starter/api/db"
)

// Order 订单模型
type Order struct {
	ID        string `json:"id" db:"id"`
	UserID    string `json:"user_id" db:"user_id"`
	Amount    int64  `json:"amount" db:"amount"`
	Status    string `json:"status" db:"status"`
	CreatedAt string `json:"created_at,omitempty" db:"created_at"`
}

// OrderItem 订单项模型
type OrderItem struct {
	ID        string `json:"id" db:"id"`
	OrderID   string `json:"order_id" db:"order_id"`
	ProductID string `json:"product_id" db:"product_id"`
	Quantity  int    `json:"quantity" db:"quantity"`
	Price     int64  `json:"price" db:"price"`
}

// Product 产品模型
type Product struct {
	ID          string `json:"id" db:"id"`
	Name        string `json:"name" db:"name"`
	Description string `json:"description,omitempty" db:"description"`
	Price       int64  `json:"price" db:"price"`
	Stock       int    `json:"stock" db:"stock"`
	CreatedAt   string `json:"created_at,omitempty" db:"created_at"`
}

// CreateOrder 创建订单（复杂事务示例）
// 在一个事务中完成：创建订单主表 → 创建订单项 → 扣减库存
func CreateOrder(userID string, items []OrderItem) (*Order, error) {
	var order *Order

	// 使用事务封装函数
	err := db.WithTransaction(func(tx *sqlx.Tx) error {
		// 1. 创建订单主表记录
		order = &Order{
			ID:     uuid.New().String(),
			UserID: userID,
			Status: "pending",
		}

		// 计算订单总金额
		var totalAmount int64
		for i := range items {
			items[i].ID = uuid.New().String()
			items[i].OrderID = order.ID
			totalAmount += items[i].Price * int64(items[i].Quantity)
		}
		order.Amount = totalAmount
		order.CreatedAt = time.Now().Format("2006-01-02 15:04:05")

		insertOrderSQL := `INSERT INTO orders (id, user_id, amount, status, created_at) VALUES (:id, :user_id, :amount, :status, :created_at)`
		if _, err := tx.NamedExec(insertOrderSQL, order); err != nil {
			return fmt.Errorf("创建订单失败: %w", err)
		}

		// 2. 批量插入订单项
		insertItemSQL := `INSERT INTO order_items (id, order_id, product_id, quantity, price) VALUES (:id, :order_id, :product_id, :quantity, :price)`
		if _, err := tx.NamedExec(insertItemSQL, items); err != nil {
			return fmt.Errorf("创建订单项失败: %w", err)
		}

		// 3. 扣减库存（带乐观锁检查）
		for _, item := range items {
			result, err := tx.Exec(`
				UPDATE products 
				SET stock = stock - ? 
				WHERE id = ? AND stock >= ?
			`, item.Quantity, item.ProductID, item.Quantity)
			if err != nil {
				return fmt.Errorf("扣减库存失败: %w", err)
			}
			rowsAffected, _ := result.RowsAffected()
			if rowsAffected == 0 {
				return fmt.Errorf("库存不足或产品不存在: %s", item.ProductID)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return order, nil
}

// GetOrdersByUserID 获取用户的订单列表
func GetOrdersByUserID(userID string) ([]Order, error) {
	var orders []Order
	query := `SELECT * FROM orders WHERE user_id = ? ORDER BY created_at DESC`
	err := db.GetDB().Select(&orders, query, userID)
	if err != nil {
		return nil, fmt.Errorf("获取订单列表失败: %w", err)
	}
	return orders, nil
}

// GetOrderItems 获取订单的所有项
func GetOrderItems(orderID string) ([]OrderItem, error) {
	var items []OrderItem
	query := `SELECT * FROM order_items WHERE order_id = ?`
	err := db.GetDB().Select(&items, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("获取订单项失败: %w", err)
	}
	return items, nil
}

// UpdateOrderStatus 更新订单状态
func UpdateOrderStatus(orderID string, status string) error {
	query := `UPDATE orders SET status = ? WHERE id = ?`
	result, err := db.GetDB().Exec(query, status, orderID)
	if err != nil {
		return fmt.Errorf("更新订单状态失败: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("订单不存在")
	}
	return nil
}

// CreateProduct 创建产品
func CreateProduct(product *Product) error {
	if product.ID == "" {
		product.ID = uuid.New().String()
	}
	product.CreatedAt = time.Now().Format("2006-01-02 15:04:05")
	query := `INSERT INTO products (id, name, description, price, stock, created_at) VALUES (:id, :name, :description, :price, :stock, :created_at)`
	_, err := db.GetDB().NamedExec(query, product)
	return err
}

// GetProductByID 获取产品
func GetProductByID(id string) (*Product, error) {
	var product Product
	query := `SELECT * FROM products WHERE id = ?`
	err := db.GetDB().Get(&product, query, id)
	if err != nil {
		return nil, fmt.Errorf("获取产品失败: %w", err)
	}
	return &product, nil
}

// UpdateProductStock 更新产品库存
func UpdateProductStock(productID string, newStock int) error {
	query := `UPDATE products SET stock = ? WHERE id = ?`
	_, err := db.GetDB().Exec(query, newStock, productID)
	return err
}
