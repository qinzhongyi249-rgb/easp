package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/easp-platform/easp/internal/auth"
	"github.com/easp-platform/easp/internal/middleware"
	"github.com/easp-platform/easp/internal/models"
	"github.com/easp-platform/easp/internal/repositories"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RoleHandler 角色处理器
type RoleHandler struct {
	roleRepo     *repositories.RoleRepository
	userRepo     *repositories.UserRepository
	userRoleRepo *repositories.UserRoleRepository
}

func NewRoleHandler() *RoleHandler {
	return &RoleHandler{
		roleRepo:     repositories.NewRoleRepository(),
		userRepo:     repositories.NewUserRepository(),
		userRoleRepo: repositories.NewUserRoleRepository(),
	}
}

// CreateRoleRequest 创建角色请求
type CreateRoleRequest struct {
	Name            string `json:"name" binding:"required"`
	Description     string `json:"description"`
	Tools           string `json:"tools"`
	AllowedMCPTools string `json:"allowed_mcp_tools"`
	AllowedSkills   string `json:"allowed_skills"`
	RateLimit       string `json:"rate_limit"`
	DataScope       string `json:"data_scope"`
}

// UpdateRoleRequest 更新角色请求
type UpdateRoleRequest struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	Tools           string `json:"tools"`
	AllowedMCPTools string `json:"allowed_mcp_tools"`
	AllowedSkills   string `json:"allowed_skills"`
	RateLimit       string `json:"rate_limit"`
	DataScope       string `json:"data_scope"`
}

// AssignRoleRequest 分配角色请求
type AssignRoleRequest struct {
	UserID string `json:"user_id" binding:"required"`
	RoleID string `json:"role_id" binding:"required"`
}

// toPtr 转换为指针
func toPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// CreateRole 创建角色
func (h *RoleHandler) CreateRole(c *gin.Context) {
	tenantID := c.Param("tenantId")

	var req CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	role := &models.Role{
		ID:              uuid.New().String(),
		TenantID:        tenantID,
		Name:            req.Name,
		Description:     toPtr(req.Description),
		Tools:           toPtr(req.Tools),
		AllowedMCPTools: toPtr(req.AllowedMCPTools),
		AllowedSkills:   toPtr(req.AllowedSkills),
		RateLimit:       toPtr(req.RateLimit),
		DataScope:       toPtr(req.DataScope),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := h.roleRepo.Create(role); err != nil {
		log.Printf("Failed to create role: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create role"})
		return
	}

	c.JSON(http.StatusCreated, role)
}

// GetRole 获取角色
func (h *RoleHandler) GetRole(c *gin.Context) {
	roleID := c.Param("roleId")

	role, err := h.roleRepo.GetByID(roleID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Role not found"})
		return
	}

	c.JSON(http.StatusOK, role)
}

// ListRoles 列出角色
func (h *RoleHandler) ListRoles(c *gin.Context) {
	tenantID := c.Param("tenantId")

	// 获取租户级角色。租户管理员只能看到/分配租户级角色，不能看到系统级超级管理员角色。
	roles, err := h.roleRepo.ListByTenant(tenantID)
	if err != nil {
		log.Printf("Failed to list roles: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list roles"})
		return
	}

	response := gin.H{
		"tenant_roles": roles,
		"system_roles": []models.Role{},
	}

	if currentUserIsSystemAdmin(c) {
		// 系统级角色只给系统管理员作为只读参考；普通租户管理员不返回，避免前端误用于分配。
		systemRoles, err := h.roleRepo.ListSystem()
		if err != nil {
			log.Printf("Failed to list system roles: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list system roles"})
			return
		}
		response["system_roles"] = systemRoles
	}

	c.JSON(http.StatusOK, response)
}

func currentUserIsSystemAdmin(c *gin.Context) bool {
	roleIDs, exists := c.Get(middleware.ContextRoleIDs)
	if !exists || roleIDs == nil {
		return false
	}

	var roles []string
	switch v := roleIDs.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &roles); err != nil {
			return false
		}
	case []string:
		roles = v
	default:
		return false
	}

	for _, roleID := range roles {
		if auth.IsAdminRole(roleID) {
			return true
		}
	}
	return false
}

// ListSystemRoles 列出系统级角色（仅系统管理员可访问）
func (h *RoleHandler) ListSystemRoles(c *gin.Context) {
	roles, err := h.roleRepo.ListSystem()
	if err != nil {
		log.Printf("Failed to list system roles: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list system roles"})
		return
	}

	c.JSON(http.StatusOK, roles)
}

