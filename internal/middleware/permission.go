package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/easp-platform/easp/internal/repositories"
	"github.com/gin-gonic/gin"
)

// RequirePermission 要求特定权限的中间件
func RequirePermission(requiredTool string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取当前用户ID
		userID, exists := c.Get(ContextUserID)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
			c.Abort()
			return
		}

		// 获取用户角色
		userRoleRepo := repositories.NewUserRoleRepository()
		roles, err := userRoleRepo.GetUserRoles(userID.(string))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user roles"})
			c.Abort()
			return
		}

		// 检查是否有权限
		hasPermission := false
		for _, role := range roles {
			if role.Tools != nil && checkToolPermission(*role.Tools, requiredTool) {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Permission denied",
				"required": requiredTool,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAnyPermission 要求任意一个权限的中间件
func RequireAnyPermission(requiredTools ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取当前用户ID
		userID, exists := c.Get(ContextUserID)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
			c.Abort()
			return
		}

		// 获取用户角色
		userRoleRepo := repositories.NewUserRoleRepository()
		roles, err := userRoleRepo.GetUserRoles(userID.(string))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user roles"})
			c.Abort()
			return
		}

		// 检查是否有任意权限
		hasPermission := false
		for _, role := range roles {
			if role.Tools == nil {
				continue
			}
			for _, tool := range requiredTools {
				if checkToolPermission(*role.Tools, tool) {
					hasPermission = true
					break
				}
			}
			if hasPermission {
				break
			}
		}

		if !hasPermission {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Permission denied",
				"required": requiredTools,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAdmin 要求管理员权限
func RequireAdmin() gin.HandlerFunc {
	return RequirePermission("*")
}

// checkToolPermission 检查工具权限
func checkToolPermission(toolsJSON, requiredTool string) bool {
	// 解析工具列表
	var tools []string
	if err := json.Unmarshal([]byte(toolsJSON), &tools); err != nil {
		return false
	}

	// 检查是否有通配符权限
	for _, tool := range tools {
		if tool == "*" {
			return true
		}
	}

	// 检查具体权限
	// 支持层级权限，如 "connectors" 匹配 "connectors:read", "connectors:write" 等
	for _, tool := range tools {
		if tool == requiredTool {
			return true
		}
		// 检查前缀匹配
		if strings.HasPrefix(requiredTool, tool+":") {
			return true
		}
	}

	return false
}

// GetUserPermissions 获取用户所有权限
func GetUserPermissions(userID string) ([]string, error) {
	userRoleRepo := repositories.NewUserRoleRepository()
	roles, err := userRoleRepo.GetUserRoles(userID)
	if err != nil {
		return nil, err
	}

	permissionSet := make(map[string]bool)
	for _, role := range roles {
		if role.Tools == nil {
			continue
		}
		var tools []string
		if err := json.Unmarshal([]byte(*role.Tools), &tools); err != nil {
			continue
		}
		for _, tool := range tools {
			permissionSet[tool] = true
		}
	}

	permissions := make([]string, 0, len(permissionSet))
	for p := range permissionSet {
		permissions = append(permissions, p)
	}

	return permissions, nil
}
