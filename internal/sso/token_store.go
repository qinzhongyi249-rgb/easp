package sso

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/easp-platform/easp/internal/auth"
	"github.com/easp-platform/easp/internal/database"
	"github.com/google/uuid"
)

func SaveUserToken(tenantID, userID, token string) error {
	if tenantID == "" || userID == "" || token == "" {
		return nil
	}
	ciphertext, err := encryptToken(token)
	if err != nil {
		return err
	}
	_, err = database.DB.Exec(`INSERT INTO user_sso_tokens (id, tenant_id, user_id, token_ciphertext, created_at, updated_at)
		VALUES (?, ?, ?, ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE token_ciphertext = VALUES(token_ciphertext), updated_at = NOW()`, uuid.New().String(), tenantID, userID, ciphertext)
	return err
}

func GetUserToken(tenantID, userID string) (string, error) {
	var ciphertext string
	err := database.DB.Get(&ciphertext, "SELECT token_ciphertext FROM user_sso_tokens WHERE tenant_id = ? AND user_id = ?", tenantID, userID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return decryptToken(ciphertext)
}

func encryptToken(plaintext string) (string, error) {
	block, err := aes.NewCipher(tokenKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func decryptToken(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(tokenKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", fmt.Errorf("invalid encrypted SSO token")
	}
	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func tokenKey() []byte {
	sum := sha256.Sum256(auth.JWTSecret)
	return sum[:]
}
