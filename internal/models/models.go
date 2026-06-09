package models

import (
	"time"
)

// Tenant 租户模型
type Tenant struct {
	ID        string     `db:"id" json:"id"`
	Name      string     `db:"name" json:"name"`
	Plan      string     `db:"plan" json:"plan"`
	Status    string     `db:"status" json:"status"`
	ExpiresAt *time.Time `db:"expires_at" json:"expires_at,omitempty"`
	MaxUsers  int        `db:"max_users" json:"max_users"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
}

// TenantSSOConfig 租户SSO配置
type TenantSSOConfig struct {
	ID                string     `db:"id" json:"id"`
	TenantID          string     `db:"tenant_id" json:"tenant_id"`
	Enabled           bool       `db:"enabled" json:"enabled"`
	LoginURL          string     `db:"login_url" json:"login_url"`
	LoginMethod       string     `db:"login_method" json:"login_method"`
	LoginHeaders      *string    `db:"login_headers" json:"login_headers,omitempty"`
	LoginBodyTemplate *string    `db:"login_body_template" json:"login_body_template,omitempty"`
	UserInfoURL       *string    `db:"user_info_url" json:"user_info_url,omitempty"`
	UserInfoMethod    string     `db:"user_info_method" json:"user_info_method"`
	UserInfoHeaders   *string    `db:"user_info_headers" json:"user_info_headers,omitempty"`
	ResponseMapping   *string    `db:"response_mapping" json:"response_mapping,omitempty"`
	CallbackURL       *string    `db:"callback_url" json:"callback_url,omitempty"`
	SyncUserOnLogin   bool       `db:"sync_user_on_login" json:"sync_user_on_login"`
	SyncURL           *string    `db:"sync_url" json:"sync_url,omitempty"`
	SyncMethod        string     `db:"sync_method" json:"sync_method"`
	SyncHeaders       *string    `db:"sync_headers" json:"sync_headers,omitempty"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at" json:"updated_at"`
}

// User 用户模型
type User struct {
	ID           string     `db:"id" json:"id"`
	TenantID     string     `db:"tenant_id" json:"tenant_id"`
	Email        string     `db:"email" json:"email"`
	DisplayName  string     `db:"display_name" json:"display_name"`
	Avatar       string     `db:"avatar" json:"avatar"`
	Phone        string     `db:"phone" json:"phone"`
	Status       string     `db:"status" json:"status"`
	PasswordHash string     `db:"password_hash" json:"-"`
	SSOProvider  string     `db:"sso_provider" json:"sso_provider"`
	SSOUserID    string     `db:"sso_user_id" json:"sso_user_id"`
	SSOLinkedAt  *time.Time `db:"sso_linked_at" json:"sso_linked_at"`
	Metadata     *string    `db:"metadata" json:"metadata,omitempty"`
	LastLoginAt  *time.Time `db:"last_login_at" json:"last_login_at"`
	LoginCount   int        `db:"login_count" json:"login_count"`
	DeletedAt    *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
}

// Role 角色模型
type Role struct {
	ID          string  `db:"id" json:"id"`
	TenantID    string  `db:"tenant_id" json:"tenant_id"`
	Name        string  `db:"name" json:"name"`
	Description *string `db:"description" json:"description,omitempty"`
	Tools       *string `db:"tools" json:"tools,omitempty"`
	RateLimit   *string `db:"rate_limit" json:"rate_limit,omitempty"`
	DataScope   *string `db:"data_scope" json:"data_scope,omitempty"`
	IsSystem    bool    `db:"is_system" json:"is_system"`
	IsDefault   bool    `db:"is_default" json:"is_default"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// UserRole 用户角色关联
type UserRole struct {
	UserID string `db:"user_id" json:"user_id"`
	RoleID string `db:"role_id" json:"role_id"`
}

// Connector 连接器模型
type Connector struct {
	ID          string     `db:"id" json:"id"`
	TenantID    string     `db:"tenant_id" json:"tenant_id"`
	Name        string     `db:"name" json:"name"`
	Type        string     `db:"type" json:"type"`
	BaseURL     string     `db:"base_url" json:"base_url"`
	AuthType    *string    `db:"auth_type" json:"auth_type,omitempty"`
	AuthConfig  *string    `db:"auth_config" json:"auth_config,omitempty"`
	SpecURL     *string    `db:"spec_url" json:"spec_url,omitempty"`
	SpecContent *string    `db:"spec_content" json:"spec_content,omitempty"`
	Status      string     `db:"status" json:"status"`
	ToolsCount  int        `db:"tools_count" json:"tools_count"`
	LastSyncAt  *time.Time `db:"last_sync_at" json:"last_sync_at"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at" json:"updated_at"`
}

