package main

import (
	"fmt"
	"log"

	"github.com/easp-platform/easp/internal/database"
)

func main() {
	dbConfig := database.Config{
		Host:     "rm-8vbh4iqcp8534vs5p6o.mysql.zhangbei.rds.aliyuncs.com",
		Port:     3306,
		User:     "easp_dev",
		Password: "Easp_dev123",
		Database: "easp_dev",
	}

	if err := database.Init(dbConfig); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	migrations := []string{
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

		`CREATE TABLE IF NOT EXISTS skill_executions (
			id VARCHAR(36) PRIMARY KEY,
			skill_id VARCHAR(36) NOT NULL,
			tenant_id VARCHAR(36) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			inputs JSON,
			outputs JSON,
			step_results JSON,
			started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			ended_at TIMESTAMP NULL,
			error TEXT,
			INDEX idx_skill (skill_id),
			INDEX idx_tenant (tenant_id),
			INDEX idx_status (status)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}

	for i, m := range migrations {
		if _, err := database.DB.Exec(m); err != nil {
			log.Printf("Migration %d failed: %v", i+1, err)
		} else {
			fmt.Printf("Migration %d: OK\n", i+1)
		}
	}

	fmt.Println("Done!")
}
