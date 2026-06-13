package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/middleware"
	"github.com/gin-gonic/gin"
)

// UsageHandler 使用量统计处理器
type UsageHandler struct{}

func NewUsageHandler() *UsageHandler {
	return &UsageHandler{}
}

// UsageStats 使用量统计响应
type UsageStats struct {
	// API 调用统计
	TodayAPICalls int `json:"today_api_calls"`
	MonthAPICalls int `json:"month_api_calls"`
	DailyQuota    int `json:"daily_quota"`   // 0=不限
	MonthlyQuota  int `json:"monthly_quota"` // 0=不限
	RateLimit     int `json:"rate_limit"`    // 每分钟，0=不限

	// Token 消耗统计
	TodayInputTokens  int `json:"today_input_tokens"`
	TodayOutputTokens int `json:"today_output_tokens"`
	TodayCachedTokens int `json:"today_cached_tokens"`
	TodayTotalTokens  int `json:"today_total_tokens"`
	MonthInputTokens  int `json:"month_input_tokens"`
	MonthOutputTokens int `json:"month_output_tokens"`
	MonthCachedTokens int `json:"month_cached_tokens"`
	MonthTotalTokens  int `json:"month_total_tokens"`
	DailyTokenQuota   int `json:"daily_token_quota"`   // 0=不限
	MonthlyTokenQuota int `json:"monthly_token_quota"` // 0=不限

	// 按模型分组的 token 消耗
	ModelUsage []ModelUsageStats `json:"model_usage"`
}

// ModelUsageStats 按模型分组的使用量
type ModelUsageStats struct {
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	TodayTokens  int    `json:"today_tokens"`
	MonthTokens  int    `json:"month_tokens"`
	TodayCalls   int    `json:"today_calls"`
	MonthCalls   int    `json:"month_calls"`
	InputTokens  int    `json:"month_input_tokens"`
	OutputTokens int    `json:"month_output_tokens"`
	CachedTokens int    `json:"month_cached_tokens"`
}

