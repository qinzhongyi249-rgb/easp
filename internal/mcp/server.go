package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/models"
	"github.com/easp-platform/easp/internal/skill"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// MCPServer MCP服务器
type MCPServer struct {
	sessions sync.Map // sessionID -> *MCPSession
	tools    sync.Map // tenantID -> []Tool
	handlers map[string]MethodHandler
	mu       sync.RWMutex
}

// MethodHandler 方法处理器
type MethodHandler func(ctx context.Context, session *MCPSession, params json.RawMessage) (interface{}, error)

// NewMCPServer 创建MCP服务器
func NewMCPServer() *MCPServer {
	s := &MCPServer{
		handlers: make(map[string]MethodHandler),
	}
	s.registerHandlers()
	return s
}

// registerHandlers 注册方法处理器
func (s *MCPServer) registerHandlers() {
	s.handlers[MethodInitialize] = s.handleInitialize
	s.handlers[MethodToolsList] = s.handleToolsList
	s.handlers[MethodToolsCall] = s.handleToolsCall
	s.handlers[MethodPing] = s.handlePing
}

// HandleSSE 处理SSE连接
func (s *MCPServer) HandleSSE(c *gin.Context) {
	tenantID := c.Param("tenantId")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenantId is required"})
		return
	}

	// 设置SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// 创建会话
	sessionID := uuid.New().String()
	session := &MCPSession{
		ID:        sessionID,
		TenantID:  tenantID,
		CreatedAt: time.Now(),
		LastPingAt: time.Now(),
	}
	s.sessions.Store(sessionID, session)

	// 发送endpoint事件
	endpoint := fmt.Sprintf("/api/v1/mcp/%s/message?sessionId=%s", tenantID, sessionID)
	c.SSEvent("endpoint", endpoint)
	c.Writer.Flush()

	// 保持连接
	ctx := c.Request.Context()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.sessions.Delete(sessionID)
			return
		case <-ticker.C:
			// 发送心跳
			c.SSEvent("ping", time.Now().Unix())
			c.Writer.Flush()
		}
	}
}

// HandleMessage 处理JSON-RPC消息
func (s *MCPServer) HandleMessage(c *gin.Context) {
	sessionID := c.Query("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId is required"})
		return
	}

	// 获取会话
	sessionRaw, ok := s.sessions.Load(sessionID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	session := sessionRaw.(*MCPSession)

	// 解析请求
	var request JSONRPCRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusOK, NewJSONRPCError(nil, ParseError, "Parse error", nil))
		return
	}

	// 处理请求
	s.processRequest(c, session, &request)
}

// processRequest 处理请求
func (s *MCPServer) processRequest(c *gin.Context, session *MCPSession, request *JSONRPCRequest) {
	s.mu.RLock()
	handler, exists := s.handlers[request.Method]
	s.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusOK, NewJSONRPCError(request.ID, MethodNotFound, 
			fmt.Sprintf("Method not found: %s", request.Method), nil))
		return
	}

	// 调用处理器
	ctx := c.Request.Context()
	result, err := handler(ctx, session, request.Params)
	if err != nil {
		c.JSON(http.StatusOK, NewJSONRPCError(request.ID, InternalError, 
			err.Error(), nil))
		return
	}

	c.JSON(http.StatusOK, NewJSONRPCResponse(request.ID, result))
}

// handleInitialize 处理初始化
func (s *MCPServer) handleInitialize(ctx context.Context, session *MCPSession, params json.RawMessage) (interface{}, error) {
	var initParams InitializeParams
	if err := json.Unmarshal(params, &initParams); err != nil {
		return nil, fmt.Errorf("invalid initialize params: %w", err)
	}

	// 更新会话信息
	session.ClientInfo = initParams.ClientInfo
	session.Capabilities = initParams.Capabilities

	log.Printf("MCP session initialized: %s, client: %s", session.ID, initParams.ClientInfo.Name)

	return InitializeResult{
		ProtocolVersion: MCPVersion,
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{
				ListChanged: true,
			},
		},
		ServerInfo: ServerInfo{
			Name:    "EASP MCP Server",
			Version: "1.0.0",
		},
	}, nil
}

