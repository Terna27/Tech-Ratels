package security

import (
	"strings"
	"testing"
)

func TestSafeguardingCipher_RoundTrip(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef" // 32 bytes
	cipher, err := NewSafeguardingCipher(key, false)
	if err != nil {
		t.Fatalf("unexpected error creating cipher: %v", err)
	}

	plaintext := "Sensitive incident: Misconduct reported at Ikeja cleanup on 2026-10-15 involving volunteer coordinator."
	encrypted, err := cipher.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	if !strings.HasPrefix(encrypted, "v1:") {
		t.Errorf("expected v1: prefix, got: %s", encrypted)
	}

	decrypted, err := cipher.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("decryption failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("expected plaintext %q, got %q", plaintext, decrypted)
	}
}

func TestSafeguardingCipher_TamperedCiphertext(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef"
	cipher, err := NewSafeguardingCipher(key, false)
	if err != nil {
		t.Fatalf("unexpected error creating cipher: %v", err)
	}

	plaintext := "Confidential data"
	encrypted, err := cipher.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	// Tamper with the last character of ciphertext
	runes := []rune(encrypted)
	if runes[len(runes)-1] == 'a' {
		runes[len(runes)-1] = 'b'
	} else {
		runes[len(runes)-1] = 'a'
	}
	tampered := string(runes)

	_, err = cipher.Decrypt(tampered)
	if err == nil {
		t.Error("expected decryption failure for tampered ciphertext, got nil")
	}
}

func TestSafeguardingCipher_UnconfiguredInProduction(t *testing.T) {
	cipher, err := NewSafeguardingCipher("", true)
	if err != nil {
		t.Fatalf("unexpected error creating unconfigured cipher: %v", err)
	}

	_, err = cipher.Encrypt("Sensitive info")
	if err != ErrEncryptionKeyMissing {
		t.Errorf("expected ErrEncryptionKeyMissing in production, got: %v", err)
	}
}

func TestSafeguardingCipher_DevFallback(t *testing.T) {
	cipher, err := NewSafeguardingCipher("", false)
	if err != nil {
		t.Fatalf("unexpected error creating dev fallback cipher: %v", err)
	}

	plaintext := "Dev fallback test narrative"
	encrypted, err := cipher.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("expected dev fallback to encrypt successfully, got: %v", err)
	}

	decrypted, err := cipher.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("expected dev fallback to decrypt successfully, got: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestFollowUpSecret_HashingAndVerification(t *testing.T) {
	secret, err := GenerateFollowUpSecret()
	if err != nil {
		t.Fatalf("failed to generate follow-up secret: %v", err)
	}

	if !strings.HasPrefix(secret, "sec_") || len(secret) < 20 {
		t.Errorf("unexpected secret format: %s", secret)
	}

	hash := HashFollowUpSecret(secret)
	if len(hash) != 64 {
		t.Errorf("expected 64-char sha256 hex hash, got length %d", len(hash))
	}

	if !VerifyFollowUpSecret(secret, hash) {
		t.Error("expected valid secret to verify against hash")
	}

	if VerifyFollowUpSecret("sec_wrong_secret_123456", hash) {
		t.Error("expected invalid secret to fail verification")
	}
}

func TestGeneratePublicCaseReference(t *testing.T) {
	ref1 := GeneratePublicCaseReference()
	ref2 := GeneratePublicCaseReference()

	if !strings.HasPrefix(ref1, "CASE-2026-") {
		t.Errorf("expected reference to start with CASE-2026-, got %s", ref1)
	}

	if ref1 == ref2 {
		t.Errorf("expected consecutive references to be distinct, got %s and %s", ref1, ref2)
	}
}
