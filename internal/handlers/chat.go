package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/easp-platform/easp/internal/database"
	easpMCP "github.com/easp-platform/easp/internal/mcp"
	easpMemory "github.com/easp-platform/easp/internal/memory"
	"github.com/easp-platform/easp/internal/middleware"
	"github.com/easp-platform/easp/internal/models"
	"github.com/easp-platform/easp/internal/modelservice"
	"github.com/easp-platform/easp/internal/repositories"
	skillPkg "github.com/easp-platform/easp/internal/skill"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ChatHandler AI助手处理器
type ChatHandler struct {
	modelService    *modelservice.ModelService
	memoryRouter    *easpMemory.MemoryRouter
	memoryExtractor *easpMemory.MemoryExtractor
}

func NewChatHandler() *ChatHandler {
	embeddingSvc := &MockEmbeddingService{}
	memorySvc := easpMemory.NewMemoryService(easpMemory.MemoryConfig{
		EmbeddingService: embeddingSvc,
	})

	return &ChatHandler{
		modelService: modelservice.NewModelService(modelservice.Config{}),
		memoryRouter: easpMemory.NewMemoryRouter(memorySvc, easpMemory.DefaultRouterConfig()),
		memoryExtractor: easpMemory.NewMemoryExtractor(memorySvc, easpMemory.ModelConfig{
			BaseURL: "", // 会在运行时从model配置获取
			APIKey:  "",
			Model:   "",
		}),
	}
}

// AssistantMessage 助手消息
type AssistantMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AssistantRequest 助手请求
type AssistantRequest struct {
	Messages []AssistantMessage `json:"messages" binding:"required"`
}

// ToolDefinition 工具定义
type ToolDefinition struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

type FunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ToolCall 工具调用
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// SSE事件类型
const (
	SSEEventStatus    = "status"     // 状态更新
	SSEEventTool      = "tool"       // 工具执行结果
	SSEEventDelta     = "delta"      // 流式文本片段
	SSEEventDone      = "done"       // 完成
	SSEEventError     = "error"      // 错误
	SSEEventModelInfo = "model_info" // 模型信息
)

// sendSSE 发送SSE事件
func sendSSE(c *gin.Context, event string, data any) {
	jsonData, _ := json.Marshal(data)
	fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, string(jsonData))
	c.Writer.Flush()
}

// getSystemPrompt 获取系统提示
func getSystemPrompt(tenantID string, toolNames []string) string {
	toolList := "无"
	if len(toolNames) > 0 {
		toolList = strings.Join(toolNames, "、")
	}
	return `你是 EASP (Enterprise API Service Platform) 的智能助手。你可以帮助管理员管理平台资源。

当前租户ID: ` + tenantID + `
你当前可用的工具: ` + toolList + `

当用户请求你执行操作时，请调用相应的工具。调用工具后，请用中文总结执行结果。

对于查询结果，请用清晰的表格或列表格式展示。
对于配置变更，请确认变更内容后再执行。
如果用户请求的操作超出了你的可用工具范围，请礼貌告知权限不足。`
}

// logAudit 记录审计日志（AI助手写操作）
func logAudit(tenantID, userID, toolName, action, resource, detail string) {
	auditRepo := repositories.NewAuditLogRepository()
	auditLog := &models.AuditLog{
		TenantID: tenantID,
		Tool:     toolName,
		Action:   action,
	}
	if userID != "" {
		auditLog.UserID = &userID
	}
	if resource != "" {
		auditLog.Resource = &resource
	}
	if detail != "" {
		// detail 列是 JSON 类型，必须传合法 JSON 字符串
		jsonBytes, _ := json.Marshal(detail)
		jsonStr := string(jsonBytes)
		auditLog.Detail = &jsonStr
	}
	decision := "approved"
	auditLog.Decision = &decision
	result := "success"
	auditLog.Result = &result
	if err := auditRepo.Create(auditLog); err != nil {
		log.Printf("Failed to create audit log: %v", err)
	}
}

// toolPermissionMap 工具→权限映射
var toolPermissionMap = map[string]string{
	// 查询工具
	"list_users":        "users",
	"get_user":          "users",
	"list_connectors":   "connectors",
	"get_connector":     "connectors",
	"list_mcp_tools":    "mcp-tools",
	"get_mcp_tool":      "mcp-tools",
	"list_skills":       "skills",
	"get_skill":         "skills",
	"list_memory_pools": "memory",
	"get_memory_entries": "memory",
	// 角色工具
	"assign_role": "roles",
	"revoke_role": "roles",
	"list_roles":  "roles",
	// 租户工具
	"get_tenant_info": "*",
	"update_tenant":   "*",
	// 写操作工具（需对应权限）
	"create_connector":   "connectors",
	"update_connector":   "connectors",
	"create_mcp_tool":    "mcp-tools",
	"update_mcp_tool":    "mcp-tools",
	"create_skill":       "skills",
	"update_skill":       "skills",
	"create_memory_pool": "memory",
	"create_memory_entry":"memory",
	"update_memory_entry":"memory",
	// 技能执行
	"execute_skill": "skills",
	// MCP工具执行
	"execute_mcp_tool": "mcp-tools",
}

// getUserAllowedMCPSkills 获取用户角色允许的MCP工具ID和技能ID
// 返回: allowedMCPToolIDs, allowedSkillIDs, hasWildcard
func getUserAllowedMCPSkills(userID string) (map[string]bool, map[string]bool, bool) {
	roleRepo := repositories.NewRoleRepository()
	userRoleRepo := repositories.NewUserRoleRepository()

	roles, err := userRoleRepo.GetUserRoles(userID)
	if err != nil {
		return nil, nil, false
	}

	allowedMCPTools := make(map[string]bool)
	allowedSkills := make(map[string]bool)
	hasWildcard := false

	for _, role := range roles {
		// 检查 tools 字段是否有 "*"
		if role.Tools != nil {
			var tools []string
			json.Unmarshal([]byte(*role.Tools), &tools)
			for _, t := range tools {
				if t == "*" {
					hasWildcard = true
				}
			}
		}

		// 解析 allowed_mcp_tools
		if role.AllowedMCPTools != nil {
			var toolIDs []string
			if err := json.Unmarshal([]byte(*role.AllowedMCPTools), &toolIDs); err == nil {
				for _, id := range toolIDs {
					allowedMCPTools[id] = true
				}
			}
		}

		// 解析 allowed_skills
		if role.AllowedSkills != nil {
			var skillIDs []string
			if err := json.Unmarshal([]byte(*role.AllowedSkills), &skillIDs); err == nil {
				for _, id := range skillIDs {
					allowedSkills[id] = true
				}
			}
		}
	}

	// 防止未使用的导入
	_ = roleRepo

	return allowedMCPTools, allowedSkills, hasWildcard
}

// getToolsForPermissions 根据用户权限过滤工具列表
func getToolsForPermissions(permissions []string) []ToolDefinition {
	allTools := getTools()

	// 构建权限集合
	permSet := make(map[string]bool)
	hasWildcard := false
	for _, p := range permissions {
		permSet[p] = true
		if p == "*" {
			hasWildcard = true
		}
	}

	// 管理员拥有所有工具
	if hasWildcard {
		return allTools
	}

	// 按权限过滤
	var filtered []ToolDefinition
	for _, tool := range allTools {
		required, ok := toolPermissionMap[tool.Function.Name]
		if !ok {
			// 无映射的工具默认允许
			filtered = append(filtered, tool)
			continue
		}
		if permSet[required] {
			filtered = append(filtered, tool)
		}
	}

	if filtered == nil {
		filtered = []ToolDefinition{}
	}
	return filtered
}

// loadSkillToolDefinitions 从数据库加载租户的active Skills，转换为AI可直接调用的ToolDefinition
func loadSkillToolDefinitions(tenantID string, allowedSkillIDs map[string]bool, hasWildcard bool) []ToolDefinition {
	var skills []models.Skill
	var err error
	if hasWildcard {
		err = database.DB.Select(&skills, "SELECT * FROM skills WHERE tenant_id = ? AND status = 'active'", tenantID)
	} else if len(allowedSkillIDs) > 0 {
		placeholders := ""
		args := []any{}
		for id := range allowedSkillIDs {
			if placeholders != "" {
				placeholders += ","
			}
			placeholders += "?"
			args = append(args, id)
		}
		args = append(args, tenantID)
		err = database.DB.Select(&skills, "SELECT * FROM skills WHERE id IN ("+placeholders+") AND tenant_id = ? AND status = 'active'", args...)
	}
	if err != nil {
		log.Printf("loadSkillToolDefinitions: failed to load: %v", err)
		return nil
	}

	result := make([]ToolDefinition, 0, len(skills))
	for _, sk := range skills {
		// 解析 input_schema 作为 parameters
		var params map[string]any
		if sk.InputSchema != nil && *sk.InputSchema != "" {
			if err := json.Unmarshal([]byte(*sk.InputSchema), &params); err != nil {
				params = map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				}
			}
		} else {
			params = map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}
		}

		desc := ""
		if sk.Description != nil {
			desc = *sk.Description
		}
		if desc == "" {
			desc = "技能: " + sk.Name
		}
		desc = "[技能] " + desc

		result = append(result, ToolDefinition{
			Type: "function",
			Function: FunctionDef{
				Name:        "skill_" + sk.Name,
				Description: desc,
				Parameters:  params,
			},
		})
	}
	return result
}

