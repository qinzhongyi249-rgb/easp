package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/models"
	"github.com/easp-platform/easp/internal/repositories"
	"github.com/easp-platform/easp/internal/skill"
	"github.com/google/uuid"
)

const builtinGovernanceConnectorName = "EASP内置治理工具"

// TenantBuiltinMCPToolDefinition 描述每个租户默认拥有的内置治理 MCP 工具。
type TenantBuiltinMCPToolDefinition struct {
	Name          string
	DisplayName   string
	Description   string
	InputSchema   string
	BackendMethod string
	BackendPath   string
	RiskLevel     string
}

// TenantBuiltinMCPToolDefinitions 返回租户默认内置 MCP 工具定义。
func TenantBuiltinMCPToolDefinitions() []TenantBuiltinMCPToolDefinition {
	return []TenantBuiltinMCPToolDefinition{
		{
			Name:          "builtin_query_users",
			DisplayName:   "查询用户",
			Description:   "查询当前租户用户列表及用户状态、角色信息。",
			InputSchema:   `{"type":"object","properties":{"email":{"type":"string","description":"可选，按邮箱精确查询用户"}},"required":[]}`,
			BackendMethod: "INTERNAL",
			BackendPath:   "builtin://users/query",
			RiskLevel:     "low",
		},
		{
			Name:          "builtin_query_role_permissions",
			DisplayName:   "角色权限查询",
			Description:   "查询当前租户角色、功能权限、MCP工具权限和技能权限配置。",
			InputSchema:   `{"type":"object","properties":{"role_name":{"type":"string","description":"可选，按角色名称精确查询"}},"required":[]}`,
			BackendMethod: "INTERNAL",
			BackendPath:   "builtin://roles/query-permissions",
			RiskLevel:     "low",
		},
		{
			Name:          "builtin_update_user",
			DisplayName:   "更新用户",
			Description:   "更新当前租户用户基础信息、状态，或为用户分配/撤销角色。",
			InputSchema:   `{"type":"object","properties":{"email":{"type":"string","description":"用户邮箱"},"display_name":{"type":"string","description":"显示名称"},"status":{"type":"string","description":"用户状态 active/inactive"},"assign_roles":{"type":"array","items":{"type":"string"},"description":"要分配的角色名称列表"},"revoke_roles":{"type":"array","items":{"type":"string"},"description":"要撤销的角色名称列表"}},"required":["email"]}`,
			BackendMethod: "INTERNAL",
			BackendPath:   "builtin://users/update",
			RiskLevel:     "medium",
		},
		{
			Name:          "builtin_update_role_permissions",
			DisplayName:   "角色权限更新",
			Description:   "更新当前租户角色的功能权限、MCP工具权限、技能权限和数据范围；内置锁定权限会自动保留。",
			InputSchema:   `{"type":"object","properties":{"role_name":{"type":"string","description":"角色名称"},"tools":{"type":"array","items":{"type":"string"},"description":"功能权限列表"},"allowed_mcp_tools":{"type":"array","items":{"type":"string"},"description":"允许使用的MCP工具ID列表"},"allowed_skills":{"type":"array","items":{"type":"string"},"description":"允许使用的技能ID列表"},"data_scope":{"type":"string","description":"数据范围 global/tenant/self"}},"required":["role_name"]}`,
			BackendMethod: "INTERNAL",
			BackendPath:   "builtin://roles/update-permissions",
			RiskLevel:     "medium",
		},
		{
			Name:          "builtin_test_curl_import",
			DisplayName:   "测试 curl 导入",
			Description:   "解析 curl 命令并先发起测试请求，返回可创建 MCP 工具的候选配置和测试结果 ID。",
			InputSchema:   `{"type":"object","properties":{"curl":{"type":"string","description":"curl 命令"},"timeout_seconds":{"type":"integer","description":"测试超时秒数，默认15"}},"required":["curl"]}`,
			BackendMethod: "INTERNAL",
			BackendPath:   "builtin://mcp-tools/test-curl-import",
			RiskLevel:     "medium",
		},
		{
			Name:          "builtin_create_mcp_tool_from_curl",
			DisplayName:   "通过 curl 创建 MCP 工具",
			Description:   "基于最近一次成功 curl 测试结果创建 MCP 工具；禁止未经测试直接创建。",
			InputSchema:   `{"type":"object","properties":{"test_result_id":{"type":"string","description":"curl 测试成功返回的结果ID"},"name":{"type":"string","description":"MCP工具名称"},"description":{"type":"string","description":"工具说明"},"risk_level":{"type":"string","description":"风险等级 low/medium/high"}},"required":["test_result_id","name"]}`,
			BackendMethod: "INTERNAL",
			BackendPath:   "builtin://mcp-tools/create-from-curl",
			RiskLevel:     "high",
		},
		{
			Name:          "builtin_create_skill",
			DisplayName:   "AI 创建 Skill",
			Description:   "AI 助手创建租户 Skill，默认 draft，用于沉淀可复用自动化流程。",
			InputSchema:   `{"type":"object","properties":{"name":{"type":"string"},"description":{"type":"string"},"steps":{"type":"string","description":"步骤 JSON 数组"},"triggers":{"type":"string","description":"触发器 JSON"},"input_schema":{"type":"string","description":"输入 JSON Schema"}},"required":["name","steps"]}`,
			BackendMethod: "INTERNAL",
			BackendPath:   "builtin://skills/create",
			RiskLevel:     "medium",
		},
		{
			Name:          "builtin_update_skill",
			DisplayName:   "AI 更新 Skill",
			Description:   "AI 助手更新租户 Skill，校验租户归属与生命周期。",
			InputSchema:   `{"type":"object","properties":{"skill_id":{"type":"string"},"name":{"type":"string"},"description":{"type":"string"},"steps":{"type":"string"},"status":{"type":"string","description":"draft/testing/published/disabled"}},"required":["skill_id"]}`,
			BackendMethod: "INTERNAL",
			BackendPath:   "builtin://skills/update",
			RiskLevel:     "medium",
		},
	}
}

