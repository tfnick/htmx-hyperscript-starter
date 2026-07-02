package models

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tfnick/go-svelte-starter/api/db"
)

type OpenAPIKey struct {
	ID          string `db:"id"`
	PartnerID   string `db:"partner_id"`
	AccountID   string `db:"account_id"`
	TokenHash   string `db:"token_hash"`
	Environment string `db:"environment"`
	Scopes      string `db:"scopes"`
	Status      string `db:"status"`
	CreatedAt   string `db:"created_at"`
}

type OpenAPIConsumer struct {
	KeyID       string
	PartnerID   string
	AccountID   string
	Scopes      []string
	Environment string
}

func ResolveOpenAPIConsumer(ctx context.Context, rawKey string) (*OpenAPIConsumer, error) {
	tokenHash := sha256.Sum256([]byte(rawKey))
	key, err := GetOpenAPIKeyByTokenHash(ctx, hex.EncodeToString(tokenHash[:]))
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, fmt.Errorf("open api key not found")
	}
	if err := ValidateOpenAPIKey(key); err != nil {
		return nil, err
	}

	return &OpenAPIConsumer{
		KeyID:       key.ID,
		PartnerID:   key.PartnerID,
		AccountID:   key.AccountID,
		Scopes:      splitScopes(key.Scopes),
		Environment: key.Environment,
	}, nil
}

func GetOpenAPIKeyByTokenHash(ctx context.Context, tokenHash string) (*OpenAPIKey, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	query := `
		SELECT
			k.id,
			k.partner_id,
			p.account_id,
			k.token_hash,
			k.environment,
			k.scopes,
			k.status
		FROM open_api_keys k
		JOIN open_api_partners p ON p.id = k.partner_id
		WHERE k.token_hash = ?
		  AND k.revoked_at IS NULL
		  AND (k.expires_at IS NULL OR k.expires_at > CURRENT_TIMESTAMP)
	`

	var key OpenAPIKey
	if err := d.GetP(&key, query, tokenHash); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query open api key failed: %w", err)
	}
	return &key, nil
}

func ValidateOpenAPIKey(key *OpenAPIKey) error {
	if key.Status != "active" {
		return fmt.Errorf("open api key is not active")
	}
	if key.AccountID == "" {
		return fmt.Errorf("open api key has no account binding")
	}
	return nil
}

func splitScopes(scopes string) []string {
	if scopes == "" {
		return nil
	}

	parts := strings.Split(scopes, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		scope := strings.TrimSpace(part)
		if scope != "" {
			result = append(result, scope)
		}
	}
	return result
}

func hasScope(scopes []string, required string) bool {
	for _, scope := range scopes {
		if scope == required {
			return true
		}
	}
	return false
}

// OpenAPIPartner represents a row in open_api_partners.
type OpenAPIPartner struct {
	ID        string `db:"id"`
	Name      string `db:"name"`
	AccountID string `db:"account_id"`
	Status    string `db:"status"`
	CreatedAt string `db:"created_at"`
}

// OpenAPIKeyWithPartner joins open_api_keys with partner name for display.
type OpenAPIKeyWithPartner struct {
	ID          string `db:"id"`
	PartnerID   string `db:"partner_id"`
	PartnerName string `db:"partner_name"`
	TokenHash   string `db:"token_hash"`
	Environment string `db:"environment"`
	Scopes      string `db:"scopes"`
	Status      string `db:"status"`
	RevokedAt   string `db:"revoked_at"`
	CreatedAt   string `db:"created_at"`
}

// CreateOpenAPIPartner inserts a new partner record.
func CreateOpenAPIPartner(ctx context.Context, partner *OpenAPIPartner) error {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	partner.ID = uuid.Must(uuid.NewV7()).String()
	if partner.Status == "" {
		partner.Status = "active"
	}

	_, execErr := d.ExecP(`
		INSERT INTO open_api_partners (id, name, account_id, status)
		VALUES (?, ?, ?, ?)
	`, partner.ID, partner.Name, partner.AccountID, partner.Status)
	if execErr != nil {
		return fmt.Errorf("insert open api partner failed: %w", execErr)
	}
	return nil
}

