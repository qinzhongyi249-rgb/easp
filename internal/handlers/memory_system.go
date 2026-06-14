package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/easp-platform/easp/internal/memory"
	"github.com/easp-platform/easp/internal/middleware"
	"github.com/gin-gonic/gin"
)

// MemorySystemHandler 记忆处理器
type MemorySystemHandler struct {
	memorySvc *memory.MemoryService
}

// NewMemorySystemHandler 创建记忆处理器
func NewMemorySystemHandler() *MemorySystemHandler {
	// 创建Embedding服务 (暂时用mock，后续接入真实服务)
	embeddingSvc := &MockEmbeddingService{}

	memorySvc := memory.NewMemoryService(memory.MemoryConfig{
		EmbeddingService: embeddingSvc,
	})

	return &MemorySystemHandler{
		memorySvc: memorySvc,
	}
}

// MockEmbeddingService Mock Embedding服务
type MockEmbeddingService struct{}

func (s *MockEmbeddingService) GetEmbedding(text string) ([]float32, error) {
	// TODO: 接入真实的Embedding服务
	return nil, nil
}

func (s *MockEmbeddingService) GetEmbeddings(texts []string) ([][]float32, error) {
	// TODO: 接入真实的Embedding服务
	return nil, nil
}

// SaveUserMemory 保存用户记忆
func (h *MemorySystemHandler) SaveUserMemory(c *gin.Context) {
	tenantID := c.Param("tenantId")
	userID := c.Param("userId")

	var req struct {
		Type     string                 `json:"type" binding:"required"` // preference/fact/feedback
		Content  string                 `json:"content" binding:"required"`
		Metadata map[string]interface{} `json:"metadata,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	memory, err := h.memorySvc.SaveUserMemory(tenantID, userID, req.Type, req.Content, req.Metadata)
	if err != nil {
		log.Printf("Failed to save user memory: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save memory"})
		return
	}

	c.JSON(http.StatusCreated, memory)
}

// GetUserMemories 获取用户记忆
func (h *MemorySystemHandler) GetUserMemories(c *gin.Context) {
	tenantID := c.Param("tenantId")
	userID := c.Param("userId")
	memType := c.Query("type")
	limitStr := c.DefaultQuery("limit", "50")

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 50
	}

	memories, err := h.memorySvc.GetUserMemories(tenantID, userID, memType, limit)
	if err != nil {
		log.Printf("Failed to get user memories: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get memories"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"memories": memories})
}

// SearchUserMemories 搜索用户记忆
func (h *MemorySystemHandler) SearchUserMemories(c *gin.Context) {
	tenantID := c.Param("tenantId")
	userID := c.Param("userId")
	query := c.Query("q")
	limitStr := c.DefaultQuery("limit", "10")

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query is required"})
		return
	}

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 10
	}

	memories, explanations, err := h.memorySvc.SearchUserMemoriesWithExplanations(tenantID, userID, query, limit)
	if err != nil {
		log.Printf("Failed to search user memories: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search memories"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"memories": memories, "explanations": explanations})
}

// SaveSessionMemory 保存会话记忆
func (h *MemorySystemHandler) SaveSessionMemory(c *gin.Context) {
	tenantID := c.Param("tenantId")
	userID := c.Param("userId")

	var req struct {
		SessionID string `json:"session_id" binding:"required"`
		Role      string `json:"role" binding:"required"` // user/assistant/system
		Content   string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	memory, err := h.memorySvc.SaveSessionMemory(tenantID, userID, req.SessionID, req.Role, req.Content)
	if err != nil {
		log.Printf("Failed to save session memory: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save memory"})
		return
	}

	c.JSON(http.StatusCreated, memory)
}

// GetSessionMemories 获取会话记忆
func (h *MemorySystemHandler) GetSessionMemories(c *gin.Context) {
	sessionID := c.Param("sessionId")
	limitStr := c.DefaultQuery("limit", "50")

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 50
	}

	memories, err := h.memorySvc.GetSessionMemories(sessionID, limit)
	if err != nil {
		log.Printf("Failed to get session memories: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get memories"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"memories": memories})
}

// SaveEntity 保存实体
func (h *MemorySystemHandler) SaveEntity(c *gin.Context) {
	tenantID := c.Param("tenantId")

	var req struct {
		Name     string                 `json:"name" binding:"required"`
		Type     string                 `json:"type" binding:"required"` // tenant/user/connector/tool/skill
		RefID    string                 `json:"ref_id,omitempty"`
		Metadata map[string]interface{} `json:"metadata,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	entity, err := h.memorySvc.SaveEntity(tenantID, req.Name, req.Type, req.RefID, req.Metadata)
	if err != nil {
		log.Printf("Failed to save entity: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save entity"})
		return
	}

	c.JSON(http.StatusCreated, entity)
}

// SearchEntities 搜索实体
func (h *MemorySystemHandler) SearchEntities(c *gin.Context) {
	tenantID := c.Param("tenantId")
	query := c.Query("q")
	limitStr := c.DefaultQuery("limit", "10")

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query is required"})
		return
	}

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 10
	}

	entities, err := h.memorySvc.SearchEntities(tenantID, query, limit)
	if err != nil {
		log.Printf("Failed to search entities: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search entities"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"entities": entities})
}

