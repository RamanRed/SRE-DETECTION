package secretbox

import (
	"errors"
	"strings"
	"testing"
)

func TestEncryptDecryptAndRandomNonce(t *testing.T) {
	box, err := New(strings.Repeat("k", 32))
	if err != nil {
		t.Fatal(err)
	}
	first, err := box.Encrypt("top-secret-token")
	if err != nil {
		t.Fatal(err)
	}
	second, err := box.Encrypt("top-secret-token")
	if err != nil {
		t.Fatal(err)
	}
	if first == second || strings.Contains(first, "top-secret-token") || !strings.HasPrefix(first, prefix) {
		t.Fatalf("ciphertexts are not safely randomized: %q %q", first, second)
	}
	plaintext, err := box.Decrypt(first)
	if err != nil || plaintext != "top-secret-token" {
		t.Fatalf("Decrypt() = %q, %v", plaintext, err)
	}
}

func TestMissingKeyNeverStoresPlaintext(t *testing.T) {
	box, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := box.Encrypt("secret"); !errors.Is(err, ErrKeyNotConfigured) {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if _, err := box.Decrypt("plaintext"); err == nil {
		t.Fatal("Decrypt() accepted a plaintext credential")
	}
}
