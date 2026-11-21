package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

const (
	keySize   = 32 // AES-256
	nonceSize = 12 // GCM standard
)

// MessageEncryptor chiffre et déchiffre les messages chat avec AES-256-GCM.
// Chaque thread a une clé dérivée du master key via HKDF (isolation par thread).
// Le thread_id sert aussi d'Additional Data (AEAD) pour lier cryptographiquement
// le ciphertext à son thread, empêchant toute réutilisation cross-thread.
type MessageEncryptor struct {
	masterKey []byte // 32 bytes
}

// NewMessageEncryptor initialise le chiffreur depuis une clé master en base64.
// CHAT_ENCRYPTION_KEY doit être un aléatoire de 32 bytes encodé en base64.
func NewMessageEncryptor(masterKeyBase64 string) (*MessageEncryptor, error) {
	key, err := base64.StdEncoding.DecodeString(masterKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("crypto: decode master key: %w", err)
	}
	if len(key) != keySize {
		return nil, fmt.Errorf("crypto: master key must be %d bytes, got %d", keySize, len(key))
	}
	return &MessageEncryptor{masterKey: key}, nil
}

// Encrypt chiffre plaintext pour le thread threadID.
// Retourne (ciphertext, nonce, error).
func (e *MessageEncryptor) Encrypt(plaintext []byte, threadID string) (ciphertext, nonce []byte, err error) {
	threadKey, err := e.deriveThreadKey(threadID)
	if err != nil {
		return nil, nil, err
	}

	block, err := aes.NewCipher(threadKey)
	if err != nil {
		return nil, nil, fmt.Errorf("crypto: new cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("crypto: new gcm: %w", err)
	}

	nonce = make([]byte, nonceSize)
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("crypto: generate nonce: %w", err)
	}

	// threadID sert d'additional data pour lier le ciphertext au thread.
	ciphertext = aead.Seal(nil, nonce, plaintext, []byte(threadID))
	return ciphertext, nonce, nil
}

// Decrypt déchiffre ciphertext pour le thread threadID.
func (e *MessageEncryptor) Decrypt(ciphertext, nonce []byte, threadID string) ([]byte, error) {
	threadKey, err := e.deriveThreadKey(threadID)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(threadKey)
	if err != nil {
		return nil, fmt.Errorf("crypto: new cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: new gcm: %w", err)
	}

	plaintext, err := aead.Open(nil, nonce, ciphertext, []byte(threadID))
	if err != nil {
		return nil, fmt.Errorf("crypto: decrypt: %w", err)
	}
	return plaintext, nil
}

// deriveThreadKey dérive une clé 32 bytes spécifique à un thread via HKDF-SHA256.
// Même master key + thread différent → clé différente.
func (e *MessageEncryptor) deriveThreadKey(threadID string) ([]byte, error) {
	info := []byte("tissimah-chat-thread-" + threadID)
	r := hkdf.New(sha256.New, e.masterKey, nil, info)
	key := make([]byte, keySize)
	if _, err := io.ReadFull(r, key); err != nil {
		return nil, fmt.Errorf("crypto: derive thread key: %w", err)
	}
	return key, nil
}