// handleToolsList 处理工具列表 - 返回 MCP 工具 + Skills
func (s *MCPServer) handleToolsList(ctx context.Context, session *MCPSession, params json.RawMessage) (interface{}, error) {
	// 从session中获取tenantID（通过SSE连接时的URL参数）
	// 这里需要从context或session中获取tenantID
	// 由于SSE连接时tenantID在URL中，我们需要存储到session中
	tenantID := session.TenantID
	if tenantID == "" {
		return map[string]interface{}{
			"tools": []Tool{},
		}, nil
	}

	var allTools []Tool

	// 1. 查询 MCP 工具
	var mcpTools []struct {
		Name        string  `db:"name"`
		Description *string `db:"description"`
		InputSchema *string `db:"input_schema"`
	}
	err := database.DB.Select(&mcpTools, "SELECT name, description, input_schema FROM mcp_tools WHERE tenant_id = ? AND enabled = true ORDER BY name", tenantID)
	if err == nil {
		for _, t := range mcpTools {
			tool := Tool{Name: t.Name}
			if t.Description != nil {
				tool.Description = *t.Description
			}
			if t.InputSchema != nil {
				var schema interface{}
				json.Unmarshal([]byte(*t.InputSchema), &schema)
				tool.InputSchema = schema
			}
			allTools = append(allTools, tool)
		}
	}

	// 2. 查询 Skills，转换为 MCP 工具格式（加 skill_ 前缀避免冲突）
	var skills []struct {
		Name        string  `db:"name"`
		Description *string `db:"description"`
		InputSchema *string `db:"input_schema"`
		Steps       string  `db:"steps"`
	}
	err = database.DB.Select(&skills, "SELECT name, description, input_schema, steps FROM skills WHERE tenant_id = ? AND status = 'active' ORDER BY name", tenantID)
	if err == nil {
		for _, sk := range skills {
			toolName := "skill_" + sk.Name
			tool := Tool{Name: toolName}
			if sk.Description != nil {
				tool.Description = "[技能] " + *sk.Description
			} else {
				tool.Description = "[技能] " + sk.Name
			}
			// 使用 Skill 的 input_schema 作为 MCP 工具的 inputSchema
			if sk.InputSchema != nil {
				var schema interface{}
				json.Unmarshal([]byte(*sk.InputSchema), &schema)
				tool.InputSchema = schema
			} else {
				// 默认接受任意 JSON 参数
				tool.InputSchema = map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				}
			}
			allTools = append(allTools, tool)
		}
	}

	if allTools == nil {
		allTools = []Tool{}
	}

	return map[string]interface{}{
		"tools": allTools,
	}, nil
}

// handleToolsCall 处理工具调用 - 支持 MCP 工具和 Skill
func (s *MCPServer) handleToolsCall(ctx context.Context, session *MCPSession, params json.RawMessage) (interface{}, error) {
	var callParams ToolCallParams
	if err := json.Unmarshal(params, &callParams); err != nil {
		return nil, fmt.Errorf("invalid tool call params: %w", err)
	}

	log.Printf("Tool call: %s, session: %s, tenant: %s", callParams.Name, session.ID, session.TenantID)

	tenantID := session.TenantID
	if tenantID == "" {
		return ToolCallResult{
			Content: []Content{
				NewTextContent("Error: tenant not configured for this session"),
			},
			IsError: true,
		}, nil
	}

	// 判断是 Skill 还是 MCP 工具
	if len(callParams.Name) > 6 && callParams.Name[:6] == "skill_" {
		// 执行 Skill
		skillName := callParams.Name[6:] // 去掉 skill_ 前缀
		return s.executeSkillByName(ctx, tenantID, skillName, callParams.Arguments)
	}

	// 执行 MCP 工具
	return s.executeMCPToolByName(ctx, tenantID, callParams.Name, callParams.Arguments)
}

