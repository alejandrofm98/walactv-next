package api

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
)

type sourceSecret struct {
	OriginalURL string `json:"originalUrl"`
	Base        string `json:"base"`
	Username    string `json:"username"`
	Password    string `json:"password"`
}

func encryptSourceSecret(key []byte, userID, sourceID string, secret sourceSecret) ([]byte, error) {
	if len(key) != 32 || userID == "" || sourceID == "" {
		return nil, errors.New("source encryption is not configured")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New("source encryption is not configured")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.New("source encryption is not configured")
	}
	plain, err := json.Marshal(secret)
	if err != nil {
		return nil, errors.New("source encryption failed")
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, errors.New("source encryption failed")
	}
	return append(nonce, aead.Seal(nil, nonce, plain, sourceSecretAAD(userID, sourceID))...), nil
}

func decryptSourceSecret(key []byte, userID, sourceID string, ciphertext []byte) (sourceSecret, error) {
	if len(key) != 32 || userID == "" || sourceID == "" {
		return sourceSecret{}, errors.New("source encryption key is invalid")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return sourceSecret{}, errors.New("source encryption key is invalid")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil || len(ciphertext) < aead.NonceSize()+aead.Overhead() {
		return sourceSecret{}, errors.New("source credentials cannot be decrypted")
	}
	nonce, sealed := ciphertext[:aead.NonceSize()], ciphertext[aead.NonceSize():]
	plain, err := aead.Open(nil, nonce, sealed, sourceSecretAAD(userID, sourceID))
	if err != nil {
		return sourceSecret{}, errors.New("source credentials cannot be decrypted")
	}
	var secret sourceSecret
	if err := json.Unmarshal(plain, &secret); err != nil || secret.OriginalURL == "" || secret.Base == "" || secret.Username == "" || secret.Password == "" {
		return sourceSecret{}, errors.New("source credentials cannot be decrypted")
	}
	return secret, nil
}

func sourceSecretAAD(userID, sourceID string) []byte {
	return []byte("walactv:source:v1\x00" + userID + "\x00" + sourceID)
}
