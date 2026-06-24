package repositories

import (
	"strings"
	"time"

	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/models"
	"github.com/google/uuid"
)

// TenantEmbedAppRepository 嵌入式接入应用仓库
type TenantEmbedAppRepository struct{}

func NewTenantEmbedAppRepository() *TenantEmbedAppRepository { return &TenantEmbedAppRepository{} }

func (r *TenantEmbedAppRepository) Create(app *models.TenantEmbedApp) error {
	if app.ID == "" {
		app.ID = uuid.New().String()
	}
	if app.TokenTTLSeconds <= 0 {
		app.TokenTTLSeconds = 7200
	}
	if app.Status == "" {
		app.Status = "active"
	}
	app.CreatedAt = time.Now()
	app.UpdatedAt = time.Now()
	_, err := database.DB.NamedExec(`INSERT INTO tenant_embed_apps
		(id, tenant_id, app_id, app_secret_hash, name, external_system, allowed_origins, allowed_scopes, token_ttl_seconds, auto_create_user, default_role_ids, status, created_at, updated_at)
		VALUES (:id, :tenant_id, :app_id, :app_secret_hash, :name, :external_system, :allowed_origins, :allowed_scopes, :token_ttl_seconds, :auto_create_user, :default_role_ids, :status, :created_at, :updated_at)`, app)
	return err
}

func (r *TenantEmbedAppRepository) GetByAppID(appID string) (*models.TenantEmbedApp, error) {
	var app models.TenantEmbedApp
	err := database.DB.Get(&app, "SELECT * FROM tenant_embed_apps WHERE app_id = ?", appID)
	if err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *TenantEmbedAppRepository) GetByID(tenantID, id string) (*models.TenantEmbedApp, error) {
	var app models.TenantEmbedApp
	err := database.DB.Get(&app, "SELECT * FROM tenant_embed_apps WHERE tenant_id = ? AND id = ?", tenantID, id)
	if err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *TenantEmbedAppRepository) ListByTenant(tenantID string) ([]models.TenantEmbedApp, error) {
	var apps []models.TenantEmbedApp
	err := database.DB.Select(&apps, `SELECT * FROM tenant_embed_apps WHERE tenant_id = ? ORDER BY created_at DESC`, tenantID)
	return apps, err
}

func (r *TenantEmbedAppRepository) Touch(appID string) {
	_, _ = database.DB.Exec("UPDATE tenant_embed_apps SET last_used_at = NOW() WHERE app_id = ?", appID)
}

// ExternalUserBindingRepository 外部用户绑定仓库
type ExternalUserBindingRepository struct{}

func NewExternalUserBindingRepository() *ExternalUserBindingRepository {
	return &ExternalUserBindingRepository{}
}

func (r *ExternalUserBindingRepository) Upsert(binding *models.ExternalUserBinding) error {
	if binding.ID == "" {
		binding.ID = uuid.New().String()
	}
	if binding.Status == "" {
		binding.Status = "active"
	}
	binding.CreatedAt = time.Now()
	binding.UpdatedAt = time.Now()
	_, err := database.DB.NamedExec(`INSERT INTO external_user_bindings
		(id, tenant_id, user_id, external_system, external_user_id, display_name, email, phone, metadata, status, created_at, updated_at)
		VALUES (:id, :tenant_id, :user_id, :external_system, :external_user_id, :display_name, :email, :phone, :metadata, :status, :created_at, :updated_at)
		ON DUPLICATE KEY UPDATE user_id=VALUES(user_id), display_name=VALUES(display_name), email=VALUES(email), phone=VALUES(phone), metadata=VALUES(metadata), status=VALUES(status), updated_at=NOW()`, binding)
	return err
}

