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
		Collection: "memories",
		Dimension:  1536,
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

// ExecuteSkill 执行Skill
func (h *SkillEngineHandler) ExecuteSkill(c *gin.Context) {
	tenantID := c.Param("tenantId")
	skillID := c.Param("skillId")

	var req struct {
		Inputs map[string]interface{} `json:"inputs"`
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

	// 创建MCP caller函数
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
		resp, callErr := mcpHandler.proxy.CallTool(ctx, mcp.ToolCallRequest{
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

	// 创建带MCP调用能力的引擎
	engine := skill.NewSkillEngineWithCaller(tenantID, mcpCaller)
	execution, err := engine.Execute(c.Request.Context(), skillDef, req.Inputs)
	if err != nil {
		log.Printf("Failed to execute skill: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to execute skill"})
		return
	}

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

	var executions []skill.SkillExecution
	err := database.DB.Select(&executions, query, args...)
	if err != nil {
		log.Printf("Failed to list executions: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list executions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"executions": executions})
}
