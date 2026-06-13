package models

import (
	"time"
)

// Tenant 租户模型
type Tenant struct {
	ID                string     `db:"id" json:"id"`
	Name              string     `db:"name" json:"name"`
	Plan              string     `db:"plan" json:"plan"`
	Status            string     `db:"status" json:"status"`
	ExpiresAt         *time.Time `db:"expires_at" json:"expires_at,omitempty"`
	MaxUsers          int        `db:"max_users" json:"max_users"`
	RateLimit         int        `db:"rate_limit" json:"rate_limit"`                   // 每分钟最大请求数，0=不限
	DailyQuota        int        `db:"daily_quota" json:"daily_quota"`                 // 每日API调用上限，0=不限
	MonthlyQuota      int        `db:"monthly_quota" json:"monthly_quota"`             // 每月API调用上限，0=不限
	DailyTokenQuota   int        `db:"daily_token_quota" json:"daily_token_quota"`     // 每日token消耗上限，0=不限
	MonthlyTokenQuota int        `db:"monthly_token_quota" json:"monthly_token_quota"` // 每月token消耗上限，0=不限
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at" json:"updated_at"`
}

// ApiUsage API调用记录
type ApiUsage struct {
	ID         int64     `db:"id" json:"id"`
	TenantID   string    `db:"tenant_id" json:"tenant_id"`
	UserID     string    `db:"user_id" json:"user_id"`
	Endpoint   string    `db:"endpoint" json:"endpoint"`
	Method     string    `db:"method" json:"method"`
	StatusCode int       `db:"status_code" json:"status_code"`
	LatencyMs  int       `db:"latency_ms" json:"latency_ms"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}

// ModelUsage 模型调用记录（含token消耗）
type ModelUsage struct {
	ID            int64     `db:"id" json:"id"`
	TenantID      string    `db:"tenant_id" json:"tenant_id"`
	UserID        string    `db:"user_id" json:"user_id"`
	ModelProvider string    `db:"model_provider" json:"model_provider"`
	ModelName     string    `db:"model_name" json:"model_name"`
	InputTokens   int       `db:"input_tokens" json:"input_tokens"`
	OutputTokens  int       `db:"output_tokens" json:"output_tokens"`
	CachedTokens  int       `db:"cached_tokens" json:"cached_tokens"`
	TotalTokens   int       `db:"total_tokens" json:"total_tokens"`
	LatencyMs     int       `db:"latency_ms" json:"latency_ms"`
	Endpoint      string    `db:"endpoint" json:"endpoint"`
	Source        string    `db:"source" json:"source"`
	SourceName    string    `db:"source_name" json:"source_name"`
	ResourceType  string    `db:"resource_type" json:"resource_type"`
	ResourceID    string    `db:"resource_id" json:"resource_id"`
	RequestID     string    `db:"request_id" json:"request_id"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
}

