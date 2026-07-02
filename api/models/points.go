package models

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
)

const PointTransactionTypeOrderPaid = "order_paid"

type PointAccount struct {
	UserID    string `json:"user_id" db:"user_id"`
	Balance   int64  `json:"balance" db:"balance"`
	UpdatedAt string `json:"updated_at,omitempty" db:"updated_at"`
}

type PointTransaction struct {
	ID        string `json:"id" db:"id"`
	UserID    string `json:"user_id" db:"user_id"`
	OrderID   string `json:"order_id" db:"order_id"`
	Points    int64  `json:"points" db:"points"`
	Type      string `json:"type" db:"type"`
	CreatedAt string `json:"created_at,omitempty" db:"created_at"`
}

func GetPointBalance(ctx context.Context, userID string) (int64, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return 0, fmt.Errorf("database unavailable: %w", err)
	}

	var balance int64
	query := `SELECT balance FROM point_accounts WHERE user_id = ?`
	if err := d.GetP(&balance, query, userID); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, fmt.Errorf("get point balance failed: %w", err)
	}
	return balance, nil
}

func AwardOrderPaidPoints(ctx context.Context, userID string, orderID string, points int64) (int64, bool, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return 0, false, fmt.Errorf("database unavailable: %w", err)
	}

	upsertAccount := `INSERT OR IGNORE INTO point_accounts (user_id, balance, updated_at) VALUES (?, 0, ?)`
	now := timefmt.NowSQLiteDateTime()
	if _, err := d.ExecP(upsertAccount, userID, now); err != nil {
		return 0, false, fmt.Errorf("ensure point account failed: %w", err)
	}

	insertTx := `
		INSERT OR IGNORE INTO point_transactions (id, user_id, order_id, points, type, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	result, err := d.ExecP(insertTx, uuid.Must(uuid.NewV7()).String(), userID, orderID, points, PointTransactionTypeOrderPaid, now)
	if err != nil {
		return 0, false, fmt.Errorf("create point transaction failed: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, false, fmt.Errorf("read point transaction affected rows failed: %w", err)
	}
	if rows == 0 {
		balance, err := GetPointBalance(ctx, userID)
		return balance, false, err
	}

	updateAccount := `UPDATE point_accounts SET balance = balance + ?, updated_at = ? WHERE user_id = ?`
	if _, err := d.ExecP(updateAccount, points, now, userID); err != nil {
		return 0, false, fmt.Errorf("update point balance failed: %w", err)
	}

	balance, err := GetPointBalance(ctx, userID)
	return balance, true, err
}
