package handlers

import (
	crypto_rand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/easp-platform/easp/internal/auth"
	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/middleware"
	"github.com/easp-platform/easp/internal/models"
	"github.com/easp-platform/easp/internal/repositories"
	skillpkg "github.com/easp-platform/easp/internal/skill"
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

func randomHexString(n int) string {
	b := make([]byte, n)
	if _, err := crypto_rand.Read(b); err != nil {
		return fmt.Sprintf("%d%x", time.Now().UnixNano(), n)
	}
	return hex.EncodeToString(b)
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func jsonStringPtr(v any) *string {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	s := string(b)
	return &s
}

// CreateEmbedApp 创建嵌入式接入应用，返回 app_secret 仅此一次。
func (h *UserHandler) CreateEmbedApp(c *gin.Context) {
	tenantID := c.Param("tenantId")
	var req struct {
		Name            string   `json:"name" binding:"required"`
		ExternalSystem  string   `json:"external_system" binding:"required"`
		AllowedOrigins  []string `json:"allowed_origins"`
		AllowedScopes   []string `json:"allowed_scopes"`
		TokenTTLSeconds int      `json:"token_ttl_seconds"`
		DefaultRoleIDs  []string `json:"default_role_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tenant, err := repositories.NewTenantRepository().GetByID(tenantID)
	if err != nil || tenant.Status != "active" || (tenant.ExpiresAt != nil && tenant.ExpiresAt.Before(time.Now())) {
		c.JSON(http.StatusForbidden, gin.H{"error": "TENANT_UNAVAILABLE", "message": "租户不存在、未启用或已到期"})
		return
	}
	appID := "app_" + randomHexString(12)
	appSecret := "easp_secret_" + randomHexString(32)
	app := &models.TenantEmbedApp{
		TenantID:        tenantID,
		AppID:           appID,
		AppSecretHash:   sha256Hex(appSecret),
		Name:            req.Name,
		ExternalSystem:  strings.TrimSpace(req.ExternalSystem),
		AllowedOrigins:  jsonStringPtr(req.AllowedOrigins),
		AllowedScopes:   jsonStringPtr(req.AllowedScopes),
		TokenTTLSeconds: req.TokenTTLSeconds,
		AutoCreateUser:  false,
		DefaultRoleIDs:  jsonStringPtr(req.DefaultRoleIDs),
		Status:          "active",
	}
	if len(req.AllowedScopes) == 0 {
		app.AllowedScopes = jsonStringPtr([]string{"assistant:chat", "assistant:history"})
	}
	if err := repositories.NewTenantEmbedAppRepository().Create(app); err != nil {
		log.Printf("CreateEmbedApp failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create embed app"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"app": app, "app_secret": appSecret})
}

func (h *UserHandler) ListEmbedApps(c *gin.Context) {
	apps, err := repositories.NewTenantEmbedAppRepository().ListByTenant(c.Param("tenantId"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list embed apps"})
		return
	}
	if apps == nil {
		apps = []models.TenantEmbedApp{}
	}
	c.JSON(http.StatusOK, apps)
}

func parseJSONStringArray(value *string) []string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return []string{}
	}
	var items []string
	if err := json.Unmarshal([]byte(*value), &items); err != nil {
		return []string{}
	}
	return items
}

func (h *UserHandler) GetEmbedAppGuide(c *gin.Context) {
	tenantID := c.Param("tenantId")
	app, err := repositories.NewTenantEmbedAppRepository().GetByID(tenantID, c.Param("appId"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "EMBED_APP_NOT_FOUND", "message": "接入应用不存在"})
		return
	}
	baseURL := strings.TrimRight(c.GetHeader("X-Forwarded-Proto")+"://"+c.GetHeader("Host"), "://")
	if c.GetHeader("X-Forwarded-Proto") == "" {
		baseURL = "http://" + c.GetHeader("Host")
	}
	c.JSON(http.StatusOK, gin.H{
		"tenant_id":         tenantID,
		"app_id":            app.AppID,
		"app_name":          app.Name,
		"external_system":   app.ExternalSystem,
		"allowed_origins":   parseJSONStringArray(app.AllowedOrigins),
		"allowed_scopes":    parseJSONStringArray(app.AllowedScopes),
		"token_ttl_seconds": app.TokenTTLSeconds,
		"endpoints": gin.H{
			"token_exchange":  "/api/v1/embed/token/exchange",
			"assistant_frame": "/embed/assistant-frame.html",
			"sdk":             "/embed/assistant.js",
		},
		"examples": gin.H{
			"iframe":            fmt.Sprintf(`<iframe src="%s/embed/assistant-frame.html?token=EASP_API_TOKEN" style="width:100%%;height:600px;border:0;"></iframe>`, baseURL),
			"sdk":               `EASPAssistant.mount({ container: '#assistant', token: easpApiToken });`,
			"signature_payload": gin.H{"tenant_id": tenantID, "external_system": app.ExternalSystem, "external_user_id": "当前业务用户ID"},
		},
		"warnings": []string{"app_secret 只允许存放在业务系统服务端", "前端只接收短期 easp-api-token", "外部用户未导入时 Token 换取会返回 EXTERNAL_USER_NOT_IMPORTED"},
	})
}

func (h *UserHandler) DiagnoseEmbedApp(c *gin.Context) {
	tenantID := c.Param("tenantId")
	app, err := repositories.NewTenantEmbedAppRepository().GetByID(tenantID, c.Param("appId"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "EMBED_APP_NOT_FOUND", "message": "接入应用不存在"})
		return
	}
	var req struct {
		Origin         string `json:"origin"`
		ExternalUserID string `json:"external_user_id"`
	}
	_ = c.ShouldBindJSON(&req)
	checks := []gin.H{}
	add := func(key, label string, ok bool, code, suggestion string) {
		item := gin.H{"key": key, "label": label, "ok": ok}
		if !ok {
			item["code"] = code
			item["suggestion"] = suggestion
		}
		checks = append(checks, item)
	}
	add("app_status", "接入应用状态", app.Status == "active", "EMBED_APP_DISABLED", "请启用接入应用后再联调")
	tenant, terr := repositories.NewTenantRepository().GetByID(tenantID)
	tenantOK := terr == nil && tenant.Status == "active" && (tenant.ExpiresAt == nil || tenant.ExpiresAt.After(time.Now()))
	add("tenant_status", "租户状态", tenantOK, "TENANT_UNAVAILABLE", "请确认租户存在、启用且未到期")
	originOK := req.Origin == "" || isOriginAllowed(req.Origin, app.AllowedOrigins)
	add("origin_allowed", "来源白名单", originOK, "ORIGIN_NOT_ALLOWED", "请把业务系统 Origin 加入允许来源，或留空表示不限制")
	userOK := false
	if strings.TrimSpace(req.ExternalUserID) != "" {
		if binding, err := repositories.NewExternalUserBindingRepository().GetActive(tenantID, app.ExternalSystem, strings.TrimSpace(req.ExternalUserID)); err == nil {
			var u models.User
			userOK = database.DB.Get(&u, "SELECT * FROM users WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL AND status = 'active'", binding.UserID, tenantID) == nil
		}
	}
	add("external_user", "外部用户绑定", userOK, "EXTERNAL_USER_NOT_IMPORTED", "请先在外部用户中导入该 external_user_id，或使用服务端同步接口导入")
	canIssue := app.Status == "active" && tenantOK && originOK && userOK
	c.JSON(http.StatusOK, gin.H{
		"can_issue_token": canIssue,
		"app_id":          app.AppID,
		"external_system": app.ExternalSystem,
		"checks":          checks,
	})
}

// ImportExternalUsers 导入外部业务用户。第一阶段只支持显式导入，不在 token exchange 时自动创建。
func (h *UserHandler) ImportExternalUsers(c *gin.Context) {
	tenantID := c.Param("tenantId")
	var req struct {
		ExternalSystem  string `json:"external_system" binding:"required"`
		DefaultPassword string `json:"default_password"`
		Users           []struct {
			Account        string          `json:"account"`
			ExternalUserID string          `json:"external_user_id" binding:"required"`
			Password       string          `json:"password"`
			UserUID        string          `json:"user_uid"`
			DisplayName    string          `json:"display_name"`
			Email          string          `json:"email"`
			Phone          string          `json:"phone"`
			Avatar         string          `json:"avatar"`
			Department     string          `json:"department"`
			Position       string          `json:"position"`
			RoleIDs        []string        `json:"role_ids"`
			Tags           []string        `json:"tags"`
			Profile        json.RawMessage `json:"profile"`
			Attributes     json.RawMessage `json:"attributes"`
			Identities     []struct {
				Provider       string          `json:"provider" binding:"required"`
				ProviderUserID string          `json:"provider_user_id" binding:"required"`
				UnionID        string          `json:"union_id"`
				OpenID         string          `json:"open_id"`
				DisplayName    string          `json:"display_name"`
				Avatar         string          `json:"avatar"`
				Email          string          `json:"email"`
				Phone          string          `json:"phone"`
				Metadata       json.RawMessage `json:"metadata"`
			} `json:"identities"`
			Metadata json.RawMessage `json:"metadata"`
		} `json:"users" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	bindingRepo := repositories.NewExternalUserBindingRepository()
	identityRepo := repositories.NewUserIdentityBindingRepository()
	imported := make([]gin.H, 0, len(req.Users))
	for _, item := range req.Users {
		metadata := map[string]any{"external_system": req.ExternalSystem, "external_user_id": item.ExternalUserID, "source": "external_import"}
		if item.Department != "" {
			metadata["department"] = item.Department
		}
		if item.Position != "" {
			metadata["position"] = item.Position
		}
		if len(item.Tags) > 0 {
			metadata["tags"] = item.Tags
		}
		if len(item.Metadata) > 0 {
			var custom any
			if json.Unmarshal(item.Metadata, &custom) == nil {
				metadata["external_metadata"] = custom
			}
		}
		metaPtr := jsonStringPtr(metadata)
		account := strings.ToLower(strings.TrimSpace(item.Account))
		if account == "" {
			account = strings.ToLower(strings.TrimSpace(item.ExternalUserID))
		}
		loginPassword := strings.TrimSpace(item.Password)
		if loginPassword == "" {
			loginPassword = strings.TrimSpace(req.DefaultPassword)
		}
		passwordConfigured := loginPassword != ""
		if account == "" {
			imported = append(imported, gin.H{"external_user_id": item.ExternalUserID, "status": "conflict", "error": "ACCOUNT_REQUIRED"})
			continue
		}
		if passwordConfigured && len([]rune(loginPassword)) < 6 {
			imported = append(imported, gin.H{"external_user_id": item.ExternalUserID, "status": "conflict", "error": "PASSWORD_TOO_SHORT"})
			continue
		}

		var user *models.User
		createdUser := false
		if existingBinding, err := bindingRepo.GetActive(tenantID, req.ExternalSystem, item.ExternalUserID); err == nil {
			user, _ = h.repo.GetByID(existingBinding.UserID)
		}
		if user == nil {
			passwordForHash := loginPassword
			if passwordForHash == "" {
				passwordForHash = "external-user-" + randomHexString(16)
			}
			passwordHash, _ := bcrypt.GenerateFromPassword([]byte(passwordForHash), bcrypt.DefaultCost)
			newUser := &models.User{UserUID: strings.TrimSpace(item.UserUID), Account: account, TenantID: tenantID, Email: strings.TrimSpace(item.Email), DisplayName: item.DisplayName, Avatar: strings.TrimSpace(item.Avatar), Phone: strings.TrimSpace(item.Phone), PasswordHash: string(passwordHash), Status: "active", SSOProvider: "external", SSOUserID: item.ExternalUserID, Metadata: metaPtr}
			if len(item.Profile) > 0 {
				s := string(item.Profile)
				newUser.Profile = &s
			}
			if len(item.Attributes) > 0 {
				s := string(item.Attributes)
				newUser.Attributes = &s
			}
			if newUser.DisplayName == "" {
				newUser.DisplayName = item.ExternalUserID
			}
			if err := h.repo.Create(newUser); err != nil {
				// 第一阶段不静默合并邮箱/手机号冲突，避免外部用户误绑定既有 EASP 用户。
				imported = append(imported, gin.H{"external_user_id": item.ExternalUserID, "status": "conflict", "error": err.Error()})
				continue
			}
			user = newUser
			createdUser = true
		} else {
			if user.Account == "" {
				user.Account = account
			}
			user.Email = strings.TrimSpace(item.Email)
			user.Phone = strings.TrimSpace(item.Phone)
			if item.DisplayName != "" {
				user.DisplayName = item.DisplayName
			}
			if strings.TrimSpace(item.Avatar) != "" {
				user.Avatar = strings.TrimSpace(item.Avatar)
			}
			user.Metadata = metaPtr
			if passwordConfigured {
				passwordHash, _ := bcrypt.GenerateFromPassword([]byte(loginPassword), bcrypt.DefaultCost)
				user.PasswordHash = string(passwordHash)
			}
			if err := h.repo.Update(user); err != nil {
				imported = append(imported, gin.H{"external_user_id": item.ExternalUserID, "user_id": user.ID, "status": "conflict", "error": err.Error()})
				continue
			}
		}
		for _, roleID := range item.RoleIDs {
			_ = h.roleRepo.Assign(user.ID, roleID)
		}
		var rawMeta *string
		if len(item.Metadata) > 0 {
			s := string(item.Metadata)
			rawMeta = &s
		}
		binding := &models.ExternalUserBinding{TenantID: tenantID, UserID: user.ID, ExternalSystem: req.ExternalSystem, ExternalUserID: item.ExternalUserID, DisplayName: item.DisplayName, Email: item.Email, Phone: item.Phone, Metadata: rawMeta, Status: "active"}
		if binding.DisplayName == "" {
			binding.DisplayName = user.DisplayName
		}
		if err := bindingRepo.Upsert(binding); err != nil {
			imported = append(imported, gin.H{"external_user_id": item.ExternalUserID, "user_id": user.ID, "status": "binding_failed", "error": err.Error()})
			continue
		}
		for _, identity := range item.Identities {
			var identityMeta *string
			if len(identity.Metadata) > 0 {
				s := string(identity.Metadata)
				identityMeta = &s
			}
			_ = identityRepo.Upsert(&models.UserIdentityBinding{
				TenantID:       tenantID,
				UserID:         user.ID,
				Provider:       strings.TrimSpace(identity.Provider),
				ProviderUserID: strings.TrimSpace(identity.ProviderUserID),
				UnionID:        strings.TrimSpace(identity.UnionID),
				OpenID:         strings.TrimSpace(identity.OpenID),
				ExternalSystem: req.ExternalSystem,
				DisplayName:    identity.DisplayName,
				Avatar:         identity.Avatar,
				Email:          strings.TrimSpace(identity.Email),
				Phone:          strings.TrimSpace(identity.Phone),
				Metadata:       identityMeta,
				Status:         "active",
			})
		}
		resultItem := gin.H{"external_user_id": item.ExternalUserID, "user_id": user.ID, "user_uid": user.UserUID, "account": user.Account, "status": "imported", "login_identifier": user.Account, "password_configured": passwordConfigured}
		if createdUser {
			resultItem["status"] = "created"
		}
		if passwordConfigured {
			resultItem["password_updated"] = !createdUser
		}
		imported = append(imported, resultItem)
	}
	c.JSON(http.StatusOK, gin.H{"items": imported})
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (h *UserHandler) ListExternalUsers(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "500"))
	bindings, err := repositories.NewExternalUserBindingRepository().Search(c.Param("tenantId"), c.Query("external_system"), c.Query("keyword"), c.Query("status"), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list external users"})
		return
	}
	if bindings == nil {
		bindings = []models.ExternalUserBinding{}
	}
	c.JSON(http.StatusOK, bindings)
}

func (h *UserHandler) ListUserIdentities(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
	items, err := repositories.NewUserIdentityBindingRepository().Search(c.Param("tenantId"), c.Query("provider"), c.Query("keyword"), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list user identities"})
		return
	}
	if items == nil {
		items = []models.UserIdentityBinding{}
	}
	c.JSON(http.StatusOK, items)
}

// Create 创建用户
func (h *UserHandler) Create(c *gin.Context) {
	tenantID := c.Param("tenantId")
	var req struct {
		Account     string `json:"account" binding:"required"`
		Email       string `json:"email"`
		Password    string `json:"password" binding:"required,min=6"`
		DisplayName string `json:"display_name"`
		Phone       string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.Account = strings.ToLower(strings.TrimSpace(req.Account))
	if req.Account == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Account is required"})
		return
	}
	if existing, _ := h.repo.GetByTenantAndAccount(tenantID, req.Account); existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Account already exists in this tenant"})
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	user := &models.User{
		TenantID:     tenantID,
		Account:      req.Account,
		Email:        strings.TrimSpace(req.Email),
		DisplayName:  req.DisplayName,
		Phone:        strings.TrimSpace(req.Phone),
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
	identities, _ := repositories.NewUserIdentityBindingRepository().ListByUser(user.TenantID, user.ID)
	if identities == nil {
		identities = []models.UserIdentityBinding{}
	}
	c.JSON(http.StatusOK, gin.H{"user": user, "identities": identities})
}

// ListByTenant 列出租户下的用户
func (h *UserHandler) ListByTenant(c *gin.Context) {
	tenantID := c.Param("tenantId")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
	users, err := h.repo.SearchByTenant(tenantID, strings.TrimSpace(c.Query("keyword")), strings.TrimSpace(c.Query("status")), limit)
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
			"user_uid":     user.UserUID,
			"account":      user.Account,
			"tenant_id":    user.TenantID,
			"email":        user.Email,
			"phone":        user.Phone,
			"display_name": user.DisplayName,
			"avatar":       user.Avatar,
			"status":       user.Status,
			"profile":      user.Profile,
			"attributes":   user.Attributes,
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
	// 删除用户时级联清理外部用户绑定关系
	// 因为外部绑定记录指向已删除用户，保留会导致脏数据显示
	database.DB.Exec("DELETE FROM external_user_bindings WHERE user_id = ?", userID)
	// 同时清理用户角色关系
	database.DB.Exec("DELETE FROM user_roles WHERE user_id = ?", userID)

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
	if err := EnsureConnectorMutable(connector); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	if err := c.ShouldBindJSON(connector); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	connector.ID = connectorID
	connector.IsBuiltin = false
	connector.Locked = false

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
	connector, err := h.repo.GetByID(connectorID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Connector not found"})
		return
	}
	if err := EnsureConnectorMutable(connector); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
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
	if tool.Status == "" {
		tool.Status = skillpkg.SkillStatusDraft
	}
	if tool.RiskLevel == "" {
		tool.RiskLevel = "medium"
	}

	tool.IsBuiltin = false
	tool.Locked = false

	if err := h.repo.Create(&tool); err != nil {
		log.Printf("Failed to create MCP tool: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create MCP tool", "details": err.Error()})
		return
	}

	// 更新连接器工具数量
	if tool.ConnectorID != "" {
		database.DB.Exec("UPDATE connectors SET tools_count = (SELECT COUNT(*) FROM mcp_tools WHERE connector_id = ?), last_sync_at = NOW() WHERE id = ?", tool.ConnectorID, tool.ConnectorID)
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

type MCPToolAuthorizationRole struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Wildcard bool   `json:"wildcard"`
}

type MCPToolGovernanceStatus struct {
	ToolID                  string                     `json:"tool_id"`
	AuthorizationStatus     string                     `json:"authorization_status"`
	AuthorizedRoleCount     int                        `json:"authorized_role_count"`
	AuthorizedRoles         []MCPToolAuthorizationRole `json:"authorized_roles"`
	CurrentUserCanExecute   bool                       `json:"current_user_can_execute"`
	CurrentUserGrantedRoles []MCPToolAuthorizationRole `json:"current_user_granted_roles"`
	BlockReasons            []string                   `json:"block_reasons"`
}

func jsonStringArray(value *string) []string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return []string{}
	}
	var items []string
	if err := json.Unmarshal([]byte(*value), &items); err != nil {
		return []string{}
	}
	return items
}

func containsStringValue(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func roleGrantsMCPTool(role models.Role, toolID string) bool {
	if containsStringValue(jsonStringArray(role.Tools), "*") {
		return true
	}
	return containsStringValue(jsonStringArray(role.AllowedMCPTools), toolID)
}

func isPublishedMCPToolStatus(status string) bool {
	return status == skillpkg.SkillStatusPublished || status == "active"
}

func mcpToolBlockReasons(tool *models.MCPTool) []string {
	reasons := []string{}
	if !tool.Enabled {
		reasons = append(reasons, "工具未启用")
	}
	if !isPublishedMCPToolStatus(tool.Status) {
		reasons = append(reasons, "工具未发布")
	}
	return reasons
}

// GovernanceStatus 获取 MCP 工具治理授权状态
func (h *MCPToolHandler) GovernanceStatus(c *gin.Context) {
	tenantID := c.Param("tenantId")
	toolID := c.Param("toolId")
	tool, err := h.repo.GetByID(toolID)
	if err != nil || tool.TenantID != tenantID {
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP tool not found"})
		return
	}

	roles, err := repositories.NewRoleRepository().ListByTenant(tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list roles"})
		return
	}

	authorizedRoles := []MCPToolAuthorizationRole{}
	for _, role := range roles {
		if role.IsSystem {
			continue
		}
		if roleGrantsMCPTool(role, toolID) {
			authorizedRoles = append(authorizedRoles, MCPToolAuthorizationRole{ID: role.ID, Name: role.Name, Wildcard: containsStringValue(jsonStringArray(role.Tools), "*")})
		}
	}

	currentUserRoles := []models.Role{}
	if userID, ok := c.Get(middleware.ContextUserID); ok {
		currentUserRoles, _ = repositories.NewUserRoleRepository().GetUserRoles(userID.(string))
	}
	currentUserGrantedRoles := []MCPToolAuthorizationRole{}
	for _, role := range currentUserRoles {
		if roleGrantsMCPTool(role, toolID) {
			currentUserGrantedRoles = append(currentUserGrantedRoles, MCPToolAuthorizationRole{ID: role.ID, Name: role.Name, Wildcard: containsStringValue(jsonStringArray(role.Tools), "*")})
		}
	}

	blockReasons := mcpToolBlockReasons(tool)
	if len(currentUserGrantedRoles) == 0 {
		blockReasons = append(blockReasons, "当前用户角色未授权")
	}
	canExecute := len(blockReasons) == 0
	authStatus := "not_granted"
	if len(authorizedRoles) > 0 {
		authStatus = "granted"
	}
	if !tool.Enabled || !isPublishedMCPToolStatus(tool.Status) {
		authStatus = "unavailable"
	}

	c.JSON(http.StatusOK, MCPToolGovernanceStatus{
		ToolID:                  toolID,
		AuthorizationStatus:     authStatus,
		AuthorizedRoleCount:     len(authorizedRoles),
		AuthorizedRoles:         authorizedRoles,
		CurrentUserCanExecute:   canExecute,
		CurrentUserGrantedRoles: currentUserGrantedRoles,
		BlockReasons:            blockReasons,
	})
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
	existing, err := h.repo.GetByID(toolID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP tool not found"})
		return
	}

	if err := EnsureMCPToolMutable(existing); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var req models.MCPTool
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 保留服务端权威字段，避免前端未传 id/tenant_id/connector_id 时被零值覆盖。
	req.ID = existing.ID
	req.TenantID = existing.TenantID
	req.IsBuiltin = existing.IsBuiltin
	req.Locked = existing.Locked
	oldConnectorID := existing.ConnectorID
	if req.ConnectorID == "" {
		req.ConnectorID = existing.ConnectorID
	}
	if req.Name == "" {
		req.Name = existing.Name
	}
	if req.BackendMethod == nil || *req.BackendMethod == "" {
		req.BackendMethod = existing.BackendMethod
	}
	if req.BackendPath == nil || *req.BackendPath == "" {
		req.BackendPath = existing.BackendPath
	}
	if req.RiskLevel == "" {
		req.RiskLevel = existing.RiskLevel
	}
	if req.Status == "" {
		req.Status = existing.Status
	}

	if err := h.repo.Update(&req); err != nil {
		log.Printf("Failed to update MCP tool: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update MCP tool", "details": err.Error()})
		return
	}

	// 如果 connector_id 发生变化，更新新旧连接器的工具数量
	if req.ConnectorID != oldConnectorID {
		if oldConnectorID != "" {
			database.DB.Exec("UPDATE connectors SET tools_count = (SELECT COUNT(*) FROM mcp_tools WHERE connector_id = ?), last_sync_at = NOW() WHERE id = ?", oldConnectorID, oldConnectorID)
		}
		if req.ConnectorID != "" {
			database.DB.Exec("UPDATE connectors SET tools_count = (SELECT COUNT(*) FROM mcp_tools WHERE connector_id = ?), last_sync_at = NOW() WHERE id = ?", req.ConnectorID, req.ConnectorID)
		}
	}

	c.JSON(http.StatusOK, req)
}

// Delete 删除MCP工具
func (h *MCPToolHandler) Delete(c *gin.Context) {
	toolID := c.Param("toolId")
	existing, err := h.repo.GetByID(toolID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP tool not found"})
		return
	}
	if err := EnsureMCPToolMutable(existing); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	connectorID := existing.ConnectorID
	if err := h.repo.Delete(toolID); err != nil {
		log.Printf("Failed to delete MCP tool: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete MCP tool", "details": err.Error()})
		return
	}
	// 更新连接器工具数量
	if connectorID != "" {
		database.DB.Exec("UPDATE connectors SET tools_count = (SELECT COUNT(*) FROM mcp_tools WHERE connector_id = ?), last_sync_at = NOW() WHERE id = ?", connectorID, connectorID)
	}
	c.JSON(http.StatusNoContent, nil)
}

