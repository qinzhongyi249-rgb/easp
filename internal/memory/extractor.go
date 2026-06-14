package memory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MemoryExtractor 记忆提取器
// 从对话历史中提取有价值的信息，保存为用户记忆和实体
type MemoryExtractor struct {
	memorySvc *MemoryService
	modelCfg  ModelConfig
}

// ModelConfig 模型配置（用于调用LLM提取记忆）
type ModelConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

// ExtractRequest 提取请求
type ExtractRequest struct {
	TenantID string           `json:"tenant_id"`
	UserID   string           `json:"user_id"`
	Messages []ExtractMessage `json:"messages"`
}

// ExtractMessage 对话消息
type ExtractMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ExtractResult 提取结果
type ExtractResult struct {
	Memories []ExtractedMemory `json:"memories"`
	Entities []ExtractedEntity `json:"entities"`
}

// ExtractedMemory 提取的记忆
type ExtractedMemory struct {
	Type    string `json:"type"` // preference/fact
	Content string `json:"content"`
}

// ExtractedEntity 提取的实体
type ExtractedEntity struct {
	Name string `json:"name"`
	Type string `json:"type"` // tenant/user/connector/tool/skill
}

// NewMemoryExtractor 创建记忆提取器
func NewMemoryExtractor(memorySvc *MemoryService, modelCfg ModelConfig) *MemoryExtractor {
	return &MemoryExtractor{
		memorySvc: memorySvc,
		modelCfg:  modelCfg,
	}
}

// ExtractAndSave 从对话中提取记忆并保存
func (e *MemoryExtractor) ExtractAndSave(req ExtractRequest) {
	if len(req.Messages) < 2 {
		return // 对话太短，不需要提取
	}
	if e.memorySvc != nil {
		settings := e.memorySvc.GetMemorySettings(req.TenantID, req.UserID)
		if !settings.AutoExtractEnabled {
			log.Printf("MemoryExtractor: auto extract disabled for tenant=%s user=%s", req.TenantID, req.UserID)
			return
		}
	}

	result, err := e.extract(req.Messages)
	if err != nil {
		log.Printf("MemoryExtractor: failed to extract: %v", err)
		return
	}

	// 保存提取的记忆
	for _, mem := range result.Memories {
		if mem.Content == "" || mem.Type == "" {
			continue
		}
		_, err := e.memorySvc.SaveUserMemory(req.TenantID, req.UserID, mem.Type, mem.Content, map[string]interface{}{
			"source": "auto_extract",
		})
		if err != nil {
			log.Printf("MemoryExtractor: failed to save memory: %v", err)
		}
	}

	// 保存提取的实体
	for _, ent := range result.Entities {
		if ent.Name == "" || ent.Type == "" {
			continue
		}
		_, err := e.memorySvc.SaveEntity(req.TenantID, ent.Name, ent.Type, "", map[string]interface{}{
			"source": "auto_extract",
		})
		if err != nil {
			log.Printf("MemoryExtractor: failed to save entity: %v", err)
		}
	}

	if len(result.Memories) > 0 || len(result.Entities) > 0 {
		log.Printf("MemoryExtractor: extracted %d memories, %d entities for user %s",
			len(result.Memories), len(result.Entities), req.UserID)
	}
}

// extract 调用LLM提取记忆
func (e *MemoryExtractor) extract(messages []ExtractMessage) (*ExtractResult, error) {
	// 构建对话文本
	var convBuilder strings.Builder
	for _, m := range messages {
		role := "用户"
		if m.Role == "assistant" {
			role = "助手"
		}
		convBuilder.WriteString(fmt.Sprintf("%s: %s\n", role, m.Content))
	}

	prompt := fmt.Sprintf(`从以下对话中提取有价值的信息。只提取新的、持久性的信息，不要提取临时性的操作结果。

对话内容:
%s

请以JSON格式输出提取结果:
{
  "memories": [
    {"type": "preference", "content": "用户偏好信息"},
    {"type": "fact", "content": "用户身份/背景/事实信息"}
  ],
  "entities": [
    {"name": "实体名称", "type": "connector|tool|skill|user"}
  ]
}

规则:
1. preference: 用户的习惯、偏好、风格（如"喜欢简洁回答"、"常用XX模型"）
2. fact: 用户的身份、职责、背景（如"负责管理MCP工具"）
3. entities: 对话中明确提到的平台资源（连接器、工具、Skill等）
4. 不要提取临时性信息（如查询结果、操作反馈）
5. 不要重复已知信息
6. 如果没有值得提取的信息，返回空数组
7. 只返回JSON，不要其他内容`, convBuilder.String())

	reqBody := map[string]any{
		"model": e.modelCfg.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.1,
		"max_tokens":  500,
	}

	body, _ := json.Marshal(reqBody)
	httpReq, _ := http.NewRequest("POST", e.modelCfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+e.modelCfg.APIKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("LLM error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var respData struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(respData.Choices) == 0 {
		return &ExtractResult{}, nil
	}

	// 解析LLM输出的JSON
	content := respData.Choices[0].Message.Content
	content = strings.TrimSpace(content)

	// 去掉可能的markdown代码块标记
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var result ExtractResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		log.Printf("MemoryExtractor: failed to parse LLM output: %v, content: %s", err, content)
		return &ExtractResult{}, nil
	}

	return &result, nil
}

// GenerateSessionID 生成会话ID
func GenerateSessionID() string {
	return uuid.New().String()
}