// SaveEntityRelation 保存实体关系
func (h *MemorySystemHandler) SaveEntityRelation(c *gin.Context) {
	tenantID := c.Param("tenantId")

	var req struct {
		SourceEntityID string                 `json:"source_entity_id" binding:"required"`
		TargetEntityID string                 `json:"target_entity_id" binding:"required"`
		RelationType   string                 `json:"relation_type" binding:"required"` // belongs_to/uses/manages
		Metadata       map[string]interface{} `json:"metadata,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	relation, err := h.memorySvc.SaveEntityRelation(tenantID, req.SourceEntityID, req.TargetEntityID, req.RelationType, req.Metadata)
	if err != nil {
		log.Printf("Failed to save entity relation: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save relation"})
		return
	}

	c.JSON(http.StatusCreated, relation)
}

// SaveSkillMemory 保存技能记忆
func (h *MemorySystemHandler) SaveSkillMemory(c *gin.Context) {
	tenantID := c.Param("tenantId")
	userID := c.GetString("user_id") // 从JWT中获取

	var req struct {
		Name        string   `json:"name" binding:"required"`
		Description string   `json:"description,omitempty"`
		Content     string   `json:"content" binding:"required"`
		Category    string   `json:"category,omitempty"` // config/deploy/debug/faq
		Tags        []string `json:"tags,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	memory, err := h.memorySvc.SaveSkillMemory(tenantID, userID, req.Name, req.Description, req.Content, req.Category, req.Tags)
	if err != nil {
		log.Printf("Failed to save skill memory: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save skill memory"})
		return
	}

	c.JSON(http.StatusCreated, memory)
}

// SearchSkillMemories 搜索技能记忆
func (h *MemorySystemHandler) SearchSkillMemories(c *gin.Context) {
	tenantID := c.Param("tenantId")
	query := c.Query("q")
	limitStr := c.DefaultQuery("limit", "10")

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query is required"})
		return
	}

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 10
	}

	memories, err := h.memorySvc.SearchSkillMemories(tenantID, query, limit)
	if err != nil {
		log.Printf("Failed to search skill memories: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search skill memories"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"memories": memories})
}

// ListEntities 列出实体
func (h *MemorySystemHandler) ListEntities(c *gin.Context) {
	tenantID := c.Param("tenantId")
	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 50
	}

	entities, err := h.memorySvc.ListEntities(tenantID, limit)
	if err != nil {
		log.Printf("Failed to list entities: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list entities"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"entities": entities})
}

// ListSkillMemories 列出技能记忆
func (h *MemorySystemHandler) ListSkillMemories(c *gin.Context) {
	tenantID := c.Param("tenantId")
	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 50
	}

	memories, err := h.memorySvc.ListSkillMemories(tenantID, limit)
	if err != nil {
		log.Printf("Failed to list skill memories: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list skill memories"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"memories": memories})
}

// ListAllUserMemories 列出所有用户记忆
func (h *MemorySystemHandler) ListAllUserMemories(c *gin.Context) {
	tenantID := c.Param("tenantId")
	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 50
	}

	memories, err := h.memorySvc.ListAllUserMemories(tenantID, limit)
	if err != nil {
		log.Printf("Failed to list user memories: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list user memories"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"memories": memories})
}

// GetMemorySettings 获取当前用户记忆治理设置
func (h *MemorySystemHandler) GetMemorySettings(c *gin.Context) {
	tenantID := c.Param("tenantId")
	userID := c.Query("user_id")
	if userID == "" {
		if v, ok := c.Get(middleware.ContextUserID); ok {
			userID, _ = v.(string)
		}
	}
	c.JSON(http.StatusOK, h.memorySvc.GetMemorySettings(tenantID, userID))
}

// UpdateMemorySettings 更新当前用户或租户级记忆治理设置
func (h *MemorySystemHandler) UpdateMemorySettings(c *gin.Context) {
	tenantID := c.Param("tenantId")
	userID := c.Query("user_id")
	if userID == "" {
		if v, ok := c.Get(middleware.ContextUserID); ok {
			userID, _ = v.(string)
		}
	}
	var req struct {
		AutoExtractEnabled     *bool  `json:"auto_extract_enabled"`
		RecallEnabled          *bool  `json:"recall_enabled"`
		SensitiveFilterEnabled *bool  `json:"sensitive_filter_enabled"`
		AuditEnabled           *bool  `json:"audit_enabled"`
		HybridSearchEnabled    *bool  `json:"hybrid_search_enabled"`
		HybridSearchMode       string `json:"hybrid_search_mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	settings := h.memorySvc.GetMemorySettings(tenantID, userID)
	if req.AutoExtractEnabled != nil {
		settings.AutoExtractEnabled = *req.AutoExtractEnabled
	}
	if req.RecallEnabled != nil {
		settings.RecallEnabled = *req.RecallEnabled
	}
	if req.SensitiveFilterEnabled != nil {
		settings.SensitiveFilterEnabled = *req.SensitiveFilterEnabled
	}
	if req.AuditEnabled != nil {
		settings.AuditEnabled = *req.AuditEnabled
	}
	if req.HybridSearchEnabled != nil {
		settings.HybridSearchEnabled = *req.HybridSearchEnabled
	}
	if req.HybridSearchMode != "" {
		settings.HybridSearchMode = req.HybridSearchMode
	}
	if err := h.memorySvc.SaveMemorySettings(settings); err != nil {
		log.Printf("Failed to save memory settings: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save memory settings"})
		return
	}
	c.JSON(http.StatusOK, settings)
}

// ListMemoryAuditLogs 列出记忆治理审计日志
func (h *MemorySystemHandler) ListMemoryAuditLogs(c *gin.Context) {
	tenantID := c.Param("tenantId")
	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 50
	}
	logs, err := h.memorySvc.ListMemoryAuditLogs(tenantID, limit)
	if err != nil {
		log.Printf("Failed to list memory audit logs: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list memory audit logs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

// GetMemoryStats 获取记忆统计
func (h *MemorySystemHandler) GetMemoryStats(c *gin.Context) {
	tenantID := c.Param("tenantId")

	stats, err := h.memorySvc.GetMemoryStats(tenantID)
	if err != nil {
		log.Printf("Failed to get memory stats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}
