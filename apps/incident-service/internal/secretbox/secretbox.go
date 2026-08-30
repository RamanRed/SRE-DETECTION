package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

const prefix = "enc:v1:"

var ErrKeyNotConfigured = errors.New("integration encryption key is not configured")

type Codec interface {
	Encrypt(string) (string, error)
	Decrypt(string) (string, error)
}

type Box struct {
	aead cipher.AEAD
}

func New(configuredKey string) (*Box, error) {
	configuredKey = strings.TrimSpace(configuredKey)
	if configuredKey == "" {
		return &Box{}, nil
	}
	key, err := decodeKey(configuredKey)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create integration credential cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create integration credential AEAD: %w", err)
	}
	return &Box{aead: aead}, nil
}

func (b *Box) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if strings.HasPrefix(plaintext, prefix) {
		return "", errors.New("refusing to encrypt an already encrypted credential")
	}
	if b == nil || b.aead == nil {
		return "", ErrKeyNotConfigured
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate credential nonce: %w", err)
	}
	sealed := b.aead.Seal(nonce, nonce, []byte(plaintext), []byte(prefix))
	return prefix + base64.RawStdEncoding.EncodeToString(sealed), nil
}

func (b *Box) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	if !strings.HasPrefix(ciphertext, prefix) {
		return "", errors.New("unencrypted integration credential rejected; reconnect the integration")
	}
	if b == nil || b.aead == nil {
		return "", ErrKeyNotConfigured
	}
	encoded := strings.TrimPrefix(ciphertext, prefix)
	sealed, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", errors.New("integration credential ciphertext is malformed")
	}
	if len(sealed) < b.aead.NonceSize() {
		return "", errors.New("integration credential ciphertext is truncated")
	}
	nonce, payload := sealed[:b.aead.NonceSize()], sealed[b.aead.NonceSize():]
	plaintext, err := b.aead.Open(nil, nonce, payload, []byte(prefix))
	if err != nil {
		return "", errors.New("integration credential could not be decrypted")
	}
	return string(plaintext), nil
}

func decodeKey(configured string) ([]byte, error) {
	if decoded, err := base64.StdEncoding.DecodeString(configured); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(configured); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	if decoded, err := hex.DecodeString(configured); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	if len([]byte(configured)) == 32 {
		return []byte(configured), nil
	}
	return nil, errors.New("INTEGRATION_ENCRYPTION_KEY must decode to exactly 32 bytes")
}
