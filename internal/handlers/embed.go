package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/middleware"
	"github.com/easp-platform/easp/internal/models"
	"github.com/easp-platform/easp/internal/modelservice"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// EmbedHandler Embed API 处理器
type EmbedHandler struct {
	chatHandler *ChatHandler
}

func NewEmbedHandler() *EmbedHandler {
	return &EmbedHandler{
		chatHandler: NewChatHandler(),
	}
}

// EmbedChatRequest Embed 聊天请求
type EmbedChatRequest struct {
	SessionID string          `json:"session_id"` // 可选，不传则自动创建
	VisitorID string          `json:"visitor_id"` // 外部访客ID
	Message   string          `json:"message" binding:"required"`
	Context   json.RawMessage `json:"context"` // 业务上下文（订单号、用户信息等）
}

// EmbedSessionResponse 会话响应
type EmbedSessionResponse struct {
	ID        string `json:"id"`
	VisitorID string `json:"visitor_id"`
}

// EmbedMessageResponse 消息响应
type EmbedMessageResponse struct {
	ID        string  `json:"id"`
	Role      string  `json:"role"`
	Content   string  `json:"content"`
	CreatedAt string  `json:"created_at"`
	Metadata  *string `json:"metadata,omitempty"`
}

// Chat 处理 Embed 聊天请求（SSE 流式）
// POST /embed/v1/chat
func (h *EmbedHandler) Chat(c *gin.Context) {
	var req EmbedChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := c.GetString(middleware.ContextEmbedTenantID)
	userID := c.GetString(middleware.ContextEmbedUserID)
	apiKeyVal, _ := c.Get(middleware.ContextAPIKey)
	apiKey := apiKeyVal.(models.APIKey)

	// 处理会话
	sessionID := req.SessionID
	if sessionID == "" {
		// 创建新会话
		sessionID = uuid.New().String()
		visitorID := req.VisitorID
		if visitorID == "" {
			visitorID = "anonymous"
		}

		var contextJSON *string
		if len(req.Context) > 0 {
			s := string(req.Context)
			contextJSON = &s
		}

		_, err := database.DB.Exec(`
			INSERT INTO embed_sessions (id, tenant_id, api_key_id, visitor_id, metadata, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, NOW(), NOW())`,
			sessionID, tenantID, apiKey.ID, visitorID, contextJSON)
		if err != nil {
			log.Printf("EmbedHandler: failed to create session: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
			return
		}
	} else {
		// 验证会话存在且属于当前租户
		var count int
		database.DB.Get(&count,
			"SELECT COUNT(*) FROM embed_sessions WHERE id = ? AND tenant_id = ?",
			sessionID, tenantID)
		if count == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
			return
		}
	}

	// 保存用户消息
	userMsgID := uuid.New().String()
	database.DB.Exec(`
		INSERT INTO embed_messages (id, session_id, role, content, created_at)
		VALUES (?, ?, 'user', ?, NOW())`,
		userMsgID, sessionID, req.Message)

	// 更新会话消息计数
	database.DB.Exec("UPDATE embed_sessions SET message_count = message_count + 1, updated_at = NOW() WHERE id = ?", sessionID)

	// 获取历史消息构建上下文（最近10条）
	var history []struct {
		Role    string `db:"role"`
		Content string `db:"content"`
	}
	database.DB.Select(&history,
		"SELECT role, content FROM embed_messages WHERE session_id = ? ORDER BY created_at ASC LIMIT 10",
		sessionID)

	// 构建消息列表
	var messages []AssistantMessage
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
	_, cfgErr := h.chatHandler.modelService.GetConfigForTenant(tenantID, "")
	if cfgErr != nil {
		sendSSE(c, "error", map[string]string{"message": "未配置可用的模型"})
		sendSSE(c, "done", nil)
		return
	}

	// 构建模型消息
	modelMessages := []modelservice.Message{
		{Role: "system", Content: "你是一个智能助手，帮助用户解答问题。请用简洁专业的语气回答。"},
	}
	for _, m := range messages {
		modelMessages = append(modelMessages, modelservice.Message{Role: m.Role, Content: m.Content})
	}

	// 调用模型（非流式，简化实现）
	response, err := h.chatHandler.callModelWithTools(tenantID, modelMessages, nil)
	if err != nil {
		log.Printf("EmbedHandler: model call failed: %v", err)
		sendSSE(c, "error", map[string]string{"message": "模型调用失败"})
		sendSSE(c, "done", nil)
		return
	}

	// 记录 token 消耗
	if response.InputTokens > 0 || response.OutputTokens > 0 {
		RecordModelUsage(tenantID, userID, response.Provider, response.Model,
			"/embed/chat", response.InputTokens, response.OutputTokens, 0)
	}

	// 发送响应
	sendSSE(c, "delta", map[string]string{"content": response.Content})
	sendSSE(c, "done", nil)

	// 保存助手消息
	assistantMsgID := uuid.New().String()
	database.DB.Exec(`
		INSERT INTO embed_messages (id, session_id, role, content, created_at)
		VALUES (?, ?, 'assistant', ?, NOW())`,
		assistantMsgID, sessionID, response.Content)
	database.DB.Exec("UPDATE embed_sessions SET message_count = message_count + 1, updated_at = NOW() WHERE id = ?", sessionID)
}

