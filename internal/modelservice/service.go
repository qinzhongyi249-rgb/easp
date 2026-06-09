package modelservice

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/easp-platform/easp/internal/models"
	"github.com/easp-platform/easp/internal/repositories"
)

// Config 模型服务配置
type Config struct {
	BaseURL      string  `json:"base_url"`
	APIKey       string  `json:"api_key"`
	Model        string  `json:"model"`
	ProviderName string  `json:"provider_name"`
	DisplayName  string  `json:"display_name"`
	Temperature  float64 `json:"temperature"`
	MaxTokens    int     `json:"max_tokens"`
}

// ToolCall 工具调用
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Message 消息结构
type Message struct {
	Role       string     `json:"role,omitempty"`
	Content    string     `json:"content"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream"`
}

// ChatResponse 聊天响应
type ChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// StreamChunk 流式响应块
type StreamChunk struct {
	Content string
	Done    bool
	Error   error
}

// ModelService 模型服务
type ModelService struct {
	config        Config
	httpClient    *http.Client
	configRepo    *repositories.ModelConfigRepository
	providerRepo  *repositories.ModelProviderRepository
}

// NewModelService 创建模型服务
func NewModelService(config Config) *ModelService {
	return &ModelService{
		config: config,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		configRepo:   repositories.NewModelConfigRepository(),
		providerRepo: repositories.NewModelProviderRepository(),
	}
}

// GetConfigForTenant 获取租户的模型配置
func (s *ModelService) GetConfigForTenant(tenantID string, modelName string) (*Config, error) {
	// 如果指定了模型名称
	if modelName != "" {
		config, err := s.configRepo.GetByTenantAndModel(tenantID, modelName)
		if err != nil {
			return nil, fmt.Errorf("model config not found: %w", err)
		}
		return s.buildConfig(config)
	}
	
	// 获取默认配置
	config, err := s.configRepo.GetDefault(tenantID)
	if err != nil {
		return nil, fmt.Errorf("no default model configured: %w", err)
	}
	return s.buildConfig(config)
}

// buildConfig 从数据库配置构建服务配置
func (s *ModelService) buildConfig(config *models.ModelConfig) (*Config, error) {
	// 获取提供商信息
	provider, err := s.providerRepo.GetByID(config.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("provider not found: %w", err)
	}
	
	if !provider.Enabled {
		return nil, fmt.Errorf("provider is disabled")
	}
	
	return &Config{
		BaseURL:      provider.BaseURL,
		APIKey:       provider.APIKey,
		Model:        config.ModelName,
		ProviderName: provider.Name,
		DisplayName:  config.DisplayName,
		Temperature:  config.Temperature,
		MaxTokens:    config.MaxTokens,
	}, nil
}

// ChatWithTenant 使用租户配置进行聊天
func (s *ModelService) ChatWithTenant(tenantID string, modelName string, messages []Message) (string, error) {
	config, err := s.GetConfigForTenant(tenantID, modelName)
	if err != nil {
		return "", err
	}
	
	return s.chatWithConfig(config, messages)
}

// ChatWithTenantStream 使用租户配置进行流式聊天
func (s *ModelService) ChatWithTenantStream(tenantID string, modelName string, messages []Message) (<-chan StreamChunk, error) {
	config, err := s.GetConfigForTenant(tenantID, modelName)
	if err != nil {
		return nil, err
	}
	
	return s.chatStreamWithConfig(config, messages)
}

// chatWithConfig 使用指定配置进行聊天
func (s *ModelService) chatWithConfig(config *Config, messages []Message) (string, error) {
	req := ChatRequest{
		Model:       config.Model,
		Messages:    messages,
		Temperature: config.Temperature,
		MaxTokens:   config.MaxTokens,
		Stream:      false,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request failed: %w", err)
	}

	httpReq, err := http.NewRequest("POST", config.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request failed: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+config.APIKey)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("decode response failed: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response from model")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// chatStreamWithConfig 使用指定配置进行流式聊天
func (s *ModelService) chatStreamWithConfig(config *Config, messages []Message) (<-chan StreamChunk, error) {
	req := ChatRequest{
		Model:       config.Model,
		Messages:    messages,
		Temperature: config.Temperature,
		MaxTokens:   config.MaxTokens,
		Stream:      true,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	httpReq, err := http.NewRequest("POST", config.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+config.APIKey)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	chunks := make(chan StreamChunk, 100)

	go func() {
		defer close(chunks)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			// 解析SSE格式
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					chunks <- StreamChunk{Done: true}
					return
				}

				var chatResp ChatResponse
				if err := json.Unmarshal([]byte(data), &chatResp); err != nil {
					chunks <- StreamChunk{Error: fmt.Errorf("parse chunk failed: %w", err)}
					return
				}

				if len(chatResp.Choices) > 0 {
					content := chatResp.Choices[0].Delta.Content
					if content != "" {
						chunks <- StreamChunk{Content: content}
					}
				}
			}
		}

		if err := scanner.Err(); err != nil {
			chunks <- StreamChunk{Error: err}
		}
	}()

	return chunks, nil
}

// Chat 非流式聊天 (使用默认配置)
func (s *ModelService) Chat(messages []Message) (string, error) {
	return s.chatWithConfig(&s.config, messages)
}

// ChatStream 流式聊天 (使用默认配置)
func (s *ModelService) ChatStream(messages []Message) (<-chan StreamChunk, error) {
	return s.chatStreamWithConfig(&s.config, messages)
}

// ChatWithSystem 带系统提示的聊天
func (s *ModelService) ChatWithSystem(systemPrompt string, userMessage string) (string, error) {
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	}
	return s.Chat(messages)
}

// ChatWithSystemStream 带系统提示的流式聊天
func (s *ModelService) ChatWithSystemStream(systemPrompt string, userMessage string) (<-chan StreamChunk, error) {
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	}
	return s.ChatStream(messages)
}
