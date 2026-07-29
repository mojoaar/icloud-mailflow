package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"

	"golang.org/x/crypto/bcrypt"
)

var ErrDecryptFailed = errors.New("decryption failed")

func GenerateKey(password string) []byte {
	h := sha256.Sum256([]byte(password))
	return h[:]
}

func Encrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	out := make([]byte, base64.StdEncoding.EncodedLen(len(ciphertext)))
	base64.StdEncoding.Encode(out, ciphertext)
	return out, nil
}

func Decrypt(encodedText, key []byte) ([]byte, error) {
	ciphertext := make([]byte, base64.StdEncoding.DecodedLen(len(encodedText)))
	n, err := base64.StdEncoding.Decode(ciphertext, encodedText)
	if err != nil {
		return nil, ErrDecryptFailed
	}
	ciphertext = ciphertext[:n]
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(ciphertext) < ns {
		return nil, ErrDecryptFailed
	}
	return gcm.Open(nil, ciphertext[:ns], ciphertext[ns:], nil)
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