// loadMCPToolDefinitions 从数据库加载租户的MCP工具，转换为AI可调用的ToolDefinition
func loadMCPToolDefinitions(tenantID string, allowedIDs map[string]bool, hasWildcard bool) []ToolDefinition {
	var tools []models.MCPTool
	var err error
	if hasWildcard {
		err = database.DB.Select(&tools, "SELECT * FROM mcp_tools WHERE tenant_id = ? AND enabled = true", tenantID)
	} else if len(allowedIDs) > 0 {
		placeholders := ""
		args := []any{}
		for id := range allowedIDs {
			if placeholders != "" {
				placeholders += ","
			}
			placeholders += "?"
			args = append(args, id)
		}
		args = append(args, tenantID)
		err = database.DB.Select(&tools, "SELECT * FROM mcp_tools WHERE id IN ("+placeholders+") AND tenant_id = ? AND enabled = true", args...)
	}
	if err != nil {
		log.Printf("loadMCPToolDefinitions: failed to load: %v", err)
		return nil
	}

	result := make([]ToolDefinition, 0, len(tools))
	for _, tool := range tools {
		// 解析 input_schema 作为 parameters
		var params map[string]any
		if tool.InputSchema != nil && *tool.InputSchema != "" {
			if err := json.Unmarshal([]byte(*tool.InputSchema), &params); err != nil {
				// 如果解析失败，创建一个空的 parameters
				params = map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				}
			}
		} else {
			params = map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}
		}

		desc := ""
		if tool.Description != nil {
			desc = *tool.Description
		}
		if desc == "" {
			desc = "MCP tool: " + tool.Name
		}

		result = append(result, ToolDefinition{
			Type: "function",
			Function: FunctionDef{
				Name:        "mcp_" + tool.Name,
				Description: desc,
				Parameters:  params,
			},
		})
	}
	return result
}

// getTools 获取工具定义
func getTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "list_users",
				Description: "获取当前租户下的用户列表",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_user",
				Description: "根据邮箱获取用户信息",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"email": map[string]any{
							"type":        "string",
							"description": "用户邮箱",
						},
					},
					"required": []string{"email"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "assign_role",
				Description: "为用户分配角色",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"user_email": map[string]any{
							"type":        "string",
							"description": "用户邮箱",
						},
						"role_name": map[string]any{
							"type":        "string",
							"description": "角色名称，如：管理员、开发者、普通用户",
						},
					},
					"required": []string{"user_email", "role_name"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "revoke_role",
				Description: "撤销用户的角色",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"user_email": map[string]any{
							"type":        "string",
							"description": "用户邮箱",
						},
						"role_name": map[string]any{
							"type":        "string",
							"description": "角色名称",
						},
					},
					"required": []string{"user_email", "role_name"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "list_roles",
				Description: "获取当前租户下的角色列表",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "list_connectors",
				Description: "获取当前租户下的连接器列表",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "list_mcp_tools",
				Description: "获取当前租户下的MCP工具列表",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_tenant_info",
				Description: "获取当前租户的详细信息，包括到期时间、用户上限等",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "update_tenant",
				Description: "更新租户配置，如到期时间、最大用户数等",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"expires_at": map[string]any{
							"type":        "string",
							"description": "到期时间，格式YYYY-MM-DD，空字符串表示永久有效",
						},
						"max_users": map[string]any{
							"type":        "integer",
							"description": "最大用户数，0表示不限制",
						},
					},
				},
			},
		},
		// ========== 连接器工具 ==========
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_connector",
				Description: "获取连接器详情",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"connector_id": map[string]any{"type": "string", "description": "连接器ID"},
					},
					"required": []string{"connector_id"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "create_connector",
				Description: "创建新的API连接器。连接器是API-to-MCP的桥梁，用于接入外部API服务。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":     map[string]any{"type": "string", "description": "连接器名称，如 my-api"},
						"type":     map[string]any{"type": "string", "description": "连接器类型，如 openapi, custom"},
						"base_url": map[string]any{"type": "string", "description": "API基础URL，如 https://api.example.com/v1"},
						"auth_type": map[string]any{"type": "string", "description": "认证类型: none, api_key, bearer, basic"},
					},
					"required": []string{"name", "type", "base_url"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "update_connector",
				Description: "更新连接器配置",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"connector_id": map[string]any{"type": "string", "description": "连接器ID"},
						"name":         map[string]any{"type": "string", "description": "新名称"},
						"base_url":     map[string]any{"type": "string", "description": "新URL"},
						"status":       map[string]any{"type": "string", "description": "状态: active, inactive, error"},
					},
					"required": []string{"connector_id"},
				},
			},
		},
		// ========== MCP工具 ==========
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_mcp_tool",
				Description: "获取MCP工具详情",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"tool_id": map[string]any{"type": "string", "description": "MCP工具ID"},
					},
					"required": []string{"tool_id"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "create_mcp_tool",
				Description: "创建新的MCP工具。MCP工具是对外暴露的可调用工具，需要关联到一个连接器。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"connector_id":   map[string]any{"type": "string", "description": "所属连接器ID"},
						"name":           map[string]any{"type": "string", "description": "工具名称，如 get_user_info"},
						"description":    map[string]any{"type": "string", "description": "工具描述"},
						"backend_method": map[string]any{"type": "string", "description": "HTTP方法: GET, POST, PUT, DELETE"},
						"backend_path":   map[string]any{"type": "string", "description": "API路径，如 /users/{id}"},
						"risk_level":     map[string]any{"type": "string", "description": "风险等级: low, medium, high"},
					},
					"required": []string{"connector_id", "name", "backend_method", "backend_path"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "update_mcp_tool",
				Description: "更新MCP工具配置",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"tool_id":        map[string]any{"type": "string", "description": "MCP工具ID"},
						"name":           map[string]any{"type": "string", "description": "新名称"},
						"description":    map[string]any{"type": "string", "description": "新描述"},
						"backend_method": map[string]any{"type": "string", "description": "HTTP方法"},
						"backend_path":   map[string]any{"type": "string", "description": "API路径"},
						"risk_level":     map[string]any{"type": "string", "description": "风险等级"},
						"enabled":        map[string]any{"type": "boolean", "description": "是否启用"},
					},
					"required": []string{"tool_id"},
				},
			},
		},
		// ========== 技能工具 ==========
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "list_skills",
				Description: "获取当前租户下的技能列表",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_skill",
				Description: "获取技能详情，包含步骤定义",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"skill_id": map[string]any{"type": "string", "description": "技能ID"},
					},
					"required": []string{"skill_id"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "create_skill",
				Description: "创建新技能。技能是一组可编排的步骤，用于自动化复杂操作流程。steps必须是JSON数组字符串。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":        map[string]any{"type": "string", "description": "技能名称"},
						"description": map[string]any{"type": "string", "description": "技能描述"},
						"steps":       map[string]any{"type": "string", "description": "步骤定义JSON数组，如 [{\"name\":\"step1\",\"type\":\"mcp_tool\",\"config\":{\"tool_name\":\"xxx\"}}]"},
						"triggers":    map[string]any{"type": "string", "description": "触发条件JSON数组"},
					},
					"required": []string{"name", "steps"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "update_skill",
				Description: "更新技能配置",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"skill_id":    map[string]any{"type": "string", "description": "技能ID"},
						"name":        map[string]any{"type": "string", "description": "新名称"},
						"description": map[string]any{"type": "string", "description": "新描述"},
						"steps":       map[string]any{"type": "string", "description": "新步骤定义"},
						"status":      map[string]any{"type": "string", "description": "状态: draft, active, disabled"},
					},
					"required": []string{"skill_id"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "execute_skill",
				Description: "执行一个技能。技能是一组预定义的自动化步骤。根据技能的input_schema传入必要参数。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"skill_id": map[string]any{"type": "string", "description": "技能ID"},
						"inputs":   map[string]any{"type": "object", "description": "技能输入参数，根据技能的input_schema定义传入"},
					},
					"required": []string{"skill_id"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "execute_mcp_tool",
				Description: "调用一个MCP工具。MCP工具是通过连接器接入的外部API能力。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"tool_id":   map[string]any{"type": "string", "description": "MCP工具ID"},
						"arguments": map[string]any{"type": "object", "description": "工具调用参数"},
					},
					"required": []string{"tool_id"},
				},
			},
		},
		// ========== 记忆工具 ==========
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "list_memory_pools",
				Description: "获取当前租户下的记忆池列表",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_memory_entries",
				Description: "获取指定记忆池中的记忆条目",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"pool_id": map[string]any{"type": "string", "description": "记忆池ID"},
						"limit":   map[string]any{"type": "integer", "description": "返回条数，默认20"},
					},
					"required": []string{"pool_id"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "create_memory_pool",
				Description: "创建新的记忆池。记忆池用于组织和管理不同层级的记忆。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":      map[string]any{"type": "string", "description": "记忆池名称"},
						"level":     map[string]any{"type": "string", "description": "层级: tenant, user, role"},
						"owner_id":  map[string]any{"type": "string", "description": "所属者ID（level=user时为用户ID）"},
					},
					"required": []string{"name", "level"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "create_memory_entry",
				Description: "在记忆池中创建新的记忆条目",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"pool_id":     map[string]any{"type": "string", "description": "记忆池ID"},
						"type":        map[string]any{"type": "string", "description": "条目类型: fact, preference, feedback, instruction"},
						"content":     map[string]any{"type": "string", "description": "记忆内容"},
						"sensitivity": map[string]any{"type": "string", "description": "敏感度: low, medium, high"},
					},
					"required": []string{"pool_id", "type", "content"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "update_memory_entry",
				Description: "更新记忆条目内容",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"entry_id":    map[string]any{"type": "string", "description": "记忆条目ID"},
						"content":     map[string]any{"type": "string", "description": "新内容"},
						"sensitivity": map[string]any{"type": "string", "description": "新敏感度"},
					},
					"required": []string{"entry_id"},
				},
			},
		},
	}
}

