package handlers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/easp-platform/easp/internal/database"
	easpMemory "github.com/easp-platform/easp/internal/memory"
	"github.com/easp-platform/easp/internal/middleware"
	"github.com/easp-platform/easp/internal/modelservice"
	"github.com/easp-platform/easp/internal/repositories"
	"github.com/gin-gonic/gin"
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

// toolPermissionMap 工具→权限映射
var toolPermissionMap = map[string]string{
	"list_users":      "users",
	"get_user":        "users",
	"assign_role":     "roles",
	"revoke_role":     "roles",
	"list_roles":      "roles",
	"list_connectors": "connectors",
	"list_mcp_tools":  "mcp-tools",
	"get_tenant_info": "*",
	"update_tenant":   "*",
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
	}
}

// getToolDisplayName 获取工具中文名
func getToolDisplayName(toolName string) string {
	names := map[string]string{
		"list_users":       "查询用户列表",
		"get_user":         "查询用户信息",
		"assign_role":      "分配角色",
		"revoke_role":      "撤销角色",
		"list_roles":       "查询角色列表",
		"list_connectors":  "查询连接器列表",
		"list_mcp_tools":   "查询MCP工具列表",
		"get_tenant_info":  "查询租户信息",
		"update_tenant":    "更新租户配置",
	}
	if name, ok := names[toolName]; ok {
		return name
	}
	return toolName
}

// ExecuteToolByName 导出工具执行函数，供SkillEngine等外部调用
func ExecuteToolByName(tenantID, toolName string, args map[string]any) string {
	h := &ChatHandler{}
	return h.executeTool(tenantID, toolName, args)
}

// executeTool 执行工具调用
func (h *ChatHandler) executeTool(tenantID string, toolName string, args map[string]any) string {
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
		connectors, err := queryMaps("SELECT id, name, type, base_url, status, tools_count FROM connectors WHERE tenant_id = ?", tenantID)
		if err != nil {
			return `{"error": "查询连接器失败: ` + err.Error() + `"}`
		}
		data, _ := json.Marshal(connectors)
		return string(data)

	case "list_mcp_tools":
		tools, err := queryMaps("SELECT id, name, description, backend_method, backend_path, risk_level, enabled FROM mcp_tools WHERE tenant_id = ?", tenantID)
		if err != nil {
			return `{"error": "查询MCP工具失败: ` + err.Error() + `"}`
		}
		data, _ := json.Marshal(tools)
		return string(data)

	case "get_tenant_info":
		var tenant map[string]any
		err := database.DB.Get(&tenant, "SELECT id, name, plan, status, expires_at, max_users, created_at FROM tenants WHERE id = ?", tenantID)
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
		return `{"success": true, "message": "租户配置已更新"}`

	default:
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

	// 获取工具名称列表
	toolNames := make([]string, 0, len(tools))
	for _, tool := range tools {
		toolNames = append(toolNames, tool.Function.Name)
	}

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

	// 动态构建system prompt（注入记忆上下文）
	basePrompt := getSystemPrompt(tid, toolNames)
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

		// 检查是否有工具调用
		if len(response.ToolCalls) > 0 {
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
				result := h.executeTool(tid, tc.Function.Name, args)
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
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls"`
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

		return &ModelResponse{
			Content:   chatResp.Choices[0].Message.Content,
			ToolCalls: chatResp.Choices[0].Message.ToolCalls,
		}, nil
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
