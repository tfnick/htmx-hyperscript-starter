package auth

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestIssueAndParseUserToken(t *testing.T) {
	t.Setenv(EnvJWTSecret, "test-secret")
	resetJWTSecretForTest()

	token, err := issueUserTokenAt("u1", time.Unix(1000, 0))
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	if token.TokenType != TokenTypeBearer {
		t.Fatalf("expected bearer token type, got %s", token.TokenType)
	}

	claims, err := parseUserTokenAt(token.AccessToken, time.Unix(1001, 0))
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.Subject != "u1" || claims.TokenType != "access" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestParseUserTokenRejectsExpiredToken(t *testing.T) {
	t.Setenv(EnvJWTSecret, "test-secret")
	resetJWTSecretForTest()

	token, err := issueUserTokenAt("u1", time.Unix(1000, 0))
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	_, err = parseUserTokenAt(token.AccessToken, time.Unix(1000, 0).Add(AccessTokenTTL+time.Second))
	if !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("expected expired token error, got %v", err)
	}
}

func TestParseUserTokenRejectsTamperedToken(t *testing.T) {
	t.Setenv(EnvJWTSecret, "test-secret")
	resetJWTSecretForTest()

	token, err := issueUserTokenAt("u1", time.Unix(1000, 0))
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	_, err = parseUserTokenAt(token.AccessToken+"x", time.Unix(1001, 0))
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected invalid token error, got %v", err)
	}
}

func resetJWTSecretForTest() {
	secretOnce = sync.Once{}
	secretKey = nil
	secretErr = nil
}