// ToolCallUsage MCP工具/Skill/内置工具调用记录
// 用于统计功能调用次数和耗时，不等同于模型token消耗。
type ToolCallUsage struct {
	ID           int64     `db:"id" json:"id"`
	TenantID     string    `db:"tenant_id" json:"tenant_id"`
	UserID       string    `db:"user_id" json:"user_id"`
	ResourceType string    `db:"resource_type" json:"resource_type"`
	ResourceID   string    `db:"resource_id" json:"resource_id"`
	ResourceName string    `db:"resource_name" json:"resource_name"`
	Source       string    `db:"source" json:"source"`
	Status       string    `db:"status" json:"status"`
	LatencyMs    int       `db:"latency_ms" json:"latency_ms"`
	RequestID    string    `db:"request_id" json:"request_id"`
	ErrorMessage *string   `db:"error_message" json:"error_message,omitempty"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
}

// TenantSSOConfig 租户SSO配置
type TenantSSOConfig struct {
	ID                string    `db:"id" json:"id"`
	TenantID          string    `db:"tenant_id" json:"tenant_id"`
	Enabled           bool      `db:"enabled" json:"enabled"`
	LoginURL          string    `db:"login_url" json:"login_url"`
	LoginMethod       string    `db:"login_method" json:"login_method"`
	LoginHeaders      *string   `db:"login_headers" json:"login_headers,omitempty"`
	LoginBodyTemplate *string   `db:"login_body_template" json:"login_body_template,omitempty"`
	UserInfoURL       *string   `db:"user_info_url" json:"user_info_url,omitempty"`
	UserInfoMethod    string    `db:"user_info_method" json:"user_info_method"`
	UserInfoHeaders   *string   `db:"user_info_headers" json:"user_info_headers,omitempty"`
	ResponseMapping   *string   `db:"response_mapping" json:"response_mapping,omitempty"`
	CallbackURL       *string   `db:"callback_url" json:"callback_url,omitempty"`
	SyncUserOnLogin   bool      `db:"sync_user_on_login" json:"sync_user_on_login"`
	SyncURL           *string   `db:"sync_url" json:"sync_url,omitempty"`
	SyncMethod        string    `db:"sync_method" json:"sync_method"`
	SyncHeaders       *string   `db:"sync_headers" json:"sync_headers,omitempty"`
	CreatedAt         time.Time `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time `db:"updated_at" json:"updated_at"`
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
	ID              string    `db:"id" json:"id"`
	TenantID        string    `db:"tenant_id" json:"tenant_id"`
	Name            string    `db:"name" json:"name"`
	Description     *string   `db:"description" json:"description,omitempty"`
	Tools           *string   `db:"tools" json:"tools,omitempty"`                         // UI菜单权限 JSON数组
	AllowedMCPTools *string   `db:"allowed_mcp_tools" json:"allowed_mcp_tools,omitempty"` // 允许使用的MCP工具ID JSON数组
	AllowedSkills   *string   `db:"allowed_skills" json:"allowed_skills,omitempty"`       // 允许使用的技能ID JSON数组
	RateLimit       *string   `db:"rate_limit" json:"rate_limit,omitempty"`
	DataScope       *string   `db:"data_scope" json:"data_scope,omitempty"`
	IsSystem        bool      `db:"is_system" json:"is_system"`
	IsDefault       bool      `db:"is_default" json:"is_default"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time `db:"updated_at" json:"updated_at"`
}

// UserRole 用户角色关联
type UserRole struct {
	UserID string `db:"user_id" json:"user_id"`
	RoleID string `db:"role_id" json:"role_id"`
}

// Connector 连接器模型
type Connector struct {
	ID            string     `db:"id" json:"id"`
	TenantID      string     `db:"tenant_id" json:"tenant_id"`
	Name          string     `db:"name" json:"name"`
	Type          string     `db:"type" json:"type"`
	BaseURL       string     `db:"base_url" json:"base_url"`
	TransportType *string    `db:"transport_type" json:"transport_type,omitempty"` // sse / streamable_http，MCP传输方式
	MCPServerURL  *string    `db:"mcp_server_url" json:"mcp_server_url,omitempty"`
	Headers       *string    `db:"headers" json:"headers,omitempty"` // JSON: 自定义HTTP头
	AuthType      *string    `db:"auth_type" json:"auth_type,omitempty"`
	AuthConfig    *string    `db:"auth_config" json:"auth_config,omitempty"`
	SpecURL       *string    `db:"spec_url" json:"spec_url,omitempty"`
	SpecContent   *string    `db:"spec_content" json:"spec_content,omitempty"`
	Status        string     `db:"status" json:"status"`
	ToolsCount    int        `db:"tools_count" json:"tools_count"`
	LastSyncAt    *time.Time `db:"last_sync_at" json:"last_sync_at,omitempty"`
	CreatedAt     time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at" json:"updated_at"`
}

