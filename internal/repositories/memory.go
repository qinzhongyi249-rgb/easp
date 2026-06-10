package repositories

import (
	"time"

	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/models"
	"github.com/google/uuid"
)

// MemoryPoolRepository 记忆池仓库
type MemoryPoolRepository struct{}

func NewMemoryPoolRepository() *MemoryPoolRepository {
	return &MemoryPoolRepository{}
}

func (r *MemoryPoolRepository) Create(pool *models.MemoryPool) error {
	pool.ID = uuid.New().String()
	pool.CreatedAt = time.Now()
	pool.UpdatedAt = time.Now()

	// 设置默认值
	if pool.Type == "" {
		pool.Type = "personal"
	}
	if pool.Purpose == "" {
		pool.Purpose = "conversation"
	}
	if pool.Priority == 0 {
		pool.Priority = 5
	}

	query := `INSERT INTO memory_pools 
		(id, tenant_id, name, description, type, purpose, priority, max_tokens, auto_activate, trigger_rules, owner_id, enabled, memory_count, created_at, updated_at) 
		VALUES (:id, :tenant_id, :name, :description, :type, :purpose, :priority, :max_tokens, :auto_activate, :trigger_rules, :owner_id, :enabled, :memory_count, :created_at, :updated_at)`
	_, err := database.DB.NamedExec(query, pool)
	return err
}

func (r *MemoryPoolRepository) GetByID(id string) (*models.MemoryPool, error) {
	var pool models.MemoryPool
	err := database.DB.Get(&pool, "SELECT * FROM memory_pools WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	return &pool, nil
}

func (r *MemoryPoolRepository) ListByTenant(tenantID string) ([]models.MemoryPool, error) {
	var pools []models.MemoryPool
	err := database.DB.Select(&pools, "SELECT * FROM memory_pools WHERE tenant_id = ? ORDER BY priority DESC, created_at DESC", tenantID)
	if pools == nil {
		pools = []models.MemoryPool{}
	}
	return pools, err
}

func (r *MemoryPoolRepository) ListActiveByTenant(tenantID string) ([]models.MemoryPool, error) {
	var pools []models.MemoryPool
	err := database.DB.Select(&pools, "SELECT * FROM memory_pools WHERE tenant_id = ? AND enabled = true ORDER BY priority DESC", tenantID)
	if pools == nil {
		pools = []models.MemoryPool{}
	}
	return pools, err
}

func (r *MemoryPoolRepository) Update(pool *models.MemoryPool) error {
	pool.UpdatedAt = time.Now()
	query := `UPDATE memory_pools SET 
		name=:name, description=:description, type=:type, purpose=:purpose, 
		priority=:priority, max_tokens=:max_tokens, auto_activate=:auto_activate, 
		trigger_rules=:trigger_rules, enabled=:enabled, updated_at=:updated_at 
		WHERE id=:id`
	_, err := database.DB.NamedExec(query, pool)
	return err
}

func (r *MemoryPoolRepository) Delete(id string) error {
	_, err := database.DB.Exec("DELETE FROM memory_pools WHERE id = ?", id)
	return err
}

// UpdateMemoryCount 更新记忆池的记忆数量
func (r *MemoryPoolRepository) UpdateMemoryCount(poolID string) error {
	_, err := database.DB.Exec(`
		UPDATE memory_pools SET memory_count = (
			SELECT COUNT(*) FROM user_memories WHERE pool_id = ?
		) + (
			SELECT COUNT(*) FROM entities WHERE pool_id = ?
		) + (
			SELECT COUNT(*) FROM skill_memories WHERE pool_id = ?
		) WHERE id = ?
	`, poolID, poolID, poolID, poolID)
	return err
}

// MemoryEntryRepository 记忆条目仓库
type MemoryEntryRepository struct{}

func NewMemoryEntryRepository() *MemoryEntryRepository {
	return &MemoryEntryRepository{}
}

func (r *MemoryEntryRepository) Create(entry *models.MemoryEntry) error {
	entry.ID = uuid.New().String()
	entry.CreatedAt = time.Now()
	entry.UpdatedAt = time.Now()

	query := `INSERT INTO memory_entries (id, pool_id, type, content, metadata, sensitivity, created_at, updated_at) 
			  VALUES (:id, :pool_id, :type, :content, :metadata, :sensitivity, :created_at, :updated_at)`
	_, err := database.DB.NamedExec(query, entry)
	return err
}

func (r *MemoryEntryRepository) GetByID(id string) (*models.MemoryEntry, error) {
	var entry models.MemoryEntry
	err := database.DB.Get(&entry, "SELECT * FROM memory_entries WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func (r *MemoryEntryRepository) ListByPool(poolID string, limit int) ([]models.MemoryEntry, error) {
	var entries []models.MemoryEntry
	err := database.DB.Select(&entries, "SELECT * FROM memory_entries WHERE pool_id = ? ORDER BY created_at DESC LIMIT ?", poolID, limit)
	return entries, err
}

func (r *MemoryEntryRepository) ListByType(poolID, entryType string, limit int) ([]models.MemoryEntry, error) {
	var entries []models.MemoryEntry
	err := database.DB.Select(&entries, "SELECT * FROM memory_entries WHERE pool_id = ? AND type = ? ORDER BY created_at DESC LIMIT ?", poolID, entryType, limit)
	return entries, err
}

func (r *MemoryEntryRepository) SearchByContent(poolID, keyword string, limit int) ([]models.MemoryEntry, error) {
	var entries []models.MemoryEntry
	query := `SELECT * FROM memory_entries WHERE pool_id = ? AND content LIKE ? ORDER BY created_at DESC LIMIT ?`
	err := database.DB.Select(&entries, query, poolID, "%"+keyword+"%", limit)
	return entries, err
}

func (r *MemoryEntryRepository) Update(entry *models.MemoryEntry) error {
	entry.UpdatedAt = time.Now()
	query := `UPDATE memory_entries SET content=:content, metadata=:metadata, sensitivity=:sensitivity, updated_at=:updated_at WHERE id=:id`
	_, err := database.DB.NamedExec(query, entry)
	return err
}

func (r *MemoryEntryRepository) Delete(id string) error {
	_, err := database.DB.Exec("DELETE FROM memory_entries WHERE id = ?", id)
	return err
}

func (r *MemoryEntryRepository) CleanupOldEntries(poolID string, daysOld int) (int64, error) {
	result, err := database.DB.Exec("DELETE FROM memory_entries WHERE pool_id = ? AND created_at < DATE_SUB(NOW(), INTERVAL ? DAY)", poolID, daysOld)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
