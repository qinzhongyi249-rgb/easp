package repositories

import (
	"strings"
	"time"

	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/models"
	skillpkg "github.com/easp-platform/easp/internal/skill"
	"github.com/google/uuid"
)

// SkillRepository Skill模板仓库
type SkillRepository struct{}

func NewSkillRepository() *SkillRepository {
	return &SkillRepository{}
}

func (r *SkillRepository) Create(skill *models.Skill) error {
	skill.ID = uuid.New().String()
	if skill.Status == "" {
		skill.Status = skillpkg.SkillStatusDraft
	} else {
		skill.Status = skillpkg.NormalizeSkillStatus(skill.Status)
	}
	if skill.Version == "" {
		skill.Version = "1.0.0"
	}
	skill.CreatedAt = time.Now()
	skill.UpdatedAt = time.Now()

	query := `INSERT INTO skills (id, tenant_id, name, description, category, version, tags, triggers, input_schema, output_schema, steps, permission_topology, status, usage_count, last_used_at, created_by, created_at, updated_at)
			  VALUES (:id, :tenant_id, :name, :description, :category, :version, :tags, :triggers, :input_schema, :output_schema, :steps, :permission_topology, :status, :usage_count, :last_used_at, :created_by, :created_at, :updated_at)`
	_, err := database.DB.NamedExec(query, skill)
	return err
}

