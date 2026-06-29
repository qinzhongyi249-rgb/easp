package embed

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/handlers"
	"github.com/easp-platform/easp/internal/middleware"
	"github.com/easp-platform/easp/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AssistantMessage 助手消息
type AssistantMessage struct {
	ID        string  `json:"id"`
	Role      string  `json:"role"`
	Content   string  `json:"content"`
	CreatedAt string  `json:"created_at"`
	Metadata  *string `json:"metadata,omitempty"`
}

// AssistantRequest 助手请求
type AssistantRequest struct {
	ConversationID string              `json:"conversation_id"`
	Messages        []AssistantMessage  `json:"messages"`
	ExecutionMode   string              `json:"execution_mode"`
	PageContext     json.RawMessage     `json:"page_context"`
}

// EmbedChatRequest 嵌入式聊天请求
type EmbedChatRequest struct {
	SessionID     string          `json:"session_id"`      // 会话ID，首次请求为空，后端自动创建
	VisitorID     string          `json:"visitor_id"`      // 访客ID，用于区分同一业务用户的不同访客
	Message       string          `json:"message" binding:"required"` // 用户消息
	Assistant     string          `json:"assistant" binding:"required"` // 技能ID
	AssistantName *string         `json:"assistant_name"`  // 自定义AI助手名称
	Context       json.RawMessage `json:"context"`         // 业务上下文，JSON格式
	ExecutionMode string          `json:"execution_mode"`  // 执行模式: normal/production/sandbox，默认 normal
	PageContext   json.RawMessage `json:"page_context"`    // 页面上下文
}

// Chat 处理 Embed 聊天请求（SSE 流式）
// POST /embed/v1/chat
func Chat(c *gin.Context, chatHandler *handlers.ChatHandler) {
	var req EmbedChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := c.GetString(middleware.ContextEmbedTenantID)
	var apiKey *models.APIKey
	if apiKeyVal, ok := c.Get(middleware.ContextAPIKey); ok {
		if ak, ok := apiKeyVal.(models.APIKey); ok {
			apiKey = &ak
		}
	}

	// 创建或验证会话
	sessionID, ok := CreateOrVerifySession(c, tenantID, apiKey, &req)
	if !ok {
		return
	}

	// 保存用户消息
	_ = SaveUserMessage(sessionID, req.Message)

	// 获取历史消息构建上下文（最近10条）
	history := GetHistory(sessionID)

	// 构建消息列表
	var messages []AssistantMessage
	// 添加系统提示：嵌入式场景使用自定义名称，不主动列出不可用权限
	assistantName := "EASP企业智能服务平台助手"
	if req.AssistantName != nil && *req.AssistantName != "" {
		assistantName = *req.AssistantName
	}
	systemPrompt := fmt.Sprintf("你是 %s。\n你可以帮助用户查询业务数据、操作业务功能，请根据用户需求，调用对应工具完成任务。\n\n规则：\n- 需要操作/查询时优先调用工具，不猜测。\n- 输出尽量精简：先结论，少铺垫；查询结果只列关键字段。\n- 工具返回的数据优先于记忆和页面上下文。\n- 无权限或无对应工具时直接说明。", assistantName)
	messages = append(messages, AssistantMessage{Role: "system", Content: systemPrompt})
	// 添加业务上下文作为系统消息
	if len(req.Context) > 0 {
		contextMsg := fmt.Sprintf("业务上下文信息: %s", string(req.Context))
		messages = append(messages, AssistantMessage{Role: "system", Content: contextMsg})
	}
	for _, h := range history {
		messages = append(messages, AssistantMessage{Role: h.Role, Content: h.Content})
	}

	// 设置 SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// 获取模型配置
	_, cfgErr := chatHandler.ModelService.GetConfigForTenant(tenantID, "")
	if cfgErr != nil {
		SendSSE(c, "error", map[string]string{"message": "未配置可用的模型"})
		SendSSE(c, "done", nil)
		return
	}

	// 完整工具调用流程（和主站一致，多轮 + 流式输出执行过程）
	executionMode := req.ExecutionMode
	if executionMode == "" {
		executionMode = "normal" // 默认真实调用工具
	}
	var bindReq = AssistantRequest{
		ConversationID: sessionID,
		Messages:       messages,
		ExecutionMode:  executionMode,
		PageContext:    req.PageContext,
	}
	// 重新序列化 body 让 ChatStream.ShouldBindJSON 能读取
	bodyBytes, _ := json.Marshal(bindReq)
	c.Request.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
	// 直接走和主站一样的完整流程，工具调用多轮循环，流式输出每一步
	chatHandler.ChatStream(c)
}

