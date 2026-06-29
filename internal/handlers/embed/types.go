package embed

import (
	"github.com/easp-platform/easp/internal/models"
)

// TokenExchangeRequest token exchange 请求
type TokenExchangeRequest struct {
	TenantID               string `json:"tenant_id" binding:"required"`
	ExternalSystem         string `json:"external_system" binding:"required"`
	ExternalUserID         string `json:"external_user_id" binding:"required"`
	DisplayName            string `json:"display_name"`
	Email                  string `json:"email"`
	Phone                  string `json:"phone"`
	ExternalAccessToken    string `json:"external_access_token"`
	ExternalTokenType      string `json:"external_token_type"`
	ExternalTokenExpiresAt int64  `json:"external_token_expires_at"`
}

// TokenExchangeResponse token exchange 响应
type TokenExchangeResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	UserID      string `json:"user_id"`
	TenantID    string `json:"tenant_id"`
}

// ChatRequest 嵌入式聊天请求
type ChatRequest struct {
	SessionID  string          `json:"session_id"`  // 会话ID，首次请求为空，后端自动创建
	VisitorID  string          `json:"visitor_id"`  // 访客ID，用于区分同一业务用户的不同访客
	Message    string          `json:"message" binding:"required"` // 用户消息
	Assistant  string          `json:"assistant" binding:"required"` // 技能ID
	AssistantName string       `json:"assistant_name"` // 自定义AI助手名称
	Context    json.RawMessage `json:"context"`       // 业务上下文，JSON格式
}

// EmbedSession 嵌入式会话
type EmbedSession struct {
	ID            string          `db:"id"`
	TenantID      string          `db:"tenant_id"`
	APIKeyID      string          `db:"api_key_id"`
	VisitorID     string          `db:"visitor_id"`
	Metadata      json.RawMessage `db:"metadata"`
	MessageCount  int             `db:"message_count"`
	CreatedAt     time.Time       `db:"created_at"`
	UpdatedAt     time.Time       `db:"updated_at"`
}

// EmbedMessage 嵌入式消息
type EmbedMessage struct {
	ID         string    `db:"id"`
	SessionID  string    `db:"session_id"`
	Role       string    `db:"role"`
	Content    string    `db:"content"`
	CreatedAt  time.Time `db:"created_at"`
}

// GetUserFromContext 从context获取用户
func GetUserFromContext(ctx any) (models.User, bool) {
	if u, ok := ctx.(models.User); ok {
		return u, true
	}
	return models.User{}, false
}
