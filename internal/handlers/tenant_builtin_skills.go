package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/easp-platform/easp/internal/database"
	"github.com/google/uuid"
)

const (
	BuiltinCreateUserSkillID = "builtin-create-user-workflow"
	BuiltinCreateRoleSkillID = "builtin-create-role-workflow"
	BuiltinCreateMCPSkillID  = "builtin-create-mcp-workflow"
)

type tenantBuiltinSkillDef struct {
	ID          string
	Name        string
	Description string
	Category    string
	Tags        string
	Triggers    string
	InputSchema string
	Steps       string
}

func tenantBuiltinSkillDefs() []tenantBuiltinSkillDef {
	return []tenantBuiltinSkillDef{
		{
			ID:          BuiltinCreateUserSkillID,
			Name:        "内置创建用户流程",
			Description: "先检查必要信息，再并发查询用户和角色现状，最后创建用户并可选分配角色。缺少 email/display_name 时会要求补充。",
			Category:    "内置治理",
			Tags:        `["builtin","governance","user","parallel"]`,
			Triggers:    `["创建用户","新增用户","添加用户"]`,
			InputSchema: `{"type":"object","properties":{"email":{"type":"string","description":"用户邮箱"},"display_name":{"type":"string","description":"显示名称"},"role_name":{"type":"string","description":"可选，创建后分配的角色名称"}},"required":["email","display_name"]}`,
			Steps: `[
				{"name":"check_required","type":"required","params":{"fields":["email","display_name"],"message":"创建用户需要补充邮箱和显示名称。角色名称可选。"}},
				{"name":"inspect_current","type":"parallel","params":{"steps":[
					{"name":"users_snapshot","type":"mcp_tool","action":"list_users","output_var":"users"},
					{"name":"roles_snapshot","type":"mcp_tool","action":"list_roles","output_var":"roles"}
				]},"output_var":"current_state"},
				{"name":"create_user","type":"mcp_tool","action":"create_user","params":{"email":"${email}","display_name":"${display_name}","role_name":"${role_name}"},"output_var":"created_user"}
			]`,
		},
		{
			ID:          BuiltinCreateRoleSkillID,
			Name:        "内置创建角色流程",
			Description: "先检查角色名称，再并发查询角色、MCP工具和Skill现状，最后创建角色。缺少 name 时会要求补充。",
			Category:    "内置治理",
			Tags:        `["builtin","governance","role","parallel"]`,
			Triggers:    `["创建角色","新增角色","添加角色"]`,
			InputSchema: `{"type":"object","properties":{"name":{"type":"string","description":"角色名称"},"description":{"type":"string"},"tools":{"type":"array","items":{"type":"string"}},"allowed_mcp_tools":{"type":"array","items":{"type":"string"}},"allowed_skills":{"type":"array","items":{"type":"string"}},"data_scope":{"type":"string","default":"tenant"},"rate_limit":{"type":"string","default":"500/hour"}},"required":["name"]}`,
			Steps: `[
				{"name":"check_required","type":"required","params":{"fields":["name"],"message":"创建角色需要补充角色名称。"}},
				{"name":"inspect_current","type":"parallel","params":{"steps":[
					{"name":"roles_snapshot","type":"mcp_tool","action":"list_roles","output_var":"roles"},
					{"name":"mcp_tools_snapshot","type":"mcp_tool","action":"list_mcp_tools","output_var":"mcp_tools"},
					{"name":"skills_snapshot","type":"mcp_tool","action":"list_skills","output_var":"skills"}
				]},"output_var":"current_state"},
				{"name":"create_role","type":"mcp_tool","action":"create_role","params":{"name":"${name}","description":"${description}","tools":"${tools}","allowed_mcp_tools":"${allowed_mcp_tools}","allowed_skills":"${allowed_skills}","data_scope":"${data_scope}","rate_limit":"${rate_limit}"},"output_var":"created_role"}
			]`,
		},
		{
			ID:          BuiltinCreateMCPSkillID,
			Name:        "内置创建MCP工具流程",
			Description: "先检查 curl 命令和工具名称，再测试 curl，测试成功后基于 test_result_id 创建 draft/disabled MCP 工具。缺少 curl/name 时会要求补充。",
			Category:    "内置治理",
			Tags:        `["builtin","governance","mcp","curl"]`,
			Triggers:    `["创建MCP工具","新增MCP工具","curl导入MCP","导入MCP"]`,
			InputSchema: `{"type":"object","properties":{"curl":{"type":"string","description":"待测试并导入的 curl 命令"},"name":{"type":"string","description":"MCP工具名称"},"description":{"type":"string","description":"工具说明"},"risk_level":{"type":"string","description":"风险等级 low/medium/high，默认 medium"},"timeout_seconds":{"type":"integer","description":"curl 测试超时秒数，默认15"}},"required":["curl","name"]}`,
			Steps: `[
				{"name":"check_required","type":"required","params":{"fields":["curl","name"],"message":"创建 MCP 工具需要补充 curl 命令和工具名称。"}},
				{"name":"test_curl","type":"mcp_tool","action":"builtin_test_curl_import","params":{"curl":"${curl}","timeout_seconds":"${timeout_seconds}"},"output_var":"curl_test"},
				{"name":"create_mcp_tool","type":"mcp_tool","action":"builtin_create_mcp_tool_from_curl","params":{"test_result_id":"${curl_test.test_result_id}","name":"${name}","description":"${description}","risk_level":"${risk_level}"},"output_var":"created_mcp_tool"}
			]`,
		},
	}
}

