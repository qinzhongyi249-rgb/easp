package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/easp-platform/easp/internal/database"
	"github.com/gin-gonic/gin"
)

type UsageAnalyticsResponse struct {
	Summary  UsageAnalyticsSummary `json:"summary"`
	Trend    []TokenTrendPoint     `json:"trend"`
	ByModel  []UsageGroupStats     `json:"by_model"`
	BySource []UsageGroupStats     `json:"by_source"`
	ByTool   []ToolUsageStats      `json:"by_tool"`
	Details  []UsageDetail         `json:"details"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
	Total    int                   `json:"total"`
}

type UsageAnalyticsSummary struct {
	InputTokens      int `json:"input_tokens" db:"input_tokens"`
	OutputTokens     int `json:"output_tokens" db:"output_tokens"`
	CachedTokens     int `json:"cached_tokens" db:"cached_tokens"`
	TotalTokens      int `json:"total_tokens" db:"total_tokens"`
	ModelCalls       int `json:"model_calls" db:"model_calls"`
	ToolCalls        int `json:"tool_calls" db:"tool_calls"`
	MCPToolCalls     int `json:"mcp_tool_calls" db:"mcp_tool_calls"`
	SkillCalls       int `json:"skill_calls" db:"skill_calls"`
	BuiltinToolCalls int `json:"builtin_tool_calls" db:"builtin_tool_calls"`
	AvgLatencyMs     int `json:"avg_latency_ms" db:"avg_latency_ms"`
}

type TokenTrendPoint struct {
	Period       string `json:"period" db:"period"`
	InputTokens  int    `json:"input_tokens" db:"input_tokens"`
	OutputTokens int    `json:"output_tokens" db:"output_tokens"`
	CachedTokens int    `json:"cached_tokens" db:"cached_tokens"`
	TotalTokens  int    `json:"total_tokens" db:"total_tokens"`
	Calls        int    `json:"calls" db:"calls"`
}

type UsageGroupStats struct {
	Name         string `json:"name" db:"name"`
	Provider     string `json:"provider,omitempty" db:"provider"`
	Model        string `json:"model,omitempty" db:"model"`
	InputTokens  int    `json:"input_tokens" db:"input_tokens"`
	OutputTokens int    `json:"output_tokens" db:"output_tokens"`
	CachedTokens int    `json:"cached_tokens" db:"cached_tokens"`
	TotalTokens  int    `json:"total_tokens" db:"total_tokens"`
	Calls        int    `json:"calls" db:"calls"`
	AvgLatencyMs int    `json:"avg_latency_ms" db:"avg_latency_ms"`
}

type ToolUsageStats struct {
	ResourceType string `json:"resource_type" db:"resource_type"`
	ResourceID   string `json:"resource_id" db:"resource_id"`
	ResourceName string `json:"resource_name" db:"resource_name"`
	Source       string `json:"source" db:"source"`
	Calls        int    `json:"calls" db:"calls"`
	SuccessCalls int    `json:"success_calls" db:"success_calls"`
	FailedCalls  int    `json:"failed_calls" db:"failed_calls"`
	AvgLatencyMs int    `json:"avg_latency_ms" db:"avg_latency_ms"`
}

type UsageDetail struct {
	Kind         string    `json:"kind" db:"kind"`
	ID           int64     `json:"id" db:"id"`
	UserID       string    `json:"user_id" db:"user_id"`
	Source       string    `json:"source" db:"source"`
	SourceName   string    `json:"source_name" db:"source_name"`
	Provider     string    `json:"provider" db:"provider"`
	Model        string    `json:"model" db:"model"`
	ResourceType string    `json:"resource_type" db:"resource_type"`
	ResourceID   string    `json:"resource_id" db:"resource_id"`
	ResourceName string    `json:"resource_name" db:"resource_name"`
	InputTokens  int       `json:"input_tokens" db:"input_tokens"`
	OutputTokens int       `json:"output_tokens" db:"output_tokens"`
	CachedTokens int       `json:"cached_tokens" db:"cached_tokens"`
	TotalTokens  int       `json:"total_tokens" db:"total_tokens"`
	LatencyMs    int       `json:"latency_ms" db:"latency_ms"`
	Status       string    `json:"status" db:"status"`
	RequestID    string    `json:"request_id" db:"request_id"`
	ErrorMessage string    `json:"error_message" db:"error_message"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

type usageAnalyticsFilters struct {
	StartDate    string
	EndDate      string
	Granularity  string
	Source       string
	ModelName    string
	ResourceType string
	Page         int
	PageSize     int
}

func (h *UsageHandler) GetUsageAnalytics(c *gin.Context) {
	tenantID := c.Param("tenantId")
	filters := parseUsageAnalyticsFilters(c)

	modelWhere, modelArgs := buildModelUsageWhere(tenantID, filters)
	toolWhere, toolArgs := buildToolUsageWhere(tenantID, filters)

	resp := UsageAnalyticsResponse{Page: filters.Page, PageSize: filters.PageSize}

	_ = database.DB.Get(&resp.Summary, fmt.Sprintf(`
		SELECT COALESCE(SUM(input_tokens),0) input_tokens,
		       COALESCE(SUM(output_tokens),0) output_tokens,
		       COALESCE(SUM(cached_tokens),0) cached_tokens,
		       COALESCE(SUM(total_tokens),0) total_tokens,
		       COUNT(*) model_calls,
		       COALESCE(ROUND(AVG(latency_ms)),0) avg_latency_ms
		FROM model_usage %s`, modelWhere), modelArgs...)

	var toolSummary struct {
		ToolCalls        int `db:"tool_calls"`
		MCPToolCalls     int `db:"mcp_tool_calls"`
		SkillCalls       int `db:"skill_calls"`
		BuiltinToolCalls int `db:"builtin_tool_calls"`
	}
	_ = database.DB.Get(&toolSummary, fmt.Sprintf(`
		SELECT COUNT(*) tool_calls,
		       SUM(CASE WHEN resource_type='mcp_tool' THEN 1 ELSE 0 END) mcp_tool_calls,
		       SUM(CASE WHEN resource_type='skill' THEN 1 ELSE 0 END) skill_calls,
		       SUM(CASE WHEN resource_type='builtin_tool' THEN 1 ELSE 0 END) builtin_tool_calls
		FROM tool_call_usage %s`, toolWhere), toolArgs...)
	resp.Summary.ToolCalls = toolSummary.ToolCalls
	resp.Summary.MCPToolCalls = toolSummary.MCPToolCalls
	resp.Summary.SkillCalls = toolSummary.SkillCalls
	resp.Summary.BuiltinToolCalls = toolSummary.BuiltinToolCalls

	periodExpr := usagePeriodExpr(filters.Granularity)
	_ = database.DB.Select(&resp.Trend, fmt.Sprintf(`
		SELECT %s period,
		       COALESCE(SUM(input_tokens),0) input_tokens,
		       COALESCE(SUM(output_tokens),0) output_tokens,
		       COALESCE(SUM(cached_tokens),0) cached_tokens,
		       COALESCE(SUM(total_tokens),0) total_tokens,
		       COUNT(*) calls
		FROM model_usage %s
		GROUP BY period ORDER BY period ASC`, periodExpr, modelWhere), modelArgs...)

	_ = database.DB.Select(&resp.ByModel, fmt.Sprintf(`
		SELECT CONCAT(model_provider, '/', model_name) name,
		       model_provider provider,
		       model_name model,
		       COALESCE(SUM(input_tokens),0) input_tokens,
		       COALESCE(SUM(output_tokens),0) output_tokens,
		       COALESCE(SUM(cached_tokens),0) cached_tokens,
		       COALESCE(SUM(total_tokens),0) total_tokens,
		       COUNT(*) calls,
		       COALESCE(ROUND(AVG(latency_ms)),0) avg_latency_ms
		FROM model_usage %s
		GROUP BY model_provider, model_name ORDER BY total_tokens DESC LIMIT 20`, modelWhere), modelArgs...)

	_ = database.DB.Select(&resp.BySource, fmt.Sprintf(`
		SELECT %s name,
		       '' provider, '' model,
		       COALESCE(SUM(input_tokens),0) input_tokens,
		       COALESCE(SUM(output_tokens),0) output_tokens,
		       COALESCE(SUM(cached_tokens),0) cached_tokens,
		       COALESCE(SUM(total_tokens),0) total_tokens,
		       COUNT(*) calls,
		       COALESCE(ROUND(AVG(latency_ms)),0) avg_latency_ms
		FROM model_usage %s
		GROUP BY name ORDER BY total_tokens DESC`, normalizedModelSourceExpr(), modelWhere), modelArgs...)

	_ = database.DB.Select(&resp.ByTool, fmt.Sprintf(`
		SELECT resource_type, resource_id, resource_name, source,
		       COUNT(*) calls,
		       SUM(CASE WHEN status='success' THEN 1 ELSE 0 END) success_calls,
		       SUM(CASE WHEN status<>'success' THEN 1 ELSE 0 END) failed_calls,
		       COALESCE(ROUND(AVG(latency_ms)),0) avg_latency_ms
		FROM tool_call_usage %s
		GROUP BY resource_type, resource_id, resource_name, source
		ORDER BY calls DESC LIMIT 30`, toolWhere), toolArgs...)

	resp.Total = countUsageDetails(modelWhere, modelArgs, toolWhere, toolArgs)
	detailsArgs := append([]any{}, modelArgs...)
	detailsArgs = append(detailsArgs, toolArgs...)
	detailsArgs = append(detailsArgs, (filters.Page-1)*filters.PageSize, filters.PageSize)
	_ = database.DB.Select(&resp.Details, fmt.Sprintf(`
		SELECT * FROM (
			SELECT 'model' kind, id, user_id, %s source, %s source_name,
			       model_provider provider, model_name model,
			       resource_type, resource_id, '' resource_name,
			       input_tokens, output_tokens, cached_tokens, total_tokens, latency_ms,
			       'success' status, request_id, '' error_message, created_at
			FROM model_usage %s
			UNION ALL
			SELECT 'tool' kind, id, user_id, source, '' source_name,
			       '' provider, '' model,
			       resource_type, resource_id, resource_name,
			       0 input_tokens, 0 output_tokens, 0 cached_tokens, 0 total_tokens, latency_ms,
			       status, request_id, COALESCE(error_message,'') error_message, created_at
			FROM tool_call_usage %s
		) u
		ORDER BY created_at DESC LIMIT ?, ?`, normalizedModelSourceExpr(), normalizedModelSourceNameExpr(), modelWhere, toolWhere), detailsArgs...)

	normalizeUsageAnalyticsResponse(&resp)
	c.JSON(http.StatusOK, resp)
}

func (h *UsageHandler) GetUsageSummary(c *gin.Context) {
	tenantID := c.Param("tenantId")
	today := time.Now().Format("2006-01-02")
	monthStart := time.Now().Format("2006-01") + "-01"
	var resp struct {
		TodayTokens     int `json:"today_tokens" db:"today_tokens"`
		MonthTokens     int `json:"month_tokens" db:"month_tokens"`
		TodayInput      int `json:"today_input_tokens" db:"today_input_tokens"`
		TodayOutput     int `json:"today_output_tokens" db:"today_output_tokens"`
		TodayCached     int `json:"today_cached_tokens" db:"today_cached_tokens"`
		TodayModelCalls int `json:"today_model_calls" db:"today_model_calls"`
		TodayToolCalls  int `json:"today_tool_calls" db:"today_tool_calls"`
		TodaySkillCalls int `json:"today_skill_calls" db:"today_skill_calls"`
	}
	_ = database.DB.Get(&resp, `
		SELECT COALESCE(SUM(CASE WHEN DATE(created_at)=? THEN total_tokens ELSE 0 END),0) today_tokens,
		       COALESCE(SUM(CASE WHEN created_at>=? THEN total_tokens ELSE 0 END),0) month_tokens,
		       COALESCE(SUM(CASE WHEN DATE(created_at)=? THEN input_tokens ELSE 0 END),0) today_input_tokens,
		       COALESCE(SUM(CASE WHEN DATE(created_at)=? THEN output_tokens ELSE 0 END),0) today_output_tokens,
		       COALESCE(SUM(CASE WHEN DATE(created_at)=? THEN cached_tokens ELSE 0 END),0) today_cached_tokens,
		       SUM(CASE WHEN DATE(created_at)=? THEN 1 ELSE 0 END) today_model_calls
		FROM model_usage WHERE tenant_id=?`, today, monthStart, today, today, today, today, tenantID)
	var calls struct {
		ToolCalls  int `db:"tool_calls"`
		SkillCalls int `db:"skill_calls"`
	}
	_ = database.DB.Get(&calls, `
		SELECT COUNT(*) tool_calls,
		       SUM(CASE WHEN resource_type='skill' THEN 1 ELSE 0 END) skill_calls
		FROM tool_call_usage WHERE tenant_id=? AND DATE(created_at)=?`, tenantID, today)
	resp.TodayToolCalls = calls.ToolCalls
	resp.TodaySkillCalls = calls.SkillCalls
	c.JSON(http.StatusOK, resp)
}

func parseUsageAnalyticsFilters(c *gin.Context) usageAnalyticsFilters {
	now := time.Now()
	start := c.DefaultQuery("start_date", now.AddDate(0, 0, -29).Format("2006-01-02"))
	end := c.DefaultQuery("end_date", now.Format("2006-01-02"))
	granularity := c.DefaultQuery("granularity", "day")
	if granularity != "month" && granularity != "year" {
		granularity = "day"
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return usageAnalyticsFilters{
		StartDate:    start,
		EndDate:      end,
		Granularity:  granularity,
		Source:       strings.TrimSpace(c.Query("source")),
		ModelName:    strings.TrimSpace(c.Query("model_name")),
		ResourceType: strings.TrimSpace(c.Query("resource_type")),
		Page:         page,
		PageSize:     pageSize,
	}
}

func buildModelUsageWhere(tenantID string, f usageAnalyticsFilters) (string, []any) {
	clauses := []string{"tenant_id = ?", "created_at >= ?", "created_at < DATE_ADD(?, INTERVAL 1 DAY)"}
	args := []any{tenantID, f.StartDate, f.EndDate}
	if f.Source != "" {
		clauses = append(clauses, normalizedModelSourceExpr()+" = ?")
		args = append(args, f.Source)
	}
	if f.ModelName != "" {
		clauses = append(clauses, "model_name = ?")
		args = append(args, f.ModelName)
	}
	if f.ResourceType != "" {
		clauses = append(clauses, "resource_type = ?")
		args = append(args, f.ResourceType)
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func buildToolUsageWhere(tenantID string, f usageAnalyticsFilters) (string, []any) {
	clauses := []string{"tenant_id = ?", "created_at >= ?", "created_at < DATE_ADD(?, INTERVAL 1 DAY)"}
	args := []any{tenantID, f.StartDate, f.EndDate}
	if f.Source != "" {
		clauses = append(clauses, "source = ?")
		args = append(args, f.Source)
	}
	if f.ResourceType != "" {
		clauses = append(clauses, "resource_type = ?")
		args = append(args, f.ResourceType)
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func normalizedModelSourceExpr() string {
	return "CASE WHEN source IS NULL OR source='' OR source='unknown' THEN 'ai_assistant' ELSE source END"
}

func normalizedModelSourceNameExpr() string {
	return "CASE WHEN source_name IS NULL OR source_name='' THEN CASE WHEN source IS NULL OR source='' OR source='unknown' THEN 'AI助手' ELSE source END ELSE source_name END"
}

func usagePeriodExpr(granularity string) string {
	switch granularity {
	case "month":
		return "DATE_FORMAT(created_at, '%Y-%m')"
	case "year":
		return "DATE_FORMAT(created_at, '%Y')"
	default:
		return "DATE(created_at)"
	}
}

func countUsageDetails(modelWhere string, modelArgs []any, toolWhere string, toolArgs []any) int {
	var modelCount, toolCount int
	_ = database.DB.Get(&modelCount, fmt.Sprintf("SELECT COUNT(*) FROM model_usage %s", modelWhere), modelArgs...)
	_ = database.DB.Get(&toolCount, fmt.Sprintf("SELECT COUNT(*) FROM tool_call_usage %s", toolWhere), toolArgs...)
	return modelCount + toolCount
}

func normalizeUsageAnalyticsResponse(resp *UsageAnalyticsResponse) {
	if resp.Trend == nil {
		resp.Trend = []TokenTrendPoint{}
	}
	if resp.ByModel == nil {
		resp.ByModel = []UsageGroupStats{}
	}
	if resp.BySource == nil {
		resp.BySource = []UsageGroupStats{}
	}
	if resp.ByTool == nil {
		resp.ByTool = []ToolUsageStats{}
	}
	if resp.Details == nil {
		resp.Details = []UsageDetail{}
	}
}
