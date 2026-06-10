package handlers

import (
	crypto_rand "crypto/rand"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/easp-platform/easp/internal/auth"
	"github.com/easp-platform/easp/internal/middleware"
	"github.com/easp-platform/easp/internal/models"
	"github.com/easp-platform/easp/internal/repositories"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// TenantHandler 租户处理器
type TenantHandler struct {
	repo *repositories.TenantRepository
}

func NewTenantHandler() *TenantHandler {
	return &TenantHandler{repo: repositories.NewTenantRepository()}
}

// CreateTenantRequest 创建租户请求
type CreateTenantRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	AdminEmail  string `json:"admin_email" binding:"required,email"`
	AdminPass   string `json:"admin_pass" binding:"required,min=6"`
}

// Create 创建租户
func (h *TenantHandler) Create(c *gin.Context) {
	var req CreateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 创建租户
	tenant := &models.Tenant{
		Name:   req.Name,
		Plan:   "free",
		Status: "active",
	}

	if err := h.repo.Create(tenant); err != nil {
		log.Printf("Failed to create tenant: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create tenant", "details": err.Error()})
		return
	}

	// 为新租户创建默认角色
	InitTenantDefaultRoles(tenant.ID)

	// 获取管理员角色
	roleRepo := repositories.NewRoleRepository()
	adminRole, _ := roleRepo.GetByName(tenant.ID, "管理员")
	if adminRole == nil {
		log.Printf("Failed to get admin role for tenant %s", tenant.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get admin role"})
		return
	}

	// 创建管理员用户
	userRepo := repositories.NewUserRepository()
	passwordHash, _ := bcrypt.GenerateFromPassword([]byte(req.AdminPass), bcrypt.DefaultCost)
	adminUser := &models.User{
		TenantID:     tenant.ID,
		Email:        req.AdminEmail,
		DisplayName:  "管理员",
		PasswordHash: string(passwordHash),
		Status:       "active",
		
	}

	if err := userRepo.Create(adminUser); err != nil {
		log.Printf("Failed to create admin user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create admin user", "details": err.Error()})
		return
	}

	// 分配管理员角色
	userRoleRepo := repositories.NewUserRoleRepository()
	if err := userRoleRepo.Assign(adminUser.ID, adminRole.ID); err != nil {
		log.Printf("Failed to assign admin role: %v", err)
	}

	c.JSON(http.StatusCreated, gin.H{
		"tenant": tenant,
		"admin": gin.H{
			"id":    adminUser.ID,
			"email": adminUser.Email,
		},
	})
}

// GetByID 获取租户
func (h *TenantHandler) GetByID(c *gin.Context) {
	id := c.Param("tenantId")
	tenant, err := h.repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tenant not found"})
		return
	}
	c.JSON(http.StatusOK, tenant)
}

// List 列出租户（admin返回全部，普通用户只返回自己的租户）
func (h *TenantHandler) List(c *gin.Context) {
	// 检查是否为admin
	isAdmin := false
	roleIDs, _ := c.Get(middleware.ContextRoleIDs)
	if roleIDs != nil {
		var roles []string
		json.Unmarshal([]byte(roleIDs.(string)), &roles)
		for _, role := range roles {
			if auth.IsAdminRole(role) {
				isAdmin = true
				break
			}
		}
	}

	if isAdmin {
		// admin返回全部租户
		tenants, err := h.repo.List()
		if err != nil {
			log.Printf("Failed to list tenants: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list tenants", "details": err.Error()})
			return
		}
		if tenants == nil {
			tenants = []models.Tenant{}
		}
		c.JSON(http.StatusOK, tenants)
	} else {
		// 普通用户只返回自己的租户
		userTenantID, _ := c.Get(middleware.ContextTenantID)
		tenant, err := h.repo.GetByID(userTenantID.(string))
		if err != nil {
			c.JSON(http.StatusOK, []models.Tenant{})
			return
		}
		c.JSON(http.StatusOK, []models.Tenant{*tenant})
	}
}

