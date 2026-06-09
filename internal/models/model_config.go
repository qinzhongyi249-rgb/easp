package models

import (
	"time"
)

// ModelProvider 模型提供商
type ModelProvider struct {
	ID          string    `db:"id" json:"id"`
	TenantID    string    `db:"tenant_id" json:"tenant_id"`
	Name        string    `db:"name" json:"name"`
	DisplayName string    `db:"display_name" json:"display_name"`
	BaseURL     string    `db:"base_url" json:"base_url"`
	APIKey      string    `db:"api_key" json:"api_key"`
	Enabled     bool      `db:"enabled" json:"enabled"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// ModelConfig 模型配置
type ModelConfig struct {
	ID          string  `db:"id" json:"id"`
	TenantID    string  `db:"tenant_id" json:"tenant_id"`
	ProviderID  string  `db:"provider_id" json:"provider_id"`
	ModelName   string  `db:"model_name" json:"model_name"`
	DisplayName string  `db:"display_name" json:"display_name"`
	Temperature float64 `db:"temperature" json:"temperature"`
	MaxTokens   int     `db:"max_tokens" json:"max_tokens"`
	IsDefault   bool    `db:"is_default" json:"is_default"`
	Enabled     bool    `db:"enabled" json:"enabled"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// ModelProviderWithConfigs 带配置的提供商
type ModelProviderWithConfigs struct {
	ModelProvider
	Configs []ModelConfig `json:"configs"`
}