// MCPTool MCP工具模型
type MCPTool struct {
	ID            string    `db:"id" json:"id"`
	TenantID      string    `db:"tenant_id" json:"tenant_id"`
	ConnectorID   string    `db:"connector_id" json:"connector_id"`
	Name          string    `db:"name" json:"name"`
	Description   *string   `db:"description" json:"description,omitempty"`
	InputSchema   *string   `db:"input_schema" json:"input_schema,omitempty"`
	BackendMethod *string   `db:"backend_method" json:"backend_method,omitempty"`
	BackendPath   *string   `db:"backend_path" json:"backend_path,omitempty"`
	RiskLevel     string    `db:"risk_level" json:"risk_level"`
	Enabled       bool      `db:"enabled" json:"enabled"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
}

// MemoryPool 记忆池模型 - 记忆的"作用域" + "检索策略"
type MemoryPool struct {
	ID           string    `db:"id" json:"id"`
	TenantID     string    `db:"tenant_id" json:"tenant_id"`
	Name         string    `db:"name" json:"name"`
	Description  *string   `db:"description" json:"description,omitempty"`
	Type         string    `db:"type" json:"type"`                             // personal / team / system
	Purpose      string    `db:"purpose" json:"purpose"`                       // conversation / skill / knowledge
	Priority     int       `db:"priority" json:"priority"`                     // 1-10, 越高越优先
	MaxTokens    int       `db:"max_tokens" json:"max_tokens"`                 // 该池最大注入token数, 0=不限
	AutoActivate bool      `db:"auto_activate" json:"auto_activate"`           // 是否默认激活
	TriggerRules *string   `db:"trigger_rules" json:"trigger_rules,omitempty"` // JSON: 条件触发规则
	OwnerID      *string   `db:"owner_id" json:"owner_id,omitempty"`           // 个人级池的拥有者
	Enabled      bool      `db:"enabled" json:"enabled"`
	MemoryCount  int       `db:"memory_count" json:"memory_count"` // 池中记忆数量（冗余字段）
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
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

// Skill Skill模板模型 - 标准智能体范式
type Skill struct {
	ID                 string     `db:"id" json:"id"`
	TenantID           string     `db:"tenant_id" json:"tenant_id"`
	Name               string     `db:"name" json:"name"`
	Description        *string    `db:"description" json:"description,omitempty"`
	Category           *string    `db:"category" json:"category,omitempty"` // 分类: 数据处理/工作流/API调用/自定义
	Version            string     `db:"version" json:"version"`
	Tags               *string    `db:"tags" json:"tags,omitempty"`                   // JSON数组: 标签
	Triggers           *string    `db:"triggers" json:"triggers,omitempty"`           // JSON: 触发条件
	InputSchema        *string    `db:"input_schema" json:"input_schema,omitempty"`   // JSON Schema: 输入参数定义
	OutputSchema       *string    `db:"output_schema" json:"output_schema,omitempty"` // JSON Schema: 输出定义
	Steps              string     `db:"steps" json:"steps"`                           // JSON数组: 执行步骤定义
	PermissionTopology *string    `db:"permission_topology" json:"permission_topology,omitempty"`
	Status             string     `db:"status" json:"status"`           // draft/active/archived
	UsageCount         int        `db:"usage_count" json:"usage_count"` // 使用次数
	LastUsedAt         *time.Time `db:"last_used_at" json:"last_used_at,omitempty"`
	CreatedBy          *string    `db:"created_by" json:"created_by,omitempty"`
	CreatedAt          time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time  `db:"updated_at" json:"updated_at"`
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

// APIKey API密钥（绑定到用户）
type APIKey struct {
	ID         string     `db:"id" json:"id"`
	TenantID   string     `db:"tenant_id" json:"tenant_id"`
	UserID     string     `db:"user_id" json:"user_id"` // 绑定的用户ID
	Name       string     `db:"name" json:"name"`
	KeyPrefix  string     `db:"key_prefix" json:"key_prefix"`   // 前8字符，用于显示
	KeyHash    string     `db:"key_hash" json:"-"`              // bcrypt hash，不返回给前端
	Scopes     *string    `db:"scopes" json:"scopes,omitempty"` // JSON数组：["chat","sessions"]
	Enabled    bool       `db:"enabled" json:"enabled"`
	ExpiresAt  *time.Time `db:"expires_at" json:"expires_at,omitempty"`
	LastUsedAt *time.Time `db:"last_used_at" json:"last_used_at,omitempty"`
	UsageCount int64      `db:"usage_count" json:"usage_count"`
	CreatedAt  time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at" json:"updated_at"`
}

// EmbedSession Embed API 会话
type EmbedSession struct {
	ID           string    `db:"id" json:"id"`
	TenantID     string    `db:"tenant_id" json:"tenant_id"`
	APIKeyID     string    `db:"api_key_id" json:"api_key_id"`
	VisitorID    string    `db:"visitor_id" json:"visitor_id"`       // 外部访客ID
	Metadata     *string   `db:"metadata" json:"metadata,omitempty"` // JSON: 业务上下文
	MessageCount int       `db:"message_count" json:"message_count"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

// EmbedMessage Embed API 消息
type EmbedMessage struct {
	ID        string    `db:"id" json:"id"`
	SessionID string    `db:"session_id" json:"session_id"`
	Role      string    `db:"role" json:"role"` // user/assistant/system
	Content   string    `db:"content" json:"content"`
	Metadata  *string   `db:"metadata" json:"metadata,omitempty"` // JSON: 工具调用等
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}
