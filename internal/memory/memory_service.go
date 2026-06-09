package memory

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/models"
	"github.com/google/uuid"
)

// MemoryService 记忆服务
type MemoryService struct {
	embeddingSvc EmbeddingService
}

// EmbeddingService Embedding服务接口
type EmbeddingService interface {
	GetEmbedding(text string) ([]float32, error)
	GetEmbeddings(texts []string) ([][]float32, error)
}

// MemoryConfig 记忆配置
type MemoryConfig struct {
	EmbeddingService EmbeddingService
}

// NewMemoryService 创建记忆服务
func NewMemoryService(config MemoryConfig) *MemoryService {
	return &MemoryService{
		embeddingSvc: config.EmbeddingService,
	}
}

// SaveUserMemory 保存用户记忆
func (s *MemoryService) SaveUserMemory(tenantID, userID, memType, content string, metadata map[string]interface{}) (*models.UserMemory, error) {
	// 生成向量
	embedding, err := s.embeddingSvc.GetEmbedding(content)
	if err != nil {
		log.Printf("Failed to get embedding: %v", err)
		// 不阻塞保存，embedding失败只影响检索
		embedding = nil
	}

	embeddingBytes, _ := json.Marshal(embedding)
	metadataBytes, _ := json.Marshal(metadata)

	memory := &models.UserMemory{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		UserID:    userID,
		Type:      memType,
		Content:   content,
		Embedding: embeddingBytes,
		Metadata:  metadataBytes,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err = database.DB.NamedExec(`
		INSERT INTO user_memories (id, tenant_id, user_id, type, content, embedding, metadata, created_at, updated_at)
		VALUES (:id, :tenant_id, :user_id, :type, :content, :embedding, :metadata, :created_at, :updated_at)
	`, memory)

	if err != nil {
		return nil, fmt.Errorf("failed to save user memory: %w", err)
	}

	return memory, nil
}

// GetUserMemories 获取用户记忆
func (s *MemoryService) GetUserMemories(tenantID, userID string, memType string, limit int) ([]models.UserMemory, error) {
	var memories []models.UserMemory
	query := "SELECT id, tenant_id, user_id, type, content, COALESCE(entity_ids, '[]') as entity_ids, COALESCE(metadata, '{}') as metadata, access_count, last_accessed_at, created_at, updated_at FROM user_memories WHERE tenant_id = ? AND user_id = ?"
	args := []interface{}{tenantID, userID}

	if memType != "" {
		query += " AND type = ?"
		args = append(args, memType)
	}

	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	err := database.DB.Select(&memories, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get user memories: %w", err)
	}

	return memories, nil
}

// SearchUserMemories 搜索用户记忆
func (s *MemoryService) SearchUserMemories(tenantID, userID, query string, limit int) ([]models.UserMemory, error) {
	// 1. 语义搜索 (暂时跳过，后续接入向量DB)
	_, err := s.embeddingSvc.GetEmbedding(query)
	if err != nil {
		// embedding失败不影响关键词搜索
		log.Printf("Failed to get embedding: %v", err)
	}

	// 2. 关键词搜索
	keywords := extractKeywords(query)
	log.Printf("SearchUserMemories: tenant=%s, user=%s, query=%s, keywords=%v", tenantID, userID, query, keywords)

	var memories []models.UserMemory
	sqlQuery := `SELECT id, tenant_id, user_id, type, content, 
		COALESCE(entity_ids, '[]') as entity_ids, 
		COALESCE(metadata, '{}') as metadata, 
		access_count, last_accessed_at, created_at, updated_at 
		FROM user_memories 
		WHERE tenant_id = ? AND user_id = ?`
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

	sqlQuery += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	log.Printf("SearchUserMemories: SQL=%s, args=%v", sqlQuery, args)
	err = database.DB.Select(&memories, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search user memories: %w", err)
	}

	log.Printf("SearchUserMemories: found %d memories", len(memories))

	// 更新访问计数
	for _, mem := range memories {
		s.updateAccessCount(mem.ID)
	}

	return memories, nil
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
	sqlQuery := `SELECT id, tenant_id, user_id, type, content, 
		COALESCE(entity_ids, '[]') as entity_ids, 
		COALESCE(metadata, '{}') as metadata, 
		access_count, last_accessed_at, created_at, updated_at 
		FROM user_memories 
		WHERE tenant_id = ? AND user_id != ? AND type = 'fact'`
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

	sqlQuery += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	err := database.DB.Select(&memories, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get role memories: %w", err)
	}

	return memories, nil
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
