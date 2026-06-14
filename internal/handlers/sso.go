package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/easp-platform/easp/internal/auth"
	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/models"
	"github.com/easp-platform/easp/internal/repositories"
	ssoStore "github.com/easp-platform/easp/internal/sso"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// SSOHandler SSO处理器
type SSOHandler struct{}

// NewSSOHandler 创建SSO处理器
func NewSSOHandler() *SSOHandler {
	return &SSOHandler{}
}

// GetConfig 获取租户SSO配置
func (h *SSOHandler) GetConfig(c *gin.Context) {
	tenantID := c.Param("tenantId")

	var config models.TenantSSOConfig
	err := database.DB.Get(&config, "SELECT * FROM tenant_sso_configs WHERE tenant_id = ?", tenantID)
	if err != nil {
		// 返回默认空配置
		c.JSON(http.StatusOK, gin.H{
			"tenant_id": tenantID,
			"enabled":   false,
		})
		return
	}

	c.JSON(http.StatusOK, config)
}

// SaveConfig 保存租户SSO配置
func (h *SSOHandler) SaveConfig(c *gin.Context) {
	tenantID := c.Param("tenantId")

	var req struct {
		Enabled           bool   `json:"enabled"`
		LoginURL          string `json:"login_url" binding:"required"`
		LoginMethod       string `json:"login_method"`
		LoginHeaders      string `json:"login_headers"`
		LoginBodyTemplate string `json:"login_body_template"`
		UserInfoURL       string `json:"user_info_url"`
		UserInfoMethod    string `json:"user_info_method"`
		UserInfoHeaders   string `json:"user_info_headers"`
		ResponseMapping   string `json:"response_mapping"`
		CallbackURL       string `json:"callback_url"`
		SyncUserOnLogin   bool   `json:"sync_user_on_login"`
		SyncURL           string `json:"sync_url"`
		SyncMethod        string `json:"sync_method"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 设置默认值
	if req.LoginMethod == "" {
		req.LoginMethod = "POST"
	}
	if req.UserInfoMethod == "" {
		req.UserInfoMethod = "GET"
	}
	if req.SyncMethod == "" {
		req.SyncMethod = "POST"
	}

	// 默认请求体模板
	if req.LoginBodyTemplate == "" {
		req.LoginBodyTemplate = `{"username":"{{username}}","password":"{{password}}"}`
	}

	// 默认响应映射
	if req.ResponseMapping == "" {
		req.ResponseMapping = `{"token":"$.token","user_id":"$.user.id","email":"$.user.email","display_name":"$.user.name"}`
	}

	// 检查是否已存在配置
	var existingID string
	err := database.DB.QueryRow("SELECT id FROM tenant_sso_configs WHERE tenant_id = ?", tenantID).Scan(&existingID)

	if err != nil {
		// 创建新配置
		configID := uuid.New().String()
		_, err = database.DB.Exec(`INSERT INTO tenant_sso_configs 
			(id, tenant_id, enabled, login_url, login_method, login_headers, login_body_template, 
			 user_info_url, user_info_method, user_info_headers, response_mapping, callback_url,
			 sync_user_on_login, sync_url, sync_method, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`,
			configID, tenantID, req.Enabled, req.LoginURL, req.LoginMethod,
			nilIfEmpty(req.LoginHeaders), nilIfEmpty(req.LoginBodyTemplate),
			nilIfEmpty(req.UserInfoURL), req.UserInfoMethod, nilIfEmpty(req.UserInfoHeaders),
			nilIfEmpty(req.ResponseMapping), nilIfEmpty(req.CallbackURL),
			req.SyncUserOnLogin, nilIfEmpty(req.SyncURL), req.SyncMethod)
		if err != nil {
			log.Printf("Failed to create SSO config: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create SSO config", "details": err.Error()})
			return
		}
	} else {
		// 更新现有配置
		_, err = database.DB.Exec(`UPDATE tenant_sso_configs SET 
			enabled = ?, login_url = ?, login_method = ?, login_headers = ?, login_body_template = ?,
			user_info_url = ?, user_info_method = ?, user_info_headers = ?, response_mapping = ?,
			callback_url = ?, sync_user_on_login = ?, sync_url = ?, sync_method = ?, updated_at = NOW()
			WHERE tenant_id = ?`,
			req.Enabled, req.LoginURL, req.LoginMethod,
			nilIfEmpty(req.LoginHeaders), nilIfEmpty(req.LoginBodyTemplate),
			nilIfEmpty(req.UserInfoURL), req.UserInfoMethod, nilIfEmpty(req.UserInfoHeaders),
			nilIfEmpty(req.ResponseMapping), nilIfEmpty(req.CallbackURL),
			req.SyncUserOnLogin, nilIfEmpty(req.SyncURL), req.SyncMethod, tenantID)
		if err != nil {
			log.Printf("Failed to update SSO config: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update SSO config"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "SSO config saved"})
}

// TenantLogin 租户级SSO登录
func (h *SSOHandler) TenantLogin(c *gin.Context) {
	tenantID := c.Param("tenantId")

	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查租户是否存在且有效
	var tenant models.Tenant
	if err := database.DB.Get(&tenant, "SELECT * FROM tenants WHERE id = ?", tenantID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tenant not found"})
		return
	}
	if tenant.Status != "active" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Tenant is not active"})
		return
	}
	if tenant.ExpiresAt != nil && tenant.ExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Tenant has expired"})
		return
	}

	// 获取租户SSO配置
	var config models.TenantSSOConfig
	err := database.DB.Get(&config, "SELECT * FROM tenant_sso_configs WHERE tenant_id = ? AND enabled = true", tenantID)

	if err == nil && config.Enabled {
		// ===== SSO 登录 =====
		loginResp, err := callBusinessLogin(config, req.Username, req.Password)
		if err != nil {
			log.Printf("Business login failed: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Login failed", "details": err.Error()})
			return
		}

		userInfo, err := parseLoginResponse(config, loginResp)
		if err != nil {
			log.Printf("Failed to parse login response: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse login response"})
			return
		}

		user, err := createOrUpdateUser(tenantID, userInfo)
		if err != nil {
			log.Printf("Failed to create/update user: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}

		if config.SyncUserOnLogin && config.SyncURL != nil && *config.SyncURL != "" {
			go syncUserToBusiness(config, user, loginResp)
		}

		roleRepo := repositories.NewUserRoleRepository()
		roles, _ := roleRepo.GetUserRoles(user.ID)
		roleIDs := "["
		for i, role := range roles {
			if i > 0 {
				roleIDs += ","
			}
			roleIDs += `"` + role.ID + `"`
		}
		roleIDs += "]"

		tokenPair, err := auth.GenerateTokenPair(user.ID, tenantID, user.Email, roleIDs)
		if err != nil {
			log.Printf("Failed to generate token: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		user.LastLoginAt = &time.Time{}
		*user.LastLoginAt = time.Now()
		user.LoginCount++
		repositories.NewUserRepository().Update(user)
		if token := userInfo["token"]; token != "" {
			if err := ssoStore.SaveUserToken(tenantID, user.ID, token); err != nil {
				log.Printf("Failed to save user SSO token: %v", err)
			}
		}

		callbackURL := ""
		if config.CallbackURL != nil {
			callbackURL = *config.CallbackURL
		}

		c.JSON(http.StatusOK, gin.H{
			"user": gin.H{
				"id":           user.ID,
				"tenant_id":    user.TenantID,
				"email":        user.Email,
				"display_name": user.DisplayName,
				"status":       user.Status,
			},
			"tokens":       tokenPair,
			"callback_url": callbackURL,
			"biz_token":    userInfo["token"],
		})
	} else {
		// ===== 标准登录（SSO未开启） =====
		userRepo := repositories.NewUserRepository()
		identifier := NormalizeLoginIdentifier(req.Username)
		users, err := userRepo.ListByIdentifier(identifier)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid account or password"})
			return
		}
		user, err := SelectLoginUser(users, tenantID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid account or password"})
			return
		}

		// 校验用户必须属于当前租户
		if user.TenantID != tenantID {
			c.JSON(http.StatusForbidden, gin.H{"error": "This account does not belong to this tenant"})
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid account or password"})
			return
		}

		if user.Status != "active" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Account is not active"})
			return
		}

		roleRepo := repositories.NewUserRoleRepository()
		roles, _ := roleRepo.GetUserRoles(user.ID)
		roleIDs := "["
		for i, role := range roles {
			if i > 0 {
				roleIDs += ","
			}
			roleIDs += `"` + role.ID + `"`
		}
		roleIDs += "]"

		tokenPair, err := auth.GenerateTokenPair(user.ID, tenantID, user.Email, roleIDs)
		if err != nil {
			log.Printf("Failed to generate token: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		user.LastLoginAt = &time.Time{}
		*user.LastLoginAt = time.Now()
		user.LoginCount++
		userRepo.Update(user)

		// 检查是否为admin
		isAdmin := false
		for _, role := range roles {
			if auth.IsAdminRole(role.ID) {
				isAdmin = true
				break
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"user": gin.H{
				"id":           user.ID,
				"tenant_id":    user.TenantID,
				"email":        user.Email,
				"display_name": user.DisplayName,
				"status":       user.Status,
				"is_admin":     isAdmin,
			},
			"tokens": tokenPair,
		})
	}
}

// GenerateLoginURL 生成租户登录链接
func (h *SSOHandler) GenerateLoginURL(c *gin.Context) {
	tenantID := c.Param("tenantId")

	// 验证租户存在
	var tenant models.Tenant
	err := database.DB.Get(&tenant, "SELECT * FROM tenants WHERE id = ?", tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tenant not found"})
		return
	}

	// 获取当前域名
	scheme := "https"
	if c.Request.TLS == nil {
		scheme = "http"
	}
	host := c.Request.Host
	baseURL := fmt.Sprintf("%s://%s", scheme, host)

	loginURL := fmt.Sprintf("%s/sso/%s", baseURL, tenantID)
	ssoLoginURL := fmt.Sprintf("%s/sso/%s", baseURL, tenantID)

	c.JSON(http.StatusOK, gin.H{
		"tenant_id":     tenantID,
		"tenant_name":   tenant.Name,
		"login_url":     loginURL,
		"sso_login_url": ssoLoginURL,
	})
}

// TestSSOConnection 测试SSO连接
func (h *SSOHandler) TestSSOConnection(c *gin.Context) {
	tenantID := c.Param("tenantId")

	var config models.TenantSSOConfig
	err := database.DB.Get(&config, "SELECT * FROM tenant_sso_configs WHERE tenant_id = ?", tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "SSO config not found"})
		return
	}

	// 发送测试请求
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(config.LoginMethod, config.LoginURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid URL", "details": err.Error()})
		return
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"status_code": resp.StatusCode,
		"message":     "Connection successful",
	})
}

// ========== 内部函数 ==========

// callBusinessLogin 调用业务系统登录接口
func callBusinessLogin(config models.TenantSSOConfig, username, password string) (map[string]interface{}, error) {
	// 构建请求体
	bodyTemplate := `{"username":"{{username}}","password":"{{password}}"}`
	if config.LoginBodyTemplate != nil {
		bodyTemplate = *config.LoginBodyTemplate
	}

	body := strings.ReplaceAll(bodyTemplate, "{{username}}", username)
	body = strings.ReplaceAll(bodyTemplate, "{{password}}", password)

	// 创建请求
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(config.LoginMethod, config.LoginURL, bytes.NewBufferString(body))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// 添加自定义请求头
	if config.LoginHeaders != nil {
		var headers map[string]string
		json.Unmarshal([]byte(*config.LoginHeaders), &headers)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("login failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// 解析JSON响应
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response failed: %v", err)
	}

	return result, nil
}

// parseLoginResponse 解析登录响应
func parseLoginResponse(config models.TenantSSOConfig, resp map[string]interface{}) (map[string]string, error) {
	// 默认映射
	mapping := map[string]string{
		"token":        "$.token",
		"user_id":      "$.user.id",
		"email":        "$.user.email",
		"display_name": "$.user.name",
	}

	// 使用自定义映射
	if config.ResponseMapping != nil {
		json.Unmarshal([]byte(*config.ResponseMapping), &mapping)
	}

	result := make(map[string]string)
	for key, path := range mapping {
		val := extractValue(resp, path)
		if val != "" {
			result[key] = val
		}
	}

	return result, nil
}

// extractValue 从JSON中提取值（简单的JSONPath实现）
func extractValue(data map[string]interface{}, path string) string {
	if !strings.HasPrefix(path, "$.") {
		return fmt.Sprintf("%v", data[path])
	}

	parts := strings.Split(strings.TrimPrefix(path, "$."), ".")
	current := data

	for i, part := range parts {
		if i == len(parts)-1 {
			if val, ok := current[part]; ok {
				return fmt.Sprintf("%v", val)
			}
			return ""
		}

		if next, ok := current[part]; ok {
			if nextMap, ok := next.(map[string]interface{}); ok {
				current = nextMap
			} else {
				return ""
			}
		} else {
			return ""
		}
	}

	return ""
}

// createOrUpdateUser 创建或更新用户
func createOrUpdateUser(tenantID string, userInfo map[string]string) (*models.User, error) {
	userRepo := repositories.NewUserRepository()

	email := userInfo["email"]
	if email == "" {
		email = fmt.Sprintf("%s@sso.local", userInfo["user_id"])
	}

	// 尝试查找当前租户现有用户
	user, _ := userRepo.GetByTenantAndEmail(tenantID, email)
	if user != nil {
		// 更新用户信息
		if userInfo["display_name"] != "" {
			user.DisplayName = userInfo["display_name"]
		}
		user.SSOProvider = "tenant_sso"
		user.SSOUserID = userInfo["user_id"]
		now := time.Now()
		user.SSOLinkedAt = &now
		userRepo.Update(user)
		return user, nil
	}

	// 创建新用户
	user = &models.User{
		ID:          uuid.New().String(),
		TenantID:    tenantID,
		Email:       email,
		DisplayName: userInfo["display_name"],
		Status:      "active",
		SSOProvider: "tenant_sso",
		SSOUserID:   userInfo["user_id"],
		SSOLinkedAt: &time.Time{},
	}
	*user.SSOLinkedAt = time.Now()

	if err := userRepo.Create(user); err != nil {
		return nil, err
	}

	// 分配默认角色
	roleRepo := repositories.NewRoleRepository()
	defaultRole, _ := roleRepo.GetByName(tenantID, "普通用户")
	if defaultRole != nil {
		repositories.NewUserRoleRepository().Assign(user.ID, defaultRole.ID)
	}

	return user, nil
}

// syncUserToBusiness 同步用户信息到业务系统
func syncUserToBusiness(config models.TenantSSOConfig, user *models.User, bizResp map[string]interface{}) {
	if config.SyncURL == nil || *config.SyncURL == "" {
		return
	}

	syncData := map[string]interface{}{
		"easp_user_id": user.ID,
		"email":        user.Email,
		"display_name": user.DisplayName,
		"biz_token":    bizResp["token"],
	}

	body, _ := json.Marshal(syncData)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(config.SyncMethod, *config.SyncURL, bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Failed to create sync request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	// 添加自定义请求头
	if config.SyncHeaders != nil {
		var headers map[string]string
		json.Unmarshal([]byte(*config.SyncHeaders), &headers)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to sync user to business: %v", err)
		return
	}
	defer resp.Body.Close()

	log.Printf("User sync completed for %s, status: %d", user.Email, resp.StatusCode)
}

// nilIfEmpty 空字符串返回nil
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
