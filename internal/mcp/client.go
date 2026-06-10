package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// MCPClient MCP客户端，用于连接外部MCP Server发现工具
type MCPClient struct {
	httpClient *http.Client
}

// NewMCPClient 创建MCP客户端
func NewMCPClient() *MCPClient {
	return &MCPClient{
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// DiscoveredTool 发现的工具
type DiscoveredTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema interface{} `json:"inputSchema,omitempty"`
}

// DiscoverResult 发现结果
type DiscoverResult struct {
	ServerInfo ServerInfo       `json:"server_info"`
	Tools      []DiscoveredTool `json:"tools"`
	Error      string           `json:"error,omitempty"`
}

// DiscoverTools 统一发现入口，根据transportType自动选择传输方式
func (c *MCPClient) DiscoverTools(ctx context.Context, serverURL string, transportType *string, authType *string, authConfig *string, customHeaders *string) (*DiscoverResult, error) {
	tt := "sse"
	if transportType != nil && *transportType != "" {
		tt = *transportType
	}

	log.Printf("MCP Client: discovering tools from %s (transport=%s)", serverURL, tt)

	switch tt {
	case "streamable_http":
		return c.discoverStreamableHTTP(ctx, serverURL, authType, authConfig, customHeaders)
	default: // "sse"
		return c.discoverSSE(ctx, serverURL, authType, authConfig, customHeaders)
	}
}

// ===================== SSE 传输 =====================
// SSE传输: GET SSE → 获取endpoint → POST请求 → 从SSE流读取响应

// discoverSSE 通过SSE连接发现工具
func (c *MCPClient) discoverSSE(ctx context.Context, serverURL string, authType *string, authConfig *string, customHeaders *string) (*DiscoverResult, error) {
	// Step 1: 连接SSE，保持连接打开
	sseCtx, sseCancel := context.WithCancel(ctx)
	defer sseCancel()

	respCh := make(chan *JSONRPCResponse, 10) // SSE响应通道
	endpointCh := make(chan string, 1)         // endpoint URL通道

	go c.connectSSEAndRead(sseCtx, serverURL, authType, authConfig, customHeaders, endpointCh, respCh)

	// Step 2: 等待获取endpoint
	var endpointURL string
	select {
	case endpointURL = <-endpointCh:
		log.Printf("MCP Client: got endpoint %s", endpointURL)
	case <-time.After(15 * time.Second):
		return nil, fmt.Errorf("SSE连接超时，未收到endpoint事件")
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Step 3: 发送initialize请求，从SSE流读取响应
	serverInfo, err := c.sendAndWaitSSE(ctx, endpointURL, MethodInitialize, map[string]interface{}{
		"protocolVersion": MCPVersion,
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "EASP Platform",
			"version": "1.0.0",
		},
	}, authType, authConfig, customHeaders, respCh)
	if err != nil {
		return nil, fmt.Errorf("initialize失败: %w", err)
	}

	// 解析ServerInfo
	var srvInfo ServerInfo
	if si, ok := serverInfo.Result.(map[string]interface{}); ok {
		if sii, ok := si["serverInfo"]; ok {
			b, _ := json.Marshal(sii)
			json.Unmarshal(b, &srvInfo)
		}
	}

	// Step 4: 发送initialized通知（不需要响应）
	c.sendNotify(ctx, endpointURL, MethodInitialized, nil, authType, authConfig, customHeaders)

	// Step 5: 发送tools/list请求
	toolsResp, err := c.sendAndWaitSSE(ctx, endpointURL, MethodToolsList, nil, authType, authConfig, customHeaders, respCh)
	if err != nil {
		return nil, fmt.Errorf("tools/list失败: %w", err)
	}

	// 解析Tools
	var tools []DiscoveredTool
	if tr, ok := toolsResp.Result.(map[string]interface{}); ok {
		if tl, ok := tr["tools"]; ok {
			b, _ := json.Marshal(tl)
			json.Unmarshal(b, &tools)
		}
	}

	return &DiscoverResult{
		ServerInfo: srvInfo,
		Tools:      tools,
	}, nil
}

// connectSSEAndRead 连接SSE并持续读取事件，将JSON-RPC响应发送到通道
func (c *MCPClient) connectSSEAndRead(ctx context.Context, serverURL string, authType *string, authConfig *string, customHeaders *string, endpointCh chan<- string, respCh chan<- *JSONRPCResponse) {
	defer close(respCh)

	req, err := http.NewRequestWithContext(ctx, "GET", serverURL, nil)
	if err != nil {
		log.Printf("MCP SSE: create request error: %v", err)
		return
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	applyAuth(req, authType, authConfig, customHeaders)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("MCP SSE: connect error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("MCP SSE: unexpected status %d", resp.StatusCode)
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var currentEvent string
	endpointSent := false

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()

		// 解析 event 行
		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
			continue
		}

		// 解析 data 行
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			data = strings.TrimSpace(data)

			switch currentEvent {
			case "endpoint":
				// 发送endpoint到通道
				if !endpointSent {
					endpointURL := data
					if !strings.HasPrefix(endpointURL, "http") {
						// 相对路径，拼接base URL
						base := strings.TrimSuffix(serverURL, "/")
						if idx := strings.LastIndex(base, "/"); idx > 0 {
							endpointURL = base[:idx] + endpointURL
						}
					}
					endpointCh <- endpointURL
					endpointSent = true
				}

			case "message":
				// 解析JSON-RPC响应
				var rpcResp JSONRPCResponse
				if err := json.Unmarshal([]byte(data), &rpcResp); err == nil && rpcResp.JSONRPC != "" {
					select {
					case respCh <- &rpcResp:
						log.Printf("MCP SSE: received response id=%v", rpcResp.ID)
					default:
						log.Printf("MCP SSE: response channel full, dropping")
					}
				}

			default:
				// 尝试解析为JSON-RPC响应（有些服务器不发送event类型）
				var rpcResp JSONRPCResponse
				if err := json.Unmarshal([]byte(data), &rpcResp); err == nil && rpcResp.JSONRPC != "" {
					select {
					case respCh <- &rpcResp:
						log.Printf("MCP SSE: received response (no event) id=%v", rpcResp.ID)
					default:
						log.Printf("MCP SSE: response channel full, dropping")
					}
				}
			}

			currentEvent = ""
		}
	}

	log.Printf("MCP SSE: connection closed")
}