func (r *ExternalUserBindingRepository) GetActive(tenantID, externalSystem, externalUserID string) (*models.ExternalUserBinding, error) {
	var binding models.ExternalUserBinding
	err := database.DB.Get(&binding, `SELECT * FROM external_user_bindings WHERE tenant_id = ? AND external_system = ? AND external_user_id = ? AND status = 'active'`, tenantID, externalSystem, externalUserID)
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

func (r *ExternalUserBindingRepository) List(tenantID, externalSystem string) ([]models.ExternalUserBinding, error) {
	return r.Search(tenantID, externalSystem, "", "", 500)
}

func (r *ExternalUserBindingRepository) Search(tenantID, externalSystem, keyword, status string, limit int) ([]models.ExternalUserBinding, error) {
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	args := []any{tenantID}
	where := "tenant_id = ?"
	if externalSystem != "" {
		where += " AND external_system = ?"
		args = append(args, externalSystem)
	}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	if keyword != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		where += " AND (external_user_id LIKE ? OR display_name LIKE ? OR email LIKE ? OR phone LIKE ?)"
		args = append(args, like, like, like, like)
	}
	args = append(args, limit)
	var bindings []models.ExternalUserBinding
	err := database.DB.Select(&bindings, "SELECT * FROM external_user_bindings WHERE "+where+" ORDER BY created_at DESC LIMIT ?", args...)
	return bindings, err
}

// UserIdentityBindingRepository 第三方登录/身份绑定仓库
type UserIdentityBindingRepository struct{}

func NewUserIdentityBindingRepository() *UserIdentityBindingRepository {
	return &UserIdentityBindingRepository{}
}

func (r *UserIdentityBindingRepository) Upsert(identity *models.UserIdentityBinding) error {
	if identity.ID == "" {
		identity.ID = uuid.New().String()
	}
	if identity.Status == "" {
		identity.Status = "active"
	}
	if identity.LinkedAt.IsZero() {
		identity.LinkedAt = time.Now()
	}
	identity.CreatedAt = time.Now()
	identity.UpdatedAt = time.Now()
	_, err := database.DB.NamedExec(`INSERT INTO user_identity_bindings
		(id, tenant_id, user_id, provider, provider_user_id, union_id, open_id, external_system, display_name, avatar, email, phone, metadata, status, linked_at, last_login_at, created_at, updated_at)
		VALUES (:id, :tenant_id, :user_id, :provider, :provider_user_id, :union_id, :open_id, :external_system, :display_name, :avatar, :email, :phone, :metadata, :status, :linked_at, :last_login_at, :created_at, :updated_at)
		ON DUPLICATE KEY UPDATE user_id=VALUES(user_id), union_id=VALUES(union_id), open_id=VALUES(open_id), external_system=VALUES(external_system), display_name=VALUES(display_name), avatar=VALUES(avatar), email=VALUES(email), phone=VALUES(phone), metadata=VALUES(metadata), status=VALUES(status), updated_at=NOW()`, identity)
	return err
}

func (r *UserIdentityBindingRepository) ListByUser(tenantID, userID string) ([]models.UserIdentityBinding, error) {
	var identities []models.UserIdentityBinding
	err := database.DB.Select(&identities, `SELECT * FROM user_identity_bindings WHERE tenant_id = ? AND user_id = ? ORDER BY created_at DESC`, tenantID, userID)
	return identities, err
}

func (r *UserIdentityBindingRepository) Search(tenantID, provider, keyword string, limit int) ([]models.UserIdentityBinding, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	args := []any{tenantID}
	where := "tenant_id = ?"
	if provider != "" {
		where += " AND provider = ?"
		args = append(args, provider)
	}
	if keyword != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		where += " AND (provider_user_id LIKE ? OR union_id LIKE ? OR open_id LIKE ? OR display_name LIKE ? OR email LIKE ? OR phone LIKE ?)"
		args = append(args, like, like, like, like, like, like)
	}
	args = append(args, limit)
	var identities []models.UserIdentityBinding
	err := database.DB.Select(&identities, "SELECT * FROM user_identity_bindings WHERE "+where+" ORDER BY created_at DESC LIMIT ?", args...)
	return identities, err
}
