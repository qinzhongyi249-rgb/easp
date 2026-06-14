package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/models"
	"github.com/easp-platform/easp/internal/skill"
	"github.com/google/uuid"
)

var curlImportTestStore = struct {
	sync.Mutex
	items map[string]CurlImportTestResult
}{items: map[string]CurlImportTestResult{}}

// IsBuiltinGovernanceTool reports whether a tool should be executed by EASP itself
// instead of being proxied to an external REST/MCP server.
func IsBuiltinGovernanceTool(tool models.MCPTool) bool {
	method := ""
	if tool.BackendMethod != nil {
		method = strings.ToUpper(strings.TrimSpace(*tool.BackendMethod))
	}
	path := ""
	if tool.BackendPath != nil {
		path = *tool.BackendPath
	}
	return tool.IsBuiltin || method == "INTERNAL" || strings.HasPrefix(path, "builtin://")
}

// ExecuteBuiltinGovernanceTool executes tenant-scoped built-in governance tools.
func ExecuteBuiltinGovernanceTool(ctx context.Context, tenantID string, tool models.MCPTool, arguments json.RawMessage) (map[string]interface{}, error) {
	if database.DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	if tenantID == "" {
		tenantID = tool.TenantID
	}
	if tenantID == "" {
		return nil, fmt.Errorf("tenant_id is required")
	}

	args, err := parseBuiltinArgs(arguments)
	if err != nil {
		return nil, err
	}
	path := ""
	if tool.BackendPath != nil {
		path = strings.TrimSpace(*tool.BackendPath)
	}

	switch path {
	case "builtin://users/query":
		return builtinQueryUsers(ctx, tenantID, args)
	case "builtin://roles/query-permissions":
		return builtinQueryRolePermissions(ctx, tenantID, args)
	case "builtin://users/update":
		return builtinUpdateUser(ctx, tenantID, args)
	case "builtin://roles/update-permissions":
		return builtinUpdateRolePermissions(ctx, tenantID, args)
	case "builtin://mcp-tools/test-curl-import":
		return builtinTestCurlImport(ctx, tenantID, args)
	case "builtin://mcp-tools/create-from-curl":
		return builtinCreateMCPToolFromCurl(ctx, tenantID, args)
	case "builtin://skills/create":
		return builtinCreateSkill(ctx, tenantID, args)
	case "builtin://skills/update":
		return builtinUpdateSkill(ctx, tenantID, args)
	default:
		return nil, fmt.Errorf("unknown builtin governance tool path: %s", path)
	}
}

func parseBuiltinArgs(raw json.RawMessage) (map[string]interface{}, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return map[string]interface{}{}, nil
	}
	var args map[string]interface{}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid builtin tool arguments: %w", err)
	}
	if args == nil {
		args = map[string]interface{}{}
	}
	return args, nil
}

