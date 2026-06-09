package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

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

// handleToolsList 处理工具列表
func (s *MCPServer) handleToolsList(ctx context.Context, session *MCPSession, params json.RawMessage) (interface{}, error) {
	// 从params中获取tenantID（如果有）
	var listParams struct {
		Cursor string `json:"cursor,omitempty"`
	}
	if params != nil {
		json.Unmarshal(params, &listParams)
	}

	// 返回注册的工具列表
	// 实际实现中需要从数据库查询
	return map[string]interface{}{
		"tools": []Tool{},
	}, nil
}

// handleToolsCall 处理工具调用
func (s *MCPServer) handleToolsCall(ctx context.Context, session *MCPSession, params json.RawMessage) (interface{}, error) {
	var callParams ToolCallParams
	if err := json.Unmarshal(params, &callParams); err != nil {
		return nil, fmt.Errorf("invalid tool call params: %w", err)
	}

	log.Printf("Tool call: %s, session: %s", callParams.Name, session.ID)

	// TODO: 实现实际的工具调用逻辑
	// 1. 查找工具定义
	// 2. 验证参数
	// 3. 调用后端API
	// 4. 返回结果

	return ToolCallResult{
		Content: []Content{
			NewTextContent(fmt.Sprintf("Tool %s executed successfully", callParams.Name)),
		},
	}, nil
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
