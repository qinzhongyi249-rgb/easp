package embed

import (
	"log"
	"net/http"

	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CreateOrVerifySession 创建或验证会话
// 返回 sessionID, ok
func CreateOrVerifySession(c *gin.Context, tenantID string, apiKey *models.APIKey, req *ChatRequest) (string, bool) {
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

		var apiKeyID string
		if apiKey != nil {
			apiKeyID = apiKey.ID
		}
		_, err := database.DB.Exec(`
			INSERT INTO embed_sessions (id, tenant_id, api_key_id, visitor_id, metadata, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, NOW(), NOW())`,
			sessionID, tenantID, apiKeyID, visitorID, contextJSON)
		if err != nil {
			log.Printf("embed.CreateOrVerifySession: failed to create session: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
			return "", false
		}
	} else {
		// 验证会话存在且属于当前租户
		var count int
		database.DB.Get(&count,
			"SELECT COUNT(*) FROM embed_sessions WHERE id = ? AND tenant_id = ?",
			sessionID, tenantID)
		if count == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
			return "", false
		}
	}

	return sessionID, true
}

// SaveUserMessage 保存用户消息并更新会话计数
func SaveUserMessage(sessionID, message string) string {
	userMsgID := uuid.New().String()
	database.DB.Exec(`
		INSERT INTO embed_messages (id, session_id, role, content, created_at)
		VALUES (?, ?, 'user', ?, NOW())`,
		userMsgID, sessionID, message)

	// 更新会话消息计数
	database.DB.Exec("UPDATE embed_sessions SET message_count = message_count + 1, updated_at = NOW() WHERE id = ?", sessionID)

	return userMsgID
}

// GetHistory 获取最近10条历史消息
func GetHistory(sessionID string) []struct {
	Role    string `db:"role"`
	Content string `db:"content"`
} {
	var history []struct {
		Role    string `db:"role"`
		Content string `db:"content"`
	}
	database.DB.Select(&history,
		"SELECT role, content FROM embed_messages WHERE session_id = ? ORDER BY created_at ASC LIMIT 10",
		sessionID)
	return history
}