// Update 更新租户
func (h *TenantHandler) Update(c *gin.Context) {
	id := c.Param("tenantId")
	tenant, err := h.repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tenant not found"})
		return
	}

	// 绑定更新字段
	var req struct {
		Name              string  `json:"name"`
		Plan              string  `json:"plan"`
		Status            string  `json:"status"`
		ExpiresAt         *string `json:"expires_at"` // 字符串，空串=永久有效
		MaxUsers          *int    `json:"max_users"`
		RateLimit         *int    `json:"rate_limit"`
		DailyQuota        *int    `json:"daily_quota"`
		MonthlyQuota      *int    `json:"monthly_quota"`
		DailyTokenQuota   *int    `json:"daily_token_quota"`
		MonthlyTokenQuota *int    `json:"monthly_token_quota"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name != "" {
		tenant.Name = req.Name
	}
	if req.Plan != "" {
		tenant.Plan = req.Plan
	}
	if req.Status != "" {
		tenant.Status = req.Status
	}
	if req.ExpiresAt != nil {
		if *req.ExpiresAt == "" {
			// 空串 = 永久有效
			tenant.ExpiresAt = nil
		} else {
			t, err := time.Parse("2006-01-02 15:04:05", *req.ExpiresAt)
			if err != nil {
				t, err = time.Parse("2006-01-02", *req.ExpiresAt)
			}
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid expires_at format, use YYYY-MM-DD or YYYY-MM-DD HH:MM:SS"})
				return
			}
			tenant.ExpiresAt = &t
		}
	}
	if req.MaxUsers != nil {
		tenant.MaxUsers = *req.MaxUsers
	}
	if req.RateLimit != nil {
		tenant.RateLimit = *req.RateLimit
	}
	if req.DailyQuota != nil {
		tenant.DailyQuota = *req.DailyQuota
	}
	if req.MonthlyQuota != nil {
		tenant.MonthlyQuota = *req.MonthlyQuota
	}
	if req.DailyTokenQuota != nil {
		tenant.DailyTokenQuota = *req.DailyTokenQuota
	}
	if req.MonthlyTokenQuota != nil {
		tenant.MonthlyTokenQuota = *req.MonthlyTokenQuota
	}

	if err := h.repo.Update(tenant); err != nil {
		log.Printf("Failed to update tenant: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update tenant", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tenant)
}

// Delete 删除租户
func (h *TenantHandler) Delete(c *gin.Context) {
	id := c.Param("tenantId")
	if err := h.repo.Delete(id); err != nil {
		log.Printf("Failed to delete tenant: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete tenant", "details": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// UserHandler 用户处理器
type UserHandler struct {
	repo     *repositories.UserRepository
	roleRepo *repositories.UserRoleRepository
}

func NewUserHandler() *UserHandler {
	return &UserHandler{
		repo:     repositories.NewUserRepository(),
		roleRepo: repositories.NewUserRoleRepository(),
	}
}

// Create 创建用户
func (h *UserHandler) Create(c *gin.Context) {
	tenantID := c.Param("tenantId")
	var req struct {
		Email       string `json:"email" binding:"required"`
		Password    string `json:"password" binding:"required,min=6"`
		DisplayName string `json:"display_name"`
		Phone       string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	user := &models.User{
		TenantID:     tenantID,
		Email:        req.Email,
		DisplayName:  req.DisplayName,
		Phone:        req.Phone,
		PasswordHash: string(passwordHash),
		Status:       "active",
	}

	if err := h.repo.Create(user); err != nil {
		log.Printf("Failed to create user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, user)
}

// GetByID 获取用户
func (h *UserHandler) GetByID(c *gin.Context) {
	userID := c.Param("userId")
	user, err := h.repo.GetByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

// ListByTenant 列出租户下的用户
func (h *UserHandler) ListByTenant(c *gin.Context) {
	tenantID := c.Param("tenantId")
	users, err := h.repo.ListByTenant(tenantID)
	if err != nil {
		log.Printf("Failed to list users: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list users", "details": err.Error()})
		return
	}

	// 为每个用户添加is_admin字段和角色名
	roleRepo := repositories.NewUserRoleRepository()
	result := make([]gin.H, 0, len(users))
	for _, user := range users {
		roles, _ := roleRepo.GetUserRoles(user.ID)
		isAdmin := false
		roleNames := make([]string, 0, len(roles))
		for _, r := range roles {
			roleNames = append(roleNames, r.Name)
			if auth.IsAdminRole(r.ID) {
				isAdmin = true
			}
		}
		result = append(result, gin.H{
			"id":           user.ID,
			"tenant_id":    user.TenantID,
			"email":        user.Email,
			"display_name": user.DisplayName,
			"status":       user.Status,
			"is_admin":     isAdmin,
			"role_names":   roleNames,
			"login_count":  user.LoginCount,
			"created_at":   user.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, result)
}

// Update 更新用户
func (h *UserHandler) Update(c *gin.Context) {
	userID := c.Param("userId")
	user, err := h.repo.GetByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if err := c.ShouldBindJSON(user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.repo.Update(user); err != nil {
		log.Printf("Failed to update user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

// Delete 软删除用户
func (h *UserHandler) Delete(c *gin.Context) {
	userID := c.Param("userId")

	// 检查是否为超级管理员（不可删除）
	user, err := h.repo.GetByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	roles, _ := h.roleRepo.GetUserRoles(user.ID)
	for _, r := range roles {
		if auth.IsAdminRole(r.ID) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete admin user"})
			return
		}
	}

	if err := h.repo.Delete(userID); err != nil {
		log.Printf("Failed to delete user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

// Restore 恢复已删除用户
func (h *UserHandler) Restore(c *gin.Context) {
	userID := c.Param("userId")
	if err := h.repo.Restore(userID); err != nil {
		log.Printf("Failed to restore user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to restore user", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "User restored successfully"})
}

// GetUserRoles 获取用户角色
func (h *UserHandler) GetUserRoles(c *gin.Context) {
	userID := c.Param("userId")
	roles, err := h.roleRepo.GetUserRoles(userID)
	if err != nil {
		log.Printf("Failed to get user roles: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user roles", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, roles)
}

// ResetPassword 重置用户密码
// 无body或password为空 → 生成随机密码返回（不保存）
// 带password字段 → 保存密码（确认后生效）
func (h *UserHandler) ResetPassword(c *gin.Context) {
	userID := c.Param("userId")

	var req struct {
		Password string `json:"password"`
	}
	// 允许无body
	_ = c.ShouldBindJSON(&req)

	if req.Password == "" {
		// 生成随机密码：8位，含大小写字母和数字
		const upper = "ABCDEFGHJKLMNPQRSTUVWXYZ"
		const lower = "abcdefghijkmnpqrstuvwxyz"
		const digits = "23456789"
		const all = upper + lower + digits

		var randBytes [8]byte
		_, _ = crypto_rand.Read(randBytes[:])

		pwd := make([]byte, 8)
		pwd[0] = upper[int(randBytes[0])%len(upper)]
		pwd[1] = lower[int(randBytes[1])%len(lower)]
		pwd[2] = digits[int(randBytes[2])%len(digits)]
		for i := 3; i < 8; i++ {
			pwd[i] = all[int(randBytes[i])%len(all)]
		}
		// 打乱顺序
		rand.Shuffle(len(pwd), func(i, j int) { pwd[i], pwd[j] = pwd[j], pwd[i] })
		randomPwd := string(pwd)
		c.JSON(http.StatusOK, gin.H{"password": randomPwd, "saved": false})
		return
	}

	// 保存密码
	user, err := h.repo.GetByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	user.PasswordHash = string(passwordHash)
	if err := h.repo.Update(user); err != nil {
		log.Printf("Failed to reset password: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password reset successfully", "saved": true})
}

// ConnectorHandler 连接器处理器
type ConnectorHandler struct {
	repo *repositories.ConnectorRepository
}

func NewConnectorHandler() *ConnectorHandler {
	return &ConnectorHandler{repo: repositories.NewConnectorRepository()}
}

// Create 创建连接器
func (h *ConnectorHandler) Create(c *gin.Context) {
	tenantID := c.Param("tenantId")
	var connector models.Connector
	if err := c.ShouldBindJSON(&connector); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	connector.TenantID = tenantID

	if err := h.repo.Create(&connector); err != nil {
		log.Printf("Failed to create connector: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create connector", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, connector)
}

// GetByID 获取连接器
func (h *ConnectorHandler) GetByID(c *gin.Context) {
	connectorID := c.Param("connectorId")
	connector, err := h.repo.GetByID(connectorID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Connector not found"})
		return
	}
	c.JSON(http.StatusOK, connector)
}

// ListByTenant 列出租户下的连接器
func (h *ConnectorHandler) ListByTenant(c *gin.Context) {
	tenantID := c.Param("tenantId")
	connectors, err := h.repo.ListByTenant(tenantID)
	if err != nil {
		log.Printf("Failed to list connectors: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list connectors", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, connectors)
}

// Update 更新连接器
func (h *ConnectorHandler) Update(c *gin.Context) {
	connectorID := c.Param("connectorId")
	connector, err := h.repo.GetByID(connectorID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Connector not found"})
		return
	}

	if err := c.ShouldBindJSON(connector); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.repo.Update(connector); err != nil {
		log.Printf("Failed to update connector: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update connector", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, connector)
}

// Delete 删除连接器
func (h *ConnectorHandler) Delete(c *gin.Context) {
	connectorID := c.Param("connectorId")
	if err := h.repo.Delete(connectorID); err != nil {
		log.Printf("Failed to delete connector: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete connector", "details": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// MCPToolHandler MCP工具处理器
type MCPToolHandler struct {
	repo *repositories.MCPToolRepository
}

func NewMCPToolHandler() *MCPToolHandler {
	return &MCPToolHandler{repo: repositories.NewMCPToolRepository()}
}

// Create 创建MCP工具
func (h *MCPToolHandler) Create(c *gin.Context) {
	tenantID := c.Param("tenantId")
	var tool models.MCPTool
	if err := c.ShouldBindJSON(&tool); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tool.TenantID = tenantID

	if err := h.repo.Create(&tool); err != nil {
		log.Printf("Failed to create MCP tool: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create MCP tool", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, tool)
}

// GetByID 获取MCP工具
func (h *MCPToolHandler) GetByID(c *gin.Context) {
	toolID := c.Param("toolId")
	tool, err := h.repo.GetByID(toolID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP tool not found"})
		return
	}
	c.JSON(http.StatusOK, tool)
}

// ListByTenant 列出租户下的MCP工具
func (h *MCPToolHandler) ListByTenant(c *gin.Context) {
	tenantID := c.Param("tenantId")
	
	// 可选参数：只返回启用的工具
	enabledOnly := c.Query("enabled")
	
	var tools []models.MCPTool
	var err error
	
	if enabledOnly == "true" {
		tools, err = h.repo.ListEnabled(tenantID)
	} else {
		tools, err = h.repo.ListByTenant(tenantID)
	}
	
	if err != nil {
		log.Printf("Failed to list MCP tools: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list MCP tools", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tools)
}

// Update 更新MCP工具
func (h *MCPToolHandler) Update(c *gin.Context) {
	toolID := c.Param("toolId")
	tool, err := h.repo.GetByID(toolID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP tool not found"})
		return
	}

	if err := c.ShouldBindJSON(tool); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.repo.Update(tool); err != nil {
		log.Printf("Failed to update MCP tool: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update MCP tool", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tool)
}

// Delete 删除MCP工具
func (h *MCPToolHandler) Delete(c *gin.Context) {
	toolID := c.Param("toolId")
	if err := h.repo.Delete(toolID); err != nil {
		log.Printf("Failed to delete MCP tool: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete MCP tool", "details": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// ToggleEnabled 切换启用状态
func (h *MCPToolHandler) ToggleEnabled(c *gin.Context) {
	toolID := c.Param("toolId")
	enabledStr := c.Query("enabled")
	enabled, _ := strconv.ParseBool(enabledStr)
	
	if err := h.repo.ToggleEnabled(toolID, enabled); err != nil {
		log.Printf("Failed to toggle MCP tool: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to toggle MCP tool", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"enabled": enabled})
}

// SkillHandler Skill处理器
type SkillHandler struct {
	repo *repositories.SkillRepository
}

func NewSkillHandler() *SkillHandler {
	return &SkillHandler{repo: repositories.NewSkillRepository()}
}

// Create 创建Skill
func (h *SkillHandler) Create(c *gin.Context) {
	tenantID := c.Param("tenantId")
	var skill models.Skill
	if err := c.ShouldBindJSON(&skill); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	skill.TenantID = tenantID

	if err := h.repo.Create(&skill); err != nil {
		log.Printf("Failed to create skill: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create skill", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, skill)
}

// GetByID 获取Skill
func (h *SkillHandler) GetByID(c *gin.Context) {
	skillID := c.Param("skillId")
	skill, err := h.repo.GetByID(skillID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Skill not found"})
		return
	}
	c.JSON(http.StatusOK, skill)
}

// ListByTenant 列出租户下的Skill
func (h *SkillHandler) ListByTenant(c *gin.Context) {
	tenantID := c.Param("tenantId")
	
	// 可选参数：按状态过滤
	status := c.Query("status")
	
	var skills []models.Skill
	var err error
	
	if status != "" {
		skills, err = h.repo.ListByStatus(tenantID, status)
	} else {
		skills, err = h.repo.ListByTenant(tenantID)
	}
	
	if err != nil {
		log.Printf("Failed to list skills: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list skills", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, skills)
}

// Update 更新Skill
func (h *SkillHandler) Update(c *gin.Context) {
	skillID := c.Param("skillId")
	skill, err := h.repo.GetByID(skillID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Skill not found"})
		return
	}

	if err := c.ShouldBindJSON(skill); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.repo.Update(skill); err != nil {
		log.Printf("Failed to update skill: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update skill", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, skill)
}

// Delete 删除Skill
func (h *SkillHandler) Delete(c *gin.Context) {
	skillID := c.Param("skillId")
	if err := h.repo.Delete(skillID); err != nil {
		log.Printf("Failed to delete skill: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete skill", "details": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// MemoryHandler 记忆处理器
type MemoryHandler struct {
	poolRepo   *repositories.MemoryPoolRepository
	entryRepo  *repositories.MemoryEntryRepository
}

func NewMemoryHandler() *MemoryHandler {
	return &MemoryHandler{
		poolRepo:  repositories.NewMemoryPoolRepository(),
		entryRepo: repositories.NewMemoryEntryRepository(),
	}
}

// CreatePool 创建记忆池
func (h *MemoryHandler) CreatePool(c *gin.Context) {
	tenantID := c.Param("tenantId")
	var pool models.MemoryPool
	if err := c.ShouldBindJSON(&pool); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	pool.TenantID = tenantID

	if err := h.poolRepo.Create(&pool); err != nil {
		log.Printf("Failed to create memory pool: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create memory pool", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, pool)
}

// ListPools 列出记忆池
func (h *MemoryHandler) ListPools(c *gin.Context) {
	tenantID := c.Param("tenantId")
	pools, err := h.poolRepo.ListByTenant(tenantID)
	if err != nil {
		log.Printf("Failed to list memory pools: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list memory pools", "details": err.Error()})
		return
	}
	if pools == nil {
		pools = []models.MemoryPool{}
	}
	c.JSON(http.StatusOK, pools)
}

// CreateEntry 创建记忆条目
func (h *MemoryHandler) CreateEntry(c *gin.Context) {
	poolID := c.Param("poolId")
	var entry models.MemoryEntry
	if err := c.ShouldBindJSON(&entry); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	entry.PoolID = poolID

	if err := h.entryRepo.Create(&entry); err != nil {
		log.Printf("Failed to create memory entry: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create memory entry", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, entry)
}

// ListEntries 列出记忆条目
func (h *MemoryHandler) ListEntries(c *gin.Context) {
	poolID := c.Param("poolId")
	entryType := c.Query("type")
	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)
	
	var entries []models.MemoryEntry
	var err error
	
	if entryType != "" {
		entries, err = h.entryRepo.ListByType(poolID, entryType, limit)
	} else {
		entries, err = h.entryRepo.ListByPool(poolID, limit)
	}
	
	if err != nil {
		log.Printf("Failed to list memory entries: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list memory entries", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, entries)
}

// SearchEntries 搜索记忆条目
func (h *MemoryHandler) SearchEntries(c *gin.Context) {
	poolID := c.Param("poolId")
	keyword := c.Query("q")
	limitStr := c.DefaultQuery("limit", "20")
	limit, _ := strconv.Atoi(limitStr)
	
	if keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search keyword is required"})
		return
	}
	
	entries, err := h.entryRepo.SearchByContent(poolID, keyword, limit)
	if err != nil {
		log.Printf("Failed to search memory entries: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search memory entries", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, entries)
}

// GetPool 获取记忆池详情
func (h *MemoryHandler) GetPool(c *gin.Context) {
	poolID := c.Param("poolId")
	pool, err := h.poolRepo.GetByID(poolID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Memory pool not found"})
		return
	}
	c.JSON(http.StatusOK, pool)
}

// UpdatePool 更新记忆池
func (h *MemoryHandler) UpdatePool(c *gin.Context) {
	poolID := c.Param("poolId")
	pool, err := h.poolRepo.GetByID(poolID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Memory pool not found"})
		return
	}

	if err := c.ShouldBindJSON(pool); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.poolRepo.Update(pool); err != nil {
		log.Printf("Failed to update memory pool: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update memory pool", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, pool)
}

// DeletePool 删除记忆池
func (h *MemoryHandler) DeletePool(c *gin.Context) {
	poolID := c.Param("poolId")
	if err := h.poolRepo.Delete(poolID); err != nil {
		log.Printf("Failed to delete memory pool: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete memory pool", "details": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// AuditLogHandler 审计日志处理器
type AuditLogHandler struct {
	repo *repositories.AuditLogRepository
}

func NewAuditLogHandler() *AuditLogHandler {
	return &AuditLogHandler{repo: repositories.NewAuditLogRepository()}
}

// ListByTenant 列出租户下的审计日志
func (h *AuditLogHandler) ListByTenant(c *gin.Context) {
	tenantID := c.Param("tenantId")
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)
	
	logs, err := h.repo.ListByTenant(tenantID, limit, offset)
	if err != nil {
		log.Printf("Failed to list audit logs: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list audit logs", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, logs)
}
