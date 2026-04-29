package paste

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
)

const KeySize = 32

func GenerateKey() ([]byte, error) {
	key := make([]byte, KeySize)
	_, err := rand.Read(key)
	return key, err
}

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
