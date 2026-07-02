package models

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12
const resetTokenDuration = 15 * time.Minute

type PasswordReset struct {
	ID        string `json:"id" db:"id"`
	UserID    string `json:"user_id" db:"user_id"`
	TokenHash string `json:"-" db:"token_hash"`
	ExpiresAt string `json:"expires_at" db:"expires_at"`
	UsedAt    string `json:"used_at,omitempty" db:"used_at"`
	CreatedAt string `json:"created_at" db:"created_at"`
}

func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return fmt.Errorf("hash password failed: %w", err)
	}
	u.PasswordHash = string(hash)
	return nil
}

func (u *User) CheckPassword(password string) bool {
	if u.PasswordHash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) == nil
}

func CreatePasswordReset(ctx context.Context, userID string) (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generate reset token failed: %w", err)
	}

	token := hex.EncodeToString(tokenBytes)
	tokenHash := sha256.Sum256([]byte(token))

	reset := &PasswordReset{
		ID:        uuid.Must(uuid.NewV7()).String(),
		UserID:    userID,
		TokenHash: hex.EncodeToString(tokenHash[:]),
		ExpiresAt: timefmt.SQLiteDateTime(timefmt.NowUTC().Add(resetTokenDuration)),
		CreatedAt: timefmt.NowSQLiteDateTime(),
	}

	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return "", fmt.Errorf("database unavailable: %w", err)
	}

	sql := `INSERT INTO password_resets (id, user_id, token_hash, expires_at, created_at) VALUES (:id, :user_id, :token_hash, :expires_at, :created_at)`
	if _, err := eng.ExecNamed(sql, reset); err != nil {
		return "", fmt.Errorf("create password reset failed: %w", err)
	}

	return token, nil
}

func VerifyPasswordResetToken(ctx context.Context, token string) (*PasswordReset, error) {
	tokenHash := sha256.Sum256([]byte(token))

	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	now := timefmt.NowSQLiteDateTime()
	sql := `
		SELECT * FROM password_resets
		WHERE token_hash = :token_hash
		  AND expires_at > :now
		  AND used_at IS NULL
	`
	var reset PasswordReset
	err = eng.GetNamed(&reset, sql, map[string]interface{}{
		"token_hash": hex.EncodeToString(tokenHash[:]),
		"now":        now,
	})
	if err != nil {
		return nil, fmt.Errorf("reset token invalid or expired: %w", err)
	}
	return &reset, nil
}

func MarkPasswordResetUsed(ctx context.Context, resetID string) error {
	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	now := timefmt.NowSQLiteDateTime()
	sql := `UPDATE password_resets SET used_at = :now WHERE id = :id`
	_, err = eng.ExecNamed(sql, map[string]interface{}{
		"id":  resetID,
		"now": now,
	})
	return err
}

func UpdateUserPassword(ctx context.Context, userID string, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return fmt.Errorf("hash password failed: %w", err)
	}

	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	now := timefmt.NowSQLiteDateTime()
	sql := `UPDATE users SET password_hash = :password_hash, updated_at = :now WHERE id = :id`
	_, err = eng.ExecNamed(sql, map[string]interface{}{
		"id":            userID,
		"password_hash": string(hash),
		"now":           now,
	})
	return err
}

func GetUserWithPasswordByEmail(ctx context.Context, email string) (*User, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	query := `
		SELECT id, name, email, password_hash, created_at, updated_at, email_verified, is_active, is_admin, role, organization_id, membership_level, membership_expires_at
		FROM users
		WHERE email = ?
	`
	var user User
	err = d.GetP(&user, query, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get user with password failed: %w", err)
	}
	return &user, nil
}

func ActivateUser(ctx context.Context, userID string) error {
	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	now := timefmt.NowSQLiteDateTime()
	sql := `UPDATE users SET is_active = 1, email_verified = 1, updated_at = :now WHERE id = :id`
	_, err = eng.ExecNamed(sql, map[string]interface{}{
		"id":  userID,
		"now": now,
	})
	return err
}

func UserExistsByEmail(ctx context.Context, email string) (bool, error) {
	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return false, fmt.Errorf("database unavailable: %w", err)
	}

	sql := `SELECT COUNT(*) FROM users WHERE email = :email`
	var count int
	err = eng.GetNamed(&count, sql, map[string]interface{}{
		"email": email,
	})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