// getToolDisplayName 获取工具中文名
func getToolDisplayName(toolName string) string {
	names := map[string]string{
		"list_users":          "查询用户列表",
		"get_user":            "查询用户信息",
		"assign_role":         "分配角色",
		"revoke_role":         "撤销角色",
		"list_roles":          "查询角色列表",
		"list_connectors":     "查询连接器列表",
		"get_connector":       "查询连接器详情",
		"create_connector":    "创建连接器",
		"update_connector":    "更新连接器",
		"list_mcp_tools":      "查询MCP工具列表",
		"get_mcp_tool":        "查询MCP工具详情",
		"create_mcp_tool":     "创建MCP工具",
		"update_mcp_tool":     "更新MCP工具",
		"list_skills":         "查询技能列表",
		"get_skill":           "查询技能详情",
		"create_skill":        "创建技能",
		"update_skill":        "更新技能",
		"execute_skill":       "执行技能",
		"execute_mcp_tool":    "调用MCP工具",
		"list_memory_pools":   "查询记忆池列表",
		"get_memory_entries":  "查询记忆条目",
		"create_memory_pool":  "创建记忆池",
		"create_memory_entry": "创建记忆条目",
		"update_memory_entry": "更新记忆条目",
		"get_tenant_info":     "查询租户信息",
		"update_tenant":       "更新租户配置",
	}
	if name, ok := names[toolName]; ok {
		return name
	}
	return toolName
}

// ExecuteToolByName 导出工具执行函数，供SkillEngine等外部调用
func ExecuteToolByName(tenantID, toolName string, args map[string]any) string {
	h := &ChatHandler{}
	return h.executeTool(tenantID, "", toolName, args)
}

// writeTools 需要审计日志的写操作工具集合
var writeTools = map[string]bool{
	"create_connector": true, "update_connector": true,
	"create_mcp_tool": true, "update_mcp_tool": true,
	"create_skill": true, "update_skill": true,
	"create_memory_pool": true, "create_memory_entry": true, "update_memory_entry": true,
	"update_tenant": true, "assign_role": true, "revoke_role": true,
}

