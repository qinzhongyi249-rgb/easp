package handlers

import (
	crypto_rand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/middleware"
	"github.com/easp-platform/easp/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// APIKeyHandler API Key 管理处理器
type APIKeyHandler struct{}

func NewAPIKeyHandler() *APIKeyHandler {
	return &APIKeyHandler{}
}

// CreateAPIKeyRequest 创建 API Key 请求
type CreateAPIKeyRequest struct {
	Name      string   `json:"name" binding:"required"`
	Scopes    []string `json:"scopes"`    // ["chat","sessions"]
	ExpiresIn int      `json:"expires_in"` // 有效天数，0=永不过期
}

// CreateAPIKey 创建 API Key（绑定到当前登录用户）
// POST /tenants/:tenantId/api-keys
func (h *APIKeyHandler) CreateAPIKey(c *gin.Context) {
	tenantID := c.Param("tenantId")
	userID := c.GetString(middleware.ContextUserID)

	var req CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 生成随机 key (32 bytes = 64 hex chars)
	randomBytes := make([]byte, 32)
	if _, err := crypto_rand.Read(randomBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate key"})
		return
	}
	rawKey := "easp_" + hex.EncodeToString(randomBytes)
	keyPrefix := rawKey[:13] // easp_xxxxxxxx

	// bcrypt hash
	keyHash, err := bcrypt.GenerateFromPassword([]byte(rawKey), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash key"})
		return
	}

	// Scopes JSON
	var scopesJSON *string
	if len(req.Scopes) > 0 {
		b, _ := json.Marshal(req.Scopes)
		s := string(b)
		scopesJSON = &s
	}

	// Expiration
	var expiresAt *time.Time
	if req.ExpiresIn > 0 {
		t := time.Now().AddDate(0, 0, req.ExpiresIn)
		expiresAt = &t
	}

	apiKey := models.APIKey{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		UserID:    userID,
		Name:      req.Name,
		KeyPrefix: keyPrefix,
		KeyHash:   string(keyHash),
		Scopes:    scopesJSON,
		Enabled:   true,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err = database.DB.Exec(`
		INSERT INTO api_keys (id, tenant_id, user_id, name, key_prefix, key_hash, scopes, enabled, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		apiKey.ID, apiKey.TenantID, apiKey.UserID, apiKey.Name, apiKey.KeyPrefix, apiKey.KeyHash,
		apiKey.Scopes, apiKey.Enabled, apiKey.ExpiresAt, apiKey.CreatedAt, apiKey.UpdatedAt)
	if err != nil {
		log.Printf("Failed to create API key: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create API key"})
		return
	}

	// 返回时包含原始 key（只显示一次）
	c.JSON(http.StatusCreated, gin.H{
		"id":         apiKey.ID,
		"name":       apiKey.Name,
		"key":        rawKey, // 只在创建时返回
		"key_prefix": keyPrefix,
		"scopes":     req.Scopes,
		"expires_at": expiresAt,
		"created_at": apiKey.CreatedAt,
	})
}

// ListAPIKeys 列出 API Key
// GET /tenants/:tenantId/api-keys
func (h *APIKeyHandler) ListAPIKeys(c *gin.Context) {
	tenantID := c.Param("tenantId")

	var keys []models.APIKey
	err := database.DB.Select(&keys,
		"SELECT id, tenant_id, user_id, name, key_prefix, scopes, enabled, expires_at, last_used_at, usage_count, created_at, updated_at FROM api_keys WHERE tenant_id = ? ORDER BY created_at DESC", tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list API keys"})
		return
	}
	if keys == nil {
		keys = []models.APIKey{}
	}

	c.JSON(http.StatusOK, keys)
}

// DeleteAPIKey 删除 API Key
// DELETE /tenants/:tenantId/api-keys/:keyId
func (h *APIKeyHandler) DeleteAPIKey(c *gin.Context) {
	tenantID := c.Param("tenantId")
	keyID := c.Param("keyId")

	result, err := database.DB.Exec(
		"DELETE FROM api_keys WHERE id = ? AND tenant_id = ?", keyID, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete API key"})
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API key deleted"})
}

// ToggleAPIKey 启用/禁用 API Key
// PUT /tenants/:tenantId/api-keys/:keyId/toggle
func (h *APIKeyHandler) ToggleAPIKey(c *gin.Context) {
	tenantID := c.Param("tenantId")
	keyID := c.Param("keyId")

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := database.DB.Exec(
		"UPDATE api_keys SET enabled = ?, updated_at = NOW() WHERE id = ? AND tenant_id = ?",
		req.Enabled, keyID, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update API key"})
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API key updated", "enabled": req.Enabled})
}
