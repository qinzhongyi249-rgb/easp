package memory

import (
	"fmt"
	"log"
	"time"

	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/models"
	"github.com/jmoiron/sqlx"
)

func userMemorySelectBase() string {
	return `SELECT id, tenant_id, user_id, pool_id, type, content, content_hash, source, status,
		COALESCE(entity_ids, '[]') as entity_ids,
		COALESCE(metadata, '{}') as metadata,
		access_count, last_accessed_at, last_seen_at, vector_indexed_at, created_at, updated_at
		FROM user_memories `
}

func memorySource(metadata map[string]interface{}) string {
	if metadata == nil {
		return "manual"
	}
	if source, ok := metadata["source"].(string); ok && source != "" {
		return source
	}
	return "manual"
}

func getUserMemoriesByIDs(ids []string) ([]models.UserMemory, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	query, args, err := sqlx.In(userMemorySelectBase()+"WHERE id IN (?) AND status = 'active'", ids)
	if err != nil {
		return nil, err
	}
	query = database.DB.Rebind(query)
	var memories []models.UserMemory
	if err := database.DB.Select(&memories, query, args...); err != nil {
		return nil, err
	}
	byID := map[string]models.UserMemory{}
	for _, mem := range memories {
		byID[mem.ID] = mem
	}
	ordered := make([]models.UserMemory, 0, len(memories))
	for _, id := range ids {
		if mem, ok := byID[id]; ok {
			ordered = append(ordered, mem)
		}
	}
	return ordered, nil
}

func appendMissingUserMemories(base []models.UserMemory, extra []models.UserMemory) []models.UserMemory {
	seen := map[string]bool{}
	for _, mem := range base {
		seen[mem.ID] = true
	}
	merged := append([]models.UserMemory(nil), base...)
	for _, mem := range extra {
		if !seen[mem.ID] {
			merged = append(merged, mem)
			seen[mem.ID] = true
		}
	}
	return merged
}

func (s *MemoryService) tryMergeUserMemory(tenantID, userID, memType, content, source string) (*models.UserMemory, MemoryMergeDecision, error) {
	var candidates []models.UserMemory
	err := database.DB.Select(&candidates, userMemorySelectBase()+`
		WHERE tenant_id = ? AND user_id = ? AND type = ? AND status = 'active'
		ORDER BY last_seen_at DESC, updated_at DESC, created_at DESC
		LIMIT 20`, tenantID, userID, memType)
	if err != nil {
		return nil, MemoryMergeDecision{}, err
	}
	var conflict *MemoryMergeDecision
	for _, candidate := range candidates {
		decision := ShouldMergeUserMemory(candidate, content)
		if decision.Conflict {
			conflict = &decision
			s.SaveMemoryAudit(MemoryAuditLog{
				TenantID:         tenantID,
				UserID:           userID,
				MemoryID:         &candidate.ID,
				Action:           "merge_conflict",
				Source:           source,
				OriginalPreview:  candidate.Content,
				SanitizedPreview: content,
				Reason:           decision.Reason,
				Metadata: map[string]interface{}{
					"similarity": decision.Similarity,
					"type":       memType,
				},
			})
			continue
		}
		if !decision.Merge {
			continue
		}
		newHash := MemoryContentHash(decision.MergedContent)
		_, err := database.DB.Exec(`
			UPDATE user_memories
			SET content = ?, content_hash = ?, access_count = access_count + 1,
			    last_seen_at = NOW(), updated_at = NOW(), status = 'active'
			WHERE id = ?`, decision.MergedContent, newHash, candidate.ID)
		if err != nil {
			return nil, decision, fmt.Errorf("failed to merge user memory: %w", err)
		}
		var merged models.UserMemory
		if err := database.DB.Get(&merged, userMemorySelectBase()+`WHERE id = ? LIMIT 1`, candidate.ID); err != nil {
			return nil, decision, err
		}
		s.SaveMemoryAudit(MemoryAuditLog{
			TenantID:         tenantID,
			UserID:           userID,
			MemoryID:         &merged.ID,
			Action:           "merged",
			Source:           source,
			OriginalPreview:  candidate.Content,
			SanitizedPreview: merged.Content,
			Reason:           decision.Reason,
			Metadata: map[string]interface{}{
				"similarity":   decision.Similarity,
				"type":         memType,
				"content_hash": newHash,
			},
		})
		return &merged, decision, nil
	}
	if conflict != nil {
		return nil, *conflict, nil
	}
	return nil, MemoryMergeDecision{}, nil
}

