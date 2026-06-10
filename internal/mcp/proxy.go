package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/easp-platform/easp/internal/models"
	"github.com/easp-platform/easp/internal/resilience"
)

// ProxyConfig 代理配置
type ProxyConfig struct {
	Timeout         time.Duration
	MaxRetries      int
	RetryDelay      time.Duration
	CircuitBreaker  *resilience.CircuitBreaker
	RateLimiter     resilience.RateLimiter
}

// DefaultProxyConfig 默认代理配置
func DefaultProxyConfig() ProxyConfig {
	return ProxyConfig{
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		RetryDelay: 1 * time.Second,
	}
}

// MCPProxy MCP代理
type MCPProxy struct {
	client    *http.Client
	mcpClient *MCPClient
	config    ProxyConfig
	breaker   *resilience.CircuitBreakerManager
	limiter   *resilience.RateLimiterManager
}

// NewMCPProxy 创建MCP代理
func NewMCPProxy(config ProxyConfig) *MCPProxy {
	return &MCPProxy{
		client: &http.Client{
			Timeout: config.Timeout,
		},
		mcpClient: NewMCPClient(),
		config:    config,
		breaker:   resilience.NewCircuitBreakerManager(resilience.DefaultCircuitBreakerConfig()),
		limiter:   resilience.NewRateLimiterManager(),
	}
}

// ToolCallRequest 工具调用请求
type ToolCallRequest struct {
	Tool      models.MCPTool   `json:"tool"`
	Connector models.Connector `json:"connector"`
	Arguments json.RawMessage  `json:"arguments"`
}

// ToolCallResponse 工具调用响应
type ToolCallResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Latency int64       `json:"latency_ms"`
}

// CallTool 调用工具
func (p *MCPProxy) CallTool(ctx context.Context, req ToolCallRequest) (*ToolCallResponse, error) {
	start := time.Now()

	// 获取熔断器
	cbName := fmt.Sprintf("%s_%s", req.Connector.ID, req.Tool.Name)
	cb := p.breaker.GetOrCreate(cbName)

	// 获取限流器
	limiterName := fmt.Sprintf("tenant_%s", req.Connector.TenantID)
	limiter, _ := p.limiter.Get(limiterName)

	// 检查限流
	if limiter != nil && !limiter.Allow() {
		return &ToolCallResponse{
			Success: false,
			Error:   "rate limit exceeded",
			Latency: time.Since(start).Milliseconds(),
		}, nil
	}

	// 通过熔断器执行
	var resp *ToolCallResponse
	err := cb.Execute(func() error {
		var execErr error
		resp, execErr = p.executeTool(ctx, req)
		return execErr
	})

	if err != nil {
		return &ToolCallResponse{
			Success: false,
			Error:   err.Error(),
			Latency: time.Since(start).Milliseconds(),
		}, nil
	}

	resp.Latency = time.Since(start).Milliseconds()
	return resp, nil
}

// executeTool 执行工具调用
func (p *MCPProxy) executeTool(ctx context.Context, req ToolCallRequest) (*ToolCallResponse, error) {
	// 如果连接器配置了MCP Server URL，走MCP协议调用
	if req.Connector.MCPServerURL != nil && *req.Connector.MCPServerURL != "" {
		return p.executeMCPTool(ctx, req)
	}

	// 否则走REST API调用
	method := "GET"
	if req.Tool.BackendMethod != nil {
		method = *req.Tool.BackendMethod
	}

	path := ""
	if req.Tool.BackendPath != nil {
		path = *req.Tool.BackendPath
	}

	// 构建请求URL
	baseURL := strings.TrimRight(req.Connector.BaseURL, "/")
	url := baseURL + path

	// 解析参数
	var args map[string]interface{}
	if req.Arguments != nil {
		if err := json.Unmarshal(req.Arguments, &args); err != nil {
			return nil, fmt.Errorf("failed to parse arguments: %w", err)
		}
	}

	// 替换路径参数
	url = replacePathParams(url, args)

	// 构建请求体
	var body io.Reader
	if method == "POST" || method == "PUT" || method == "PATCH" {
		if args != nil {
			bodyBytes, err := json.Marshal(args)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal request body: %w", err)
			}
			body = bytes.NewReader(bodyBytes)
		}
	}

	// 创建HTTP请求
	httpReq, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")

	// 添加认证信息
	if req.Connector.AuthType != nil && req.Connector.AuthConfig != nil {
		addAuthHeader(httpReq, *req.Connector.AuthType, *req.Connector.AuthConfig)
	}

	// 发送请求
	log.Printf("MCP Proxy: %s %s", method, url)
	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// 解析响应
	var data interface{}
	if err := json.Unmarshal(respBody, &data); err != nil {
		// 如果不是JSON，返回原始文本
		data = string(respBody)
	}

	if httpResp.StatusCode >= 400 {
		return &ToolCallResponse{
			Success: false,
			Data:    data,
			Error:   fmt.Sprintf("HTTP %d: %s", httpResp.StatusCode, string(respBody)),
		}, nil
	}

	return &ToolCallResponse{
		Success: true,
		Data:    data,
	}, nil
}