func (r *SkillRepository) GetByID(id string) (*models.Skill, error) {
	var skill models.Skill
	err := database.DB.Get(&skill, "SELECT * FROM skills WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	return &skill, nil
}

func (r *SkillRepository) GetByName(tenantID, name string) (*models.Skill, error) {
	var skill models.Skill
	err := database.DB.Get(&skill, "SELECT * FROM skills WHERE tenant_id = ? AND name = ?", tenantID, name)
	if err != nil {
		return nil, err
	}
	return &skill, nil
}

func (r *SkillRepository) ListByTenant(tenantID string) ([]models.Skill, error) {
	var skills []models.Skill
	err := database.DB.Select(&skills, "SELECT * FROM skills WHERE tenant_id = ? ORDER BY name", tenantID)
	return skills, err
}

func (r *SkillRepository) ListByStatus(tenantID, status string) ([]models.Skill, error) {
	var skills []models.Skill
	err := database.DB.Select(&skills, "SELECT * FROM skills WHERE tenant_id = ? AND status = ? ORDER BY name", tenantID, status)
	return skills, err
}

func (r *SkillRepository) ListUsable(tenantID string) ([]models.Skill, error) {
	var skills []models.Skill
	err := database.DB.Select(&skills, "SELECT * FROM skills WHERE tenant_id = ? AND status IN ('published', 'active') ORDER BY name", tenantID)
	return skills, err
}

func (r *SkillRepository) Update(skill *models.Skill) error {
	skill.Status = skillpkg.NormalizeSkillStatus(skill.Status)
	skill.UpdatedAt = time.Now()
	query := `UPDATE skills SET name=:name, description=:description, category=:category, version=:version, tags=:tags, triggers=:triggers,
			  input_schema=:input_schema, output_schema=:output_schema, steps=:steps, permission_topology=:permission_topology, status=:status, updated_at=:updated_at WHERE id=:id`
	_, err := database.DB.NamedExec(query, skill)
	return err
}

func (r *SkillRepository) Delete(id string) error {
	_, err := database.DB.Exec("DELETE FROM skills WHERE id = ?", id)
	return err
}

// AuditLogRepository 审计日志仓库
type AuditLogRepository struct{}

func NewAuditLogRepository() *AuditLogRepository {
	return &AuditLogRepository{}
}

func (r *AuditLogRepository) Create(log *models.AuditLog) error {
	log.ID = uuid.New().String()
	log.CreatedAt = time.Now()

	query := `INSERT INTO audit_logs (id, tenant_id, user_id, user_uid, agent_id, source_type, source_app_id, external_system, external_user_id, tool, action, resource, detail, decision, result, duration_ms, ip, user_agent, created_at)
			  VALUES (:id, :tenant_id, :user_id, :user_uid, :agent_id, :source_type, :source_app_id, :external_system, :external_user_id, :tool, :action, :resource, :detail, :decision, :result, :duration_ms, :ip, :user_agent, :created_at)`
	_, err := database.DB.NamedExec(query, log)
	return err
}

type AuditLogFilter struct {
	SourceType     string
	SourceAppID    string
	ExternalSystem string
	ExternalUserID string
	UserUID        string
	UserID         string
	Tool           string
	Action         string
}

func (r *AuditLogRepository) ListByTenant(tenantID string, limit, offset int) ([]models.AuditLog, error) {
	return r.SearchByTenant(tenantID, AuditLogFilter{}, limit, offset)
}

func (r *AuditLogRepository) SearchByTenant(tenantID string, filter AuditLogFilter, limit, offset int) ([]models.AuditLog, error) {
	logs := make([]models.AuditLog, 0)
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	where := []string{"tenant_id = ?"}
	args := []any{tenantID}
	addEq := func(column, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		where = append(where, column+" = ?")
		args = append(args, value)
	}
	addEq("source_type", filter.SourceType)
	addEq("source_app_id", filter.SourceAppID)
	addEq("external_system", filter.ExternalSystem)
	addEq("external_user_id", filter.ExternalUserID)
	addEq("user_uid", filter.UserUID)
	addEq("user_id", filter.UserID)
	addEq("tool", filter.Tool)
	addEq("action", filter.Action)
	args = append(args, limit, offset)
	query := "SELECT * FROM audit_logs WHERE " + strings.Join(where, " AND ") + " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	err := database.DB.Select(&logs, query, args...)
	return logs, err
}

func (r *AuditLogRepository) ListByUser(userID string, limit, offset int) ([]models.AuditLog, error) {
	logs := make([]models.AuditLog, 0)
	err := database.DB.Select(&logs, "SELECT * FROM audit_logs WHERE user_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?", userID, limit, offset)
	return logs, err
}

func (r *AuditLogRepository) ListByTool(tenantID, tool string, limit int) ([]models.AuditLog, error) {
	logs := make([]models.AuditLog, 0)
	err := database.DB.Select(&logs, "SELECT * FROM audit_logs WHERE tenant_id = ? AND tool = ? ORDER BY created_at DESC LIMIT ?", tenantID, tool, limit)
	return logs, err
}

// SSOProviderRepository SSO提供商仓库
type SSOProviderRepository struct{}

func NewSSOProviderRepository() *SSOProviderRepository {
	return &SSOProviderRepository{}
}

func (r *SSOProviderRepository) Create(provider *models.SSOProvider) error {
	provider.ID = uuid.New().String()
	provider.CreatedAt = time.Now()
	provider.UpdatedAt = time.Now()

	query := `INSERT INTO sso_providers (id, tenant_id, name, type, display_name, icon, enabled, config, created_at, updated_at)
			  VALUES (:id, :tenant_id, :name, :type, :display_name, :icon, :enabled, :config, :created_at, :updated_at)`
	_, err := database.DB.NamedExec(query, provider)
	return err
}

func (r *SSOProviderRepository) GetByID(id string) (*models.SSOProvider, error) {
	var provider models.SSOProvider
	err := database.DB.Get(&provider, "SELECT * FROM sso_providers WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	return &provider, nil
}

func (r *SSOProviderRepository) ListByTenant(tenantID string) ([]models.SSOProvider, error) {
	var providers []models.SSOProvider
	err := database.DB.Select(&providers, "SELECT * FROM sso_providers WHERE tenant_id = ? ORDER BY created_at", tenantID)
	return providers, err
}

func (r *SSOProviderRepository) Update(provider *models.SSOProvider) error {
	provider.UpdatedAt = time.Now()
	query := `UPDATE sso_providers SET name=:name, type=:type, display_name=:display_name, icon=:icon, enabled=:enabled, config=:config, updated_at=:updated_at WHERE id=:id`
	_, err := database.DB.NamedExec(query, provider)
	return err
}

func (r *SSOProviderRepository) Delete(id string) error {
	_, err := database.DB.Exec("DELETE FROM sso_providers WHERE id = ?", id)
	return err
}
