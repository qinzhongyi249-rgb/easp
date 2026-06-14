package database

import (
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

// Config 数据库配置
type Config struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

// DB 全局数据库实例
var DB *sqlx.DB

// Init 初始化数据库连接
func Init(cfg Config) error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)

	var err error
	DB, err = sqlx.Connect("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// 连接池配置
	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(10)
	DB.SetConnMaxLifetime(5 * time.Minute)
	DB.SetConnMaxIdleTime(2 * time.Minute)

	// 测试连接
	if err := DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Database connected successfully")
	return nil
}

// Close 关闭数据库连接
func Close() {
	if DB != nil {
		DB.Close()
	}
}

// GetDB 获取数据库实例
func GetDB() *sqlx.DB {
	return DB
}

// AutoMigrate 自动创建表
func AutoMigrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS tenant_sso_configs (
			id VARCHAR(36) PRIMARY KEY,
			tenant_id VARCHAR(36) NOT NULL,
			enabled BOOLEAN DEFAULT FALSE,
			login_url VARCHAR(500) NOT NULL,
			login_method VARCHAR(10) DEFAULT 'POST',
			login_headers TEXT,
			login_body_template TEXT,
			user_info_url VARCHAR(500),
			user_info_method VARCHAR(10) DEFAULT 'GET',
			user_info_headers TEXT,
			response_mapping TEXT,
			callback_url VARCHAR(500),
			sync_user_on_login BOOLEAN DEFAULT TRUE,
			sync_url VARCHAR(500),
			sync_method VARCHAR(10) DEFAULT 'POST',
			sync_headers TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
			UNIQUE KEY uk_tenant_sso (tenant_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS skills (
			id VARCHAR(36) PRIMARY KEY,
			tenant_id VARCHAR(36) NOT NULL,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			version VARCHAR(50) DEFAULT '1.0.0',
			category VARCHAR(100) NULL DEFAULT NULL,
			tags JSON NULL DEFAULT NULL,
			triggers JSON,
			input_schema JSON NULL DEFAULT NULL,
			output_schema JSON NULL DEFAULT NULL,
			steps JSON NOT NULL,
			permission_topology JSON,
			status VARCHAR(50) NOT NULL DEFAULT 'draft' COMMENT '生命周期: draft/testing/published/disabled',
			usage_count INT NOT NULL DEFAULT 0,
			last_used_at DATETIME NULL DEFAULT NULL,
			created_by VARCHAR(36),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_tenant_id (tenant_id),
			UNIQUE KEY uk_tenant_name (tenant_id, name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS skill_executions (
			id VARCHAR(36) PRIMARY KEY,
			skill_id VARCHAR(36) NOT NULL,
			tenant_id VARCHAR(36) NOT NULL,
			status VARCHAR(50) NOT NULL DEFAULT 'pending',
			execution_mode VARCHAR(20) NOT NULL DEFAULT 'production' COMMENT '执行模式: dry_run/sandbox/production',
			inputs JSON,
			outputs JSON,
			step_results JSON,
			started_at DATETIME NOT NULL,
			ended_at DATETIME,
			error TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_skill_id (skill_id),
			INDEX idx_tenant_id (tenant_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		// 记忆系统表
		`CREATE TABLE IF NOT EXISTS user_memories (
			id VARCHAR(36) PRIMARY KEY,
			tenant_id VARCHAR(36) NOT NULL,
			user_id VARCHAR(36) NOT NULL,
			pool_id VARCHAR(36) NULL DEFAULT NULL COMMENT '所属记忆池',
			type VARCHAR(20) NOT NULL COMMENT 'preference/fact/feedback',
			content TEXT NOT NULL COMMENT '记忆内容',
			content_hash VARCHAR(64) NULL COMMENT '归一化内容SHA256，用于去重',
			source VARCHAR(64) NOT NULL DEFAULT 'manual' COMMENT '来源: manual/auto_extract/tool/import',
			status VARCHAR(32) NOT NULL DEFAULT 'active' COMMENT 'active/disabled/deleted/merged/archived/conflict',
			embedding LONGBLOB COMMENT '向量嵌入',
			entity_ids JSON COMMENT '关联实体ID列表',
			metadata JSON COMMENT '扩展元数据',
			access_count INT DEFAULT 0 COMMENT '访问次数/重复命中次数',
			last_accessed_at DATETIME COMMENT '最后召回时间',
			last_seen_at DATETIME COMMENT '最后重复观察时间',
			vector_indexed_at DATETIME NULL COMMENT '写入向量库时间',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_tenant_user (tenant_id, user_id),
			INDEX idx_type (type),
			INDEX idx_status (status),
			UNIQUE KEY uk_user_memory_hash (tenant_id, user_id, type, content_hash)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS session_memories (
			id VARCHAR(36) PRIMARY KEY,
			tenant_id VARCHAR(36) NOT NULL,
			user_id VARCHAR(36) NOT NULL,
			session_id VARCHAR(36) NOT NULL,
			role VARCHAR(20) NOT NULL COMMENT 'user/assistant/system',
			content TEXT NOT NULL,
			embedding LONGBLOB COMMENT '向量嵌入',
			token_count INT COMMENT 'Token数量',
			entity_ids JSON COMMENT '提到的实体ID列表',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_session (session_id),
			INDEX idx_tenant_user (tenant_id, user_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS assistant_conversations (
			id VARCHAR(36) PRIMARY KEY,
			tenant_id VARCHAR(36) NOT NULL,
			user_id VARCHAR(36) NOT NULL,
			title VARCHAR(255) NOT NULL DEFAULT '',
			page_context JSON,
			message_count INT NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_tenant_user_updated (tenant_id, user_id, updated_at),
			INDEX idx_user_updated (user_id, updated_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS memory_settings (
			id VARCHAR(36) PRIMARY KEY,
			tenant_id VARCHAR(36) NOT NULL,
			user_id VARCHAR(36) NULL,
			auto_extract_enabled TINYINT(1) NOT NULL DEFAULT 1,
			recall_enabled TINYINT(1) NOT NULL DEFAULT 1,
			sensitive_filter_enabled TINYINT(1) NOT NULL DEFAULT 1,
			audit_enabled TINYINT(1) NOT NULL DEFAULT 1,
			hybrid_search_enabled TINYINT(1) NOT NULL DEFAULT 1,
			hybrid_search_mode VARCHAR(32) NOT NULL DEFAULT 'keyword_vector',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_tenant_user (tenant_id, user_id),
			UNIQUE KEY uk_memory_settings_scope (tenant_id, user_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS memory_audit_logs (
			id VARCHAR(36) PRIMARY KEY,
			tenant_id VARCHAR(36) NOT NULL,
			user_id VARCHAR(36) NOT NULL,
			memory_id VARCHAR(36) NULL,
			action VARCHAR(64) NOT NULL,
			source VARCHAR(64) NOT NULL DEFAULT 'unknown',
			original_preview TEXT,
			sanitized_preview TEXT,
			reason TEXT,
			metadata JSON,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_tenant_created (tenant_id, created_at),
			INDEX idx_user_created (user_id, created_at),
			INDEX idx_action (action),
			INDEX idx_memory_id (memory_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS memory_vectors (
			id VARCHAR(36) PRIMARY KEY,
			tenant_id VARCHAR(36) NOT NULL,
			pool_id VARCHAR(36) NOT NULL,
			content TEXT NOT NULL,
			type VARCHAR(50) DEFAULT 'fact',
			sensitivity VARCHAR(20) DEFAULT 'normal',
			metadata JSON,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_tenant_pool (tenant_id, pool_id),
			INDEX idx_type (type)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS entities (
			id VARCHAR(36) PRIMARY KEY,
			tenant_id VARCHAR(36) NOT NULL,
			name VARCHAR(191) NOT NULL COMMENT '实体名称',
			type VARCHAR(50) NOT NULL COMMENT '实体类型: tenant/user/connector/tool/skill',
			ref_id VARCHAR(36) COMMENT '关联的业务ID',
			embedding LONGBLOB COMMENT '向量嵌入',
			metadata JSON COMMENT '实体属性',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_tenant_name_type (tenant_id, name, type),
			INDEX idx_type (type)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS entity_relations (
			id VARCHAR(36) PRIMARY KEY,
			tenant_id VARCHAR(36) NOT NULL,
			source_entity_id VARCHAR(36) NOT NULL,
			target_entity_id VARCHAR(36) NOT NULL,
			relation_type VARCHAR(50) NOT NULL COMMENT '关系类型: belongs_to/uses/manages',
			metadata JSON,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY uk_relation (source_entity_id, target_entity_id, relation_type),
			INDEX idx_source (source_entity_id),
			INDEX idx_target (target_entity_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS skill_memories (
			id VARCHAR(36) PRIMARY KEY,
			tenant_id VARCHAR(36) NOT NULL,
			user_id VARCHAR(36),
			name VARCHAR(255) NOT NULL COMMENT '技能名称',
			description TEXT COMMENT '技能描述',
			content TEXT NOT NULL COMMENT '技能内容 (Markdown)',
			category VARCHAR(50) COMMENT '分类: config/deploy/debug/faq',
			tags JSON COMMENT '标签',
			embedding LONGBLOB COMMENT '向量嵌入',
			usage_count INT DEFAULT 0 COMMENT '使用次数',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_tenant_id (tenant_id),
			INDEX idx_category (category)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS api_usage (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			tenant_id VARCHAR(36) NOT NULL,
			user_id VARCHAR(36) NOT NULL DEFAULT '',
			endpoint VARCHAR(255) NOT NULL,
			method VARCHAR(10) NOT NULL,
			status_code INT NOT NULL DEFAULT 0,
			latency_ms INT NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_tenant_date (tenant_id, created_at),
			INDEX idx_created_at (created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS model_usage (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			tenant_id VARCHAR(36) NOT NULL,
			user_id VARCHAR(36) NOT NULL DEFAULT '',
			model_provider VARCHAR(100) NOT NULL DEFAULT '',
			model_name VARCHAR(100) NOT NULL,
			input_tokens INT NOT NULL DEFAULT 0,
			output_tokens INT NOT NULL DEFAULT 0,
			cached_tokens INT NOT NULL DEFAULT 0 COMMENT '命中缓存的输入token数',
			total_tokens INT NOT NULL DEFAULT 0,
			latency_ms INT NOT NULL DEFAULT 0,
			endpoint VARCHAR(255) NOT NULL DEFAULT '',
			source VARCHAR(32) NOT NULL DEFAULT 'unknown' COMMENT '调用来源: ai_assistant/embed/mcp_api/skill/manual/unknown',
			source_name VARCHAR(100) NOT NULL DEFAULT '' COMMENT '来源名称',
			resource_type VARCHAR(32) NOT NULL DEFAULT '' COMMENT '资源类型: assistant/embed/mcp_tool/skill/builtin_tool',
			resource_id VARCHAR(64) NOT NULL DEFAULT '' COMMENT '资源ID',
			request_id VARCHAR(64) NOT NULL DEFAULT '' COMMENT '请求链路ID',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_tenant_date (tenant_id, created_at),
			INDEX idx_model (model_provider, model_name),
			INDEX idx_source (tenant_id, source, created_at),
			INDEX idx_resource (tenant_id, resource_type, resource_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS tool_call_usage (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			tenant_id VARCHAR(36) NOT NULL,
			user_id VARCHAR(36) NOT NULL DEFAULT '',
			resource_type VARCHAR(32) NOT NULL COMMENT 'mcp_tool/skill/builtin_tool',
			resource_id VARCHAR(64) NOT NULL DEFAULT '',
			resource_name VARCHAR(128) NOT NULL DEFAULT '',
			source VARCHAR(32) NOT NULL DEFAULT '' COMMENT 'assistant/skill/mcp_api/embed/manual',
			status VARCHAR(32) NOT NULL DEFAULT 'success',
			latency_ms INT NOT NULL DEFAULT 0,
			request_id VARCHAR(64) NOT NULL DEFAULT '',
			error_message TEXT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_tenant_date (tenant_id, created_at),
			INDEX idx_resource (tenant_id, resource_type, resource_id),
			INDEX idx_source (tenant_id, source, created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}

	for _, migration := range migrations {
		if _, err := DB.Exec(migration); err != nil {
			log.Printf("Migration failed: %v", err)
			// 继续执行其他迁移
		}
	}

	// ALTER TABLE 添加新字段（幂等，字段已存在会报错但不影响）
	alters := []string{
		// tenants 表新增字段
		`ALTER TABLE tenants ADD COLUMN expires_at DATETIME NULL DEFAULT NULL COMMENT '到期时间，NULL=永久有效' AFTER status`,
		`ALTER TABLE tenants ADD COLUMN max_users INT NOT NULL DEFAULT 0 COMMENT '最大用户数，0=不限制' AFTER expires_at`,
		// users 表新增字段
		`ALTER TABLE users ADD COLUMN deleted_at DATETIME NULL DEFAULT NULL COMMENT '逻辑删除时间' AFTER login_count`,
		`ALTER TABLE users ADD COLUMN email_unique_key VARCHAR(255) NULL DEFAULT NULL COMMENT '租户内邮箱唯一键' AFTER email`,
		`ALTER TABLE users ADD COLUMN phone_unique_key VARCHAR(255) NULL DEFAULT NULL COMMENT '租户内手机号唯一键' AFTER phone`,
		`UPDATE users SET email_unique_key = CONCAT(tenant_id, ':', email) WHERE email IS NOT NULL AND email <> '' AND email_unique_key IS NULL`,
		`UPDATE users SET phone_unique_key = CONCAT(tenant_id, ':', phone) WHERE phone IS NOT NULL AND phone <> '' AND phone_unique_key IS NULL`,
		`ALTER TABLE users DROP INDEX email`,
		`ALTER TABLE users DROP INDEX uk_users_email`,
		`ALTER TABLE users ADD UNIQUE KEY uk_users_tenant_email (email_unique_key)`,
		`ALTER TABLE users ADD UNIQUE KEY uk_users_tenant_phone (phone_unique_key)`,
		`ALTER TABLE users ADD INDEX idx_users_email (email)`,
		`ALTER TABLE users ADD INDEX idx_users_phone (phone)`,
		// connectors 表新增 MCP Server URL
		`ALTER TABLE connectors ADD COLUMN mcp_server_url VARCHAR(500) NULL DEFAULT NULL COMMENT 'MCP Server SSE地址' AFTER base_url`,
		// connectors 表新增传输类型 + 自定义头
		`ALTER TABLE connectors ADD COLUMN transport_type VARCHAR(32) NULL DEFAULT NULL COMMENT 'MCP传输方式: sse / streamable_http' AFTER base_url`,
		`ALTER TABLE connectors ADD COLUMN headers TEXT NULL DEFAULT NULL COMMENT '自定义HTTP头JSON' AFTER mcp_server_url`,
		`ALTER TABLE connectors ADD COLUMN credential_mode VARCHAR(32) NOT NULL DEFAULT 'static' COMMENT '凭据模式: static/user_token/none' AFTER auth_config`,
		`ALTER TABLE connectors ADD COLUMN user_token_header VARCHAR(128) NULL DEFAULT 'Authorization' COMMENT '用户Token透传Header名' AFTER credential_mode`,
		`ALTER TABLE connectors ADD COLUMN user_token_prefix VARCHAR(64) NULL DEFAULT 'Bearer' COMMENT '用户Token前缀' AFTER user_token_header`,
		`ALTER TABLE connectors ADD COLUMN user_token_required_sso TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否要求SSO登录Token' AFTER user_token_prefix`,
		`CREATE TABLE IF NOT EXISTS user_sso_tokens (
			id VARCHAR(36) PRIMARY KEY,
			tenant_id VARCHAR(36) NOT NULL,
			user_id VARCHAR(36) NOT NULL,
			token_ciphertext TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_user_sso_token (tenant_id, user_id),
			INDEX idx_tenant_user (tenant_id, user_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// ========== 记忆池重构 ==========
		// memory_pools 表新增字段
		`ALTER TABLE memory_pools ADD COLUMN description TEXT NULL DEFAULT NULL COMMENT '描述' AFTER name`,
		`ALTER TABLE memory_pools ADD COLUMN type VARCHAR(32) NOT NULL DEFAULT 'personal' COMMENT '类型: personal/team/system' AFTER description`,
		`ALTER TABLE memory_pools ADD COLUMN purpose VARCHAR(32) NOT NULL DEFAULT 'conversation' COMMENT '用途: conversation/skill/knowledge' AFTER type`,
		`ALTER TABLE memory_pools ADD COLUMN priority INT NOT NULL DEFAULT 5 COMMENT '优先级 1-10' AFTER purpose`,
		`ALTER TABLE memory_pools ADD COLUMN max_tokens INT NOT NULL DEFAULT 0 COMMENT '最大注入token数, 0=不限' AFTER priority`,
		`ALTER TABLE memory_pools ADD COLUMN auto_activate TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否默认激活' AFTER max_tokens`,
		`ALTER TABLE memory_pools ADD COLUMN trigger_rules TEXT NULL DEFAULT NULL COMMENT 'JSON: 条件触发规则' AFTER auto_activate`,
		`ALTER TABLE memory_pools ADD COLUMN enabled TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否启用' AFTER trigger_rules`,
		`ALTER TABLE memory_pools ADD COLUMN memory_count INT NOT NULL DEFAULT 0 COMMENT '池中记忆数量' AFTER enabled`,
		`ALTER TABLE memory_pools ADD COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '更新时间' AFTER created_at`,
		// user_memories 表新增 pool_id
		`ALTER TABLE user_memories ADD COLUMN pool_id VARCHAR(36) NULL DEFAULT NULL COMMENT '所属记忆池' AFTER user_id`,
		// user_memories 记忆治理字段
		`ALTER TABLE user_memories ADD COLUMN content_hash VARCHAR(64) NULL COMMENT '归一化内容SHA256，用于去重' AFTER content`,
		`ALTER TABLE user_memories ADD COLUMN source VARCHAR(64) NOT NULL DEFAULT 'manual' COMMENT '来源: manual/auto_extract/tool/import' AFTER content_hash`,
		`ALTER TABLE user_memories ADD COLUMN status VARCHAR(32) NOT NULL DEFAULT 'active' COMMENT 'active/disabled/deleted' AFTER source`,
		`ALTER TABLE user_memories ADD COLUMN last_seen_at DATETIME NULL COMMENT '最后重复观察时间' AFTER last_accessed_at`,
		`ALTER TABLE user_memories ADD COLUMN vector_indexed_at DATETIME NULL COMMENT '写入向量库时间' AFTER last_seen_at`,
		`CREATE INDEX idx_user_memories_status ON user_memories (status)`,
		`CREATE UNIQUE INDEX uk_user_memory_hash ON user_memories (tenant_id, user_id, type, content_hash)`,
		// 记忆治理表
		`CREATE TABLE IF NOT EXISTS memory_settings (
			id VARCHAR(36) PRIMARY KEY,
			tenant_id VARCHAR(36) NOT NULL,
			user_id VARCHAR(36) NULL,
			auto_extract_enabled TINYINT(1) NOT NULL DEFAULT 1,
			recall_enabled TINYINT(1) NOT NULL DEFAULT 1,
			sensitive_filter_enabled TINYINT(1) NOT NULL DEFAULT 1,
			audit_enabled TINYINT(1) NOT NULL DEFAULT 1,
			hybrid_search_enabled TINYINT(1) NOT NULL DEFAULT 1,
			hybrid_search_mode VARCHAR(32) NOT NULL DEFAULT 'keyword_vector',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_tenant_user (tenant_id, user_id),
			UNIQUE KEY uk_memory_settings_scope (tenant_id, user_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS memory_audit_logs (
			id VARCHAR(36) PRIMARY KEY,
			tenant_id VARCHAR(36) NOT NULL,
			user_id VARCHAR(36) NOT NULL,
			memory_id VARCHAR(36) NULL,
			action VARCHAR(64) NOT NULL,
			source VARCHAR(64) NOT NULL DEFAULT 'unknown',
			original_preview TEXT,
			sanitized_preview TEXT,
			reason TEXT,
			metadata JSON,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_tenant_created (tenant_id, created_at),
			INDEX idx_user_created (user_id, created_at),
			INDEX idx_action (action),
			INDEX idx_memory_id (memory_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS memory_vectors (
			id VARCHAR(36) PRIMARY KEY,
			tenant_id VARCHAR(36) NOT NULL,
			pool_id VARCHAR(36) NOT NULL,
			content TEXT NOT NULL,
			type VARCHAR(50) DEFAULT 'fact',
			sensitivity VARCHAR(20) DEFAULT 'normal',
			metadata JSON,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_tenant_pool (tenant_id, pool_id),
			INDEX idx_type (type)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		// entities 表新增 pool_id
		`ALTER TABLE entities ADD COLUMN pool_id VARCHAR(36) NULL DEFAULT NULL COMMENT '所属记忆池' AFTER tenant_id`,
		// skill_memories 表新增 pool_id
		`ALTER TABLE skill_memories ADD COLUMN pool_id VARCHAR(36) NULL DEFAULT NULL COMMENT '所属记忆池' AFTER user_id`,
		// ========== 角色权限下沉 ==========
		// roles 表新增 MCP工具和技能权限字段
		`ALTER TABLE roles ADD COLUMN allowed_mcp_tools TEXT NULL DEFAULT NULL COMMENT '允许使用的MCP工具ID JSON数组' AFTER tools`,
		`ALTER TABLE roles ADD COLUMN allowed_skills TEXT NULL DEFAULT NULL COMMENT '允许使用的技能ID JSON数组' AFTER allowed_mcp_tools`,
		// ========== 技能标准范式 ==========
		`ALTER TABLE skills ADD COLUMN category VARCHAR(100) NULL DEFAULT NULL COMMENT '技能分类' AFTER description`,
		`ALTER TABLE skills ADD COLUMN tags JSON NULL DEFAULT NULL COMMENT '标签JSON数组' AFTER version`,
		`ALTER TABLE skills ADD COLUMN input_schema JSON NULL DEFAULT NULL COMMENT '输入参数Schema' AFTER triggers`,
		`ALTER TABLE skills ADD COLUMN output_schema JSON NULL DEFAULT NULL COMMENT '输出Schema' AFTER input_schema`,
		`ALTER TABLE skills ADD COLUMN usage_count INT NOT NULL DEFAULT 0 COMMENT '使用次数' AFTER status`,
		`ALTER TABLE skills ADD COLUMN last_used_at DATETIME NULL DEFAULT NULL COMMENT '最后使用时间' AFTER usage_count`,
		`ALTER TABLE skills MODIFY COLUMN status VARCHAR(50) NOT NULL DEFAULT 'draft' COMMENT '生命周期: draft/testing/published/disabled'`,
		`UPDATE skills SET status = 'published' WHERE status = 'active'`,
		`UPDATE skills SET status = 'disabled' WHERE status = 'archived'`,
		`ALTER TABLE mcp_tools ADD COLUMN status VARCHAR(50) NOT NULL DEFAULT 'draft' COMMENT '生命周期: draft/testing/published/disabled' AFTER risk_level`,
		`ALTER TABLE mcp_tools ADD COLUMN is_builtin TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否系统内置工具' AFTER enabled`,
		`ALTER TABLE mcp_tools ADD COLUMN locked TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否锁定不可编辑删除' AFTER is_builtin`,
		`ALTER TABLE mcp_tools ADD COLUMN updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间' AFTER created_at`,
		`UPDATE mcp_tools SET status = 'published' WHERE enabled = 1 AND status = 'draft'`,
		`ALTER TABLE skill_executions ADD COLUMN execution_mode VARCHAR(20) NOT NULL DEFAULT 'production' COMMENT '执行模式: dry_run/sandbox/production' AFTER status`,
		`CREATE INDEX idx_skill_executions_mode ON skill_executions (tenant_id, execution_mode, created_at)`,
		// ========== 限流配额 ==========
		`ALTER TABLE tenants ADD COLUMN rate_limit INT NOT NULL DEFAULT 0 COMMENT '每分钟最大请求数，0=不限' AFTER max_users`,
		`ALTER TABLE tenants ADD COLUMN daily_quota INT NOT NULL DEFAULT 0 COMMENT '每日API调用上限，0=不限' AFTER rate_limit`,
		`ALTER TABLE tenants ADD COLUMN monthly_quota INT NOT NULL DEFAULT 0 COMMENT '每月API调用上限，0=不限' AFTER daily_quota`,
		`ALTER TABLE tenants ADD COLUMN daily_token_quota INT NOT NULL DEFAULT 0 COMMENT '每日token消耗上限，0=不限' AFTER monthly_quota`,
		`ALTER TABLE tenants ADD COLUMN monthly_token_quota INT NOT NULL DEFAULT 0 COMMENT '每月token消耗上限，0=不限' AFTER daily_token_quota`,
		// ========== 用量分析 ==========
		`ALTER TABLE model_usage ADD COLUMN cached_tokens INT NOT NULL DEFAULT 0 COMMENT '命中缓存的输入token数' AFTER output_tokens`,
		`ALTER TABLE model_usage ADD COLUMN source VARCHAR(32) NOT NULL DEFAULT 'unknown' COMMENT '调用来源: ai_assistant/embed/mcp_api/skill/manual/unknown' AFTER endpoint`,
		`ALTER TABLE model_usage ADD COLUMN source_name VARCHAR(100) NOT NULL DEFAULT '' COMMENT '来源名称' AFTER source`,
		`ALTER TABLE model_usage ADD COLUMN resource_type VARCHAR(32) NOT NULL DEFAULT '' COMMENT '资源类型' AFTER source_name`,
		`ALTER TABLE model_usage ADD COLUMN resource_id VARCHAR(64) NOT NULL DEFAULT '' COMMENT '资源ID' AFTER resource_type`,
		`ALTER TABLE model_usage ADD COLUMN request_id VARCHAR(64) NOT NULL DEFAULT '' COMMENT '请求链路ID' AFTER resource_id`,
		`CREATE INDEX idx_model_usage_source ON model_usage (tenant_id, source, created_at)`,
		`CREATE INDEX idx_model_usage_resource ON model_usage (tenant_id, resource_type, resource_id)`,
		// ========== API Key ==========
		`ALTER TABLE api_keys ADD COLUMN user_id VARCHAR(36) NOT NULL DEFAULT '' COMMENT '绑定的用户ID' AFTER tenant_id`,
		`CREATE TABLE IF NOT EXISTS api_keys (
			id VARCHAR(36) PRIMARY KEY,
			tenant_id VARCHAR(36) NOT NULL,
			user_id VARCHAR(36) NOT NULL COMMENT '绑定的用户ID',
			name VARCHAR(100) NOT NULL,
			key_prefix VARCHAR(16) NOT NULL COMMENT 'Key前缀，用于显示',
			key_hash VARCHAR(255) NOT NULL COMMENT 'bcrypt hash',
			scopes JSON COMMENT '权限范围: ["chat","sessions"]',
			enabled TINYINT(1) NOT NULL DEFAULT 1,
			expires_at DATETIME NULL,
			last_used_at DATETIME NULL,
			usage_count BIGINT NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_tenant_id (tenant_id),
			INDEX idx_user_id (user_id),
			UNIQUE INDEX idx_key_prefix (key_prefix)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS embed_sessions (
			id VARCHAR(36) PRIMARY KEY,
			tenant_id VARCHAR(36) NOT NULL,
			api_key_id VARCHAR(36) NOT NULL,
			visitor_id VARCHAR(100) NOT NULL COMMENT '外部访客ID',
			metadata JSON COMMENT '业务上下文',
			message_count INT NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_tenant_id (tenant_id),
			INDEX idx_api_key_id (api_key_id),
			INDEX idx_visitor (tenant_id, visitor_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS embed_messages (
			id VARCHAR(36) PRIMARY KEY,
			session_id VARCHAR(36) NOT NULL,
			role VARCHAR(20) NOT NULL COMMENT 'user/assistant/system',
			content TEXT NOT NULL,
			metadata JSON COMMENT '工具调用等扩展信息',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_session_id (session_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}

	for _, alter := range alters {
		if _, err := DB.Exec(alter); err != nil {
			// 错误码 1060 = Duplicate column name，字段已存在，忽略
			log.Printf("ALTER TABLE (may already exist): %v", err)
		}
	}

	log.Println("Database migrations completed")
	return nil
}
