package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/easp-platform/easp/internal/auth"
	"github.com/gin-gonic/gin"
)

// ContextKey 上下文键
const (
	ContextUserID   = "user_id"
	ContextTenantID = "tenant_id"
	ContextEmail    = "email"
	ContextRoleIDs  = "role_ids"
)

// JWTAuth JWT认证中间件
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从Header获取Token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// 解析Bearer Token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format"})
			c.Abort()
			return
		}

		tokenStr := parts[1]

		// 解析Token
		claims, err := auth.ParseToken(tokenStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// 将用户信息存入上下文
		c.Set(ContextUserID, claims.UserID)
		c.Set(ContextTenantID, claims.TenantID)
		c.Set(ContextEmail, claims.Email)
		c.Set(ContextRoleIDs, claims.RoleIDs)

		c.Next()
	}
}

// TenantAuth 租户权限中间件
func TenantAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取Token中的租户ID
		tokenTenantID, exists := c.Get(ContextTenantID)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Tenant context not found"})
			c.Abort()
			return
		}

		// 获取请求中的租户ID
		requestTenantID := c.Param("tenantId")
		if requestTenantID == "" {
			c.Next()
			return
		}

		// 验证租户匹配
		if tokenTenantID.(string) != requestTenantID {
			// 检查是否为admin角色（允许跨租户访问）
			roleIDs, _ := c.Get(ContextRoleIDs)
			if roleIDs != nil {
				var roles []string
				json.Unmarshal([]byte(roleIDs.(string)), &roles)
				for _, role := range roles {
					if auth.IsAdminRole(role) {
						c.Next()
						return
					}
				}
			}

			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this tenant"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetCurrentUser 获取当前用户信息
func GetCurrentUser(c *gin.Context) (userID, tenantID, email string, exists bool) {
	uid, _ := c.Get(ContextUserID)
	tid, _ := c.Get(ContextTenantID)
	em, _ := c.Get(ContextEmail)

	if uid == nil || tid == nil {
		return "", "", "", false
	}

	return uid.(string), tid.(string), em.(string), true
}