// ListOpenAPIPartners returns all partners.
func ListOpenAPIPartners(ctx context.Context) ([]OpenAPIPartner, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var partners []OpenAPIPartner
	if err := d.SelectP(&partners, `
		SELECT id, name, account_id, status, created_at
		FROM open_api_partners
		ORDER BY created_at DESC
	`); err != nil {
		return nil, fmt.Errorf("query open api partners failed: %w", err)
	}
	return partners, nil
}

// GetOpenAPIPartnerByID returns a single partner by primary key.
func GetOpenAPIPartnerByID(ctx context.Context, id string) (*OpenAPIPartner, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var partner OpenAPIPartner
	if err := d.GetP(&partner, `
		SELECT id, name, account_id, status, created_at
		FROM open_api_partners
		WHERE id = ?
	`, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query open api partner failed: %w", err)
	}
	return &partner, nil
}

// GenerateOpenAPIToken creates a cryptographically random hex token.
// Returns the raw token (show once) and its SHA-256 hash (store in DB).
func GenerateOpenAPIToken() (raw string, hash string, err error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", fmt.Errorf("failed to generate random token: %w", err)
	}
	raw = hex.EncodeToString(bytes)
	tokenHash := sha256.Sum256([]byte(raw))
	return raw, hex.EncodeToString(tokenHash[:]), nil
}

// CreateOpenAPIKey inserts a new key and returns the model.
func CreateOpenAPIKey(ctx context.Context, partnerID, tokenHash, environment, scopes string) (*OpenAPIKey, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	keyID := uuid.Must(uuid.NewV7()).String()
	_, execErr := d.ExecP(`
		INSERT INTO open_api_keys (id, partner_id, token_hash, environment, scopes, status)
		VALUES (?, ?, ?, ?, ?, 'active')
	`, keyID, partnerID, tokenHash, environment, scopes)
	if execErr != nil {
		return nil, fmt.Errorf("insert open api key failed: %w", execErr)
	}

	var key OpenAPIKey
	if err := d.GetP(&key, `
		SELECT id, partner_id, token_hash, environment, scopes, status, created_at
		FROM open_api_keys
		WHERE id = ?
	`, keyID); err != nil {
		return nil, fmt.Errorf("load created open api key failed: %w", err)
	}
	return &key, nil
}

// ListOpenAPIKeysByPartnerID returns all keys belonging to a partner.
func ListOpenAPIKeysByPartnerID(ctx context.Context, partnerID string) ([]OpenAPIKey, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var keys []OpenAPIKey
	if err := d.SelectP(&keys, `
		SELECT id, partner_id, token_hash, environment, scopes, status, created_at
		FROM open_api_keys
		WHERE partner_id = ?
		ORDER BY created_at DESC
	`, partnerID); err != nil {
		return nil, fmt.Errorf("query open api keys failed: %w", err)
	}
	return keys, nil
}

// RevokeOpenAPIKey sets status='revoked' and records revoked_at.
func RevokeOpenAPIKey(ctx context.Context, keyID string) error {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	result, execErr := d.ExecP(`
		UPDATE open_api_keys
		SET status = 'revoked', revoked_at = ?
		WHERE id = ? AND status = 'active'
	`, now, keyID)
	if execErr != nil {
		return fmt.Errorf("revoke open api key failed: %w", execErr)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("open api key not found or already revoked")
	}
	return nil
}

// ListOpenAPIKeys returns all keys joined with partner name.
func ListOpenAPIKeys(ctx context.Context) ([]OpenAPIKeyWithPartner, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var keys []OpenAPIKeyWithPartner
	if err := d.SelectP(&keys, `
		SELECT
			k.id,
			k.partner_id,
			p.name AS partner_name,
			k.token_hash,
			k.environment,
			k.scopes,
			k.status,
			COALESCE(k.revoked_at, '') AS revoked_at,
			k.created_at
		FROM open_api_keys k
		JOIN open_api_partners p ON p.id = k.partner_id
		ORDER BY k.created_at DESC
	`); err != nil {
		return nil, fmt.Errorf("query open api keys failed: %w", err)
	}
	return keys, nil
}
