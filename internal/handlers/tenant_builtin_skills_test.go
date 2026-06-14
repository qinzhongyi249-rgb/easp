package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/easp-platform/easp/internal/models"
	skillengine "github.com/easp-platform/easp/internal/skill"
)

func TestTenantBuiltinSkillDefsIncludesCreateMCPWorkflow(t *testing.T) {
	defs := tenantBuiltinSkillDefs()
	var found *tenantBuiltinSkillDef
	for i := range defs {
		if defs[i].ID == BuiltinCreateMCPSkillID {
			found = &defs[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected builtin create MCP workflow skill to be seeded")
	}
	if found.Name != "内置创建MCP工具流程" {
		t.Fatalf("unexpected name: %s", found.Name)
	}
	if !strings.Contains(found.Tags, "mcp") || !strings.Contains(found.Tags, "curl") {
		t.Fatalf("expected mcp/curl tags, got %s", found.Tags)
	}

	var schema struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal([]byte(found.InputSchema), &schema); err != nil {
		t.Fatalf("invalid input schema: %v", err)
	}
	wantRequired := map[string]bool{"curl": false, "name": false}
	for _, field := range schema.Required {
		if _, ok := wantRequired[field]; ok {
			wantRequired[field] = true
		}
	}
	for field, ok := range wantRequired {
		if !ok {
			t.Fatalf("expected required field %s in schema %#v", field, schema.Required)
		}
	}

	var steps []map[string]interface{}
	if err := json.Unmarshal([]byte(found.Steps), &steps); err != nil {
		t.Fatalf("invalid steps json: %v", err)
	}
	if len(steps) != 3 {
		t.Fatalf("expected required + test curl + create mcp steps, got %d", len(steps))
	}
	if steps[0]["type"] != "required" {
		t.Fatalf("first step should require missing information, got %#v", steps[0])
	}
	if steps[1]["action"] != "builtin_test_curl_import" {
		t.Fatalf("second step should test curl import first, got %#v", steps[1])
	}
	if steps[2]["action"] != "builtin_create_mcp_tool_from_curl" {
		t.Fatalf("third step should create MCP tool from tested curl, got %#v", steps[2])
	}
}

func TestBuiltinCreateUserWorkflowRequiresUserAndRolePermissionsBeforeAskingFields(t *testing.T) {
	var def *tenantBuiltinSkillDef
	for i := range tenantBuiltinSkillDefs() {
		defs := tenantBuiltinSkillDefs()
		if defs[i].ID == BuiltinCreateUserSkillID {
			def = &defs[i]
			break
		}
	}
	if def == nil {
		t.Fatal("builtin create user workflow not found")
	}

	missing, err := missingPermissionsForSkillSteps(def.Steps, []string{"skills"})
	if err != nil {
		t.Fatalf("missingPermissionsForSkillSteps returned error: %v", err)
	}
	if strings.Join(missing, ",") != "roles,users" {
		t.Fatalf("expected roles/users permissions to be required before asking fields, got %#v", missing)
	}

	if msg := skillMissingPermissionMessage(def.Name, missing); !strings.Contains(msg, "用户管理") || !strings.Contains(msg, "角色管理") {
		t.Fatalf("expected localized missing permission message, got %q", msg)
	}

	if missing, err := missingPermissionsForSkillSteps(def.Steps, []string{"skills", "users", "roles"}); err != nil || len(missing) != 0 {
		t.Fatalf("expected no missing permissions when users/roles are granted, missing=%#v err=%v", missing, err)
	}
}

func TestAssistantIntentPermissionPrecheckForCreateUser(t *testing.T) {
	cases := []string{
		"创建一个测试账号",
		"帮我创建个测试用户",
		"请新增一个用户",
	}
	for _, text := range cases {
		intent, missing := assistantIntentMissingPermissions(text, []string{"skills"})
		if intent != "创建用户" {
			t.Fatalf("%q: expected create user intent, got %q", text, intent)
		}
		if strings.Join(missing, ",") != "roles,users" {
			t.Fatalf("%q: expected roles/users missing, got %#v", text, missing)
		}
	}

	intent, missing := assistantIntentMissingPermissions("创建一个测试账号", []string{"users", "roles"})
	if intent != "" || len(missing) != 0 {
		t.Fatalf("expected no missing permissions when granted, intent=%q missing=%#v", intent, missing)
	}
}

func TestRequiresInputReplyUsesChineseFieldLabels(t *testing.T) {
	result := `{"success":true,"status":"requires_input","outputs":{"message":"创建用户需要补充邮箱和显示名称。角色名称可选。","missing_fields":["email","display_name"]}}`
	reply, ok := requiresInputReplyFromToolResult(result, "创建一个测试账号")
	if !ok {
		t.Fatal("expected requires_input reply")
	}
	if !strings.Contains(reply, "邮箱") || !strings.Contains(reply, "显示名称") {
		t.Fatalf("expected Chinese field labels, got %q", reply)
	}
	if strings.Contains(reply, "display_name") {
		t.Fatalf("reply should not expose raw field name display_name: %q", reply)
	}
}

func TestSkillToolParametersDoNotExposeRequiredFieldsToModel(t *testing.T) {
	var schema map[string]any
	defs := tenantBuiltinSkillDefs()
	for i := range defs {
		if defs[i].ID == BuiltinCreateUserSkillID {
			if err := json.Unmarshal([]byte(defs[i].InputSchema), &schema); err != nil {
				t.Fatalf("invalid schema: %v", err)
			}
			break
		}
	}
	params := skillToolParametersForModel(schema)
	if _, ok := params["required"]; ok {
		t.Fatalf("skill tool schema exposed required fields to model: %#v", params["required"])
	}
}

func TestUnavailableCapabilityPromptExplainsHiddenSkillPermissions(t *testing.T) {
	lines := unavailableCapabilityLines([]ToolDefinition{{
		Type: "function",
		Function: FunctionDef{
			Name:        "skill_create_user_workflow",
			Description: "[技能] 内置创建用户流程",
		},
	}}, map[string][]string{
		"skill_create_user_workflow": {"roles", "users"},
	})
	if len(lines) != 1 {
		t.Fatalf("expected one unavailable capability, got %#v", lines)
	}
	if !strings.Contains(lines[0], "内置创建用户流程") || !strings.Contains(lines[0], "用户管理") || !strings.Contains(lines[0], "角色管理") {
		t.Fatalf("expected Chinese skill/permission labels, got %q", lines[0])
	}
}

func TestSystemPromptIncludesUnavailableCapabilities(t *testing.T) {
	prompt := getSystemPrompt("tenant-a", []string{"list_roles"}, []string{"内置创建用户流程：缺少 用户管理、角色管理"})
	if !strings.Contains(prompt, "不可用能力") || !strings.Contains(prompt, "内置创建用户流程") {
		t.Fatalf("expected unavailable capabilities in prompt, got: %s", prompt)
	}
	if !strings.Contains(prompt, "如果用户请求不可用能力") {
		t.Fatalf("expected permission-first instruction in prompt, got: %s", prompt)
	}
}

func TestToolPermissionDeniedResultFromUnavailableTool(t *testing.T) {
	result, ok := toolPermissionDeniedResult("skill_create_user_workflow", map[string][]string{
		"skill_create_user_workflow": {"roles", "users"},
	})
	if !ok {
		t.Fatal("expected permission denied result for unavailable tool")
	}
	if !strings.Contains(result, "permission_denied") || !strings.Contains(result, "用户管理") || !strings.Contains(result, "角色管理") {
		t.Fatalf("expected localized permission denied result, got %s", result)
	}
}

func TestBuiltinCreateMCPWorkflowRequiresCurlAndNameBeforeToolCalls(t *testing.T) {
	var def *tenantBuiltinSkillDef
	for i := range tenantBuiltinSkillDefs() {
		if tenantBuiltinSkillDefs()[i].ID == BuiltinCreateMCPSkillID {
			def = &tenantBuiltinSkillDefs()[i]
			break
		}
	}
	if def == nil {
		t.Fatal("builtin create MCP workflow not found")
	}

	calls := 0
	engine := skillengine.NewSkillEngineWithCaller("tenant-a", func(ctx context.Context, toolName string, arguments json.RawMessage) (map[string]interface{}, error) {
		calls++
		return map[string]interface{}{"success": true}, nil
	})
	exec, err := engine.ExecuteWithMode(context.Background(), models.Skill{
		ID:       "skill-create-mcp",
		TenantID: "tenant-a",
		Name:     def.Name,
		Steps:    def.Steps,
		Status:   skillengine.SkillStatusPublished,
	}, map[string]interface{}{"curl": "curl http://127.0.0.1:8082/health"}, skillengine.ExecutionModeProduction)
	if err != nil {
		t.Fatalf("ExecuteWithMode returned unexpected error: %v", err)
	}
	if exec.Status != "requires_input" {
		t.Fatalf("expected requires_input status, got %s", exec.Status)
	}
	if calls != 0 {
		t.Fatalf("expected no MCP tool calls when name is missing, got %d", calls)
	}
	missing, _ := exec.Outputs["missing_fields"].([]string)
	if len(missing) != 1 || missing[0] != "name" {
		t.Fatalf("expected missing name, got %#v", exec.Outputs["missing_fields"])
	}
}
