package memory

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/models"
	"github.com/google/uuid"
)

// MemoryService 记忆服务
type MemoryService struct {
	embeddingSvc EmbeddingService
	vectorSvc    *VectorMemoryService
}

// EmbeddingService Embedding服务接口
type EmbeddingService interface {
	GetEmbedding(text string) ([]float32, error)
	GetEmbeddings(texts []string) ([][]float32, error)
}

// MemoryConfig 记忆配置
type MemoryConfig struct {
	EmbeddingService EmbeddingService
	VectorService    *VectorMemoryService
}

// NewMemoryService 创建记忆服务
func NewMemoryService(config MemoryConfig) *MemoryService {
	vectorSvc := config.VectorService
	if vectorSvc == nil {
		// 默认接入已有向量桥接服务。失败不阻塞主链路，写入时按审计记录失败原因。
		vectorSvc = NewVectorMemoryService(VectorMemoryConfig{
			BridgeURL:  "http://localhost:8083",
			Database:   "easp_memory",
			Collection: "memories",
			Dimension:  1024,
		})
	}
	return &MemoryService{
		embeddingSvc: config.EmbeddingService,
		vectorSvc:    vectorSvc,
	}
}

// SaveUserMemory 保存用户记忆。
// 治理边界：敏感过滤/去重/审计只作用于持久化链路，不修改当前轮模型推理或工具调用参数。
func (s *MemoryService) SaveUserMemory(tenantID, userID, memType, content string, metadata map[string]interface{}) (*models.UserMemory, error) {
	content = strings.TrimSpace(content)
	memType = strings.TrimSpace(memType)
	if tenantID == "" || userID == "" || memType == "" || content == "" {
		return nil, fmt.Errorf("tenant_id, user_id, type and content are required")
	}
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	source := memorySource(metadata)
	settings := s.GetMemorySettings(tenantID, userID)

	originalContent := content
	findings := []SensitiveFinding{}
	blocked := false
	if settings.SensitiveFilterEnabled {
		content, findings, blocked = SanitizeForPersistence(content)
		if blocked {
			s.SaveMemoryAudit(MemoryAuditLog{
				TenantID:         tenantID,
				UserID:           userID,
				Action:           "blocked_sensitive",
				Source:           source,
				OriginalPreview:  originalContent,
				SanitizedPreview: content,
				Reason:           "sensitive content blocked before persistence",
				Metadata:         map[string]interface{}{"findings": findings, "type": memType},
			})
			return nil, fmt.Errorf("memory contains blocked sensitive content")
		}
		if len(findings) > 0 {
			metadata["sensitive_findings"] = findings
		}
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("memory content is empty after sanitization")
	}

	if merged, decision, err := s.tryMergeUserMemory(tenantID, userID, memType, content, source); err != nil {
		log.Printf("memory merge check failed: %v", err)
	} else if decision.Conflict {
		metadata["merge_decision"] = decision.Reason
		metadata["similarity"] = decision.Similarity
	} else if merged != nil {
		return merged, nil
	}

	contentHashValue := MemoryContentHash(content)
	embedding, err := s.embeddingSvc.GetEmbedding(content)
	if err != nil {
		log.Printf("Failed to get embedding: %v", err)
		// 不阻塞保存，embedding失败只影响检索
		embedding = nil
	}

	embeddingBytes, _ := json.Marshal(embedding)
	metadataBytes, _ := json.Marshal(metadata)
	now := time.Now()
	memory := &models.UserMemory{
		ID:          uuid.New().String(),
		TenantID:    tenantID,
		UserID:      userID,
		Type:        memType,
		Content:     content,
		ContentHash: &contentHashValue,
		Source:      source,
		Status:      "active",
		Embedding:   embeddingBytes,
		Metadata:    metadataBytes,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	result, err := database.DB.NamedExec(`
		INSERT INTO user_memories
		(id, tenant_id, user_id, type, content, content_hash, source, status, embedding, metadata, created_at, updated_at)
		VALUES (:id, :tenant_id, :user_id, :type, :content, :content_hash, :source, :status, :embedding, :metadata, :created_at, :updated_at)
		ON DUPLICATE KEY UPDATE
			access_count = access_count + 1,
			last_seen_at = NOW(),
			updated_at = NOW()
	`, memory)
	if err != nil {
		return nil, fmt.Errorf("failed to save user memory: %w", err)
	}

	action := "created"
	if affected, _ := result.RowsAffected(); affected > 1 {
		action = "deduplicated"
	}
	var saved models.UserMemory
	selectErr := database.DB.Get(&saved, userMemorySelectBase()+`
		WHERE tenant_id = ? AND user_id = ? AND type = ? AND content_hash = ? LIMIT 1`, tenantID, userID, memType, contentHashValue)
	if selectErr == nil {
		memory = &saved
	}
	if len(findings) > 0 && action == "created" {
		s.SaveMemoryAudit(MemoryAuditLog{
			TenantID:         tenantID,
			UserID:           userID,
			MemoryID:         &memory.ID,
			Action:           "redacted",
			Source:           source,
			OriginalPreview:  originalContent,
			SanitizedPreview: content,
			Reason:           "sensitive content redacted before persistence",
			Metadata:         map[string]interface{}{"findings": findings, "type": memType},
		})
	}
	s.SaveMemoryAudit(MemoryAuditLog{
		TenantID:         tenantID,
		UserID:           userID,
		MemoryID:         &memory.ID,
		Action:           action,
		Source:           source,
		OriginalPreview:  originalContent,
		SanitizedPreview: content,
		Reason:           "user memory persistence",
		Metadata: map[string]interface{}{
			"type":                 memType,
			"content_hash":         contentHashValue,
			"sensitive_findings":   findings,
			"hybrid_search_mode":   settings.HybridSearchMode,
			"hybrid_search_enable": settings.HybridSearchEnabled,
		},
	})

	if settings.HybridSearchEnabled && action == "created" {
		s.indexUserMemoryVector(*memory, source)
	}
	s.enforceUserMemoryCapacity(tenantID, userID, memType, source)

	return memory, nil
}

// ListAllUserMemories 列出租户下所有用户记忆
func (s *MemoryService) ListAllUserMemories(tenantID string, limit int) ([]models.UserMemory, error) {
	var memories []models.UserMemory
	err := database.DB.Select(&memories,
		userMemorySelectBase()+`WHERE tenant_id = ? AND status = 'active' ORDER BY created_at DESC LIMIT ?`, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list user memories: %w", err)
	}
	return memories, nil
}

// GetUserMemories 获取用户记忆
func (s *MemoryService) GetUserMemories(tenantID, userID string, memType string, limit int) ([]models.UserMemory, error) {
	var memories []models.UserMemory
	query := userMemorySelectBase() + "WHERE tenant_id = ? AND user_id = ? AND status = 'active'"
	args := []interface{}{tenantID, userID}

	if memType != "" {
		query += " AND type = ?"
		args = append(args, memType)
	}

	query += " ORDER BY updated_at DESC, created_at DESC LIMIT ?"
	args = append(args, limit*5)

	err := database.DB.Select(&memories, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get user memories: %w", err)
	}

	return RankUserMemories(memories, nil, limit, time.Now()), nil
}

// SearchUserMemories 搜索用户记忆
func (s *MemoryService) SearchUserMemories(tenantID, userID, query string, limit int) ([]models.UserMemory, error) {
	memories, _, err := s.SearchUserMemoriesWithExplanations(tenantID, userID, query, limit)
	return memories, err
}

// SearchUserMemoriesWithExplanations 搜索用户记忆并返回层面3召回解释。
func (s *MemoryService) SearchUserMemoriesWithExplanations(tenantID, userID, query string, limit int) ([]models.UserMemory, []MemoryScoreBreakdown, error) {
	settings := s.GetMemorySettings(tenantID, userID)
	// 1. 语义搜索准备：embedding 失败不影响关键词/向量桥接检索。
	if s.embeddingSvc != nil {
		_, err := s.embeddingSvc.GetEmbedding(query)
		if err != nil {
			log.Printf("Failed to get embedding: %v", err)
		}
	}

	// 2. 关键词搜索
	keywords := extractKeywords(query)
	log.Printf("SearchUserMemories: tenant=%s, user=%s, query=%s, keywords=%v", tenantID, userID, query, keywords)

	candidateLimit := limit * 5
	if candidateLimit < 20 {
		candidateLimit = 20
	}

	vectorScores := map[string]float64{}
	var vectorMemories []models.UserMemory
	if settings.HybridSearchEnabled && strings.Contains(settings.HybridSearchMode, "vector") && s.vectorSvc != nil && strings.TrimSpace(query) != "" {
		vectorResults, err := s.vectorSvc.SearchMemoriesWithScores(tenantID, "user_memories", query, candidateLimit)
		if err != nil {
			log.Printf("SearchUserMemories: vector search failed, fallback to keyword search: %v", err)
			s.SaveMemoryAudit(MemoryAuditLog{
				TenantID: tenantID,
				UserID:   userID,
				Action:   "vector_search_fallback",
				Source:   "recall",
				Reason:   err.Error(),
				Metadata: map[string]interface{}{"query": truncateRunes(query, 128), "mode": settings.HybridSearchMode},
			})
		} else if len(vectorResults) > 0 {
			ids := make([]string, 0, len(vectorResults))
			for _, result := range vectorResults {
				ids = append(ids, result.Memory.ID)
				vectorScores[result.Memory.ID] = normalizeVectorScore(result.Score)
			}
			loaded, err := getUserMemoriesByIDs(ids)
			if err != nil {
				log.Printf("SearchUserMemories: load vector memory candidates failed: %v", err)
			} else {
				for _, mem := range loaded {
					if mem.TenantID == tenantID && mem.UserID == userID {
						vectorMemories = append(vectorMemories, mem)
					}
				}
			}
		}
	}

	var memories []models.UserMemory
	sqlQuery := userMemorySelectBase() + `
		WHERE tenant_id = ? AND user_id = ? AND status = 'active'`
	args := []interface{}{tenantID, userID}

	// 关键词匹配
	if len(keywords) > 0 {
		sqlQuery += " AND ("
		for i, keyword := range keywords {
			if i > 0 {
				sqlQuery += " OR "
			}
			sqlQuery += "content LIKE ?"
			args = append(args, "%"+keyword+"%")
		}
		sqlQuery += ")"
	}

	sqlQuery += " ORDER BY updated_at DESC, created_at DESC LIMIT ?"
	args = append(args, candidateLimit)

	log.Printf("SearchUserMemories: SQL=%s, args=%v", sqlQuery, args)
	err := database.DB.Select(&memories, sqlQuery, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to search user memories: %w", err)
	}

	memories = appendMissingUserMemories(memories, vectorMemories)
	memories = RankUserMemoriesHybrid(memories, keywords, vectorScores, limit, time.Now())
	explanations := ExplainUserMemoryRanking(memories, keywords, vectorScores, limit, time.Now())
	log.Printf("SearchUserMemories: found %d memories, explanations=%v", len(memories), explanations)

	// 更新访问计数
	for _, mem := range memories {
		s.updateAccessCount(mem.ID)
	}

	return memories, explanations, nil
}

// SaveSessionMemory 保存会话记忆
func (s *MemoryService) SaveSessionMemory(tenantID, userID, sessionID, role, content string) (*models.SessionMemory, error) {
	embedding, err := s.embeddingSvc.GetEmbedding(content)
	if err != nil {
		log.Printf("Failed to get embedding: %v", err)
		embedding = nil
	}

	embeddingBytes, _ := json.Marshal(embedding)

	memory := &models.SessionMemory{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		UserID:    userID,
		SessionID: sessionID,
		Role:      role,
		Content:   content,
		Embedding: embeddingBytes,
		CreatedAt: time.Now(),
	}

	_, err = database.DB.NamedExec(`
		INSERT INTO session_memories (id, tenant_id, user_id, session_id, role, content, embedding, created_at)
		VALUES (:id, :tenant_id, :user_id, :session_id, :role, :content, :embedding, :created_at)
	`, memory)

	if err != nil {
		return nil, fmt.Errorf("failed to save session memory: %w", err)
	}

	return memory, nil
}

// GetSessionMemories 获取会话记忆
func (s *MemoryService) GetSessionMemories(sessionID string, limit int) ([]models.SessionMemory, error) {
	var memories []models.SessionMemory
	err := database.DB.Select(&memories, `
		SELECT id, tenant_id, user_id, session_id, role, content, token_count, entity_ids, created_at 
		FROM session_memories 
		WHERE session_id = ? 
		ORDER BY created_at DESC 
		LIMIT ?
	`, sessionID, limit)

	if err != nil {
		return nil, fmt.Errorf("failed to get session memories: %w", err)
	}

	return memories, nil
}

// SaveEntity 保存实体
func (s *MemoryService) SaveEntity(tenantID, name, entityType, refID string, metadata map[string]interface{}) (*models.Entity, error) {
	embedding, err := s.embeddingSvc.GetEmbedding(name)
	if err != nil {
		log.Printf("Failed to get embedding: %v", err)
		embedding = nil
	}

	embeddingBytes, _ := json.Marshal(embedding)
	metadataBytes, _ := json.Marshal(metadata)

	entity := &models.Entity{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		Name:      name,
		Type:      entityType,
		Embedding: embeddingBytes,
		Metadata:  metadataBytes,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if refID != "" {
		entity.RefID = &refID
	}

	_, err = database.DB.NamedExec(`
		INSERT INTO entities (id, tenant_id, name, type, ref_id, embedding, metadata, created_at, updated_at)
		VALUES (:id, :tenant_id, :name, :type, :ref_id, :embedding, :metadata, :created_at, :updated_at)
		ON DUPLICATE KEY UPDATE embedding = :embedding, metadata = :metadata, updated_at = :updated_at
	`, entity)

	if err != nil {
		return nil, fmt.Errorf("failed to save entity: %w", err)
	}

	return entity, nil
}

// ListEntities 列出实体
func (s *MemoryService) ListEntities(tenantID string, limit int) ([]models.Entity, error) {
	var entities []models.Entity
	err := database.DB.Select(&entities,
		`SELECT id, tenant_id, name, type, ref_id, metadata, created_at, updated_at
		 FROM entities WHERE tenant_id = ? ORDER BY created_at DESC LIMIT ?`, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list entities: %w", err)
	}
	return entities, nil
}

// SearchEntities 搜索实体
func (s *MemoryService) SearchEntities(tenantID, query string, limit int) ([]models.Entity, error) {
	keywords := extractKeywords(query)
	log.Printf("SearchEntities: tenant=%s, query=%s, keywords=%v", tenantID, query, keywords)

	var entities []models.Entity
	sqlQuery := `SELECT id, tenant_id, name, type, ref_id, metadata, created_at, updated_at
		FROM entities
		WHERE tenant_id = ?`
	args := []interface{}{tenantID}

	if len(keywords) > 0 {
		sqlQuery += " AND ("
		for i, kw := range keywords {
			if i > 0 {
				sqlQuery += " OR "
			}
			sqlQuery += "name LIKE ?"
			args = append(args, "%"+kw+"%")
		}
		sqlQuery += ")"
	}

	sqlQuery += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	err := database.DB.Select(&entities, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search entities: %w", err)
	}

	return entities, nil
}

// SaveEntityRelation 保存实体关系
func (s *MemoryService) SaveEntityRelation(tenantID, sourceID, targetID, relationType string, metadata map[string]interface{}) (*models.EntityRelation, error) {
	metadataBytes, _ := json.Marshal(metadata)

	relation := &models.EntityRelation{
		ID:             uuid.New().String(),
		TenantID:       tenantID,
		SourceEntityID: sourceID,
		TargetEntityID: targetID,
		RelationType:   relationType,
		Metadata:       metadataBytes,
		CreatedAt:      time.Now(),
	}

	_, err := database.DB.NamedExec(`
		INSERT INTO entity_relations (id, tenant_id, source_entity_id, target_entity_id, relation_type, metadata, created_at)
		VALUES (:id, :tenant_id, :source_entity_id, :target_entity_id, :relation_type, :metadata, :created_at)
		ON DUPLICATE KEY UPDATE metadata = :metadata
	`, relation)

	if err != nil {
		return nil, fmt.Errorf("failed to save entity relation: %w", err)
	}

	return relation, nil
}

// SaveSkillMemory 保存技能记忆
func (s *MemoryService) SaveSkillMemory(tenantID, userID, name, description, content, category string, tags []string) (*models.SkillMemory, error) {
	embedding, err := s.embeddingSvc.GetEmbedding(content)
	if err != nil {
		log.Printf("Failed to get embedding: %v", err)
		embedding = nil
	}

	embeddingBytes, _ := json.Marshal(embedding)
	tagsBytes, _ := json.Marshal(tags)

	memory := &models.SkillMemory{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		Name:      name,
		Content:   content,
		Embedding: embeddingBytes,
		Tags:      tagsBytes,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if userID != "" {
		memory.UserID = &userID
	}
	if description != "" {
		memory.Description = &description
	}
	if category != "" {
		memory.Category = &category
	}

	_, err = database.DB.NamedExec(`
		INSERT INTO skill_memories (id, tenant_id, user_id, name, description, content, category, tags, embedding, created_at, updated_at)
		VALUES (:id, :tenant_id, :user_id, :name, :description, :content, :category, :tags, :embedding, :created_at, :updated_at)
	`, memory)

	if err != nil {
		return nil, fmt.Errorf("failed to save skill memory: %w", err)
	}

	return memory, nil
}

// ListSkillMemories 列出技能记忆
func (s *MemoryService) ListSkillMemories(tenantID string, limit int) ([]models.SkillMemory, error) {
	var memories []models.SkillMemory
	err := database.DB.Select(&memories,
		`SELECT id, tenant_id, user_id, name, description, content, category, tags, usage_count, created_at, updated_at
		 FROM skill_memories WHERE tenant_id = ? ORDER BY usage_count DESC, created_at DESC LIMIT ?`, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list skill memories: %w", err)
	}
	return memories, nil
}

// SearchSkillMemories 搜索技能记忆
func (s *MemoryService) SearchSkillMemories(tenantID, query string, limit int) ([]models.SkillMemory, error) {
	keywords := extractKeywords(query)
	log.Printf("SearchSkillMemories: tenant=%s, query=%s, keywords=%v", tenantID, query, keywords)

	var memories []models.SkillMemory
	sqlQuery := `SELECT id, tenant_id, user_id, name, description, content, category, tags, usage_count, created_at, updated_at
		FROM skill_memories
		WHERE tenant_id = ?`
	args := []interface{}{tenantID}

	if len(keywords) > 0 {
		sqlQuery += " AND ("
		for i, kw := range keywords {
			if i > 0 {
				sqlQuery += " OR "
			}
			sqlQuery += "name LIKE ? OR content LIKE ?"
			args = append(args, "%"+kw+"%", "%"+kw+"%")
		}
		sqlQuery += ")"
	}

	sqlQuery += " ORDER BY usage_count DESC, created_at DESC LIMIT ?"
	args = append(args, limit)

	err := database.DB.Select(&memories, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search skill memories: %w", err)
	}

	return memories, nil
}

// GetMemoryStats 获取记忆统计
func (s *MemoryService) GetMemoryStats(tenantID string) (*models.MemoryStats, error) {
	stats := &models.MemoryStats{
		ByType: make(map[string]int),
	}

	// 用户记忆统计
	var userCount int
	err := database.DB.Get(&userCount, "SELECT COUNT(*) FROM user_memories WHERE tenant_id = ?", tenantID)
	if err == nil {
		stats.TotalUserMemories = userCount
	}

	// 会话记忆统计
	var sessionCount int
	err = database.DB.Get(&sessionCount, "SELECT COUNT(*) FROM session_memories WHERE tenant_id = ?", tenantID)
	if err == nil {
		stats.TotalSessionMemories = sessionCount
	}

	// 实体统计
	var entityCount int
	err = database.DB.Get(&entityCount, "SELECT COUNT(*) FROM entities WHERE tenant_id = ?", tenantID)
	if err == nil {
		stats.TotalEntities = entityCount
	}

	// 技能记忆统计
	var skillCount int
	err = database.DB.Get(&skillCount, "SELECT COUNT(*) FROM skill_memories WHERE tenant_id = ?", tenantID)
	if err == nil {
		stats.TotalSkillMemories = skillCount
	}

	// 按类型统计
	rows, err := database.DB.Query("SELECT type, COUNT(*) FROM user_memories WHERE tenant_id = ? GROUP BY type", tenantID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var memType string
			var count int
			if err := rows.Scan(&memType, &count); err == nil {
				stats.ByType[memType] = count
			}
		}
	}

	return stats, nil
}

// updateAccessCount 更新访问计数
func (s *MemoryService) updateAccessCount(memoryID string) {
	database.DB.Exec(`
		UPDATE user_memories 
		SET access_count = access_count + 1, last_accessed_at = NOW() 
		WHERE id = ?
	`, memoryID)
}

// GetRoleMemories 获取角色共享记忆（同租户其他用户的fact类型记忆）
func (s *MemoryService) GetRoleMemories(tenantID, excludeUserID, query string, limit int) ([]models.UserMemory, error) {
	var memories []models.UserMemory
	sqlQuery := userMemorySelectBase() + `
		WHERE tenant_id = ? AND user_id != ? AND type = 'fact' AND status = 'active'`
	args := []interface{}{tenantID, excludeUserID}

	if query != "" {
		keywords := extractKeywords(query)
		if len(keywords) > 0 {
			sqlQuery += " AND ("
			for i, kw := range keywords {
				if i > 0 {
					sqlQuery += " OR "
				}
				sqlQuery += "content LIKE ?"
				args = append(args, "%"+kw+"%")
			}
			sqlQuery += ")"
		}
	}

	keywords := extractKeywords(query)
	candidateLimit := limit * 5
	if candidateLimit < 20 {
		candidateLimit = 20
	}
	sqlQuery += " ORDER BY updated_at DESC, created_at DESC LIMIT ?"
	args = append(args, candidateLimit)

	err := database.DB.Select(&memories, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get role memories: %w", err)
	}

	return RankUserMemories(memories, keywords, limit, time.Now()), nil
}

// extractKeywords 提取关键词（支持中文）
// 策略：先按标点分词，长短语再滑动窗口拆分，短词直接保留
func extractKeywords(query string) []string {
	// 中文停用词（常见无意义词）
	stopWords := map[string]bool{
		"的": true, "了": true, "是": true, "在": true, "我": true,
		"有": true, "和": true, "就": true, "不": true, "人": true,
		"都": true, "一": true, "一个": true, "上": true, "也": true,
		"很": true, "到": true, "说": true, "要": true, "去": true,
		"你": true, "会": true, "着": true, "没有": true, "看": true,
		"好": true, "自己": true, "这": true, "他": true, "什么": true,
		"她": true, "吗": true, "呢": true, "吧": true, "啊": true,
		"嗯": true, "哦": true, "把": true,
		"被": true, "让": true, "给": true, "从": true, "向": true,
		"对": true, "但": true, "而": true, "又": true, "与": true,
		"或": true, "如果": true, "因为": true, "所以": true, "可以": true,
		"这个": true, "那个": true, "怎么": true, "哪些": true,
		"请": true, "帮我": true, "告诉": true, "一下": true,
		"怎样": true, "如何": true, "为什么": true, "多少": true, "几个": true,
		"我的": true, "你的": true, "他的": true, "她的": true,
		"我们": true, "你们": true, "他们": true, "她们": true,
		"什么的": true, "是不是": true, "能不能": true, "会不会": true,
	}

	runes := []rune(query)
	keywords := make([]string, 0)
	seen := make(map[string]bool)

	// 1. 按标点/空格分词
	current := make([]rune, 0)
	flush := func() {
		if len(current) == 0 {
			return
		}
		word := string(current)
		current = current[:0]
		// 太短的跳过
		if len([]rune(word)) < 2 {
			return
		}
		// 停用词跳过
		if stopWords[word] {
			return
		}
		// 短词（≤6字符）直接保留
		runeLen := len([]rune(word))
		if runeLen <= 6 {
			if !seen[word] {
				keywords = append(keywords, word)
				seen[word] = true
			}
			return
		}
		// 长短语：滑动窗口提取 2-4 字子串
		wRunes := []rune(word)
		for i := 0; i < len(wRunes); i++ {
			for size := 2; size <= 4 && i+size <= len(wRunes); size++ {
				sub := string(wRunes[i : i+size])
				if !stopWords[sub] && !seen[sub] {
					keywords = append(keywords, sub)
					seen[sub] = true
				}
			}
		}
	}

	isDelimiter := func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' ||
			r == ',' || r == '.' || r == '!' || r == '?' ||
			r == '，' || r == '。' || r == '！' || r == '？' ||
			r == '；' || r == '：' || r == '、' ||
			r == '(' || r == ')' || r == '[' || r == ']' ||
			r == '{' || r == '}' || r == '《' || r == '》' ||
			r == '【' || r == '】' || r == '「' || r == '」' ||
			r == '\'' || r == '"' || r == '`' || r == '~' || r == '～' ||
			r == '\u201c' || r == '\u201d' // 中文双引号 ""
	}

	for _, r := range runes {
		if isDelimiter(r) {
			flush()
		} else {
			current = append(current, r)
		}
	}
	flush()

	// 2. 如果分词后关键词太少（<2个），对整个query做滑动窗口兜底
	if len(keywords) < 2 && len(runes) >= 2 {
		for i := 0; i < len(runes); i++ {
			for size := 2; size <= 4 && i+size <= len(runes); size++ {
				sub := string(runes[i : i+size])
				if !stopWords[sub] && !seen[sub] {
					keywords = append(keywords, sub)
					seen[sub] = true
				}
			}
		}
	}

	// 限制关键词数量，避免SQL LIKE太多
	if len(keywords) > 10 {
		keywords = keywords[:10]
	}

	return keywords
}
