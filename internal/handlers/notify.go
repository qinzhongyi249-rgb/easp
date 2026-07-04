package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// NotifyNewRegistration 通知企业微信有新用户注册（异步，不阻塞响应）
func NotifyNewRegistration(tenantID, tenantName, account, phone, email, displayName string, createdAt time.Time) {
	webhookURL := os.Getenv("EASP_WECOM_WEBHOOK")
	if webhookURL == "" {
		log.Println("[notify] WECOM_WEBHOOK not configured, skip notification")
		return
	}

	go func() {
		msg := buildRegistrationMarkdown(tenantID, tenantName, account, phone, email, displayName, createdAt)
		if err := sendWecomWebhook(webhookURL, msg); err != nil {
			log.Printf("[notify] Failed to send wecom notification: %v", err)
		} else {
			log.Printf("[notify] Registration notification sent for user: %s", account)
		}
	}()
}

func buildRegistrationMarkdown(tenantID, tenantName, account, phone, email, displayName string, createdAt time.Time) map[string]interface{} {
	name := displayName
	if name == "" {
		name = account
	}
	emailStr := email
	if emailStr == "" {
		emailStr = "未填写"
	}

	content := fmt.Sprintf(
		`## 🎉 新用户注册试用 EASP

> 有人刚刚注册了免费试用！

**账号：** %s
**姓名：** %s
**手机号：** <font color="warning">%s</font>
**邮箱：** %s
**租户：** %s (%s)
**注册时间：** %s

[进入 EASP 管理后台](https://easp.jindiyun.com)` +
			"\n\n> 请及时跟进，引导客户完成试用转化。",
		account, name, phone, emailStr, tenantName, tenantID,
		createdAt.Format("2006-01-02 15:04:05"),
	)

	return map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": content,
		},
	}
}

func sendWecomWebhook(webhookURL string, payload map[string]interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("post webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}
