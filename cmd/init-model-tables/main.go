package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	dsn := "easp_dev:Easp_dev123@tcp(rm-8vbh4iqcp8534vs5p6o.mysql.zhangbei.rds.aliyuncs.com:3306)/easp_dev?charset=utf8mb4&parseTime=True&loc=Local"
	
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// 创建model_providers表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS model_providers (
			id VARCHAR(36) PRIMARY KEY,
			tenant_id VARCHAR(36) NOT NULL,
			name VARCHAR(100) NOT NULL,
			display_name VARCHAR(100),
			base_url TEXT NOT NULL,
			api_key TEXT NOT NULL,
			enabled BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_tenant_id (tenant_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`)
	if err != nil {
		log.Fatalf("Failed to create model_providers table: %v", err)
	}
	fmt.Println("✓ model_providers table created")

	// 创建model_configs表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS model_configs (
			id VARCHAR(36) PRIMARY KEY,
			tenant_id VARCHAR(36) NOT NULL,
			provider_id VARCHAR(36) NOT NULL,
			model_name VARCHAR(100) NOT NULL,
			display_name VARCHAR(100),
			temperature DECIMAL(3,2) DEFAULT 1.00,
			max_tokens INT DEFAULT 4096,
			is_default BOOLEAN DEFAULT FALSE,
			enabled BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_tenant_id (tenant_id),
			INDEX idx_provider_id (provider_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`)
	if err != nil {
		log.Fatalf("Failed to create model_configs table: %v", err)
	}
	fmt.Println("✓ model_configs table created")

	// 插入默认数据
	// 获取默认租户ID
	var tenantID string
	err = db.QueryRow("SELECT id FROM tenants LIMIT 1").Scan(&tenantID)
	if err != nil {
		log.Fatalf("Failed to get tenant: %v", err)
	}

	// 插入默认提供商
	_, err = db.Exec(`
		INSERT IGNORE INTO model_providers (id, tenant_id, name, display_name, base_url, api_key, enabled)
		VALUES ('default-apigo', ?, 'apigo', 'APIGo AI', 'https://maas.apigo.ai/v1', 'sk-platform-228fe8d21e2a407f3f35ecf5e1ea72ca3adb23f3023432d2', TRUE)
	`, tenantID)
	if err != nil {
		log.Fatalf("Failed to insert default provider: %v", err)
	}
	fmt.Println("✓ Default provider inserted")

	// 插入默认模型配置
	_, err = db.Exec(`
		INSERT IGNORE INTO model_configs (id, tenant_id, provider_id, model_name, display_name, temperature, max_tokens, is_default, enabled)
		VALUES ('default-claude', ?, 'default-apigo', 'claude-opus-4-7', 'Claude Opus 4', 1.00, 4096, TRUE, TRUE)
	`, tenantID)
	if err != nil {
		log.Fatalf("Failed to insert default model config: %v", err)
	}
	fmt.Println("✓ Default model config inserted")

	fmt.Println("\nAll tables created successfully!")
}
