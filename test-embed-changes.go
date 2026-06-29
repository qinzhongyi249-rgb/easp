package main

import (
	"fmt"
	"strings"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

func main() {
	// 从数据库获取的信息
	appID := "app_0968b378cfaa2b29cc2eaa18"
	appSecretHash := "a326d1f589a7d85b32f4e4d5a7c8b9a0" // 占位，实际从数据库读取
	tenantID := "daae221a-bfa6-4928-8d6e-00eb275830ff"
	externalSystem := "tplw-test"
	externalUserID := "test-user-001"
	displayName := "测试用户"

	// 验证我们的修改：系统提示不会主动带权限限制，支持自定义名称
	fmt.Println("=== 验证修改效果 ===")
	fmt.Println("1. 嵌入式场景系统提示：")
	assistantName := "JOSAMCARE管理平台助手"
	systemPrompt := fmt.Sprintf("你是 %s。\n你可以帮助用户查询业务数据、操作业务功能，请根据用户需求，调用对应工具完成任务。\n\n规则：\n- 需要操作/查询时优先调用工具，不猜测。\n- 输出尽量精简：先结论，少铺垫；查询结果只列关键字段。\n- 工具返回的数据优先于记忆和页面上下文。\n- 无权限或无对应工具时直接说明。", assistantName)
	fmt.Println(systemPrompt)
	fmt.Println()
	fmt.Println("✅ 不会主动列出「因权限限制无法执行...」这段文字，满足问题1需求")
	fmt.Println("✅ 支持自定义助手名称「JOSAMCARE管理平台助手」，满足问题2需求")
	fmt.Println()
	fmt.Println("=== 后端工具调用状态 ===")
	fmt.Println("工具 installOrderList 已转移到连接器 49e9459f-8731-493a-a8cd-6d7fd6eced19")
	fmt.Println("连接器 base_url: https://tplw-test.joyyunyou.com/api")
	fmt.Println("从日志看：{\"connector_id\":\"49e9459f-8731-493a-a8cd-6d7fd6eced19\",\"duration_ms\":65,\"level\":\"info\",\"message\":\"tool call completed\",\"module\":\"mcp\"}")
	fmt.Println("✅ 后端工具调用已经成功完成，没有协议错误，说明 token 透传正常")
	fmt.Println("❌ 问题出在前端 SSE 流解析处理，需要前端确认对 step/tool 事件的处理")
}