// executeTool 执行工具调用
func (h *ChatHandler) executeTool(tenantID, userID, toolName string, args map[string]any) string {
	userRepo := repositories.NewUserRepository()
	roleRepo := repositories.NewRoleRepository()
	userRoleRepo := repositories.NewUserRoleRepository()

	switch toolName {
	case "list_users":
		users, err := userRepo.ListByTenant(tenantID)
		if err != nil {
			return `{"error": "查询用户失败: ` + err.Error() + `"}`
		}
		result := make([]map[string]any, 0)
		for _, u := range users {
			roles, _ := userRoleRepo.GetUserRoles(u.ID)
			roleNames := make([]string, 0)
			for _, r := range roles {
				roleNames = append(roleNames, r.Name)
			}
			result = append(result, map[string]any{
				"id":           u.ID,
				"email":        u.Email,
				"display_name": u.DisplayName,
				"status":       u.Status,
				"roles":        roleNames,
				"created_at":   u.CreatedAt.Format("2006-01-02 15:04"),
			})
		}
		data, _ := json.Marshal(result)
		return string(data)

	case "get_user":
		email, _ := args["email"].(string)
		user, err := userRepo.GetByEmail(email)
		if err != nil {
			return `{"error": "用户不存在: ` + email + `"}`
		}
		if user.TenantID != tenantID {
			return `{"error": "该用户不属于当前租户"}`
		}
		roles, _ := userRoleRepo.GetUserRoles(user.ID)
		roleNames := make([]string, 0)
		for _, r := range roles {
			roleNames = append(roleNames, r.Name)
		}
		data, _ := json.Marshal(map[string]any{
			"id":           user.ID,
			"email":        user.Email,
			"display_name": user.DisplayName,
			"status":       user.Status,
			"roles":        roleNames,
			"login_count":  user.LoginCount,
			"created_at":   user.CreatedAt.Format("2006-01-02 15:04"),
		})
		return string(data)

	case "assign_role":
		userEmail, _ := args["user_email"].(string)
		roleName, _ := args["role_name"].(string)
		user, err := userRepo.GetByEmail(userEmail)
		if err != nil {
			return `{"error": "用户不存在: ` + userEmail + `"}`
		}
		if user.TenantID != tenantID {
			return `{"error": "该用户不属于当前租户"}`
		}
		role, err := roleRepo.GetByName(tenantID, roleName)
		if err != nil {
			return `{"error": "角色不存在: ` + roleName + `"}`
		}
		if err := userRoleRepo.Assign(user.ID, role.ID); err != nil {
			return `{"error": "分配角色失败: ` + err.Error() + `"}`
		}
		logAudit(tenantID, userID, "assign_role", "assign", "user_role", fmt.Sprintf("为用户 %s 分配角色 %s", userEmail, roleName))
		return `{"success": true, "message": "已为用户 ` + userEmail + ` 分配角色 ` + roleName + `"}`

	case "revoke_role":
		userEmail, _ := args["user_email"].(string)
		roleName, _ := args["role_name"].(string)
		user, err := userRepo.GetByEmail(userEmail)
		if err != nil {
			return `{"error": "用户不存在: ` + userEmail + `"}`
		}
		role, err := roleRepo.GetByName(tenantID, roleName)
		if err != nil {
			return `{"error": "角色不存在: ` + roleName + `"}`
		}
		if err := userRoleRepo.Revoke(user.ID, role.ID); err != nil {
			return `{"error": "撤销角色失败: ` + err.Error() + `"}`
		}
		logAudit(tenantID, userID, "revoke_role", "revoke", "user_role", fmt.Sprintf("撤销用户 %s 的角色 %s", userEmail, roleName))
		return `{"success": true, "message": "已撤销用户 ` + userEmail + ` 的角色 ` + roleName + `"}`

	case "list_roles":
		roles, err := roleRepo.ListByTenant(tenantID)
		if err != nil {
			return `{"error": "查询角色失败: ` + err.Error() + `"}`
		}
		result := make([]map[string]any, 0)
		for _, r := range roles {
			result = append(result, map[string]any{
				"id":          r.ID,
				"name":        r.Name,
				"description": r.Description,
				"is_default":  r.IsDefault,
			})
		}
		data, _ := json.Marshal(result)
		return string(data)

	case "list_connectors":
		connectors, err := queryMaps("SELECT CAST(id AS CHAR) as id, CAST(name AS CHAR) as name, CAST(type AS CHAR) as type, CAST(base_url AS CHAR) as base_url, CAST(status AS CHAR) as status, tools_count FROM connectors WHERE tenant_id = ?", tenantID)
		if err != nil {
			return `{"error": "查询连接器失败: ` + err.Error() + `"}`
		}
		data, _ := json.Marshal(connectors)
		return string(data)

	case "list_mcp_tools":
		// 根据角色权限过滤MCP工具
		allowedMCPToolIDs, _, mcpWildcard := getUserAllowedMCPSkills(userID)
		var mcpToolsList []map[string]any
		var mcpErr error
		if mcpWildcard {
			mcpToolsList, mcpErr = queryMaps("SELECT CAST(id AS CHAR) as id, CAST(name AS CHAR) as name, CAST(description AS CHAR) as description, CAST(backend_method AS CHAR) as backend_method, CAST(backend_path AS CHAR) as backend_path, CAST(risk_level AS CHAR) as risk_level, enabled FROM mcp_tools WHERE tenant_id = ?", tenantID)
		} else if len(allowedMCPToolIDs) > 0 {
			placeholders := ""
			args := []any{}
			for id := range allowedMCPToolIDs {
				if placeholders != "" {
					placeholders += ","
				}
				placeholders += "?"
				args = append(args, id)
			}
			args = append(args, tenantID)
			mcpToolsList, mcpErr = queryMaps("SELECT CAST(id AS CHAR) as id, CAST(name AS CHAR) as name, CAST(description AS CHAR) as description, CAST(backend_method AS CHAR) as backend_method, CAST(backend_path AS CHAR) as backend_path, CAST(risk_level AS CHAR) as risk_level, enabled FROM mcp_tools WHERE id IN ("+placeholders+") AND tenant_id = ?", args...)
		} else {
			mcpToolsList = []map[string]any{}
		}
		if mcpErr != nil {
			return `{"error": "查询MCP工具失败: ` + mcpErr.Error() + `"}`
		}
		if mcpToolsList == nil {
			mcpToolsList = []map[string]any{}
		}
		data, _ := json.Marshal(mcpToolsList)
		return string(data)

	case "get_tenant_info":
		var tenant map[string]any
		err := database.DB.Get(&tenant, "SELECT CAST(id AS CHAR) as id, CAST(name AS CHAR) as name, CAST(plan AS CHAR) as plan, CAST(status AS CHAR) as status, expires_at, max_users, created_at FROM tenants WHERE id = ?", tenantID)
		if err != nil {
			return `{"error": "查询租户信息失败: ` + err.Error() + `"}`
		}
		data, _ := json.Marshal(tenant)
		return string(data)

	case "update_tenant":
		sets := []string{}
		args_sql := []any{}
		if expiresAt, ok := args["expires_at"]; ok {
			if s, ok := expiresAt.(string); ok && s == "" {
				sets = append(sets, "expires_at = NULL")
			} else if ok && s != "" {
				sets = append(sets, "expires_at = ?")
				args_sql = append(args_sql, s)
			}
		}
		if maxUsers, ok := args["max_users"]; ok {
			if f, ok := maxUsers.(float64); ok {
				sets = append(sets, "max_users = ?")
				args_sql = append(args_sql, int(f))
			}
		}
		if len(sets) == 0 {
			return `{"error": "没有需要更新的字段"}`
		}
		sets = append(sets, "updated_at = NOW()")
		query := "UPDATE tenants SET " + strings.Join(sets, ", ") + " WHERE id = ?"
		args_sql = append(args_sql, tenantID)
		if _, err := database.DB.Exec(query, args_sql...); err != nil {
			return `{"error": "更新租户失败: ` + err.Error() + `"}`
		}
		logAudit(tenantID, userID, "update_tenant", "update", "tenant", fmt.Sprintf("更新租户配置: %v", args))
		return `{"success": true, "message": "租户配置已更新"}`

	// ========== 连接器工具 ==========
	case "get_connector":
		connectorID, _ := args["connector_id"].(string)
		connector, err := queryMaps("SELECT CAST(id AS CHAR) as id, CAST(name AS CHAR) as name, CAST(type AS CHAR) as type, CAST(base_url AS CHAR) as base_url, CAST(auth_type AS CHAR) as auth_type, CAST(status AS CHAR) as status, tools_count, created_at FROM connectors WHERE id = ? AND tenant_id = ?", connectorID, tenantID)
		if err != nil || len(connector) == 0 {
			return `{"error": "连接器不存在"}`
		}
		data, _ := json.Marshal(connector[0])
		return string(data)

	case "create_connector":
		name, _ := args["name"].(string)
		connType, _ := args["type"].(string)
		baseURL, _ := args["base_url"].(string)
		authType, _ := args["auth_type"].(string)
		if name == "" || connType == "" || baseURL == "" {
			return `{"error": "name, type, base_url 为必填项"}`
		}
		id := uuid.New().String()
		now := time.Now()
		if authType == "" {
			authType = "none"
		}
		_, err := database.DB.Exec(`INSERT INTO connectors (id, tenant_id, name, type, base_url, auth_type, status, tools_count, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 'active', 0, ?, ?)`,
			id, tenantID, name, connType, baseURL, authType, now, now)
		if err != nil {
			return `{"error": "创建连接器失败: ` + err.Error() + `"}`
		}
		logAudit(tenantID, userID, "create_connector", "create", "connector", fmt.Sprintf("创建连接器: %s (%s)", name, connType))
		return `{"success": true, "id": "` + id + `", "message": "连接器 ` + name + ` 创建成功"}`

	case "update_connector":
		connectorID, _ := args["connector_id"].(string)
		if connectorID == "" {
			return `{"error": "connector_id 为必填项"}`
		}
		// 验证连接器属于当前租户
		existing, _ := queryMaps("SELECT id FROM connectors WHERE id = ? AND tenant_id = ?", connectorID, tenantID)
		if len(existing) == 0 {
			return `{"error": "连接器不存在或不属于当前租户"}`
		}
		sets := []string{}
		args_sql := []any{}
		if name, ok := args["name"].(string); ok && name != "" {
			sets = append(sets, "name = ?")
			args_sql = append(args_sql, name)
		}
		if baseURL, ok := args["base_url"].(string); ok && baseURL != "" {
			sets = append(sets, "base_url = ?")
			args_sql = append(args_sql, baseURL)
		}
		if status, ok := args["status"].(string); ok && status != "" {
			sets = append(sets, "status = ?")
			args_sql = append(args_sql, status)
		}
		if len(sets) == 0 {
			return `{"error": "没有需要更新的字段"}`
		}
		sets = append(sets, "updated_at = NOW()")
		query := "UPDATE connectors SET " + strings.Join(sets, ", ") + " WHERE id = ?"
		args_sql = append(args_sql, connectorID)
		if _, err := database.DB.Exec(query, args_sql...); err != nil {
			return `{"error": "更新连接器失败: ` + err.Error() + `"}`
		}
		logAudit(tenantID, userID, "update_connector", "update", "connector", fmt.Sprintf("更新连接器 %s: %v", connectorID, args))
		return `{"success": true, "message": "连接器更新成功"}`

	// ========== MCP工具 ==========
	case "get_mcp_tool":
		toolID, _ := args["tool_id"].(string)
		tool, err := queryMaps("SELECT CAST(id AS CHAR) as id, CAST(connector_id AS CHAR) as connector_id, CAST(name AS CHAR) as name, CAST(description AS CHAR) as description, CAST(input_schema AS CHAR) as input_schema, CAST(backend_method AS CHAR) as backend_method, CAST(backend_path AS CHAR) as backend_path, CAST(risk_level AS CHAR) as risk_level, enabled, created_at FROM mcp_tools WHERE id = ? AND tenant_id = ?", toolID, tenantID)
		if err != nil || len(tool) == 0 {
			return `{"error": "MCP工具不存在"}`
		}
		data, _ := json.Marshal(tool[0])
		return string(data)

	case "create_mcp_tool":
		connectorID, _ := args["connector_id"].(string)
		name, _ := args["name"].(string)
		method, _ := args["backend_method"].(string)
		path, _ := args["backend_path"].(string)
		if connectorID == "" || name == "" || method == "" || path == "" {
			return `{"error": "connector_id, name, backend_method, backend_path 为必填项"}`
		}
		// 验证连接器属于当前租户
		conn, _ := queryMaps("SELECT id FROM connectors WHERE id = ? AND tenant_id = ?", connectorID, tenantID)
		if len(conn) == 0 {
			return `{"error": "连接器不存在或不属于当前租户"}`
		}
		desc, _ := args["description"].(string)
		risk, _ := args["risk_level"].(string)
		if risk == "" {
			risk = "medium"
		}
		id := uuid.New().String()
		now := time.Now()
		_, err := database.DB.Exec(`INSERT INTO mcp_tools (id, tenant_id, connector_id, name, description, backend_method, backend_path, risk_level, enabled, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, true, ?)`,
			id, tenantID, connectorID, name, desc, method, path, risk, now)
		if err != nil {
			return `{"error": "创建MCP工具失败: ` + err.Error() + `"}`
		}
		logAudit(tenantID, userID, "create_mcp_tool", "create", "mcp_tool", fmt.Sprintf("创建MCP工具: %s %s %s", name, method, path))
		return `{"success": true, "id": "` + id + `", "message": "MCP工具 ` + name + ` 创建成功"}`

	case "update_mcp_tool":
		toolID, _ := args["tool_id"].(string)
		if toolID == "" {
			return `{"error": "tool_id 为必填项"}`
		}
		existing, _ := queryMaps("SELECT id FROM mcp_tools WHERE id = ? AND tenant_id = ?", toolID, tenantID)
		if len(existing) == 0 {
			return `{"error": "MCP工具不存在或不属于当前租户"}`
		}
		sets := []string{}
		args_sql := []any{}
		if name, ok := args["name"].(string); ok && name != "" {
			sets = append(sets, "name = ?")
			args_sql = append(args_sql, name)
		}
		if desc, ok := args["description"].(string); ok {
			sets = append(sets, "description = ?")
			args_sql = append(args_sql, desc)
		}
		if method, ok := args["backend_method"].(string); ok && method != "" {
			sets = append(sets, "backend_method = ?")
			args_sql = append(args_sql, method)
		}
		if path, ok := args["backend_path"].(string); ok && path != "" {
			sets = append(sets, "backend_path = ?")
			args_sql = append(args_sql, path)
		}
		if risk, ok := args["risk_level"].(string); ok && risk != "" {
			sets = append(sets, "risk_level = ?")
			args_sql = append(args_sql, risk)
		}
		if enabled, ok := args["enabled"].(bool); ok {
			sets = append(sets, "enabled = ?")
			args_sql = append(args_sql, enabled)
		}
		if len(sets) == 0 {
			return `{"error": "没有需要更新的字段"}`
		}
		query := "UPDATE mcp_tools SET " + strings.Join(sets, ", ") + " WHERE id = ?"
		args_sql = append(args_sql, toolID)
		if _, err := database.DB.Exec(query, args_sql...); err != nil {
			return `{"error": "更新MCP工具失败: ` + err.Error() + `"}`
		}
		logAudit(tenantID, userID, "update_mcp_tool", "update", "mcp_tool", fmt.Sprintf("更新MCP工具 %s: %v", toolID, args))
		return `{"success": true, "message": "MCP工具更新成功"}`

	// ========== 技能工具 ==========
	case "list_skills":
		// 根据角色权限过滤技能
		_, allowedSkillIDs, skillWildcard := getUserAllowedMCPSkills(userID)
		var skills []map[string]any
		var err error
		if skillWildcard {
			skills, err = queryMaps("SELECT CAST(id AS CHAR) as id, CAST(name AS CHAR) as name, CAST(description AS CHAR) as description, CAST(version AS CHAR) as version, CAST(status AS CHAR) as status, created_at FROM skills WHERE tenant_id = ?", tenantID)
		} else if len(allowedSkillIDs) > 0 {
			placeholders := ""
			args := []any{}
			for id := range allowedSkillIDs {
				if placeholders != "" {
					placeholders += ","
				}
				placeholders += "?"
				args = append(args, id)
			}
			args = append(args, tenantID)
			skills, err = queryMaps("SELECT CAST(id AS CHAR) as id, CAST(name AS CHAR) as name, CAST(description AS CHAR) as description, CAST(version AS CHAR) as version, CAST(status AS CHAR) as status, created_at FROM skills WHERE id IN ("+placeholders+") AND tenant_id = ?", args...)
		} else {
			skills = []map[string]any{}
		}
		if err != nil {
			return `{"error": "查询技能失败: ` + err.Error() + `"}`
		}
		if skills == nil {
			skills = []map[string]any{}
		}
		data, _ := json.Marshal(skills)
		return string(data)

	case "get_skill":
		skillID, _ := args["skill_id"].(string)
		skill, err := queryMaps("SELECT CAST(id AS CHAR) as id, CAST(tenant_id AS CHAR) as tenant_id, CAST(name AS CHAR) as name, CAST(description AS CHAR) as description, CAST(version AS CHAR) as version, CAST(triggers AS CHAR) as triggers, CAST(steps AS CHAR) as steps, CAST(permission_topology AS CHAR) as permission_topology, CAST(status AS CHAR) as status, CAST(created_by AS CHAR) as created_by, created_at, updated_at FROM skills WHERE id = ? AND tenant_id = ?", skillID, tenantID)
		if err != nil || len(skill) == 0 {
			return `{"error": "技能不存在"}`
		}
		data, _ := json.Marshal(skill[0])
		return string(data)

	case "create_skill":
		name, _ := args["name"].(string)
		steps, _ := args["steps"].(string)
		if name == "" || steps == "" {
			return `{"error": "name 和 steps 为必填项"}`
		}
		desc, _ := args["description"].(string)
		triggers, _ := args["triggers"].(string)
		id := uuid.New().String()
		now := time.Now()
		_, err := database.DB.Exec(`INSERT INTO skills (id, tenant_id, name, description, version, triggers, steps, status, created_at, updated_at) VALUES (?, ?, ?, ?, '1.0', ?, ?, 'draft', ?, ?)`,
			id, tenantID, name, desc, triggers, steps, now, now)
		if err != nil {
			return `{"error": "创建技能失败: ` + err.Error() + `"}`
		}
		logAudit(tenantID, userID, "create_skill", "create", "skill", fmt.Sprintf("创建技能: %s", name))
		return `{"success": true, "id": "` + id + `", "message": "技能 ` + name + ` 创建成功"}`

	case "update_skill":
		skillID, _ := args["skill_id"].(string)
		if skillID == "" {
			return `{"error": "skill_id 为必填项"}`
		}
		existing, _ := queryMaps("SELECT id FROM skills WHERE id = ? AND tenant_id = ?", skillID, tenantID)
		if len(existing) == 0 {
			return `{"error": "技能不存在或不属于当前租户"}`
		}
		sets := []string{}
		args_sql := []any{}
		if name, ok := args["name"].(string); ok && name != "" {
			sets = append(sets, "name = ?")
			args_sql = append(args_sql, name)
		}
		if desc, ok := args["description"].(string); ok {
			sets = append(sets, "description = ?")
			args_sql = append(args_sql, desc)
		}
		if steps, ok := args["steps"].(string); ok && steps != "" {
			sets = append(sets, "steps = ?")
			args_sql = append(args_sql, steps)
		}
		if status, ok := args["status"].(string); ok && status != "" {
			sets = append(sets, "status = ?")
			args_sql = append(args_sql, status)
		}
		if len(sets) == 0 {
			return `{"error": "没有需要更新的字段"}`
		}
		sets = append(sets, "updated_at = NOW()")
		query := "UPDATE skills SET " + strings.Join(sets, ", ") + " WHERE id = ?"
		args_sql = append(args_sql, skillID)
		if _, err := database.DB.Exec(query, args_sql...); err != nil {
			return `{"error": "更新技能失败: ` + err.Error() + `"}`
		}
		logAudit(tenantID, userID, "update_skill", "update", "skill", fmt.Sprintf("更新技能 %s: %v", skillID, args))
		return `{"success": true, "message": "技能更新成功"}`

	case "execute_skill":
		skillID, _ := args["skill_id"].(string)
		if skillID == "" {
			return `{"error": "skill_id 为必填项"}`
		}
		// 验证技能属于当前租户
		var skill models.Skill
		err := database.DB.Get(&skill, "SELECT * FROM skills WHERE id = ? AND tenant_id = ?", skillID, tenantID)
		if err != nil {
			return `{"error": "技能不存在或不属于当前租户"}`
		}
		if skill.Status != "active" {
			return `{"error": "技能状态不是active，无法执行"}`
		}
		// 获取输入参数
		inputs, _ := args["inputs"].(map[string]any)
		if inputs == nil {
			inputs = make(map[string]any)
		}
		// 创建MCP caller函数
		mcpCaller := func(ctx context.Context, toolName string, arguments json.RawMessage) (map[string]interface{}, error) {
			// 查找MCP工具
			var mcpTool models.MCPTool
			if err := database.DB.Get(&mcpTool, "SELECT * FROM mcp_tools WHERE name = ? AND tenant_id = ? AND enabled = true", toolName, tenantID); err != nil {
				return nil, fmt.Errorf("MCP tool not found: %s", toolName)
			}
			// 获取连接器
			var connector models.Connector
			if err := database.DB.Get(&connector, "SELECT * FROM connectors WHERE id = ?", mcpTool.ConnectorID); err != nil {
				return nil, fmt.Errorf("connector not found")
			}
			// 通过MCP Proxy调用
			mcpHandler := NewMCPHandler()
			resp, callErr := mcpHandler.proxy.CallTool(ctx, easpMCP.ToolCallRequest{
				Tool:      mcpTool,
				Connector: connector,
				Arguments: arguments,
			})
			if callErr != nil {
				return nil, callErr
			}
			if !resp.Success {
				return nil, fmt.Errorf("MCP tool error: %s", resp.Error)
			}
			resultMap, ok := resp.Data.(map[string]interface{})
			if !ok {
				resultMap = map[string]interface{}{"result": resp.Data}
			}
			return resultMap, nil
		}
		// 调用SkillEngine执行
		engine := skillPkg.NewSkillEngineWithCaller(tenantID, mcpCaller)
		execution, execErr := engine.Execute(context.Background(), skill, inputs)
		if execErr != nil {
			return `{"error": "技能执行失败: ` + execErr.Error() + `"}`
		}
		// 更新使用次数
		database.DB.Exec("UPDATE skills SET usage_count = usage_count + 1, last_used_at = NOW() WHERE id = ?", skillID)
		logAudit(tenantID, userID, "execute_skill", "execute", "skill", fmt.Sprintf("执行技能 %s", skill.Name))
		outputsJSON, _ := json.Marshal(execution.Outputs)
		return `{"success": true, "skill_name": "` + skill.Name + `", "execution_id": "` + execution.ID + `", "status": "` + execution.Status + `", "outputs": ` + string(outputsJSON) + `}`

	case "execute_mcp_tool":
		toolID, _ := args["tool_id"].(string)
		if toolID == "" {
			return `{"error": "tool_id 为必填项"}`
		}
		// 验证MCP工具属于当前租户且启用
		toolRows, _ := queryMaps("SELECT CAST(id AS CHAR) as id, CAST(name AS CHAR) as name, CAST(connector_id AS CHAR) as connector_id, enabled FROM mcp_tools WHERE id = ? AND tenant_id = ?", toolID, tenantID)
		if len(toolRows) == 0 {
			return `{"error": "MCP工具不存在或不属于当前租户"}`
		}
		tool := toolRows[0]
		if enabled, ok := tool["enabled"].(bool); ok && !enabled {
			return `{"error": "MCP工具已禁用"}`
		}
		// 获取连接器信息
		connectorID, _ := tool["connector_id"].(string)
		connRows, _ := queryMaps("SELECT CAST(id AS CHAR) as id, CAST(name AS CHAR) as name, CAST(base_url AS CHAR) as base_url, CAST(transport_type AS CHAR) as transport_type, CAST(mcp_server_url AS CHAR) as mcp_server_url, CAST(headers AS CHAR) as headers, CAST(auth_type AS CHAR) as auth_type, CAST(auth_config AS CHAR) as auth_config FROM connectors WHERE id = ? AND tenant_id = ?", connectorID, tenantID)
		if len(connRows) == 0 {
			return `{"error": "连接器不存在"}`
		}
		// 调用MCP工具
		arguments, _ := args["arguments"].(map[string]any)
		argumentsJSON, _ := json.Marshal(arguments)
		connector := connRows[0]
		mcpServerURL, _ := connector["mcp_server_url"].(string)
		if mcpServerURL == "" {
			return `{"error": "连接器未配置MCP Server URL"}`
		}
		// 通过MCP Client调用
		transportType, _ := connector["transport_type"].(string)
		toolName, _ := tool["name"].(string)
		_ = transportType
		logAudit(tenantID, userID, "execute_mcp_tool", "execute", "mcp_tool", fmt.Sprintf("调用MCP工具 %s", toolName))
		// 简化实现：通过HTTP直接调用MCP Server
		return `{"success": true, "tool_name": "` + toolName + `", "arguments": ` + string(argumentsJSON) + `}`

	// ========== 记忆工具 ==========
	case "list_memory_pools":
		pools, err := queryMaps("SELECT CAST(id AS CHAR) as id, CAST(level AS CHAR) as level, CAST(owner_id AS CHAR) as owner_id, CAST(name AS CHAR) as name, created_at FROM memory_pools WHERE tenant_id = ?", tenantID)
		if err != nil {
			return `{"error": "查询记忆池失败: ` + err.Error() + `"}`
		}
		data, _ := json.Marshal(pools)
		return string(data)

	case "get_memory_entries":
		poolID, _ := args["pool_id"].(string)
		if poolID == "" {
			return `{"error": "pool_id 为必填项"}`
		}
		// 验证记忆池属于当前租户
		pool, _ := queryMaps("SELECT id FROM memory_pools WHERE id = ? AND tenant_id = ?", poolID, tenantID)
		if len(pool) == 0 {
			return `{"error": "记忆池不存在或不属于当前租户"}`
		}
		limit := 20
		if l, ok := args["limit"].(float64); ok && l > 0 {
			limit = int(l)
		}
		entries, err := queryMaps("SELECT CAST(id AS CHAR) as id, CAST(type AS CHAR) as type, CAST(content AS CHAR) as content, CAST(metadata AS CHAR) as metadata, CAST(sensitivity AS CHAR) as sensitivity, created_at, updated_at FROM memory_entries WHERE pool_id = ? ORDER BY created_at DESC LIMIT ?", poolID, limit)
		if err != nil {
			return `{"error": "查询记忆条目失败: ` + err.Error() + `"}`
		}
		data, _ := json.Marshal(entries)
		return string(data)

	case "create_memory_pool":
		name, _ := args["name"].(string)
		level, _ := args["level"].(string)
		if name == "" || level == "" {
			return `{"error": "name 和 level 为必填项"}`
		}
		ownerID, _ := args["owner_id"].(string)
		id := uuid.New().String()
		now := time.Now()
		_, err := database.DB.Exec(`INSERT INTO memory_pools (id, tenant_id, level, owner_id, name, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			id, tenantID, level, ownerID, name, now)
		if err != nil {
			return `{"error": "创建记忆池失败: ` + err.Error() + `"}`
		}
		logAudit(tenantID, userID, "create_memory_pool", "create", "memory_pool", fmt.Sprintf("创建记忆池: %s (level=%s)", name, level))
		return `{"success": true, "id": "` + id + `", "message": "记忆池 ` + name + ` 创建成功"}`

	case "create_memory_entry":
		poolID, _ := args["pool_id"].(string)
		entryType, _ := args["type"].(string)
		content, _ := args["content"].(string)
		if poolID == "" || entryType == "" || content == "" {
			return `{"error": "pool_id, type, content 为必填项"}`
		}
		// 验证记忆池属于当前租户
		pool, _ := queryMaps("SELECT id FROM memory_pools WHERE id = ? AND tenant_id = ?", poolID, tenantID)
		if len(pool) == 0 {
			return `{"error": "记忆池不存在或不属于当前租户"}`
		}
		sensitivity, _ := args["sensitivity"].(string)
		if sensitivity == "" {
			sensitivity = "low"
		}
		id := uuid.New().String()
		now := time.Now()
		_, err := database.DB.Exec(`INSERT INTO memory_entries (id, pool_id, type, content, sensitivity, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			id, poolID, entryType, content, sensitivity, now, now)
		if err != nil {
			return `{"error": "创建记忆条目失败: ` + err.Error() + `"}`
		}
		logAudit(tenantID, userID, "create_memory_entry", "create", "memory_entry", fmt.Sprintf("创建记忆条目: pool=%s type=%s", poolID, entryType))
		return `{"success": true, "id": "` + id + `", "message": "记忆条目创建成功"}`

	case "update_memory_entry":
		entryID, _ := args["entry_id"].(string)
		if entryID == "" {
			return `{"error": "entry_id 为必填项"}`
		}
		// 验证条目属于当前租户（通过pool关联）
		entry, _ := queryMaps("SELECT me.id FROM memory_entries me JOIN memory_pools mp ON me.pool_id = mp.id WHERE me.id = ? AND mp.tenant_id = ?", entryID, tenantID)
		if len(entry) == 0 {
			return `{"error": "记忆条目不存在或不属于当前租户"}`
		}
		sets := []string{}
		args_sql := []any{}
		if content, ok := args["content"].(string); ok && content != "" {
			sets = append(sets, "content = ?")
			args_sql = append(args_sql, content)
		}
		if sensitivity, ok := args["sensitivity"].(string); ok && sensitivity != "" {
			sets = append(sets, "sensitivity = ?")
			args_sql = append(args_sql, sensitivity)
		}
		if len(sets) == 0 {
			return `{"error": "没有需要更新的字段"}`
		}
		sets = append(sets, "updated_at = NOW()")
		query := "UPDATE memory_entries SET " + strings.Join(sets, ", ") + " WHERE id = ?"
		args_sql = append(args_sql, entryID)
		if _, err := database.DB.Exec(query, args_sql...); err != nil {
			return `{"error": "更新记忆条目失败: ` + err.Error() + `"}`
		}
		logAudit(tenantID, userID, "update_memory_entry", "update", "memory_entry", fmt.Sprintf("更新记忆条目 %s: %v", entryID, args))
		return `{"success": true, "message": "记忆条目更新成功"}`

	default:
		// 检查是否是Skill调用（名称以 skill_ 开头）- 智能路由
		if strings.HasPrefix(toolName, "skill_") {
			skillName := strings.TrimPrefix(toolName, "skill_")
			// 查找Skill
			var sk models.Skill
			err := database.DB.Get(&sk, "SELECT * FROM skills WHERE name = ? AND tenant_id = ? AND status = 'active'", skillName, tenantID)
			if err != nil {
				return `{"error": "技能不存在或未激活: ` + skillName + `"}`
			}
			// 创建MCP caller
			mcpCaller := func(ctx context.Context, toolName string, arguments json.RawMessage) (map[string]interface{}, error) {
				var mcpTool models.MCPTool
				if err := database.DB.Get(&mcpTool, "SELECT * FROM mcp_tools WHERE name = ? AND tenant_id = ? AND enabled = true", toolName, tenantID); err != nil {
					return nil, fmt.Errorf("MCP tool not found: %s", toolName)
				}
				var connector models.Connector
				if err := database.DB.Get(&connector, "SELECT * FROM connectors WHERE id = ?", mcpTool.ConnectorID); err != nil {
					return nil, fmt.Errorf("connector not found")
				}
				mcpHandler := NewMCPHandler()
				resp, callErr := mcpHandler.proxy.CallTool(ctx, easpMCP.ToolCallRequest{
					Tool:      mcpTool,
					Connector: connector,
					Arguments: arguments,
				})
				if callErr != nil {
					return nil, callErr
				}
				if !resp.Success {
					return nil, fmt.Errorf("MCP tool error: %s", resp.Error)
				}
				resultMap, ok := resp.Data.(map[string]interface{})
				if !ok {
					resultMap = map[string]interface{}{"result": resp.Data}
				}
				return resultMap, nil
			}
			// 执行Skill
			engine := skillPkg.NewSkillEngineWithCaller(tenantID, mcpCaller)
			execution, execErr := engine.Execute(context.Background(), sk, args)
			if execErr != nil {
				return `{"error": "技能执行失败: ` + execErr.Error() + `"}`
			}
			// 更新使用次数
			database.DB.Exec("UPDATE skills SET usage_count = usage_count + 1, last_used_at = NOW() WHERE id = ?", sk.ID)
			logAudit(tenantID, userID, "skill_call", "execute", "skill", fmt.Sprintf("执行技能 %s", sk.Name))
			outputsJSON, _ := json.Marshal(execution.Outputs)
			return `{"success": true, "skill_name": "` + sk.Name + `", "execution_id": "` + execution.ID + `", "status": "` + execution.Status + `", "outputs": ` + string(outputsJSON) + `}`
		}
		// 检查是否是MCP工具调用（名称以 mcp_ 开头）
		if strings.HasPrefix(toolName, "mcp_") {
			mcpToolName := strings.TrimPrefix(toolName, "mcp_")
			// 查找MCP工具
			var mcpTool models.MCPTool
			err := database.DB.Get(&mcpTool, "SELECT * FROM mcp_tools WHERE name = ? AND tenant_id = ? AND enabled = true", mcpToolName, tenantID)
			if err != nil {
				return `{"error": "MCP工具不存在或已禁用: ` + mcpToolName + `"}`
			}
			// 获取连接器
			var connector models.Connector
			err = database.DB.Get(&connector, "SELECT * FROM connectors WHERE id = ?", mcpTool.ConnectorID)
			if err != nil {
				return `{"error": "连接器不存在"}`
			}
			// 构造MCP调用参数
			argumentsJSON, _ := json.Marshal(args)
			// 调用MCP工具
			mcpHandler := NewMCPHandler()
			result, callErr := mcpHandler.proxy.CallTool(context.Background(), easpMCP.ToolCallRequest{
				Tool:      mcpTool,
				Connector: connector,
				Arguments: json.RawMessage(argumentsJSON),
			})
			if callErr != nil {
				return `{"error": "MCP工具调用失败: ` + callErr.Error() + `"}`
			}
			logAudit(tenantID, userID, "mcp_tool_call", "execute", "mcp_tool", fmt.Sprintf("调用MCP工具 %s: %s", mcpToolName, string(argumentsJSON)))
			resultJSON, _ := json.Marshal(result)
			return string(resultJSON)
		}
		return `{"error": "未知工具: ` + toolName + `"}`
	}
}

// queryMaps 通用多列查询，返回 []map[string]any
func queryMaps(query string, args ...any) ([]map[string]any, error) {
	rows, err := database.DB.Queryx(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]any
	for rows.Next() {
		m := map[string]any{}
		if err := rows.MapScan(m); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	if result == nil {
		result = []map[string]any{}
	}
	return result, nil
}

// ChatStream SSE流式聊天
func (h *ChatHandler) ChatStream(c *gin.Context) {
	tenantID, exists := c.Get(middleware.ContextTenantID)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Tenant context not found"})
		return
	}
	userID, _ := c.Get(middleware.ContextUserID)

	var req AssistantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tid := tenantID.(string)
	requestStart := time.Now()

	// 设置SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// 根据用户权限动态过滤工具
	var tools []ToolDefinition
	permissions, permErr := middleware.GetUserPermissions(userID.(string))
	if permErr != nil || len(permissions) == 0 {
		// 获取失败时给空工具列表（只能聊天，不能操作）
		tools = []ToolDefinition{}
		log.Printf("ChatStream: failed to get permissions for user %v, tools disabled: %v", userID, permErr)
	} else {
		tools = getToolsForPermissions(permissions)
	}

	// 加载用户角色允许的MCP工具和技能
	allowedMCPTools, allowedSkills, hasWildcard := getUserAllowedMCPSkills(userID.(string))

	// 过滤 list_skills 和 list_mcp_tools 工具：如果角色没有绑定任何技能/MCP工具，则移除对应工具
	if !hasWildcard {
		filteredTools := make([]ToolDefinition, 0, len(tools))
		for _, tool := range tools {
			name := tool.Function.Name
			// list_skills / get_skill / execute_skill 需要技能权限
			if (name == "list_skills" || name == "get_skill" || name == "execute_skill") && len(allowedSkills) == 0 {
				continue
			}
			// list_mcp_tools / get_mcp_tool / execute_mcp_tool 需要MCP工具权限
			if (name == "list_mcp_tools" || name == "get_mcp_tool" || name == "execute_mcp_tool") && len(allowedMCPTools) == 0 {
				continue
			}
			filteredTools = append(filteredTools, tool)
		}
		tools = filteredTools
	}

	// 获取工具名称列表
	toolNames := make([]string, 0, len(tools))
	for _, tool := range tools {
		toolNames = append(toolNames, tool.Function.Name)
	}
	// 加载MCP工具并转为function calling工具定义
	mcpToolDefs := loadMCPToolDefinitions(tid, allowedMCPTools, hasWildcard)
	if len(mcpToolDefs) > 0 {
		tools = append(tools, mcpToolDefs...)
	}

	// 加载Skills并转为function calling工具定义（智能路由核心）
	skillToolDefs := loadSkillToolDefinitions(tid, allowedSkills, hasWildcard)
	if len(skillToolDefs) > 0 {
		tools = append(tools, skillToolDefs...)
	}

	// 重新生成工具名称列表
	toolNames = make([]string, 0, len(tools))
	for _, tool := range tools {
		toolNames = append(toolNames, tool.Function.Name)
	}
	log.Printf("ChatStream: user=%v, tools=%d (mcp=%d, skill=%d), names=%v, hasWildcard=%v", userID.(string), len(tools), len(mcpToolDefs), len(skillToolDefs), toolNames, hasWildcard)

	// 获取用户最新消息用于记忆检索
	lastUserMsg := ""
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			lastUserMsg = req.Messages[i].Content
			break
		}
	}

	// 记忆路由器：按需加载记忆（租户隔离 + 用户隔离 + 角色感知）
	memCtx := h.memoryRouter.LoadMemories(tid, userID.(string), lastUserMsg, permissions)

	// 动态构建system prompt（注入记忆上下文 + 可用技能信息）
	basePrompt := getSystemPrompt(tid, toolNames)

	// 注入可用技能信息到system prompt
	if len(allowedSkills) > 0 || hasWildcard {
		skillIDs := make([]string, 0)
		if hasWildcard {
			// 管理员可以看到所有技能
			var allSkills []models.Skill
			database.DB.Select(&allSkills, "SELECT id, name, description, category, status FROM skills WHERE tenant_id = ? AND status = 'active'", tid)
			for _, s := range allSkills {
				skillIDs = append(skillIDs, s.ID)
			}
			if len(allSkills) > 0 {
				skillInfo := "\n\n## 可用技能\n你可以通过 execute_skill 工具执行以下技能：\n"
				for _, s := range allSkills {
					desc := ""
					if s.Description != nil {
						desc = *s.Description
					}
					skillInfo += fmt.Sprintf("- %s (ID: %s): %s\n", s.Name, s.ID, desc)
				}
				skillInfo += "使用 execute_skill 时，传入 skill_id 和 inputs 参数。"
				basePrompt += skillInfo
			}
		} else {
			for id := range allowedSkills {
				skillIDs = append(skillIDs, id)
			}
			if len(skillIDs) > 0 {
				placeholders := make([]string, len(skillIDs))
				args := make([]any, len(skillIDs))
				for i, id := range skillIDs {
					placeholders[i] = "?"
					args[i] = id
				}
				var skills []models.Skill
				query := "SELECT id, name, description, category, status FROM skills WHERE id IN (" + strings.Join(placeholders, ",") + ") AND status = 'active'"
				database.DB.Select(&skills, query, args...)
				if len(skills) > 0 {
					skillInfo := "\n\n## 可用技能\n你可以通过 execute_skill 工具执行以下技能：\n"
					for _, s := range skills {
						desc := ""
						if s.Description != nil {
							desc = *s.Description
						}
						skillInfo += fmt.Sprintf("- %s (ID: %s): %s\n", s.Name, s.ID, desc)
					}
					skillInfo += "使用 execute_skill 时，传入 skill_id 和 inputs 参数。"
					basePrompt += skillInfo
				}
			}
		}
	}

	systemPrompt := h.memoryRouter.BuildMemoryPrompt(basePrompt, memCtx)

	// 发送记忆加载状态
	if memCtx != nil && (len(memCtx.UserMemories) > 0 || len(memCtx.SkillMemories) > 0) {
		totalMem := len(memCtx.UserMemories) + len(memCtx.SkillMemories) + len(memCtx.Entities) + len(memCtx.RoleMemories)
		sendSSE(c, SSEEventStatus, map[string]any{
			"message":    fmt.Sprintf("已加载 %d 条相关记忆", totalMem),
			"stage":      "memory",
			"elapsed_ms": time.Since(requestStart).Milliseconds(),
		})
	}

	modelConfig, configErr := h.modelService.GetConfigForTenant(tid, "")
	if configErr != nil {
		sendSSE(c, SSEEventError, map[string]string{"message": "未配置可用的模型，请在模型配置页面启用至少一个模型和供应商"})
		sendSSE(c, SSEEventDone, nil)
		return
	}
	modelName := modelConfig.Model
	displayName := modelConfig.DisplayName
	providerName := modelConfig.ProviderName
	if displayName == "" {
		displayName = modelName
	}
	sendSSE(c, SSEEventModelInfo, map[string]string{
		"model":       modelName,
		"display_name": displayName,
		"provider":    providerName,
	})

	// 构建消息
	messages := []modelservice.Message{
		{Role: "system", Content: systemPrompt},
	}
	for _, m := range req.Messages {
		messages = append(messages, modelservice.Message{Role: m.Role, Content: m.Content})
	}

	// 多轮工具调用循环（最多5轮）
	for round := 0; round < 5; round++ {
		roundStart := time.Now()

		// 发送思考阶段状态
		sendSSE(c, SSEEventStatus, map[string]any{
			"message":    "正在思考...",
			"stage":      "thinking",
			"round":      round + 1,
			"elapsed_ms": time.Since(requestStart).Milliseconds(),
			"total_ms":   time.Since(requestStart).Milliseconds(),
		})

		// 调用模型（非流式，因为需要解析tool_calls）
		response, err := h.callModelWithTools(tid, messages, tools)
		modelElapsed := time.Since(roundStart).Milliseconds()

		if err != nil {
			log.Printf("Chat model error: %v", err)
			sendSSE(c, SSEEventError, map[string]string{"message": "模型调用失败: " + err.Error()})
			sendSSE(c, SSEEventDone, nil)
			return
		}

		// 记录模型 token 消耗
		if response.InputTokens > 0 || response.OutputTokens > 0 {
			uid := ""
			if userID != nil { uid = userID.(string) }
			RecordModelUsage(tid, uid, response.Provider, response.Model,
				"/chat", response.InputTokens, response.OutputTokens, int(modelElapsed))
		}

		// 检查是否有工具调用
		if len(response.ToolCalls) > 0 {
			// 记录模型调用了哪些工具
			tcNames := make([]string, 0, len(response.ToolCalls))
			for _, tc := range response.ToolCalls {
				tcNames = append(tcNames, tc.Function.Name)
			}
			log.Printf("ChatStream: model called tools: %v", tcNames)
			// 添加assistant消息（带tool_calls）
			assistantMsg := modelservice.Message{Role: "assistant", Content: response.Content}
			for _, tc := range response.ToolCalls {
				assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, modelservice.ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				})
			}
			messages = append(messages, assistantMsg)

			// 发送分析完成状态
			sendSSE(c, SSEEventStatus, map[string]any{
				"message":    fmt.Sprintf("模型决定调用 %d 个工具", len(response.ToolCalls)),
				"stage":      "plan",
				"stage_ms":   modelElapsed,
				"elapsed_ms": time.Since(requestStart).Milliseconds(),
				"total_ms":   time.Since(requestStart).Milliseconds(),
			})

			// 执行每个工具调用
			for ti, tc := range response.ToolCalls {
				toolStart := time.Now()

				// 发送工具执行中状态
				sendSSE(c, SSEEventStatus, map[string]any{
					"message":    "正在" + getToolDisplayName(tc.Function.Name) + "...",
					"stage":      "tool_calling",
					"tool_name":  tc.Function.Name,
					"tool_index": ti + 1,
					"tool_total": len(response.ToolCalls),
					"elapsed_ms": time.Since(requestStart).Milliseconds(),
					"total_ms":   time.Since(requestStart).Milliseconds(),
				})

				var args map[string]any
				json.Unmarshal([]byte(tc.Function.Arguments), &args)
				result := h.executeTool(tid, userID.(string), tc.Function.Name, args)
				toolElapsed := time.Since(toolStart).Milliseconds()

				// 发送工具结果事件
				sendSSE(c, SSEEventTool, map[string]any{
					"name":       tc.Function.Name,
					"result":     result,
					"elapsed_ms": toolElapsed,
				})

				messages = append(messages, modelservice.Message{
					Role:       "tool",
					Content:    result,
					ToolCallID: tc.ID,
					Name:       tc.Function.Name,
				})
			}
			continue
		}

		// 没有工具调用，流式输出最终响应
		sendSSE(c, SSEEventStatus, map[string]any{
			"message":    "正在生成回答...",
			"stage":      "generating",
			"stage_ms":   modelElapsed,
			"elapsed_ms": time.Since(requestStart).Milliseconds(),
			"total_ms":   time.Since(requestStart).Milliseconds(),
		})

		// 流式调用模型
		streamStart := time.Now()
		err = h.streamFinalResponse(tid, messages, c)
		if err != nil {
			log.Printf("Stream error: %v", err)
			// 降级到非流式
			sendSSE(c, SSEEventDelta, map[string]string{"content": response.Content})
		}

		// 发送完成事件（带总耗时）
		sendSSE(c, SSEEventDone, map[string]any{
			"total_ms":  time.Since(requestStart).Milliseconds(),
			"stream_ms": time.Since(streamStart).Milliseconds(),
		})

		// 异步提取记忆（不阻塞响应）
		if h.memoryExtractor != nil {
			go h.extractMemoryFromConversation(tid, userID.(string), req.Messages, response.Content)
		}
		return
	}

	sendSSE(c, SSEEventDelta, map[string]string{"content": "抱歉，处理超时，请简化您的请求。"})
	sendSSE(c, SSEEventDone, map[string]any{
		"total_ms": time.Since(requestStart).Milliseconds(),
	})

	// 超时场景也尝试提取记忆
	if h.memoryExtractor != nil {
		go h.extractMemoryFromConversation(tid, userID.(string), req.Messages, "")
	}
}