// SendSSE 发送 SSE 事件
func SendSSE(c *gin.Context, event string, data interface{}) {
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err == nil {
			c.Writer.Write([]byte("event: " + event + "\n"))
			c.Writer.Write([]byte("data: " + string(jsonData) + "\n\n"))
		}
	} else {
		c.Writer.Write([]byte("event: " + event + "\n"))
		c.Writer.Write([]byte("data: \n\n"))
	}
	c.Writer.Flush()
}

// ListAssistantConversations 查询当前嵌入用户自己的助手历史会话。
func ListAssistantConversations(c *gin.Context) {
	tenantID := c.GetString(middleware.ContextTenantID)
	userID := c.GetString(middleware.ContextUserID)
	var conversations []models.AssistantConversation
	err := database.DB.Select(&conversations, `SELECT * FROM assistant_conversations WHERE tenant_id = ? AND user_id = ? ORDER BY updated_at DESC LIMIT 50`, tenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list conversations"})
		return
	}
	if conversations == nil {
		conversations = []models.AssistantConversation{}
	}
	c.JSON(http.StatusOK, conversations)
}

// GetAssistantConversationMessages 查询当前嵌入用户自己的助手历史消息。
func GetAssistantConversationMessages(c *gin.Context) {
	tenantID := c.GetString(middleware.ContextTenantID)
	userID := c.GetString(middleware.ContextUserID)
	conversationID := c.Param("conversationId")
	var count int
	if err := database.DB.Get(&count, `SELECT COUNT(*) FROM assistant_conversations WHERE id = ? AND tenant_id = ? AND user_id = ?`, conversationID, tenantID, userID); err != nil || count == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Conversation not found"})
		return
	}
	var messages []models.SessionMemory
	err := database.DB.Select(&messages, `SELECT id, tenant_id, user_id, session_id, role, content, token_count, entity_ids, created_at FROM session_memories WHERE tenant_id = ? AND user_id = ? AND session_id = ? ORDER BY created_at ASC`, tenantID, userID, conversationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list messages"})
		return
	}
	if messages == nil {
		messages = []models.SessionMemory{}
	}
	c.JSON(http.StatusOK, gin.H{"conversation_id": conversationID, "messages": messages})
}

// CreateSession 创建会话
// POST /embed/v1/sessions
func CreateSession(c *gin.Context) {
	tenantID := c.GetString(middleware.ContextEmbedTenantID)
	var apiKey *models.APIKey
	if apiKeyVal, ok := c.Get(middleware.ContextAPIKey); ok {
		if ak, ok := apiKeyVal.(models.APIKey); ok {
			apiKey = &ak
		}
	}

	var req struct {
		VisitorID string          `json:"visitor_id"`
		Metadata  json.RawMessage `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	visitorID := req.VisitorID
	if visitorID == "" {
		visitorID = "anonymous"
	}

	sessionID := uuid.New().String()

	var contextJSON *string
	if len(req.Metadata) > 0 {
		s := string(req.Metadata)
		contextJSON = &s
	}

	var apiKeyID string
	if apiKey != nil {
		apiKeyID = apiKey.ID
	}
	_, err := database.DB.Exec(`
		INSERT INTO embed_sessions (id, tenant_id, api_key_id, visitor_id, metadata, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())`,
		sessionID, tenantID, apiKeyID, visitorID, contextJSON)
	if err != nil {
		log.Printf("embed.CreateSession: failed to create session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"session_id": sessionID})
}
