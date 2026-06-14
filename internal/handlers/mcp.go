package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/mcp"
	"github.com/easp-platform/easp/internal/middleware"
	"github.com/easp-platform/easp/internal/models"
	"github.com/easp-platform/easp/internal/openapi"
	skillPkg "github.com/easp-platform/easp/internal/skill"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// MCPHandler MCP处理器
type MCPHandler struct {
	server *mcp.MCPServer
	proxy  *mcp.MCPProxy
}

// NewMCPHandler 创建MCP处理器
func NewMCPHandler() *MCPHandler {
	return &MCPHandler{
		server: mcp.NewMCPServer(),
		proxy:  mcp.NewMCPProxy(mcp.DefaultProxyConfig()),
	}
}

// GetServer 获取MCP服务器
func (h *MCPHandler) GetServer() *mcp.MCPServer {
	return h.server
}

// HandleSSE 处理SSE连接
func (h *MCPHandler) HandleSSE(c *gin.Context) {
	h.server.HandleSSE(c)
}

// HandleMessage 处理MCP消息
func (h *MCPHandler) HandleMessage(c *gin.Context) {
	h.server.HandleMessage(c)
}

// SyncFromOpenAPI 从OpenAPI规范同步工具
func (h *MCPHandler) SyncFromOpenAPI(c *gin.Context) {
	tenantID := c.Param("tenantId")
	connectorID := c.Param("connectorId")

	// 获取连接器
	var connector models.Connector
	err := database.DB.Get(&connector, "SELECT * FROM connectors WHERE id = ? AND tenant_id = ?", connectorID, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Connector not found"})
		return
	}

	// 获取OpenAPI规范
	var spec *openapi.OpenAPISpec
	if connector.SpecURL != nil && *connector.SpecURL != "" {
		spec, err = openapi.FetchOpenAPISpec(*connector.SpecURL)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to fetch OpenAPI spec", "details": err.Error()})
			return
		}
	} else if connector.SpecContent != nil && *connector.SpecContent != "" {
		spec, err = openapi.ParseOpenAPISpec([]byte(*connector.SpecContent))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse OpenAPI spec", "details": err.Error()})
			return
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No OpenAPI spec found"})
		return
	}

	// 转换为MCP工具
	tools := openapi.ConvertToMCPTools(spec)

	// 保存到数据库
	created := 0
	updated := 0
	for _, tool := range tools {
		inputSchemaJSON, _ := json.Marshal(tool.InputSchema)

		// 检查是否已存在
		var existing models.MCPTool
		err := database.DB.Get(&existing, "SELECT * FROM mcp_tools WHERE tenant_id = ? AND connector_id = ? AND name = ?",
			tenantID, connectorID, tool.Name)

		if err != nil {
			// 创建新工具
			newTool := models.MCPTool{
				ID:            uuid.New().String(),
				TenantID:      tenantID,
				ConnectorID:   connectorID,
				Name:          tool.Name,
				Description:   &tool.Description,
				InputSchema:   ptrString(string(inputSchemaJSON)),
				BackendMethod: &tool.Method,
				BackendPath:   &tool.Path,
				RiskLevel:     "low",
				Enabled:       true,
			}
			_, err = database.DB.NamedExec(`INSERT INTO mcp_tools (id, tenant_id, connector_id, name, description, input_schema, backend_method, backend_path, risk_level, enabled, created_at)
				VALUES (:id, :tenant_id, :connector_id, :name, :description, :input_schema, :backend_method, :backend_path, :risk_level, :enabled, NOW())`, newTool)
			if err == nil {
				created++
			}
		} else {
			// 更新已有工具
			_, err = database.DB.Exec(`UPDATE mcp_tools SET description = ?, input_schema = ?, backend_method = ?, backend_path = ? WHERE id = ?`,
				tool.Description, string(inputSchemaJSON), tool.Method, tool.Path, existing.ID)
			if err == nil {
				updated++
			}
		}
	}

	// 更新连接器工具数量
	database.DB.Exec("UPDATE connectors SET tools_count = ?, last_sync_at = NOW() WHERE id = ?", len(tools), connectorID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Sync completed",
		"total":   len(tools),
		"created": created,
		"updated": updated,
	})
}

