package embed

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/easp-platform/easp/internal/auth"
	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/models"
	"github.com/easp-platform/easp/internal/repositories"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// TokenExchange 业务系统后端用 app_id/app_secret 签名换取 easp-api-token。
// 第一阶段只允许已导入 external_user_bindings 的外部用户换取 Token，不自动创建用户。
func TokenExchange(c *gin.Context) {
	var req TokenExchangeRequest
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
	if !IsOriginAllowed(c.GetHeader("Origin"), app.AllowedOrigins) {
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
	if !VerifyEmbedSignature(app.AppSecretHash, appID, timestamp, nonce, bodyMap, signature) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "INVALID_SIGNATURE", "message": "Signature verification failed"})
		return
	}
	binding, err := repositories.NewExternalUserBindingRepository().GetActive(req.TenantID, req.ExternalSystem, req.ExternalUserID)
	var user models.User
	if err != nil {
		// 如果开启了 auto_create_user，则自动创建 EASP 用户和绑定
		if !app.AutoCreateUser {
			c.JSON(http.StatusForbidden, gin.H{"error": "EXTERNAL_USER_NOT_IMPORTED", "message": "外部用户未导入 EASP，无法换取嵌入式 Token"})
			return
		}
		// 自动创建用户
		// 检查租户用户上限
		tenant, err := repositories.NewTenantRepository().GetByID(req.TenantID)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "TENANT_UNAVAILABLE", "message": "租户不存在或不可用"})
			return
		}
		// 检查租户是否超过用户上限
		if tenant.MaxUsers > 0 {
			count, err := repositories.NewUserRepository().CountByTenant(req.TenantID)
			if err == nil && count >= tenant.MaxUsers {
				c.JSON(http.StatusForbidden, gin.H{"error": "TENANT_USER_LIMIT_REACHED", "message": "租户已达到用户数量上限"})
				return
			}
		}
		// 生成随机密码，account 使用 external_system + "_" + external_user_id 保证唯一性
		account := strings.ToLower(req.ExternalSystem + "_" + req.ExternalUserID)
		// 检查 account 是否已存在
		if existing, err := repositories.NewUserRepository().GetByTenantAndAccount(req.TenantID, account); err == nil && existing != nil {
			// 用户已存在，说明绑定记录丢失，重新创建绑定
			binding = &models.ExternalUserBinding{
				TenantID:       req.TenantID,
				UserID:         existing.ID,
				ExternalSystem: req.ExternalSystem,
				ExternalUserID: req.ExternalUserID,
				DisplayName:    req.DisplayName,
				Email:          req.Email,
				Phone:          req.Phone,
				Status:         "active",
			}
			if err := repositories.NewExternalUserBindingRepository().Upsert(binding); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "FAILED_CREATE_BINDING", "message": "创建用户绑定失败"})
				return
			}
			user = *existing
		} else {
			// 真正创建新用户
			defaultPassword := "external-" + uuid.NewString()
			passwordHash, _ := bcrypt.GenerateFromPassword([]byte(defaultPassword), bcrypt.DefaultCost)
			displayName := req.DisplayName
			if displayName == "" {
				displayName = req.ExternalUserID
			}
			user = models.User{
				ID:          uuid.NewString(),
				Account:     account,
				TenantID:    req.TenantID,
				Email:       req.Email,
				DisplayName: displayName,
				Phone:       req.Phone,
				PasswordHash: string(passwordHash),
				Status:      "active",
				SSOProvider: "embed",
				SSOUserID:   req.ExternalUserID,
			}
			if err := repositories.NewUserRepository().Create(&user); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "FAILED_CREATE_USER", "message": "创建 EASP 用户失败"})
				return
			}
			// 创建绑定
			binding = &models.ExternalUserBinding{
				TenantID:       req.TenantID,
				UserID:         user.ID,
				ExternalSystem: req.ExternalSystem,
				ExternalUserID: req.ExternalUserID,
				DisplayName:    displayName,
				Email:          req.Email,
				Phone:          req.Phone,
				Status:         "active",
			}
			if err := repositories.NewExternalUserBindingRepository().Upsert(binding); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "FAILED_CREATE_BINDING", "message": "创建用户绑定失败"})
				return
			}
			// 分配默认角色
			if app.DefaultRoleIDs != nil && *app.DefaultRoleIDs != "" {
				var roleIDs []string
				_ = json.Unmarshal([]byte(*app.DefaultRoleIDs), &roleIDs)
				for _, roleID := range roleIDs {
					roleID = strings.TrimSpace(roleID)
					if roleID == "" {
						continue
					}
					role, err := repositories.NewRoleRepository().GetByID(roleID)
					if err != nil || role.IsSystem || role.TenantID != req.TenantID {
						continue
					}
					_ = repositories.NewUserRoleRepository().Assign(user.ID, roleID)
				}
			}
		}
	} else {
		if err := database.DB.Get(&user, "SELECT * FROM users WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL AND status = 'active'", binding.UserID, req.TenantID); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "EASP_USER_INACTIVE", "message": "绑定的 EASP 用户不存在或未启用"})
			return
		}
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
	if binding != nil {
		database.DB.Exec("UPDATE external_user_bindings SET last_login_at = NOW(), updated_at = NOW() WHERE id = ?", binding.ID)
	}
	repositories.NewTenantEmbedAppRepository().Touch(app.AppID)

	// 记录审计日志
	auditRepo := repositories.NewAuditLogRepository()
	detail := fmt.Sprintf("External user %s (%s) exchanged token for EASP user %s", req.ExternalUserID, req.ExternalSystem, user.ID)
	detailPtr := &detail
	auditLog := &models.AuditLog{
		TenantID:       req.TenantID,
		UserID:         &user.ID,
		Tool:           "embed_token_exchange",
		Action:         "exchange",
		SourceType:     toPtr("embed"),
		SourceAppID:    &app.AppID,
		ExternalSystem: &req.ExternalSystem,
		ExternalUserID: &req.ExternalUserID,
		Detail:         detailPtr,
	}
	_ = auditRepo.Create(auditLog)

	c.Header("easp-api-token", token)
	c.Header("Access-Control-Expose-Headers", "easp-api-token")
	c.JSON(http.StatusOK, TokenExchangeResponse{
		AccessToken: token,
		ExpiresIn:   int(exp.Unix() - time.Now().Unix()),
		UserID:      user.ID,
		TenantID:    req.TenantID,
	})
}

// toPtr 辅助函数转换指针
func toPtr[T any](v T) *T {
	return &v
}