// sendAndWaitSSE 发送请求并等待SSE响应
func (c *MCPClient) sendAndWaitSSE(ctx context.Context, endpointURL, method string, params interface{}, authType *string, authConfig *string, customHeaders *string, respCh <-chan *JSONRPCResponse) (*JSONRPCResponse, error) {
	reqBody := map[string]interface{}{
		"jsonrpc": JSONRPCVersion,
		"id":      1,
		"method":  method,
	}
	if params != nil {
		reqBody["params"] = params
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpointURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	applyAuth(req, authType, authConfig, customHeaders)

	log.Printf("MCP Client SSE RPC: %s → %s", method, endpointURL)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// POST响应可能是202 Accepted或200
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("POST返回 HTTP %d: %s", resp.StatusCode, string(body))
	}

	// 从SSE通道等待响应（带超时）
	select {
	case rpcResp := <-respCh:
		if rpcResp == nil {
			return nil, fmt.Errorf("SSE连接已关闭")
		}
		if rpcResp.Error != nil {
			return nil, fmt.Errorf("RPC error: %s", rpcResp.Error.Message)
		}
		return rpcResp, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("等待SSE响应超时")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// sendNotify 发送JSON-RPC通知（不需要响应）
func (c *MCPClient) sendNotify(ctx context.Context, endpointURL, method string, params interface{}, authType *string, authConfig *string, customHeaders *string) {
	reqBody := map[string]interface{}{
		"jsonrpc": JSONRPCVersion,
		"method":  method,
	}
	if params != nil {
		reqBody["params"] = params
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", endpointURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	applyAuth(req, authType, authConfig, customHeaders)

	c.httpClient.Do(req) // 忽略响应
}

// ===================== StreamableHTTP 传输 =====================

// discoverStreamableHTTP 通过StreamableHTTP发现工具
func (c *MCPClient) discoverStreamableHTTP(ctx context.Context, serverURL string, authType *string, authConfig *string, customHeaders *string) (*DiscoverResult, error) {
	// Step 1: initialize
	serverInfo, err := c.httpDoRPC(ctx, serverURL, MethodInitialize, map[string]interface{}{
		"protocolVersion": MCPVersion,
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "EASP Platform",
			"version": "1.0.0",
		},
	}, authType, authConfig, customHeaders)
	if err != nil {
		return nil, fmt.Errorf("initialize失败: %w", err)
	}

	// Step 2: initialized 通知
	c.httpDoNotify(ctx, serverURL, MethodInitialized, nil, authType, authConfig, customHeaders)

	// Step 3: tools/list
	toolsResp, err := c.httpDoRPC(ctx, serverURL, MethodToolsList, nil, authType, authConfig, customHeaders)
	if err != nil {
		return nil, fmt.Errorf("tools/list失败: %w", err)
	}

	// 解析 ServerInfo
	var srvInfo ServerInfo
	if si, ok := serverInfo.Result.(map[string]interface{}); ok {
		if sii, ok := si["serverInfo"]; ok {
			b, _ := json.Marshal(sii)
			json.Unmarshal(b, &srvInfo)
		}
	}

	// 解析 Tools
	var tools []DiscoveredTool
	if tr, ok := toolsResp.Result.(map[string]interface{}); ok {
		if tl, ok := tr["tools"]; ok {
			b, _ := json.Marshal(tl)
			json.Unmarshal(b, &tools)
		}
	}

	return &DiscoverResult{
		ServerInfo: srvInfo,
		Tools:      tools,
	}, nil
}

// httpDoRPC 通过StreamableHTTP发送JSON-RPC请求
func (c *MCPClient) httpDoRPC(ctx context.Context, serverURL, method string, params interface{}, authType *string, authConfig *string, customHeaders *string) (*JSONRPCResponse, error) {
	reqBody := map[string]interface{}{
		"jsonrpc": JSONRPCVersion,
		"id":      1,
		"method":  method,
	}
	if params != nil {
		reqBody["params"] = params
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", serverURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	applyAuth(req, authType, authConfig, customHeaders)

	log.Printf("MCP Client HTTP RPC: %s → %s", method, serverURL)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusAccepted {
		// 202 Accepted: 通知已收到
		return &JSONRPCResponse{JSONRPC: JSONRPCVersion}, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP RPC返回 %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 尝试直接解析JSON
	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(body, &rpcResp); err == nil && rpcResp.JSONRPC != "" {
		return &rpcResp, nil
	}

	// 尝试SSE格式解析
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			dataStr := strings.TrimPrefix(line, "data: ")
			if err := json.Unmarshal([]byte(dataStr), &rpcResp); err == nil && rpcResp.JSONRPC != "" {
				return &rpcResp, nil
			}
		}
	}

	return nil, fmt.Errorf("未收到有效JSON-RPC响应 (body=%s)", string(body))
}

// httpDoNotify 通过StreamableHTTP发送JSON-RPC通知
func (c *MCPClient) httpDoNotify(ctx context.Context, serverURL, method string, params interface{}, authType *string, authConfig *string, customHeaders *string) {
	reqBody := map[string]interface{}{
		"jsonrpc": JSONRPCVersion,
		"method":  method,
	}
	if params != nil {
		reqBody["params"] = params
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", serverURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	applyAuth(req, authType, authConfig, customHeaders)

	c.httpClient.Do(req)
}

// ===================== 公共工具函数 =====================

// applyAuth 统一应用认证和自定义头
func applyAuth(req *http.Request, authType *string, authConfig *string, customHeaders *string) {
	// 1. 认证头
	if authType != nil && authConfig != nil {
		var config map[string]interface{}
		if err := json.Unmarshal([]byte(*authConfig), &config); err == nil {
			switch *authType {
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
	}

	// 2. 自定义头
	if customHeaders != nil && *customHeaders != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(*customHeaders), &headers); err == nil {
			for k, v := range headers {
				req.Header.Set(k, v)
			}
		}
	}
}

// ===================== 全局默认客户端 =====================

var defaultClient = NewMCPClient()
var clientOnce sync.Once

func GetDefaultClient() *MCPClient {
	clientOnce.Do(func() {
		defaultClient = NewMCPClient()
	})
	return defaultClient
}
