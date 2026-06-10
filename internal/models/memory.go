package models

import (
	"encoding/json"
	"time"
)

// UserMemory 用户记忆
type UserMemory struct {
	ID             string          `json:"id" db:"id"`
	TenantID       string          `json:"tenant_id" db:"tenant_id"`
	UserID         string          `json:"user_id" db:"user_id"`
	PoolID         *string         `json:"pool_id" db:"pool_id"` // 所属记忆池
	Type           string          `json:"type" db:"type"` // preference/fact/feedback
	Content        string          `json:"content" db:"content"`
	Embedding      []byte          `json:"-" db:"embedding"`
	EntityIDs      json.RawMessage `json:"entity_ids" db:"entity_ids"`
	Metadata       json.RawMessage `json:"metadata" db:"metadata"`
	AccessCount    int             `json:"access_count" db:"access_count"`
	LastAccessedAt *time.Time      `json:"last_accessed_at" db:"last_accessed_at"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at" db:"updated_at"`
}

// SessionMemory 会话记忆
type SessionMemory struct {
	ID        string          `json:"id" db:"id"`
	TenantID  string          `json:"tenant_id" db:"tenant_id"`
	UserID    string          `json:"user_id" db:"user_id"`
	SessionID string          `json:"session_id" db:"session_id"`
	Role      string          `json:"role" db:"role"` // user/assistant/system
	Content   string          `json:"content" db:"content"`
	Embedding []byte          `json:"-" db:"embedding"`
	TokenCount *int           `json:"token_count" db:"token_count"`
	EntityIDs json.RawMessage `json:"entity_ids" db:"entity_ids"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
}

// Entity 实体
type Entity struct {
	ID        string          `json:"id" db:"id"`
	TenantID  string          `json:"tenant_id" db:"tenant_id"`
	PoolID    *string         `json:"pool_id" db:"pool_id"` // 所属记忆池
	Name      string          `json:"name" db:"name"`
	Type      string          `json:"type" db:"type"` // tenant/user/connector/tool/skill
	RefID     *string         `json:"ref_id" db:"ref_id"`
	Embedding []byte          `json:"-" db:"embedding"`
	Metadata  json.RawMessage `json:"metadata" db:"metadata"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
}

// EntityRelation 实体关系
type EntityRelation struct {
	ID             string          `json:"id" db:"id"`
	TenantID       string          `json:"tenant_id" db:"tenant_id"`
	SourceEntityID string          `json:"source_entity_id" db:"source_entity_id"`
	TargetEntityID string          `json:"target_entity_id" db:"target_entity_id"`
	RelationType   string          `json:"relation_type" db:"relation_type"` // belongs_to/uses/manages
	Metadata       json.RawMessage `json:"metadata" db:"metadata"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
}

// SkillMemory 技能记忆
type SkillMemory struct {
	ID          string          `json:"id" db:"id"`
	TenantID    string          `json:"tenant_id" db:"tenant_id"`
	UserID      *string         `json:"user_id" db:"user_id"`
	PoolID      *string         `json:"pool_id" db:"pool_id"` // 所属记忆池
	Name        string          `json:"name" db:"name"`
	Description *string         `json:"description" db:"description"`
	Content     string          `json:"content" db:"content"`
	Category    *string         `json:"category" db:"category"`
	Tags        json.RawMessage `json:"tags" db:"tags"`
	Embedding   []byte          `json:"-" db:"embedding"`
	UsageCount  int             `json:"usage_count" db:"usage_count"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}

// MemorySearchResult 记忆搜索结果
type MemorySearchResult struct {
	Memory    interface{} `json:"memory"`
	Score     float64     `json:"score"`
	Source    string      `json:"source"` // semantic/keyword/entity
}

// MemoryStats 记忆统计
type MemoryStats struct {
	TotalUserMemories    int `json:"total_user_memories"`
	TotalSessionMemories int `json:"total_session_memories"`
	TotalEntities        int `json:"total_entities"`
	TotalSkillMemories   int `json:"total_skill_memories"`
	ByType               map[string]int `json:"by_type"`
}
