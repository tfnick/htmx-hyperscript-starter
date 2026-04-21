package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/zachatrocity/htmx-hyperscript-starter/api/db"
)

// Product 产品模型
type Product struct {
	ID          string `json:"id" db:"id"`
	Name        string `json:"name" db:"name"`
	Description string `json:"description,omitempty" db:"description"`
	Price       int64  `json:"price" db:"price"`
	Stock       int    `json:"stock" db:"stock"`
	CreatedAt   string `json:"created_at,omitempty" db:"created_at"`
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
