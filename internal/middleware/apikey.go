package middleware

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/easp-platform/easp/internal/auth"
	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/mcp"
	"github.com/easp-platform/easp/internal/models"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// Context keys for Embed API
const (
	ContextAPIKey           = "api_key"
	ContextEmbedTenantID    = "embed_tenant_id"
	ContextEmbedUserID      = "embed_user_id"
	ContextEmbedTokenType   = "embed_token_type"
	ContextSourceType       = "source_type"
	ContextSourceAppID      = "source_app_id"
	ContextExternalSystem   = "external_system"
	ContextExternalUserID   = "external_user_id"
	ContextExternalTokenRef = "external_token_ref"
)

// APIKeyAuth API Key 认证中间件（用于 Embed API）
func APIKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Header 读取 API Key
		// 支持两种方式：
		// 1. Authorization: Bearer easp_xxxxx
		// 2. X-API-Key: easp_xxxxx
		apiKey := ""

		authHeader := c.GetHeader("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer easp_") {
			apiKey = strings.TrimPrefix(authHeader, "Bearer ")
		}

		if apiKey == "" {
			apiKey = c.GetHeader("X-API-Key")
		}

		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "api_key_required",
				"message": "API key is required. Use Authorization: Bearer easp_xxx or X-API-Key header.",
			})
			c.Abort()
			return
		}

		// 验证 key 格式
		if !strings.HasPrefix(apiKey, "easp_") {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_api_key",
				"message": "Invalid API key format",
			})
			c.Abort()
			return
		}

		// 通过前缀查找 key
		keyPrefix := apiKey[:13] // easp_xxxxxxxx
		var key models.APIKey
		err := database.DB.Get(&key,
			"SELECT * FROM api_keys WHERE key_prefix = ? AND enabled = true", keyPrefix)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_api_key",
				"message": "Invalid or disabled API key",
			})
			c.Abort()
			return
		}

		// 验证 key hash
		if err := bcrypt.CompareHashAndPassword([]byte(key.KeyHash), []byte(apiKey)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_api_key",
				"message": "Invalid API key",
			})
			c.Abort()
			return
		}

		// 检查过期
		if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "api_key_expired",
				"message": "API key has expired",
			})
			c.Abort()
			return
		}

		// 更新使用统计（异步）
		go func() {
			database.DB.Exec(
				"UPDATE api_keys SET last_used_at = NOW(), usage_count = usage_count + 1 WHERE id = ?",
				key.ID)
		}()

		// 设置上下文
		c.Set(ContextAPIKey, key)
		c.Set(ContextEmbedTenantID, key.TenantID)
		c.Set(ContextEmbedUserID, key.UserID)

		c.Next()
	}
}

// CheckScope 检查 API Key 权限范围
func CheckScope(requiredScope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		keyVal, exists := c.Get(ContextAPIKey)
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "API key context not found"})
			c.Abort()
			return
		}

		key := keyVal.(models.APIKey)

		// 如果没有设置 scopes，默认允许所有
		if key.Scopes == nil {
			c.Next()
			return
		}

		var scopes []string
		if err := json.Unmarshal([]byte(*key.Scopes), &scopes); err != nil {
			c.Next()
			return
		}

		// 空 scopes = 全部权限
		if len(scopes) == 0 {
			c.Next()
			return
		}

		for _, s := range scopes {
			if s == requiredScope || s == "*" {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{
			"error":   "insufficient_scope",
			"message": "API key does not have the required scope: " + requiredScope,
		})
		c.Abort()
	}
}

// EmbedTokenAuth easp-api-token 认证中间件：嵌入式助手短 Token 复用 EASP 用户/角色/工具/Skill 权限体系。
func EmbedTokenAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := strings.TrimSpace(c.GetHeader("easp-api-token"))
		if tokenStr == "" {
			tokenStr = strings.TrimSpace(c.GetHeader("Easp-Api-Token"))
		}
		if tokenStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "EASP_API_TOKEN_REQUIRED", "message": "easp-api-token header is required"})
			c.Abort()
			return
		}

		claims, err := auth.ParseEmbedToken(tokenStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "INVALID_EASP_API_TOKEN", "message": "Invalid or expired easp-api-token"})
			c.Abort()
			return
		}

		c.Set(ContextUserID, claims.UserID)
		c.Set(ContextTenantID, claims.TenantID)
		c.Set(ContextEmail, claims.Email)
		roleIDs := "[]"
		var roleRows []models.Role
		if err := database.DB.Select(&roleRows, `SELECT r.* FROM roles r JOIN user_roles ur ON r.id = ur.role_id WHERE ur.user_id = ?`, claims.UserID); err == nil {
			ids := make([]string, 0, len(roleRows))
			for _, r := range roleRows {
				ids = append(ids, r.ID)
			}
			if b, err := json.Marshal(ids); err == nil {
				roleIDs = string(b)
			}
		}
		c.Set(ContextRoleIDs, roleIDs)
		c.Set(ContextEmbedTenantID, claims.TenantID)
		c.Set(ContextEmbedUserID, claims.UserID)
		c.Set(ContextEmbedTokenType, "embed")
		c.Set(ContextSourceType, "embed")
		c.Set(ContextSourceAppID, claims.AppID)
		c.Set(ContextExternalSystem, claims.ExternalSystem)
		c.Set(ContextExternalUserID, claims.ExternalUserID)
		c.Set(ContextExternalTokenRef, claims.ExternalTokenRef)
		if externalToken, ok := auth.GetEmbedExternalUserToken(claims.ExternalTokenRef); ok {
			c.Request = c.Request.WithContext(mcp.WithUserSSOToken(c.Request.Context(), externalToken))
		}
		c.Next()
	}
}

// UpdateAPIKeyUsage 更新 API Key 使用记录（同步版本，用于精确计数）
func UpdateAPIKeyUsage(keyID string) {
	_, err := database.DB.Exec(
		"UPDATE api_keys SET last_used_at = NOW(), usage_count = usage_count + 1 WHERE id = ?", keyID)
	if err != nil {
		log.Printf("UpdateAPIKeyUsage failed: %v", err)
	}
}
