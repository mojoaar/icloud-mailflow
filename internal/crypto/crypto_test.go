package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := GenerateKey("test-password")
	plaintext := []byte("hello world")

	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	decrypted, err := Decrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("round-trip failed: got %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptProducesDifferentOutputs(t *testing.T) {
	key := GenerateKey("test-password")
	plaintext := []byte("hello")

	c1, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("encrypt 1: %v", err)
	}
	c2, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("encrypt 2: %v", err)
	}

	if bytes.Equal(c1, c2) {
		t.Error("encrypt should produce different ciphertexts due to random nonce")
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	key := GenerateKey("correct-password")
	wrongKey := GenerateKey("wrong-password")
	plaintext := []byte("secret")

	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	_, err = Decrypt(ciphertext, wrongKey)
	if err == nil {
		t.Error("decrypt with wrong key should fail")
	}
}

func TestDecryptInvalidInput(t *testing.T) {
	key := GenerateKey("test-password")

	_, err := Decrypt([]byte("not-valid-base64!!!"), key)
	if err != ErrDecryptFailed {
		t.Errorf("expected ErrDecryptFailed, got %v", err)
	}
}

func TestDecryptTooShort(t *testing.T) {
	key := GenerateKey("test-password")

	_, err := Decrypt([]byte("YQ=="), key)
	if err != ErrDecryptFailed {
		t.Errorf("expected ErrDecryptFailed for short input, got %v", err)
	}
}

func TestHashPasswordAndCheck(t *testing.T) {
	password := "my-secret-password"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	if hash == password {
		t.Error("hash should not equal the plaintext password")
	}

	if !CheckPassword(hash, password) {
		t.Error("CheckPassword should return true for correct password")
	}
}

func TestCheckPasswordWrongPassword(t *testing.T) {
	hash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	if CheckPassword(hash, "wrong-password") {
		t.Error("CheckPassword should return false for wrong password")
	}
}

func TestGenerateKeyIsDeterministic(t *testing.T) {
	k1 := GenerateKey("same-password")
	k2 := GenerateKey("same-password")

	if !bytes.Equal(k1, k2) {
		t.Error("GenerateKey should produce the same key for the same password")
	}
}

func TestGenerateKeyDifferentPasswords(t *testing.T) {
	k1 := GenerateKey("password-1")
	k2 := GenerateKey("password-2")

	if bytes.Equal(k1, k2) {
		t.Error("GenerateKey should produce different keys for different passwords")
	}
}

func TestEncryptDecryptEmptyPlaintext(t *testing.T) {
	key := GenerateKey("test-password")

	ciphertext, err := Encrypt([]byte{}, key)
	if err != nil {
		t.Fatalf("encrypt empty: %v", err)
	}

	decrypted, err := Decrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("decrypt empty: %v", err)
	}

	if !bytes.Equal([]byte{}, decrypted) {
		t.Errorf("expected empty string, got %q", decrypted)
	}
}

func TestEncryptDecryptLargePayload(t *testing.T) {
	key := GenerateKey("test-password")
	plaintext := bytes.Repeat([]byte("a"), 10000)

	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("encrypt large: %v", err)
	}

	decrypted, err := Decrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("decrypt large: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Error("large payload round-trip failed")
	}
}