// GetUsageStats 获取租户使用量统计
func (h *UsageHandler) GetUsageStats(c *gin.Context) {
	tenantID := c.Param("tenantId")

	// 获取租户配额配置
	var rateLimit, dailyQuota, monthlyQuota, dailyTokenQuota, monthlyTokenQuota int
	err := database.DB.QueryRow(`
		SELECT COALESCE(rate_limit, 0), COALESCE(daily_quota, 0), COALESCE(monthly_quota, 0),
		       COALESCE(daily_token_quota, 0), COALESCE(monthly_token_quota, 0)
		FROM tenants WHERE id = ?`, tenantID).Scan(&rateLimit, &dailyQuota, &monthlyQuota, &dailyTokenQuota, &monthlyTokenQuota)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tenant not found"})
		return
	}

	stats := UsageStats{
		RateLimit:         rateLimit,
		DailyQuota:        dailyQuota,
		MonthlyQuota:      monthlyQuota,
		DailyTokenQuota:   dailyTokenQuota,
		MonthlyTokenQuota: monthlyTokenQuota,
	}

	today := time.Now().Format("2006-01-02")
	monthStart := time.Now().Format("2006-01") + "-01"

	// API 调用次数统计
	database.DB.Get(&stats.TodayAPICalls,
		"SELECT COUNT(*) FROM api_usage WHERE tenant_id = ? AND DATE(created_at) = ?", tenantID, today)
	database.DB.Get(&stats.MonthAPICalls,
		"SELECT COUNT(*) FROM api_usage WHERE tenant_id = ? AND created_at >= ?", tenantID, monthStart)

	// Token 消耗统计（当日）
	database.DB.QueryRow(`
		SELECT COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COALESCE(SUM(cached_tokens), 0), COALESCE(SUM(total_tokens), 0)
		FROM model_usage WHERE tenant_id = ? AND DATE(created_at) = ?`, tenantID, today).
		Scan(&stats.TodayInputTokens, &stats.TodayOutputTokens, &stats.TodayCachedTokens, &stats.TodayTotalTokens)

	// Token 消耗统计（当月）
	database.DB.QueryRow(`
		SELECT COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COALESCE(SUM(cached_tokens), 0), COALESCE(SUM(total_tokens), 0)
		FROM model_usage WHERE tenant_id = ? AND created_at >= ?`, tenantID, monthStart).
		Scan(&stats.MonthInputTokens, &stats.MonthOutputTokens, &stats.MonthCachedTokens, &stats.MonthTotalTokens)

	// 按模型分组统计
	rows, err := database.DB.Query(`
		SELECT model_provider, model_name,
		       SUM(CASE WHEN DATE(created_at) = ? THEN total_tokens ELSE 0 END) as today_tokens,
		       SUM(total_tokens) as month_tokens,
		       SUM(CASE WHEN DATE(created_at) = ? THEN 1 ELSE 0 END) as today_calls,
		       COUNT(*) as month_calls,
		       SUM(input_tokens) as month_input_tokens,
		       SUM(output_tokens) as month_output_tokens,
		       SUM(cached_tokens) as month_cached_tokens
		FROM model_usage
		WHERE tenant_id = ? AND created_at >= ?
		GROUP BY model_provider, model_name
		ORDER BY month_tokens DESC`, today, today, tenantID, monthStart)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var m ModelUsageStats
			if err := rows.Scan(&m.Provider, &m.Model, &m.TodayTokens, &m.MonthTokens,
				&m.TodayCalls, &m.MonthCalls, &m.InputTokens, &m.OutputTokens, &m.CachedTokens); err == nil {
				stats.ModelUsage = append(stats.ModelUsage, m)
			}
		}
	}
	if stats.ModelUsage == nil {
		stats.ModelUsage = []ModelUsageStats{}
	}

	c.JSON(http.StatusOK, stats)
}

// RecordAPIUsage 异步记录 API 调用（供中间件调用）
func RecordAPIUsage(tenantID, userID, endpoint, method string, statusCode, latencyMs int) {
	go func() {
		_, err := database.DB.Exec(`
			INSERT INTO api_usage (tenant_id, user_id, endpoint, method, status_code, latency_ms, created_at)
			VALUES (?, ?, ?, ?, ?, ?, NOW())`,
			tenantID, userID, endpoint, method, statusCode, latencyMs)
		if err != nil {
			log.Printf("RecordAPIUsage failed: %v", err)
		}
	}()
}

// RecordModelUsage 记录模型调用（含 token 消耗）
func RecordModelUsage(tenantID, userID, provider, model, endpoint string, inputTokens, outputTokens, latencyMs int) {
	RecordModelUsageWithContext(tenantID, userID, provider, model, endpoint, inputTokens, outputTokens, 0, latencyMs,
		"unknown", "", "", "", "")
}

// RecordModelUsageWithContext 记录模型调用（含来源、资源和链路ID）
func RecordModelUsageWithContext(tenantID, userID, provider, model, endpoint string, inputTokens, outputTokens, cachedTokens, latencyMs int, source, sourceName, resourceType, resourceID, requestID string) {
	if source == "" {
		source = "unknown"
	}
	go func() {
		_, err := database.DB.Exec(`
			INSERT INTO model_usage (tenant_id, user_id, model_provider, model_name, input_tokens, output_tokens, cached_tokens, total_tokens, latency_ms, endpoint, source, source_name, resource_type, resource_id, request_id, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())`,
			tenantID, userID, provider, model, inputTokens, outputTokens, cachedTokens, inputTokens+outputTokens, latencyMs, endpoint, source, sourceName, resourceType, resourceID, requestID)
		if err != nil {
			log.Printf("RecordModelUsageWithContext failed: %v", err)
		}
	}()
}

