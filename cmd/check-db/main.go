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

	// 列出所有表
	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		log.Fatalf("Failed to show tables: %v", err)
	}
	defer rows.Close()

	fmt.Println("\nTables in database:")
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			log.Printf("Error scanning table name: %v", err)
			continue
		}
		fmt.Printf("  - %s\n", tableName)
	}
}
