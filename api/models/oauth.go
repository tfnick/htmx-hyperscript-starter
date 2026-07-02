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
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
)

const (
	oauthStateDuration       = 10 * time.Minute
	oauthLoginResultDuration = 5 * time.Minute
)

type OAuthIdentity struct {
	ID             string `db:"id"`
	Provider       string `db:"provider"`
	ProviderUserID string `db:"provider_user_id"`
	UserID         string `db:"user_id"`
	Email          string `db:"email"`
	EmailVerified  int    `db:"email_verified"`
	DisplayName    string `db:"display_name"`
	CreatedAt      string `db:"created_at"`
	UpdatedAt      string `db:"updated_at"`
}

type OAuthIdentityInput struct {
	Provider       string
	ProviderUserID string
	UserID         string
	Email          string
	EmailVerified  int
	DisplayName    string
}

type OAuthState struct {
	ID           string `db:"id"`
	StateHash    string `db:"state_hash"`
	Provider     string `db:"provider"`
	RedirectPath string `db:"redirect_path"`
	ExpiresAt    string `db:"expires_at"`
	UsedAt       string `db:"used_at"`
	CreatedAt    string `db:"created_at"`
}

type OAuthLoginResult struct {
	ID           string `db:"id"`
	TokenHash    string `db:"token_hash"`
	UserID       string `db:"user_id"`
	RedirectPath string `db:"redirect_path"`
	ExpiresAt    string `db:"expires_at"`
	UsedAt       string `db:"used_at"`
	CreatedAt    string `db:"created_at"`
}

func CreateOAuthState(ctx context.Context, provider string, redirectPath string) (string, error) {
	token, tokenHash, err := generateOpaqueToken()
	if err != nil {
		return "", fmt.Errorf("generate oauth state failed: %w", err)
	}

	now := timefmt.NowUTC()
	state := OAuthState{
		ID:           uuid.Must(uuid.NewV7()).String(),
		StateHash:    tokenHash,
		Provider:     provider,
		RedirectPath: redirectPath,
		ExpiresAt:    timefmt.SQLiteDateTime(now.Add(oauthStateDuration)),
		CreatedAt:    timefmt.SQLiteDateTime(now),
	}

	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return "", fmt.Errorf("database unavailable: %w", err)
	}

	sql := `
		INSERT INTO oauth_states (id, state_hash, provider, redirect_path, expires_at, created_at)
		VALUES (:id, :state_hash, :provider, :redirect_path, :expires_at, :created_at)
	`
	if _, err := eng.ExecNamed(sql, state); err != nil {
		return "", fmt.Errorf("create oauth state failed: %w", err)
	}
	return token, nil
}

