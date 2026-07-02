package credentials

import "testing"

func TestEncryptDecryptStringRoundTrip(t *testing.T) {
	ciphertext, err := EncryptString("secret-api-key")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if ciphertext == "secret-api-key" {
		t.Fatal("expected ciphertext to differ from plaintext")
	}

	plaintext, err := DecryptString(ciphertext)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if plaintext != "secret-api-key" {
		t.Fatalf("unexpected plaintext: %q", plaintext)
	}
}

func TestMaskSecret(t *testing.T) {
	tests := map[string]string{
		"":               "",
		"abc":            "***",
		"abcdef":         "ab**ef",
		"secret-api-key": "secr******-key",
	}

	for input, expected := range tests {
		if got := MaskSecret(input); got != expected {
			t.Fatalf("MaskSecret(%q) = %q, want %q", input, got, expected)
		}
	}
}
