package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/easp-platform/easp/internal/modelservice"
	"github.com/gin-gonic/gin"
)

// ModelHandler 模型处理器
type ModelHandler struct {
	service *modelservice.ModelService
}

// NewModelHandler 创建模型处理器
func NewModelHandler() *ModelHandler {
	// 默认配置（用于向后兼容）
	config := modelservice.Config{
		BaseURL:     "https://maas.apigo.ai/v1",
		APIKey:      "sk-platform-228fe8d21e2a407f3f35ecf5e1ea72ca3adb23f3023432d2",
		Model:       "claude-opus-4-7",
		Temperature: 1.0,
		MaxTokens:   4096,
	}

	return &ModelHandler{
		service: modelservice.NewModelService(config),
	}
}

// ChatRequest API聊天请求
type ChatRequest struct {
	TenantID    string                   `json:"tenant_id,omitempty"`
	ModelName   string                   `json:"model_name,omitempty"`
	Messages    []modelservice.Message   `json:"messages" binding:"required"`
	Temperature float64                  `json:"temperature,omitempty"`
	MaxTokens   int                      `json:"max_tokens,omitempty"`
	Stream      bool                     `json:"stream,omitempty"`
}

// Chat 非流式聊天
func (h *ModelHandler) Chat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var response string
	var err error

	// 如果指定了租户ID，使用租户配置
	if req.TenantID != "" {
		response, err = h.service.ChatWithTenant(req.TenantID, req.ModelName, req.Messages)
	} else {
		// 使用默认配置
		response, err = h.service.Chat(req.Messages)
	}

	if err != nil {
		log.Printf("Model chat failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Model call failed", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"content": response,
	})
}

// ChatStream 流式聊天
func (h *ModelHandler) ChatStream(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 设置SSE头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	var chunks <-chan modelservice.StreamChunk
	var err error

	// 如果指定了租户ID，使用租户配置
	if req.TenantID != "" {
		chunks, err = h.service.ChatWithTenantStream(req.TenantID, req.ModelName, req.Messages)
	} else {
		chunks, err = h.service.ChatStream(req.Messages)
	}

	if err != nil {
		log.Printf("Model stream failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Model call failed", "details": err.Error()})
		return
	}

	// 流式输出
	c.Stream(func(w io.Writer) bool {
		for chunk := range chunks {
			if chunk.Error != nil {
				log.Printf("Stream error: %v", chunk.Error)
				data, _ := json.Marshal(gin.H{"error": chunk.Error.Error()})
				c.SSEvent("error", string(data))
				return false
			}

			if chunk.Done {
				c.SSEvent("done", "[DONE]")
				return false
			}

			data, _ := json.Marshal(gin.H{"content": chunk.Content})
			c.SSEvent("message", string(data))
		}
		return false
	})
}

// SkillExecuteRequest Skill执行请求
type SkillExecuteRequest struct {
	TenantID    string                 `json:"tenant_id,omitempty"`
	ModelName   string                 `json:"model_name,omitempty"`
	SkillName   string                 `json:"skill_name" binding:"required"`
	Input       string                 `json:"input" binding:"required"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Temperature float64                `json:"temperature,omitempty"`
}

// ExecuteSkill 执行Skill
func (h *ModelHandler) ExecuteSkill(c *gin.Context) {
	var req SkillExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 构建系统提示
	systemPrompt := buildSkillPrompt(req.SkillName, req.Context)

	// 构建消息
	messages := []modelservice.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: req.Input},
	}

	var response string
	var err error

	// 如果指定了租户ID，使用租户配置
	if req.TenantID != "" {
		response, err = h.service.ChatWithTenant(req.TenantID, req.ModelName, messages)
	} else {
		response, err = h.service.Chat(messages)
	}

	if err != nil {
		log.Printf("Skill execution failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Skill execution failed", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"skill":    req.SkillName,
		"input":    req.Input,
		"output":   response,
	})
}

// MCPExecuteRequest MCP工具执行请求
type MCPExecuteRequest struct {
	TenantID    string                 `json:"tenant_id,omitempty"`
	ModelName   string                 `json:"model_name,omitempty"`
	ToolName    string                 `json:"tool_name" binding:"required"`
	Parameters  map[string]interface{} `json:"parameters" binding:"required"`
	Temperature float64                `json:"temperature,omitempty"`
}

// ExecuteMCPTool 执行MCP工具
func (h *ModelHandler) ExecuteMCPTool(c *gin.Context) {
	var req MCPExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 构建系统提示
	systemPrompt := buildMCPPrompt(req.ToolName, req.Parameters)

	// 构建用户消息
	userMsg, _ := json.Marshal(req.Parameters)

	// 构建消息
	messages := []modelservice.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: string(userMsg)},
	}

	var response string
	var err error

	// 如果指定了租户ID，使用租户配置
	if req.TenantID != "" {
		response, err = h.service.ChatWithTenant(req.TenantID, req.ModelName, messages)
	} else {
		response, err = h.service.Chat(messages)
	}

	if err != nil {
		log.Printf("MCP tool execution failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "MCP tool execution failed", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tool":      req.ToolName,
		"parameters": req.Parameters,
		"output":    response,
	})
}

// buildSkillPrompt 构建Skill提示
func buildSkillPrompt(skillName string, context map[string]interface{}) string {
	prompt := `你是一个专业的AI助手，负责执行指定的Skill任务。

Skill名称: ` + skillName + `

请根据以下规则执行任务：
1. 理解用户的输入意图
2. 按照Skill的定义执行相应操作
3. 返回结构化的执行结果
4. 如果遇到错误，提供清晰的错误说明`

	if context != nil {
		contextJSON, _ := json.Marshal(context)
		prompt += "\n\n上下文信息:\n" + string(contextJSON)
	}

	return prompt
}

// buildMCPPrompt 构建MCP工具提示
func buildMCPPrompt(toolName string, parameters map[string]interface{}) string {
	prompt := `你是一个API调用助手，负责执行MCP工具调用。

工具名称: ` + toolName + `

请根据以下规则执行：
1. 理解工具的输入参数
2. 模拟执行工具调用
3. 返回符合工具定义的输出格式
4. 确保输出是有效的JSON格式`

	if parameters != nil {
		paramsJSON, _ := json.Marshal(parameters)
		prompt += "\n\n输入参数:\n" + string(paramsJSON)
	}

	return prompt
}