func UseOAuthState(ctx context.Context, provider string, token string) (*OAuthState, error) {
	tokenHash := hashOpaqueToken(token)

	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var state OAuthState
	err = eng.GetNamed(&state, `
		SELECT id, state_hash, provider, redirect_path, expires_at, COALESCE(used_at, '') AS used_at, created_at
		FROM oauth_states
		WHERE state_hash = :state_hash
		  AND provider = :provider
		  AND expires_at > :now
		  AND used_at IS NULL
	`, map[string]interface{}{
		"state_hash": tokenHash,
		"provider":   provider,
		"now":        timefmt.NowSQLiteDateTime(),
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("oauth state not found: %w", modelerror.ErrNotFound)
		}
		return nil, fmt.Errorf("get oauth state failed: %w", err)
	}

	result, err := eng.ExecNamed(`
		UPDATE oauth_states
		SET used_at = :used_at
		WHERE id = :id
		  AND used_at IS NULL
	`, map[string]interface{}{
		"id":      state.ID,
		"used_at": timefmt.NowSQLiteDateTime(),
	})
	if err != nil {
		return nil, fmt.Errorf("mark oauth state used failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, fmt.Errorf("oauth state already used: %w", modelerror.ErrNotFound)
	}
	return &state, nil
}

func GetOAuthIdentityByProviderUserID(ctx context.Context, provider string, providerUserID string) (*OAuthIdentity, error) {
	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var identity OAuthIdentity
	err = eng.GetNamed(&identity, `
		SELECT id, provider, provider_user_id, user_id, email, email_verified, display_name, created_at, updated_at
		FROM oauth_identities
		WHERE provider = :provider
		  AND provider_user_id = :provider_user_id
	`, map[string]interface{}{
		"provider":         provider,
		"provider_user_id": providerUserID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get oauth identity failed: %w", err)
	}
	return &identity, nil
}

func CreateOAuthIdentity(ctx context.Context, input OAuthIdentityInput) (*OAuthIdentity, error) {
	now := timefmt.NowSQLiteDateTime()
	identity := OAuthIdentity{
		ID:             uuid.Must(uuid.NewV7()).String(),
		Provider:       input.Provider,
		ProviderUserID: input.ProviderUserID,
		UserID:         input.UserID,
		Email:          input.Email,
		EmailVerified:  input.EmailVerified,
		DisplayName:    input.DisplayName,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	sql := `
		INSERT INTO oauth_identities (
			id, provider, provider_user_id, user_id, email, email_verified, display_name, created_at, updated_at
		) VALUES (
			:id, :provider, :provider_user_id, :user_id, :email, :email_verified, :display_name, :created_at, :updated_at
		)
	`
	if _, err := eng.ExecNamed(sql, identity); err != nil {
		return nil, fmt.Errorf("create oauth identity failed: %w", err)
	}
	return &identity, nil
}

func CreateOAuthLoginResult(ctx context.Context, userID string, redirectPath string) (string, error) {
	token, tokenHash, err := generateOpaqueToken()
	if err != nil {
		return "", fmt.Errorf("generate oauth login result failed: %w", err)
	}

	now := timefmt.NowUTC()
	result := OAuthLoginResult{
		ID:           uuid.Must(uuid.NewV7()).String(),
		TokenHash:    tokenHash,
		UserID:       userID,
		RedirectPath: redirectPath,
		ExpiresAt:    timefmt.SQLiteDateTime(now.Add(oauthLoginResultDuration)),
		CreatedAt:    timefmt.SQLiteDateTime(now),
	}

	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return "", fmt.Errorf("database unavailable: %w", err)
	}

	sql := `
		INSERT INTO oauth_login_results (id, token_hash, user_id, redirect_path, expires_at, created_at)
		VALUES (:id, :token_hash, :user_id, :redirect_path, :expires_at, :created_at)
	`
	if _, err := eng.ExecNamed(sql, result); err != nil {
		return "", fmt.Errorf("create oauth login result failed: %w", err)
	}
	return token, nil
}

func UseOAuthLoginResult(ctx context.Context, token string) (*OAuthLoginResult, error) {
	tokenHash := hashOpaqueToken(token)

	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var result OAuthLoginResult
	err = eng.GetNamed(&result, `
		SELECT id, token_hash, user_id, redirect_path, expires_at, COALESCE(used_at, '') AS used_at, created_at
		FROM oauth_login_results
		WHERE token_hash = :token_hash
		  AND expires_at > :now
		  AND used_at IS NULL
	`, map[string]interface{}{
		"token_hash": tokenHash,
		"now":        timefmt.NowSQLiteDateTime(),
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("oauth login result not found: %w", modelerror.ErrNotFound)
		}
		return nil, fmt.Errorf("get oauth login result failed: %w", err)
	}

	updateResult, err := eng.ExecNamed(`
		UPDATE oauth_login_results
		SET used_at = :used_at
		WHERE id = :id
		  AND used_at IS NULL
	`, map[string]interface{}{
		"id":      result.ID,
		"used_at": timefmt.NowSQLiteDateTime(),
	})
	if err != nil {
		return nil, fmt.Errorf("mark oauth login result used failed: %w", err)
	}
	rows, _ := updateResult.RowsAffected()
	if rows == 0 {
		return nil, fmt.Errorf("oauth login result already used: %w", modelerror.ErrNotFound)
	}
	return &result, nil
}

func generateOpaqueToken() (string, string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(tokenBytes)
	return token, hashOpaqueToken(token), nil
}

func hashOpaqueToken(token string) string {
	tokenHash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(tokenHash[:])
}
