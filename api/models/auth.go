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
	"github.com/zachatrocity/htmx-hyperscript-starter/api/db"
	"golang.org/x/crypto/bcrypt"
)

// Session 用户会话
type Session struct {
	ID        string `json:"id" db:"id"`
	UserID    string `json:"user_id" db:"user_id"`
	ExpiresAt string `json:"expires_at" db:"expires_at"`
	CreatedAt string `json:"created_at" db:"created_at"`
}

// PasswordReset 密码重置记录
type PasswordReset struct {
	ID        string `json:"id" db:"id"`
	UserID    string `json:"user_id" db:"user_id"`
	TokenHash string `json:"-" db:"token_hash"`
	ExpiresAt string `json:"expires_at" db:"expires_at"`
	UsedAt    string `json:"used_at,omitempty" db:"used_at"`
	CreatedAt string `json:"created_at" db:"created_at"`
}

// 密码哈希配置
const bcryptCost = 12

// Session 有效期（7天）
const sessionDuration = 7 * 24 * time.Hour

// 密码重置 Token 有效期（15分钟）
const resetTokenDuration = 15 * time.Minute

// ========== 密码相关方法 ==========

// SetPassword 设置用户密码（哈希后存储）
func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return fmt.Errorf("密码哈希失败: %w", err)
	}
	u.PasswordHash = string(hash)
	return nil
}

// CheckPassword 验证密码
func (u *User) CheckPassword(password string) bool {
	if u.PasswordHash == "" {
		return false
	}
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}

// ========== Session CRUD ==========

// CreateSession 创建新会话
func CreateSession(userID string) (*Session, error) {
	// 生成随机 session token 作为 ID
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("生成会话 Token 失败: %w", err)
	}
	sessionID := hex.EncodeToString(tokenBytes)

	expiresAt := time.Now().Add(sessionDuration).Format("2006-01-02 15:04:05")

	session := &Session{
		ID:        sessionID,
		UserID:    userID,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	sql := `INSERT INTO sessions (id, user_id, expires_at, created_at) VALUES (:id, :user_id, :expires_at, :created_at)`
	_, err := db.GetEngine().Exec(context.Background(), sql, session)
	if err != nil {
		return nil, fmt.Errorf("创建会话失败: %w", err)
	}

	return session, nil
}

// GetSessionByID 根据 ID 获取会话
func GetSessionByID(sessionID string) (*Session, error) {
	sql := `SELECT * FROM sessions WHERE id = :id AND expires_at > datetime('now')`
	var session Session
	err := db.GetEngine().Get(context.Background(), &session, sql, map[string]interface{}{
		"id": sessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("会话不存在或已过期: %w", err)
	}
	return &session, nil
}

// DeleteSession 删除会话（登出）
func DeleteSession(sessionID string) error {
	sql := `DELETE FROM sessions WHERE id = :id`
	_, err := db.GetEngine().Exec(context.Background(), sql, map[string]interface{}{
		"id": sessionID,
	})
	return err
}

// DeleteUserSessions 删除用户所有会话
func DeleteUserSessions(userID string) error {
	sql := `DELETE FROM sessions WHERE user_id = :user_id`
	_, err := db.GetEngine().Exec(context.Background(), sql, map[string]interface{}{
		"user_id": userID,
	})
	return err
}

// CleanExpiredSessions 清理过期会话
func CleanExpiredSessions() error {
	sql := `DELETE FROM sessions WHERE expires_at < datetime('now')`
	_, err := db.GetEngine().Exec(context.Background(), sql, map[string]interface{}{})
	return err
}

// ========== 密码重置 ==========

// CreatePasswordReset 创建密码重置 Token
func CreatePasswordReset(userID string) (string, error) {
	// 生成随机 token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("生成重置 Token 失败: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	// SHA256 哈希存储
	tokenHash := sha256.Sum256([]byte(token))
	tokenHashStr := hex.EncodeToString(tokenHash[:])

	expiresAt := time.Now().Add(resetTokenDuration).Format("2006-01-02 15:04:05")

	reset := &PasswordReset{
		ID:        uuid.New().String(),
		UserID:    userID,
		TokenHash: tokenHashStr,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	sql := `INSERT INTO password_resets (id, user_id, token_hash, expires_at, created_at) VALUES (:id, :user_id, :token_hash, :expires_at, :created_at)`
	_, err := db.GetEngine().Exec(context.Background(), sql, reset)
	if err != nil {
		return "", fmt.Errorf("创建重置记录失败: %w", err)
	}

	return token, nil
}

// VerifyPasswordResetToken 验证密码重置 Token
func VerifyPasswordResetToken(token string) (*PasswordReset, error) {
	// 计算 token hash
	tokenHash := sha256.Sum256([]byte(token))
	tokenHashStr := hex.EncodeToString(tokenHash[:])

	sql := `
		SELECT * FROM password_resets
		WHERE token_hash = :token_hash
		  AND expires_at > datetime('now')
		  AND used_at IS NULL
	`
	var reset PasswordReset
	err := db.GetEngine().Get(context.Background(), &reset, sql, map[string]interface{}{
		"token_hash": tokenHashStr,
	})
	if err != nil {
		return nil, fmt.Errorf("重置链接无效或已过期: %w", err)
	}
	return &reset, nil
}

// MarkPasswordResetUsed 标记重置 Token 已使用
func MarkPasswordResetUsed(resetID string) error {
	sql := `UPDATE password_resets SET used_at = datetime('now') WHERE id = :id`
	_, err := db.GetEngine().Exec(context.Background(), sql, map[string]interface{}{
		"id": resetID,
	})
	return err
}

// UpdateUserPassword 更新用户密码
func UpdateUserPassword(userID string, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return fmt.Errorf("密码哈希失败: %w", err)
	}

	sql := `UPDATE users SET password_hash = :password_hash, updated_at = datetime('now') WHERE id = :id`
	_, err = db.GetEngine().Exec(context.Background(), sql, map[string]interface{}{
		"id":           userID,
		"password_hash": string(hash),
	})
	return err
}

// ========== 用户认证相关查询 ==========

// GetUserWithPasswordByEmail 根据邮箱获取用户（包含密码哈希）
func GetUserWithPasswordByEmail(email string) (*User, error) {
	query := `SELECT id, name, email, password_hash, created_at, updated_at, email_verified, is_active FROM users WHERE email = ?`
	var user User
	err := db.GetDB().Get(&user, query, email)
	if err != nil {
		// sql.ErrNoRows 表示未找到记录
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("获取用户失败: %w", err)
	}
	return &user, nil
}

// ActivateUser 激活用户
func ActivateUser(userID string) error {
	sql := `UPDATE users SET is_active = 1, email_verified = 1, updated_at = datetime('now') WHERE id = :id`
	_, err := db.GetEngine().Exec(context.Background(), sql, map[string]interface{}{
		"id": userID,
	})
	return err
}

// UserExistsByEmail 检查邮箱是否已注册
func UserExistsByEmail(email string) (bool, error) {
	sql := `SELECT COUNT(*) FROM users WHERE email = :email`
	var count int
	err := db.GetEngine().Get(context.Background(), &count, sql, map[string]interface{}{
		"email": email,
	})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