// executeMCPTool 通过MCP协议（JSON-RPC）调用工具
func (p *MCPProxy) executeMCPTool(ctx context.Context, req ToolCallRequest) (*ToolCallResponse, error) {
	serverURL := *req.Connector.MCPServerURL
	transportType := "sse"
	if req.Connector.TransportType != nil && *req.Connector.TransportType != "" {
		transportType = *req.Connector.TransportType
	}

	// 解析参数
	var args map[string]interface{}
	if req.Arguments != nil {
		if err := json.Unmarshal(req.Arguments, &args); err != nil {
			args = nil
		}
	}

	log.Printf("MCP Proxy: calling tool %s via %s on %s", req.Tool.Name, transportType, serverURL)

	if transportType == "streamable_http" {
		return p.callToolStreamableHTTP(ctx, serverURL, req.Tool.Name, args, req.Connector)
	}

	// 默认走SSE
	return p.callToolSSE(ctx, serverURL, req.Tool.Name, args, req.Connector)
}

// callToolSSE 通过SSE传输调用MCP工具
func (p *MCPProxy) callToolSSE(ctx context.Context, serverURL, toolName string, args map[string]interface{}, connector models.Connector) (*ToolCallResponse, error) {
	start := time.Now()
	respCh := make(chan *JSONRPCResponse, 10)
	endpointCh := make(chan string, 1)

	// 启动SSE连接
	go p.mcpClient.connectSSEAndRead(ctx, serverURL, connector.AuthType, connector.AuthConfig, connector.Headers, endpointCh, respCh)

	// 等待获取endpoint
	var endpointURL string
	select {
	case ep := <-endpointCh:
		endpointURL = ep
	case <-time.After(15 * time.Second):
		return &ToolCallResponse{Success: false, Error: "SSE connect timeout", Latency: time.Since(start).Milliseconds()}, nil
	case <-ctx.Done():
		return &ToolCallResponse{Success: false, Error: "context cancelled", Latency: time.Since(start).Milliseconds()}, nil
	}

	// 发送initialize
	initResp, err := p.mcpClient.sendAndWaitSSE(ctx, endpointURL, "initialize", map[string]interface{}{
		"protocolVersion": JSONRPCVersion,
		"capabilities":    map[string]interface{}{},
		"clientInfo":      map[string]interface{}{"name": "easp-proxy", "version": "1.0.0"},
	}, connector.AuthType, connector.AuthConfig, connector.Headers, respCh)
	if err != nil {
		return &ToolCallResponse{Success: false, Error: fmt.Sprintf("initialize failed: %v", err), Latency: time.Since(start).Milliseconds()}, nil
	}
	if initResp.Error != nil {
		return &ToolCallResponse{Success: false, Error: fmt.Sprintf("initialize error: %v", initResp.Error), Latency: time.Since(start).Milliseconds()}, nil
	}

	// 发送 initialized 通知
	p.mcpClient.sendNotify(ctx, endpointURL, "notifications/initialized", nil, connector.AuthType, connector.AuthConfig, connector.Headers)

	// 调用工具
	callResp, err := p.mcpClient.sendAndWaitSSE(ctx, endpointURL, "tools/call", map[string]interface{}{
		"name":      toolName,
		"arguments": args,
	}, connector.AuthType, connector.AuthConfig, connector.Headers, respCh)
	if err != nil {
		return &ToolCallResponse{Success: false, Error: fmt.Sprintf("tools/call failed: %v", err), Latency: time.Since(start).Milliseconds()}, nil
	}
	if callResp.Error != nil {
		return &ToolCallResponse{Success: false, Error: fmt.Sprintf("tools/call error: %v", callResp.Error), Latency: time.Since(start).Milliseconds()}, nil
	}

	return &ToolCallResponse{Success: true, Data: callResp.Result, Latency: time.Since(start).Milliseconds()}, nil
}