// RecordToolCallUsage 记录 MCP工具 / Skill / 内置工具调用次数和耗时
func RecordToolCallUsage(tenantID, userID, resourceType, resourceID, resourceName, source, status string, latencyMs int, requestID string, callErr error) {
	if status == "" {
		status = "success"
	}
	var errorMessage any
	if callErr != nil {
		if status == "success" {
			status = "failed"
		}
		errorMessage = callErr.Error()
	}
	go func() {
		_, err := database.DB.Exec(`
			INSERT INTO tool_call_usage (tenant_id, user_id, resource_type, resource_id, resource_name, source, status, latency_ms, request_id, error_message, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())`,
			tenantID, userID, resourceType, resourceID, resourceName, source, status, latencyMs, requestID, errorMessage)
		if err != nil {
			log.Printf("RecordToolCallUsage failed: %v", err)
		}
	}()
}

// CheckQuota 检查配额是否超限
// 返回 nil 表示允许，返回 error 表示超限
func CheckQuota(tenantID string) *quotaError {
	var config struct {
		DailyQuota        int `db:"daily_quota"`
		MonthlyQuota      int `db:"monthly_quota"`
		DailyTokenQuota   int `db:"daily_token_quota"`
		MonthlyTokenQuota int `db:"monthly_token_quota"`
	}
	err := database.DB.Get(&config, `
		SELECT COALESCE(daily_quota, 0) as daily_quota,
		       COALESCE(monthly_quota, 0) as monthly_quota,
		       COALESCE(daily_token_quota, 0) as daily_token_quota,
		       COALESCE(monthly_token_quota, 0) as monthly_token_quota
		FROM tenants WHERE id = ?`, tenantID)
	if err != nil {
		return nil // 获取失败不阻塞
	}

	today := time.Now().Format("2006-01-02")
	monthStart := time.Now().Format("2006-01") + "-01"

	// 检查日调用次数
	if config.DailyQuota > 0 {
		var count int
		database.DB.Get(&count, "SELECT COUNT(*) FROM api_usage WHERE tenant_id = ? AND DATE(created_at) = ?", tenantID, today)
		if count >= config.DailyQuota {
			return &quotaError{Code: "daily_quota_exceeded", Message: "今日API调用次数已达上限"}
		}
	}

	// 检查月调用次数
	if config.MonthlyQuota > 0 {
		var count int
		database.DB.Get(&count, "SELECT COUNT(*) FROM api_usage WHERE tenant_id = ? AND created_at >= ?", tenantID, monthStart)
		if count >= config.MonthlyQuota {
			return &quotaError{Code: "monthly_quota_exceeded", Message: "本月API调用次数已达上限"}
		}
	}

	// 检查日 token 配额
	if config.DailyTokenQuota > 0 {
		var total int
		database.DB.Get(&total, "SELECT COALESCE(SUM(total_tokens), 0) FROM model_usage WHERE tenant_id = ? AND DATE(created_at) = ?", tenantID, today)
		if total >= config.DailyTokenQuota {
			return &quotaError{Code: "daily_token_quota_exceeded", Message: "今日Token消耗已达上限"}
		}
	}

	// 检查月 token 配额
	if config.MonthlyTokenQuota > 0 {
		var total int
		database.DB.Get(&total, "SELECT COALESCE(SUM(total_tokens), 0) FROM model_usage WHERE tenant_id = ? AND created_at >= ?", tenantID, monthStart)
		if total >= config.MonthlyTokenQuota {
			return &quotaError{Code: "monthly_token_quota_exceeded", Message: "本月Token消耗已达上限"}
		}
	}

	return nil
}

type quotaError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// QuotaMiddleware 配额检查中间件
func QuotaMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, exists := c.Get(middleware.ContextTenantID)
		if !exists {
			c.Next()
			return
		}

		tid := tenantID.(string)
		if qe := CheckQuota(tid); qe != nil {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   qe.Code,
				"message": qe.Message,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