func builtinQueryUsers(ctx context.Context, tenantID string, args map[string]interface{}) (map[string]interface{}, error) {
	email := stringArg(args, "email")
	query := `SELECT id, email, display_name, status, login_count, created_at FROM users WHERE tenant_id = ? AND deleted_at IS NULL`
	qargs := []interface{}{tenantID}
	if email != "" {
		query += " AND email = ?"
		qargs = append(qargs, email)
	}
	query += " ORDER BY created_at DESC LIMIT 200"

	rows, err := database.DB.QueryxContext(ctx, query, qargs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []map[string]interface{}{}
	for rows.Next() {
		var id, userEmail, displayName, status string
		var loginCount int
		var createdAt time.Time
		if err := rows.Scan(&id, &userEmail, &displayName, &status, &loginCount, &createdAt); err != nil {
			return nil, err
		}
		roles, err := roleNamesForUser(ctx, id)
		if err != nil {
			return nil, err
		}
		users = append(users, map[string]interface{}{
			"id":           id,
			"email":        userEmail,
			"display_name": displayName,
			"status":       status,
			"login_count":  loginCount,
			"roles":        roles,
			"created_at":   createdAt,
		})
	}
	return map[string]interface{}{"users": users, "total": len(users)}, rows.Err()
}

func builtinQueryRolePermissions(ctx context.Context, tenantID string, args map[string]interface{}) (map[string]interface{}, error) {
	roleName := stringArg(args, "role_name")
	query := `SELECT id, name, description, tools, allowed_mcp_tools, allowed_skills, rate_limit, data_scope, is_default, created_at, updated_at FROM roles WHERE tenant_id = ? AND is_system = 0`
	qargs := []interface{}{tenantID}
	if roleName != "" {
		query += " AND name = ?"
		qargs = append(qargs, roleName)
	}
	query += " ORDER BY name"

	rows, err := database.DB.QueryxContext(ctx, query, qargs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roles := []map[string]interface{}{}
	for rows.Next() {
		var id, name string
		var description, tools, allowedMCPTools, allowedSkills, rateLimit, dataScope sql.NullString
		var isDefault bool
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &name, &description, &tools, &allowedMCPTools, &allowedSkills, &rateLimit, &dataScope, &isDefault, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		roles = append(roles, map[string]interface{}{
			"id":                id,
			"name":              name,
			"description":       nullableString(description),
			"tools":             jsonArrayOrNil(tools.String, tools.Valid),
			"allowed_mcp_tools": jsonArrayOrNil(allowedMCPTools.String, allowedMCPTools.Valid),
			"allowed_skills":    jsonArrayOrNil(allowedSkills.String, allowedSkills.Valid),
			"rate_limit":        nullableString(rateLimit),
			"data_scope":        nullableString(dataScope),
			"is_default":        isDefault,
			"created_at":        createdAt,
			"updated_at":        updatedAt,
		})
	}
	return map[string]interface{}{"roles": roles, "total": len(roles)}, rows.Err()
}

func builtinUpdateUser(ctx context.Context, tenantID string, args map[string]interface{}) (map[string]interface{}, error) {
	email := stringArg(args, "email")
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}
	var userID string
	if err := database.DB.GetContext(ctx, &userID, "SELECT id FROM users WHERE tenant_id = ? AND email = ? AND deleted_at IS NULL", tenantID, email); err != nil {
		return nil, fmt.Errorf("user not found in tenant: %s", email)
	}

	sets := []string{}
	qargs := []interface{}{}
	if displayName := stringArg(args, "display_name"); displayName != "" {
		sets = append(sets, "display_name = ?")
		qargs = append(qargs, displayName)
	}
	if status := stringArg(args, "status"); status != "" {
		sets = append(sets, "status = ?")
		qargs = append(qargs, status)
	}
	if len(sets) > 0 {
		sets = append(sets, "updated_at = NOW()")
		qargs = append(qargs, userID)
		if _, err := database.DB.ExecContext(ctx, "UPDATE users SET "+strings.Join(sets, ", ")+" WHERE id = ?", qargs...); err != nil {
			return nil, err
		}
	}

	assigned := []string{}
	for _, roleName := range stringArrayArg(args, "assign_roles") {
		roleID, err := roleIDByName(ctx, tenantID, roleName)
		if err != nil {
			return nil, err
		}
		if _, err := database.DB.ExecContext(ctx, "INSERT IGNORE INTO user_roles (user_id, role_id) VALUES (?, ?)", userID, roleID); err != nil {
			return nil, err
		}
		assigned = append(assigned, roleName)
	}
	revoked := []string{}
	for _, roleName := range stringArrayArg(args, "revoke_roles") {
		roleID, err := roleIDByName(ctx, tenantID, roleName)
		if err != nil {
			return nil, err
		}
		if _, err := database.DB.ExecContext(ctx, "DELETE FROM user_roles WHERE user_id = ? AND role_id = ?", userID, roleID); err != nil {
			return nil, err
		}
		revoked = append(revoked, roleName)
	}

	return map[string]interface{}{"success": true, "user_id": userID, "email": email, "assigned_roles": assigned, "revoked_roles": revoked}, nil
}

func builtinUpdateRolePermissions(ctx context.Context, tenantID string, args map[string]interface{}) (map[string]interface{}, error) {
	roleName := stringArg(args, "role_name")
	if roleName == "" {
		return nil, fmt.Errorf("role_name is required")
	}
	var role struct {
		ID       string `db:"id"`
		Name     string `db:"name"`
		IsSystem bool   `db:"is_system"`
	}
	if err := database.DB.GetContext(ctx, &role, "SELECT id, name, is_system FROM roles WHERE tenant_id = ? AND name = ?", tenantID, roleName); err != nil {
		return nil, fmt.Errorf("role not found in tenant: %s", roleName)
	}
	if role.IsSystem {
		return nil, fmt.Errorf("system role permissions cannot be updated by tenant builtin tool")
	}

	sets := []string{}
	qargs := []interface{}{}
	if value, ok := jsonArrayField(args, "tools"); ok {
		sets = append(sets, "tools = ?")
		qargs = append(qargs, value)
	}
	if value, ok := jsonArrayField(args, "allowed_mcp_tools"); ok {
		if role.Name == "管理员" {
			lockedIDs, err := lockedBuiltinMCPToolIDs(ctx, tenantID)
			if err != nil {
				return nil, err
			}
			value, err = mergeJSONIDArray(value, lockedIDs)
			if err != nil {
				return nil, err
			}
		}
		sets = append(sets, "allowed_mcp_tools = ?")
		qargs = append(qargs, value)
	}
	if value, ok := jsonArrayField(args, "allowed_skills"); ok {
		sets = append(sets, "allowed_skills = ?")
		qargs = append(qargs, value)
	}
	if value := stringArg(args, "data_scope"); value != "" {
		sets = append(sets, "data_scope = ?")
		qargs = append(qargs, value)
	}
	if len(sets) == 0 {
		return nil, fmt.Errorf("no permission fields to update")
	}
	sets = append(sets, "updated_at = NOW()")
	qargs = append(qargs, role.ID)
	if _, err := database.DB.ExecContext(ctx, "UPDATE roles SET "+strings.Join(sets, ", ")+" WHERE id = ?", qargs...); err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true, "role_id": role.ID, "role_name": role.Name}, nil
}