// ToggleEnabled 切换启用状态
func (h *MCPToolHandler) ToggleEnabled(c *gin.Context) {
	toolID := c.Param("toolId")
	existing, err := h.repo.GetByID(toolID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP tool not found"})
		return
	}
	if err := EnsureMCPToolMutable(existing); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
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

func ensureSkillInputSchema(sk *models.Skill) {
	if sk == nil || (sk.InputSchema != nil && strings.TrimSpace(*sk.InputSchema) != "") {
		return
	}

	vars := inferSkillInputVars(sk.Steps)
	if len(vars) == 0 {
		return
	}

	properties := map[string]any{}
	for _, name := range vars {
		properties[name] = map[string]any{
			"type":        "string",
			"title":       humanizeSkillInputName(name),
			"description": humanizeSkillInputName(name),
		}
	}
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   vars,
	}
	if b, err := json.Marshal(schema); err == nil {
		s := string(b)
		sk.InputSchema = &s
	}
}

func inferSkillInputVars(steps string) []string {
	seen := map[string]bool{}
	vars := []string{}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_.-]*)\s*\}\}`),
		regexp.MustCompile(`\$\{\s*([a-zA-Z_][a-zA-Z0-9_.-]*)\s*\}`),
	}
	for _, re := range patterns {
		for _, match := range re.FindAllStringSubmatch(steps, -1) {
			name := strings.TrimSpace(match[1])
			if strings.HasPrefix(name, "steps.") || strings.HasPrefix(name, "outputs.") {
				continue
			}
			name = strings.TrimPrefix(name, "inputs.")
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			vars = append(vars, name)
		}
	}
	return vars
}

func humanizeSkillInputName(name string) string {
	switch name {
	case "user_email", "email":
		return "用户邮箱"
	case "role_name":
		return "角色名称"
	case "role_id":
		return "角色ID"
	case "user_id":
		return "用户ID"
	default:
		return name
	}
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
	ensureSkillInputSchema(&skill)

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
	ensureSkillInputSchema(skill)
	c.JSON(http.StatusOK, skill)
}

// ListByTenant 列出租户下的Skill
func (h *SkillHandler) ListByTenant(c *gin.Context) {
	tenantID := c.Param("tenantId")

	// 可选参数：按状态过滤
	status := c.Query("status")

	var skills []models.Skill
	var err error

	if status == "usable" {
		skills, err = h.repo.ListUsable(tenantID)
	} else if status != "" {
		skills, err = h.repo.ListByStatus(tenantID, status)
	} else {
		skills, err = h.repo.ListByTenant(tenantID)
	}

	if err != nil {
		log.Printf("Failed to list skills: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list skills", "details": err.Error()})
		return
	}
	if skills == nil {
		skills = []models.Skill{}
	}
	for i := range skills {
		ensureSkillInputSchema(&skills[i])
	}
	c.JSON(http.StatusOK, skills)
}

func isSystemBuiltinSkill(sk *models.Skill) bool {
	return sk != nil && sk.CreatedBy != nil && *sk.CreatedBy == "system"
}

// Update 更新Skill
func (h *SkillHandler) Update(c *gin.Context) {
	skillID := c.Param("skillId")
	skill, err := h.repo.GetByID(skillID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Skill not found"})
		return
	}

	if isSystemBuiltinSkill(skill) {
		c.JSON(http.StatusForbidden, gin.H{"error": "内置技能不可编辑"})
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
	skill, err := h.repo.GetByID(skillID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Skill not found"})
		return
	}
	if isSystemBuiltinSkill(skill) {
		c.JSON(http.StatusForbidden, gin.H{"error": "内置技能不可删除"})
		return
	}
	if err := h.repo.Delete(skillID); err != nil {
		log.Printf("Failed to delete skill: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete skill", "details": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// MemoryHandler 记忆处理器
type MemoryHandler struct {
	poolRepo  *repositories.MemoryPoolRepository
	entryRepo *repositories.MemoryEntryRepository
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

	logs, err := h.repo.SearchByTenant(tenantID, repositories.AuditLogFilter{
		SourceType:     c.Query("source_type"),
		SourceAppID:    c.Query("source_app_id"),
		ExternalSystem: c.Query("external_system"),
		ExternalUserID: c.Query("external_user_id"),
		UserUID:        c.Query("user_uid"),
		UserID:         c.Query("user_id"),
		Tool:           c.Query("tool"),
		Action:         c.Query("action"),
	}, limit, offset)
	if err != nil {
		log.Printf("Failed to list audit logs: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list audit logs", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, logs)
}
