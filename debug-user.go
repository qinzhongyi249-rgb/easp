package main

import (
	"fmt"
	"os"

	"github.com/easp-platform/easp/internal/database"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	database.Connect()

	userId := "b7321626-e2e1-475a-b735-f584202884fd"

	var exists bool
	err := database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE id = ? AND deleted_at IS NULL)`, userId).Scan(&exists)
	if err != nil {
		fmt.Printf("Query error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Checking user ID: %s\n", userId)
	fmt.Printf("User exists in users table (active): %v\n", exists)

	if exists {
		var account, email, displayName, tenantId string
		var status string
		err := database.DB.QueryRow(`SELECT account, email, display_name, tenant_id, status FROM users WHERE id = ?`, userId).
			Scan(&account, &email, &displayName, &tenantId)
		if err != nil {
			fmt.Printf("Get user info error: %v\n", err)
		} else {
			fmt.Printf("  Account: %s\n", account)
			fmt.Printf("  Email: %s\n", email)
			fmt.Printf("  DisplayName: %s\n", displayName)
			fmt.Printf("  TenantID: %s\n", tenantId)
			fmt.Printf("  Status: %s\n", status)
		}
	}

	// Check binding
	var bindingExists bool
	var externalSystem string
	err = database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM external_user_bindings WHERE user_id = ? AND status = 'active')`, userId).Scan(&bindingExists)
	if err != nil {
		fmt.Printf("Query binding error: %v\n", err)
	} else {
		fmt.Printf("\nActive external binding exists: %v\n", bindingExists)
		if bindingExists {
			err := database.DB.QueryRow(`SELECT external_system FROM external_user_bindings WHERE user_id = ? AND status = 'active'`, userId).Scan(&externalSystem)
			if err == nil {
				fmt.Printf("  ExternalSystem: %s\n", externalSystem)
			}
		}
	}
}