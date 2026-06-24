package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/easp-platform/easp/internal/auth"
	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/middleware"
	"github.com/easp-platform/easp/internal/models"
	"github.com/easp-platform/easp/internal/modelservice"
	"github.com/easp-platform/easp/internal/repositories"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// EmbedHandler Embed API 处理器
type EmbedHandler struct {
	chatHandler *ChatHandler
}

func NewEmbedHandler() *EmbedHandler {
	return &EmbedHandler{
		chatHandler: NewChatHandler(),
	}
}

// TokenExchange 业务系统后端用 app_id/app_secret 签名换取 easp-api-token。
// 第一阶段只允许已导入 external_user_bindings 的外部用户换取 Token，不自动创建用户。
func (h *EmbedHandler) TokenExchange(c *gin.Context) {
	var req struct {
		TenantID               string `json:"tenant_id" binding:"required"`
		ExternalSystem         string `json:"external_system" binding:"required"`
		ExternalUserID         string `json:"external_user_id" binding:"required"`
		DisplayName            string `json:"display_name"`
		Email                  string `json:"email"`
		Phone                  string `json:"phone"`
		ExternalAccessToken    string `json:"external_access_token"`
		ExternalTokenType      string `json:"external_token_type"`
		ExternalTokenExpiresAt int64  `json:"external_token_expires_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	appID := strings.TrimSpace(c.GetHeader("X-EASP-App-Id"))
	timestamp := strings.TrimSpace(c.GetHeader("X-EASP-Timestamp"))
	nonce := strings.TrimSpace(c.GetHeader("X-EASP-Nonce"))
	signature := strings.TrimSpace(c.GetHeader("X-EASP-Signature"))
	if appID == "" || timestamp == "" || nonce == "" || signature == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "EASP_SIGNATURE_REQUIRED", "message": "X-EASP-App-Id/Timestamp/Nonce/Signature headers are required"})
		return
	}
	app, err := repositories.NewTenantEmbedAppRepository().GetByAppID(appID)
	if err != nil || app.Status != "active" || app.TenantID != req.TenantID || app.ExternalSystem != req.ExternalSystem {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "INVALID_EMBED_APP", "message": "Invalid or disabled embed app"})
		return
	}
	if !isOriginAllowed(c.GetHeader("Origin"), app.AllowedOrigins) {
		c.JSON(http.StatusForbidden, gin.H{"error": "ORIGIN_NOT_ALLOWED", "message": "Origin is not allowed for this embed app"})
		return
	}
	tenant, err := repositories.NewTenantRepository().GetByID(req.TenantID)
	if err != nil || tenant.Status != "active" || (tenant.ExpiresAt != nil && tenant.ExpiresAt.Before(time.Now())) {
		c.JSON(http.StatusForbidden, gin.H{"error": "TENANT_UNAVAILABLE", "message": "租户不存在、未启用或已到期"})
		return
	}
	ts, err := strconvParseInt(timestamp)
	if err != nil || time.Since(time.Unix(ts, 0)) > 5*time.Minute || time.Until(time.Unix(ts, 0)) > 5*time.Minute {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "INVALID_TIMESTAMP", "message": "Timestamp expired or invalid"})
		return
	}
	bodyMap := map[string]string{"tenant_id": req.TenantID, "external_system": req.ExternalSystem, "external_user_id": req.ExternalUserID}
	if !verifyEmbedSignature(app.AppSecretHash, appID, timestamp, nonce, bodyMap, signature) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "INVALID_SIGNATURE", "message": "Signature verification failed"})
		return
	}
	binding, err := repositories.NewExternalUserBindingRepository().GetActive(req.TenantID, req.ExternalSystem, req.ExternalUserID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "EXTERNAL_USER_NOT_IMPORTED", "message": "外部用户未导入 EASP，无法换取嵌入式 Token"})
		return
	}
	var user models.User
	if err := database.DB.Get(&user, "SELECT * FROM users WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL AND status = 'active'", binding.UserID, req.TenantID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "EASP_USER_INACTIVE", "message": "绑定的 EASP 用户不存在或未启用"})
		return
	}
	scopes := []string{"assistant:chat", "assistant:history"}
	if app.AllowedScopes != nil && *app.AllowedScopes != "" {
		_ = json.Unmarshal([]byte(*app.AllowedScopes), &scopes)
	}
	externalTokenRef := ""
	if strings.TrimSpace(req.ExternalAccessToken) != "" {
		externalTokenExpiresAt := time.Now().Add(time.Duration(app.TokenTTLSeconds) * time.Second)
		if req.ExternalTokenExpiresAt > 0 {
			externalTokenExpiresAt = time.Unix(req.ExternalTokenExpiresAt, 0)
		}
		externalTokenRef = auth.StoreEmbedExternalUserToken(req.ExternalAccessToken, externalTokenExpiresAt)
		if externalTokenRef == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "FAILED_TO_STORE_EXTERNAL_TOKEN", "message": "外部业务 Token 暂存失败"})
			return
		}
	}
	token, exp, err := auth.GenerateEmbedTokenWithExternalTokenRef(req.TenantID, user.ID, user.Email, req.ExternalSystem, req.ExternalUserID, app.AppID, externalTokenRef, scopes, app.TokenTTLSeconds)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}
	database.DB.Exec("UPDATE external_user_bindings SET last_login_at = NOW(), updated_at = NOW() WHERE id = ?", binding.ID)
	repositories.NewTenantEmbedAppRepository().Touch(app.AppID)
	c.Header("easp-api-token", token)
	c.Header("Access-Control-Expose-Headers", "easp-api-token")
	c.JSON(http.StatusOK, gin.H{"token": token, "expires_at": exp, "user": gin.H{"id": user.ID, "tenant_id": user.TenantID, "account": user.Account, "display_name": user.DisplayName}})
}

type EmbedUserSyncRequest struct {
	TenantID        string              `json:"tenant_id" binding:"required"`
	ExternalSystem  string              `json:"external_system" binding:"required"`
	BatchID         string              `json:"batch_id"`
	Mode            string              `json:"mode"` // init/incremental，当前均按幂等 upsert 处理
	DefaultPassword string              `json:"default_password"`
	Users           []EmbedUserSyncItem `json:"users" binding:"required"`
}

type EmbedUserSyncItem struct {
	Account        string              `json:"account"`
	ExternalUserID string              `json:"external_user_id" binding:"required"`
	Password       string              `json:"password"`
	UserUID        string              `json:"user_uid"`
	DisplayName    string              `json:"display_name"`
	Email          string              `json:"email"`
	Phone          string              `json:"phone"`
	Avatar         string              `json:"avatar"`
	Department     string              `json:"department"`
	Position       string              `json:"position"`
	RoleIDs        []string            `json:"role_ids"`
	Tags           []string            `json:"tags"`
	Profile        json.RawMessage     `json:"profile"`
	Attributes     json.RawMessage     `json:"attributes"`
	Identities     []EmbedUserIdentity `json:"identities"`
	Metadata       json.RawMessage     `json:"metadata"`
}

type EmbedUserIdentity struct {
	Provider       string          `json:"provider" binding:"required"`
	ProviderUserID string          `json:"provider_user_id" binding:"required"`
	UnionID        string          `json:"union_id"`
	OpenID         string          `json:"open_id"`
	DisplayName    string          `json:"display_name"`
	Avatar         string          `json:"avatar"`
	Email          string          `json:"email"`
	Phone          string          `json:"phone"`
	Metadata       json.RawMessage `json:"metadata"`
}

// SyncExternalUsers 业务系统服务端使用嵌入应用 AppID + HMAC 签名同步外部用户。
// 这是初始化/异步同步通道，不接受后台 JWT；后端按 external_user_id/user_uid/email/phone/第三方身份做重复性校验和幂等 upsert。
func (h *EmbedHandler) SyncExternalUsers(c *gin.Context) {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_BODY", "message": "读取请求体失败"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var req EmbedUserSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Users) == 0 || len(req.Users) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_USERS_SIZE", "message": "users 数量必须在 1-500 之间"})
		return
	}

	appID := strings.TrimSpace(c.GetHeader("X-EASP-App-Id"))
	timestamp := strings.TrimSpace(c.GetHeader("X-EASP-Timestamp"))
	nonce := strings.TrimSpace(c.GetHeader("X-EASP-Nonce"))
	signature := strings.TrimSpace(c.GetHeader("X-EASP-Signature"))
	if appID == "" || timestamp == "" || nonce == "" || signature == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "EASP_SIGNATURE_REQUIRED", "message": "X-EASP-App-Id/Timestamp/Nonce/Signature headers are required"})
		return
	}
	app, err := repositories.NewTenantEmbedAppRepository().GetByAppID(appID)
	if err != nil || app.Status != "active" || app.TenantID != req.TenantID || app.ExternalSystem != req.ExternalSystem {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "INVALID_EMBED_APP", "message": "Invalid or disabled embed app"})
		return
	}
	tenant, err := repositories.NewTenantRepository().GetByID(req.TenantID)
	if err != nil || tenant.Status != "active" || (tenant.ExpiresAt != nil && tenant.ExpiresAt.Before(time.Now())) {
		c.JSON(http.StatusForbidden, gin.H{"error": "TENANT_UNAVAILABLE", "message": "租户不存在、未启用或已到期"})
		return
	}
	ts, err := strconvParseInt(timestamp)
	if err != nil || time.Since(time.Unix(ts, 0)) > 5*time.Minute || time.Until(time.Unix(ts, 0)) > 5*time.Minute {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "INVALID_TIMESTAMP", "message": "Timestamp expired or invalid"})
		return
	}
	bodyHash := sha256Hex(string(bodyBytes))
	bodyMap := map[string]string{"tenant_id": req.TenantID, "external_system": req.ExternalSystem, "body_sha256": bodyHash}
	if !verifyEmbedSignature(app.AppSecretHash, appID, timestamp, nonce, bodyMap, signature) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "INVALID_SIGNATURE", "message": "Signature verification failed"})
		return
	}

	bindingRepo := repositories.NewExternalUserBindingRepository()
	identityRepo := repositories.NewUserIdentityBindingRepository()
	userRepo := repositories.NewUserRepository()
	roleRepo := repositories.NewRoleRepository()
	userRoleRepo := repositories.NewUserRoleRepository()
	items := make([]gin.H, 0, len(req.Users))
	counts := gin.H{"created": 0, "updated": 0, "bound_existing": 0, "conflict": 0}

	for _, item := range req.Users {
		result, user, status := h.syncOneExternalUser(req.TenantID, req.ExternalSystem, req.DefaultPassword, item, tenant.MaxUsers, userRepo, bindingRepo, identityRepo, roleRepo, userRoleRepo)
		if user != nil {
			result["user_id"] = user.ID
			result["user_uid"] = user.UserUID
		}
		items = append(items, result)
		if v, ok := counts[status].(int); ok {
			counts[status] = v + 1
		}
	}
	repositories.NewTenantEmbedAppRepository().Touch(app.AppID)
	c.JSON(http.StatusOK, gin.H{"batch_id": req.BatchID, "items": items, "summary": counts})
}

func (h *EmbedHandler) syncOneExternalUser(tenantID, externalSystem, defaultPassword string, item EmbedUserSyncItem, maxUsers int, userRepo *repositories.UserRepository, bindingRepo *repositories.ExternalUserBindingRepository, identityRepo *repositories.UserIdentityBindingRepository, roleRepo *repositories.RoleRepository, userRoleRepo *repositories.UserRoleRepository) (gin.H, *models.User, string) {
	externalUserID := strings.TrimSpace(item.ExternalUserID)
	result := gin.H{"external_user_id": externalUserID}
	if externalUserID == "" {
		result["status"] = "conflict"
		result["error"] = "external_user_id required"
		return result, nil, "conflict"
	}

	candidates := map[string]*models.User{}
	candidateSources := map[string][]string{}
	addCandidate := func(source string, u *models.User) {
		if u == nil || u.ID == "" {
			return
		}
		candidates[u.ID] = u
		candidateSources[u.ID] = append(candidateSources[u.ID], source)
	}

	if existingBinding, err := bindingRepo.GetActive(tenantID, externalSystem, externalUserID); err == nil {
		if u, err := userRepo.GetByID(existingBinding.UserID); err == nil {
			addCandidate("external_binding", u)
		}
	}
	if item.UserUID != "" {
		if u, err := getActiveUserByTenantField(tenantID, "user_uid", strings.TrimSpace(item.UserUID)); err == nil {
			addCandidate("user_uid", u)
		}
	}
	account := strings.ToLower(strings.TrimSpace(item.Account))
	if account == "" {
		account = strings.ToLower(strings.TrimSpace(externalUserID))
	}
	if account != "" {
		if u, err := userRepo.GetByTenantAndAccount(tenantID, account); err == nil {
			addCandidate("account", u)
		}
	}
	for _, identity := range item.Identities {
		provider := strings.TrimSpace(identity.Provider)
		providerUserID := strings.TrimSpace(identity.ProviderUserID)
		if provider == "" || providerUserID == "" {
			continue
		}
		if u, err := getActiveUserByIdentity(tenantID, provider, providerUserID); err == nil {
			addCandidate("identity:"+provider, u)
		}
	}
	if len(candidates) > 1 {
		result["status"] = "conflict"
		result["error"] = "DUPLICATE_USER_CANDIDATES"
		result["candidates"] = candidateSources
		return result, nil, "conflict"
	}

	var user *models.User
	status := "created"
	for _, u := range candidates {
		user = u
		status = "bound_existing"
		break
	}

	metadata := buildExternalUserMetadata(externalSystem, item)
	loginPassword := strings.TrimSpace(item.Password)
	if loginPassword == "" {
		loginPassword = strings.TrimSpace(defaultPassword)
	}
	passwordConfigured := loginPassword != ""
	if account == "" {
		result["status"] = "conflict"
		result["error"] = "ACCOUNT_REQUIRED"
		return result, nil, "conflict"
	}
	if passwordConfigured && len([]rune(loginPassword)) < 6 {
		result["status"] = "conflict"
		result["error"] = "PASSWORD_TOO_SHORT"
		return result, nil, "conflict"
	}
	metaPtr := jsonStringPtr(metadata)
	profilePtr := rawJSONPtr(item.Profile)
	attributesPtr := rawJSONPtr(item.Attributes)
	if user == nil {
		if maxUsers > 0 {
			if count, err := userRepo.CountByTenant(tenantID); err == nil && count >= maxUsers {
				result["status"] = "conflict"
				result["error"] = "TENANT_USER_LIMIT_REACHED"
				return result, nil, "conflict"
			}
		}
		passwordForHash := loginPassword
		if passwordForHash == "" {
			passwordForHash = "external-user-" + randomHexString(16)
		}
		passwordHash, _ := bcrypt.GenerateFromPassword([]byte(passwordForHash), bcrypt.DefaultCost)
		newUser := &models.User{UserUID: normalizedExternalUserUID(item.UserUID, externalSystem, externalUserID), Account: account, TenantID: tenantID, Email: strings.TrimSpace(item.Email), DisplayName: item.DisplayName, Avatar: strings.TrimSpace(item.Avatar), Phone: strings.TrimSpace(item.Phone), PasswordHash: string(passwordHash), Status: "active", SSOProvider: "external", SSOUserID: externalUserID, Metadata: metaPtr, Profile: profilePtr, Attributes: attributesPtr}
		if newUser.DisplayName == "" {
			newUser.DisplayName = externalUserID
		}
		if err := userRepo.Create(newUser); err != nil {
			result["status"] = "conflict"
			result["error"] = err.Error()
			return result, nil, "conflict"
		}
		user = newUser
	} else {
		if user.Account == "" {
			user.Account = account
		}
		if strings.TrimSpace(item.Email) != "" {
			user.Email = strings.TrimSpace(item.Email)
		}
		if strings.TrimSpace(item.Phone) != "" {
			user.Phone = strings.TrimSpace(item.Phone)
		}
		if item.DisplayName != "" {
			user.DisplayName = item.DisplayName
		}
		if strings.TrimSpace(item.Avatar) != "" {
			user.Avatar = strings.TrimSpace(item.Avatar)
		}
		user.Metadata = metaPtr
		if profilePtr != nil {
			user.Profile = profilePtr
		}
		if attributesPtr != nil {
			user.Attributes = attributesPtr
		}
		if passwordConfigured {
			passwordHash, _ := bcrypt.GenerateFromPassword([]byte(loginPassword), bcrypt.DefaultCost)
			user.PasswordHash = string(passwordHash)
		}
		if err := userRepo.Update(user); err != nil {
			result["status"] = "conflict"
			result["error"] = err.Error()
			return result, user, "conflict"
		}
		if status != "bound_existing" {
			status = "updated"
		}
	}

	assignedRoles := make([]string, 0, len(item.RoleIDs))
	for _, roleID := range item.RoleIDs {
		roleID = strings.TrimSpace(roleID)
		if roleID == "" {
			continue
		}
		role, err := roleRepo.GetByID(roleID)
		if err != nil || role.IsSystem || role.TenantID != tenantID {
			result["status"] = "conflict"
			result["error"] = "INVALID_ROLE_ID"
			result["role_id"] = roleID
			return result, user, "conflict"
		}
		if err := userRoleRepo.Assign(user.ID, roleID); err == nil {
			assignedRoles = append(assignedRoles, roleID)
		}
	}
	if len(assignedRoles) > 0 {
		result["assigned_role_ids"] = assignedRoles
	}
	binding := &models.ExternalUserBinding{TenantID: tenantID, UserID: user.ID, ExternalSystem: externalSystem, ExternalUserID: externalUserID, DisplayName: item.DisplayName, Email: strings.TrimSpace(item.Email), Phone: strings.TrimSpace(item.Phone), Metadata: rawJSONPtr(item.Metadata), Status: "active"}
	if binding.DisplayName == "" {
		binding.DisplayName = user.DisplayName
	}
	if err := bindingRepo.Upsert(binding); err != nil {
		result["status"] = "conflict"
		result["error"] = err.Error()
		return result, user, "conflict"
	}
	for _, identity := range item.Identities {
		provider := strings.TrimSpace(identity.Provider)
		providerUserID := strings.TrimSpace(identity.ProviderUserID)
		if provider == "" || providerUserID == "" {
			continue
		}
		if existingUser, err := getActiveUserByIdentity(tenantID, provider, providerUserID); err == nil && existingUser.ID != user.ID {
			result["status"] = "conflict"
			result["error"] = "IDENTITY_ALREADY_BOUND"
			result["provider"] = provider
			result["provider_user_id"] = providerUserID
			return result, user, "conflict"
		}
		_ = identityRepo.Upsert(&models.UserIdentityBinding{TenantID: tenantID, UserID: user.ID, Provider: provider, ProviderUserID: providerUserID, UnionID: strings.TrimSpace(identity.UnionID), OpenID: strings.TrimSpace(identity.OpenID), ExternalSystem: externalSystem, DisplayName: identity.DisplayName, Avatar: identity.Avatar, Email: strings.TrimSpace(identity.Email), Phone: strings.TrimSpace(identity.Phone), Metadata: rawJSONPtr(identity.Metadata), Status: "active"})
	}

	result["status"] = status
	result["account"] = user.Account
	result["login_identifier"] = user.Account
	result["password_configured"] = passwordConfigured
	if passwordConfigured {
		result["password_updated"] = status != "created"
	}
	return result, user, status
}

func getActiveUserByTenantField(tenantID, field, value string) (*models.User, error) {
	if value == "" || field != "user_uid" {
		return nil, sql.ErrNoRows
	}
	var user models.User
	err := database.DB.Get(&user, "SELECT * FROM users WHERE tenant_id = ? AND "+field+" = ? AND deleted_at IS NULL AND status = 'active'", tenantID, value)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func getActiveUserByIdentity(tenantID, provider, providerUserID string) (*models.User, error) {
	var user models.User
	err := database.DB.Get(&user, `SELECT u.* FROM user_identity_bindings i JOIN users u ON u.id = i.user_id WHERE i.tenant_id = ? AND i.provider = ? AND i.provider_user_id = ? AND i.status = 'active' AND u.deleted_at IS NULL AND u.status = 'active'`, tenantID, provider, providerUserID)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func buildExternalUserMetadata(externalSystem string, item EmbedUserSyncItem) map[string]any {
	metadata := map[string]any{"external_system": externalSystem, "external_user_id": item.ExternalUserID, "source": "embed_app_sync"}
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
	return metadata
}

func rawJSONPtr(raw json.RawMessage) *string {
	if len(raw) == 0 {
		return nil
	}
	s := string(raw)
	return &s
}

func normalizedExternalUserUID(userUID, externalSystem, externalUserID string) string {
	if v := strings.TrimSpace(userUID); v != "" {
		return v
	}
	candidate := externalSystem + ":" + externalUserID
	if len(candidate) <= 64 {
		return candidate
	}
	sum := sha256.Sum256([]byte(candidate))
	return "ext_" + hex.EncodeToString(sum[:])[:32]
}

func strconvParseInt(s string) (int64, error) {
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

func isOriginAllowed(origin string, allowedOrigins *string) bool {
	if origin == "" || allowedOrigins == nil || strings.TrimSpace(*allowedOrigins) == "" {
		return true
	}
	var origins []string
	if err := json.Unmarshal([]byte(*allowedOrigins), &origins); err != nil || len(origins) == 0 {
		return true
	}
	for _, allowed := range origins {
		allowed = strings.TrimSpace(allowed)
		if allowed == "*" || strings.EqualFold(allowed, origin) {
			return true
		}
	}
	return false
}

func verifyEmbedSignature(secretHash, appID, timestamp, nonce string, body map[string]string, signature string) bool {
	keys := make([]string, 0, len(body))
	for k := range body {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := []string{"app_id=" + appID, "timestamp=" + timestamp, "nonce=" + nonce}
	for _, k := range keys {
		parts = append(parts, k+"="+body[k])
	}
	payload := strings.Join(parts, "&")
	mac := hmac.New(sha256.New, []byte(secretHash))
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(strings.ToLower(signature)), []byte(expected))
}

// EmbedChatRequest Embed 聊天请求
type EmbedChatRequest struct {
	SessionID string          `json:"session_id"` // 可选，不传则自动创建
	VisitorID string          `json:"visitor_id"` // 外部访客ID
	Message   string          `json:"message" binding:"required"`
	Context   json.RawMessage `json:"context"` // 业务上下文（订单号、用户信息等）
}

// EmbedSessionResponse 会话响应
type EmbedSessionResponse struct {
	ID        string `json:"id"`
	VisitorID string `json:"visitor_id"`
}

// EmbedMessageResponse 消息响应
type EmbedMessageResponse struct {
	ID        string  `json:"id"`
	Role      string  `json:"role"`
	Content   string  `json:"content"`
	CreatedAt string  `json:"created_at"`
	Metadata  *string `json:"metadata,omitempty"`
}

// Chat 处理 Embed 聊天请求（SSE 流式）
// POST /embed/v1/chat
func (h *EmbedHandler) Chat(c *gin.Context) {
	var req EmbedChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := c.GetString(middleware.ContextEmbedTenantID)
	userID := c.GetString(middleware.ContextEmbedUserID)
	apiKeyVal, _ := c.Get(middleware.ContextAPIKey)
	apiKey := apiKeyVal.(models.APIKey)

	// 处理会话
	sessionID := req.SessionID
	if sessionID == "" {
		// 创建新会话
		sessionID = uuid.New().String()
		visitorID := req.VisitorID
		if visitorID == "" {
			visitorID = "anonymous"
		}

		var contextJSON *string
		if len(req.Context) > 0 {
			s := string(req.Context)
			contextJSON = &s
		}

		_, err := database.DB.Exec(`
			INSERT INTO embed_sessions (id, tenant_id, api_key_id, visitor_id, metadata, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, NOW(), NOW())`,
			sessionID, tenantID, apiKey.ID, visitorID, contextJSON)
		if err != nil {
			log.Printf("EmbedHandler: failed to create session: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
			return
		}
	} else {
		// 验证会话存在且属于当前租户
		var count int
		database.DB.Get(&count,
			"SELECT COUNT(*) FROM embed_sessions WHERE id = ? AND tenant_id = ?",
			sessionID, tenantID)
		if count == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
			return
		}
	}

	// 保存用户消息
	userMsgID := uuid.New().String()
	database.DB.Exec(`
		INSERT INTO embed_messages (id, session_id, role, content, created_at)
		VALUES (?, ?, 'user', ?, NOW())`,
		userMsgID, sessionID, req.Message)

	// 更新会话消息计数
	database.DB.Exec("UPDATE embed_sessions SET message_count = message_count + 1, updated_at = NOW() WHERE id = ?", sessionID)

	// 获取历史消息构建上下文（最近10条）
	var history []struct {
		Role    string `db:"role"`
		Content string `db:"content"`
	}
	database.DB.Select(&history,
		"SELECT role, content FROM embed_messages WHERE session_id = ? ORDER BY created_at ASC LIMIT 10",
		sessionID)

	// 构建消息列表
	var messages []AssistantMessage
	// 添加业务上下文作为系统消息
	if len(req.Context) > 0 {
		contextMsg := fmt.Sprintf("业务上下文信息: %s", string(req.Context))
		messages = append(messages, AssistantMessage{Role: "system", Content: contextMsg})
	}
	for _, h := range history {
		messages = append(messages, AssistantMessage{Role: h.Role, Content: h.Content})
	}

	// 设置 SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// 获取模型配置
	_, cfgErr := h.chatHandler.modelService.GetConfigForTenant(tenantID, "")
	if cfgErr != nil {
		sendSSE(c, "error", map[string]string{"message": "未配置可用的模型"})
		sendSSE(c, "done", nil)
		return
	}

	// 构建模型消息
	modelMessages := []modelservice.Message{
		{Role: "system", Content: "你是一个智能助手，帮助用户解答问题。请用简洁专业的语气回答。"},
	}
	for _, m := range messages {
		modelMessages = append(modelMessages, modelservice.Message{Role: m.Role, Content: m.Content})
	}

	// 调用模型（非流式，简化实现）
	response, err := h.chatHandler.callModelWithTools(tenantID, modelMessages, nil)
	if err != nil {
		log.Printf("EmbedHandler: model call failed: %v", err)
		sendSSE(c, "error", map[string]string{"message": "模型调用失败"})
		sendSSE(c, "done", nil)
		return
	}

	// 记录 token 消耗
	if response.InputTokens > 0 || response.OutputTokens > 0 {
		RecordModelUsageWithContext(tenantID, userID, response.Provider, response.Model,
			"/embed/chat", response.InputTokens, response.OutputTokens, response.CachedTokens, 0,
			"embed", "嵌入式助手", "embed", sessionID, "")
	}

	// 发送响应
	sendSSE(c, "delta", map[string]string{"content": response.Content})
	sendSSE(c, "done", nil)

	// 保存助手消息
	assistantMsgID := uuid.New().String()
	database.DB.Exec(`
		INSERT INTO embed_messages (id, session_id, role, content, created_at)
		VALUES (?, ?, 'assistant', ?, NOW())`,
		assistantMsgID, sessionID, response.Content)
	database.DB.Exec("UPDATE embed_sessions SET message_count = message_count + 1, updated_at = NOW() WHERE id = ?", sessionID)
}

// ListAssistantConversations 查询当前嵌入用户自己的助手历史会话。
func (h *EmbedHandler) ListAssistantConversations(c *gin.Context) {
	tenantID := c.GetString(middleware.ContextTenantID)
	userID := c.GetString(middleware.ContextUserID)
	var conversations []models.AssistantConversation
	err := database.DB.Select(&conversations, `SELECT * FROM assistant_conversations WHERE tenant_id = ? AND user_id = ? ORDER BY updated_at DESC LIMIT 50`, tenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list conversations"})
		return
	}
	if conversations == nil {
		conversations = []models.AssistantConversation{}
	}
	c.JSON(http.StatusOK, conversations)
}

// GetAssistantConversationMessages 查询当前嵌入用户自己的助手历史消息。
func (h *EmbedHandler) GetAssistantConversationMessages(c *gin.Context) {
	tenantID := c.GetString(middleware.ContextTenantID)
	userID := c.GetString(middleware.ContextUserID)
	conversationID := c.Param("conversationId")
	var count int
	if err := database.DB.Get(&count, `SELECT COUNT(*) FROM assistant_conversations WHERE id = ? AND tenant_id = ? AND user_id = ?`, conversationID, tenantID, userID); err != nil || count == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Conversation not found"})
		return
	}
	var messages []models.SessionMemory
	err := database.DB.Select(&messages, `SELECT id, tenant_id, user_id, session_id, role, content, token_count, entity_ids, created_at FROM session_memories WHERE tenant_id = ? AND user_id = ? AND session_id = ? ORDER BY created_at ASC`, tenantID, userID, conversationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list messages"})
		return
	}
	if messages == nil {
		messages = []models.SessionMemory{}
	}
	c.JSON(http.StatusOK, gin.H{"conversation_id": conversationID, "messages": messages})
}

// CreateSession 创建会话
// POST /embed/v1/sessions
func (h *EmbedHandler) CreateSession(c *gin.Context) {
	tenantID := c.GetString(middleware.ContextEmbedTenantID)
	apiKeyVal, _ := c.Get(middleware.ContextAPIKey)
	apiKey := apiKeyVal.(models.APIKey)

	var req struct {
		VisitorID string          `json:"visitor_id"`
		Metadata  json.RawMessage `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	visitorID := req.VisitorID
	if visitorID == "" {
		visitorID = "anonymous"
	}

	sessionID := uuid.New().String()
	var metadataJSON *string
	if len(req.Metadata) > 0 {
		s := string(req.Metadata)
		metadataJSON = &s
	}

	_, err := database.DB.Exec(`
		INSERT INTO embed_sessions (id, tenant_id, api_key_id, visitor_id, metadata, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())`,
		sessionID, tenantID, apiKey.ID, visitorID, metadataJSON)
	if err != nil {
		log.Printf("EmbedHandler: failed to create session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":         sessionID,
		"visitor_id": visitorID,
		"created_at": time.Now(),
	})
}

// GetSessionMessages 获取会话消息
// GET /embed/v1/sessions/:sessionId/messages
func (h *EmbedHandler) GetSessionMessages(c *gin.Context) {
	tenantID := c.GetString(middleware.ContextEmbedTenantID)
	sessionID := c.Param("sessionId")

	// 验证会话属于当前租户
	var session models.EmbedSession
	err := database.DB.Get(&session,
		"SELECT * FROM embed_sessions WHERE id = ? AND tenant_id = ?", sessionID, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// 获取消息
	var messages []models.EmbedMessage
	database.DB.Select(&messages,
		"SELECT * FROM embed_messages WHERE session_id = ? ORDER BY created_at ASC", sessionID)
	if messages == nil {
		messages = []models.EmbedMessage{}
	}

	c.JSON(http.StatusOK, gin.H{
		"session":  session,
		"messages": messages,
	})
}

// ListSessions 列出会话
// GET /embed/v1/sessions
func (h *EmbedHandler) ListSessions(c *gin.Context) {
	tenantID := c.GetString(middleware.ContextEmbedTenantID)
	visitorID := c.Query("visitor_id")

	var sessions []models.EmbedSession
	var err error
	if visitorID != "" {
		err = database.DB.Select(&sessions,
			"SELECT * FROM embed_sessions WHERE tenant_id = ? AND visitor_id = ? ORDER BY updated_at DESC LIMIT 50",
			tenantID, visitorID)
	} else {
		err = database.DB.Select(&sessions,
			"SELECT * FROM embed_sessions WHERE tenant_id = ? ORDER BY updated_at DESC LIMIT 50",
			tenantID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list sessions"})
		return
	}
	if sessions == nil {
		sessions = []models.EmbedSession{}
	}

	c.JSON(http.StatusOK, sessions)
}
