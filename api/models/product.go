package models

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	"github.com/tfnick/go-svelte-starter/api/framework/data/namelookup"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
)

type Product struct {
	ID                   string `json:"id" db:"id"`
	Name                 string `json:"name" db:"name"`
	Description          string `json:"description,omitempty" db:"description"`
	Price                int64  `json:"price" db:"price"`
	Currency             string `json:"currency" db:"currency"`
	Stock                int    `json:"stock" db:"stock"`
	Enabled              int    `json:"enabled" db:"enabled"`
	CreemProductID       string `json:"creem_product_id" db:"creem_product_id"`
	BillingType          string `json:"billing_type" db:"billing_type"`
	MembershipLevel      string `json:"membership_level" db:"membership_level"`
	SubscriptionInterval string `json:"subscription_interval" db:"subscription_interval"`
	CreatedAt            string `json:"created_at,omitempty" db:"created_at"`
	UpdatedAt            string `json:"updated_at,omitempty" db:"updated_at"`
}

func CreateProduct(ctx context.Context, product *Product) error {
	if product.ID == "" {
		product.ID = uuid.Must(uuid.NewV7()).String()
	}
	product.CreatedAt = timefmt.NowSQLiteDateTime()
	product.UpdatedAt = product.CreatedAt

	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	query := `
		INSERT INTO products (
			id, name, description, price, currency, stock, enabled, creem_product_id,
			billing_type, membership_level, subscription_interval, created_at, updated_at
		) VALUES (
			:id, :name, :description, :price, :currency, :stock, :enabled, :creem_product_id,
			:billing_type, :membership_level, :subscription_interval, :created_at, :updated_at
		)
	`
	if _, err := d.ExecNamed(query, product); err != nil {
		return fmt.Errorf("create product failed: %w", err)
	}
	return nil
}

func GetProductByID(ctx context.Context, id string) (*Product, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var product Product
	query := `SELECT * FROM products WHERE id = ?`
	err = d.GetP(&product, query, id)
	if err != nil {
		return nil, fmt.Errorf("get product failed: %w", err)
	}
	return &product, nil
}

func ListProducts(ctx context.Context) ([]Product, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var products []Product
	if err := d.SelectP(&products, `SELECT * FROM products ORDER BY created_at DESC, id DESC`); err != nil {
		return nil, fmt.Errorf("list products failed: %w", err)
	}
	return products, nil
}

func UpdateProduct(ctx context.Context, product *Product) error {
	product.UpdatedAt = timefmt.NowSQLiteDateTime()

	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	query := `
		UPDATE products SET
			name = ?,
			description = ?,
			price = ?,
			currency = ?,
			stock = ?,
			enabled = ?,
			creem_product_id = ?,
			billing_type = ?,
			membership_level = ?,
			subscription_interval = ?,
			updated_at = ?
		WHERE id = ?
	`
	result, err := d.ExecP(query,
		product.Name,
		product.Description,
		product.Price,
		product.Currency,
		product.Stock,
		product.Enabled,
		product.CreemProductID,
		product.BillingType,
		product.MembershipLevel,
		product.SubscriptionInterval,
		product.UpdatedAt,
		product.ID,
	)
	if err != nil {
		return fmt.Errorf("update product failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("product not found: %w", modelerror.ErrNotFound)
	}
	return nil
}

func GetProductDisplayNamesByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	uniqueIDs := namelookup.UniqueNonEmpty(ids)
	if len(uniqueIDs) == 0 {
		return map[string]string{}, nil
	}

	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	type query struct {
		IDs []string `db:"ids"`
	}

	var rows []namelookup.Row
	if err := eng.Select(&rows, `SELECT id, name FROM products WHERE id IN :ids`, query{IDs: uniqueIDs}); err != nil {
		return nil, fmt.Errorf("query product names failed: %w", err)
	}
	return namelookup.RowsToMap(rows), nil
}

func UpdateProductStock(ctx context.Context, productID string, newStock int) error {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	query := `UPDATE products SET stock = ? WHERE id = ?`
	_, err = d.ExecP(query, newStock, productID)
	return err
}
