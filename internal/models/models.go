// Package models provides shared data types for the EASP open source core.
// This file contains only the structs needed by internal/mcp.
// The full commercial version includes additional models (Tenant, User, Role, etc.).
package models

import "time"

// Connector 连接器模型
type Connector struct {
	ID                   string     `db:"id" json:"id"`
	TenantID             string     `db:"tenant_id" json:"tenant_id"`
	Name                 string     `db:"name" json:"name"`
	Type                 string     `db:"type" json:"type"`
	BaseURL              string     `db:"base_url" json:"base_url"`
	TransportType        *string    `db:"transport_type" json:"transport_type,omitempty"`
	MCPServerURL         *string    `db:"mcp_server_url" json:"mcp_server_url,omitempty"`
	Headers              *string    `db:"headers" json:"headers,omitempty"`
	AuthType             *string    `db:"auth_type" json:"auth_type,omitempty"`
	AuthConfig           *string    `db:"auth_config" json:"auth_config,omitempty"`
	CredentialMode       *string    `db:"credential_mode" json:"credential_mode,omitempty"`
	UserTokenHeader      *string    `db:"user_token_header" json:"user_token_header,omitempty"`
	UserTokenPrefix      *string    `db:"user_token_prefix" json:"user_token_prefix,omitempty"`
	UserTokenRequiredSSO bool       `db:"user_token_required_sso" json:"user_token_required_sso"`
	SpecURL              *string    `db:"spec_url" json:"spec_url,omitempty"`
	SpecContent          *string    `db:"spec_content" json:"spec_content,omitempty"`
	Status               string     `db:"status" json:"status"`
	ToolsCount           int        `db:"tools_count" json:"tools_count"`
	IsBuiltin            bool       `db:"is_builtin" json:"is_builtin"`
	Locked               bool       `db:"locked" json:"locked"`
	LastSyncAt           *time.Time `db:"last_sync_at" json:"last_sync_at,omitempty"`
	CreatedAt            time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt            time.Time  `db:"updated_at" json:"updated_at"`
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
	Status        string    `db:"status" json:"status"`
	Enabled       bool      `db:"enabled" json:"enabled"`
	IsBuiltin     bool      `db:"is_builtin" json:"is_builtin"`
	Locked        bool      `db:"locked" json:"locked"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at" json:"updated_at"`
}