func builtinConnectorID(tenantID string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("easp:builtin-governance-connector:"+tenantID)).String()
}

func builtinToolID(tenantID, name string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("easp:builtin-governance-tool:"+tenantID+":"+name)).String()
}

// EnsureTenantBuiltinMCPTools 创建/修复租户内置治理 MCP 工具，并强制绑定到租户管理员角色。
func EnsureTenantBuiltinMCPTools(tenantID string) ([]string, error) {
	if tenantID == "" || database.DB == nil {
		return nil, errors.New("tenantID and database are required")
	}

	connectorID := builtinConnectorID(tenantID)
	now := time.Now()
	_, err := database.DB.Exec(`
		INSERT INTO connectors (id, tenant_id, name, type, base_url, status, tools_count, is_builtin, locked, created_at, updated_at)
		VALUES (?, ?, ?, 'builtin', 'internal://easp', 'active', ?, 1, 1, ?, ?)
		ON DUPLICATE KEY UPDATE
			name = VALUES(name), type = VALUES(type), base_url = VALUES(base_url), status = 'active', tools_count = VALUES(tools_count), is_builtin = 1, locked = 1, updated_at = VALUES(updated_at)`,
		connectorID, tenantID, builtinGovernanceConnectorName, len(TenantBuiltinMCPToolDefinitions()), now, now)
	if err != nil {
		return nil, fmt.Errorf("ensure builtin connector: %w", err)
	}

	toolIDs := make([]string, 0, len(TenantBuiltinMCPToolDefinitions()))
	for _, def := range TenantBuiltinMCPToolDefinitions() {
		id := builtinToolID(tenantID, def.Name)
		toolIDs = append(toolIDs, id)
		_, err := database.DB.Exec(`
			INSERT INTO mcp_tools
			(id, tenant_id, connector_id, name, description, input_schema, backend_method, backend_path, risk_level, status, enabled, is_builtin, locked, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 1, 1, ?, ?)
			ON DUPLICATE KEY UPDATE
				connector_id = VALUES(connector_id), description = VALUES(description), input_schema = VALUES(input_schema),
				backend_method = VALUES(backend_method), backend_path = VALUES(backend_path), risk_level = VALUES(risk_level),
				status = VALUES(status), enabled = 1, is_builtin = 1, locked = 1, updated_at = VALUES(updated_at)`,
			id, tenantID, connectorID, def.Name, def.Description, def.InputSchema, def.BackendMethod, def.BackendPath, def.RiskLevel, skill.SkillStatusPublished, now, now)
		if err != nil {
			return nil, fmt.Errorf("ensure builtin MCP tool %s: %w", def.Name, err)
		}
	}

	if err := BindLockedMCPToolsToTenantAdmin(tenantID, toolIDs); err != nil {
		return nil, err
	}
	return toolIDs, nil
}

