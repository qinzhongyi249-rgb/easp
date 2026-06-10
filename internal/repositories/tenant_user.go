package repositories

import (
	"time"

	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/models"
	"github.com/google/uuid"
)

// TenantRepository 租户仓库
type TenantRepository struct{}

func NewTenantRepository() *TenantRepository {
	return &TenantRepository{}
}

func (r *TenantRepository) Create(tenant *models.Tenant) error {
	tenant.ID = uuid.New().String()
	tenant.CreatedAt = time.Now()
	tenant.UpdatedAt = time.Now()

	query := `INSERT INTO tenants (id, name, plan, status, expires_at, max_users, created_at, updated_at) 
			  VALUES (:id, :name, :plan, :status, :expires_at, :max_users, :created_at, :updated_at)`
	_, err := database.DB.NamedExec(query, tenant)
	return err
}

func (r *TenantRepository) GetByID(id string) (*models.Tenant, error) {
	var tenant models.Tenant
	err := database.DB.Get(&tenant, "SELECT * FROM tenants WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	return &tenant, nil
}

func (r *TenantRepository) List() ([]models.Tenant, error) {
	var tenants []models.Tenant
	err := database.DB.Select(&tenants, "SELECT * FROM tenants ORDER BY created_at DESC")
	return tenants, err
}

func (r *TenantRepository) Update(tenant *models.Tenant) error {
	tenant.UpdatedAt = time.Now()
	query := `UPDATE tenants SET name=:name, plan=:plan, status=:status, expires_at=:expires_at, max_users=:max_users, rate_limit=:rate_limit, daily_quota=:daily_quota, monthly_quota=:monthly_quota, daily_token_quota=:daily_token_quota, monthly_token_quota=:monthly_token_quota, updated_at=:updated_at WHERE id=:id`
	_, err := database.DB.NamedExec(query, tenant)
	return err
}

func (r *TenantRepository) Delete(id string) error {
	_, err := database.DB.Exec("DELETE FROM tenants WHERE id = ?", id)
	return err
}

// UserRepository 用户仓库
type UserRepository struct{}

func NewUserRepository() *UserRepository {
	return &UserRepository{}
}

func (r *UserRepository) Create(user *models.User) error {
	user.ID = uuid.New().String()
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	if user.Metadata == nil {
		defaultMeta := "{}"
		user.Metadata = &defaultMeta
	}

	query := `INSERT INTO users (id, tenant_id, email, display_name, avatar, phone, status, password_hash, sso_provider, sso_user_id, sso_linked_at, metadata, last_login_at, login_count, deleted_at, created_at, updated_at) 
			  VALUES (:id, :tenant_id, :email, :display_name, :avatar, :phone, :status, :password_hash, :sso_provider, :sso_user_id, :sso_linked_at, :metadata, :last_login_at, :login_count, :deleted_at, :created_at, :updated_at)`
	_, err := database.DB.NamedExec(query, user)
	return err
}

func (r *UserRepository) GetByID(id string) (*models.User, error) {
	var user models.User
	err := database.DB.Get(&user, "SELECT * FROM users WHERE id = ? AND deleted_at IS NULL", id)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByIDIncludeDeleted(id string) (*models.User, error) {
	var user models.User
	err := database.DB.Get(&user, "SELECT * FROM users WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByEmail(email string) (*models.User, error) {
	var user models.User
	err := database.DB.Get(&user, "SELECT * FROM users WHERE email = ? AND deleted_at IS NULL", email)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) ListByTenant(tenantID string) ([]models.User, error) {
	var users []models.User
	err := database.DB.Select(&users, "SELECT * FROM users WHERE tenant_id = ? AND deleted_at IS NULL ORDER BY created_at DESC", tenantID)
	return users, err
}

// ListByTenantIncludeDeleted 列出租户下所有用户（含已删除）
func (r *UserRepository) ListByTenantIncludeDeleted(tenantID string) ([]models.User, error) {
	var users []models.User
	err := database.DB.Select(&users, "SELECT * FROM users WHERE tenant_id = ? ORDER BY created_at DESC", tenantID)
	return users, err
}

func (r *UserRepository) Update(user *models.User) error {
	user.UpdatedAt = time.Now()
	query := `UPDATE users SET email=:email, display_name=:display_name, avatar=:avatar, phone=:phone, 
			  status=:status, password_hash=:password_hash, metadata=:metadata, last_login_at=:last_login_at, login_count=:login_count, updated_at=:updated_at 
			  WHERE id=:id`
	_, err := database.DB.NamedExec(query, user)
	return err
}

// Delete 软删除用户（设置 deleted_at）
func (r *UserRepository) Delete(id string) error {
	_, err := database.DB.Exec("UPDATE users SET deleted_at = NOW(), updated_at = NOW() WHERE id = ?", id)
	return err
}

// Restore 恢复已删除用户
func (r *UserRepository) Restore(id string) error {
	_, err := database.DB.Exec("UPDATE users SET deleted_at = NULL, updated_at = NOW() WHERE id = ?", id)
	return err
}

// CountByTenant 统计租户下有效用户数
func (r *UserRepository) CountByTenant(tenantID string) (int, error) {
	var count int
	err := database.DB.Get(&count, "SELECT COUNT(*) FROM users WHERE tenant_id = ? AND deleted_at IS NULL", tenantID)
	return count, err
}

// RoleRepository 角色仓库
type RoleRepository struct{}

func NewRoleRepository() *RoleRepository {
	return &RoleRepository{}
}

func (r *RoleRepository) Create(role *models.Role) error {
	if role.ID == "" {
		role.ID = uuid.New().String()
	}
	role.CreatedAt = time.Now()
	role.UpdatedAt = time.Now()

	query := `INSERT INTO roles (id, tenant_id, name, description, tools, allowed_mcp_tools, allowed_skills, rate_limit, data_scope, is_system, is_default, created_at, updated_at) 
			  VALUES (:id, :tenant_id, :name, :description, :tools, :allowed_mcp_tools, :allowed_skills, :rate_limit, :data_scope, :is_system, :is_default, :created_at, :updated_at)`
	_, err := database.DB.NamedExec(query, role)
	return err
}

func (r *RoleRepository) GetByID(id string) (*models.Role, error) {
	var role models.Role
	err := database.DB.Get(&role, "SELECT * FROM roles WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *RoleRepository) GetByName(tenantID, name string) (*models.Role, error) {
	var role models.Role
	err := database.DB.Get(&role, "SELECT * FROM roles WHERE tenant_id = ? AND name = ?", tenantID, name)
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *RoleRepository) ListByTenant(tenantID string) ([]models.Role, error) {
	var roles []models.Role
	err := database.DB.Select(&roles, "SELECT * FROM roles WHERE tenant_id = ? ORDER BY created_at", tenantID)
	return roles, err
}

func (r *RoleRepository) ListSystem() ([]models.Role, error) {
	var roles []models.Role
	err := database.DB.Select(&roles, "SELECT * FROM roles WHERE is_system = true ORDER BY created_at")
	return roles, err
}

func (r *RoleRepository) ListAll() ([]models.Role, error) {
	var roles []models.Role
	err := database.DB.Select(&roles, "SELECT * FROM roles ORDER BY created_at")
	return roles, err
}

func (r *RoleRepository) Update(role *models.Role) error {
	role.UpdatedAt = time.Now()
	query := `UPDATE roles SET name=:name, description=:description, tools=:tools, allowed_mcp_tools=:allowed_mcp_tools, allowed_skills=:allowed_skills, rate_limit=:rate_limit, data_scope=:data_scope, is_system=:is_system, is_default=:is_default, updated_at=:updated_at WHERE id=:id`
	_, err := database.DB.NamedExec(query, role)
	return err
}

func (r *RoleRepository) Delete(id string) error {
	_, err := database.DB.Exec("DELETE FROM roles WHERE id = ?", id)
	return err
}

// UserRoleRepository 用户角色关联仓库
type UserRoleRepository struct{}

func NewUserRoleRepository() *UserRoleRepository {
	return &UserRoleRepository{}
}

func (r *UserRoleRepository) Assign(userID, roleID string) error {
	query := `INSERT IGNORE INTO user_roles (user_id, role_id) VALUES (?, ?)`
	_, err := database.DB.Exec(query, userID, roleID)
	return err
}

func (r *UserRoleRepository) Revoke(userID, roleID string) error {
	_, err := database.DB.Exec("DELETE FROM user_roles WHERE user_id = ? AND role_id = ?", userID, roleID)
	return err
}

// RevokeAll 撤销用户所有角色
func (r *UserRoleRepository) RevokeAll(userID string) error {
	_, err := database.DB.Exec("DELETE FROM user_roles WHERE user_id = ?", userID)
	return err
}

func (r *UserRoleRepository) GetUserRoles(userID string) ([]models.Role, error) {
	var roles []models.Role
	query := `SELECT r.* FROM roles r JOIN user_roles ur ON r.id = ur.role_id WHERE ur.user_id = ?`
	err := database.DB.Select(&roles, query, userID)
	return roles, err
}

func (r *UserRoleRepository) GetRoleUsers(roleID string) ([]models.User, error) {
	var users []models.User
	query := `SELECT u.* FROM users u JOIN user_roles ur ON u.id = ur.user_id WHERE ur.role_id = ? AND u.deleted_at IS NULL`
	err := database.DB.Select(&users, query, roleID)
	return users, err
}
