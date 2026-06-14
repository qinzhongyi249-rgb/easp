package repositories

import (
	"time"

	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/models"
	"github.com/easp-platform/easp/internal/skill"
	"github.com/google/uuid"
)

// ConnectorRepository 连接器仓库
type ConnectorRepository struct{}

func NewConnectorRepository() *ConnectorRepository {
	return &ConnectorRepository{}
}

func (r *ConnectorRepository) Create(connector *models.Connector) error {
	connector.ID = uuid.New().String()
	connector.CreatedAt = time.Now()
	connector.UpdatedAt = time.Now()

	query := `INSERT INTO connectors (id, tenant_id, name, type, base_url, transport_type, mcp_server_url, headers, auth_type, auth_config, credential_mode, user_token_header, user_token_prefix, user_token_required_sso, spec_url, spec_content, status, tools_count, last_sync_at, created_at, updated_at)
			  VALUES (:id, :tenant_id, :name, :type, :base_url, :transport_type, :mcp_server_url, :headers, :auth_type, :auth_config, :credential_mode, :user_token_header, :user_token_prefix, :user_token_required_sso, :spec_url, :spec_content, :status, :tools_count, :last_sync_at, :created_at, :updated_at)`
	_, err := database.DB.NamedExec(query, connector)
	return err
}

func (r *ConnectorRepository) GetByID(id string) (*models.Connector, error) {
	var connector models.Connector
	err := database.DB.Get(&connector, "SELECT * FROM connectors WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	return &connector, nil
}

func (r *ConnectorRepository) ListByTenant(tenantID string) ([]models.Connector, error) {
	var connectors []models.Connector
	err := database.DB.Select(&connectors, "SELECT * FROM connectors WHERE tenant_id = ? ORDER BY created_at DESC", tenantID)
	return connectors, err
}

func (r *ConnectorRepository) Update(connector *models.Connector) error {
	connector.UpdatedAt = time.Now()
	query := `UPDATE connectors SET name=:name, type=:type, base_url=:base_url, transport_type=:transport_type, mcp_server_url=:mcp_server_url, headers=:headers, auth_type=:auth_type, auth_config=:auth_config, credential_mode=:credential_mode, user_token_header=:user_token_header, user_token_prefix=:user_token_prefix, user_token_required_sso=:user_token_required_sso,
			  spec_url=:spec_url, spec_content=:spec_content, status=:status, tools_count=:tools_count, last_sync_at=:last_sync_at, updated_at=:updated_at WHERE id=:id`
	_, err := database.DB.NamedExec(query, connector)
	return err
}

func (r *ConnectorRepository) Delete(id string) error {
	_, err := database.DB.Exec("DELETE FROM connectors WHERE id = ?", id)
	return err
}

func (r *ConnectorRepository) UpdateStatus(id, status string) error {
	_, err := database.DB.Exec("UPDATE connectors SET status = ?, updated_at = ? WHERE id = ?", status, time.Now(), id)
	return err
}

// MCPToolRepository MCP工具仓库
type MCPToolRepository struct{}

func NewMCPToolRepository() *MCPToolRepository {
	return &MCPToolRepository{}
}

func (r *MCPToolRepository) Create(tool *models.MCPTool) error {
	tool.ID = uuid.New().String()
	tool.CreatedAt = time.Now()
	tool.UpdatedAt = time.Now()
	tool.Status = skill.NormalizeSkillStatus(tool.Status)
	if tool.RiskLevel == "" {
		tool.RiskLevel = "medium"
	}

	query := `INSERT INTO mcp_tools (id, tenant_id, connector_id, name, description, input_schema, backend_method, backend_path, risk_level, status, enabled, is_builtin, locked, created_at, updated_at)
			  VALUES (:id, :tenant_id, :connector_id, :name, :description, :input_schema, :backend_method, :backend_path, :risk_level, :status, :enabled, :is_builtin, :locked, :created_at, :updated_at)`
	_, err := database.DB.NamedExec(query, tool)
	return err
}

func (r *MCPToolRepository) GetByID(id string) (*models.MCPTool, error) {
	var tool models.MCPTool
	err := database.DB.Get(&tool, "SELECT * FROM mcp_tools WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	return &tool, nil
}

func (r *MCPToolRepository) GetByName(tenantID, name string) (*models.MCPTool, error) {
	var tool models.MCPTool
	err := database.DB.Get(&tool, "SELECT * FROM mcp_tools WHERE tenant_id = ? AND name = ?", tenantID, name)
	if err != nil {
		return nil, err
	}
	return &tool, nil
}

func (r *MCPToolRepository) ListByConnector(connectorID string) ([]models.MCPTool, error) {
	var tools []models.MCPTool
	err := database.DB.Select(&tools, "SELECT * FROM mcp_tools WHERE connector_id = ? ORDER BY name", connectorID)
	return tools, err
}

func (r *MCPToolRepository) ListByTenant(tenantID string) ([]models.MCPTool, error) {
	var tools []models.MCPTool
	err := database.DB.Select(&tools, "SELECT * FROM mcp_tools WHERE tenant_id = ? ORDER BY name", tenantID)
	return tools, err
}

func (r *MCPToolRepository) ListEnabled(tenantID string) ([]models.MCPTool, error) {
	var tools []models.MCPTool
	err := database.DB.Select(&tools, "SELECT * FROM mcp_tools WHERE tenant_id = ? AND enabled = true AND status IN ('published', 'active') ORDER BY name", tenantID)
	return tools, err
}

func (r *MCPToolRepository) Update(tool *models.MCPTool) error {
	tool.UpdatedAt = time.Now()
	tool.Status = skill.NormalizeSkillStatus(tool.Status)
	if tool.RiskLevel == "" {
		tool.RiskLevel = "medium"
	}
	query := `UPDATE mcp_tools SET name=:name, description=:description, input_schema=:input_schema,
			  backend_method=:backend_method, backend_path=:backend_path, risk_level=:risk_level, status=:status, enabled=:enabled, is_builtin=:is_builtin, locked=:locked, updated_at=:updated_at WHERE id=:id`
	_, err := database.DB.NamedExec(query, tool)
	return err
}

func (r *MCPToolRepository) Delete(id string) error {
	_, err := database.DB.Exec("DELETE FROM mcp_tools WHERE id = ?", id)
	return err
}

func (r *MCPToolRepository) ToggleEnabled(id string, enabled bool) error {
	_, err := database.DB.Exec("UPDATE mcp_tools SET enabled = ?, updated_at = ? WHERE id = ?", enabled, time.Now(), id)
	return err
}