func builtinTestCurlImport(ctx context.Context, tenantID string, args map[string]interface{}) (map[string]interface{}, error) {
	curlCommand := stringArg(args, "curl")
	if curlCommand == "" {
		return nil, fmt.Errorf("curl is required")
	}
	candidate, err := ParseCurlImportCommand(curlCommand)
	if err != nil {
		return nil, err
	}
	result := CurlImportTestResult{ID: uuid.New().String(), TenantID: tenantID, Candidate: candidate, CreatedAt: time.Now()}

	timeout := 15 * time.Second
	if raw := stringArg(args, "timeout_seconds"); raw != "" {
		// 保持简单：非法值使用默认超时。
		var n int
		if _, scanErr := fmt.Sscanf(raw, "%d", &n); scanErr == nil && n > 0 && n <= 60 {
			timeout = time.Duration(n) * time.Second
		}
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(reqCtx, candidate.Method, candidate.URL, strings.NewReader(candidate.Body))
	if err != nil {
		return nil, err
	}
	for k, v := range candidate.Headers {
		httpReq.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		result.Success = false
		result.ResponseBody = err.Error()
	} else {
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		result.StatusCode = resp.StatusCode
		result.ResponseBody = string(body)
		result.Success = resp.StatusCode >= 200 && resp.StatusCode < 400
	}
	curlImportTestStore.Lock()
	curlImportTestStore.items[result.ID] = result
	curlImportTestStore.Unlock()
	return map[string]interface{}{"success": result.Success, "test_result_id": result.ID, "candidate": result.Candidate, "status_code": result.StatusCode, "response_body": result.ResponseBody}, nil
}

func builtinCreateMCPToolFromCurl(ctx context.Context, tenantID string, args map[string]interface{}) (map[string]interface{}, error) {
	testResultID := stringArg(args, "test_result_id")
	if testResultID == "" {
		return nil, fmt.Errorf("test_result_id is required")
	}
	curlImportTestStore.Lock()
	result, ok := curlImportTestStore.items[testResultID]
	curlImportTestStore.Unlock()
	if !ok {
		return nil, fmt.Errorf("curl 测试结果不存在或已过期")
	}
	tool, err := BuildMCPToolFromCurlTestResult(tenantID, CurlImportCreateRequest{Name: stringArg(args, "name"), Description: stringArg(args, "description"), RiskLevel: stringArg(args, "risk_level")}, &result)
	if err != nil {
		return nil, err
	}
	connectorID, err := ensureCurlConnector(ctx, tenantID, result.Candidate.BaseURL)
	if err != nil {
		return nil, err
	}
	tool.ConnectorID = connectorID
	_, err = database.DB.ExecContext(ctx, `INSERT INTO mcp_tools (id, tenant_id, connector_id, name, description, input_schema, backend_method, backend_path, risk_level, status, enabled, is_builtin, locked, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, ?, ?)`, tool.ID, tool.TenantID, tool.ConnectorID, tool.Name, valueOrEmpty(tool.Description), valueOrEmpty(tool.InputSchema), valueOrEmpty(tool.BackendMethod), valueOrEmpty(tool.BackendPath), tool.RiskLevel, tool.Status, tool.Enabled, tool.CreatedAt, tool.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true, "tool_id": tool.ID, "name": tool.Name, "status": tool.Status, "enabled": tool.Enabled}, nil
}

func builtinCreateSkill(ctx context.Context, tenantID string, args map[string]interface{}) (map[string]interface{}, error) {
	name := stringArg(args, "name")
	steps := stringArg(args, "steps")
	if name == "" || steps == "" {
		return nil, fmt.Errorf("name and steps are required")
	}
	if err := validateJSONArrayString(steps, "steps"); err != nil {
		return nil, err
	}
	triggers := stringArg(args, "triggers")
	if triggers != "" {
		if err := validateJSONValueString(triggers, "triggers"); err != nil {
			return nil, err
		}
	}
	inputSchema := stringArg(args, "input_schema")
	if inputSchema != "" {
		if err := validateJSONValueString(inputSchema, "input_schema"); err != nil {
			return nil, err
		}
	}
	id := uuid.New().String()
	now := time.Now()
	_, err := database.DB.ExecContext(ctx, `INSERT INTO skills (id, tenant_id, name, description, version, triggers, input_schema, steps, status, created_at, updated_at) VALUES (?, ?, ?, ?, '1.0', ?, ?, ?, 'draft', ?, ?)`, id, tenantID, name, stringArg(args, "description"), nullableJSONString(triggers), nullableJSONString(inputSchema), steps, now, now)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true, "skill_id": id, "status": "draft"}, nil
}