// MCPTool MCP工具模型
type MCPTool struct {
	ID            string  `db:"id" json:"id"`
	TenantID      string  `db:"tenant_id" json:"tenant_id"`
	ConnectorID   string  `db:"connector_id" json:"connector_id"`
	Name          string  `db:"name" json:"name"`
	Description   *string `db:"description" json:"description,omitempty"`
	InputSchema   *string `db:"input_schema" json:"input_schema,omitempty"`
	BackendMethod *string `db:"backend_method" json:"backend_method,omitempty"`
	BackendPath   *string `db:"backend_path" json:"backend_path,omitempty"`
	RiskLevel     string  `db:"risk_level" json:"risk_level"`
	Enabled       bool    `db:"enabled" json:"enabled"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
}

// MemoryPool 记忆池模型
type MemoryPool struct {
	ID        string    `db:"id" json:"id"`
	TenantID  string    `db:"tenant_id" json:"tenant_id"`
	Level     string    `db:"level" json:"level"`
	OwnerID   string    `db:"owner_id" json:"owner_id"`
	Name      string    `db:"name" json:"name"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// MemoryEntry 记忆条目模型
type MemoryEntry struct {
	ID          string    `db:"id" json:"id"`
	PoolID      string    `db:"pool_id" json:"pool_id"`
	Type        string    `db:"type" json:"type"`
	Content     string    `db:"content" json:"content"`
	Metadata    *string   `db:"metadata" json:"metadata,omitempty"`
	Sensitivity string    `db:"sensitivity" json:"sensitivity"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// Skill Skill模板模型
type Skill struct {
	ID                 string  `db:"id" json:"id"`
	TenantID           string  `db:"tenant_id" json:"tenant_id"`
	Name               string  `db:"name" json:"name"`
	Description        *string `db:"description" json:"description,omitempty"`
	Version            string  `db:"version" json:"version"`
	Triggers           *string `db:"triggers" json:"triggers,omitempty"`
	Steps              string  `db:"steps" json:"steps"`
	PermissionTopology *string `db:"permission_topology" json:"permission_topology,omitempty"`
	Status             string  `db:"status" json:"status"`
	CreatedBy          *string `db:"created_by" json:"created_by,omitempty"`
	CreatedAt          time.Time `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time `db:"updated_at" json:"updated_at"`
}

// AuditLog 审计日志模型
type AuditLog struct {
	ID         string    `db:"id" json:"id"`
	TenantID   string    `db:"tenant_id" json:"tenant_id"`
	UserID     *string   `db:"user_id" json:"user_id,omitempty"`
	AgentID    *string   `db:"agent_id" json:"agent_id,omitempty"`
	Tool       string    `db:"tool" json:"tool"`
	Action     string    `db:"action" json:"action"`
	Resource   *string   `db:"resource" json:"resource,omitempty"`
	Detail     *string   `db:"detail" json:"detail,omitempty"`
	Decision   *string   `db:"decision" json:"decision,omitempty"`
	Result     *string   `db:"result" json:"result,omitempty"`
	DurationMs *int      `db:"duration_ms" json:"duration_ms,omitempty"`
	IP         *string   `db:"ip" json:"ip,omitempty"`
	UserAgent  *string   `db:"user_agent" json:"user_agent,omitempty"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}

// CircuitBreakerState 熔断器状态模型
type CircuitBreakerState struct {
	ToolName    string     `db:"tool_name" json:"tool_name"`
	TenantID    string     `db:"tenant_id" json:"tenant_id"`
	State       string     `db:"state" json:"state"`
	FailCount   int        `db:"fail_count" json:"fail_count"`
	LastFailure *time.Time `db:"last_failure" json:"last_failure"`
	Degradation *string    `db:"degradation" json:"degradation,omitempty"`
}

// ABACRule ABAC规则模型
type ABACRule struct {
	ID            string    `db:"id" json:"id"`
	TenantID      string    `db:"tenant_id" json:"tenant_id"`
	Name          string    `db:"name" json:"name"`
	Description   *string   `db:"description" json:"description,omitempty"`
	RuleCondition string    `db:"rule_condition" json:"rule_condition"`
	Action        string    `db:"action" json:"action"`
	Priority      int       `db:"priority" json:"priority"`
	Enabled       bool      `db:"enabled" json:"enabled"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
}

// SSOProvider SSO提供商配置模型
type SSOProvider struct {
	ID          string    `db:"id" json:"id"`
	TenantID    string    `db:"tenant_id" json:"tenant_id"`
	Name        string    `db:"name" json:"name"`
	Type        string    `db:"type" json:"type"`
	DisplayName *string   `db:"display_name" json:"display_name,omitempty"`
	Icon        *string   `db:"icon" json:"icon,omitempty"`
	Enabled     bool      `db:"enabled" json:"enabled"`
	Config      string    `db:"config" json:"config"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}
