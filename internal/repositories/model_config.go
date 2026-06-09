package repositories

import (
	"time"

	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/models"
	"github.com/google/uuid"
)

// ModelProviderRepository 模型提供商仓库
type ModelProviderRepository struct{}

func NewModelProviderRepository() *ModelProviderRepository {
	return &ModelProviderRepository{}
}

func (r *ModelProviderRepository) Create(provider *models.ModelProvider) error {
	provider.ID = uuid.New().String()
	provider.CreatedAt = time.Now()
	provider.UpdatedAt = time.Now()
	
	query := `INSERT INTO model_providers (id, tenant_id, name, display_name, base_url, api_key, enabled, created_at, updated_at) 
			  VALUES (:id, :tenant_id, :name, :display_name, :base_url, :api_key, :enabled, :created_at, :updated_at)`
	_, err := database.DB.NamedExec(query, provider)
	return err
}

func (r *ModelProviderRepository) GetByID(id string) (*models.ModelProvider, error) {
	var provider models.ModelProvider
	err := database.DB.Get(&provider, "SELECT * FROM model_providers WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	return &provider, nil
}

func (r *ModelProviderRepository) GetByTenantAndName(tenantID, name string) (*models.ModelProvider, error) {
	var provider models.ModelProvider
	err := database.DB.Get(&provider, "SELECT * FROM model_providers WHERE tenant_id = ? AND name = ?", tenantID, name)
	if err != nil {
		return nil, err
	}
	return &provider, nil
}

func (r *ModelProviderRepository) ListByTenant(tenantID string) ([]models.ModelProvider, error) {
	var providers []models.ModelProvider
	err := database.DB.Select(&providers, "SELECT * FROM model_providers WHERE tenant_id = ? ORDER BY name", tenantID)
	return providers, err
}

func (r *ModelProviderRepository) ListEnabled(tenantID string) ([]models.ModelProvider, error) {
	var providers []models.ModelProvider
	err := database.DB.Select(&providers, "SELECT * FROM model_providers WHERE tenant_id = ? AND enabled = TRUE ORDER BY name", tenantID)
	return providers, err
}

func (r *ModelProviderRepository) Update(provider *models.ModelProvider) error {
	provider.UpdatedAt = time.Now()
	query := `UPDATE model_providers SET name=:name, display_name=:display_name, base_url=:base_url, 
			  api_key=:api_key, enabled=:enabled, updated_at=:updated_at WHERE id=:id`
	_, err := database.DB.NamedExec(query, provider)
	return err
}

func (r *ModelProviderRepository) Delete(id string) error {
	_, err := database.DB.Exec("DELETE FROM model_providers WHERE id = ?", id)
	return err
}

// ModelConfigRepository 模型配置仓库
type ModelConfigRepository struct{}

func NewModelConfigRepository() *ModelConfigRepository {
	return &ModelConfigRepository{}
}

func (r *ModelConfigRepository) Create(config *models.ModelConfig) error {
	config.ID = uuid.New().String()
	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()
	
	query := `INSERT INTO model_configs (id, tenant_id, provider_id, model_name, display_name, temperature, max_tokens, is_default, enabled, created_at, updated_at) 
			  VALUES (:id, :tenant_id, :provider_id, :model_name, :display_name, :temperature, :max_tokens, :is_default, :enabled, :created_at, :updated_at)`
	_, err := database.DB.NamedExec(query, config)
	if err != nil {
		return err
	}
	// 如果新配置是默认的，先清除旧默认
	if config.IsDefault {
		if err := r.SetDefault(config.ID, config.TenantID); err != nil {
			return err
		}
	}
	return nil
}

func (r *ModelConfigRepository) GetByID(id string) (*models.ModelConfig, error) {
	var config models.ModelConfig
	err := database.DB.Get(&config, "SELECT * FROM model_configs WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *ModelConfigRepository) GetDefault(tenantID string) (*models.ModelConfig, error) {
	var config models.ModelConfig
	err := database.DB.Get(&config, "SELECT * FROM model_configs WHERE tenant_id = ? AND is_default = TRUE AND enabled = TRUE LIMIT 1", tenantID)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *ModelConfigRepository) GetByTenantAndModel(tenantID, modelName string) (*models.ModelConfig, error) {
	var config models.ModelConfig
	err := database.DB.Get(&config, "SELECT * FROM model_configs WHERE tenant_id = ? AND model_name = ? AND enabled = TRUE LIMIT 1", tenantID, modelName)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *ModelConfigRepository) ListByTenant(tenantID string) ([]models.ModelConfig, error) {
	var configs []models.ModelConfig
	err := database.DB.Select(&configs, "SELECT * FROM model_configs WHERE tenant_id = ? ORDER BY is_default DESC, model_name", tenantID)
	return configs, err
}

func (r *ModelConfigRepository) ListByProvider(providerID string) ([]models.ModelConfig, error) {
	var configs []models.ModelConfig
	err := database.DB.Select(&configs, "SELECT * FROM model_configs WHERE provider_id = ? ORDER BY model_name", providerID)
	return configs, err
}

func (r *ModelConfigRepository) ListEnabled(tenantID string) ([]models.ModelConfig, error) {
	var configs []models.ModelConfig
	err := database.DB.Select(&configs, "SELECT * FROM model_configs WHERE tenant_id = ? AND enabled = TRUE ORDER BY is_default DESC, model_name", tenantID)
	return configs, err
}

func (r *ModelConfigRepository) Update(config *models.ModelConfig) error {
	config.UpdatedAt = time.Now()
	query := `UPDATE model_configs SET provider_id=:provider_id, model_name=:model_name, display_name=:display_name, 
			  temperature=:temperature, max_tokens=:max_tokens, is_default=:is_default, enabled=:enabled, updated_at=:updated_at WHERE id=:id`
	_, err := database.DB.NamedExec(query, config)
	return err
}

func (r *ModelConfigRepository) SetDefault(id, tenantID string) error {
	// 先取消所有默认
	_, err := database.DB.Exec("UPDATE model_configs SET is_default = FALSE WHERE tenant_id = ?", tenantID)
	if err != nil {
		return err
	}
	// 设置新的默认
	_, err = database.DB.Exec("UPDATE model_configs SET is_default = TRUE, updated_at = ? WHERE id = ?", time.Now(), id)
	return err
}

func (r *ModelConfigRepository) Delete(id string) error {
	_, err := database.DB.Exec("DELETE FROM model_configs WHERE id = ?", id)
	return err
}