// executeSkillByName 通过名称执行 Skill
func (s *MCPServer) executeSkillByName(ctx context.Context, tenantID, skillName string, args json.RawMessage) (interface{}, error) {
	// 查询 Skill
	var skillRecord struct {
		ID    string `db:"id"`
		Steps string `db:"steps"`
	}
	err := database.DB.Get(&skillRecord, "SELECT id, steps FROM skills WHERE name = ? AND tenant_id = ? AND status = 'active'", skillName, tenantID)
	if err != nil {
		return ToolCallResult{
			Content: []Content{
				NewTextContent(fmt.Sprintf("Skill '%s' not found or not active", skillName)),
			},
			IsError: true,
		}, nil
	}

	// 解析输入参数
	var inputs map[string]interface{}
	if args != nil {
		json.Unmarshal(args, &inputs)
	}
	if inputs == nil {
		inputs = map[string]interface{}{}
	}

	// 解析步骤
	var steps []interface{}
	if err := json.Unmarshal([]byte(skillRecord.Steps), &steps); err != nil {
		return ToolCallResult{
			Content: []Content{
				NewTextContent(fmt.Sprintf("Invalid skill steps: %v", err)),
			},
			IsError: true,
		}, nil
	}

	// 执行 Skill（使用带 MCP 调用能力的引擎）
	proxy := NewMCPProxy(DefaultProxyConfig())
	engine := skill.NewSkillEngineWithCaller(tenantID, func(ctx context.Context, toolName string, arguments json.RawMessage) (map[string]interface{}, error) {
		// 查询工具和连接器
		var mcpTool models.MCPTool
		if err := database.DB.Get(&mcpTool, "SELECT * FROM mcp_tools WHERE name = ? AND tenant_id = ? AND enabled = true", toolName, tenantID); err != nil {
			return nil, fmt.Errorf("MCP tool '%s' not found", toolName)
		}
		var connector models.Connector
		if err := database.DB.Get(&connector, "SELECT * FROM connectors WHERE id = ?", mcpTool.ConnectorID); err != nil {
			return nil, fmt.Errorf("connector not found for tool '%s'", toolName)
		}
		req := ToolCallRequest{
			Tool:      mcpTool,
			Connector: connector,
			Arguments: arguments,
		}
		resp, err := proxy.CallTool(ctx, req)
		if err != nil {
			return nil, err
		}
		if !resp.Success {
			return nil, fmt.Errorf("MCP tool call failed: %s", resp.Error)
		}
		if data, ok := resp.Data.(map[string]interface{}); ok {
			return data, nil
		}
		return map[string]interface{}{"result": resp.Data}, nil
	})
	result, err := engine.ExecuteWithMCP(ctx, tenantID, steps, inputs)
	if err != nil {
		return ToolCallResult{
			Content: []Content{
				NewTextContent(fmt.Sprintf("Skill execution failed: %v", err)),
			},
			IsError: true,
		}, nil
	}

	// 格式化结果
	resultJSON, _ := json.Marshal(result)
	return ToolCallResult{
		Content: []Content{
			NewTextContent(string(resultJSON)),
		},
	}, nil
}