// CallTool 调用MCP工具
func (h *MCPHandler) CallTool(c *gin.Context) {
	tenantID := c.Param("tenantId")
	toolID := c.Param("toolId")
	start := time.Now()
	uid := ""
	if v, ok := c.Get(middleware.ContextUserID); ok {
		uid, _ = v.(string)
	}
	requestID := uuid.New().String()

	// 获取工具
	var tool models.MCPTool
	err := database.DB.Get(&tool, "SELECT * FROM mcp_tools WHERE id = ? AND tenant_id = ?", toolID, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tool not found"})
		return
	}

	if !tool.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tool is disabled"})
		return
	}

	// 解析参数与执行模式。空 execution_mode 归一为 sandbox，避免测试调用默认打生产。
	var arguments json.RawMessage
	executionMode := skillPkg.ExecutionModeSandbox
	if c.Request.Body != nil {
		body, _ := c.GetRawData()
		if len(body) > 0 {
			// 尝试解析为arguments
			var req struct {
				Arguments     json.RawMessage `json:"arguments"`
				ExecutionMode string          `json:"execution_mode"`
			}
			if err := json.Unmarshal(body, &req); err == nil {
				executionMode = skillPkg.NormalizeExecutionMode(req.ExecutionMode)
				if req.Arguments != nil {
					arguments = req.Arguments
				} else {
					arguments = body
				}
			} else {
				arguments = body
			}
		}
	}
	if err := skillPkg.CanExecuteMCPTool(tool, executionMode); err != nil {
		RecordToolCallUsage(tenantID, uid, "mcp_tool", tool.ID, tool.Name, "mcp_api", "failed", int(time.Since(start).Milliseconds()), requestID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "execution_mode": executionMode})
		return
	}
	if skillPkg.ShouldSkipSideEffects(executionMode) {
		RecordToolCallUsage(tenantID, uid, "mcp_tool", tool.ID, tool.Name, "mcp_api", "success", int(time.Since(start).Milliseconds()), requestID, nil)
		c.JSON(http.StatusOK, mcp.ToolCallResponse{
			Success: true,
			Data: gin.H{
				"execution_mode": executionMode,
				"dry_run":        true,
				"message":        "沙箱/预演模式未执行MCP外部调用",
				"tool_id":        tool.ID,
				"tool_name":      tool.Name,
				"arguments":      arguments,
			},
			Latency: time.Since(start).Milliseconds(),
		})
		return
	}

	// 获取连接器
	var connector models.Connector
	err = database.DB.Get(&connector, "SELECT * FROM connectors WHERE id = ?", tool.ConnectorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Connector not found"})
		return
	}

	// 调用工具
	ctx := contextWithUserSSOToken(c.Request.Context(), tenantID, uid)
	resp, err := h.proxy.CallTool(ctx, mcp.ToolCallRequest{
		Tool:      tool,
		Connector: connector,
		Arguments: arguments,
	})
	if err != nil {
		log.Printf("Tool call failed: %v", err)
		RecordToolCallUsage(tenantID, uid, "mcp_tool", tool.ID, tool.Name, "mcp_api", "failed", int(time.Since(start).Milliseconds()), requestID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tool call failed", "details": err.Error()})
		return
	}

	RecordToolCallUsage(tenantID, uid, "mcp_tool", tool.ID, tool.Name, "mcp_api", "success", int(time.Since(start).Milliseconds()), requestID, nil)
	c.JSON(http.StatusOK, resp)
}

// GetMCPInfo 获取MCP服务信息
func (h *MCPHandler) GetMCPInfo(c *gin.Context) {
	tenantID := c.Param("tenantId")

	// 获取工具数量
	var toolCount int
	database.DB.Get(&toolCount, "SELECT COUNT(*) FROM mcp_tools WHERE tenant_id = ? AND enabled = true AND status IN ('published', 'active')", tenantID)

	// 获取连接器数量
	var connectorCount int
	database.DB.Get(&connectorCount, "SELECT COUNT(*) FROM connectors WHERE tenant_id = ?", tenantID)

	c.JSON(http.StatusOK, gin.H{
		"service":          "EASP MCP Server",
		"version":          "1.0.0",
		"protocol_version": mcp.MCPVersion,
		"tenant_id":        tenantID,
		"tools_count":      toolCount,
		"connectors_count": connectorCount,
		"active_sessions":  h.server.GetActiveSessions(),
		"endpoints": gin.H{
			"sse":     "/api/v1/mcp/" + tenantID + "/sse",
			"message": "/api/v1/mcp/" + tenantID + "/message",
			"tools":   "/api/v1/mcp/" + tenantID + "/tools",
		},
	})
}

