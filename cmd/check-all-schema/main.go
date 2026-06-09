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
