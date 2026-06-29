package handlers

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/logger"
	easpMCP "github.com/easp-platform/easp/internal/mcp"
	easpMemory "github.com/easp-platform/easp/internal/memory"
	"github.com/easp-platform/easp/internal/middleware"
	"github.com/easp-platform/easp/internal/models"
	"github.com/easp-platform/easp/internal/modelservice"
	"github.com/easp-platform/easp/internal/repositories"
	skillPkg "github.com/easp-platform/easp/internal/skill"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// sanitizeToolName 将工具名称转换为符合 Gemini API 规范的格式
// 规则：1) 必须以字母或下划线开头 2) 只能包含字母数字._:- 3) 最大 128 字符
// 中文/特殊字符会被替换为下划线，连续的下划线会被合并
var toolNameRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_.:-]{0,127}$`)
var skillIDPrefixRegex = regexp.MustCompile(`^[a-f0-9]{8}$`)

func makeSkillToolName(sk models.Skill) string {
	idPrefix := strings.ReplaceAll(sk.ID, "-", "")
	if len(idPrefix) > 8 {
		idPrefix = idPrefix[:8]
	}
	safeName := sanitizeToolName(sk.Name)
	toolName := "skill_" + idPrefix + "_" + safeName
	if len(toolName) > 128 {
		toolName = toolName[:128]
	}
	if !toolNameRegex.MatchString(toolName) {
		toolName = "skill_" + idPrefix
	}
	return toolName
}

func sanitizeToolName(name string) string {
	// 如果已经符合规范，直接返回
	if toolNameRegex.MatchString(name) {
		return name
	}

	// 1. 将所有非字母数字字符替换为下划线
	sanitized := ""
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			sanitized += string(r)
		} else {
			sanitized += "_"
		}
	}

	// 2. 合并连续的下划线
	for strings.Contains(sanitized, "__") {
		sanitized = strings.ReplaceAll(sanitized, "__", "_")
	}

	// 3. 确保以字母或下划线开头
	if len(sanitized) > 0 {
		first := sanitized[0]
		if first >= '0' && first <= '9' {
			sanitized = "_" + sanitized
		} else if first != '_' && (first < 'a' || first > 'z') && (first < 'A' || first > 'Z') {
			sanitized = "_" + sanitized
		}
	}

	// 4. 截取最大 128 字符
	if len(sanitized) > 128 {
		sanitized = sanitized[:128]
	}

	// 5. 如果为空，返回默认值
	if sanitized == "" {
		sanitized = "unknown_tool"
	}

	return sanitized
}

func executionModeFromArgs(args map[string]any) string {
	if args == nil {
		return skillPkg.ExecutionModeProduction
	}
	if mode, ok := args["execution_mode"].(string); ok {
		return skillPkg.NormalizeExecutionMode(mode)
	}
	return skillPkg.ExecutionModeProduction
}

// ChatHandler AI 助手处理器
type ChatHandler struct {
	modelService    *modelservice.ModelService
	memoryRouter    *easpMemory.MemoryRouter
	memoryExtractor *easpMemory.MemoryExtractor
}

func NewChatHandler() *ChatHandler {
	embeddingSvc := &MockEmbeddingService{}
	memorySvc := easpMemory.NewMemoryService(easpMemory.MemoryConfig{
		EmbeddingService: embeddingSvc,
	})

	return &ChatHandler{
		modelService: modelservice.NewModelService(modelservice.Config{}),
		memoryRouter: easpMemory.NewMemoryRouter(memorySvc, easpMemory.DefaultRouterConfig()),
		memoryExtractor: easpMemory.NewMemoryExtractor(memorySvc, easpMemory.ModelConfig{
			BaseURL: "", // 会在运行时从model配置获取
			APIKey:  "",
			Model:   "",
		}),
	}
}

// AssistantMessage 助手消息
type AssistantMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AssistantPageContext 前端传入的事实型页面上下文。
// 前端只负责传递当前页面/对象事实，业务理解和决策由后端提示词与模型完成。
type AssistantPageContext map[string]any

// AssistantRequest 助手请求
type AssistantRequest struct {
	Messages       []AssistantMessage   `json:"messages" binding:"required"`
	ConversationID string               `json:"conversation_id,omitempty"`
	PageContext    AssistantPageContext `json:"page_context,omitempty"`
	ExecutionMode  string               `json:"execution_mode,omitempty"` // sandbox | normal
}

// ToolDefinition 工具定义
type ToolDefinition struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

type FunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ToolCall 工具调用
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// SSE事件类型
const (
	SSEEventStatus       = "status"       // 状态更新
	SSEEventHeartbeat    = "heartbeat"    // 长任务心跳
	SSEEventTool         = "tool"         // 工具执行结果
	SSEEventDelta        = "delta"        // 流式文本片段
	SSEEventDone         = "done"         // 完成
	SSEEventError        = "error"        // 错误
	SSEEventModelInfo    = "model_info"   // 模型信息
	SSEEventConversation = "conversation" // 会话信息
)

// sendSSE 发送SSE事件
func sendSSE(c *gin.Context, event string, data any) {
	jsonData, _ := json.Marshal(data)
	fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, string(jsonData))
	c.Writer.Flush()
}

func sendSSEActive(c *gin.Context, activity *assistantActivityTracker, event string, data any) {
	if activity != nil {
		activity.touch()
	}
	sendSSE(c, event, data)
}

type modelCallResult struct {
	response *ModelResponse
	err      error
}

type toolExecResult struct {
	result string
}

type assistantActivityTracker struct {
	idleWindow time.Duration
	lastActive time.Time
}

type assistantDeltaBuffer struct {
	minRunes      int
	maxRunes      int
	flushInterval time.Duration
	pending       strings.Builder
	lastFlushAt   time.Time
}

func newAssistantDeltaBuffer(minRunes, maxRunes int, flushInterval time.Duration) *assistantDeltaBuffer {
	if minRunes <= 0 {
		minRunes = 8
	}
	if maxRunes < minRunes {
		maxRunes = minRunes * 4
	}
	if flushInterval <= 0 {
		flushInterval = 80 * time.Millisecond
	}
	return &assistantDeltaBuffer{minRunes: minRunes, maxRunes: maxRunes, flushInterval: flushInterval}
}

func (b *assistantDeltaBuffer) push(piece string, now time.Time, force bool) []string {
	if piece != "" {
		b.pending.WriteString(piece)
		if b.lastFlushAt.IsZero() {
			b.lastFlushAt = now
		}
	}
	pending := b.pending.String()
	if pending == "" {
		return nil
	}

	pendingRunes := []rune(pending)
	timedFlush := !b.lastFlushAt.IsZero() && now.Sub(b.lastFlushAt) >= b.flushInterval
	shouldFlush := force || len(pendingRunes) >= b.minRunes || timedFlush
	if !shouldFlush {
		return nil
	}

	chunks := make([]string, 0)
	allowSmallFlush := force || timedFlush
	for len(pendingRunes) > 0 {
		if !allowSmallFlush && len(pendingRunes) < b.minRunes {
			break
		}
		take := len(pendingRunes)
		if take > b.maxRunes {
			take = b.maxRunes
		}
		chunks = append(chunks, string(pendingRunes[:take]))
		pendingRunes = pendingRunes[take:]
		if !allowSmallFlush && len(pendingRunes) < b.minRunes {
			break
		}
	}

	b.pending.Reset()
	if len(pendingRunes) > 0 {
		b.pending.WriteString(string(pendingRunes))
	}
	if len(chunks) > 0 {
		b.lastFlushAt = now
	}
	return chunks
}

func sendAssistantBufferedDelta(c *gin.Context, activity *assistantActivityTracker, content string) {
	buffer := newAssistantDeltaBuffer(8, 32, 80*time.Millisecond)
	for _, chunk := range buffer.push(content, time.Now(), true) {
		if chunk == "" {
			continue
		}
		sendSSEActive(c, activity, SSEEventDelta, map[string]string{"content": chunk})
	}
}

func newAssistantActivityTracker(idleWindow time.Duration) *assistantActivityTracker {
	if idleWindow <= 0 {
		idleWindow = 90 * time.Second
	}
	return &assistantActivityTracker{idleWindow: idleWindow, lastActive: time.Now()}
}

func (t *assistantActivityTracker) touch() {
	t.touchAt(time.Now())
}

func (t *assistantActivityTracker) touchAt(now time.Time) {
	t.lastActive = now
}

func (t *assistantActivityTracker) idleTimedOut() bool {
	return t.idleTimedOutAt(time.Now())
}

func (t *assistantActivityTracker) idleTimedOutAt(now time.Time) bool {
	return !t.lastActive.IsZero() && now.Sub(t.lastActive) > t.idleWindow
}

// waitModelWithHeartbeat 在模型等待期间持续向前端发送 heartbeat，避免代理/浏览器长时间无数据断开。
// 注意：模型调用在 goroutine 中执行，SSE 写入只在当前 goroutine 中发生，避免并发写 ResponseWriter。
func waitModelWithHeartbeat(c *gin.Context, requestStart time.Time, activity *assistantActivityTracker, stage, message string, call func() (*ModelResponse, error)) (*ModelResponse, error) {
	resultCh := make(chan modelCallResult, 1)
	go func() {
		resp, err := call()
		resultCh <- modelCallResult{response: resp, err: err}
	}()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return nil, c.Request.Context().Err()
		case res := <-resultCh:
			if activity != nil {
				activity.touch()
			}
			return res.response, res.err
		case <-ticker.C:
			if activity != nil && activity.idleTimedOut() {
				return nil, fmt.Errorf("模型调用长时间无进展，请稍后重试")
			}
			sendSSEActive(c, activity, SSEEventHeartbeat, map[string]any{
				"message":    message,
				"stage":      stage,
				"elapsed_ms": time.Since(requestStart).Milliseconds(),
				"total_ms":   time.Since(requestStart).Milliseconds(),
			})
		}
	}
}

// waitToolWithHeartbeat 在工具执行等待期间持续发送 heartbeat。
func waitToolWithHeartbeat(c *gin.Context, requestStart time.Time, activity *assistantActivityTracker, stage, message string, call func() string) (string, error) {
	resultCh := make(chan toolExecResult, 1)
	go func() {
		resultCh <- toolExecResult{result: call()}
	}()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return "", c.Request.Context().Err()
		case res := <-resultCh:
			if activity != nil {
				activity.touch()
			}
			return res.result, nil
		case <-ticker.C:
			if activity != nil && activity.idleTimedOut() {
				return "", fmt.Errorf("工具调用长时间无进展，请稍后重试")
			}
			sendSSEActive(c, activity, SSEEventHeartbeat, map[string]any{
				"message":    message,
				"stage":      stage,
				"elapsed_ms": time.Since(requestStart).Milliseconds(),
				"total_ms":   time.Since(requestStart).Milliseconds(),
			})
		}
	}
}

// getSystemPrompt 获取系统提示
func getSystemPrompt(tenantID string, toolNames []string, unavailableCapabilities ...[]string) string {
	toolList := "无"
	if len(toolNames) > 0 {
		visible := toolNames
		if len(visible) > 30 {
			visible = visible[:30]
		}
		toolList = strings.Join(visible, "、")
		if len(toolNames) > len(visible) {
			toolList += fmt.Sprintf(" 等%d个", len(toolNames))
		}
	}
	unavailableSection := ""
	if len(unavailableCapabilities) > 0 && len(unavailableCapabilities[0]) > 0 {
		visible := unavailableCapabilities[0]
		if len(visible) > 20 {
			visible = visible[:20]
		}
		unavailableSection = "\n\n## 不可用能力\n以下能力当前用户没有权限，不能调用，也不要追问这些能力的业务必填项；如果用户请求不可用能力，直接提示缺少权限：\n- " + strings.Join(visible, "\n- ")
		if len(unavailableCapabilities[0]) > len(visible) {
			unavailableSection += fmt.Sprintf("\n- 其余 %d 项不可用能力已省略", len(unavailableCapabilities[0])-len(visible))
		}
	}
	return `你是 EASP 企业智能服务平台助手。