// UpdateRole 更新角色
func (h *RoleHandler) UpdateRole(c *gin.Context) {
	roleID := c.Param("roleId")

	role, err := h.roleRepo.GetByID(roleID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Role not found"})
		return
	}

	var req UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name != "" {
		role.Name = req.Name
	}
	if req.Description != "" {
		role.Description = toPtr(req.Description)
	}
	if req.Tools != "" {
		role.Tools = toPtr(req.Tools)
	}
	// allowed_mcp_tools 和 allowed_skills 支持清空（传 "[]" 表示清空）
	allowedMCPTools := req.AllowedMCPTools
	if IsTenantAdminRole(role) {
		lockedIDs, err := GetLockedBuiltinMCPToolIDs(role.TenantID)
		if err != nil {
			log.Printf("Failed to load locked builtin MCP tools for role %s: %v", role.ID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load locked MCP tools"})
			return
		}
		protected, err := ProtectedAllowedMCPToolsForRole(role, allowedMCPTools, lockedIDs)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		allowedMCPTools = protected
	}
	role.AllowedMCPTools = toPtr(allowedMCPTools)
	allowedSkills := req.AllowedSkills
	if IsTenantAdminRole(role) {
		lockedSkillIDs, err := GetLockedBuiltinSkillIDs(role.TenantID)
		if err != nil {
			log.Printf("Failed to load locked builtin skills for role %s: %v", role.ID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load locked skills"})
			return
		}
		protected, err := ProtectedAllowedSkillsForRole(role.TenantID, allowedSkills, lockedSkillIDs)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		allowedSkills = protected
	}
	role.AllowedSkills = toPtr(allowedSkills)
	if req.RateLimit != "" {
		role.RateLimit = toPtr(req.RateLimit)
	}
	if req.DataScope != "" {
		role.DataScope = toPtr(req.DataScope)
	}
	role.UpdatedAt = time.Now()

	if err := h.roleRepo.Update(role); err != nil {
		log.Printf("Failed to update role: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update role"})
		return
	}

	c.JSON(http.StatusOK, role)
}

// DeleteRole 删除角色
func (h *RoleHandler) DeleteRole(c *gin.Context) {
	roleID := c.Param("roleId")

	if err := h.roleRepo.Delete(roleID); err != nil {
		log.Printf("Failed to delete role: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete role"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role deleted successfully"})
}

// AssignRole 分配角色给用户
func (h *RoleHandler) AssignRole(c *gin.Context) {
	var req AssignRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	role, err := h.roleRepo.GetByID(req.RoleID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Role not found"})
		return
	}
	targetUser, err := h.userRepo.GetByID(req.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if role.IsSystem {
		if !currentUserIsSystemAdmin(c) {
			c.JSON(http.StatusForbidden, gin.H{"error": "SYSTEM_ROLE_NOT_ASSIGNABLE", "message": "租户管理员不能分配系统级角色"})
			return
		}
	} else if role.TenantID != targetUser.TenantID {
		c.JSON(http.StatusForbidden, gin.H{"error": "ROLE_TENANT_MISMATCH", "message": "不能分配其他租户的角色"})
		return
	}
	if requestTenantID, ok := c.Get(middleware.ContextTenantID); ok && !currentUserIsSystemAdmin(c) && requestTenantID.(string) != targetUser.TenantID {
		c.JSON(http.StatusForbidden, gin.H{"error": "USER_TENANT_MISMATCH", "message": "不能给其他租户用户分配角色"})
		return
	}

	if err := h.userRoleRepo.Assign(req.UserID, req.RoleID); err != nil {
		log.Printf("Failed to assign role: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign role"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role assigned successfully"})
}

// RevokeRole 撤销用户角色
func (h *RoleHandler) RevokeRole(c *gin.Context) {
	userID := c.Param("userId")
	roleID := c.Param("roleId")

	if err := h.userRoleRepo.Revoke(userID, roleID); err != nil {
		log.Printf("Failed to revoke role: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke role"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role revoked successfully"})
}

// GetUserRoles 获取用户角色
func (h *RoleHandler) GetUserRoles(c *gin.Context) {
	userID := c.Param("userId")

	roles, err := h.userRoleRepo.GetUserRoles(userID)
	if err != nil {
		log.Printf("Failed to get user roles: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user roles"})
		return
	}

	c.JSON(http.StatusOK, roles)
}

// GetRoleUsers 获取角色下的用户
func (h *RoleHandler) GetRoleUsers(c *gin.Context) {
	roleID := c.Param("roleId")

	users, err := h.userRoleRepo.GetRoleUsers(roleID)
	if err != nil {
		log.Printf("Failed to get role users: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get role users"})
		return
	}

	c.JSON(http.StatusOK, users)
}

// InitDefaultRoles 初始化系统级角色
func InitDefaultRoles() {
	roleRepo := repositories.NewRoleRepository()

	systemRoles := []struct {
		id          string
		name        string
		description string
		tools       string
		rateLimit   string
		dataScope   string
	}{
		{"sys-admin", "超级管理员", "全平台权限，管理所有租户", `["*"]`, "unlimited", "global"},
		{"sys-operator", "平台运维", "管理连接器和MCP配置", `["connectors","mcp-tools","skills"]`, "1000/hour", "global"},
	}

	for _, def := range systemRoles {
		existing, _ := roleRepo.GetByID(def.id)
		if existing != nil {
			// 更新已有系统角色
			existing.Name = def.name
			existing.Description = &def.description
			existing.Tools = &def.tools
			existing.RateLimit = &def.rateLimit
			existing.DataScope = &def.dataScope
			existing.IsSystem = true
			roleRepo.Update(existing)
			continue
		}

		role := &models.Role{
			ID:          def.id,
			TenantID:    "system",
			Name:        def.name,
			Description: &def.description,
			Tools:       &def.tools,
			RateLimit:   &def.rateLimit,
			DataScope:   &def.dataScope,
			IsSystem:    true,
		}
		if err := roleRepo.Create(role); err != nil {
			log.Printf("Failed to create system role %s: %v", def.name, err)
		} else {
			log.Printf("System role created: %s", def.name)
		}
	}

	// 确保 admin 用户拥有 sys-admin 角色
	ensureAdminRole()

	// 加载所有租户管理员角色到 AdminRoleIDs 集合
	loadAllAdminRoleIDs()

	log.Printf("System roles initialized. AdminRoleIDs: %v", auth.AdminRoleIDs)
}

// InitTenantDefaultRoles 为租户创建默认角色（新租户注册时调用）
func InitTenantDefaultRoles(tenantID string) {
	roleRepo := repositories.NewRoleRepository()

	defaultRoles := []struct {
		name        string
		description string
		tools       string
		rateLimit   string
		dataScope   string
	}{
		{"管理员", "租户管理员，管理本租户用户和角色", `["*"]`, "1000/hour", "tenant"},
		{"开发者", "开发者，管理连接器和工具", `["connectors","mcp-tools","skills"]`, "500/hour", "tenant"},
		{"普通用户", "普通用户，使用工具", `["mcp-tools"]`, "100/hour", "self"},
	}

	for _, def := range defaultRoles {
		// 检查是否已存在
		existing, _ := roleRepo.GetByName(tenantID, def.name)
		if existing != nil {
			continue
		}

		role := &models.Role{
			TenantID:    tenantID,
			Name:        def.name,
			Description: &def.description,
			Tools:       &def.tools,
			RateLimit:   &def.rateLimit,
			DataScope:   &def.dataScope,
			IsSystem:    false,
			IsDefault:   true,
		}
		if err := roleRepo.Create(role); err != nil {
			log.Printf("Failed to create tenant role %s for %s: %v", def.name, tenantID, err)
		} else {
			log.Printf("Tenant role created: %s for tenant %s", def.name, tenantID)
		}
	}

	if _, err := EnsureTenantBuiltinMCPTools(tenantID); err != nil {
		log.Printf("Failed to ensure tenant builtin MCP tools for %s: %v", tenantID, err)
	}
	EnsureTenantBuiltinSkillsOrLog(tenantID)
}

// ensureAdminRole 确保admin用户拥有系统管理员角色
func ensureAdminRole() {
	userRepo := repositories.NewUserRepository()
	admin, _ := userRepo.GetByEmail("admin@easp.com")
	if admin == nil {
		return
	}

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
			log.Printf("Failed to assign sys-admin to admin user: %v", err)
		} else {
			log.Println("Assigned sys-admin to admin@easp.com")
		}
	}
}

// loadAllAdminRoleIDs 从数据库加载系统级管理员角色ID到 AdminRoleIDs 集合
// 只加载 is_system=true 的角色（如 sys-admin），不加载租户级"管理员"角色
func loadAllAdminRoleIDs() {
	roleRepo := repositories.NewRoleRepository()
	roles, err := roleRepo.ListAll()
	if err != nil {
		log.Printf("Failed to load admin role IDs: %v", err)
		return
	}
	count := 0
	for _, r := range roles {
		if r.IsSystem {
			auth.AddAdminRole(r.ID)
			count++
		}
	}
	log.Printf("Loaded %d system admin role IDs into AdminRoleIDs", count)
}