// ListMCPTools 列出MCP工具（MCP协议格式）
func (h *MCPHandler) ListMCPTools(c *gin.Context) {
	tenantID := c.Param("tenantId")

	var tools []models.MCPTool
	err := database.DB.Select(&tools, "SELECT * FROM mcp_tools WHERE tenant_id = ? AND enabled = true AND status IN ('published', 'active') ORDER BY name", tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list tools"})
		return
	}

	// 转换为MCP格式
	mcpTools := make([]mcp.Tool, len(tools))
	for i, tool := range tools {
		mcpTool := mcp.Tool{
			Name: tool.Name,
		}
		if tool.Description != nil {
			mcpTool.Description = *tool.Description
		}
		if tool.InputSchema != nil {
			var schema interface{}
			json.Unmarshal([]byte(*tool.InputSchema), &schema)
			mcpTool.InputSchema = schema
		}
		mcpTools[i] = mcpTool
	}

	c.JSON(http.StatusOK, gin.H{
		"tools": mcpTools,
	})
}

// GetCircuitBreakerStats 获取熔断器统计
func (h *MCPHandler) GetCircuitBreakerStats(c *gin.Context) {
	stats := h.proxy.GetCircuitBreakerStats()
	c.JSON(http.StatusOK, gin.H{"circuit_breakers": stats})
}

// GetRateLimiterStats 获取限流器统计
func (h *MCPHandler) GetRateLimiterStats(c *gin.Context) {
	stats := h.proxy.GetRateLimiterStats()
	c.JSON(http.StatusOK, gin.H{"rate_limiters": stats})
}

// GetOpenAPISpec 获取连接器的OpenAPI规范
func (h *MCPHandler) GetOpenAPISpec(c *gin.Context) {
	connectorID := c.Param("connectorId")

	var connector models.Connector
	err := database.DB.Get(&connector, "SELECT * FROM connectors WHERE id = ?", connectorID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Connector not found"})
		return
	}

	if connector.SpecURL != nil && *connector.SpecURL != "" {
		spec, err := openapi.FetchOpenAPISpec(*connector.SpecURL)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch spec", "details": err.Error()})
			return
		}
		c.JSON(http.StatusOK, spec)
	} else if connector.SpecContent != nil && *connector.SpecContent != "" {
		var spec interface{}
		json.Unmarshal([]byte(*connector.SpecContent), &spec)
		c.JSON(http.StatusOK, spec)
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": "No OpenAPI spec found"})
	}
}

