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
			version VARCHAR(50) DEFAULT '1.0',
			triggers JSON,
			steps JSON NOT NULL,
			permission_topology JSON,
			status VARCHAR(50) DEFAULT 'active',
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
			type VARCHAR(20) NOT NULL COMMENT 'preference/fact/feedback',
			content TEXT NOT NULL COMMENT '记忆内容',
			embedding LONGBLOB COMMENT '向量嵌入',
			entity_ids JSON COMMENT '关联实体ID列表',
			metadata JSON COMMENT '扩展元数据',
			access_count INT DEFAULT 0 COMMENT '访问次数',
			last_accessed_at DATETIME COMMENT '最后访问时间',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_tenant_user (tenant_id, user_id),
			INDEX idx_type (type)
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
			total_tokens INT NOT NULL DEFAULT 0,
			latency_ms INT NOT NULL DEFAULT 0,
			endpoint VARCHAR(255) NOT NULL DEFAULT '',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_tenant_date (tenant_id, created_at),
			INDEX idx_model (model_provider, model_name)
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
		// connectors 表新增 MCP Server URL
		`ALTER TABLE connectors ADD COLUMN mcp_server_url VARCHAR(500) NULL DEFAULT NULL COMMENT 'MCP Server SSE地址' AFTER base_url`,
		// connectors 表新增传输类型 + 自定义头
		`ALTER TABLE connectors ADD COLUMN transport_type VARCHAR(32) NULL DEFAULT NULL COMMENT 'MCP传输方式: sse / streamable_http' AFTER base_url`,
		`ALTER TABLE connectors ADD COLUMN headers TEXT NULL DEFAULT NULL COMMENT '自定义HTTP头JSON' AFTER mcp_server_url`,

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
		// entities 表新增 pool_id
		`ALTER TABLE entities ADD COLUMN pool_id VARCHAR(36) NULL DEFAULT NULL COMMENT '所属记忆池' AFTER tenant_id`,
		// skill_memories 表新增 pool_id
		`ALTER TABLE skill_memories ADD COLUMN pool_id VARCHAR(36) NULL DEFAULT NULL COMMENT '所属记忆池' AFTER user_id`,
		// ========== 角色权限下沉 ==========
		// roles 表新增 MCP工具和技能权限字段
		`ALTER TABLE roles ADD COLUMN allowed_mcp_tools TEXT NULL DEFAULT NULL COMMENT '允许使用的MCP工具ID JSON数组' AFTER tools`,
		`ALTER TABLE roles ADD COLUMN allowed_skills TEXT NULL DEFAULT NULL COMMENT '允许使用的技能ID JSON数组' AFTER allowed_mcp_tools`,
		// ========== 限流配额 ==========
		`ALTER TABLE tenants ADD COLUMN rate_limit INT NOT NULL DEFAULT 0 COMMENT '每分钟最大请求数，0=不限' AFTER max_users`,
		`ALTER TABLE tenants ADD COLUMN daily_quota INT NOT NULL DEFAULT 0 COMMENT '每日API调用上限，0=不限' AFTER rate_limit`,
		`ALTER TABLE tenants ADD COLUMN monthly_quota INT NOT NULL DEFAULT 0 COMMENT '每月API调用上限，0=不限' AFTER daily_quota`,
		`ALTER TABLE tenants ADD COLUMN daily_token_quota INT NOT NULL DEFAULT 0 COMMENT '每日token消耗上限，0=不限' AFTER monthly_quota`,
		`ALTER TABLE tenants ADD COLUMN monthly_token_quota INT NOT NULL DEFAULT 0 COMMENT '每月token消耗上限，0=不限' AFTER daily_token_quota`,
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