租户: ` + tenantID + `
可用工具: ` + toolList + `
规则:
- 需要操作/查询时优先调用工具，不猜测。
- 配置变更前说明关键影响；高危操作先确认。
- 输出尽量精简：先结论，少铺垫；查询结果只列关键字段。
- 工具返回的数据优先于记忆和页面上下文。
- 创建用户/角色/MCP 等多步骤任务，优先使用可用 Skill；Skill 返回 requires_input 时只追问缺失字段，不继续调用工具。
- 创建用户只需要 email 和 display_name，role_name 可选；不要询问、收集或输出初始密码，后续通过重置密码或 SSO 完成登录。
- 无权限或无工具时直接说明。` + unavailableSection
}

func buildPageContextPrompt(pageContext AssistantPageContext) string {
	if len(pageContext) == 0 {
		return ""
	}
	ctxBytes, err := json.MarshalIndent(pageContext, "", "  ")
	if err != nil || len(ctxBytes) == 0 {
		return ""
	}
	if len(ctxBytes) > 4096 {
		ctxBytes = ctxBytes[:4096]
	}
	return "\n\n## 当前页面上下文（前端传入的事实，不是业务结论）\n" + string(ctxBytes) + "\n请结合这些页面事实理解用户指代，例如‘当前页面’、‘这个对象’、‘刚才看到的配置’。如需执行操作，仍必须依据后端工具返回的数据做最终判断。"
}

func lastUserMessage(messages []AssistantMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return strings.TrimSpace(messages[i].Content)
		}
	}
	return ""
}

func titleFromMessage(content string) string {
	content = strings.TrimSpace(strings.ReplaceAll(content, "\n", " "))
	if content == "" {
		return "新对话"
	}
	runes := []rune(content)
	if len(runes) > 40 {
		return string(runes[:40]) + "..."
	}
	return content
}

func ensureAssistantConversation(tenantID, userID string, req *AssistantRequest) string {
	conversationID := strings.TrimSpace(req.ConversationID)
	pageContextJSON := "null"
	if len(req.PageContext) > 0 {
		if b, err := json.Marshal(req.PageContext); err == nil && len(b) > 0 {
			if len(b) > 8192 {
				b = b[:8192]
			}
			pageContextJSON = string(b)
		}
	}

	if conversationID != "" {
		var count int
		if err := database.DB.Get(&count,
			"SELECT COUNT(*) FROM assistant_conversations WHERE id = ? AND tenant_id = ? AND user_id = ?",
			conversationID, tenantID, userID); err == nil && count > 0 {
			database.DB.Exec("UPDATE assistant_conversations SET page_context = ?, updated_at = NOW() WHERE id = ?", pageContextJSON, conversationID)
			return conversationID
		}
	}

	conversationID = uuid.New().String()
	title := titleFromMessage(lastUserMessage(req.Messages))
	if _, err := database.DB.Exec(`
		INSERT INTO assistant_conversations (id, tenant_id, user_id, title, page_context, message_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 0, NOW(), NOW())`, conversationID, tenantID, userID, title, pageContextJSON); err != nil {
		logger.Warn("chat", "create assistant conversation failed",
			logger.Field("tenant_id", tenantID),
			logger.Field("user_id", userID),
			logger.Field("error", err.Error()),
		)
	}
	return conversationID
}

func loadConversationMessages(tenantID, userID, conversationID string, limit int) []AssistantMessage {
	if conversationID == "" || limit <= 0 {
		return nil
	}
	var rows []models.SessionMemory
	err := database.DB.Select(&rows, `
		SELECT id, tenant_id, user_id, session_id, role, content, token_count, entity_ids, created_at
		FROM session_memories
		WHERE tenant_id = ? AND user_id = ? AND session_id = ? AND role IN ('user','assistant')
		ORDER BY created_at DESC
		LIMIT ?`, tenantID, userID, conversationID, limit)
	if err != nil {
		logger.Warn("chat", "load conversation history failed",
			logger.Field("tenant_id", tenantID),
			logger.Field("user_id", userID),
			logger.Field("conversation_id", conversationID),
			logger.Field("error", err.Error()),
		)
		return nil
	}

	messages := make([]AssistantMessage, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		messages = append(messages, AssistantMessage{Role: rows[i].Role, Content: rows[i].Content})
	}
	return messages
}

func saveConversationMessage(tenantID, userID, conversationID, role, content string) {
	content = strings.TrimSpace(content)
	if tenantID == "" || userID == "" || conversationID == "" || content == "" {
		return
	}
	if len([]rune(content)) > 8000 {
		content = string([]rune(content)[:8000])
	}
	_, err := database.DB.Exec(`
		INSERT INTO session_memories (id, tenant_id, user_id, session_id, role, content, created_at)
		VALUES (?, ?, ?, ?, ?, ?, NOW())`, uuid.New().String(), tenantID, userID, conversationID, role, content)
	if err != nil {
		logger.Warn("chat", "save conversation message failed",
			logger.Field("tenant_id", tenantID),
			logger.Field("user_id", userID),
			logger.Field("conversation_id", conversationID),
			logger.Field("role", role),
			logger.Field("error", err.Error()),
		)
		return
	}
	database.DB.Exec("UPDATE assistant_conversations SET message_count = message_count + 1, updated_at = NOW() WHERE id = ?", conversationID)
}

type auditContext struct {
	AgentID        string
	IP             string
	UserAgent      string
	StartedAt      time.Time
	SourceType     string
	SourceAppID    string
	ExternalSystem string
	ExternalUserID string
	UserUID        string
	RequestContext context.Context
}

// logAudit 记录审计日志（AI助手写操作）
func logAudit(ctx *auditContext, tenantID, userID, toolName, action, resource, detail string) {
	auditRepo := repositories.NewAuditLogRepository()
	auditLog := &models.AuditLog{
		TenantID: tenantID,
		Tool:     toolName,
		Action:   action,
	}
	if userID != "" {
		auditLog.UserID = &userID
		var userUID string
		if ctx != nil && ctx.UserUID != "" {
			userUID = ctx.UserUID
		} else {
			_ = database.DB.Get(&userUID, "SELECT COALESCE(user_uid, '') FROM users WHERE id = ?", userID)
		}
		if userUID != "" {
			auditLog.UserUID = &userUID
		}
	}
	if ctx != nil {
		if ctx.AgentID != "" {
			auditLog.AgentID = &ctx.AgentID
		}
		if ctx.SourceType != "" {
			auditLog.SourceType = &ctx.SourceType
		}
		if ctx.SourceAppID != "" {
			auditLog.SourceAppID = &ctx.SourceAppID
		}
		if ctx.ExternalSystem != "" {
			auditLog.ExternalSystem = &ctx.ExternalSystem
		}
		if ctx.ExternalUserID != "" {
			auditLog.ExternalUserID = &ctx.ExternalUserID
		}
		if ctx.IP != "" {
			auditLog.IP = &ctx.IP
		}
		if ctx.UserAgent != "" {
			auditLog.UserAgent = &ctx.UserAgent
		}
		if !ctx.StartedAt.IsZero() {
			durationMs := int(time.Since(ctx.StartedAt).Milliseconds())
			auditLog.DurationMs = &durationMs
		}
	}
	if resource != "" {
		auditLog.Resource = &resource
	}
	if detail != "" {
		// detail 列是 JSON 类型，必须传合法 JSON 字符串
		jsonBytes, _ := json.Marshal(detail)
		jsonStr := string(jsonBytes)
		auditLog.Detail = &jsonStr
	}
	decision := "approved"
	auditLog.Decision = &decision
	result := "success"
	auditLog.Result = &result
	if err := auditRepo.Create(auditLog); err != nil {
		log.Printf("Failed to create audit log: %v", err)
	}
}

// toolPermissionMap 工具→权限映射
var toolPermissionMap = map[string]string{
	// 查询工具
	"list_users":         "users",
	"get_user":           "users",
	"create_user":        "users",
	"list_connectors":    "connectors",
	"get_connector":      "connectors",
	"list_mcp_tools":     "mcp-tools",
	"get_mcp_tool":       "mcp-tools",
	"list_skills":        "skills",
	"get_skill":          "skills",
	"list_memory_pools":  "memory",
	"get_memory_entries": "memory",
	// 角色工具
	"assign_role": "roles",
	"revoke_role": "roles",
	"list_roles":  "roles",
	"create_role": "roles",
	// 租户工具
	"get_tenant_info": "*",
	"update_tenant":   "*",
	// 写操作工具（需对应权限）
	"create_connector":    "connectors",
	"update_connector":    "connectors",
	"create_mcp_tool":     "mcp-tools",
	"update_mcp_tool":     "mcp-tools",
	"create_skill":        "skills",
	"update_skill":        "skills",
	"create_memory_pool":  "memory",
	"create_memory_entry": "memory",
	"update_memory_entry": "memory",
	// 技能执行
	"execute_skill": "skills",
	// MCP工具执行
	"execute_mcp_tool": "mcp-tools",
}

// getUserAllowedMCPSkills 获取用户角色允许的MCP工具ID和技能ID
// 返回: allowedMCPToolIDs, allowedSkillIDs, hasWildcard
func getUserAllowedMCPSkills(userID string) (map[string]bool, map[string]bool, bool) {
	roleRepo := repositories.NewRoleRepository()
	userRoleRepo := repositories.NewUserRoleRepository()

	roles, err := userRoleRepo.GetUserRoles(userID)
	if err != nil {
		return nil, nil, false
	}

	allowedMCPTools := make(map[string]bool)
	allowedSkills := make(map[string]bool)
	hasWildcard := false

	for _, role := range roles {
		// 检查 tools 字段是否有 "*"
		if role.Tools != nil {
			var tools []string
			json.Unmarshal([]byte(*role.Tools), &tools)
			for _, t := range tools {
				if t == "*" {
					hasWildcard = true
				}
			}
		}

		// 解析 allowed_mcp_tools
		if role.AllowedMCPTools != nil {
			var toolIDs []string
			if err := json.Unmarshal([]byte(*role.AllowedMCPTools), &toolIDs); err == nil {
				// 验证这些MCP工具确实存在且已启用
				for _, id := range toolIDs {
					allowedMCPTools[id] = true
				}
			}
		}

		// 解析 allowed_skills
		if role.AllowedSkills != nil {
			var skillIDs []string
			if err := json.Unmarshal([]byte(*role.AllowedSkills), &skillIDs); err == nil {
				for _, id := range skillIDs {
					allowedSkills[id] = true
				}
			}
		}
	}

	// 对于非通配符情况，二次过滤：只保留确实存在且已启用/已发布的
	// 因为角色允许列表中可能残留已删除的工具ID
	if !hasWildcard && len(allowedMCPTools) > 0 {
		validIDs := make([]string, 0, len(allowedMCPTools))
		for id := range allowedMCPTools {
			validIDs = append(validIDs, id)
		}
		placeholders := ""
		args := []any{}
		for i, id := range validIDs {
			if i > 0 {
				placeholders += ","
			}
			placeholders += "?"
			args = append(args, id)
		}
		// 当前租户下查询存在且有效的
		rows, err := database.DB.Query(`SELECT id FROM mcp_tools WHERE id IN (`+placeholders+`) AND enabled = true AND status IN ('published', 'active')`, args...)
		if err == nil {
			defer rows.Close()
			var valid []string
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err == nil {
					valid = append(valid, id)
				}
			}
			// 重建集合，只保留有效ID
			filtered := make(map[string]bool)
			for _, id := range valid {
				filtered[id] = true
			}
			allowedMCPTools = filtered
		}
	}

	// 技能也做同样过滤
	if !hasWildcard && len(allowedSkills) > 0 {
		validIDs := make([]string, 0, len(allowedSkills))
		for id := range allowedSkills {
			validIDs = append(validIDs, id)
		}
		placeholders := ""
		args := []any{}
		for i, id := range validIDs {
			if i > 0 {
				placeholders += ","
			}
			placeholders += "?"
			args = append(args, id)
		}
		rows, err := database.DB.Query(`SELECT id FROM skills WHERE id IN (`+placeholders+`) AND status IN ('published', 'active')`, args...)
		if err == nil {
			defer rows.Close()
			var valid []string
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err == nil {
					valid = append(valid, id)
				}
			}
			filtered := make(map[string]bool)
			for _, id := range valid {
				filtered[id] = true
			}
			allowedSkills = filtered
		}
	}

	// 防止未使用的导入
	_ = roleRepo

	return allowedMCPTools, allowedSkills, hasWildcard
}

// getToolsForPermissions 根据用户权限过滤工具列表
func getToolsForPermissions(permissions []string) []ToolDefinition {
	allTools := getTools()

	// 构建权限集合
	permSet := make(map[string]bool)
	hasWildcard := false
	for _, p := range permissions {
		permSet[p] = true
		if p == "*" {
			hasWildcard = true
		}
	}

	// 管理员拥有所有工具
	if hasWildcard {
		return allTools
	}

	// 按权限过滤
	var filtered []ToolDefinition
	for _, tool := range allTools {
		required, ok := toolPermissionMap[tool.Function.Name]
		if !ok {
			// 无映射的工具默认允许
			filtered = append(filtered, tool)
			continue
		}
		if permSet[required] {
			filtered = append(filtered, tool)
		}
	}

	if filtered == nil {
		filtered = []ToolDefinition{}
	}
	return filtered
}

type skillToolLoadResult struct {
	Tools                   []ToolDefinition
	UnavailableByToolName   map[string][]string
	UnavailableCapabilities []string
}

// loadSkillToolDefinitions 从数据库加载租户的 published Skills，转换为 AI 可直接调用的 ToolDefinition
func loadSkillToolDefinitions(tenantID string, allowedSkillIDs map[string]bool, hasWildcard bool, permissions []string) skillToolLoadResult {
	loadResult := skillToolLoadResult{UnavailableByToolName: map[string][]string{}}
	var skills []models.Skill
	var err error
	if hasWildcard {
		err = database.DB.Select(&skills, "SELECT * FROM skills WHERE tenant_id = ? AND status IN ('published', 'active')", tenantID)
	} else if len(allowedSkillIDs) > 0 {
		placeholders := ""
		args := []any{}
		for id := range allowedSkillIDs {
			if placeholders != "" {
				placeholders += ","
			}
			placeholders += "?"
			args = append(args, id)
		}
		args = append(args, tenantID)
		err = database.DB.Select(&skills, "SELECT * FROM skills WHERE id IN ("+placeholders+") AND tenant_id = ? AND status IN ('published', 'active')", args...)
	}
	if err != nil {
		log.Printf("loadSkillToolDefinitions: failed to load: %v", err)
		return loadResult
	}

	result := make([]ToolDefinition, 0, len(skills))
	for _, sk := range skills {
		// 跳过空名称的技能
		if sk.Name == "" {
			log.Printf("loadSkillToolDefinitions: skipping skill with empty name, id=%s", sk.ID)
			continue
		}

		log.Printf("loadSkillToolDefinitions: loaded skill id=%s, name=%s", sk.ID, sk.Name)

		toolName := makeSkillToolName(sk)

		params := map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
		if sk.InputSchema != nil && *sk.InputSchema != "" {
			if err := json.Unmarshal([]byte(*sk.InputSchema), &params); err != nil {
				log.Printf("loadSkillToolDefinitions: failed to parse input_schema for skill %s: %v", sk.ID, err)
			}
		}
		params = skillToolParametersForModel(params)

		desc := ""
		if sk.Description != nil {
			desc = *sk.Description
		}
		if desc == "" {
			desc = "技能：" + sk.Name
		}
		desc = "[技能] " + desc
		toolDef := ToolDefinition{
			Type: "function",
			Function: FunctionDef{
				Name:        toolName,
				Description: desc,
				Parameters:  params,
			},
		}

		if !hasWildcard {
			missingPermissions, err := missingPermissionsForSkillSteps(sk.Steps, permissions)
			if err != nil {
				log.Printf("loadSkillToolDefinitions: failed to inspect permissions for skill %s: %v", sk.ID, err)
				continue
			}
			if len(missingPermissions) > 0 {
				log.Printf("loadSkillToolDefinitions: hiding skill %s due missing permissions: %v", sk.ID, missingPermissions)
				loadResult.UnavailableByToolName[toolName] = missingPermissions
				loadResult.UnavailableCapabilities = append(loadResult.UnavailableCapabilities, unavailableCapabilityLines([]ToolDefinition{toolDef}, map[string][]string{toolName: missingPermissions})...)
				continue
			}
		}

		result = append(result, toolDef)
	}
	loadResult.Tools = result
	return loadResult
}

// loadMCPToolDefinitions 从数据库加载租户的MCP工具，转换为AI可调用的ToolDefinition
func skillToolParametersForModel(params map[string]any) map[string]any {
	if params == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	cloned := make(map[string]any, len(params))
	for k, v := range params {
		if k == "required" {
			continue
		}
		cloned[k] = v
	}
	if _, ok := cloned["type"]; !ok {
		cloned["type"] = "object"
	}
	if _, ok := cloned["properties"]; !ok {
		cloned["properties"] = map[string]any{}
	}
	return cloned
}

func missingPermissionsForSkillSteps(stepsJSON string, permissions []string) ([]string, error) {
	var steps []map[string]any
	if err := json.Unmarshal([]byte(stepsJSON), &steps); err != nil {
		return nil, err
	}
	permSet := make(map[string]bool, len(permissions))
	hasWildcard := false
	for _, p := range permissions {
		permSet[p] = true
		if p == "*" {
			hasWildcard = true
		}
	}
	if hasWildcard {
		return nil, nil
	}

	missingSet := map[string]bool{}
	var walk func([]map[string]any)
	walk = func(items []map[string]any) {
		for _, step := range items {
			stepType, _ := step["type"].(string)
			if stepType == "mcp_tool" {
				name, _ := step["action"].(string)
				if required, ok := toolPermissionMap[name]; ok && required != "*" && !permSet[required] {
					missingSet[required] = true
				}
			}
			params, _ := step["params"].(map[string]any)
			if childRaw, ok := params["steps"]; ok {
				if childSteps, err := parseSkillStepMaps(childRaw); err == nil {
					walk(childSteps)
				}
			}
		}
	}
	walk(steps)

	missing := make([]string, 0, len(missingSet))
	for permission := range missingSet {
		missing = append(missing, permission)
	}
	sort.Strings(missing)
	return missing, nil
}

func assistantIntentMissingPermissions(userText string, permissions []string) (string, []string) {
	text := strings.TrimSpace(userText)
	if text == "" {
		return "", nil
	}
	permSet := make(map[string]bool, len(permissions))
	for _, permission := range permissions {
		permSet[permission] = true
	}
	if permSet["*"] {
		return "", nil
	}

	missingFor := func(required ...string) []string {
		missing := make([]string, 0, len(required))
		for _, permission := range required {
			if !permSet[permission] {
				missing = append(missing, permission)
			}
		}
		sort.Strings(missing)
		return missing
	}

	if isCreateUserIntent(text) {
		missing := missingFor("users", "roles")
		if len(missing) > 0 {
			return "创建用户", missing
		}
	}
	if containsAnyText(text, "创建角色", "新增角色", "添加角色") {
		missing := missingFor("roles")
		if len(missing) > 0 {
			return "创建角色", missing
		}
	}
	if containsAnyText(text, "创建MCP", "新增MCP", "添加MCP", "导入MCP", "创建 mcp", "新增 mcp", "导入 mcp") {
		missing := missingFor("mcp-tools")
		if len(missing) > 0 {
			return "创建 MCP 工具", missing
		}
	}
	return "", nil
}

func isCreateUserIntent(text string) bool {
	if containsAnyText(text, "创建用户", "新增用户", "添加用户", "创建账号", "新增账号", "添加账号", "测试账号") {
		return true
	}
	return containsAnyText(text, "创建", "新增", "添加") && containsAnyText(text, "用户", "账号", "帐号")
}

func containsAnyText(text string, keywords ...string) bool {
	lower := strings.ToLower(text)
	for _, keyword := range keywords {
		if strings.Contains(lower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func skillMissingPermissionMessage(skillName string, permissions []string) string {
	if len(permissions) == 0 {
		return ""
	}
	labels := permissionLabels(permissions)
	return fmt.Sprintf("你当前没有执行“%s”所需的权限：%s。请联系管理员为你的角色开通对应权限后再操作。", skillName, strings.Join(labels, "、"))
}

var permissionDisplayLabels = map[string]string{
	"users":      "用户管理",
	"roles":      "角色管理",
	"mcp-tools":  "MCP 工具管理",
	"skills":     "技能管理",
	"connectors": "连接器管理",
	"memory":     "记忆管理",
}

func permissionLabels(permissions []string) []string {
	labels := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		if label, ok := permissionDisplayLabels[permission]; ok {
			labels = append(labels, label)
		} else {
			labels = append(labels, permission)
		}
	}
	return labels
}

func toolDisplayLabel(tool ToolDefinition) string {
	desc := strings.TrimSpace(tool.Function.Description)
	desc = strings.TrimPrefix(desc, "[技能]")
	desc = strings.TrimSpace(desc)
	if desc != "" {
		return desc
	}
	return getToolDisplayName(tool.Function.Name)
}

func unavailableCapabilityLines(tools []ToolDefinition, missingByToolName map[string][]string) []string {
	lines := make([]string, 0)
	seen := map[string]bool{}
	for _, tool := range tools {
		missing := missingByToolName[tool.Function.Name]
		if len(missing) == 0 {
			continue
		}
		line := fmt.Sprintf("%s：缺少 %s", toolDisplayLabel(tool), strings.Join(permissionLabels(missing), "、"))
		if !seen[line] {
			seen[line] = true
			lines = append(lines, line)
		}
	}
	sort.Strings(lines)
	return lines
}

func unavailableCapabilityLinesForMissingPermissions(permissions []string) []string {
	permSet := make(map[string]bool, len(permissions))
	for _, permission := range permissions {
		permSet[permission] = true
	}
	if permSet["*"] {
		return nil
	}
	missingByToolName := map[string][]string{}
	for _, tool := range getTools() {
		required, ok := toolPermissionMap[tool.Function.Name]
		if !ok || required == "*" || permSet[required] {
			continue
		}
		missingByToolName[tool.Function.Name] = []string{required}
	}
	return unavailableCapabilityLines(getTools(), missingByToolName)
}

func toolPermissionDeniedResult(toolName string, missingByToolName map[string][]string) (string, bool) {
	missing := missingByToolName[toolName]
	if len(missing) == 0 {
		return "", false
	}
	message := skillMissingPermissionMessage(getToolDisplayName(toolName), missing)
	data, _ := json.Marshal(map[string]any{"error": message, "permission_denied": true})
	return string(data), true
}

func parseSkillStepMaps(raw any) ([]map[string]any, error) {
	switch v := raw.(type) {
	case []map[string]any:
		return v, nil
	case []any:
		steps := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				steps = append(steps, m)
			}
		}
		return steps, nil
	case string:
		var steps []map[string]any
		if err := json.Unmarshal([]byte(v), &steps); err != nil {
			return nil, err
		}
		return steps, nil
	default:
		return nil, fmt.Errorf("unsupported steps type %T", raw)
	}
}

func loadMCPToolDefinitions(tenantID string, allowedIDs map[string]bool, hasWildcard bool) []ToolDefinition {
	var tools []models.MCPTool
	var err error
	if hasWildcard {
		err = database.DB.Select(&tools, "SELECT * FROM mcp_tools WHERE tenant_id = ? AND enabled = true AND status IN ('published', 'active')", tenantID)
	} else if len(allowedIDs) > 0 {
		placeholders := ""
		args := []any{}
		for id := range allowedIDs {
			if placeholders != "" {
				placeholders += ","
			}
			placeholders += "?"
			args = append(args, id)
		}
		args = append(args, tenantID)
		err = database.DB.Select(&tools, "SELECT * FROM mcp_tools WHERE id IN ("+placeholders+") AND tenant_id = ? AND enabled = true AND status IN ('published', 'active')", args...)
	}
	if err != nil {
		log.Printf("loadMCPToolDefinitions: failed to load: %v", err)
		return nil
	}

	result := make([]ToolDefinition, 0, len(tools))
	for _, tool := range tools {
		// 解析 input_schema 作为 parameters
		var params map[string]any
		if tool.InputSchema != nil && *tool.InputSchema != "" {
			if err := json.Unmarshal([]byte(*tool.InputSchema), &params); err != nil {
				// 如果解析失败，创建一个空的 parameters
				params = map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				}
			}
		} else {
			params = map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}
		}

		desc := ""
		if tool.Description != nil {
			desc = *tool.Description
		}
		if desc == "" {
			desc = "MCP tool: " + tool.Name
		}

		result = append(result, ToolDefinition{
			Type: "function",
			Function: FunctionDef{
				Name:        "mcp_" + sanitizeToolName(tool.Name),
				Description: desc,
				Parameters:  params,
			},
		})
	}
	return result
}

// getTools 获取工具定义
func getTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "list_users",
				Description: "获取当前租户下的用户列表",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_user",
				Description: "根据邮箱获取用户信息",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"email": map[string]any{
							"type":        "string",
							"description": "用户邮箱",
						},
					},
					"required": []string{"email"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "assign_role",
				Description: "为用户分配角色",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"user_email": map[string]any{
							"type":        "string",
							"description": "用户邮箱",
						},
						"role_name": map[string]any{
							"type":        "string",
							"description": "角色名称，如：管理员、开发者、普通用户",
						},
					},
					"required": []string{"user_email", "role_name"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "revoke_role",
				Description: "撤销用户的角色",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"user_email": map[string]any{
							"type":        "string",
							"description": "用户邮箱",
						},
						"role_name": map[string]any{
							"type":        "string",
							"description": "角色名称",
						},
					},
					"required": []string{"user_email", "role_name"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "list_roles",
				Description: "获取当前租户下的角色列表",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "list_connectors",
				Description: "获取当前租户下的连接器列表",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "list_mcp_tools",
				Description: "获取当前租户下的MCP工具列表",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_tenant_info",
				Description: "获取当前租户的详细信息，包括到期时间、用户上限等",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "update_tenant",
				Description: "更新租户配置，如到期时间、最大用户数等",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"expires_at": map[string]any{
							"type":        "string",
							"description": "到期时间，格式YYYY-MM-DD，空字符串表示永久有效",
						},
						"max_users": map[string]any{
							"type":        "integer",
							"description": "最大用户数，0表示不限制",
						},
					},
				},
			},
		},
		// ========== 连接器工具 ==========
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_connector",
				Description: "获取连接器详情",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"connector_id": map[string]any{"type": "string", "description": "连接器ID"},
					},
					"required": []string{"connector_id"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "create_connector",
				Description: "创建新的API连接器。连接器是API-to-MCP的桥梁，用于接入外部API服务。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":      map[string]any{"type": "string", "description": "连接器名称，如 my-api"},
						"type":      map[string]any{"type": "string", "description": "连接器类型，如 openapi, custom"},
						"base_url":  map[string]any{"type": "string", "description": "API基础URL，如 https://api.example.com/v1"},
						"auth_type": map[string]any{"type": "string", "description": "认证类型: none, api_key, bearer, basic"},
					},
					"required": []string{"name", "type", "base_url"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "update_connector",
				Description: "更新连接器配置",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"connector_id": map[string]any{"type": "string", "description": "连接器ID"},
						"name":         map[string]any{"type": "string", "description": "新名称"},
						"base_url":     map[string]any{"type": "string", "description": "新URL"},
						"status":       map[string]any{"type": "string", "description": "状态: active, inactive, error"},
					},
					"required": []string{"connector_id"},
				},
			},
		},
		// ========== MCP工具 ==========
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_mcp_tool",
				Description: "获取MCP工具详情",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"tool_id": map[string]any{"type": "string", "description": "MCP工具ID"},
						"name":    map[string]any{"type": "string", "description": "MCP工具名称，tool_id 和 name 必填其一"},
					},
					"required": []string{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "create_mcp_tool",
				Description: "创建新的MCP工具。MCP工具是对外暴露的可调用工具，需要关联到一个连接器。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"connector_id":   map[string]any{"type": "string", "description": "所属连接器ID"},
					"name":           map[string]any{"type": "string", "description": "工具名称，如 get_user_info"},
					"description":    map[string]any{"type": "string", "description": "工具描述"},
					"input_schema":   map[string]any{"type": "string", "description": "输入参数JSON Schema，JSON格式字符串"},
					"backend_method": map[string]any{"type": "string", "description": "HTTP方法: GET, POST, PUT, DELETE"},
					"backend_path":   map[string]any{"type": "string", "description": "API路径，如 /users/{id}"},
					"risk_level":     map[string]any{"type": "string", "description": "风险等级: low, medium, high"},
				},
				"required": []string{"connector_id", "name", "backend_method", "backend_path"},
			},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "update_mcp_tool",
				Description: "更新MCP工具配置",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"tool_id":        map[string]any{"type": "string", "description": "MCP工具ID"},
					"name":           map[string]any{"type": "string", "description": "新名称"},
					"description":    map[string]any{"type": "string", "description": "新描述"},
					"input_schema":   map[string]any{"type": "string", "description": "新的输入参数JSON Schema"},
					"backend_method": map[string]any{"type": "string", "description": "HTTP方法"},
					"backend_path":   map[string]any{"type": "string", "description": "API路径"},
					"risk_level":     map[string]any{"type": "string", "description": "风险等级"},
					"enabled":        map[string]any{"type": "boolean", "description": "是否启用"},
				},
				"required": []string{"tool_id"},
			},
			},
		},
		// ========== 技能工具 ==========
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "list_skills",
				Description: "获取当前租户下的技能列表",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_skill",
				Description: "获取技能详情，包含步骤定义",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"skill_id": map[string]any{"type": "string", "description": "技能ID"},
						"name":     map[string]any{"type": "string", "description": "技能名称，skill_id 和 name 必填其一"},
					},
					"required": []string{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "create_skill",
				Description: "创建新技能。技能是一组可编排的步骤，用于自动化复杂操作流程。steps必须是JSON数组字符串。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":        map[string]any{"type": "string", "description": "技能名称"},
						"description": map[string]any{"type": "string", "description": "技能描述"},
						"steps":       map[string]any{"type": "string", "description": "步骤定义JSON数组，如 [{\"name\":\"step1\",\"type\":\"mcp_tool\",\"config\":{\"tool_name\":\"xxx\"}}]"},
						"triggers":    map[string]any{"type": "string", "description": "触发条件JSON数组"},
					},
					"required": []string{"name", "steps"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "update_skill",
				Description: "更新技能配置",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"skill_id":    map[string]any{"type": "string", "description": "技能ID"},
						"name":        map[string]any{"type": "string", "description": "新名称"},
						"description": map[string]any{"type": "string", "description": "新描述"},
						"steps":       map[string]any{"type": "string", "description": "新步骤定义"},
						"status":      map[string]any{"type": "string", "description": "生命周期: draft, testing, published, disabled"},
					},
					"required": []string{"skill_id"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "execute_skill",
				Description: "执行一个技能。技能是一组预定义的自动化步骤。根据技能的input_schema传入必要参数。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"skill_id":       map[string]any{"type": "string", "description": "技能ID"},
						"inputs":         map[string]any{"type": "object", "description": "技能输入参数，根据技能的input_schema定义传入"},
						"execution_mode": map[string]any{"type": "string", "enum": []string{"sandbox", "dry_run", "production"}, "description": "执行模式，默认sandbox；production只允许published技能"},
					},
					"required": []string{"skill_id"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "execute_mcp_tool",
				Description: "调用一个MCP工具。MCP工具是通过连接器接入的外部API能力。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"tool_id":        map[string]any{"type": "string", "description": "MCP工具ID"},
						"name":           map[string]any{"type": "string", "description": "MCP工具名称，tool_id 和 name 必填其一"},
						"arguments":      map[string]any{"type": "object", "description": "工具调用参数"},
						"execution_mode": map[string]any{"type": "string", "enum": []string{"sandbox", "dry_run", "production"}, "description": "执行模式，默认sandbox；production只允许published工具"},
					},
					"required": []string{},
				},
			},
		},
		// ========== 记忆工具 ==========
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "list_memory_pools",
				Description: "获取当前租户下的记忆池列表",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_memory_entries",
				Description: "获取指定记忆池中的记忆条目",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"pool_id": map[string]any{"type": "string", "description": "记忆池ID"},
						"limit":   map[string]any{"type": "integer", "description": "返回条数，默认20"},
					},
					"required": []string{"pool_id"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "create_memory_pool",
				Description: "创建新的记忆池。记忆池用于按类型和用途组织长期记忆。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":        map[string]any{"type": "string", "description": "记忆池名称"},
						"type":        map[string]any{"type": "string", "description": "记忆池类型: personal, team, system"},
						"purpose":     map[string]any{"type": "string", "description": "用途: conversation, skill, knowledge"},
						"description": map[string]any{"type": "string", "description": "描述"},
						"owner_id":    map[string]any{"type": "string", "description": "个人池所属用户ID，可选"},
					},
					"required": []string{"name"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "create_memory_entry",
				Description: "在记忆池中创建新的记忆条目",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"pool_id":     map[string]any{"type": "string", "description": "记忆池ID"},
						"type":        map[string]any{"type": "string", "description": "条目类型: fact, preference, feedback, instruction"},
						"content":     map[string]any{"type": "string", "description": "记忆内容"},
						"sensitivity": map[string]any{"type": "string", "description": "敏感度: low, medium, high"},
					},
					"required": []string{"pool_id", "type", "content"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "update_memory_entry",
				Description: "更新记忆条目内容",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"entry_id":    map[string]any{"type": "string", "description": "记忆条目ID"},
						"content":     map[string]any{"type": "string", "description": "新内容"},
						"sensitivity": map[string]any{"type": "string", "description": "新敏感度"},
					},
					"required": []string{"entry_id"},
				},
			},
		},
	}
}

// getToolDisplayName 获取工具中文名
func getToolDisplayName(toolName string) string {
	names := map[string]string{
		"list_users":          "查询用户列表",
		"get_user":            "查询用户信息",
		"assign_role":         "分配角色",
		"revoke_role":         "撤销角色",
		"list_roles":          "查询角色列表",
		"list_connectors":     "查询连接器列表",
		"get_connector":       "查询连接器详情",
		"create_connector":    "创建连接器",
		"update_connector":    "更新连接器",
		"list_mcp_tools":      "查询MCP工具列表",
		"get_mcp_tool":        "查询MCP工具详情",
		"create_mcp_tool":     "创建MCP工具",
		"update_mcp_tool":     "更新MCP工具",
		"list_skills":         "查询技能列表",
		"get_skill":           "查询技能详情",
		"create_skill":        "创建技能",
		"update_skill":        "更新技能",
		"execute_skill":       "执行技能",
		"execute_mcp_tool":    "调用MCP工具",
		"list_memory_pools":   "查询记忆池列表",
		"get_memory_entries":  "查询记忆条目",
		"create_memory_pool":  "创建记忆池",
		"create_memory_entry": "创建记忆条目",
		"update_memory_entry": "更新记忆条目",
		"get_tenant_info":     "查询租户信息",
		"update_tenant":       "更新租户配置",
	}
	if name, ok := names[toolName]; ok {
		return name
	}
	return toolName
}

// ExecuteToolByName 导出工具执行函数，供SkillEngine等外部调用
func ExecuteToolByName(tenantID, toolName string, args map[string]any) string {
	h := &ChatHandler{}
	return h.executeTool(tenantID, "", toolName, args)
}

func classifyCalledTool(toolName string) (resourceType, resourceID, resourceName string) {
	resourceName = toolName
	if strings.HasPrefix(toolName, "skill_") {
		return "skill", strings.TrimPrefix(toolName, "skill_"), resourceName
	}
	if strings.HasPrefix(toolName, "mcp_") {
		return "mcp_tool", strings.TrimPrefix(toolName, "mcp_"), resourceName
	}
	return "builtin_tool", "", resourceName
}

func toolCallStatusFromResult(result string) (string, error) {
	var obj map[string]any
	if err := json.Unmarshal([]byte(result), &obj); err == nil {
		if errText, ok := obj["error"].(string); ok && errText != "" {
			return "failed", fmt.Errorf("%s", errText)
		}
	}
	return "success", nil
}

func permissionDeniedReplyFromToolResult(result string) (string, bool) {
	var obj map[string]any
	if err := json.Unmarshal([]byte(result), &obj); err != nil {
		return "", false
	}
	denied, _ := obj["permission_denied"].(bool)
	if !denied {
		return "", false
	}
	message, _ := obj["error"].(string)
	if strings.TrimSpace(message) == "" {
		message = "当前没有执行该操作所需的权限，请联系管理员开通后再操作。"
	}
	return message, true
}

func requiresInputReplyFromToolResult(result string, userText ...string) (string, bool) {
	var obj map[string]any
	if err := json.Unmarshal([]byte(result), &obj); err != nil {
		return "", false
	}
	status, _ := obj["status"].(string)
	if status != "requires_input" {
		return "", false
	}
	outputs, _ := obj["outputs"].(map[string]any)
	message, _ := outputs["message"].(string)
	if strings.TrimSpace(message) == "" {
		message = "需要补充必要信息后才能继续。"
	}
	missing := stringifyFieldList(outputs["missing_fields"])
	if len(missing) > 0 {
		return fmt.Sprintf("%s\n请补充：%s", message, strings.Join(localizeMissingFields(missing, firstNonEmpty(userText...)), "、")), true
	}
	return message, true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func localizeMissingFields(fields []string, userText string) []string {
	localized := make([]string, 0, len(fields))
	for _, field := range fields {
		localized = append(localized, localizeFieldName(field, userText))
	}
	return localized
}

func localizeFieldName(field, userText string) string {
	if isLikelyChinese(userText) {
		if label, ok := chineseFieldLabels[field]; ok {
			return label
		}
		return strings.ReplaceAll(field, "_", " ")
	}
	return field
}

func isLikelyChinese(text string) bool {
	for _, r := range text {
		if r >= '\u4e00' && r <= '\u9fff' {
			return true
		}
	}
	return false
}

var chineseFieldLabels = map[string]string{
	"email":             "邮箱",
	"display_name":      "显示名称",
	"role_name":         "角色名称",
	"name":              "名称",
	"curl":              "curl 命令",
	"test_result_id":    "curl 测试结果ID",
	"description":       "说明",
	"tools":             "功能权限",
	"allowed_mcp_tools": "允许使用的 MCP 工具",
	"allowed_skills":    "允许使用的技能",
	"data_scope":        "数据范围",
	"risk_level":        "风险等级",
	"timeout_seconds":   "测试超时秒数",
	"connector_id":      "连接器ID",
	"tool_id":           "工具ID",
	"skill_id":          "技能ID",
	"backend_method":    "后端请求方法",
	"backend_path":      "后端接口路径",
	"input_schema":      "输入参数JSON Schema",
	"base_url":          "服务地址",
	"type":              "类型",
	"pool_id":           "记忆池ID",
	"entry_id":          "记忆条目ID",
	"content":           "内容",
	"user_email":        "用户邮箱",
	"new_password":      "新密码",
	"old_password":      "旧密码",
	"tenant_id":         "租户ID",
	"refresh_token":     "刷新令牌",
}

func stringifyFieldList(v any) []string {
	result := []string{}
	switch fields := v.(type) {
	case []string:
		result = fields
	case []any:
		for _, item := range fields {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				result = append(result, strings.TrimSpace(s))
			}
		}
	}
	return result
}

func jsonStringArg(args map[string]any, key string, fallback string) string {
	value, ok := args[key]
	if !ok || value == nil {
		return fallback
	}
	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return fallback
		}
		return v
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fallback
		}
		return string(data)
	}
}

func truthy(v any) bool {
	switch value := v.(type) {
	case bool:
		return value
	case int:
		return value != 0
	case int64:
		return value != 0
	case uint64:
		return value != 0
	case []byte:
		s := strings.TrimSpace(string(value))
		return s == "1" || strings.EqualFold(s, "true")
	case string:
		s := strings.TrimSpace(value)
		return s == "1" || strings.EqualFold(s, "true")
	default:
		return fmt.Sprint(value) == "1" || strings.EqualFold(fmt.Sprint(value), "true")
	}
}

// writeTools 需要审计日志的写操作工具集合
var writeTools = map[string]bool{
	"create_connector": true, "update_connector": true,
	"create_mcp_tool": true, "update_mcp_tool": true,
	"create_user": true, "create_role": true,
	"create_skill": true, "update_skill": true,
	"create_memory_pool": true, "create_memory_entry": true, "update_memory_entry": true,
	"update_tenant": true, "assign_role": true, "revoke_role": true,
}

// executeTool 执行工具调用
func (h *ChatHandler) executeTool(tenantID, userID, toolName string, args map[string]any, auditCtx ...*auditContext) string {
	var ctx *auditContext
	if len(auditCtx) > 0 {
		ctx = auditCtx[0]
	}
	userRepo := repositories.NewUserRepository()
	roleRepo := repositories.NewRoleRepository()
	userRoleRepo := repositories.NewUserRoleRepository()

	switch toolName {
	case "list_users":
		users, err := userRepo.ListByTenant(tenantID)
		if err != nil {
			return `{"error": "查询用户失败: ` + err.Error() + `"}`
		}
		result := make([]map[string]any, 0)
		for _, u := range users {
			roles, _ := userRoleRepo.GetUserRoles(u.ID)
			roleNames := make([]string, 0)
			for _, r := range roles {
				roleNames = append(roleNames, r.Name)
			}
			result = append(result, map[string]any{
				"id":           u.ID,
				"email":        u.Email,
				"display_name": u.DisplayName,
				"status":       u.Status,
				"roles":        roleNames,
				"created_at":   u.CreatedAt.Format("2006-01-02 15:04"),
			})
		}
		data, _ := json.Marshal(result)
		return string(data)

	case "get_user":
		email, _ := args["email"].(string)
		user, err := userRepo.GetByEmail(email)
		if err != nil {
			return `{"error": "用户不存在: ` + email + `"}`
		}
		if user.TenantID != tenantID {
			return `{"error": "该用户不属于当前租户"}`
		}
		roles, _ := userRoleRepo.GetUserRoles(user.ID)
		roleNames := make([]string, 0)
		for _, r := range roles {
			roleNames = append(roleNames, r.Name)
		}
		data, _ := json.Marshal(map[string]any{
			"id":           user.ID,
			"email":        user.Email,
			"display_name": user.DisplayName,
			"status":       user.Status,
			"roles":        roleNames,
			"login_count":  user.LoginCount,
			"created_at":   user.CreatedAt.Format("2006-01-02 15:04"),
		})
		return string(data)

	case "create_user":
		email, _ := args["email"].(string)
		displayName, _ := args["display_name"].(string)
		roleName, _ := args["role_name"].(string)
		if email == "" || displayName == "" {
			return `{"error": "email, display_name 为必填项"}`
		}
		if existing, err := userRepo.GetByEmail(email); err == nil && existing != nil {
			if existing.TenantID == tenantID {
				return `{"success": true, "id": "` + existing.ID + `", "message": "用户已存在，未重复创建"}`
			}
			return `{"error": "邮箱已被其他租户占用"}`
		}
		randomPassword := uuid.New().String()
		passwordHash, _ := bcrypt.GenerateFromPassword([]byte(randomPassword), bcrypt.DefaultCost)
		now := time.Now()
		user := &models.User{ID: uuid.New().String(), TenantID: tenantID, Email: email, DisplayName: displayName, Status: "active", PasswordHash: string(passwordHash), CreatedAt: now, UpdatedAt: now}
		if err := userRepo.Create(user); err != nil {
			return `{"error": "创建用户失败: ` + err.Error() + `"}`
		}
		if roleName != "" {
			if role, err := roleRepo.GetByName(tenantID, roleName); err == nil && role != nil {
				_ = userRoleRepo.Assign(user.ID, role.ID)
			}
		}
		logAudit(ctx, tenantID, userID, "create_user", "create", "user", fmt.Sprintf("创建用户: %s", email))
		return `{"success": true, "id": "` + user.ID + `", "message": "用户 ` + email + ` 创建成功；未设置明文密码，请通过重置密码或SSO登录。"}`

	case "assign_role":
		userEmail, _ := args["user_email"].(string)
		roleName, _ := args["role_name"].(string)
		user, err := userRepo.GetByEmail(userEmail)
		if err != nil {
			return `{"error": "用户不存在: ` + userEmail + `"}`
		}
		if user.TenantID != tenantID {
			return `{"error": "该用户不属于当前租户"}`
		}
		role, err := roleRepo.GetByName(tenantID, roleName)
		if err != nil {
			return `{"error": "角色不存在: ` + roleName + `"}`
		}
		if err := userRoleRepo.Assign(user.ID, role.ID); err != nil {
			return `{"error": "分配角色失败: ` + err.Error() + `"}`
		}
		logAudit(ctx, tenantID, userID, "assign_role", "assign", "user_role", fmt.Sprintf("为用户 %s 分配角色 %s", userEmail, roleName))
		return `{"success": true, "message": "已为用户 ` + userEmail + ` 分配角色 ` + roleName + `"}`

	case "revoke_role":
		userEmail, _ := args["user_email"].(string)
		roleName, _ := args["role_name"].(string)
		user, err := userRepo.GetByEmail(userEmail)
		if err != nil {
			return `{"error": "用户不存在: ` + userEmail + `"}`
		}
		role, err := roleRepo.GetByName(tenantID, roleName)
		if err != nil {
			return `{"error": "角色不存在: ` + roleName + `"}`
		}
		if err := userRoleRepo.Revoke(user.ID, role.ID); err != nil {
			return `{"error": "撤销角色失败: ` + err.Error() + `"}`
		}
		logAudit(ctx, tenantID, userID, "revoke_role", "revoke", "user_role", fmt.Sprintf("撤销用户 %s 的角色 %s", userEmail, roleName))
		return `{"success": true, "message": "已撤销用户 ` + userEmail + ` 的角色 ` + roleName + `"}`

	case "list_roles":
		roles, err := roleRepo.ListByTenant(tenantID)
		if err != nil {
			return `{"error": "查询角色失败: ` + err.Error() + `"}`
		}
		result := make([]map[string]any, 0)
		for _, r := range roles {
			result = append(result, map[string]any{
				"id":          r.ID,
				"name":        r.Name,
				"description": r.Description,
				"is_default":  r.IsDefault,
			})
		}
		data, _ := json.Marshal(result)
		return string(data)

	case "create_role":
		name, _ := args["name"].(string)
		if name == "" {
			return `{"error": "name 为必填项"}`
		}
		if existing, err := roleRepo.GetByName(tenantID, name); err == nil && existing != nil {
			return `{"success": true, "id": "` + existing.ID + `", "message": "角色已存在，未重复创建"}`
		}
		desc, _ := args["description"].(string)
		tools := jsonStringArg(args, "tools", "[]")
		allowedMCPTools := jsonStringArg(args, "allowed_mcp_tools", "[]")
		allowedSkills := jsonStringArg(args, "allowed_skills", "[]")
		dataScope, _ := args["data_scope"].(string)
		if dataScope == "" {
			dataScope = "tenant"
		}
		rateLimit, _ := args["rate_limit"].(string)
		if rateLimit == "" {
			rateLimit = "500/hour"
		}
		now := time.Now()
		role := &models.Role{ID: uuid.New().String(), TenantID: tenantID, Name: name, Description: toPtr(desc), Tools: toPtr(tools), AllowedMCPTools: toPtr(allowedMCPTools), AllowedSkills: toPtr(allowedSkills), RateLimit: toPtr(rateLimit), DataScope: toPtr(dataScope), IsSystem: false, IsDefault: false, CreatedAt: now, UpdatedAt: now}
		if err := roleRepo.Create(role); err != nil {
			return `{"error": "创建角色失败: ` + err.Error() + `"}`
		}
		logAudit(ctx, tenantID, userID, "create_role", "create", "role", fmt.Sprintf("创建角色: %s", name))
		return `{"success": true, "id": "` + role.ID + `", "message": "角色 ` + name + ` 创建成功"}`

	case "list_connectors":
		keyword, _ := args["keyword"].(string)
		query := "SELECT CAST(id AS CHAR) as id, CAST(name AS CHAR) as name, CAST(type AS CHAR) as type, CAST(base_url AS CHAR) as base_url, CAST(status AS CHAR) as status, tools_count FROM connectors WHERE tenant_id = ?"
		args_sql := []any{tenantID}
		if keyword != "" {
			query += " AND (name LIKE ? OR base_url LIKE ? OR description LIKE ? OR id LIKE ?)"
			kw := "%" + keyword + "%"
			args_sql = append(args_sql, kw, kw, kw, kw)
		}
		connectors, err := queryMaps(query, args_sql...)
		if err != nil {
			return `{"error": "查询连接器失败: ` + err.Error() + `"}`
		}
		data, _ := json.Marshal(connectors)
		return string(data)

	case "list_mcp_tools":
		// 根据角色权限过滤MCP工具
		allowedMCPToolIDs, _, mcpWildcard := getUserAllowedMCPSkills(userID)
		var mcpToolsList []map[string]any
		var mcpErr error
		if mcpWildcard {
			mcpToolsList, mcpErr = queryMaps("SELECT CAST(id AS CHAR) as id, CAST(name AS CHAR) as name, CAST(description AS CHAR) as description, CAST(backend_method AS CHAR) as backend_method, CAST(backend_path AS CHAR) as backend_path, CAST(risk_level AS CHAR) as risk_level, enabled FROM mcp_tools WHERE tenant_id = ?", tenantID)
		} else if len(allowedMCPToolIDs) > 0 {
			placeholders := ""
			args := []any{}
			for id := range allowedMCPToolIDs {
				if placeholders != "" {
					placeholders += ","
				}
				placeholders += "?"
				args = append(args, id)
			}
			args = append(args, tenantID)
			mcpToolsList, mcpErr = queryMaps("SELECT CAST(id AS CHAR) as id, CAST(name AS CHAR) as name, CAST(description AS CHAR) as description, CAST(backend_method AS CHAR) as backend_method, CAST(backend_path AS CHAR) as backend_path, CAST(risk_level AS CHAR) as risk_level, enabled FROM mcp_tools WHERE id IN ("+placeholders+") AND tenant_id = ?", args...)
		} else {
			mcpToolsList = []map[string]any{}
		}
		if mcpErr != nil {
			return `{"error": "查询MCP工具失败: ` + mcpErr.Error() + `"}`
		}
		if mcpToolsList == nil {
			mcpToolsList = []map[string]any{}
		}
		data, _ := json.Marshal(mcpToolsList)
		return string(data)

	case "get_tenant_info":
		tenants, err := queryMaps("SELECT CAST(id AS CHAR) as id, CAST(name AS CHAR) as name, CAST(plan AS CHAR) as plan, CAST(status AS CHAR) as status, expires_at, max_users, created_at FROM tenants WHERE id = ?", tenantID)
		if err != nil {
			return `{"error": "查询租户信息失败: ` + err.Error() + `"}`
		}
		if len(tenants) == 0 {
			return `{"error": "租户不存在"}`
		}
		data, _ := json.Marshal(tenants[0])
		return string(data)

	case "update_tenant":
		sets := []string{}
		args_sql := []any{}
		if expiresAt, ok := args["expires_at"]; ok {
			if s, ok := expiresAt.(string); ok && s == "" {
				sets = append(sets, "expires_at = NULL")
			} else if ok && s != "" {
				sets = append(sets, "expires_at = ?")
				args_sql = append(args_sql, s)
			}
		}
		if maxUsers, ok := args["max_users"]; ok {
			if f, ok := maxUsers.(float64); ok {
				sets = append(sets, "max_users = ?")
				args_sql = append(args_sql, int(f))
			}
		}
		if len(sets) == 0 {
			return `{"error": "没有需要更新的字段"}`
		}
		sets = append(sets, "updated_at = NOW()")
		query := "UPDATE tenants SET " + strings.Join(sets, ", ") + " WHERE id = ?"
		args_sql = append(args_sql, tenantID)
		if _, err := database.DB.Exec(query, args_sql...); err != nil {
			return `{"error": "更新租户失败: ` + err.Error() + `"}`
		}
		logAudit(ctx, tenantID, userID, "update_tenant", "update", "tenant", fmt.Sprintf("更新租户配置: %v", args))
		return `{"success": true, "message": "租户配置已更新"}`

	// ========== 连接器工具 ==========
	case "get_connector":
		connectorID, _ := args["connector_id"].(string)
		connector, err := queryMaps("SELECT CAST(id AS CHAR) as id, CAST(name AS CHAR) as name, CAST(type AS CHAR) as type, CAST(base_url AS CHAR) as base_url, CAST(auth_type AS CHAR) as auth_type, CAST(status AS CHAR) as status, tools_count, created_at FROM connectors WHERE id = ? AND tenant_id = ?", connectorID, tenantID)
		if err != nil || len(connector) == 0 {
			return `{"error": "连接器不存在"}`
		}
		data, _ := json.Marshal(connector[0])
		return string(data)

	case "create_connector":
		name, _ := args["name"].(string)
		connType, _ := args["type"].(string)
		baseURL, _ := args["base_url"].(string)
		authType, _ := args["auth_type"].(string)
		if name == "" || connType == "" || baseURL == "" {
			return `{"error": "name, type, base_url 为必填项"}`
		}
		id := uuid.New().String()
		now := time.Now()
		if authType == "" {
			authType = "none"
		}
		_, err := database.DB.Exec(`INSERT INTO connectors (id, tenant_id, name, type, base_url, auth_type, status, tools_count, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 'active', 0, ?, ?)`,
			id, tenantID, name, connType, baseURL, authType, now, now)
		if err != nil {
			return `{"error": "创建连接器失败: ` + err.Error() + `"}`
		}
		logAudit(ctx, tenantID, userID, "create_connector", "create", "connector", fmt.Sprintf("创建连接器: %s (%s)", name, connType))
		return `{"success": true, "id": "` + id + `", "message": "连接器 ` + name + ` 创建成功"}`

	case "update_connector":
		connectorID, _ := args["connector_id"].(string)
		if connectorID == "" {
			return `{"error": "connector_id 为必填项"}`
		}
		// 验证连接器属于当前租户，并拦截内置/锁定连接器，防止通过助手治理工具绕过管理 API。
		existing, _ := queryMaps("SELECT CAST(id AS CHAR) as id, type, is_builtin, locked FROM connectors WHERE id = ? AND tenant_id = ?", connectorID, tenantID)
		if len(existing) == 0 {
			return `{"error": "连接器不存在或不属于当前租户"}`
		}
		if truthy(existing[0]["is_builtin"]) || truthy(existing[0]["locked"]) || fmt.Sprint(existing[0]["type"]) == "builtin" {
			return `{"error": "内置锁定连接器不可编辑、停用或删除"}`
		}
		sets := []string{}
		args_sql := []any{}
		if name, ok := args["name"].(string); ok && name != "" {
			sets = append(sets, "name = ?")
			args_sql = append(args_sql, name)
		}
		if baseURL, ok := args["base_url"].(string); ok && baseURL != "" {
			sets = append(sets, "base_url = ?")
			args_sql = append(args_sql, baseURL)
		}
		if status, ok := args["status"].(string); ok && status != "" {
			sets = append(sets, "status = ?")
			args_sql = append(args_sql, status)
		}
		if len(sets) == 0 {
			return `{"error": "没有需要更新的字段"}`
		}
		sets = append(sets, "updated_at = NOW()")
		query := "UPDATE connectors SET " + strings.Join(sets, ", ") + " WHERE id = ?"
		args_sql = append(args_sql, connectorID)
		if _, err := database.DB.Exec(query, args_sql...); err != nil {
			return `{"error": "更新连接器失败: ` + err.Error() + `"}`
		}
		logAudit(ctx, tenantID, userID, "update_connector", "update", "connector", fmt.Sprintf("更新连接器 %s: %v", connectorID, args))
		return `{"success": true, "message": "连接器更新成功"}`

	// ========== MCP工具 ==========
	case "get_mcp_tool":
		toolID, _ := args["tool_id"].(string)
		name, _ := args["name"].(string)
		if toolID == "" && name == "" {
			return `{"error": "tool_id 或 name 必填其中一项"}`
		}
		var tool []map[string]any
		var err error
		if toolID != "" {
			tool, err = queryMaps("SELECT CAST(id AS CHAR) as id, CAST(connector_id AS CHAR) as connector_id, CAST(name AS CHAR) as name, CAST(description AS CHAR) as description, CAST(input_schema AS CHAR) as input_schema, CAST(backend_method AS CHAR) as backend_method, CAST(backend_path AS CHAR) as backend_path, CAST(risk_level AS CHAR) as risk_level, enabled, created_at FROM mcp_tools WHERE id = ? AND tenant_id = ?", toolID, tenantID)
		} else {
			tool, err = queryMaps("SELECT CAST(id AS CHAR) as id, CAST(connector_id AS CHAR) as connector_id, CAST(name AS CHAR) as name, CAST(description AS CHAR) as description, CAST(input_schema AS CHAR) as input_schema, CAST(backend_method AS CHAR) as backend_method, CAST(backend_path AS CHAR) as backend_path, CAST(risk_level AS CHAR) as risk_level, enabled, created_at FROM mcp_tools WHERE name = ? AND tenant_id = ?", name, tenantID)
		}
		if err != nil || len(tool) == 0 {
			return `{"error": "MCP工具不存在"}`
		}
		data, _ := json.Marshal(tool[0])
		return string(data)

	case "create_mcp_tool":
		connectorID, _ := args["connector_id"].(string)
		name, _ := args["name"].(string)
		method, _ := args["backend_method"].(string)
		path, _ := args["backend_path"].(string)
		if connectorID == "" || name == "" || method == "" || path == "" {
			return `{"error": "connector_id, name, backend_method, backend_path 为必填项"}`
		}
		// 验证连接器属于当前租户
		conn, _ := queryMaps("SELECT id FROM connectors WHERE id = ? AND tenant_id = ?", connectorID, tenantID)
		if len(conn) == 0 {
			return `{"error": "连接器不存在或不属于当前租户"}`
		}
		desc, _ := args["description"].(string)
		risk, _ := args["risk_level"].(string)
		if risk == "" {
			risk = "medium"
		}
		inputSchema, _ := args["input_schema"].(string)
		id := uuid.New().String()
		now := time.Now()
		_, err := database.DB.Exec(`INSERT INTO mcp_tools (id, tenant_id, connector_id, name, description, input_schema, backend_method, backend_path, risk_level, enabled, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, true, ?)`,
			id, tenantID, connectorID, name, desc, inputSchema, method, path, risk, now)
		if err != nil {
			return `{"error": "创建MCP工具失败: ` + err.Error() + `"}`
		}
		logAudit(ctx, tenantID, userID, "create_mcp_tool", "create", "mcp_tool", fmt.Sprintf("创建MCP工具: %s %s %s", name, method, path))
		return `{"success": true, "id": "` + id + `", "message": "MCP工具 ` + name + ` 创建成功"}`

	case "update_mcp_tool":
		toolID, _ := args["tool_id"].(string)
		if toolID == "" {
			return `{"error": "tool_id 为必填项"}`
		}
		existing, _ := queryMaps("SELECT CAST(id AS CHAR) as id, is_builtin, locked FROM mcp_tools WHERE id = ? AND tenant_id = ?", toolID, tenantID)
		if len(existing) == 0 {
			return `{"error": "MCP工具不存在或不属于当前租户"}`
		}
		if truthy(existing[0]["is_builtin"]) || truthy(existing[0]["locked"]) {
			return `{"error": "内置锁定 MCP 工具不可编辑、停用或删除"}`
		}
		sets := []string{}
		args_sql := []any{}
		if name, ok := args["name"].(string); ok && name != "" {
			sets = append(sets, "name = ?")
			args_sql = append(args_sql, name)
		}
		if desc, ok := args["description"].(string); ok {
			sets = append(sets, "description = ?")
			args_sql = append(args_sql, desc)
		}
		if inputSchema, ok := args["input_schema"].(string); ok {
			sets = append(sets, "input_schema = ?")
			args_sql = append(args_sql, inputSchema)
		}
		if method, ok := args["backend_method"].(string); ok && method != "" {
			sets = append(sets, "backend_method = ?")
			args_sql = append(args_sql, method)
		}
		if path, ok := args["backend_path"].(string); ok && path != "" {
			sets = append(sets, "backend_path = ?")
			args_sql = append(args_sql, path)
		}
		if risk, ok := args["risk_level"].(string); ok && risk != "" {
			sets = append(sets, "risk_level = ?")
			args_sql = append(args_sql, risk)
		}
		if enabled, ok := args["enabled"].(bool); ok {
			sets = append(sets, "enabled = ?")
			args_sql = append(args_sql, enabled)
		}
		if len(sets) == 0 {
			return `{"error": "没有需要更新的字段"}`
		}
		query := "UPDATE mcp_tools SET " + strings.Join(sets, ", ") + " WHERE id = ?"
		args_sql = append(args_sql, toolID)
		if _, err := database.DB.Exec(query, args_sql...); err != nil {
			return `{"error": "更新MCP工具失败: ` + err.Error() + `"}`
		}
		logAudit(ctx, tenantID, userID, "update_mcp_tool", "update", "mcp_tool", fmt.Sprintf("更新MCP工具 %s: %v", toolID, args))
		return `{"success": true, "message": "MCP工具更新成功"}`

	// ========== 技能工具 ==========
	case "list_skills":
		// 根据角色权限过滤技能
		_, allowedSkillIDs, skillWildcard := getUserAllowedMCPSkills(userID)
		var skills []map[string]any
		var err error
		if skillWildcard {
			skills, err = queryMaps("SELECT CAST(id AS CHAR) as id, CAST(name AS CHAR) as name, CAST(description AS CHAR) as description, CAST(version AS CHAR) as version, CAST(status AS CHAR) as status, created_at FROM skills WHERE tenant_id = ?", tenantID)
		} else if len(allowedSkillIDs) > 0 {
			placeholders := ""
			args := []any{}
			for id := range allowedSkillIDs {
				if placeholders != "" {
					placeholders += ","
				}
				placeholders += "?"
				args = append(args, id)
			}
			args = append(args, tenantID)
			skills, err = queryMaps("SELECT CAST(id AS CHAR) as id, CAST(name AS CHAR) as name, CAST(description AS CHAR) as description, CAST(version AS CHAR) as version, CAST(status AS CHAR) as status, created_at FROM skills WHERE id IN ("+placeholders+") AND tenant_id = ?", args...)
		} else {
			skills = []map[string]any{}
		}
		if err != nil {
			return `{"error": "查询技能失败: ` + err.Error() + `"}`
		}
		if skills == nil {
			skills = []map[string]any{}
		}
		data, _ := json.Marshal(skills)
		return string(data)

	case "get_skill":
		skillID, _ := args["skill_id"].(string)
		name, _ := args["name"].(string)
		if skillID == "" && name == "" {
			return `{"error": "skill_id 或 name 必填其中一项"}`
		}
		var skill []map[string]any
		var err error
		if skillID != "" {
			skill, err = queryMaps("SELECT CAST(id AS CHAR) as id, CAST(tenant_id AS CHAR) as tenant_id, CAST(name AS CHAR) as name, CAST(description AS CHAR) as description, CAST(version AS CHAR) as version, CAST(triggers AS CHAR) as triggers, CAST(steps AS CHAR) as steps, CAST(permission_topology AS CHAR) as permission_topology, CAST(status AS CHAR) as status, CAST(created_by AS CHAR) as created_by, created_at, updated_at FROM skills WHERE id = ? AND tenant_id = ?", skillID, tenantID)
		} else {
			skill, err = queryMaps("SELECT CAST(id AS CHAR) as id, CAST(tenant_id AS CHAR) as tenant_id, CAST(name AS CHAR) as name, CAST(description AS CHAR) as description, CAST(version AS CHAR) as version, CAST(triggers AS CHAR) as triggers, CAST(steps AS CHAR) as steps, CAST(permission_topology AS CHAR) as permission_topology, CAST(status AS CHAR) as status, CAST(created_by AS CHAR) as created_by, created_at, updated_at FROM skills WHERE name = ? AND tenant_id = ?", name, tenantID)
		}
		if err != nil || len(skill) == 0 {
			return `{"error": "技能不存在"}`
		}
		data, _ := json.Marshal(skill[0])
		return string(data)

	case "create_skill":
		name, _ := args["name"].(string)
		steps, _ := args["steps"].(string)
		if name == "" || steps == "" {
			return `{"error": "name 和 steps 为必填项"}`
		}
		desc, _ := args["description"].(string)
		triggers, _ := args["triggers"].(string)
		id := uuid.New().String()
		now := time.Now()
		_, err := database.DB.Exec(`INSERT INTO skills (id, tenant_id, name, description, version, triggers, steps, status, created_at, updated_at) VALUES (?, ?, ?, ?, '1.0', ?, ?, 'draft', ?, ?)`,
			id, tenantID, name, desc, triggers, steps, now, now)
		if err != nil {
			return `{"error": "创建技能失败: ` + err.Error() + `"}`
		}
		logAudit(ctx, tenantID, userID, "create_skill", "create", "skill", fmt.Sprintf("创建技能: %s", name))
		return `{"success": true, "id": "` + id + `", "message": "技能 ` + name + ` 创建成功"}`

	case "update_skill":
		skillID, _ := args["skill_id"].(string)
		if skillID == "" {
			return `{"error": "skill_id 为必填项"}`
		}
		existing, _ := queryMaps("SELECT id, CAST(created_by AS CHAR) as created_by FROM skills WHERE id = ? AND tenant_id = ?", skillID, tenantID)
		if len(existing) == 0 {
			return `{"error": "技能不存在或不属于当前租户"}`
		}
		if createdBy, _ := existing[0]["created_by"].(string); createdBy == "system" {
			return `{"error": "内置技能不可编辑"}`
		}
		sets := []string{}
		args_sql := []any{}
		if name, ok := args["name"].(string); ok && name != "" {
			sets = append(sets, "name = ?")
			args_sql = append(args_sql, name)
		}
		if desc, ok := args["description"].(string); ok {
			sets = append(sets, "description = ?")
			args_sql = append(args_sql, desc)
		}
		if steps, ok := args["steps"].(string); ok && steps != "" {
			sets = append(sets, "steps = ?")
			args_sql = append(args_sql, steps)
		}
		if status, ok := args["status"].(string); ok && status != "" {
			sets = append(sets, "status = ?")
			args_sql = append(args_sql, status)
		}
		if len(sets) == 0 {
			return `{"error": "没有需要更新的字段"}`
		}
		sets = append(sets, "updated_at = NOW()")
		query := "UPDATE skills SET " + strings.Join(sets, ", ") + " WHERE id = ?"
		args_sql = append(args_sql, skillID)
		if _, err := database.DB.Exec(query, args_sql...); err != nil {
			return `{"error": "更新技能失败: ` + err.Error() + `"}`
		}
		logAudit(ctx, tenantID, userID, "update_skill", "update", "skill", fmt.Sprintf("更新技能 %s: %v", skillID, args))
		return `{"success": true, "message": "技能更新成功"}`

	case "execute_skill":
		skillID, _ := args["skill_id"].(string)
		if skillID == "" {
			return `{"error": "skill_id 为必填项"}`
		}
		// 验证技能属于当前租户
		var skill models.Skill
		err := database.DB.Get(&skill, "SELECT * FROM skills WHERE id = ? AND tenant_id = ?", skillID, tenantID)
		if err != nil {
			return `{"error": "技能不存在或不属于当前租户"}`
		}
		executionMode := executionModeFromArgs(args)
		if userID != "" {
			permissions, _ := middleware.GetUserPermissions(userID)
			if _, _, hasWildcard := getUserAllowedMCPSkills(userID); !hasWildcard {
				if missingPermissions, err := missingPermissionsForSkillSteps(skill.Steps, permissions); err == nil && len(missingPermissions) > 0 {
					data, _ := json.Marshal(map[string]any{"error": skillMissingPermissionMessage(skill.Name, missingPermissions), "permission_denied": true})
					return string(data)
				}
			}
		}
		// 获取输入参数
		inputs, _ := args["inputs"].(map[string]any)
		if inputs == nil {
			inputs = make(map[string]any)
		}
		// 创建MCP caller函数
		mcpCaller := func(ctx context.Context, toolName string, arguments json.RawMessage) (map[string]interface{}, error) {
			var builtinArgs map[string]any
			_ = json.Unmarshal(arguments, &builtinArgs)
			if isBuiltinSkillTool(toolName) {
				result := h.executeTool(tenantID, userID, toolName, builtinArgs)
				status, statusErr := toolCallStatusFromResult(result)
				if statusErr != nil || status == "failed" {
					return nil, statusErr
				}
				var resultMap map[string]interface{}
				if err := json.Unmarshal([]byte(result), &resultMap); err != nil {
					return map[string]interface{}{"result": result}, nil
				}
				if denied, _ := resultMap["permission_denied"].(bool); denied {
					message, _ := resultMap["error"].(string)
					return nil, fmt.Errorf("%s", message)
				}
				return resultMap, nil
			}
			// 查找MCP工具
			var mcpTool models.MCPTool
			if err := database.DB.Get(&mcpTool, "SELECT * FROM mcp_tools WHERE name = ? AND tenant_id = ? AND enabled = true", toolName, tenantID); err != nil {
				return nil, fmt.Errorf("MCP tool not found: %s", toolName)
			}
			if err := skillPkg.CanExecuteMCPTool(mcpTool, executionMode); err != nil {
				return nil, err
			}
			// 获取连接器
			var connector models.Connector
			if err := database.DB.Get(&connector, "SELECT * FROM connectors WHERE id = ?", mcpTool.ConnectorID); err != nil {
				return nil, fmt.Errorf("connector not found")
			}
			// 通过MCP Proxy调用
			mcpHandler := NewMCPHandler()
			ctxWithToken := contextWithUserSSOToken(ctx, tenantID, userID)
			resp, callErr := mcpHandler.proxy.CallTool(ctxWithToken, easpMCP.ToolCallRequest{
				Tool:      mcpTool,
				Connector: connector,
				Arguments: arguments,
			})
			if callErr != nil {
				return nil, callErr
			}
			if !resp.Success {
				return nil, fmt.Errorf("MCP tool error: %s", resp.Error)
			}
			resultMap, ok := resp.Data.(map[string]interface{})
			if !ok {
				resultMap = map[string]interface{}{"result": resp.Data}
			}
			return resultMap, nil
		}
		// 调用SkillEngine执行
		engine := skillPkg.NewSkillEngineWithCaller(tenantID, mcpCaller)
		execution, execErr := engine.ExecuteWithMode(context.Background(), skill, inputs, executionMode)
		if execErr != nil {
			return `{"error": "技能执行失败: ` + execErr.Error() + `"}`
		}
		// 更新使用次数
		database.DB.Exec("UPDATE skills SET usage_count = usage_count + 1, last_used_at = NOW() WHERE id = ?", skillID)
		logAudit(ctx, tenantID, userID, "execute_skill", "execute", "skill", fmt.Sprintf("执行技能 %s", skill.Name))
		outputsJSON, _ := json.Marshal(execution.Outputs)
		return `{"success": true, "skill_name": "` + skill.Name + `", "execution_id": "` + execution.ID + `", "status": "` + execution.Status + `", "outputs": ` + string(outputsJSON) + `}`

	case "execute_mcp_tool":
		toolID, _ := args["tool_id"].(string)
		name, _ := args["name"].(string)
		if toolID == "" && name == "" {
			return `{"error": "tool_id 或 name 必填其中一项"}`
		}
		// 验证MCP工具属于当前租户且启用，并按执行模式做生命周期收口。
		var mcpTool models.MCPTool
		var err error
		if toolID != "" {
			err = database.DB.Get(&mcpTool, "SELECT * FROM mcp_tools WHERE id = ? AND tenant_id = ?", toolID, tenantID)
		} else {
			err = database.DB.Get(&mcpTool, "SELECT * FROM mcp_tools WHERE name = ? AND tenant_id = ?", name, tenantID)
		}
		if err != nil {
			return `{"error": "MCP工具不存在或不属于当前租户"}`
		}
		if !mcpTool.Enabled {
			return `{"error": "MCP工具已禁用"}`
		}
		executionMode := executionModeFromArgs(args)
		if err := skillPkg.CanExecuteMCPTool(mcpTool, executionMode); err != nil {
			return `{"error": "` + err.Error() + `", "execution_mode": "` + executionMode + `"}`
		}
		arguments, _ := args["arguments"].(map[string]any)
		argumentsJSON, _ := json.Marshal(arguments)
		logAudit(ctx, tenantID, userID, "execute_mcp_tool", "execute", "mcp_tool", fmt.Sprintf("调用MCP工具 %s", mcpTool.Name))
		if skillPkg.ShouldSkipSideEffects(executionMode) {
			return `{"success": true, "tool_name": "` + mcpTool.Name + `", "execution_mode": "` + executionMode + `", "dry_run": true, "message": "沙箱/预演模式未执行MCP外部调用", "arguments": ` + string(argumentsJSON) + `}`
		}
		// 简化实现：当前AI内置 execute_mcp_tool 只回显调用计划；动态 mcp_* 路由负责真实MCP代理调用。
		return `{"success": true, "tool_name": "` + mcpTool.Name + `", "execution_mode": "` + executionMode + `", "arguments": ` + string(argumentsJSON) + `}`

	// ========== 记忆工具 ==========
	case "list_memory_pools":
		pools, err := queryMaps(`
			SELECT CAST(id AS CHAR) as id,
			       CAST(type AS CHAR) as type,
			       CAST(purpose AS CHAR) as purpose,
			       CAST(owner_id AS CHAR) as owner_id,
			       CAST(name AS CHAR) as name,
			       CAST(description AS CHAR) as description,
			       priority, max_tokens, auto_activate, enabled, memory_count, created_at, updated_at
			FROM memory_pools
			WHERE tenant_id = ?
			ORDER BY priority DESC, created_at DESC`, tenantID)
		if err != nil {
			logger.Warn("chat", "list memory pools failed",
				logger.Field("tenant_id", tenantID),
				logger.Field("error", err.Error()),
			)
			return `{"error": "查询记忆池失败，请稍后重试"}`
		}
		data, _ := json.Marshal(pools)
		return string(data)

	case "get_memory_entries":
		poolID, _ := args["pool_id"].(string)
		if poolID == "" {
			return `{"error": "pool_id 为必填项"}`
		}
		// 验证记忆池属于当前租户
		pool, _ := queryMaps("SELECT id FROM memory_pools WHERE id = ? AND tenant_id = ?", poolID, tenantID)
		if len(pool) == 0 {
			return `{"error": "记忆池不存在或不属于当前租户"}`
		}
		limit := 20
		if l, ok := args["limit"].(float64); ok && l > 0 {
			limit = int(l)
		}
		entries, err := queryMaps("SELECT CAST(id AS CHAR) as id, CAST(type AS CHAR) as type, CAST(content AS CHAR) as content, CAST(metadata AS CHAR) as metadata, CAST(sensitivity AS CHAR) as sensitivity, created_at, updated_at FROM memory_entries WHERE pool_id = ? ORDER BY created_at DESC LIMIT ?", poolID, limit)
		if err != nil {
			return `{"error": "查询记忆条目失败: ` + err.Error() + `"}`
		}
		data, _ := json.Marshal(entries)
		return string(data)

	case "create_memory_pool":
		name, _ := args["name"].(string)
		if name == "" {
			return `{"error": "name 为必填项"}`
		}
		poolType, _ := args["type"].(string)
		if poolType == "" {
			// 兼容旧工具参数 level：tenant/role 映射为 team，user 映射为 personal。
			if legacyLevel, _ := args["level"].(string); legacyLevel != "" {
				switch legacyLevel {
				case "tenant", "role":
					poolType = "team"
				case "user":
					poolType = "personal"
				default:
					poolType = "personal"
				}
			} else {
				poolType = "personal"
			}
		}
		purpose, _ := args["purpose"].(string)
		if purpose == "" {
			purpose = "conversation"
		}
		description, _ := args["description"].(string)
		ownerID, _ := args["owner_id"].(string)
		id := uuid.New().String()
		now := time.Now()
		_, err := database.DB.Exec(`
			INSERT INTO memory_pools
			(id, tenant_id, name, description, type, purpose, priority, max_tokens, auto_activate, owner_id, enabled, memory_count, created_at, updated_at)
			VALUES (?, ?, ?, NULLIF(?, ''), ?, ?, 5, 0, 1, NULLIF(?, ''), 1, 0, ?, ?)`,
			id, tenantID, name, description, poolType, purpose, ownerID, now, now)
		if err != nil {
			logger.Warn("chat", "create memory pool failed",
				logger.Field("tenant_id", tenantID),
				logger.Field("name", name),
				logger.Field("error", err.Error()),
			)
			return `{"error": "创建记忆池失败，请稍后重试"}`
		}
		logAudit(ctx, tenantID, userID, "create_memory_pool", "create", "memory_pool", fmt.Sprintf("创建记忆池: %s (type=%s purpose=%s)", name, poolType, purpose))
		return `{"success": true, "id": "` + id + `", "message": "记忆池 ` + name + ` 创建成功"}`

	case "create_memory_entry":
		poolID, _ := args["pool_id"].(string)
		entryType, _ := args["type"].(string)
		content, _ := args["content"].(string)
		if entryType == "" {
			entryType = "fact"
		}
		if poolID == "" || content == "" {
			return `{"error": "pool_id 和 content 为必填项"}`
		}
		// 验证记忆池属于当前租户
		pool, _ := queryMaps("SELECT id FROM memory_pools WHERE id = ? AND tenant_id = ?", poolID, tenantID)
		if len(pool) == 0 {
			return `{"error": "记忆池不存在或不属于当前租户"}`
		}
		sensitivity, _ := args["sensitivity"].(string)
		if sensitivity == "" {
			sensitivity = "low"
		}
		entry := &models.MemoryEntry{
			PoolID:      poolID,
			Type:        entryType,
			Content:     content,
			Sensitivity: sensitivity,
		}
		if err := repositories.NewMemoryEntryRepository().Create(entry); err != nil {
			return `{"error": "创建记忆条目失败: ` + err.Error() + `"}`
		}
		logAudit(ctx, tenantID, userID, "create_memory_entry", "create", "memory_entry", fmt.Sprintf("创建记忆条目: pool=%s type=%s", poolID, entryType))
		return `{"success": true, "id": "` + entry.ID + `", "message": "记忆条目创建成功"}`

	case "update_memory_entry":
		entryID, _ := args["entry_id"].(string)
		if entryID == "" {
			return `{"error": "entry_id 为必填项"}`
		}
		// 验证条目属于当前租户（通过pool关联）
		entry, _ := queryMaps("SELECT me.id FROM memory_entries me JOIN memory_pools mp ON me.pool_id = mp.id WHERE me.id = ? AND mp.tenant_id = ?", entryID, tenantID)
		if len(entry) == 0 {
			return `{"error": "记忆条目不存在或不属于当前租户"}`
		}
		sets := []string{}
		args_sql := []any{}
		if content, ok := args["content"].(string); ok && content != "" {
			sets = append(sets, "content = ?")
			args_sql = append(args_sql, content)
		}
		if sensitivity, ok := args["sensitivity"].(string); ok && sensitivity != "" {
			sets = append(sets, "sensitivity = ?")
			args_sql = append(args_sql, sensitivity)
		}
		if len(sets) == 0 {
			return `{"error": "没有需要更新的字段"}`
		}
		sets = append(sets, "updated_at = NOW()")
		query := "UPDATE memory_entries SET " + strings.Join(sets, ", ") + " WHERE id = ?"
		args_sql = append(args_sql, entryID)
		if _, err := database.DB.Exec(query, args_sql...); err != nil {
			return `{"error": "更新记忆条目失败: ` + err.Error() + `"}`
		}
		logAudit(ctx, tenantID, userID, "update_memory_entry", "update", "memory_entry", fmt.Sprintf("更新记忆条目 %s: %v", entryID, args))
		return `{"success": true, "message": "记忆条目更新成功"}`

	default:
		// 检查是否是Skill调用（名称以 skill_ 开头）- 智能路由
		if strings.HasPrefix(toolName, "skill_") {
			skillKey := strings.TrimPrefix(toolName, "skill_")
			// loadSkillToolDefinitions 暴露给模型的工具名格式为 skill_{8位ID前缀}_{安全名称}。
			// 执行时按完整生成名反查，并二次校验用户权限，避免模型返回未暴露的函数名时越权执行。
			var sk models.Skill
			var err error
			isPrefixedSkillTool := false
			if idx := strings.Index(skillKey, "_"); idx == 8 {
				idPrefix := skillKey[:idx]
				isPrefixedSkillTool = skillIDPrefixRegex.MatchString(idPrefix)
				if isPrefixedSkillTool {
					var candidates []models.Skill
					err = database.DB.Select(&candidates, "SELECT * FROM skills WHERE REPLACE(id, '-', '') LIKE ? AND tenant_id = ? AND status IN ('published', 'active')", idPrefix+"%", tenantID)
					if err == nil {
						for _, candidate := range candidates {
							if makeSkillToolName(candidate) == toolName {
								sk = candidate
								break
							}
						}
						if sk.ID == "" {
							err = fmt.Errorf("skill tool name mismatch")
						}
					}
				}
			}
			if !isPrefixedSkillTool && (err != nil || sk.ID == "") {
				// 兼容旧格式：skill_{name}。前缀格式必须完整匹配生成名，不再回退到名称查找。
				err = database.DB.Get(&sk, "SELECT * FROM skills WHERE name = ? AND tenant_id = ? AND status IN ('published', 'active')", skillKey, tenantID)
			}
			if err != nil || sk.ID == "" {
				data, _ := json.Marshal(map[string]any{"error": "技能不存在或未激活: " + skillKey})
				return string(data)
			}
			if userID != "" {
				_, allowedSkillIDs, hasWildcard := getUserAllowedMCPSkills(userID)
				if !hasWildcard && !allowedSkillIDs[sk.ID] {
					data, _ := json.Marshal(map[string]any{"error": "无权执行技能: " + sk.Name})
					return string(data)
				}
				if !hasWildcard {
					permissions, _ := middleware.GetUserPermissions(userID)
					if missingPermissions, err := missingPermissionsForSkillSteps(sk.Steps, permissions); err == nil && len(missingPermissions) > 0 {
						data, _ := json.Marshal(map[string]any{"error": skillMissingPermissionMessage(sk.Name, missingPermissions), "permission_denied": true})
						return string(data)
					}
				}
			}
			executionMode := executionModeFromArgs(args)
			// 创建MCP caller
			mcpCaller := func(ctx context.Context, toolName string, arguments json.RawMessage) (map[string]interface{}, error) {
				var mcpTool models.MCPTool
				if err := database.DB.Get(&mcpTool, "SELECT * FROM mcp_tools WHERE name = ? AND tenant_id = ? AND enabled = true", toolName, tenantID); err != nil {
					return nil, fmt.Errorf("MCP tool not found: %s", toolName)
				}
				if err := skillPkg.CanExecuteMCPTool(mcpTool, executionMode); err != nil {
					return nil, err
				}
				var connector models.Connector
				if err := database.DB.Get(&connector, "SELECT * FROM connectors WHERE id = ?", mcpTool.ConnectorID); err != nil {
					return nil, fmt.Errorf("connector not found")
				}
				mcpHandler := NewMCPHandler()
				resp, callErr := mcpHandler.proxy.CallTool(ctx, easpMCP.ToolCallRequest{
					Tool:      mcpTool,
					Connector: connector,
					Arguments: arguments,
				})
				if callErr != nil {
					return nil, callErr
				}
				if !resp.Success {
					return nil, fmt.Errorf("MCP tool error: %s", resp.Error)
				}
				resultMap, ok := resp.Data.(map[string]interface{})
				if !ok {
					resultMap = map[string]interface{}{"result": resp.Data}
				}
				return resultMap, nil
			}
			// 执行Skill
			engine := skillPkg.NewSkillEngineWithCaller(tenantID, mcpCaller)
			execution, execErr := engine.ExecuteWithMode(context.Background(), sk, args, executionMode)
			if execErr != nil {
				return `{"error": "技能执行失败: ` + execErr.Error() + `"}`
			}
			// 更新使用次数
			database.DB.Exec("UPDATE skills SET usage_count = usage_count + 1, last_used_at = NOW() WHERE id = ?", sk.ID)
			logAudit(ctx, tenantID, userID, "skill_call", "execute", "skill", fmt.Sprintf("执行技能 %s", sk.Name))
			outputsJSON, _ := json.Marshal(execution.Outputs)
			return `{"success": true, "skill_name": "` + sk.Name + `", "execution_id": "` + execution.ID + `", "status": "` + execution.Status + `", "outputs": ` + string(outputsJSON) + `}`
		}
		// 检查是否是MCP工具调用（名称以 mcp_ 开头）
		if strings.HasPrefix(toolName, "mcp_") {
			mcpToolName := strings.TrimPrefix(toolName, "mcp_")
			executionMode := executionModeFromArgs(args)
			// 查找MCP工具
			var mcpTool models.MCPTool
			err := database.DB.Get(&mcpTool, "SELECT * FROM mcp_tools WHERE name = ? AND tenant_id = ? AND enabled = true", mcpToolName, tenantID)
			if err != nil {
				return `{"error": "MCP工具不存在或已禁用: ` + mcpToolName + `"}`
			}
			if err := skillPkg.CanExecuteMCPTool(mcpTool, executionMode); err != nil {
				return `{"error": "` + err.Error() + `", "execution_mode": "` + executionMode + `"}`
			}
			// 构造MCP调用参数
			argumentsJSON, _ := json.Marshal(args)
			if skillPkg.ShouldSkipSideEffects(executionMode) {
				return `{"success": true, "tool_name": "` + mcpTool.Name + `", "execution_mode": "` + executionMode + `", "dry_run": true, "message": "沙箱/预演模式未执行MCP外部调用", "arguments": ` + string(argumentsJSON) + `}`
			}
			// 获取连接器
			var connector models.Connector
			err = database.DB.Get(&connector, "SELECT * FROM connectors WHERE id = ?", mcpTool.ConnectorID)
			if err != nil {
				return `{"error": "连接器不存在"}`
			}
			// 调用MCP工具
			mcpHandler := NewMCPHandler()
			callCtx := context.Background()
			if ctx != nil && ctx.RequestContext != nil {
				callCtx = ctx.RequestContext
			}
			ctxWithToken := contextWithUserSSOToken(callCtx, tenantID, userID)
			result, callErr := mcpHandler.proxy.CallTool(ctxWithToken, easpMCP.ToolCallRequest{
				Tool:      mcpTool,
				Connector: connector,
				Arguments: json.RawMessage(argumentsJSON),
			})
			if callErr != nil {
				return `{"error": "MCP工具调用失败: ` + callErr.Error() + `"}`
			}
			logAudit(ctx, tenantID, userID, "mcp_tool_call", "execute", "mcp_tool", fmt.Sprintf("调用MCP工具 %s: %s", mcpToolName, string(argumentsJSON)))
			resultJSON, _ := json.Marshal(result)
			return string(resultJSON)
		}
		return `{"error": "未知工具: ` + toolName + `"}`
	}
}

// queryMaps 通用多列查询，返回 []map[string]any
func queryMaps(query string, args ...any) ([]map[string]any, error) {
	rows, err := database.DB.Queryx(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]any
	for rows.Next() {
		m := map[string]any{}
		if err := rows.MapScan(m); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	if result == nil {
		result = []map[string]any{}
	}
	return result, nil
}

// ChatStream SSE流式聊天
func (h *ChatHandler) ChatStream(c *gin.Context) {
	tenantID, exists := c.Get(middleware.ContextTenantID)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Tenant context not found"})
		return
	}
	userID, _ := c.Get(middleware.ContextUserID)

	var req AssistantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tid := tenantID.(string)
	uid := userID.(string)
	requestStart := time.Now()
	requestID := logger.GetRequestID(c)
	if requestID == "" {
		requestID = uuid.New().String()
	}
	conversationID := ensureAssistantConversation(tid, uid, &req)
	logger.Info("chat", "chat stream started",
		logger.Field("request_id", requestID),
		logger.Field("tenant_id", tid),
		logger.Field("user_id", uid),
		logger.Field("conversation_id", conversationID),
		logger.Field("messages", len(req.Messages)),
		logger.Field("client_ip", c.ClientIP()),
	)

	activity := newAssistantActivityTracker(120 * time.Second)

	// 设置SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	sendSSEActive(c, activity, SSEEventConversation, map[string]any{
		"conversation_id": conversationID,
		"request_id":      requestID,
	})

	// 根据用户权限动态过滤工具，且根据 execution_mode 决定是否启用工具
	enableTools := true
	if req.ExecutionMode == "sandbox" {
		enableTools = false
	}

var tools []ToolDefinition
	var permissions []string
	var permErr error
	var allowedMCPTools map[string]bool
	var allowedSkills map[string]bool
	var hasWildcard bool
	var toolNames []string
	var skillLoadResultVar skillToolLoadResult

	allowedMCPTools = make(map[string]bool)
	allowedSkills = make(map[string]bool)

	if enableTools {
		permissions, permErr = middleware.GetUserPermissions(userID.(string))
		if permErr != nil || len(permissions) == 0 {
			// 获取失败时给空工具列表（只能聊天，不能操作）
			tools = []ToolDefinition{}
			logger.Warn("chat", "permissions load failed; tools disabled",
				logger.Field("request_id", requestID),
				logger.Field("tenant_id", tid),
				logger.Field("user_id", userID),
				logger.Field("error", fmt.Sprintf("%v", permErr)),
			)
		} else {
			tools = getToolsForPermissions(permissions)
		}

		// 加载用户角色允许的MCP工具和技能
		allowedMCPTools, allowedSkills, hasWildcard = getUserAllowedMCPSkills(userID.(string))

		// 过滤 list_skills 和 list_mcp_tools 工具：如果角色没有绑定任何技能/MCP工具，则移除对应工具
		if !hasWildcard {
			filteredTools := make([]ToolDefinition, 0, len(tools))
			for _, tool := range tools {
				name := tool.Function.Name
				// list_skills / get_skill / execute_skill 需要技能权限
				if (name == "list_skills" || name == "get_skill" || name == "execute_skill") && len(allowedSkills) == 0 {
					continue
				}
				// list_mcp_tools / get_mcp_tool / execute_mcp_tool 需要MCP工具权限
				if (name == "list_mcp_tools" || name == "get_mcp_tool" || name == "execute_mcp_tool") && len(allowedMCPTools) == 0 {
					continue
				}
				filteredTools = append(filteredTools, tool)
			}
			tools = filteredTools
		}

		// 加载MCP工具并转为function calling工具定义
		mcpToolDefs := loadMCPToolDefinitions(tid, allowedMCPTools, hasWildcard)
		if len(mcpToolDefs) > 0 {
			tools = append(tools, mcpToolDefs...)
		}

		// 加载Skills并转为function calling工具定义（智能路由核心）
		skillLoadResultVar = loadSkillToolDefinitions(tid, allowedSkills, hasWildcard, permissions)
		skillToolDefs := skillLoadResultVar.Tools
		if len(skillToolDefs) > 0 {
			tools = append(tools, skillToolDefs...)
		}

		// 重新生成工具名称列表
		toolNames = make([]string, 0, len(tools))
		for _, tool := range tools {
			toolNames = append(toolNames, tool.Function.Name)
		}
		logger.Info("chat", "tools loaded",
			logger.Field("request_id", requestID),
			logger.Field("tenant_id", tid),
			logger.Field("user_id", userID),
			logger.Field("tools", len(tools)),
			logger.Field("mcp_tools", len(allowedMCPTools)),
			logger.Field("skills", len(allowedSkills)),
			logger.Field("has_wildcard", hasWildcard),
		)
	} else {
		// sandbox mode: 不加载任何工具，只能纯聊天
		tools = []ToolDefinition{}
		toolNames = []string{}
		permissions = []string{}
		allowedMCPTools = map[string]bool{}
		allowedSkills = map[string]bool{}
	}

	// 获取用户最新消息用于记忆检索
	// 获取用户最新消息用于记忆检索
	lastUserMsg := lastUserMessage(req.Messages)

		if intent, missingPermissions := assistantIntentMissingPermissions(lastUserMsg, permissions); len(missingPermissions) > 0 {
			reply := skillMissingPermissionMessage(intent, missingPermissions)
			sendAssistantBufferedDelta(c, activity, reply)
			saveConversationMessage(tid, uid, conversationID, "user", lastUserMsg)
			saveConversationMessage(tid, uid, conversationID, "assistant", reply)
			sendSSEActive(c, activity, SSEEventDone, map[string]any{
				"total_ms":          time.Since(requestStart).Milliseconds(),
				"conversation_id":   conversationID,
				"permission_denied": true,
			})
			return
		}

		// 记忆路由器：按需加载记忆（租户隔离 + 用户隔离 + 角色感知）
		memCtx := h.memoryRouter.LoadMemories(tid, uid, lastUserMsg, permissions)

		// 动态构建system prompt（注入记忆上下文 + 可用技能信息）
	unavailableCapabilities := unavailableCapabilityLinesForMissingPermissions(permissions)
	unavailableCapabilities = append(unavailableCapabilities, skillLoadResultVar.UnavailableCapabilities...)
		basePrompt := getSystemPrompt(tid, toolNames, unavailableCapabilities) + buildPageContextPrompt(req.PageContext)

		// 注入可用技能信息到system prompt
		if len(allowedSkills) > 0 || hasWildcard {
			skillIDs := make([]string, 0)
			if hasWildcard {
				// 管理员可以看到所有技能
				var allSkills []models.Skill
				database.DB.Select(&allSkills, "SELECT id, name, description, category, status FROM skills WHERE tenant_id = ? AND status IN ('published', 'active')", tid)
				for _, s := range allSkills {
					skillIDs = append(skillIDs, s.ID)
				}
				if len(allSkills) > 0 {
					skillInfo := "\n\n## 可用技能\n你可以通过 execute_skill 工具执行以下技能：\n"
					for i, s := range allSkills {
						if i >= 10 {
							skillInfo += fmt.Sprintf("- 其余 %d 个技能可通过 list_skills 查询。\n", len(allSkills)-i)
							break
						}
						desc := ""
						if s.Description != nil {
							desc = *s.Description
						}
						skillInfo += fmt.Sprintf("- %s (ID: %s): %s\n", s.Name, s.ID, desc)
					}
					skillInfo += "使用 execute_skill 时，传入 skill_id 和 inputs 参数。"
					basePrompt += skillInfo
				}
			} else {
				for id := range allowedSkills {
					skillIDs = append(skillIDs, id)
				}
				if len(skillIDs) > 0 {
					placeholders := make([]string, len(skillIDs))
					args := make([]any, len(skillIDs))
					for i, id := range skillIDs {
						placeholders[i] = "?"
						args[i] = id
					}
					var skills []models.Skill
					query := "SELECT id, name, description, category, status FROM skills WHERE id IN (" + strings.Join(placeholders, ",") + ") AND status IN ('published', 'active')"
					database.DB.Select(&skills, query, args...)
					if len(skills) > 0 {
						skillInfo := "\n\n## 可用技能\n你可以通过 execute_skill 工具执行以下技能：\n"
						for i, s := range skills {
							if i >= 10 {
								skillInfo += fmt.Sprintf("- 其余 %d 个技能可通过 list_skills 查询。\n", len(skills)-i)
								break
							}
							desc := ""
							if s.Description != nil {
								desc = *s.Description
							}
							skillInfo += fmt.Sprintf("- %s (ID: %s): %s\n", s.Name, s.ID, desc)
						}
						skillInfo += "使用 execute_skill 时，传入 skill_id 和 inputs 参数。"
						basePrompt += skillInfo
					}
				}
			}
		}

		systemPrompt := h.memoryRouter.BuildMemoryPrompt(basePrompt, memCtx)

		// 发送记忆加载状态
		if memCtx != nil && (len(memCtx.UserMemories) > 0 || len(memCtx.SkillMemories) > 0) {
			totalMem := len(memCtx.UserMemories) + len(memCtx.SkillMemories) + len(memCtx.Entities) + len(memCtx.RoleMemories)
			sendSSEActive(c, activity, SSEEventStatus, map[string]any{
				"message":    fmt.Sprintf("已加载 %d 条相关记忆", totalMem),
				"stage":      "memory",
				"elapsed_ms": time.Since(requestStart).Milliseconds(),
			})
		}

		modelConfig, configErr := h.modelService.GetConfigForTenant(tid, "")
		if configErr != nil {
			sendSSE(c, SSEEventError, map[string]string{"message": "未配置可用的模型，请在模型配置页面启用至少一个模型和供应商"})
			sendSSE(c, SSEEventDone, nil)
			return
		}
		modelName := modelConfig.Model
		displayName := modelConfig.DisplayName
		providerName := modelConfig.ProviderName
		if displayName == "" {
			displayName = modelName
		}
		sendSSE(c, SSEEventModelInfo, map[string]string{
			"model":        modelName,
			"display_name": displayName,
			"provider":     providerName,
		})

		// 构建消息：服务端会话历史为主，限制历史长度以降低模型理解时间。
		modelMessages := append(loadConversationMessages(tid, uid, conversationID, 10), req.Messages...)
		if len(modelMessages) == 0 {
			modelMessages = req.Messages
		}
		messages := []modelservice.Message{
			{Role: "system", Content: systemPrompt},
		}
		for _, m := range modelMessages {
			role := strings.TrimSpace(m.Role)
			if role != "user" && role != "assistant" && role != "system" {
				continue
			}
			content := strings.TrimSpace(m.Content)
			if content == "" {
				continue
			}
			messages = append(messages, modelservice.Message{Role: role, Content: content})
		}
		saveConversationMessage(tid, uid, conversationID, "user", lastUserMsg)

		// 多轮工具调用循环：按活动续期，避免固定总耗时误杀；仍限制轮数防止模型循环。
		for round := 0; round < 8; round++ {
			roundStart := time.Now()

			// 发送思考阶段状态
			sendSSEActive(c, activity, SSEEventStatus, map[string]any{
				"message":    "正在思考...",
				"stage":      "thinking",
				"round":      round + 1,
				"elapsed_ms": time.Since(requestStart).Milliseconds(),
				"total_ms":   time.Since(requestStart).Milliseconds(),
			})

			// 调用模型（非流式，因为需要解析tool_calls）；等待期间持续发送 heartbeat
			response, err := waitModelWithHeartbeat(c, requestStart, activity, "thinking", fmt.Sprintf("正在思考...（第 %d 轮）", round+1), func() (*ModelResponse, error) {
				uid := userID.(string)
				return h.callModelWithTools(tid, uid, messages, tools, true)
			})
			modelElapsed := time.Since(roundStart).Milliseconds()

			if err != nil {
				logger.Error("chat", "model call failed",
					logger.Field("request_id", requestID),
					logger.Field("tenant_id", tid),
					logger.Field("user_id", userID),
					logger.Field("round", round+1),
					logger.Field("duration_ms", modelElapsed),
					logger.Field("error", err.Error()),
				)
				sendSSE(c, SSEEventError, map[string]string{"message": "模型调用失败: " + err.Error()})
				sendSSE(c, SSEEventDone, nil)
				return
			}

			// 记录模型 token 消耗
			if response.InputTokens > 0 || response.OutputTokens > 0 {
				uid := ""
				if userID != nil {
					uid = userID.(string)
				}
				RecordModelUsageWithContext(tid, uid, response.Provider, response.Model,
					"/chat", response.InputTokens, response.OutputTokens, response.CachedTokens, int(modelElapsed),
					"ai_assistant", "AI助手", "assistant", "", requestID)
			}

			// 检查是否有工具调用
			if len(response.ToolCalls) > 0 {
				// 记录模型调用了哪些工具
				tcNames := make([]string, 0, len(response.ToolCalls))
				for _, tc := range response.ToolCalls {
					tcNames = append(tcNames, tc.Function.Name)
				}
				logger.Info("chat", "model requested tools",
					logger.Field("request_id", requestID),
					logger.Field("tenant_id", tid),
					logger.Field("user_id", userID),
					logger.Field("round", round+1),
					logger.Field("tool_count", len(response.ToolCalls)),
					logger.Field("tools", tcNames),
				)
				// 添加assistant消息（带tool_calls）
				assistantMsg := modelservice.Message{Role: "assistant", Content: response.Content}
				for _, tc := range response.ToolCalls {
					assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, modelservice.ToolCall{
						ID:   tc.ID,
						Type: tc.Type,
						Function: struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						}{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					})
				}
				messages = append(messages, assistantMsg)

				// 发送分析完成状态
				sendSSEActive(c, activity, SSEEventStatus, map[string]any{
					"message":    fmt.Sprintf("模型决定调用 %d 个工具", len(response.ToolCalls)),
					"stage":      "plan",
					"stage_ms":   modelElapsed,
					"elapsed_ms": time.Since(requestStart).Milliseconds(),
					"total_ms":   time.Since(requestStart).Milliseconds(),
				})

				// 执行每个工具调用
				for ti, tc := range response.ToolCalls {
					toolStart := time.Now()

					// 发送工具执行中状态
					sendSSEActive(c, activity, SSEEventStatus, map[string]any{
						"message":    "正在" + getToolDisplayName(tc.Function.Name) + "...",
						"stage":      "tool_calling",
						"tool_name":  tc.Function.Name,
						"tool_index": ti + 1,
						"tool_total": len(response.ToolCalls),
						"elapsed_ms": time.Since(requestStart).Milliseconds(),
						"total_ms":   time.Since(requestStart).Milliseconds(),
					})

					var args map[string]any
					json.Unmarshal([]byte(tc.Function.Arguments), &args)
					if deniedResult, denied := toolPermissionDeniedResult(tc.Function.Name, skillLoadResultVar.UnavailableByToolName); denied {
						sendSSEActive(c, activity, SSEEventTool, map[string]any{
							"name":       tc.Function.Name,
							"result":     deniedResult,
							"elapsed_ms": int64(0),
						})
						if reply, ok := permissionDeniedReplyFromToolResult(deniedResult); ok {
							sendAssistantBufferedDelta(c, activity, reply)
							saveConversationMessage(tid, uid, conversationID, "assistant", reply)
							sendSSEActive(c, activity, SSEEventDone, map[string]any{
								"total_ms":          time.Since(requestStart).Milliseconds(),
								"conversation_id":   conversationID,
								"permission_denied": true,
							})
							return
						}
					}
					toolMessage := "正在" + getToolDisplayName(tc.Function.Name) + "..."
					auditCtx := &auditContext{
						AgentID:        requestID,
						IP:             c.ClientIP(),
						UserAgent:      c.Request.UserAgent(),
						StartedAt:      toolStart,
						SourceType:     c.GetString(middleware.ContextSourceType),
						SourceAppID:    c.GetString(middleware.ContextSourceAppID),
						ExternalSystem: c.GetString(middleware.ContextExternalSystem),
						ExternalUserID: c.GetString(middleware.ContextExternalUserID),
						RequestContext: c.Request.Context(),
					}
					result, toolWaitErr := waitToolWithHeartbeat(c, requestStart, activity, "tool_calling", toolMessage, func() string {
						return h.executeTool(tid, userID.(string), tc.Function.Name, args, auditCtx)
					})
					if toolWaitErr != nil {
						logger.Warn("chat", "tool execution interrupted",
							logger.Field("request_id", requestID),
							logger.Field("tenant_id", tid),
							logger.Field("user_id", userID),
							logger.Field("tool", tc.Function.Name),
							logger.Field("error", toolWaitErr.Error()),
						)
						return
					}
					toolElapsed := time.Since(toolStart).Milliseconds()
					resourceType, resourceID, resourceName := classifyCalledTool(tc.Function.Name)
					status, resultErr := toolCallStatusFromResult(result)
					RecordToolCallUsage(tid, userID.(string), resourceType, resourceID, resourceName,
						"ai_assistant", status, int(toolElapsed), requestID, resultErr)

					// 发送工具结果事件
					sendSSEActive(c, activity, SSEEventTool, map[string]any{
						"name":       tc.Function.Name,
						"result":     result,
						"elapsed_ms": toolElapsed,
					})
				if reply, ok := requiresInputReplyFromToolResult(result, lastUserMsg); ok {
					sendAssistantBufferedDelta(c, activity, reply);
					saveConversationMessage(tid, uid, conversationID, "assistant", reply);
					sendSSEActive(c, activity, SSEEventDone, map[string]any{
						"total_ms":        time.Since(requestStart).Milliseconds(),
						"conversation_id": conversationID,
						"requires_input":  true,
					});
					return
				}
				if msg, ok := permissionDeniedReplyFromToolResult(result); ok {
					sendAssistantBufferedDelta(c, activity, msg);
					saveConversationMessage(tid, uid, conversationID, "assistant", msg);
					sendSSEActive(c, activity, SSEEventDone, map[string]any{
						"total_ms":        time.Since(requestStart).Milliseconds(),
						"conversation_id": conversationID,
					});
					return
				}
				// 工具执行失败，提示用户
				if status == "failed" && resultErr != nil {
					errMsg := fmt.Sprintf("工具调用失败：%s\n\n请检查参数是否正确，或联系该工具的开发者协助排查。", resultErr.Error());
					sendAssistantBufferedDelta(c, activity, errMsg);
					saveConversationMessage(tid, uid, conversationID, "assistant", errMsg);
					sendSSEActive(c, activity, SSEEventDone, map[string]any{
						"total_ms":        time.Since(requestStart).Milliseconds(),
						"conversation_id": conversationID,
					});
					return
				}

					messages = append(messages, modelservice.Message{
						Role:       "tool",
						Content:    result,
						ToolCallID: tc.ID,
						Name:       tc.Function.Name,
					})
				}
				continue
			}

			// 没有工具调用，流式输出最终响应
			sendSSEActive(c, activity, SSEEventStatus, map[string]any{
				"message":    "正在生成回答...",
				"stage":      "generating",
				"stage_ms":   modelElapsed,
				"elapsed_ms": time.Since(requestStart).Milliseconds(),
				"total_ms":   time.Since(requestStart).Milliseconds(),
			})

			// 流式调用模型
			streamStart := time.Now()
			assistantContent, err := h.streamFinalResponse(tid, messages, c, activity)
			if err != nil {
				log.Printf("Stream error: %v", err)
				// 降级到非流式
				assistantContent = response.Content
				sendAssistantBufferedDelta(c, activity, response.Content)
			}
			saveConversationMessage(tid, uid, conversationID, "assistant", assistantContent)

			// 发送完成事件（带总耗时）
			sendSSE(c, SSEEventDone, map[string]any{
				"total_ms":        time.Since(requestStart).Milliseconds(),
				"stream_ms":       time.Since(streamStart).Milliseconds(),
				"conversation_id": conversationID,
			})

			// 异步提取记忆（不阻塞响应）
			if h.memoryExtractor != nil {
				go h.extractMemoryFromConversation(tid, uid, req.Messages, assistantContent)
			}
			return
		}

		sendAssistantBufferedDelta(c, activity, "已达到连续工具调用步骤上限，请根据已返回结果拆分后继续。")
		saveConversationMessage(tid, uid, conversationID, "assistant", "已达到连续工具调用步骤上限，请根据已返回结果拆分后继续。")
		sendSSEActive(c, activity, SSEEventDone, map[string]any{
			"total_ms":        time.Since(requestStart).Milliseconds(),
			"conversation_id": conversationID,
		})

		// 超时场景也尝试提取记忆
		if h.memoryExtractor != nil {
		go h.extractMemoryFromConversation(tid, uid, req.Messages, "")
	}
}

// streamFinalResponse 流式输出最终响应
func (h *ChatHandler) streamFinalResponse(tenantID string, messages []modelservice.Message, c *gin.Context, activity *assistantActivityTracker) (string, error) {
	config, err := h.modelService.GetConfigForTenant(tenantID, "")
	if err != nil {
		return "", fmt.Errorf("未配置可用的模型: %w", err)
	}

	reqBody := map[string]any{
		"model":       config.Model,
		"messages":    messages,
		"temperature": config.Temperature,
		"max_tokens":  config.MaxTokens,
		"stream":      true,
	}

	body, _ := json.Marshal(reqBody)
	httpReq, _ := http.NewRequest("POST", config.BaseURL+"/chat/completions", strings.NewReader(string(body)))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+config.APIKey)

	client := &http.Client{Timeout: 0}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d)", resp.StatusCode)
	}

	var fullContent strings.Builder
	deltaBuffer := newAssistantDeltaBuffer(8, 32, 80*time.Millisecond)
	sendDeltaChunks := func(chunks []string) {
		for _, chunk := range chunks {
			if chunk == "" {
				continue
			}
			sendSSEActive(c, activity, SSEEventDelta, map[string]string{
				"content": chunk,
			})
		}
	}

	// 解析SSE流
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				sendDeltaChunks(deltaBuffer.push("", time.Now(), true))
				return fullContent.String(), nil
			}

			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}

			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				piece := chunk.Choices[0].Delta.Content
				fullContent.WriteString(piece)
				sendDeltaChunks(deltaBuffer.push(piece, time.Now(), false))
			}
		}
	}
	sendDeltaChunks(deltaBuffer.push("", time.Now(), true))

	return fullContent.String(), scanner.Err()
}

// ModelResponse 模型响应
type ModelResponse struct {
	Content      string     `json:"content"`
	ToolCalls    []ToolCall `json:"tool_calls"`
	InputTokens  int        `json:"input_tokens"`
	OutputTokens int        `json:"output_tokens"`
	CachedTokens int        `json:"cached_tokens"`
	Provider     string     `json:"provider"`
	Model        string     `json:"model"`
}

// callModelWithTools 调用模型（支持工具，非流式，带重试）
// callModelWithTools 调用模型（支持工具，非流式，带重试）
// 如果 enableTools = true，会加载用户允许的 MCP 工具并让模型调用；false 不调用工具（沙箱模式）
func (h *ChatHandler) callModelWithTools(tenantID string, userID string, messages []modelservice.Message, tools []ToolDefinition, enableTools bool) (*ModelResponse, error) {
	config, err := h.modelService.GetConfigForTenant(tenantID, "")
	if err != nil {
		return nil, fmt.Errorf("未配置可用的模型: %w", err)
	}

	// 获取用户允许的 MCP 工具
	var mcpToolDefs []ToolDefinition
	if enableTools {
		// 获取允许的工具ID列表
		allowedMCPToolIDs, allowedSkillIDs, _ := getUserAllowedMCPSkills(userID)

		// 加载 MCP 工具详情
		var mcpTools []models.MCPTool
		if len(allowedMCPToolIDs) > 0 {
			// 构造 IN 查询
			placeholders := ""
			args := []any{tenantID}
			for id := range allowedMCPToolIDs {
				if placeholders != "" {
					placeholders += ","
				}
				placeholders += "?"
				args = append(args, id)
			}
			database.DB.Select(&mcpTools, "SELECT * FROM mcp_tools WHERE tenant_id = ? AND id IN ("+placeholders+")", args...)
		}
		for _, tool := range mcpTools {
			if tool.Status != "published" || !tool.Enabled {
				continue
			}
			// 转换为 OpenAI 工具格式
			var inputSchema map[string]interface{}
			if tool.InputSchema != nil && *tool.InputSchema != "" {
				_ = json.Unmarshal([]byte(*tool.InputSchema), &inputSchema)
			}
			desc := ""
			if tool.Description != nil {
				desc = *tool.Description
			}
			mcpToolDefs = append(mcpToolDefs, ToolDefinition{
				Type: "function",
				Function: FunctionDef{
					Name:        tool.Name,
					Description: desc,
					Parameters:  inputSchema,
				},
			})
		}

		// 加载技能工具详情
		var skills []models.Skill
		if len(allowedSkillIDs) > 0 {
			placeholders := ""
			args := []any{tenantID}
			for id := range allowedSkillIDs {
				if placeholders != "" {
					placeholders += ","
				}
				placeholders += "?"
				args = append(args, id)
			}
			database.DB.Select(&skills, "SELECT * FROM skills WHERE tenant_id = ? AND id IN ("+placeholders+")", args...)
		}
		for _, skill := range skills {
			if skill.Status != "published" {
				continue
			}
			var inputSchema map[string]interface{}
			if skill.InputSchema != nil && *skill.InputSchema != "" {
				_ = json.Unmarshal([]byte(*skill.InputSchema), &inputSchema)
			}
			desc := ""
			if skill.Description != nil {
				desc = *skill.Description
			}
			mcpToolDefs = append(mcpToolDefs, ToolDefinition{
				Type: "function",
				Function: FunctionDef{
					Name:        skill.Name,
					Description: desc,
					Parameters:  inputSchema,
				},
			})
		}
	}

	// 合并外部传入的工具
	var allTools []ToolDefinition
	allTools = append(allTools, mcpToolDefs...)
	for _, t := range tools {
		allTools = append(allTools, ToolDefinition{
			Type: "function",
			Function: FunctionDef{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  t.Function.Parameters,
			},
		})
	}

	reqBody := map[string]any{
		"model":       config.Model,
		"messages":    messages,
		"temperature": config.Temperature,
		"max_tokens":  config.MaxTokens,
		"stream":      false,
	}
	if len(allTools) > 0 {
		reqBody["tools"] = allTools
	}

	body, _ := json.Marshal(reqBody)

	// 重试3次
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
			log.Printf("Retrying model call (attempt %d/3)", attempt+1)
		}

		httpReq, _ := http.NewRequest("POST", config.BaseURL+"/chat/completions", strings.NewReader(string(body)))
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+config.APIKey)

		client := &http.Client{Timeout: 90 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			lastErr = err
			log.Printf("Model call attempt %d failed: %v", attempt+1, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
			continue
		}

		var chatResp struct {
			Choices []struct {
				Message struct {
					Role      string     `json:"role"`
					Content   string     `json:"content"`
					ToolCalls []ToolCall `json:"tool_calls"`
				} `json:"message"`
			} `json:"choices"`
			Usage struct {
				PromptTokens        json.Number `json:"prompt_tokens"`
				CompletionTokens    json.Number `json:"completion_tokens"`
				TotalTokens         json.Number `json:"total_tokens"`
				PromptTokensDetails struct {
					CachedTokens json.Number `json:"cached_tokens"`
				} `json:"prompt_tokens_details"`
			} `json:"usage"`
		}

		// 先读取原始 JSON 用于调试
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
			lastErr = fmt.Errorf("read response failed: %w", err)
			continue
		}
		resp.Body.Close()

		// 清理可能导致 JSON 解析失败的 BOM / gzip 包装。
		if len(respBody) >= 3 && respBody[0] == 0xEF && respBody[1] == 0xBB && respBody[2] == 0xBF {
			log.Printf("BOM detected, stripping")
			respBody = respBody[3:]
		}

		// 检查并处理 gzip 压缩
		if resp.Header.Get("Content-Encoding") == "gzip" || strings.HasPrefix(resp.Header.Get("Content-Type"), "application/gzip") {
			reader, err := gzip.NewReader(strings.NewReader(string(respBody)))
			if err == nil {
				respBody, err = io.ReadAll(reader)
				if err != nil {
					log.Printf("Gzip decompress failed: %v", err)
				}
				reader.Close()
			} else {
				log.Printf("Gzip reader creation failed: %v", err)
			}
		}

		// 再次检查 BOM (gzip 解压后可能还有 BOM)
		if len(respBody) >= 3 && respBody[0] == 0xEF && respBody[1] == 0xBB && respBody[2] == 0xBF {
			log.Printf("BOM detected after gzip, stripping")
			respBody = respBody[3:]
		}

		// 尝试解析
		if err := json.Unmarshal(respBody, &chatResp); err != nil {
			log.Printf("JSON unmarshal failed: %v", err)
			if syntaxErr, ok := err.(*json.SyntaxError); ok {
				log.Printf("Syntax error at offset: %d", syntaxErr.Offset)
			}
			lastErr = err
			continue
		}

		if len(chatResp.Choices) == 0 {
			lastErr = fmt.Errorf("no response from model")
			continue
		}

		response := ModelResponse{
			Content:      chatResp.Choices[0].Message.Content,
			ToolCalls:    chatResp.Choices[0].Message.ToolCalls,
			InputTokens:  0,
			OutputTokens: 0,
			CachedTokens: 0,
			Provider:     config.ProviderName,
			Model:        config.Model,
		}
		if n, err := chatResp.Usage.PromptTokens.Int64(); err == nil {
			response.InputTokens = int(n)
		}
		if n, err := chatResp.Usage.CompletionTokens.Int64(); err == nil {
			response.OutputTokens = int(n)
		}
		if n, err := chatResp.Usage.PromptTokensDetails.CachedTokens.Int64(); err == nil {
			response.CachedTokens = int(n)
		}
		return &response, nil
	}

	return nil, fmt.Errorf("model call failed after 3 attempts: %w", lastErr)
}

// extractMemoryFromConversation 从对话中异步提取记忆
func (h *ChatHandler) extractMemoryFromConversation(tenantID, userID string, userMessages []AssistantMessage, assistantResponse string) {
	// 构建提取请求
	var extractMsgs []easpMemory.ExtractMessage
	for _, m := range userMessages {
		extractMsgs = append(extractMsgs, easpMemory.ExtractMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}
	if assistantResponse != "" {
		extractMsgs = append(extractMsgs, easpMemory.ExtractMessage{
			Role:    "assistant",
			Content: assistantResponse,
		})
	}

	// 至少需要一轮对话
	if len(extractMsgs) < 2 {
		return
	}

	// 获取模型配置用于LLM调用
	modelConfig, err := h.modelService.GetConfigForTenant(tenantID, "")
	if err != nil {
		log.Printf("MemoryExtractor: failed to get model config: %v", err)
		return
	}

	// 更新提取器的模型配置
	h.memoryExtractor = easpMemory.NewMemoryExtractor(
		h.memoryRouter.GetMemoryService(),
		easpMemory.ModelConfig{
			BaseURL: modelConfig.BaseURL,
			APIKey:  modelConfig.APIKey,
			Model:   modelConfig.Model,
		},
	)

	// 执行提取
	h.memoryExtractor.ExtractAndSave(easpMemory.ExtractRequest{
		TenantID: tenantID,
		UserID:   userID,
		Messages: extractMsgs,
	})
}
