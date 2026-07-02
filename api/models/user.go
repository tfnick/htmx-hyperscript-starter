package models

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	"github.com/tfnick/go-svelte-starter/api/framework/data/namelookup"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
)

type User struct {
	ID                  string `json:"id" db:"id"`
	Name                string `json:"name" db:"name"`
	Email               string `json:"email" db:"email"`
	PasswordHash        string `json:"-" db:"password_hash"`
	EmailVerified       int    `json:"email_verified,omitempty" db:"email_verified"`
	IsActive            int    `json:"is_active,omitempty" db:"is_active"`
	IsAdmin             int    `json:"is_admin,omitempty" db:"is_admin"`
	Role                string `json:"role,omitempty" db:"role"`
	OrganizationID      string `json:"organization_id,omitempty" db:"organization_id"`
	MembershipLevel     string `json:"membership_level" db:"membership_level"`
	MembershipExpiresAt string `json:"membership_expires_at" db:"membership_expires_at"`
	CreatedAt           string `json:"created_at,omitempty" db:"created_at"`
	UpdatedAt           string `json:"updated_at,omitempty" db:"updated_at"`
}

type UserListItem struct {
	ID                    string `db:"id"`
	Name                  string `db:"name"`
	Email                 string `db:"email"`
	EmailVerified         int    `db:"email_verified"`
	IsActive              int    `db:"is_active"`
	IsAdmin               int    `db:"is_admin"`
	Role                  string `db:"role"`
	OrganizationID        string `db:"organization_id"`
	MembershipLevel       string `db:"membership_level"`
	MembershipExpiresAt   string `db:"membership_expires_at"`
	RegistrationIP        string `db:"registration_ip"`
	RegistrationCountry   string `db:"registration_country"`
	RegistrationRegion    string `db:"registration_region"`
	RegistrationUserAgent string `db:"registration_user_agent"`
	UtmSource             string `db:"utm_source"`
	UtmMedium             string `db:"utm_medium"`
	UtmCampaign           string `db:"utm_campaign"`
	CreatedAt             string `db:"created_at"`
	UpdatedAt             string `db:"updated_at"`
}

type UserQuery struct {
	ID    string `db:"id"`
	Name  string `db:"name"`
	Email string `db:"email"`
}

type UserListQuery struct {
	StartTime string `db:"start_time"`
	EndTime   string `db:"end_time"`
	Limit     int    `db:"limit"`
	Offset    int    `db:"offset"`
}

func CreateUser(ctx context.Context, user *User) error {
	if user.ID == "" {
		user.ID = uuid.Must(uuid.NewV7()).String()
	}
	user.CreatedAt = timefmt.NowSQLiteDateTime()
	user.UpdatedAt = user.CreatedAt

	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	sql := `INSERT INTO users (id, name, email, password_hash, created_at, updated_at) VALUES (:id, :name, :email, :password_hash, :created_at, :updated_at)`
	if _, err := eng.ExecNamed(sql, user); err != nil {
		return fmt.Errorf("create user failed: %w", err)
	}
	return nil
}

func CreateOAuthUser(ctx context.Context, user *User) error {
	if user.ID == "" {
		user.ID = uuid.Must(uuid.NewV7()).String()
	}
	if user.IsActive == 0 {
		user.IsActive = 1
	}
	user.EmailVerified = 1
	user.CreatedAt = timefmt.NowSQLiteDateTime()
	user.UpdatedAt = user.CreatedAt

	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	sql := `
		INSERT INTO users (
			id, name, email, password_hash, email_verified, is_active, created_at, updated_at
		) VALUES (
			:id, :name, :email, :password_hash, :email_verified, :is_active, :created_at, :updated_at
		)
	`
	if _, err := eng.ExecNamed(sql, user); err != nil {
		return fmt.Errorf("create oauth user failed: %w", err)
	}
	return nil
}