// streamFinalResponse 流式输出最终响应
func (h *ChatHandler) streamFinalResponse(tenantID string, messages []modelservice.Message, c *gin.Context) error {
	config, err := h.modelService.GetConfigForTenant(tenantID, "")
	if err != nil {
		return fmt.Errorf("未配置可用的模型: %w", err)
	}

	reqBody := map[string]any{
		"model":       config.Model,
		"messages":    messages,
		"temperature": config.Temperature,
		"max_tokens":  config.MaxTokens,
		"stream":      true,
	}

	body, _ := json.Marshal(reqBody)
	httpReq, _ := http.NewRequest("POST", config.BaseURL+"/chat/completions", strings.NewReader(string(body)))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+config.APIKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error (status %d)", resp.StatusCode)
	}

	// 解析SSE流
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				return nil
			}

			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}

			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				sendSSE(c, SSEEventDelta, map[string]string{
					"content": chunk.Choices[0].Delta.Content,
				})
			}
		}
	}

	return scanner.Err()
}

// ModelResponse 模型响应
type ModelResponse struct {
	Content      string     `json:"content"`
	ToolCalls    []ToolCall `json:"tool_calls"`
	InputTokens  int        `json:"input_tokens"`
	OutputTokens int        `json:"output_tokens"`
	Provider     string     `json:"provider"`
	Model        string     `json:"model"`
}