// UpdateOpenAPISpec 更新连接器的OpenAPI规范
func (h *MCPHandler) UpdateOpenAPISpec(c *gin.Context) {
	connectorID := c.Param("connectorId")

	var req struct {
		SpecURL     string `json:"spec_url"`
		SpecContent string `json:"spec_content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.SpecURL != "" {
		_, err := openapi.FetchOpenAPISpec(req.SpecURL)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OpenAPI spec URL", "details": err.Error()})
			return
		}
		database.DB.Exec("UPDATE connectors SET spec_url = ?, spec_content = NULL WHERE id = ?", req.SpecURL, connectorID)
	} else if req.SpecContent != "" {
		_, err := openapi.ParseOpenAPISpec([]byte(req.SpecContent))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OpenAPI spec content", "details": err.Error()})
			return
		}
		database.DB.Exec("UPDATE connectors SET spec_content = ?, spec_url = NULL WHERE id = ?", req.SpecContent, connectorID)
	}

	c.JSON(http.StatusOK, gin.H{"message": "OpenAPI spec updated"})
}

// ptrString 返回字符串指针
func ptrString(s string) *string {
	return &s
}

// ImportOpenAPIRequest 导入OpenAPI/RESTful API请求
type ImportOpenAPIRequest struct {
	Name         string `json:"name"`
	BaseURL      string `json:"base_url"`
	SpecContent  string `json:"spec_content"`
	SpecURL      string `json:"spec_url"`
	ConnectorID  string `json:"connector_id"`
	APIPath      string `json:"api_path"`
	Method       string `json:"method"`
	Description  string `json:"description"`
	InputSchema  string `json:"input_schema"`
	Status       string `json:"status"`
	RiskLevel    string `json:"risk_level"`
	EnabledValue *bool  `json:"enabled"`
}

// ImportOpenAPI 导入OpenAPI文档或单个RESTful API并自动生成MCP工具
func (h *MCPHandler) ImportOpenAPI(c *gin.Context) {
	tenantID := c.Param("tenantId")

	var req ImportOpenAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 单接口 RESTful API 导入：复用已有连接器，直接生成 MCP 工具。
	if req.ConnectorID != "" || req.APIPath != "" || req.Method != "" {
		restReq, err := normalizeRESTImportRequest(req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var connector models.Connector
		if err := database.DB.Get(&connector, "SELECT * FROM connectors WHERE id = ? AND tenant_id = ?", restReq.ConnectorID, tenantID); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "连接器不存在"})
			return
		}
		_, err = database.DB.Exec(`INSERT INTO mcp_tools (id, tenant_id, connector_id, name, description, input_schema, backend_method, backend_path, risk_level, status, enabled, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`, uuid.New().String(), tenantID, restReq.ConnectorID, restReq.Name, restReq.Description, restReq.InputSchema, restReq.Method, restReq.APIPath, restReq.RiskLevel, restReq.Status, restReq.Enabled)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建RESTful MCP工具失败", "details": err.Error()})
			return
		}
		database.DB.Exec("UPDATE connectors SET tools_count = (SELECT COUNT(*) FROM mcp_tools WHERE connector_id = ?), last_sync_at = NOW() WHERE id = ?", restReq.ConnectorID, restReq.ConnectorID)
		c.JSON(http.StatusOK, gin.H{"message": "RESTful API import completed", "connector_id": restReq.ConnectorID, "total": 1, "created": 1})
		return
	}

	// OpenAPI 文档导入：支持 spec_content 或 spec_url。
	var spec *openapi.OpenAPISpec
	var err error
	if req.SpecContent != "" {
		spec, err = openapi.ParseOpenAPISpec([]byte(req.SpecContent))
	} else if req.SpecURL != "" {
		spec, err = openapi.FetchOpenAPISpec(req.SpecURL)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "spec_content 或 spec_url 为必填项"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OpenAPI spec", "details": err.Error()})
		return
	}

	connectorID := uuid.New().String()
	_, err = database.DB.NamedExec(`INSERT INTO connectors (id, tenant_id, name, type, base_url, spec_url, spec_content, status, created_at, updated_at)
		VALUES (:id, :tenant_id, :name, 'openapi', :base_url, :spec_url, :spec_content, 'active', NOW(), NOW())`, map[string]interface{}{
		"id":           connectorID,
		"tenant_id":    tenantID,
		"name":         req.Name,
		"base_url":     req.BaseURL,
		"spec_url":     req.SpecURL,
		"spec_content": req.SpecContent,
	})
	if err != nil {
		log.Printf("Failed to create connector: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create connector", "details": err.Error()})
		return
	}

	tools := openapi.ConvertToMCPTools(spec)
	created := 0
	for _, tool := range tools {
		inputSchemaJSON, _ := json.Marshal(tool.InputSchema)
		_, err = database.DB.Exec(`INSERT INTO mcp_tools (id, tenant_id, connector_id, name, description, input_schema, backend_method, backend_path, risk_level, status, enabled, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'low', 'published', true, NOW(), NOW())`, uuid.New().String(), tenantID, connectorID, tool.Name, tool.Description, string(inputSchemaJSON), tool.Method, tool.Path)
		if err == nil {
			created++
		}
	}

	database.DB.Exec("UPDATE connectors SET tools_count = ?, last_sync_at = NOW() WHERE id = ?", created, connectorID)
	c.JSON(http.StatusOK, gin.H{"message": "Import completed", "connector_id": connectorID, "total": len(tools), "created": created})
}

// DiscoverMCPTools 从连接器的MCP Server发现工具
func (h *MCPHandler) DiscoverMCPTools(c *gin.Context) {
	tenantID := c.Param("tenantId")
	connectorID := c.Param("connectorId")

	// 获取连接器
	var connector models.Connector
	err := database.DB.Get(&connector, "SELECT * FROM connectors WHERE id = ? AND tenant_id = ?", connectorID, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "连接器不存在"})
		return
	}

	if connector.MCPServerURL == nil || *connector.MCPServerURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该连接器未配置MCP Server地址"})
		return
	}

	// 调用MCP Client发现工具
	client := mcp.GetDefaultClient()
	ctx := c.Request.Context()
	result, err := client.DiscoverTools(ctx, *connector.MCPServerURL, connector.TransportType, connector.AuthType, connector.AuthConfig, connector.Headers)
	if err != nil {
		log.Printf("MCP discover failed for connector %s: %v", connectorID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "MCP工具发现失败", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"server_info": result.ServerInfo,
		"tools":       result.Tools,
		"total":       len(result.Tools),
	})
}

// ImportMCPToolsRequest 导入MCP工具请求
type ImportMCPToolsRequest struct {
	ToolNames []string `json:"tool_names"` // 要导入的工具名列表，空=全部导入
}

// ImportMCPTools 从MCP Server导入工具到EASP
func (h *MCPHandler) ImportMCPTools(c *gin.Context) {
	tenantID := c.Param("tenantId")
	connectorID := c.Param("connectorId")

	var req ImportMCPToolsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取连接器
	var connector models.Connector
	err := database.DB.Get(&connector, "SELECT * FROM connectors WHERE id = ? AND tenant_id = ?", connectorID, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "连接器不存在"})
		return
	}

	if connector.MCPServerURL == nil || *connector.MCPServerURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该连接器未配置MCP Server地址"})
		return
	}

	// 发现工具
	client := mcp.GetDefaultClient()
	ctx := c.Request.Context()
	result, err := client.DiscoverTools(ctx, *connector.MCPServerURL, connector.TransportType, connector.AuthType, connector.AuthConfig, connector.Headers)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "MCP工具发现失败", "details": err.Error()})
		return
	}

	// 过滤要导入的工具
	toolsToImport := result.Tools
	if len(req.ToolNames) > 0 {
		nameSet := make(map[string]bool)
		for _, name := range req.ToolNames {
			nameSet[name] = true
		}
		filtered := make([]mcp.DiscoveredTool, 0)
		for _, tool := range result.Tools {
			if nameSet[tool.Name] {
				filtered = append(filtered, tool)
			}
		}
		toolsToImport = filtered
	}

	// 导入工具
	created := 0
	updated := 0
	for _, tool := range toolsToImport {
		inputSchemaJSON, _ := json.Marshal(tool.InputSchema)
		desc := tool.Description
		method := "POST" // MCP工具默认POST
		path := "/" + tool.Name

		// 检查是否已存在
		var existing models.MCPTool
		err := database.DB.Get(&existing, "SELECT * FROM mcp_tools WHERE tenant_id = ? AND connector_id = ? AND name = ?",
			tenantID, connectorID, tool.Name)

		if err != nil {
			// 创建新工具
			newTool := models.MCPTool{
				ID:            uuid.New().String(),
				TenantID:      tenantID,
				ConnectorID:   connectorID,
				Name:          tool.Name,
				Description:   &desc,
				InputSchema:   ptrString(string(inputSchemaJSON)),
				BackendMethod: &method,
				BackendPath:   &path,
				RiskLevel:     "low",
				Enabled:       true,
			}
			_, err = database.DB.NamedExec(`INSERT INTO mcp_tools (id, tenant_id, connector_id, name, description, input_schema, backend_method, backend_path, risk_level, enabled, created_at)
				VALUES (:id, :tenant_id, :connector_id, :name, :description, :input_schema, :backend_method, :backend_path, :risk_level, :enabled, NOW())`, newTool)
			if err == nil {
				created++
			} else {
				log.Printf("Failed to import MCP tool %s: %v", tool.Name, err)
			}
		} else {
			// 更新已有工具
			_, err = database.DB.Exec(`UPDATE mcp_tools SET description = ?, input_schema = ? WHERE id = ?`,
				desc, string(inputSchemaJSON), existing.ID)
			if err == nil {
				updated++
			}
		}
	}

	// 更新连接器工具数量
	var totalTools int
	database.DB.Get(&totalTools, "SELECT COUNT(*) FROM mcp_tools WHERE connector_id = ?", connectorID)
	database.DB.Exec("UPDATE connectors SET tools_count = ?, last_sync_at = NOW() WHERE id = ?", totalTools, connectorID)

	c.JSON(http.StatusOK, gin.H{
		"message":     "导入完成",
		"total":       len(toolsToImport),
		"created":     created,
		"updated":     updated,
		"server_info": result.ServerInfo,
	})
}

// ParseLimitOffset 解析分页参数
func ParseLimitOffset(c *gin.Context, defaultLimit, maxLimit int) (int, int) {
	limitStr := c.DefaultQuery("limit", strconv.Itoa(defaultLimit))
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	return limit, offset
}
