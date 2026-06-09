package repositories

import (
	"time"

	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/models"
	"github.com/google/uuid"
)

// SkillRepository Skill模板仓库
type SkillRepository struct{}

func NewSkillRepository() *SkillRepository {
	return &SkillRepository{}
}

func (r *SkillRepository) Create(skill *models.Skill) error {
	skill.ID = uuid.New().String()
	skill.CreatedAt = time.Now()
	skill.UpdatedAt = time.Now()
	
	query := `INSERT INTO skills (id, tenant_id, name, description, version, triggers, steps, permission_topology, status, created_by, created_at, updated_at) 
			  VALUES (:id, :tenant_id, :name, :description, :version, :triggers, :steps, :permission_topology, :status, :created_by, :created_at, :updated_at)`
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

func (r *SkillRepository) Update(skill *models.Skill) error {
	skill.UpdatedAt = time.Now()
	query := `UPDATE skills SET name=:name, description=:description, version=:version, triggers=:triggers, 
			  steps=:steps, permission_topology=:permission_topology, status=:status, updated_at=:updated_at WHERE id=:id`
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
	
	query := `INSERT INTO audit_logs (id, tenant_id, user_id, agent_id, tool, action, resource, detail, decision, result, duration_ms, ip, user_agent, created_at) 
			  VALUES (:id, :tenant_id, :user_id, :agent_id, :tool, :action, :resource, :detail, :decision, :result, :duration_ms, :ip, :user_agent, :created_at)`
	_, err := database.DB.NamedExec(query, log)
	return err
}

func (r *AuditLogRepository) ListByTenant(tenantID string, limit, offset int) ([]models.AuditLog, error) {
	var logs []models.AuditLog
	err := database.DB.Select(&logs, "SELECT * FROM audit_logs WHERE tenant_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?", tenantID, limit, offset)
	return logs, err
}

func (r *AuditLogRepository) ListByUser(userID string, limit, offset int) ([]models.AuditLog, error) {
	var logs []models.AuditLog
	err := database.DB.Select(&logs, "SELECT * FROM audit_logs WHERE user_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?", userID, limit, offset)
	return logs, err
}

func (r *AuditLogRepository) ListByTool(tenantID, tool string, limit int) ([]models.AuditLog, error) {
	var logs []models.AuditLog
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