// CreateSession 创建会话
// POST /embed/v1/sessions
func (h *EmbedHandler) CreateSession(c *gin.Context) {
	tenantID := c.GetString(middleware.ContextEmbedTenantID)
	apiKeyVal, _ := c.Get(middleware.ContextAPIKey)
	apiKey := apiKeyVal.(models.APIKey)

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
	var metadataJSON *string
	if len(req.Metadata) > 0 {
		s := string(req.Metadata)
		metadataJSON = &s
	}

	_, err := database.DB.Exec(`
		INSERT INTO embed_sessions (id, tenant_id, api_key_id, visitor_id, metadata, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())`,
		sessionID, tenantID, apiKey.ID, visitorID, metadataJSON)
	if err != nil {
		log.Printf("EmbedHandler: failed to create session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":         sessionID,
		"visitor_id": visitorID,
		"created_at": time.Now(),
	})
}

// GetSessionMessages 获取会话消息
// GET /embed/v1/sessions/:sessionId/messages
func (h *EmbedHandler) GetSessionMessages(c *gin.Context) {
	tenantID := c.GetString(middleware.ContextEmbedTenantID)
	sessionID := c.Param("sessionId")

	// 验证会话属于当前租户
	var session models.EmbedSession
	err := database.DB.Get(&session,
		"SELECT * FROM embed_sessions WHERE id = ? AND tenant_id = ?", sessionID, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// 获取消息
	var messages []models.EmbedMessage
	database.DB.Select(&messages,
		"SELECT * FROM embed_messages WHERE session_id = ? ORDER BY created_at ASC", sessionID)
	if messages == nil {
		messages = []models.EmbedMessage{}
	}

	c.JSON(http.StatusOK, gin.H{
		"session":  session,
		"messages": messages,
	})
}

// ListSessions 列出会话
// GET /embed/v1/sessions
func (h *EmbedHandler) ListSessions(c *gin.Context) {
	tenantID := c.GetString(middleware.ContextEmbedTenantID)
	visitorID := c.Query("visitor_id")

	var sessions []models.EmbedSession
	var err error
	if visitorID != "" {
		err = database.DB.Select(&sessions,
			"SELECT * FROM embed_sessions WHERE tenant_id = ? AND visitor_id = ? ORDER BY updated_at DESC LIMIT 50",
			tenantID, visitorID)
	} else {
		err = database.DB.Select(&sessions,
			"SELECT * FROM embed_sessions WHERE tenant_id = ? ORDER BY updated_at DESC LIMIT 50",
			tenantID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list sessions"})
		return
	}
	if sessions == nil {
		sessions = []models.EmbedSession{}
	}

	c.JSON(http.StatusOK, sessions)
}
