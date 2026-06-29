package main

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	host := "rm-8vbh4iqcp8534vs5p6o.mysql.zhangbei.rds.aliyuncs.com"
	port := "3306"
	user := "easp_dev"
	pass := "Pachi123456"
	name := "easp_dev"

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True", user, pass, host, port, name)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Printf("Open error: %v\n", err)
		return
	}
	defer db.Close()

	userId := "b7321626-e2e1-475a-b735-f584202884fd"

	var (
		bindingID      string
		externalSystem string
		externalUserID string
		bindingUserID  string
		userID         *string
		account        *string
		email          *string
		displayName    *string
		tenantID       *string
		status         *string
		deletedAt      interface{}
	)

	query := `
	SELECT 
		b.id as binding_id,
		b.external_system,
		b.external_user_id,
		b.user_id,
		u.id as user_id,
		u.account,
		u.email,
		u.display_name,
		u.tenant_id,
		u.status,
		u.deleted_at
	FROM external_user_bindings b
	LEFT JOIN users u ON b.user_id = u.id
	WHERE b.user_id = ? AND b.status = 'active'
	`

	err = db.QueryRow(query, userId).Scan(&bindingID, &externalSystem, &externalUserID, &bindingUserID, &userID, &account, &email, &displayName, &tenantID, &status, &deletedAt)
	if err != nil {
		fmt.Printf("Query error: %v\n", err)
		return
	}

	fmt.Println("=== 外部用户绑定信息 ===")
	fmt.Printf("Binding ID: %s\n", bindingID)
	fmt.Printf("External System: %s\n", externalSystem)
	fmt.Printf("External User ID: %s\n", externalUserID)
	fmt.Printf("Bound User ID: %s\n", bindingUserID)

	fmt.Println("\n=== EASP 内部用户信息 ===")
	if userID == nil || *userID == "" {
		fmt.Println("❌ 用户不存在于 users 表！数据不一致！绑定关系有问题。")
	} else {
		fmt.Printf("User ID: %s\n", *userID)
		fmt.Printf("Account: %s\n", *account)
		fmt.Printf("Email: %s\n", *email)
		fmt.Printf("Display Name: %s\n", *displayName)
		fmt.Printf("Tenant ID: %s\n", *tenantID)
		fmt.Printf("Status: %s\n", *status)
		if deletedAt == nil {
			fmt.Println("Deleted At: NULL (not deleted)")
		} else {
			fmt.Printf("Deleted At: %v (soft deleted)\n", deletedAt)
		}
	}
}
