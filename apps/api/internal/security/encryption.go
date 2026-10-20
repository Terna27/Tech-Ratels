package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

var (
	ErrEncryptionKeyMissing = errors.New("safeguarding encryption key is unconfigured")
	ErrInvalidCiphertext    = errors.New("invalid ciphertext format")
	ErrDecryptionFailed     = errors.New("decryption failed or authentication tag mismatch")
	ErrKeyTooShort          = errors.New("encryption key must be at least 32 bytes")
)

// DevFallbackKey is strictly used in development and test environments when SAFEGUARDING_ENCRYPTION_KEY is unset.
const DevFallbackKey = "dev-safeguarding-key-32bytes-secure-for-tests!"

// SafeguardingCipher manages AES-256-GCM encryption and decryption.
type SafeguardingCipher struct {
	key       []byte
	version   string
	isProd    bool
	isConfig  bool
}

// NewSafeguardingCipher initializes a cipher instance. In production, an empty key causes Encrypt to fail-safe.
func NewSafeguardingCipher(rawKey string, isProduction bool) (*SafeguardingCipher, error) {
	trimmed := strings.TrimSpace(rawKey)
	if trimmed == "" {
		if isProduction {
			return &SafeguardingCipher{isProd: true, isConfig: false}, nil
		}
		trimmed = DevFallbackKey
	}

	keyBytes, err := parseKeyBytes(trimmed)
	if err != nil {
		if isProduction {
			return nil, err
		}
		// In dev/test fallback
		keyBytes = []byte(DevFallbackKey[:32])
	}

	return &SafeguardingCipher{
		key:      keyBytes,
		version:  "v1",
		isProd:   isProduction,
		isConfig: true,
	}, nil
}

func parseKeyBytes(s string) ([]byte, error) {
	// Try hex decoding if 64 chars
	if len(s) == 64 {
		if b, err := hex.DecodeString(s); err == nil && len(b) == 32 {
			return b, nil
		}
	}
	// Try base64 decoding if 44 chars
	if len(s) == 44 {
		if b, err := base64.StdEncoding.DecodeString(s); err == nil && len(b) == 32 {
			return b, nil
		}
	}
	// Direct bytes if at least 32 bytes
	b := []byte(s)
	if len(b) < 32 {
		return nil, ErrKeyTooShort
	}
	return b[:32], nil
}

// Encrypt encrypts plaintext using AES-256-GCM and returns "v1:<hex_nonce>:<hex_ciphertext>".
func (c *SafeguardingCipher) Encrypt(plaintext string) (string, error) {
	if !c.isConfig {
		return "", ErrEncryptionKeyMissing
	}
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher block: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return fmt.Sprintf("%s:%s:%s", c.version, hex.EncodeToString(nonce), hex.EncodeToString(ciphertext)), nil
}

// Decrypt decrypts a "v1:<hex_nonce>:<hex_ciphertext>" string.
func (c *SafeguardingCipher) Decrypt(encoded string) (string, error) {
	if !c.isConfig {
		return "", ErrEncryptionKeyMissing
	}
	if encoded == "" {
		return "", nil
	}

	parts := strings.Split(encoded, ":")
	if len(parts) != 3 {
		return "", ErrInvalidCiphertext
	}

	version := parts[0]
	if version != "v1" {
		return "", fmt.Errorf("unsupported encryption version: %s", version)
	}

	nonce, err := hex.DecodeString(parts[1])
	if err != nil {
		return "", ErrInvalidCiphertext
	}

	ciphertext, err := hex.DecodeString(parts[2])
	if err != nil {
		return "", ErrInvalidCiphertext
	}

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher block: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	if len(nonce) != gcm.NonceSize() {
		return "", ErrInvalidCiphertext
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}

// GeneratePublicCaseReference produces a high-entropy non-sequential reference like CASE-2026-F89A1B2C.
func GeneratePublicCaseReference() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		// Fallback
		return fmt.Sprintf("CASE-%d-%08X", time.Now().Year(), time.Now().UnixNano()%0xFFFFFFFF)
	}
	return fmt.Sprintf("CASE-%d-%s", time.Now().Year(), strings.ToUpper(hex.EncodeToString(b)))
}

// GenerateFollowUpSecret creates a cryptographically secure 32-character hex secret with prefix "sec_".
func GenerateFollowUpSecret() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "sec_" + hex.EncodeToString(b), nil
}

// HashFollowUpSecret creates a SHA-256 hash of the follow-up secret.
func HashFollowUpSecret(secret string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(secret)))
	return hex.EncodeToString(h[:])
}

// VerifyFollowUpSecret performs constant-time comparison of a candidate secret against a stored hash.
func VerifyFollowUpSecret(candidateSecret, storedHash string) bool {
	candidateHash := HashFollowUpSecret(candidateSecret)
	return subtle.ConstantTimeCompare([]byte(candidateHash), []byte(storedHash)) == 1
}
