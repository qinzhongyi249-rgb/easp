package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/easp-platform/easp/internal/auth"
	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/middleware"
	"github.com/easp-platform/easp/internal/models"
	"github.com/easp-platform/easp/internal/repositories"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	userRepo *repositories.UserRepository
}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{
		userRepo: repositories.NewUserRepository(),
	}
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	TenantID    string `json:"tenant_id" binding:"required"`
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=6"`
	DisplayName string `json:"display_name"`
}

// LoginRequest 登录请求
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// RefreshRequest 刷新Token请求
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// AuthResponse 认证响应
type AuthResponse struct {
	User      *models.User    `json:"user"`
	TokenPair *auth.TokenPair `json:"tokens"`
}

// Register 用户注册
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查邮箱是否已存在（包括已删除的用户，避免重复注册）
	existingUser, _ := h.userRepo.GetByEmail(req.Email)
	if existingUser != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
		return
	}

	// 验证租户是否存在
	var tenant models.Tenant
	err := database.DB.Get(&tenant, "SELECT * FROM tenants WHERE id = ?", req.TenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tenant ID does not exist"})
		return
	}

	// 检查租户状态
	if tenant.Status != "active" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Tenant is not active"})
		return
	}

	// 检查租户是否到期
	if tenant.ExpiresAt != nil && tenant.ExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Tenant has expired"})
		return
	}

	// 检查用户数上限
	if tenant.MaxUsers > 0 {
		userCount, _ := h.userRepo.CountByTenant(req.TenantID)
		if userCount >= tenant.MaxUsers {
			c.JSON(http.StatusForbidden, gin.H{"error": "Tenant user limit reached"})
			return
		}
	}

	// 加密密码
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Failed to hash password: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}

	// 创建用户
	user := &models.User{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		Email:        req.Email,
		DisplayName:  req.DisplayName,
		PasswordHash: string(passwordHash),
		Status:       "active",
		
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := h.userRepo.Create(user); err != nil {
		log.Printf("Failed to create user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// 自动分配默认角色 (role-user)
	roleRepo := repositories.NewUserRoleRepository()
	roleRepo.Assign(user.ID, "role-user")

	c.JSON(http.StatusCreated, gin.H{
		"message": "Registration successful",
		"user":    user,
	})
}

// Login 用户登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 查找用户（GetByEmail 已排除 deleted_at）
	user, err := h.userRepo.GetByEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	// 检查用户状态
	if user.Status != "active" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Account is not active"})
		return
	}

	// 检查租户状态和到期
	var tenant models.Tenant
	if err := database.DB.Get(&tenant, "SELECT * FROM tenants WHERE id = ?", user.TenantID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Tenant not found"})
		return
	}
	if tenant.Status != "active" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Tenant is not active"})
		return
	}
	if tenant.ExpiresAt != nil && tenant.ExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Tenant has expired, please contact administrator"})
		return
	}

	// 获取用户角色
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

	// 生成Token
	tokenPair, err := auth.GenerateTokenPair(user.ID, user.TenantID, user.Email, roleIDs)
	if err != nil {
		log.Printf("Failed to generate token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// 更新登录信息
	user.LastLoginAt = &time.Time{}
	*user.LastLoginAt = time.Now()
	user.LoginCount++
	h.userRepo.Update(user)

	// 检查是否为admin + 收集角色名和合并tools权限
	isAdmin := false
	roleNames := make([]string, 0, len(roles))
	toolsSet := make(map[string]bool)
	for _, role := range roles {
		roleNames = append(roleNames, role.Name)
		if auth.IsAdminRole(role.ID) {
			isAdmin = true
		}
		if role.Tools != nil {
			var tools []string
			if err := json.Unmarshal([]byte(*role.Tools), &tools); err == nil {
				for _, t := range tools {
					toolsSet[t] = true
				}
			}
		}
	}
	tools := make([]string, 0, len(toolsSet))
	for t := range toolsSet {
		tools = append(tools, t)
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":           user.ID,
			"tenant_id":    user.TenantID,
			"email":        user.Email,
			"display_name": user.DisplayName,
			"status":       user.Status,
			"is_admin":     isAdmin,
			"role_names":   roleNames,
			"tools":        tools,
			"created_at":   user.CreatedAt,
		},
		"tokens": tokenPair,
	})
}

// RefreshToken 刷新Token
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 刷新Token
	tokenPair, err := auth.RefreshAccessToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired refresh token"})
		return
	}

	c.JSON(http.StatusOK, tokenPair)
}

