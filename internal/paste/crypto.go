package paste

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
)

// KeySize is the AES-256-GCM key length in bytes used by all paste crypto.
const KeySize = 32

// GenerateKey returns a fresh random 32-byte key suitable for paste encryption.
func GenerateKey() ([]byte, error) {
	key := make([]byte, KeySize)
	_, err := rand.Read(key)
	return key, err
}

// EncryptJSON marshals v to JSON and encrypts it with AES-256-GCM under key,
// returning the ciphertext and the freshly generated nonce (iv). The nonce is
// drawn fresh from crypto/rand on every call — callers must NOT reuse a nonce
// with the same key, which would catastrophically break GCM's confidentiality.
func EncryptJSON(v any, key []byte) (ciphertext, iv []byte, err error) {
	if len(key) != KeySize {
		return nil, nil, errors.New("paste: key must be 32 bytes")
	}
	plain, err := json.Marshal(v)
	if err != nil {
		return nil, nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	iv = make([]byte, gcm.NonceSize())
	if _, err := rand.Read(iv); err != nil {
		return nil, nil, err
	}
	ciphertext = gcm.Seal(nil, iv, plain, nil)
	return ciphertext, iv, nil
}

// DecryptJSON inverts EncryptJSON: it decrypts ciphertext under key with the
// given nonce iv and unmarshals the plaintext JSON into v. Returns an error
// if the GCM tag fails to verify, the key is wrong, or the plaintext is not
// valid JSON for the target type.
func DecryptJSON(ciphertext, iv, key []byte, v any) error {
	if len(key) != KeySize {
		return errors.New("paste: key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	plain, err := gcm.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return err
	}
	return json.Unmarshal(plain, v)
}
