package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
)

const (
	EnvJWTSecret     = "APP_JWT_SECRET"
	TokenTypeBearer  = "Bearer"
	AccessTokenTTL   = 7 * 24 * time.Hour
	RefreshTokenTTL  = 30 * 24 * time.Hour
)

var (
	ErrMissingToken = errors.New("missing auth token")
	ErrInvalidToken = errors.New("invalid auth token")
	ErrExpiredToken = errors.New("auth token expired")

	secretOnce sync.Once
	secretKey  []byte
	secretErr  error
)

type Token struct {
	AccessToken string
	TokenType   string
	ExpiresAt   time.Time
	ExpiresIn   int64
}

type Claims struct {
	Subject   string `json:"sub"`
	TokenType string `json:"typ"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

type jwtHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

func IssueUserToken(userID string) (Token, error) {
	return issueUserTokenAt(userID, timefmt.NowUTC())
}

func ParseUserToken(raw string) (Claims, error) {
	return parseUserTokenAt(raw, timefmt.NowUTC())
}

func IssueRefreshToken(userID string) (Token, error) {
	return issueRefreshTokenAt(userID, timefmt.NowUTC())
}

func ParseRefreshToken(raw string) (Claims, error) {
	return parseRefreshTokenAt(raw, timefmt.NowUTC())
}

func RefreshAccessToken(raw string) (Token, error) {
	claims, err := ParseRefreshToken(raw)
	if err != nil {
		return Token{}, err
	}
	return IssueUserToken(claims.Subject)
}

func ParseToken(raw string) (Claims, error) {
	claims, err := ParseUserToken(raw)
	if err == nil {
		return claims, nil
	}
	return ParseRefreshToken(raw)
}

func issueRefreshTokenAt(userID string, now time.Time) (Token, error) {
	now = now.UTC()
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return Token{}, fmt.Errorf("%w: user ID is required", ErrInvalidToken)
	}

	expiresAt := now.Add(RefreshTokenTTL)
	claims := Claims{
		Subject:   userID,
		TokenType: "refresh",
		IssuedAt:  now.Unix(),
		ExpiresAt: expiresAt.Unix(),
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return Token{}, err
	}

	headerJSON, err := json.Marshal(jwtHeader{Algorithm: "HS256", Type: "JWT"})
	if err != nil {
		return Token{}, err
	}

	unsigned := encodeSegment(headerJSON) + "." + encodeSegment(claimsJSON)
	signature, err := sign(unsigned)
	if err != nil {
		return Token{}, err
	}

	return Token{
		AccessToken: unsigned + "." + encodeSegment(signature),
		TokenType:   TokenTypeBearer,
		ExpiresAt:   expiresAt,
		ExpiresIn:   int64(RefreshTokenTTL.Seconds()),
	}, nil
}

func parseRefreshTokenAt(raw string, now time.Time) (Claims, error) {
	now = now.UTC()
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Claims{}, ErrMissingToken
	}

	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return Claims{}, ErrInvalidToken
	}

	headerBytes, err := decodeSegment(parts[0])
	if err != nil {
		return Claims{}, fmt.Errorf("%w: malformed header", ErrInvalidToken)
	}
	var header jwtHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return Claims{}, fmt.Errorf("%w: malformed header", ErrInvalidToken)
	}
	if header.Algorithm != "HS256" || header.Type != "JWT" {
		return Claims{}, ErrInvalidToken
	}

	unsigned := parts[0] + "." + parts[1]
	expected, err := sign(unsigned)
	if err != nil {
		return Claims{}, err
	}
	actual, err := decodeSegment(parts[2])
	if err != nil {
		return Claims{}, fmt.Errorf("%w: malformed signature", ErrInvalidToken)
	}
	if !hmac.Equal(actual, expected) {
		return Claims{}, ErrInvalidToken
	}

	claimBytes, err := decodeSegment(parts[1])
	if err != nil {
		return Claims{}, fmt.Errorf("%w: malformed claims", ErrInvalidToken)
	}
	var claims Claims
	if err := json.Unmarshal(claimBytes, &claims); err != nil {
		return Claims{}, fmt.Errorf("%w: malformed claims", ErrInvalidToken)
	}
	if strings.TrimSpace(claims.Subject) == "" || claims.TokenType != "refresh" {
		return Claims{}, ErrInvalidToken
	}
	if claims.ExpiresAt <= now.Unix() {
		return Claims{}, ErrExpiredToken
	}

	return claims, nil
}

func issueUserTokenAt(userID string, now time.Time) (Token, error) {
	now = now.UTC()
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return Token{}, fmt.Errorf("%w: user ID is required", ErrInvalidToken)
	}

	expiresAt := now.Add(AccessTokenTTL)
	claims := Claims{
		Subject:   userID,
		TokenType: "access",
		IssuedAt:  now.Unix(),
		ExpiresAt: expiresAt.Unix(),
	}

	headerBytes, err := json.Marshal(jwtHeader{Algorithm: "HS256", Type: "JWT"})
	if err != nil {
		return Token{}, err
	}
	claimBytes, err := json.Marshal(claims)
	if err != nil {
		return Token{}, err
	}

	unsigned := encodeSegment(headerBytes) + "." + encodeSegment(claimBytes)
	signature, err := sign(unsigned)
	if err != nil {
		return Token{}, err
	}

	return Token{
		AccessToken: unsigned + "." + encodeSegment(signature),
		TokenType:   TokenTypeBearer,
		ExpiresAt:   expiresAt,
		ExpiresIn:   int64(AccessTokenTTL.Seconds()),
	}, nil
}

func parseUserTokenAt(raw string, now time.Time) (Claims, error) {
	now = now.UTC()
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Claims{}, ErrMissingToken
	}

	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return Claims{}, ErrInvalidToken
	}

	headerBytes, err := decodeSegment(parts[0])
	if err != nil {
		return Claims{}, fmt.Errorf("%w: malformed header", ErrInvalidToken)
	}
	var header jwtHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return Claims{}, fmt.Errorf("%w: malformed header", ErrInvalidToken)
	}
	if header.Algorithm != "HS256" || header.Type != "JWT" {
		return Claims{}, ErrInvalidToken
	}

	unsigned := parts[0] + "." + parts[1]
	expected, err := sign(unsigned)
	if err != nil {
		return Claims{}, err
	}
	actual, err := decodeSegment(parts[2])
	if err != nil {
		return Claims{}, fmt.Errorf("%w: malformed signature", ErrInvalidToken)
	}
	if !hmac.Equal(actual, expected) {
		return Claims{}, ErrInvalidToken
	}

	claimBytes, err := decodeSegment(parts[1])
	if err != nil {
		return Claims{}, fmt.Errorf("%w: malformed claims", ErrInvalidToken)
	}
	var claims Claims
	if err := json.Unmarshal(claimBytes, &claims); err != nil {
		return Claims{}, fmt.Errorf("%w: malformed claims", ErrInvalidToken)
	}
	if strings.TrimSpace(claims.Subject) == "" || claims.TokenType != "access" {
		return Claims{}, ErrInvalidToken
	}
	if claims.ExpiresAt <= now.Unix() {
		return Claims{}, ErrExpiredToken
	}

	return claims, nil
}

func sign(unsigned string) ([]byte, error) {
	key, err := jwtSecret()
	if err != nil {
		return nil, err
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(unsigned))
	return mac.Sum(nil), nil
}

func jwtSecret() ([]byte, error) {
	secretOnce.Do(func() {
		if configured := strings.TrimSpace(os.Getenv(EnvJWTSecret)); configured != "" {
			secretKey = []byte(configured)
			return
		}

		secretKey = make([]byte, 32)
		if _, err := rand.Read(secretKey); err != nil {
			secretErr = fmt.Errorf("generate jwt secret failed: %w", err)
		}
	})
	return secretKey, secretErr
}

func MapTokenError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrMissingToken) {
		return fmt.Errorf("refresh token is required")
	}
	if errors.Is(err, ErrInvalidToken) {
		return fmt.Errorf("refresh token is invalid")
	}
	if errors.Is(err, ErrExpiredToken) {
		return fmt.Errorf("refresh token is expired, please log in again")
	}
	return err
}

func encodeSegment(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeSegment(value string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(value)
}