// callToolStreamableHTTP 通过StreamableHTTP传输调用MCP工具
func (p *MCPProxy) callToolStreamableHTTP(ctx context.Context, serverURL, toolName string, args map[string]interface{}, connector models.Connector) (*ToolCallResponse, error) {
	start := time.Now()

	// initialize
	_, err := p.mcpClient.httpDoRPC(ctx, serverURL, "initialize", map[string]interface{}{
		"protocolVersion": JSONRPCVersion,
		"capabilities":    map[string]interface{}{},
		"clientInfo":      map[string]interface{}{"name": "easp-proxy", "version": "1.0.0"},
	}, connector.AuthType, connector.AuthConfig, connector.Headers)
	if err != nil {
		return &ToolCallResponse{Success: false, Error: fmt.Sprintf("initialize failed: %v", err), Latency: time.Since(start).Milliseconds()}, nil
	}

	// 调用工具
	callResp, err := p.mcpClient.httpDoRPC(ctx, serverURL, "tools/call", map[string]interface{}{
		"name":      toolName,
		"arguments": args,
	}, connector.AuthType, connector.AuthConfig, connector.Headers)
	if err != nil {
		return &ToolCallResponse{Success: false, Error: fmt.Sprintf("tools/call failed: %v", err), Latency: time.Since(start).Milliseconds()}, nil
	}
	if callResp.Error != nil {
		return &ToolCallResponse{Success: false, Error: fmt.Sprintf("tools/call error: %v", callResp.Error), Latency: time.Since(start).Milliseconds()}, nil
	}

	return &ToolCallResponse{Success: true, Data: callResp.Result, Latency: time.Since(start).Milliseconds()}, nil
}

// replacePathParams 替换路径参数
func replacePathParams(url string, args map[string]interface{}) string {
	if args == nil {
		return url
	}

	for key, value := range args {
		placeholder := "{" + key + "}"
		if strings.Contains(url, placeholder) {
			url = strings.ReplaceAll(url, placeholder, fmt.Sprintf("%v", value))
			delete(args, key) // 从参数中移除已替换的路径参数
		}
	}

	return url
}

// addAuthHeader 添加认证头
func addAuthHeader(req *http.Request, authType, authConfig string) {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(authConfig), &config); err != nil {
		return
	}

	switch authType {
	case "bearer":
		if token, ok := config["token"].(string); ok {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	case "api_key":
		if key, ok := config["key"].(string); ok {
			headerName := "X-API-Key"
			if name, ok := config["header"].(string); ok {
				headerName = name
			}
			req.Header.Set(headerName, key)
		}
	case "basic":
		if username, ok := config["username"].(string); ok {
			if password, ok := config["password"].(string); ok {
				req.SetBasicAuth(username, password)
			}
		}
	}
}

// GetCircuitBreakerStats 获取熔断器统计
func (p *MCPProxy) GetCircuitBreakerStats() map[string]interface{} {
	return p.breaker.GetAll()
}

// GetRateLimiterStats 获取限流器统计
func (p *MCPProxy) GetRateLimiterStats() map[string]interface{} {
	return p.limiter.GetAll()
}

// RegisterTenantLimiter 注册租户限流器
func (p *MCPProxy) RegisterTenantLimiter(tenantID string, rate float64, capacity int) {
	name := fmt.Sprintf("tenant_%s", tenantID)
	limiter := resilience.NewTokenBucketLimiter(name, rate, capacity)
	p.limiter.Register(name, limiter)
}