// EnsureAllTenantBuiltinMCPTools 为已有租户补齐内置治理 MCP 工具。
func EnsureAllTenantBuiltinMCPTools() {
	if database.DB == nil {
		return
	}
	var tenantIDs []string
	if err := database.DB.Select(&tenantIDs, "SELECT id FROM tenants"); err != nil {
		log.Printf("Failed to list tenants for builtin MCP seed: %v", err)
		return
	}
	for _, tenantID := range tenantIDs {
		if _, err := EnsureTenantBuiltinMCPTools(tenantID); err != nil {
			log.Printf("Failed to ensure builtin MCP tools for tenant %s: %v", tenantID, err)
		}
	}
}

func BindLockedMCPToolsToTenantAdmin(tenantID string, lockedIDs []string) error {
	roleRepo := repositories.NewRoleRepository()
	role, err := roleRepo.GetByName(tenantID, "管理员")
	if err != nil || role == nil {
		return fmt.Errorf("tenant admin role not found for %s", tenantID)
	}
	current := ""
	if role.AllowedMCPTools != nil {
		current = *role.AllowedMCPTools
	}
	merged, err := MergeLockedMCPToolIDs(current, lockedIDs)
	if err != nil {
		return err
	}
	role.AllowedMCPTools = &merged
	return roleRepo.Update(role)
}

func MergeLockedMCPToolIDs(existing string, lockedIDs []string) (string, error) {
	ids := []string{}
	if existing != "" {
		if err := json.Unmarshal([]byte(existing), &ids); err != nil {
			return "", fmt.Errorf("invalid allowed_mcp_tools JSON: %w", err)
		}
	}
	seen := map[string]bool{}
	merged := make([]string, 0, len(ids)+len(lockedIDs))
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		merged = append(merged, id)
	}
	for _, id := range lockedIDs {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		merged = append(merged, id)
	}
	b, err := json.Marshal(merged)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func ProtectedAllowedMCPToolsForRole(role *models.Role, requested string, lockedIDs []string) (string, error) {
	if role == nil || !IsTenantAdminRole(role) || len(lockedIDs) == 0 {
		if requested == "" {
			return "[]", nil
		}
		return requested, nil
	}
	return MergeLockedMCPToolIDs(requested, lockedIDs)
}

func IsTenantAdminRole(role *models.Role) bool {
	return role != nil && !role.IsSystem && role.Name == "管理员"
}

func GetLockedBuiltinMCPToolIDs(tenantID string) ([]string, error) {
	if tenantID == "" || database.DB == nil {
		return []string{}, nil
	}
	ids := []string{}
	err := database.DB.Select(&ids, "SELECT id FROM mcp_tools WHERE tenant_id = ? AND is_builtin = 1 AND locked = 1", tenantID)
	return ids, err
}

func EnsureMCPToolMutable(tool *models.MCPTool) error {
	if tool != nil && (tool.Locked || tool.IsBuiltin) {
		return errors.New("内置锁定 MCP 工具不可编辑、停用或删除")
	}
	return nil
}

func EnsureConnectorMutable(connector *models.Connector) error {
	if connector != nil && (connector.Locked || connector.IsBuiltin || connector.Type == "builtin") {
		return errors.New("内置锁定连接器不可编辑、停用或删除")
	}
	return nil
}