func (s *MemoryService) enforceUserMemoryCapacity(tenantID, userID, memType, source string) {
	const maxActivePerType = 200
	var candidates []models.UserMemory
	err := database.DB.Select(&candidates, userMemorySelectBase()+`
		WHERE tenant_id = ? AND user_id = ? AND type = ? AND status = 'active'
		ORDER BY updated_at DESC, created_at DESC
		LIMIT 1000`, tenantID, userID, memType)
	if err != nil {
		log.Printf("enforce user memory capacity query failed: %v", err)
		return
	}
	archive := SelectUserMemoriesToArchive(candidates, maxActivePerType, timeNow())
	for _, mem := range archive {
		_, err := database.DB.Exec(`UPDATE user_memories SET status = 'archived', updated_at = NOW() WHERE id = ? AND status = 'active'`, mem.ID)
		if err != nil {
			log.Printf("archive low-value memory failed: %v", err)
			continue
		}
		s.SaveMemoryAudit(MemoryAuditLog{
			TenantID:         tenantID,
			UserID:           userID,
			MemoryID:         &mem.ID,
			Action:           "archived_capacity",
			Source:           source,
			SanitizedPreview: mem.Content,
			Reason:           "active user memories exceeded capacity",
			Metadata: map[string]interface{}{
				"type":       memType,
				"max_active": maxActivePerType,
			},
		})
	}
}

func timeNow() time.Time {
	return time.Now()
}

func (s *MemoryService) indexUserMemoryVector(memory models.UserMemory, source string) {
	if s.vectorSvc == nil {
		return
	}
	poolID := "user_memories"
	if memory.PoolID != nil && *memory.PoolID != "" {
		poolID = *memory.PoolID
	}
	vectorMemory := VectorMemory{
		ID:          memory.ID,
		TenantID:    memory.TenantID,
		PoolID:      poolID,
		Content:     memory.Content,
		Type:        memory.Type,
		Sensitivity: "normal",
		Metadata: map[string]interface{}{
			"user_id": memory.UserID,
			"source":  source,
			"table":   "user_memories",
		},
		CreatedAt: memory.CreatedAt,
	}
	if err := s.vectorSvc.SaveMemory(vectorMemory); err != nil {
		log.Printf("index user memory vector failed: %v", err)
		s.SaveMemoryAudit(MemoryAuditLog{
			TenantID:         memory.TenantID,
			UserID:           memory.UserID,
			MemoryID:         &memory.ID,
			Action:           "vector_index_failed",
			Source:           source,
			SanitizedPreview: memory.Content,
			Reason:           err.Error(),
			Metadata: map[string]interface{}{
				"hybrid_search": true,
			},
		})
		return
	}
	_, err := database.DB.Exec("UPDATE user_memories SET vector_indexed_at = NOW() WHERE id = ?", memory.ID)
	if err != nil {
		log.Printf("update vector_indexed_at failed: %v", err)
	}
	s.SaveMemoryAudit(MemoryAuditLog{
		TenantID:         memory.TenantID,
		UserID:           memory.UserID,
		MemoryID:         &memory.ID,
		Action:           "vector_indexed",
		Source:           source,
		SanitizedPreview: memory.Content,
		Reason:           "indexed into vector database for hybrid retrieval",
		Metadata: map[string]interface{}{
			"hybrid_search": true,
		},
	})
}
