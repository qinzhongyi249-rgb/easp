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

	// 2. Connect SSE
	sessionID := connectSSE(baseURL, tenantID, token)
	if sessionID == "" {
		fmt.Printf("❌ SSE failed\n")
		return
	}
	fmt.Printf("✅ SSE Session: %s\n", sessionID)

	// 3. Initialize
	initResult := jsonRPC(baseURL, tenantID, sessionID, token, "initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo":      map[string]interface{}{"name": "e2e-test", "version": "1.0"},
	})
	if initResult == nil {
		fmt.Printf("❌ Initialize failed\n")
		return
	}
	fmt.Printf("✅ Initialize OK\n")

	// 4. tools/list
	toolsResult := jsonRPC(baseURL, tenantID, sessionID, token, "tools/list", nil)
	if toolsResult == nil {
		fmt.Printf("❌ tools/list failed\n")
		return
	}
	tools := toolsResult["tools"].([]interface{})
	mcpCount, skillCount := 0, 0
	for _, t := range tools {
		name := t.(map[string]interface{})["name"].(string)
		if strings.HasPrefix(name, "skill_") {
			skillCount++
			fmt.Printf("   🎯 %s\n", name)
		} else {
			mcpCount++
		}
	}
	fmt.Printf("✅ tools/list: %d total (MCP: %d, Skills: %d)\n", len(tools), mcpCount, skillCount)

	// 5. tools/call - 测试 MCP 工具
	fmt.Printf("\n=== 测试 MCP 工具调用 ===\n")
	mcpResult := jsonRPC(baseURL, tenantID, sessionID, token, "tools/call", map[string]interface{}{
		"name":      "get_time_current",
		"arguments": map[string]interface{}{"timezone": "Asia/Shanghai"},
	})
	if mcpResult != nil {
		content := mcpResult["content"].([]interface{})
		if len(content) > 0 {
			text := content[0].(map[string]interface{})["text"].(string)
			if len(text) > 100 {
				text = text[:100] + "..."
			}
			fmt.Printf("✅ get_time_current: %s\n", text)
		}
	} else {
		fmt.Printf("⚠️ get_time_current 调用失败\n")
	}

	// 6. tools/call - 测试 Skill
	fmt.Printf("\n=== 测试 Skill 调用 ===\n")
	skillResult := jsonRPC(baseURL, tenantID, sessionID, token, "tools/call", map[string]interface{}{
		"name":      "skill_用户管理",
		"arguments": map[string]interface{}{},
	})
	if skillResult != nil {
		content := skillResult["content"].([]interface{})
		if len(content) > 0 {
			text := content[0].(map[string]interface{})["text"].(string)
			if len(text) > 200 {
				text = text[:200] + "..."
			}
			fmt.Printf("✅ skill_用户管理: %s\n", text)
		}
	} else {
		fmt.Printf("⚠️ skill_用户管理 调用超时或失败\n")
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

func connectSSE(baseURL, tenantID, token string) string {
	req, _ := http.NewRequest("GET", baseURL+"/api/v1/mcp/"+tenantID+"/sse", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	// Don't close - keep session alive
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			endpoint := strings.TrimPrefix(line, "data: ")
			parts := strings.Split(endpoint, "sessionId=")
			if len(parts) > 1 {
				return parts[1]
			}
		}
	}
	return ""
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

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)
	if result["result"] != nil {
		return result["result"].(map[string]interface{})
	}
	return nil
}
