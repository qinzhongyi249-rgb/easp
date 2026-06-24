package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/easp-platform/easp/internal/database"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	dsn, err := database.DSNFromEnv()
	if err != nil {
		log.Fatalf("Invalid database configuration: %v", err)
	}
	
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	fmt.Println("Database connected successfully!")

	tables := []string{"tenants", "users", "roles", "user_roles", "connectors", "mcp_tools", "memory_pools", "memory_entries", "skills", "audit_logs", "circuit_breaker_states", "abac_rules", "sso_providers"}
	
	for _, table := range tables {
		rows, err := db.Query(fmt.Sprintf("DESCRIBE %s", table))
		if err != nil {
			log.Printf("Failed to describe %s: %v", table, err)
			continue
		}
		
		fmt.Printf("\n=== %s ===\n", table)
		for rows.Next() {
			var field, typ, null, key string
			var defaultVal, extra sql.NullString
			if err := rows.Scan(&field, &typ, &null, &key, &defaultVal, &extra); err != nil {
				log.Printf("Error scanning: %v", err)
				continue
			}
			fmt.Printf("  %-25s %-25s %-10s %-10s\n", field, typ, null, key)
		}
		rows.Close()
	}
}