func builtinUpdateSkill(ctx context.Context, tenantID string, args map[string]interface{}) (map[string]interface{}, error) {
	skillID := stringArg(args, "skill_id")
	if skillID == "" {
		return nil, fmt.Errorf("skill_id is required")
	}
	var exists string
	if err := database.DB.GetContext(ctx, &exists, "SELECT id FROM skills WHERE id = ? AND tenant_id = ?", skillID, tenantID); err != nil {
		return nil, fmt.Errorf("skill not found in tenant")
	}
	sets := []string{}
	qargs := []interface{}{}
	if name := stringArg(args, "name"); name != "" {
		sets = append(sets, "name = ?")
		qargs = append(qargs, name)
	}
	if _, ok := args["description"]; ok {
		sets = append(sets, "description = ?")
		qargs = append(qargs, stringArg(args, "description"))
	}
	if steps := stringArg(args, "steps"); steps != "" {
		if err := validateJSONArrayString(steps, "steps"); err != nil {
			return nil, err
		}
		sets = append(sets, "steps = ?")
		qargs = append(qargs, steps)
	}
	if status := stringArg(args, "status"); status != "" {
		normalized := skill.NormalizeSkillStatus(status)
		if normalized == status && normalized != "draft" && normalized != "testing" && normalized != "published" && normalized != "disabled" {
			return nil, fmt.Errorf("unsupported skill status: %s", status)
		}
		sets = append(sets, "status = ?")
		qargs = append(qargs, normalized)
	}
	if len(sets) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}
	sets = append(sets, "updated_at = NOW()")
	qargs = append(qargs, skillID)
	if _, err := database.DB.ExecContext(ctx, "UPDATE skills SET "+strings.Join(sets, ", ")+" WHERE id = ?", qargs...); err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true, "skill_id": skillID}, nil
}

