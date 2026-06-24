// Package crypto seals provider credentials (e.g. MT API keys) at rest with
// AES-256-GCM, keyed by HIJAU_ENCRYPTION_KEY. The key string is hashed to a
// 32-byte key with a single SHA-256 pass (no work factor), so it MUST be
// high-entropy random material — generate it with `openssl rand -base64 32`,
// never a human-chosen passphrase, or sealed secrets become brute-forceable
// offline if the database is exfiltrated. Sealed output is nonce||ciphertext.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
)

var ErrNoKey = errors.New("crypto: HIJAU_ENCRYPTION_KEY is not set")

type Cipher struct {
	gcm cipher.AEAD
}

// New derives an AES-256-GCM cipher from the configured key. Returns ErrNoKey
// if key is empty so callers can keep MT disabled rather than crash.
func New(key string) (*Cipher, error) {
	if key == "" {
		return nil, ErrNoKey
	}
	sum := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{gcm: gcm}, nil
}

// Seal encrypts plaintext, returning nonce||ciphertext.
func (c *Cipher) Seal(plain []byte) ([]byte, error) {
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return c.gcm.Seal(nonce, nonce, plain, nil), nil
}

// Open reverses Seal. It fails if the data was tampered with or sealed under a
// different key.
func (c *Cipher) Open(data []byte) ([]byte, error) {
	ns := c.gcm.NonceSize()
	if len(data) < ns {
		return nil, errors.New("crypto: ciphertext too short")
	}
	return c.gcm.Open(nil, data[:ns], data[ns:], nil)
}