func EnsureTenantBuiltinSkills(tenantID string) ([]string, error) {
	if database.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	now := time.Now()
	ids := make([]string, 0, len(tenantBuiltinSkillDefs()))
	for _, def := range tenantBuiltinSkillDefs() {
		skillID := tenantBuiltinSkillID(tenantID, def.ID)
		ids = append(ids, skillID)
		_, err := database.DB.Exec(`INSERT INTO skills (id, tenant_id, name, description, category, version, tags, triggers, input_schema, steps, status, created_by, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, '1.0', ?, ?, ?, ?, 'published', 'system', ?, ?)
			ON DUPLICATE KEY UPDATE description=VALUES(description), category=VALUES(category), tags=VALUES(tags), triggers=VALUES(triggers), input_schema=VALUES(input_schema), steps=VALUES(steps), status='published', created_by='system', updated_at=VALUES(updated_at)`,
			skillID, tenantID, def.Name, def.Description, def.Category, def.Tags, def.Triggers, def.InputSchema, def.Steps, now, now)
		if err != nil {
			return ids, fmt.Errorf("ensure builtin skill %s: %w", def.Name, err)
		}
	}
	if err := BindLockedSkillsToTenantAdmin(tenantID, ids); err != nil {
		return ids, err
	}
	return ids, nil
}

func BindLockedSkillsToTenantAdmin(tenantID string, lockedIDs []string) error {
	if len(lockedIDs) == 0 {
		return nil
	}
	var current string
	err := database.DB.Get(&current, "SELECT COALESCE(allowed_skills, '[]') FROM roles WHERE tenant_id = ? AND name = '管理员' AND is_system = 0 LIMIT 1", tenantID)
	if err != nil {
		return err
	}
	merged, err := mergeJSONIDArrayForSkills(current, lockedIDs)
	if err != nil {
		return err
	}
	_, err = database.DB.Exec("UPDATE roles SET allowed_skills = ?, updated_at = NOW() WHERE tenant_id = ? AND name = '管理员' AND is_system = 0", merged, tenantID)
	return err
}

func GetLockedBuiltinSkillIDs(tenantID string) ([]string, error) {
	defs := tenantBuiltinSkillDefs()
	ids := make([]string, 0, len(defs))
	for _, def := range defs {
		ids = append(ids, tenantBuiltinSkillID(tenantID, def.ID))
	}
	return ids, nil
}

func tenantBuiltinSkillID(tenantID, key string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(tenantID+":"+key)).String()
}

func ProtectedAllowedSkillsForRole(roleTenantID string, requested string, lockedIDs []string) (string, error) {
	return mergeJSONIDArrayForSkills(requested, lockedIDs)
}

func mergeJSONIDArrayForSkills(existing string, lockedIDs []string) (string, error) {
	ids := []string{}
	if existing != "" {
		if err := json.Unmarshal([]byte(existing), &ids); err != nil {
			return "", err
		}
	}
	seen := map[string]bool{}
	merged := make([]string, 0, len(ids)+len(lockedIDs))
	for _, id := range append(ids, lockedIDs...) {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		merged = append(merged, id)
	}
	data, err := json.Marshal(merged)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func EnsureAllTenantBuiltinSkills() {
	if database.DB == nil {
		return
	}
	var tenantIDs []string
	if err := database.DB.Select(&tenantIDs, "SELECT id FROM tenants"); err != nil {
		log.Printf("Failed to list tenants for builtin skill seed: %v", err)
		return
	}
	for _, tenantID := range tenantIDs {
		EnsureTenantBuiltinSkillsOrLog(tenantID)
	}
}

func EnsureTenantBuiltinSkillsOrLog(tenantID string) {
	if _, err := EnsureTenantBuiltinSkills(tenantID); err != nil {
		log.Printf("Failed to ensure tenant builtin skills for %s: %v", tenantID, err)
	}
}
