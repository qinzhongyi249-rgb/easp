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
