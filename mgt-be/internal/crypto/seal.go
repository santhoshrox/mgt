// Package crypto provides AES-GCM "sealing" used for at-rest encryption of
// sensitive values like GitHub OAuth tokens and per-repo webhook secrets.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

type Sealer struct {
	gcm cipher.AEAD
}

func NewSealer(key []byte) (*Sealer, error) {
	if len(key) != 32 {
		return nil, errors.New("crypto: key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Sealer{gcm: gcm}, nil
}

// Seal encrypts plaintext and returns a base64-encoded ciphertext that
// includes the nonce.
func (s *Sealer) Seal(plaintext string) (string, error) {
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := s.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// Open reverses Seal.
func (s *Sealer) Open(sealed string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return "", err
	}
	ns := s.gcm.NonceSize()
	if len(raw) < ns {
		return "", errors.New("crypto: ciphertext too short")
	}
	nonce, ct := raw[:ns], raw[ns:]
	pt, err := s.gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}
