package models

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/tfnick/go-svelte-starter/api/db"
)

type OpenAPIAccountReadModel struct {
	AccountID     string `db:"id"`
	ExternalRef   string `db:"external_ref"`
	Name          string `db:"name"`
	Email         string `db:"email"`
	EmailVerified int    `db:"email_verified"`
	IsActive      int    `db:"is_active"`
	CreatedAt     string `db:"created_at"`
}

func GetOpenAPIAccountByConsumerAccountID(ctx context.Context, accountID string) (*OpenAPIAccountReadModel, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	query := `
		SELECT
			u.id,
			'' AS external_ref,
			u.name,
			u.email,
			u.email_verified,
			u.is_active,
			u.created_at
		FROM users u
		WHERE u.id = ?
	`

	var account OpenAPIAccountReadModel
	if err := d.GetP(&account, query, accountID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query open api account failed: %w", err)
	}
	return &account, nil
}

func GetOpenAPIAccountStatus(isActive int) string {
	if isActive == 1 {
		return "active"
	}
	return "inactive"
}