func roleNamesForUser(ctx context.Context, userID string) ([]string, error) {
	roles := []string{}
	err := database.DB.SelectContext(ctx, &roles, `SELECT r.name FROM roles r JOIN user_roles ur ON ur.role_id = r.id WHERE ur.user_id = ? ORDER BY r.name`, userID)
	return roles, err
}

func roleIDByName(ctx context.Context, tenantID, roleName string) (string, error) {
	var roleID string
	if err := database.DB.GetContext(ctx, &roleID, "SELECT id FROM roles WHERE tenant_id = ? AND name = ?", tenantID, roleName); err != nil {
		return "", fmt.Errorf("role not found in tenant: %s", roleName)
	}
	return roleID, nil
}

func lockedBuiltinMCPToolIDs(ctx context.Context, tenantID string) ([]string, error) {
	ids := []string{}
	err := database.DB.SelectContext(ctx, &ids, "SELECT id FROM mcp_tools WHERE tenant_id = ? AND is_builtin = 1 AND locked = 1", tenantID)
	return ids, err
}

func mergeJSONIDArray(existing string, lockedIDs []string) (string, error) {
	ids := []string{}
	if strings.TrimSpace(existing) != "" {
		if err := json.Unmarshal([]byte(existing), &ids); err != nil {
			return "", fmt.Errorf("invalid JSON array: %w", err)
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
	b, err := json.Marshal(merged)
	return string(b), err
}

func jsonArrayField(args map[string]interface{}, key string) (string, bool) {
	v, ok := args[key]
	if !ok {
		return "", false
	}
	if s, ok := v.(string); ok {
		if strings.TrimSpace(s) == "" {
			return "[]", true
		}
		return s, true
	}
	b, _ := json.Marshal(v)
	return string(b), true
}

func stringArg(args map[string]interface{}, key string) string {
	v, ok := args[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func stringArrayArg(args map[string]interface{}, key string) []string {
	v, ok := args[key]
	if !ok || v == nil {
		return nil
	}
	switch typed := v.(type) {
	case []string:
		return typed
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if s := strings.TrimSpace(fmt.Sprint(item)); s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		var out []string
		if err := json.Unmarshal([]byte(typed), &out); err == nil {
			return out
		}
		return []string{strings.TrimSpace(typed)}
	default:
		return nil
	}
}

func ensureCurlConnector(ctx context.Context, tenantID, baseURL string) (string, error) {
	if baseURL == "" {
		return "", fmt.Errorf("base_url is required")
	}
	var id string
	err := database.DB.GetContext(ctx, &id, "SELECT id FROM connectors WHERE tenant_id = ? AND base_url = ? AND type = 'curl_import' LIMIT 1", tenantID, baseURL)
	if err == nil {
		return id, nil
	}
	id = uuid.New().String()
	now := time.Now()
	_, err = database.DB.ExecContext(ctx, `INSERT INTO connectors (id, tenant_id, name, type, base_url, auth_type, status, tools_count, created_at, updated_at) VALUES (?, ?, ?, 'curl_import', ?, 'none', 'active', 0, ?, ?)`, id, tenantID, "curl导入-"+baseURL, baseURL, now, now)
	return id, err
}

func valueOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func nullableJSONString(v string) interface{} {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

func validateJSONValueString(raw, field string) error {
	var decoded interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return fmt.Errorf("%s 不是合法 JSON: %w", field, err)
	}
	return nil
}

func validateJSONArrayString(raw, field string) error {
	var decoded []interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return fmt.Errorf("%s 必须是 JSON 数组: %w", field, err)
	}
	return nil
}

func nullableString(value sql.NullString) interface{} {
	if !value.Valid {
		return nil
	}
	return value.String
}

func jsonArrayOrNil(value string, valid bool) interface{} {
	if !valid || strings.TrimSpace(value) == "" {
		return nil
	}
	var out interface{}
	if err := json.Unmarshal([]byte(value), &out); err != nil {
		return value
	}
	return out
}
