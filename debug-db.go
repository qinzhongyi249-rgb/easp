package main

import (
	"fmt"
	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/models"
)

func main() {
	// 初始化数据库
	dbConfig, err := database.ConfigFromEnv()
	if err != nil {
		fmt.Printf("Config error: %v\n", err)
		return
	}
	if err := database.Init(dbConfig); err != nil {
		fmt.Printf("Init error: %v\n", err)
		return
	}
	defer database.Close()

	// 查询 tenant_sso_configs
	fmt.Println("=== tenant_sso_configs ===")
	var configs []models.TenantSSOConfig
	err = database.DB.Select(&configs, "SELECT id, tenant_id, enabled, login_url, login_method FROM tenant_sso_configs")
	if err != nil {
		fmt.Printf("Query error: %v\n", err)
		return
	}
	fmt.Printf("Found %d config(s):\n", len(configs))
	for _, c := range configs {
		fmt.Printf("  tenant_id=%s enabled=%v login_url=%s\n", c.TenantID, c.Enabled, c.LoginURL)
	}

	// 查询 external_user_bindings
	fmt.Println("\n=== external_user_bindings ===")
	var count int
	err = database.DB.Get(&count, "SELECT COUNT(*) FROM external_user_bindings")
	if err != nil {
		fmt.Printf("Count error: %v\n", err)
		return
	}
	fmt.Printf("Total %d binding(s)\n", count)

	if count > 0 {
		var bindings []struct {
			TenantID       string `db:"tenant_id"`
			ExternalSystem string `db:"external_system"`
			ExternalUserID string `db:"external_user_id"`
			UserID         string `db:"user_id"`
		}
		err = database.DB.Select(&bindings, "SELECT tenant_id, external_system, external_user_id, user_id FROM external_user_bindings LIMIT 10")
		if err != nil {
			fmt.Printf("Query error: %v\n", err)
			return
		}
		fmt.Printf("Sample (top 10):\n")
		for _, b := range bindings {
			fmt.Printf("  tenant=%s system=%s external_user_id=%s user_id=%s\n", b.TenantID, b.ExternalSystem, b.ExternalUserID, b.UserID)
		}
	}
}
