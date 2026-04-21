package models

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/zachatrocity/htmx-hyperscript-starter/api/db"
)

// User 用户模型
type User struct {
	ID        string `json:"id" db:"id"`
	Name      string `json:"name" db:"name"`
	Email     string `json:"email" db:"email"`
	CreatedAt string `json:"created_at,omitempty" db:"created_at"`
	UpdatedAt string `json:"updated_at,omitempty" db:"updated_at"`
}

// UserQuery 用户查询参数（用于动态条件查询）
type UserQuery struct {
	ID    string `db:"id"`
	Name  string `db:"name"`
	Email string `db:"email"`
}

// CreateUser 创建新用户
func CreateUser(user *User) error {
	if user.ID == "" {
		user.ID = uuid.New().String()
	}
	user.CreatedAt = time.Now().Format("2006-01-02 15:04:05")
	user.UpdatedAt = user.CreatedAt

	// 使用命名参数插入
	sql := `INSERT INTO users (id, name, email, created_at, updated_at) VALUES (:id, :name, :email, :created_at, :updated_at)`
	_, err := db.GetEngine().Exec(context.Background(), sql, user)
	if err != nil {
		return fmt.Errorf("创建用户失败: %w", err)
	}
	return nil
}

// GetUserByID 根据 ID 获取用户
func GetUserByID(id string) (*User, error) {
	sql := `SELECT * FROM users WHERE id = :id`
	var user User
	err := db.GetEngine().Get(context.Background(), &user, sql, map[string]interface{}{
		"id": id,
	})
	if err != nil {
		return nil, fmt.Errorf("获取用户失败: %w", err)
	}
	return &user, nil
}

// GetUserByEmail 根据邮箱获取用户
func GetUserByEmail(email string) (*User, error) {
	sql := `SELECT * FROM users WHERE email = :email`
	var user User
	err := db.GetEngine().Get(context.Background(), &user, sql, map[string]interface{}{
		"email": email,
	})
	if err != nil {
		return nil, fmt.Errorf("获取用户失败: %w", err)
	}
	return &user, nil
}

// GetAllUsers 获取所有用户
func GetAllUsers() ([]User, error) {
	sql := `SELECT * FROM users ORDER BY created_at DESC`
	var users []User
	err := db.GetDB().Select(&users, sql)
	if err != nil {
		return nil, fmt.Errorf("获取用户列表失败: %w", err)
	}
	return users, nil
}

// UpdateUser 更新用户信息（动态更新，只更新传入的非空字段）
func UpdateUser(user *User) error {
	user.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
	sql := `
	UPDATE users SET
		updated_at = :updated_at
		#[ , name = :name ]
		#[ , email = :email ]
	WHERE id = :id
	`
	result, err := db.GetEngine().Exec(context.Background(), sql, user)
	if err != nil {
		return fmt.Errorf("更新用户失败: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("用户不存在")
	}
	return nil
}

// DeleteUser 删除用户
func DeleteUser(id string) error {
	sql := `DELETE FROM users WHERE id = :id`
	result, err := db.GetEngine().Exec(context.Background(), sql, map[string]interface{}{
		"id": id,
	})
	if err != nil {
		return fmt.Errorf("删除用户失败: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("用户不存在")
	}
	return nil
}

// FindUsers 动态条件查询用户（支持多条件组合）
func FindUsers(query UserQuery) ([]User, error) {
	sql := `
	SELECT * FROM users
	WHERE 1=1
		#[ AND id = :id ]
		#[ AND name LIKE :name ]
		#[ AND email LIKE :email ]
	ORDER BY created_at DESC
	`
	var users []User
	err := db.GetEngine().Select(context.Background(), &users, sql, query)
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	return users, nil
}
