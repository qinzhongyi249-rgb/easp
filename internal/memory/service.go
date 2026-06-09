package memory

import (
	"fmt"
	"log"
	"time"

	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/embedding"
	"github.com/easp-platform/easp/internal/vectordb"
	"github.com/google/uuid"
)

// VectorMemoryService 向量记忆服务
type VectorMemoryService struct {
	vectorDB     *vectordb.Client
	embeddingSvc *embedding.EmbeddingService
	database     string
	collection   string
}

// VectorMemoryConfig 向量记忆配置
type VectorMemoryConfig struct {
	BridgeURL  string                `json:"bridge_url"`  // 桥接服务地址
	Database   string                `json:"database"`
	Collection string                `json:"collection"`
	Dimension  int                   `json:"dimension"`
}

// NewVectorMemoryService 创建向量记忆服务
func NewVectorMemoryService(config VectorMemoryConfig) *VectorMemoryService {
	bridgeURL := config.BridgeURL
	if bridgeURL == "" {
		bridgeURL = "http://localhost:8083"
	}

	return &VectorMemoryService{
		vectorDB: vectordb.NewClient(vectordb.Config{
			Endpoint: bridgeURL,
			Timeout:  30,
		}),
		embeddingSvc: embedding.NewEmbeddingService(embedding.EmbeddingConfig{
			Endpoint:  bridgeURL,
			Dimension: config.Dimension,
		}),
		database:   config.Database,
		collection: config.Collection,
	}
}

// VectorMemory 向量记忆
type VectorMemory struct {
	ID          string                 `json:"id" db:"id"`
	TenantID    string                 `json:"tenant_id" db:"tenant_id"`
	PoolID      string                 `json:"pool_id" db:"pool_id"`
	Content     string                 `json:"content" db:"content"`
	Type        string                 `json:"type" db:"type"`
	Sensitivity string                 `json:"sensitivity" db:"sensitivity"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at" db:"created_at"`
}

// Init 初始化
func (s *VectorMemoryService) Init() error {
	// 健康检查
	if err := s.vectorDB.HealthCheck(); err != nil {
		return fmt.Errorf("vector bridge not available: %w", err)
	}

	// 确保数据库存在
	if err := s.vectorDB.CreateDatabase(s.database); err != nil {
		log.Printf("Database may already exist: %v", err)
	}

	// 确保Collection存在
	if err := s.vectorDB.CreateCollection(s.database, s.collection, s.embeddingSvc.GetDimension()); err != nil {
		log.Printf("Collection may already exist: %v", err)
	}

	return nil
}

// SaveMemory 保存记忆
func (s *VectorMemoryService) SaveMemory(memory VectorMemory) error {
	// 生成Embedding
	vector, err := s.embeddingSvc.GetEmbedding(memory.Content)
	if err != nil {
		return fmt.Errorf("failed to get embedding: %w", err)
	}

	// 保存到向量数据库
	doc := vectordb.Document{
		ID:     memory.ID,
		Vector: vector,
		Fields: map[string]interface{}{
			"tenant_id":   memory.TenantID,
			"pool_id":     memory.PoolID,
			"content":     memory.Content,
			"type":        memory.Type,
			"sensitivity": memory.Sensitivity,
		},
	}

	if err := s.vectorDB.Insert(s.database, s.collection, []vectordb.Document{doc}); err != nil {
		return fmt.Errorf("failed to insert vector: %w", err)
	}

	// 同时保存到MySQL
	_, err = database.DB.NamedExec(`
		INSERT INTO memory_vectors (id, tenant_id, pool_id, content, type, sensitivity, created_at)
		VALUES (:id, :tenant_id, :pool_id, :content, :type, :sensitivity, :created_at)
	`, memory)
	if err != nil {
		log.Printf("Failed to save to MySQL: %v", err)
	}

	return nil
}

// SearchMemories 搜索相似记忆
func (s *VectorMemoryService) SearchMemories(tenantID, poolID, query string, limit int) ([]VectorMemory, error) {
	// 生成查询向量
	vector, err := s.embeddingSvc.GetEmbedding(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get embedding: %w", err)
	}

	// 向量搜索
	results, err := s.vectorDB.Search(s.database, s.collection, vector, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	// 转换结果 - 从MySQL获取完整数据
	memories := make([]VectorMemory, 0, len(results))
	ids := make([]string, 0, len(results))
	for _, r := range results {
		ids = append(ids, r.ID)
	}

	if len(ids) > 0 {
		// 从MySQL获取完整数据
		query := "SELECT id, tenant_id, pool_id, content, type, sensitivity, created_at FROM memory_vectors WHERE id IN (?)"
		args := make([]interface{}, len(ids))
		for i, id := range ids {
			args[i] = id
		}

		var dbMemories []VectorMemory
		err = database.DB.Select(&dbMemories, query, args...)
		if err == nil {
			memories = dbMemories
		}
	}

	return memories, nil
}

// DeleteMemory 删除记忆
func (s *VectorMemoryService) DeleteMemory(id string) error {
	if err := s.vectorDB.Delete(s.database, s.collection, []string{id}); err != nil {
		return fmt.Errorf("failed to delete vector: %w", err)
	}

	_, err := database.DB.Exec("DELETE FROM memory_vectors WHERE id = ?", id)
	if err != nil {
		log.Printf("Failed to delete from MySQL: %v", err)
	}

	return nil
}

// ListMemories 列出记忆
func (s *VectorMemoryService) ListMemories(tenantID, poolID string, limit int) ([]VectorMemory, error) {
	var memories []VectorMemory
	query := "SELECT id, tenant_id, pool_id, content, type, sensitivity, created_at FROM memory_vectors WHERE tenant_id = ?"
	args := []interface{}{tenantID}

	if poolID != "" {
		query += " AND pool_id = ?"
		args = append(args, poolID)
	}

	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	err := database.DB.Select(&memories, query, args...)
	if err != nil {
		return nil, err
	}

	return memories, nil
}

// GenerateID 生成记忆ID
func GenerateID() string {
	return uuid.New().String()
}
