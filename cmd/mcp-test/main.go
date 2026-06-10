package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func main() {
	baseURL := "http://localhost:8082"
	tenantID := "00000000-0000-0000-0000-000000000001"

	// 1. Login
	token := login(baseURL)
	fmt.Printf("✅ Login OK\n")

	// 2. Connect SSE (keep connection open)
	req, _ := http.NewRequest("GET", baseURL+"/api/v1/mcp/"+tenantID+"/sse", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("SSE error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// Read SSE events to get session ID
	sessionID := ""
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Printf("SSE: %s\n", line)
		if strings.HasPrefix(line, "data:") {
			endpoint := strings.TrimPrefix(line, "data: ")
			parts := strings.Split(endpoint, "sessionId=")
			if len(parts) > 1 {
				sessionID = parts[1]
				break
			}
		}
	}
	if sessionID == "" {
		fmt.Printf("❌ Failed to get session ID\n")
		return
	}
	fmt.Printf("✅ SSE Session: %s\n", sessionID)

	// 3. Initialize
	result := jsonRPC(baseURL, tenantID, sessionID, token, "initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo":      map[string]interface{}{"name": "test-client", "version": "1.0"},
	})
	if result != nil {
		fmt.Printf("✅ Initialize OK\n")
	}

	// 4. tools/list
	toolsResult := jsonRPC(baseURL, tenantID, sessionID, token, "tools/list", nil)
	if toolsResult == nil {
		fmt.Printf("❌ tools/list failed\n")
		return
	}
	tools := toolsResult["tools"].([]interface{})
	fmt.Printf("✅ tools/list: %d tools\n", len(tools))

	// 分类统计
	mcpCount := 0
	skillCount := 0
	for _, t := range tools {
		tool := t.(map[string]interface{})
		name := tool["name"].(string)
		if strings.HasPrefix(name, "skill_") {
			skillCount++
			desc := ""
			if tool["description"] != nil {
				desc = tool["description"].(string)
			}
			fmt.Printf("   🎯 [Skill] %s - %s\n", name, desc)
		} else {
			mcpCount++
		}
	}
	fmt.Printf("   📦 MCP工具: %d, Skills: %d\n", mcpCount, skillCount)

	// 5. tools/call (Skill)
	if skillCount > 0 {
		fmt.Printf("\n=== 测试调用 Skill ===\n")
		callResult := jsonRPC(baseURL, tenantID, sessionID, token, "tools/call", map[string]interface{}{
			"name":      "skill_用户管理",
			"arguments": map[string]interface{}{},
		})
		if callResult != nil {
			content := callResult["content"].([]interface{})
			if len(content) > 0 {
				text := content[0].(map[string]interface{})["text"].(string)
				if len(text) > 300 {
					text = text[:300] + "..."
				}
				fmt.Printf("✅ skill_用户管理 执行结果:\n%s\n", text)
			}
		}
	}

	fmt.Printf("\n=== 验证完成 ===\n")
}

func login(baseURL string) string {
	body := `{"email":"admin@easp.com","password":"admin123"}`
	resp, _ := http.Post(baseURL+"/api/v1/auth/login", "application/json", bytes.NewBufferString(body))
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	tokens := result["tokens"].(map[string]interface{})
	return tokens["access_token"].(string)
}

func jsonRPC(baseURL, tenantID, sessionID, token, method string, params interface{}) map[string]interface{} {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
	}
	if params != nil {
		reqBody["params"] = params
	}
	body, _ := json.Marshal(reqBody)

	url := fmt.Sprintf("%s/api/v1/mcp/%s/message?sessionId=%s", baseURL, tenantID, sessionID)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("JSON-RPC error: %v\n", err)
		return nil
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)
	if result["result"] != nil {
		return result["result"].(map[string]interface{})
	}
	fmt.Printf("JSON-RPC error: %s\n", string(respBody))
	return nil
}