// executeMCPToolByName 通过名称执行 MCP 工具
func (s *MCPServer) executeMCPToolByName(ctx context.Context, tenantID, toolName string, args json.RawMessage) (interface{}, error) {
	// 查询 MCP 工具
	var tool struct {
		ID            string  `db:"id"`
		ConnectorID   string  `db:"connector_id"`
		Name          string  `db:"name"`
		InputSchema   *string `db:"input_schema"`
		BackendMethod *string `db:"backend_method"`
		BackendPath   *string `db:"backend_path"`
	}
	err := database.DB.Get(&tool, "SELECT id, connector_id, name, input_schema, backend_method, backend_path FROM mcp_tools WHERE name = ? AND tenant_id = ? AND enabled = true", toolName, tenantID)
	if err != nil {
		return ToolCallResult{
			Content: []Content{
				NewTextContent(fmt.Sprintf("MCP tool '%s' not found", toolName)),
			},
			IsError: true,
		}, nil
	}

	// 查询连接器
	var connector struct {
		ID      string  `db:"id"`
		BaseURL *string `db:"base_url"`
		Auth    *string `db:"auth"`
	}
	err = database.DB.Get(&connector, "SELECT id, base_url, auth FROM connectors WHERE id = ?", tool.ConnectorID)
	if err != nil {
		return ToolCallResult{
			Content: []Content{
				NewTextContent(fmt.Sprintf("Connector not found for tool '%s'", toolName)),
			},
			IsError: true,
		}, nil
	}

	// 解析参数
	var paramsMap map[string]interface{}
	if args != nil {
		json.Unmarshal(args, &paramsMap)
	}

	// 调用后端API
	proxy := NewMCPProxy(DefaultProxyConfig())

	// 构建 ToolCallRequest
	var argsRaw json.RawMessage
	if paramsMap != nil {
		argsRaw, _ = json.Marshal(paramsMap)
	} else {
		argsRaw = json.RawMessage("{}")
	}

	// 查询完整的 MCP 工具和连接器模型
	var mcpTool models.MCPTool
	err = database.DB.Get(&mcpTool, "SELECT * FROM mcp_tools WHERE name = ? AND tenant_id = ? AND enabled = true", toolName, tenantID)
	if err != nil {
		return ToolCallResult{
			Content: []Content{
				NewTextContent(fmt.Sprintf("MCP tool '%s' not found", toolName)),
			},
			IsError: true,
		}, nil
	}

	var conn models.Connector
	err = database.DB.Get(&conn, "SELECT * FROM connectors WHERE id = ?", tool.ConnectorID)
	if err != nil {
		return ToolCallResult{
			Content: []Content{
				NewTextContent(fmt.Sprintf("Connector not found for tool '%s'", toolName)),
			},
			IsError: true,
		}, nil
	}

	toolCallReq := ToolCallRequest{
		Tool:      mcpTool,
		Connector: conn,
		Arguments: argsRaw,
	}

	result, err := proxy.CallTool(ctx, toolCallReq)
	if err != nil {
		return ToolCallResult{
			Content: []Content{
				NewTextContent(fmt.Sprintf("MCP tool execution failed: %v", err)),
			},
			IsError: true,
		}, nil
	}

	return result, nil
}

// handlePing 处理ping
func (s *MCPServer) handlePing(ctx context.Context, session *MCPSession, params json.RawMessage) (interface{}, error) {
	session.LastPingAt = time.Now()
	return map[string]interface{}{}, nil
}

// RegisterTool 注册工具
func (s *MCPServer) RegisterTool(tenantID string, tool Tool) {
	s.tools.LoadOrStore(tenantID, []Tool{})
	for {
		raw, _ := s.tools.Load(tenantID)
		tools := raw.([]Tool)
		// 检查是否已存在
		for i, t := range tools {
			if t.Name == tool.Name {
				tools[i] = tool
				s.tools.Store(tenantID, tools)
				return
			}
		}
		// 添加新工具
		tools = append(tools, tool)
		if s.tools.CompareAndSwap(tenantID, raw, tools) {
			return
		}
	}
}

// GetSession 获取会话
func (s *MCPServer) GetSession(sessionID string) (*MCPSession, bool) {
	raw, ok := s.sessions.Load(sessionID)
	if !ok {
		return nil, false
	}
	return raw.(*MCPSession), true
}

// GetActiveSessions 获取活跃会话数
func (s *MCPServer) GetActiveSessions() int {
	count := 0
	s.sessions.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}