// GetCurrentUser 获取当前用户
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	userID, _, _, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	user, err := h.userRepo.GetByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// 检查是否为admin + 收集角色名和合并tools权限
	roleRepo := repositories.NewUserRoleRepository()
	roles, _ := roleRepo.GetUserRoles(userID)
	isAdmin := false
	roleNames := make([]string, 0, len(roles))
	toolsSet := make(map[string]bool)
	for _, r := range roles {
		roleNames = append(roleNames, r.Name)
		if auth.IsAdminRole(r.ID) {
			isAdmin = true
		}
		// 合并所有角色的 tools
		if r.Tools != nil {
			var tools []string
			if err := json.Unmarshal([]byte(*r.Tools), &tools); err == nil {
				for _, t := range tools {
					toolsSet[t] = true
				}
			}
		}
	}
	tools := make([]string, 0, len(toolsSet))
	for t := range toolsSet {
		tools = append(tools, t)
	}

	c.JSON(http.StatusOK, gin.H{
		"id":           user.ID,
		"tenant_id":    user.TenantID,
		"email":        user.Email,
		"display_name": user.DisplayName,
		"status":       user.Status,
		"is_admin":     isAdmin,
		"role_names":   roleNames,
		"tools":        tools,
		"created_at":   user.CreatedAt,
	})
}

// ChangePassword 修改密码
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID, _, _, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取用户
	user, err := h.userRepo.GetByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid old password"})
		return
	}

	// 加密新密码
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Failed to hash password: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}

	// 更新密码
	user.PasswordHash = string(passwordHash)
	user.UpdatedAt = time.Now()
	if err := h.userRepo.Update(user); err != nil {
		log.Printf("Failed to update password: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	// 生成新Token（保留角色信息）
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
	tokenPair, err := auth.GenerateTokenPair(user.ID, user.TenantID, user.Email, roleIDs)
	if err != nil {
		log.Printf("Failed to generate token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Password changed successfully",
		"tokens":  tokenPair,
	})
}

// InitAdmin 初始化管理员账户
func InitAdmin() {
	userRepo := repositories.NewUserRepository()

	// 检查是否已有管理员
	admin, _ := userRepo.GetByEmail("admin@easp.com")
	if admin != nil {
		// 确保已有admin用户拥有系统管理员角色
		roleRepo := repositories.NewUserRoleRepository()
		roles, _ := roleRepo.GetUserRoles(admin.ID)
		hasAdminRole := false
		for _, r := range roles {
			if r.ID == "sys-admin" {
				hasAdminRole = true
				break
			}
		}
		if !hasAdminRole {
			if err := roleRepo.Assign(admin.ID, "sys-admin"); err != nil {
				log.Printf("Failed to assign sys-admin to existing admin: %v", err)
			} else {
				log.Println("Assigned sys-admin to existing admin@easp.com")
			}
		}
		// 同时分配默认租户的管理员角色
		if admin.TenantID != "" {
			roleRepo2 := repositories.NewRoleRepository()
			tenantAdminRole, _ := roleRepo2.GetByName(admin.TenantID, "管理员")
			if tenantAdminRole != nil {
				roleRepo.Assign(admin.ID, tenantAdminRole.ID)
			}
		}
		return
	}

	// 获取默认租户
	var tenantID string
	err := database.DB.QueryRow("SELECT id FROM tenants LIMIT 1").Scan(&tenantID)
	if err != nil {
		log.Printf("Failed to get tenant: %v", err)
		return
	}

	// 为默认租户创建角色（如果还没有）
	InitTenantDefaultRoles(tenantID)

	// 创建管理员
	passwordHash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	admin = &models.User{
		ID:           uuid.New().String(),
		TenantID:     tenantID,
		Email:        "admin@easp.com",
		DisplayName:  "Administrator",
		PasswordHash: string(passwordHash),
		Status:       "active",
		
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := userRepo.Create(admin); err != nil {
		log.Printf("Failed to create admin: %v", err)
		return
	}

	// 分配系统管理员角色
	roleRepo := repositories.NewUserRoleRepository()
	roleRepo.Assign(admin.ID, "sys-admin")

	// 分配默认租户的管理员角色
	roleRepo2 := repositories.NewRoleRepository()
	tenantAdminRole, _ := roleRepo2.GetByName(tenantID, "管理员")
	if tenantAdminRole != nil {
		roleRepo.Assign(admin.ID, tenantAdminRole.ID)
	}

	log.Println("Admin user created: admin@easp.com / admin123")
}
