package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/mcp"
	"github.com/easp-platform/easp/internal/memory"
	"github.com/easp-platform/easp/internal/middleware"
	"github.com/easp-platform/easp/internal/models"
	"github.com/easp-platform/easp/internal/skill"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// VectorMemoryHandler 向量记忆处理器
type VectorMemoryHandler struct {
	service *memory.VectorMemoryService
}

// NewVectorMemoryHandler 创建向量记忆处理器
func NewVectorMemoryHandler() *VectorMemoryHandler {
	config := memory.VectorMemoryConfig{
		BridgeURL:  "http://localhost:8083",
		Database:   "easp_memory",
		Collection: "memories", // 1024维 + 腾讯云内置 Embedding
		Dimension:  1024,       // bge-large-zh-v1.5 维度
	}

	service := memory.NewVectorMemoryService(config)
	service.Init()

	return &VectorMemoryHandler{service: service}
}

// SaveMemory 保存记忆
func (h *VectorMemoryHandler) SaveMemory(c *gin.Context) {
	tenantID := c.Param("tenantId")

	var req struct {
		PoolID      string                 `json:"pool_id"`
		Content     string                 `json:"content"`
		Type        string                 `json:"type"`
		Sensitivity string                 `json:"sensitivity"`
		Metadata    map[string]interface{} `json:"metadata,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Type == "" {
		req.Type = "fact"
	}
	if req.Sensitivity == "" {
		req.Sensitivity = "normal"
	}

	mem := memory.VectorMemory{
		ID:          uuid.New().String(),
		TenantID:    tenantID,
		PoolID:      req.PoolID,
		Content:     req.Content,
		Type:        req.Type,
		Sensitivity: req.Sensitivity,
		Metadata:    req.Metadata,
		CreatedAt:   time.Now(),
	}

	if err := h.service.SaveMemory(mem); err != nil {
		log.Printf("Failed to save memory: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save memory"})
		return
	}

	c.JSON(http.StatusCreated, mem)
}

// SearchMemories 搜索记忆
func (h *VectorMemoryHandler) SearchMemories(c *gin.Context) {
	tenantID := c.Param("tenantId")
	query := c.Query("q")
	poolID := c.Query("pool_id")
	limitStr := c.DefaultQuery("limit", "10")

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query is required"})
		return
	}

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 10
	}

	memories, err := h.service.SearchMemories(tenantID, poolID, query, limit)
	if err != nil {
		log.Printf("Failed to search memories: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search memories"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"memories": memories})
}

// ListMemories 列出记忆
func (h *VectorMemoryHandler) ListMemories(c *gin.Context) {
	tenantID := c.Param("tenantId")
	poolID := c.Query("pool_id")
	limitStr := c.DefaultQuery("limit", "50")

	limit, _ := strconv.Atoi(limitStr)

	memories, err := h.service.ListMemories(tenantID, poolID, limit)
	if err != nil {
		log.Printf("Failed to list memories: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list memories"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"memories": memories})
}

// DeleteMemory 删除记忆
func (h *VectorMemoryHandler) DeleteMemory(c *gin.Context) {
	memoryID := c.Param("memoryId")

	if err := h.service.DeleteMemory(memoryID); err != nil {
		log.Printf("Failed to delete memory: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete memory"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Memory deleted"})
}

// SkillEngineHandler Skill引擎处理器
type SkillEngineHandler struct {
	// 不再持有共享engine，每次请求创建独立的engine
}

// NewSkillEngineHandler 创建Skill引擎处理器
func NewSkillEngineHandler() *SkillEngineHandler {
	return &SkillEngineHandler{}
}

func normalizeSkillActionName(toolName string) string {
	switch toolName {
	case "getUsers":
		return "list_users"
	case "getUser":
		return "get_user"
	case "getTenantInfo":
		return "get_tenant_info"
	case "getConnectors":
		return "list_connectors"
	case "getMCPTools":
		return "list_mcp_tools"
	case "getRoles":
		return "list_roles"
	default:
		return toolName
	}
}

func isBuiltinSkillTool(toolName string) bool {
	switch toolName {
	case "list_users", "get_user", "create_user", "assign_role", "revoke_role", "list_roles", "create_role",
		"list_connectors", "get_connector", "create_connector", "update_connector",
		"list_mcp_tools", "get_mcp_tool", "create_mcp_tool", "update_mcp_tool",
		"list_skills", "get_skill", "create_skill", "update_skill",
		"list_memory_pools", "get_memory_entries", "create_memory_pool", "create_memory_entry", "update_memory_entry",
		"get_tenant_info", "update_tenant":
		return true
	default:
		return false
	}
}

// ExecuteSkill 执行Skill
func (h *SkillEngineHandler) ExecuteSkill(c *gin.Context) {
	tenantID := c.Param("tenantId")
	skillID := c.Param("skillId")
	start := time.Now()
	requestID := uuid.New().String()
	uid := ""
	if v, ok := c.Get(middleware.ContextUserID); ok {
		uid, _ = v.(string)
	}

	var req struct {
		Inputs        map[string]interface{} `json:"inputs"`
		ExecutionMode string                 `json:"execution_mode"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取Skill定义
	var skillDef models.Skill
	err := database.DB.Get(&skillDef, "SELECT * FROM skills WHERE id = ? AND tenant_id = ?", skillID, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Skill not found"})
		return
	}
	executionMode := skill.NormalizeExecutionMode(req.ExecutionMode)

	// 创建MCP caller函数
	// 兼容两类 action：
	// 1) AI助手内置工具（list_users/get_user/list_connectors/get_tenant_info 等）
	// 2) 租户 mcp_tools 表中的外部 MCP/REST 工具
	mcpCaller := func(ctx context.Context, toolName string, arguments json.RawMessage) (map[string]interface{}, error) {
		normalizedToolName := normalizeSkillActionName(toolName)

		if isBuiltinSkillTool(normalizedToolName) {
			var args map[string]any
			if len(arguments) > 0 {
				if err := json.Unmarshal(arguments, &args); err != nil {
					return nil, fmt.Errorf("invalid builtin tool arguments: %w", err)
				}
			}
			if args == nil {
				args = map[string]any{}
			}

			toolStart := time.Now()
			rawResult := ExecuteToolByName(tenantID, normalizedToolName, args)
			status, resultErr := toolCallStatusFromResult(rawResult)
			RecordToolCallUsage(tenantID, uid, "builtin_tool", "", normalizedToolName, "skill", status, int(time.Since(toolStart).Milliseconds()), requestID, resultErr)
			var resultMap map[string]interface{}
			if err := json.Unmarshal([]byte(rawResult), &resultMap); err != nil {
				return map[string]interface{}{"result": rawResult}, nil
			}
			if errText, ok := resultMap["error"].(string); ok && errText != "" {
				return nil, fmt.Errorf("%s", errText)
			}
			return resultMap, nil
		}

		var mcpTool models.MCPTool
		if err := database.DB.Get(&mcpTool, "SELECT * FROM mcp_tools WHERE name = ? AND tenant_id = ? AND enabled = true", normalizedToolName, tenantID); err != nil {
			return nil, fmt.Errorf("MCP tool not found: %s", toolName)
		}
		if err := skill.CanExecuteMCPTool(mcpTool, executionMode); err != nil {
			return nil, err
		}
		var connector models.Connector
		if err := database.DB.Get(&connector, "SELECT * FROM connectors WHERE id = ?", mcpTool.ConnectorID); err != nil {
			return nil, fmt.Errorf("connector not found")
		}
		mcpHandler := NewMCPHandler()
		toolStart := time.Now()
		ctxWithToken := contextWithUserSSOToken(ctx, tenantID, uid)
		resp, callErr := mcpHandler.proxy.CallTool(ctxWithToken, mcp.ToolCallRequest{
			Tool:      mcpTool,
			Connector: connector,
			Arguments: arguments,
		})
		if callErr != nil {
			RecordToolCallUsage(tenantID, uid, "mcp_tool", mcpTool.ID, mcpTool.Name, "skill", "failed", int(time.Since(toolStart).Milliseconds()), requestID, callErr)
			return nil, callErr
		}
		if !resp.Success {
			err := fmt.Errorf("MCP tool error: %s", resp.Error)
			RecordToolCallUsage(tenantID, uid, "mcp_tool", mcpTool.ID, mcpTool.Name, "skill", "failed", int(time.Since(toolStart).Milliseconds()), requestID, err)
			return nil, err
		}
		RecordToolCallUsage(tenantID, uid, "mcp_tool", mcpTool.ID, mcpTool.Name, "skill", "success", int(time.Since(toolStart).Milliseconds()), requestID, nil)
		resultMap, ok := resp.Data.(map[string]interface{})
		if !ok {
			resultMap = map[string]interface{}{"result": resp.Data}
		}
		return resultMap, nil
	}

	// 创建带MCP调用能力的引擎
	engine := skill.NewSkillEngineWithCaller(tenantID, mcpCaller)
	execution, err := engine.ExecuteWithMode(c.Request.Context(), skillDef, req.Inputs, executionMode)
	if err != nil {
		log.Printf("Failed to execute skill: %v", err)
		RecordToolCallUsage(tenantID, uid, "skill", skillDef.ID, skillDef.Name, "manual", "failed", int(time.Since(start).Milliseconds()), requestID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to execute skill"})
		return
	}

	RecordToolCallUsage(tenantID, uid, "skill", skillDef.ID, skillDef.Name, "manual", "success", int(time.Since(start).Milliseconds()), requestID, nil)
	c.JSON(http.StatusOK, execution)
}

// GetExecution 获取执行记录
func (h *SkillEngineHandler) GetExecution(c *gin.Context) {
	executionID := c.Param("executionId")

	var execution skill.SkillExecution
	err := database.DB.Get(&execution, "SELECT * FROM skill_executions WHERE id = ?", executionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Execution not found"})
		return
	}

	c.JSON(http.StatusOK, execution)
}

// ListExecutions 列出执行记录
func (h *SkillEngineHandler) ListExecutions(c *gin.Context) {
	tenantID := c.Param("tenantId")
	skillID := c.Query("skill_id")
	limitStr := c.DefaultQuery("limit", "20")

	limit, _ := strconv.Atoi(limitStr)

	query := "SELECT * FROM skill_executions WHERE tenant_id = ?"
	args := []interface{}{tenantID}

	if skillID != "" {
		query += " AND skill_id = ?"
		args = append(args, skillID)
	}

	query += " ORDER BY started_at DESC LIMIT ?"
	args = append(args, limit)

	type skillExecutionListItem struct {
		ID          string     `db:"id" json:"id"`
		SkillID     string     `db:"skill_id" json:"skill_id"`
		TenantID    string     `db:"tenant_id" json:"tenant_id"`
		Status      string     `db:"status" json:"status"`
		Inputs      *string    `db:"inputs" json:"inputs,omitempty"`
		Outputs     *string    `db:"outputs" json:"outputs,omitempty"`
		StepResults *string    `db:"step_results" json:"step_results,omitempty"`
		StartedAt   time.Time  `db:"started_at" json:"started_at"`
		EndedAt     *time.Time `db:"ended_at" json:"ended_at,omitempty"`
		Error       *string    `db:"error" json:"error,omitempty"`
		DurationMS  int64      `json:"duration_ms"`
	}

	var executions []skillExecutionListItem
	err := database.DB.Select(&executions, query, args...)
	if err != nil {
		log.Printf("Failed to list executions: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list executions"})
		return
	}
	if executions == nil {
		executions = []skillExecutionListItem{}
	}
	for i := range executions {
		if executions[i].EndedAt != nil {
			executions[i].DurationMS = executions[i].EndedAt.Sub(executions[i].StartedAt).Milliseconds()
		}
	}

	c.JSON(http.StatusOK, gin.H{"executions": executions})
}