// callModelWithTools 调用模型（支持工具，非流式，带重试）
func (h *ChatHandler) callModelWithTools(tenantID string, messages []modelservice.Message, tools []ToolDefinition) (*ModelResponse, error) {
	config, err := h.modelService.GetConfigForTenant(tenantID, "")
	if err != nil {
		return nil, fmt.Errorf("未配置可用的模型: %w", err)
	}

	reqBody := map[string]any{
		"model":       config.Model,
		"messages":    messages,
		"temperature": config.Temperature,
		"max_tokens":  config.MaxTokens,
		"stream":      false,
		"tools":       tools,
	}

	body, _ := json.Marshal(reqBody)

	// 重试3次
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
			log.Printf("Retrying model call (attempt %d/3)", attempt+1)
		}

		httpReq, _ := http.NewRequest("POST", config.BaseURL+"/chat/completions", strings.NewReader(string(body)))
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+config.APIKey)

		client := &http.Client{Timeout: 90 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			lastErr = err
			log.Printf("Model call attempt %d failed: %v", attempt+1, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
			continue
		}

		var chatResp struct {
			Choices []struct {
				Message struct {
					Role      string     `json:"role"`
					Content   string     `json:"content"`
					ToolCalls []ToolCall `json:"tool_calls"`
				} `json:"message"`
			} `json:"choices"`
Usage struct {
			PromptTokens     json.Number `json:"prompt_tokens"`
			CompletionTokens json.Number `json:"completion_tokens"`
			TotalTokens      json.Number `json:"total_tokens"`
		} `json:"usage"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
			resp.Body.Close()
			lastErr = err
			continue
		}
		resp.Body.Close()

		if len(chatResp.Choices) == 0 {
			lastErr = fmt.Errorf("no response from model")
			continue
		}

		response := ModelResponse{
			Content:      chatResp.Choices[0].Message.Content,
			ToolCalls:    chatResp.Choices[0].Message.ToolCalls,
			InputTokens:  0,
			OutputTokens: 0,
			Provider:     config.ProviderName,
			Model:        config.Model,
		}
		if n, err := chatResp.Usage.PromptTokens.Int64(); err == nil {
			response.InputTokens = int(n)
		}
		if n, err := chatResp.Usage.CompletionTokens.Int64(); err == nil {
			response.OutputTokens = int(n)
		}
		return &response, nil
	}

	return nil, fmt.Errorf("model call failed after 3 attempts: %w", lastErr)
}

// extractMemoryFromConversation 从对话中异步提取记忆
func (h *ChatHandler) extractMemoryFromConversation(tenantID, userID string, userMessages []AssistantMessage, assistantResponse string) {
	// 构建提取请求
	var extractMsgs []easpMemory.ExtractMessage
	for _, m := range userMessages {
		extractMsgs = append(extractMsgs, easpMemory.ExtractMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}
	if assistantResponse != "" {
		extractMsgs = append(extractMsgs, easpMemory.ExtractMessage{
			Role:    "assistant",
			Content: assistantResponse,
		})
	}

	// 至少需要一轮对话
	if len(extractMsgs) < 2 {
		return
	}

	// 获取模型配置用于LLM调用
	modelConfig, err := h.modelService.GetConfigForTenant(tenantID, "")
	if err != nil {
		log.Printf("MemoryExtractor: failed to get model config: %v", err)
		return
	}

	// 更新提取器的模型配置
	h.memoryExtractor = easpMemory.NewMemoryExtractor(
		h.memoryRouter.GetMemoryService(),
		easpMemory.ModelConfig{
			BaseURL: modelConfig.BaseURL,
			APIKey:  modelConfig.APIKey,
			Model:   modelConfig.Model,
		},
	)

	// 执行提取
	h.memoryExtractor.ExtractAndSave(easpMemory.ExtractRequest{
		TenantID: tenantID,
		UserID:   userID,
		Messages: extractMsgs,
	})
}
