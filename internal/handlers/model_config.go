package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/easp-platform/easp/internal/models"
	"github.com/easp-platform/easp/internal/repositories"
	"github.com/gin-gonic/gin"
)

// ModelConfigHandler 模型配置处理器
type ModelConfigHandler struct {
	providerRepo *repositories.ModelProviderRepository
	configRepo   *repositories.ModelConfigRepository
}

func NewModelConfigHandler() *ModelConfigHandler {
	return &ModelConfigHandler{
		providerRepo: repositories.NewModelProviderRepository(),
		configRepo:   repositories.NewModelConfigRepository(),
	}
}

// ========== 模型提供商 API ==========

// CreateProvider 创建提供商
func (h *ModelConfigHandler) CreateProvider(c *gin.Context) {
	tenantID := c.Param("tenantId")
	var provider models.ModelProvider
	if err := c.ShouldBindJSON(&provider); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	provider.TenantID = tenantID

	if err := h.providerRepo.Create(&provider); err != nil {
		log.Printf("Failed to create provider: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create provider", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, provider)
}

// GetProvider 获取提供商
func (h *ModelConfigHandler) GetProvider(c *gin.Context) {
	providerID := c.Param("providerId")
	provider, err := h.providerRepo.GetByID(providerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Provider not found"})
		return
	}
	c.JSON(http.StatusOK, provider)
}

// ListProviders 列出提供商
func (h *ModelConfigHandler) ListProviders(c *gin.Context) {
	tenantID := c.Param("tenantId")
	
	// 可选参数：只返回启用的
	enabledOnly := c.Query("enabled")
	
	var providers []models.ModelProvider
	var err error
	
	if enabledOnly == "true" {
		providers, err = h.providerRepo.ListEnabled(tenantID)
	} else {
		providers, err = h.providerRepo.ListByTenant(tenantID)
	}
	
	if err != nil {
		log.Printf("Failed to list providers: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list providers", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, providers)
}

// UpdateProvider 更新提供商
func (h *ModelConfigHandler) UpdateProvider(c *gin.Context) {
	providerID := c.Param("providerId")
	provider, err := h.providerRepo.GetByID(providerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Provider not found"})
		return
	}

	if err := c.ShouldBindJSON(provider); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.providerRepo.Update(provider); err != nil {
		log.Printf("Failed to update provider: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update provider", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, provider)
}

// DeleteProvider 删除提供商
func (h *ModelConfigHandler) DeleteProvider(c *gin.Context) {
	providerID := c.Param("providerId")
	if err := h.providerRepo.Delete(providerID); err != nil {
		log.Printf("Failed to delete provider: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete provider", "details": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// ========== 模型配置 API ==========

// CreateConfig 创建模型配置
func (h *ModelConfigHandler) CreateConfig(c *gin.Context) {
	tenantID := c.Param("tenantId")
	var config models.ModelConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.TenantID = tenantID

	if err := h.configRepo.Create(&config); err != nil {
		log.Printf("Failed to create model config: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create model config", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, config)
}

// GetConfig 获取模型配置
func (h *ModelConfigHandler) GetConfig(c *gin.Context) {
	configID := c.Param("configId")
	config, err := h.configRepo.GetByID(configID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Model config not found"})
		return
	}
	c.JSON(http.StatusOK, config)
}

// ListConfigs 列出模型配置
func (h *ModelConfigHandler) ListConfigs(c *gin.Context) {
	tenantID := c.Param("tenantId")
	
	// 可选参数
	enabledOnly := c.Query("enabled")
	providerID := c.Query("provider_id")
	
	var configs []models.ModelConfig
	var err error
	
	if providerID != "" {
		configs, err = h.configRepo.ListByProvider(providerID)
	} else if enabledOnly == "true" {
		configs, err = h.configRepo.ListEnabled(tenantID)
	} else {
		configs, err = h.configRepo.ListByTenant(tenantID)
	}
	
	if err != nil {
		log.Printf("Failed to list model configs: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list model configs", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, configs)
}

// UpdateConfig 更新模型配置
func (h *ModelConfigHandler) UpdateConfig(c *gin.Context) {
	configID := c.Param("configId")
	config, err := h.configRepo.GetByID(configID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Model config not found"})
		return
	}

	if err := c.ShouldBindJSON(config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.configRepo.Update(config); err != nil {
		log.Printf("Failed to update model config: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update model config", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, config)
}

// DeleteConfig 删除模型配置
func (h *ModelConfigHandler) DeleteConfig(c *gin.Context) {
	configID := c.Param("configId")
	if err := h.configRepo.Delete(configID); err != nil {
		log.Printf("Failed to delete model config: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete model config", "details": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// SetDefaultConfig 设置默认模型
func (h *ModelConfigHandler) SetDefaultConfig(c *gin.Context) {
	tenantID := c.Param("tenantId")
	configID := c.Param("configId")
	
	if err := h.configRepo.SetDefault(configID, tenantID); err != nil {
		log.Printf("Failed to set default model: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set default model", "details": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "Default model set successfully"})
}

// GetDefaultConfig 获取默认模型配置
func (h *ModelConfigHandler) GetDefaultConfig(c *gin.Context) {
	tenantID := c.Param("tenantId")
	
	config, err := h.configRepo.GetDefault(tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No default model configured"})
		return
	}
	
	c.JSON(http.StatusOK, config)
}

// ValidateModelRequest 模型验证请求
type ValidateModelRequest struct {
	BaseURL      string `json:"base_url" binding:"required"`
	APIKey       string `json:"api_key" binding:"required"`
	Model        string `json:"model" binding:"required"`
	CallType     string `json:"call_type"` // chat_completion / completion
	ResponseType string `json:"response_type"` // openai / custom
}

// ValidateModelResponse 模型验证响应
type ValidateModelResponse struct {
	Success        bool   `json:"success"`
	Message        string `json:"message"`
	APIType        string `json:"api_type"`         // 识别的 API 类型
	ResponseType   string `json:"response_type"`    // 识别的响应格式
	SupportsStream bool   `json:"supports_stream"`  // 是否支持流式
	SupportsTools  bool   `json:"supports_tools"`   // 是否支持工具调用
	TokenFieldType string `json:"token_field_type"` // token 字段类型：int/float/string
	PromptTokens   int64  `json:"prompt_tokens,omitempty"`
	TotalTokens    int64  `json:"total_tokens,omitempty"`
}

// ValidateModel 验证模型配置
func (h *ModelConfigHandler) ValidateModel(c *gin.Context) {
	var req ValidateModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 发送测试请求
	resp := &ValidateModelResponse{}
	
	// 默认尝试 chat_completion
	callType := req.CallType
	if callType == "" {
		callType = "chat_completion"
	}

	// 构建测试请求
	testReq := map[string]interface{}{
		"model": req.Model,
		"messages": []map[string]string{
			{"role": "user", "content": "Hello, this is a test."},
		},
		"max_tokens": 10,
		"stream": false,
	}

	// 添加简单的 tool 测试（如果支持）
	testReq["tools"] = []map[string]interface{}{
		{
			"type": "function",
			"function": map[string]interface{}{
				"name": "test_tool",
				"description": "A test tool",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{},
				},
			},
		},
	}

	// 发送 HTTP 请求
	url := strings.TrimSuffix(req.BaseURL, "/") + "/chat/completions"
	jsonData, _ := json.Marshal(testReq)
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(jsonData))
	if err != nil {
		resp.Success = false
		resp.Message = fmt.Sprintf("Failed to create request: %v", err)
		c.JSON(http.StatusOK, resp)
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		resp.Success = false
		resp.Message = fmt.Sprintf("Request failed: %v", err)
		c.JSON(http.StatusOK, resp)
		return
	}
	defer httpResp.Body.Close()

	// 检查 HTTP 状态码
	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		resp.Success = false
		resp.Message = fmt.Sprintf("HTTP %d: %s", httpResp.StatusCode, string(body))
		c.JSON(http.StatusOK, resp)
		return
	}

	// 检查流式支持
	if httpResp.Header.Get("Content-Type") == "text/event-stream" {
		resp.SupportsStream = true
	}

	// 解析响应体
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Success = false
		resp.Message = fmt.Sprintf("Failed to read response: %v", err)
		c.JSON(http.StatusOK, resp)
		return
	}

	// 尝试解析为 OpenAI 格式
	var openAIResp map[string]interface{}
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		resp.Success = false
		resp.Message = fmt.Sprintf("Failed to parse JSON: %v", err)
		c.JSON(http.StatusOK, resp)
		return
	}

	// 验证响应结构
	var message map[string]interface{}
	if choices, ok := openAIResp["choices"].([]interface{}); ok && len(choices) > 0 {
		// 检查是否有 message 字段（OpenAI chat 格式）
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if msg, ok := choice["message"].(map[string]interface{}); ok {
				message = msg
				if _, hasContent := message["content"]; hasContent {
					resp.APIType = "chat_completion"
					resp.ResponseType = "openai"
				}
			}
			
			// 检查 tool_calls 支持
			if toolCalls, ok := message["tool_calls"].([]interface{}); ok && len(toolCalls) > 0 {
				resp.SupportsTools = true
			}
		}
	} else {
		// 尝试 completion 格式
		if _, ok := openAIResp["choices"].([]interface{}); ok {
			resp.APIType = "completion"
			resp.ResponseType = "openai"
		}
	}

	// 检查 token 字段类型
	if usage, ok := openAIResp["usage"].(map[string]interface{}); ok {
		// 检测 token 字段类型
		for _, key := range []string{"prompt_tokens", "completion_tokens", "total_tokens"} {
			if val, exists := usage[key]; exists {
				switch val.(type) {
				case float64:
					resp.TokenFieldType = "float"
					// 尝试转换为 int64
					if fval, ok := val.(float64); ok {
						if key == "prompt_tokens" {
							resp.PromptTokens = int64(fval)
						} else if key == "total_tokens" {
							resp.TotalTokens = int64(fval)
						}
					}
				case int:
					resp.TokenFieldType = "int"
					if ival, ok := val.(int); ok {
						if key == "prompt_tokens" {
							resp.PromptTokens = int64(ival)
						} else if key == "total_tokens" {
							resp.TotalTokens = int64(ival)
						}
					}
				case string:
					resp.TokenFieldType = "string"
				}
				break
			}
		}
	}

	resp.Success = true
	resp.Message = "Validation successful"
	c.JSON(http.StatusOK, resp)
}
