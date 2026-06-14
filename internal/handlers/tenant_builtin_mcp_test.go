package handlers

import (
	"encoding/json"
	"testing"

	"github.com/easp-platform/easp/internal/models"
)

func TestDefaultTenantBuiltinMCPToolsCoverGovernanceCapabilities(t *testing.T) {
	defs := TenantBuiltinMCPToolDefinitions()

	want := map[string]bool{
		"查询用户":              false,
		"角色权限查询":            false,
		"更新用户":              false,
		"角色权限更新":            false,
		"测试 curl 导入":        false,
		"通过 curl 创建 MCP 工具": false,
		"AI 创建 Skill":       false,
		"AI 更新 Skill":       false,
	}
	for _, def := range defs {
		if _, ok := want[def.DisplayName]; ok {
			want[def.DisplayName] = true
		}
		if def.Name == "" || def.Description == "" || def.InputSchema == "" || def.BackendPath == "" {
			t.Fatalf("builtin tool definition must be complete: %+v", def)
		}
	}
	for name, got := range want {
		if !got {
			t.Fatalf("missing builtin governance MCP tool: %s; defs=%+v", name, defs)
		}
	}
}

func TestMergeLockedMCPToolIDsKeepsBuiltinIDs(t *testing.T) {
	input := `["custom-tool"]`
	got, err := MergeLockedMCPToolIDs(input, []string{"builtin-a", "builtin-b"})
	if err != nil {
		t.Fatalf("MergeLockedMCPToolIDs returned error: %v", err)
	}

	var ids []string
	if err := json.Unmarshal([]byte(got), &ids); err != nil {
		t.Fatalf("result is not JSON array: %v; got=%s", err, got)
	}
	for _, want := range []string{"custom-tool", "builtin-a", "builtin-b"} {
		if !stringSliceContains(ids, want) {
			t.Fatalf("merged role permission must contain %s, got %v", want, ids)
		}
	}
}

func TestRoleUpdatePreservesLockedBuiltinPermissionsForTenantAdmin(t *testing.T) {
	role := &models.Role{Name: "管理员"}
	adminTools := `["builtin-a","builtin-b"]`
	role.AllowedMCPTools = &adminTools

	requested := `[]`
	got, err := ProtectedAllowedMCPToolsForRole(role, requested, []string{"builtin-a", "builtin-b"})
	if err != nil {
		t.Fatalf("ProtectedAllowedMCPToolsForRole returned error: %v", err)
	}

	var ids []string
	if err := json.Unmarshal([]byte(got), &ids); err != nil {
		t.Fatalf("result is not JSON array: %v; got=%s", err, got)
	}
	for _, want := range []string{"builtin-a", "builtin-b"} {
		if !stringSliceContains(ids, want) {
			t.Fatalf("tenant admin role must keep locked builtin permission %s, got %v", want, ids)
		}
	}
}

func TestLockedMCPToolMutationRejected(t *testing.T) {
	locked := &models.MCPTool{Name: "builtin_list_users", IsBuiltin: true, Locked: true}
	if err := EnsureMCPToolMutable(locked); err == nil {
		t.Fatalf("locked builtin MCP tool mutation should be rejected")
	}

	custom := &models.MCPTool{Name: "custom", IsBuiltin: false, Locked: false}
	if err := EnsureMCPToolMutable(custom); err != nil {
		t.Fatalf("custom MCP tool should be mutable: %v", err)
	}
}

func stringSliceContains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
