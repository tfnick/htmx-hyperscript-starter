package models

import (
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

// CreateUser 创建新用户
func CreateUser(user *User) error {
	if user.ID == "" {
		user.ID = uuid.New().String()
	}
	user.CreatedAt = time.Now().Format("2006-01-02 15:04:05")
	user.UpdatedAt = user.CreatedAt

	query := `INSERT INTO users (id, name, email, created_at, updated_at) VALUES (:id, :name, :email, :created_at, :updated_at)`
	_, err := db.GetDB().NamedExec(query, user)
	if err != nil {
		return fmt.Errorf("创建用户失败: %w", err)
	}
	return nil
}

// GetUserByID 根据 ID 获取用户
func GetUserByID(id string) (*User, error) {
	var user User
	query := `SELECT * FROM users WHERE id = ?`
	err := db.GetDB().Get(&user, query, id)
	if err != nil {
		return nil, fmt.Errorf("获取用户失败: %w", err)
	}
	return &user, nil
}

// GetUserByEmail 根据邮箱获取用户
func GetUserByEmail(email string) (*User, error) {
	var user User
	query := `SELECT * FROM users WHERE email = ?`
	err := db.GetDB().Get(&user, query, email)
	if err != nil {
		return nil, fmt.Errorf("获取用户失败: %w", err)
	}
	return &user, nil
}

// GetAllUsers 获取所有用户
func GetAllUsers() ([]User, error) {
	var users []User
	query := `SELECT * FROM users ORDER BY created_at DESC`
	err := db.GetDB().Select(&users, query)
	if err != nil {
		return nil, fmt.Errorf("获取用户列表失败: %w", err)
	}
	return users, nil
}

// UpdateUser 更新用户信息
func UpdateUser(user *User) error {
	user.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
	query := `UPDATE users SET name = :name, email = :email, updated_at = :updated_at WHERE id = :id`
	result, err := db.GetDB().NamedExec(query, user)
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
	query := `DELETE FROM users WHERE id = ?`
	result, err := db.GetDB().Exec(query, id)
	if err != nil {
		return fmt.Errorf("删除用户失败: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("用户不存在")
	}
	return nil
}