func GetUserByID(ctx context.Context, id string) (*User, error) {
	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var user User
	err = eng.GetP(&user, `SELECT * FROM users WHERE id = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("get user failed: %w", err)
	}
	return &user, nil
}

func GetUserByEmail(ctx context.Context, email string) (*User, error) {
	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var user User
	err = eng.GetP(&user, `SELECT * FROM users WHERE email = ?`, email)
	if err != nil {
		return nil, fmt.Errorf("get user failed: %w", err)
	}
	return &user, nil
}

func GetUserByEmailOptional(ctx context.Context, email string) (*User, error) {
	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var user User
	err = eng.GetP(&user, `SELECT * FROM users WHERE LOWER(email) = LOWER(?) LIMIT 1`, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get user by email failed: %w", err)
	}
	return &user, nil
}

func GetAllUsers(ctx context.Context) ([]User, error) {
	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	sql := `
		SELECT id, name, email, email_verified, is_active, is_admin, role, organization_id, membership_level, membership_expires_at, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
	`
	var users []User
	if err := eng.SelectP(&users, sql); err != nil {
		return nil, fmt.Errorf("get users failed: %w", err)
	}
	return users, nil
}

func CountUsers(ctx context.Context, query UserListQuery) (int, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return 0, fmt.Errorf("database unavailable: %w", err)
	}

	var count int
	if err := d.Get(&count, `
		SELECT COUNT(*)
		FROM users u
		WHERE 1=1
			#[ AND u.created_at >= :start_time ]
			#[ AND u.created_at <= :end_time ]
	`, query); err != nil {
		return 0, fmt.Errorf("count users failed: %w", err)
	}
	return count, nil
}

func ListUsers(ctx context.Context, query UserListQuery) ([]UserListItem, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	sql := `
		SELECT
			u.id,
			u.name,
			u.email,
			u.email_verified,
			u.is_active,
			u.is_admin,
			u.role,
			u.organization_id,
			u.membership_level,
			u.membership_expires_at,
			COALESCE(urp.registration_ip, '') AS registration_ip,
			COALESCE(urp.registration_country, '') AS registration_country,
			COALESCE(urp.registration_region, '') AS registration_region,
			COALESCE(urp.registration_user_agent, '') AS registration_user_agent,
			COALESCE(urp.utm_source, '') AS utm_source,
			COALESCE(urp.utm_medium, '') AS utm_medium,
			COALESCE(urp.utm_campaign, '') AS utm_campaign,
			u.created_at,
			u.updated_at
		FROM users u
		LEFT JOIN user_registration_profiles urp ON urp.user_id = u.id
		WHERE 1=1
			#[ AND u.created_at >= :start_time ]
			#[ AND u.created_at <= :end_time ]
		ORDER BY u.created_at DESC, u.id DESC
		LIMIT :limit OFFSET :offset
	`
	var users []UserListItem
	if err := d.Select(&users, sql, query); err != nil {
		return nil, fmt.Errorf("list users failed: %w", err)
	}
	return users, nil
}

func GetUserDisplayNamesByIDs(ctx context.Context, ids []string) (map[string]string, error) {
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
	if err := eng.Select(&rows, `SELECT id, name FROM users WHERE id IN :ids`, query{IDs: uniqueIDs}); err != nil {
		return nil, fmt.Errorf("query user names failed: %w", err)
	}
	return namelookup.RowsToMap(rows), nil
}

func UpdateUser(ctx context.Context, user *User) error {
	user.UpdatedAt = timefmt.NowSQLiteDateTime()

	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	sql := `
		UPDATE users SET
			updated_at = :updated_at
			#[ , name = :name ]
			#[ , email = :email ]
		WHERE id = :id
		`
	result, err := eng.Exec(sql, user)
	if err != nil {
		return fmt.Errorf("update user failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user not found: %w", modelerror.ErrNotFound)
	}
	return nil
}

func SetUserActive(ctx context.Context, id string, active bool) error {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	value := 0
	if active {
		value = 1
	}

	query := `UPDATE users SET is_active = ?, updated_at = ? WHERE id = ?`
	result, err := d.ExecP(query, value, timefmt.NowSQLiteDateTime(), id)
	if err != nil {
		return fmt.Errorf("set user active failed: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected rows failed: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("user not found: %w", modelerror.ErrNotFound)
	}
	return nil
}

func UpdateUserMembership(ctx context.Context, userID string, level string, expiresAt string) error {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	query := `
		UPDATE users
		SET membership_level = ?, membership_expires_at = ?, updated_at = ?
		WHERE id = ?
	`
	result, err := d.ExecP(query, level, expiresAt, timefmt.NowSQLiteDateTime(), userID)
	if err != nil {
		return fmt.Errorf("update user membership failed: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected rows failed: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("user not found: %w", modelerror.ErrNotFound)
	}
	return nil
}

func DeleteUser(ctx context.Context, id string) error {
	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	result, err := eng.ExecP(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete user failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user not found: %w", modelerror.ErrNotFound)
	}
	return nil
}

func FindUsers(ctx context.Context, query UserQuery) ([]User, error) {
	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	sql := `
		SELECT * FROM users
		WHERE 1=1
			#[ AND id = :id ]
			#[ AND name LIKE :name ]
			#[ AND email LIKE :email ]
		ORDER BY created_at DESC
		`
	var users []User
	if err := eng.Select(&users, sql, query); err != nil {
		return nil, fmt.Errorf("find users failed: %w", err)
	}
	return users, nil
}
