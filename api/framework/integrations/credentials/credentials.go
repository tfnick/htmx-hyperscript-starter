package credentials

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

const (
	EnvMasterKey = "APP_INTEGRATION_MASTER_KEY"

	ciphertextPrefix = "v1:"
	nonceSize        = 12
)

var (
	fallbackKeyOnce sync.Once
	fallbackKey     []byte
)

func EncryptString(plaintext string) (string, error) {
	if plaintext == "" {
		return "", errors.New("plaintext is required")
	}

	block, err := aes.NewCipher(masterKey())
	if err != nil {
		return "", fmt.Errorf("create cipher failed: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm failed: %w", err)
	}

	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("create nonce failed: %w", err)
	}

	sealed := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	payload := make([]byte, 0, len(nonce)+len(sealed))
	payload = append(payload, nonce...)
	payload = append(payload, sealed...)
	return ciphertextPrefix + base64.RawURLEncoding.EncodeToString(payload), nil
}

func DecryptString(ciphertextValue string) (string, error) {
	if !strings.HasPrefix(ciphertextValue, ciphertextPrefix) {
		return "", errors.New("unsupported ciphertext format")
	}

	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(ciphertextValue, ciphertextPrefix))
	if err != nil {
		return "", fmt.Errorf("decode ciphertext failed: %w", err)
	}
	if len(raw) <= nonceSize {
		return "", errors.New("ciphertext payload is too short")
	}

	block, err := aes.NewCipher(masterKey())
	if err != nil {
		return "", fmt.Errorf("create cipher failed: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm failed: %w", err)
	}

	nonce := raw[:nonceSize]
	sealed := raw[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt ciphertext failed: %w", err)
	}
	return string(plaintext), nil
}

func MaskSecret(secret string) string {
	if secret == "" {
		return ""
	}
	if len(secret) <= 4 {
		return strings.Repeat("*", len(secret))
	}
	if len(secret) <= 10 {
		return secret[:2] + strings.Repeat("*", len(secret)-4) + secret[len(secret)-2:]
	}
	return secret[:4] + strings.Repeat("*", len(secret)-8) + secret[len(secret)-4:]
}

func masterKey() []byte {
	raw := os.Getenv(EnvMasterKey)
	if raw == "" {
		raw = string(processFallbackKey())
	}
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

func processFallbackKey() []byte {
	fallbackKeyOnce.Do(func() {
		fallbackKey = make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, fallbackKey); err != nil {
			panic(fmt.Sprintf("generate integration fallback key failed: %v", err))
		}
	})
	return fallbackKey
}
