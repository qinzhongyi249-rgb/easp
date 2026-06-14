package memory

import (
	"database/sql"
	"encoding/json"
	"log"
	"time"

	"github.com/easp-platform/easp/internal/database"
	"github.com/google/uuid"
)

// MemorySettings controls persistence, recall and governance behavior.
type MemorySettings struct {
	ID                     string    `json:"id" db:"id"`
	TenantID               string    `json:"tenant_id" db:"tenant_id"`
	UserID                 *string   `json:"user_id,omitempty" db:"user_id"`
	AutoExtractEnabled     bool      `json:"auto_extract_enabled" db:"auto_extract_enabled"`
	RecallEnabled          bool      `json:"recall_enabled" db:"recall_enabled"`
	SensitiveFilterEnabled bool      `json:"sensitive_filter_enabled" db:"sensitive_filter_enabled"`
	AuditEnabled           bool      `json:"audit_enabled" db:"audit_enabled"`
	HybridSearchEnabled    bool      `json:"hybrid_search_enabled" db:"hybrid_search_enabled"`
	HybridSearchMode       string    `json:"hybrid_search_mode" db:"hybrid_search_mode"`
	CreatedAt              time.Time `json:"created_at" db:"created_at"`
	UpdatedAt              time.Time `json:"updated_at" db:"updated_at"`
}

// MemoryAuditLog records memory governance decisions.
type MemoryAuditLog struct {
	ID               string                 `json:"id" db:"id"`
	TenantID         string                 `json:"tenant_id" db:"tenant_id"`
	UserID           string                 `json:"user_id" db:"user_id"`
	MemoryID         *string                `json:"memory_id,omitempty" db:"memory_id"`
	Action           string                 `json:"action" db:"action"`
	Source           string                 `json:"source" db:"source"`
	OriginalPreview  string                 `json:"original_preview" db:"original_preview"`
	SanitizedPreview string                 `json:"sanitized_preview" db:"sanitized_preview"`
	Reason           string                 `json:"reason" db:"reason"`
	Metadata         map[string]interface{} `json:"metadata,omitempty" db:"-"`
	MetadataJSON     []byte                 `json:"-" db:"metadata"`
	CreatedAt        time.Time              `json:"created_at" db:"created_at"`
}

// DefaultMemorySettings returns safe defaults when no tenant/user override exists.
func DefaultMemorySettings(tenantID, userID string) MemorySettings {
	var uid *string
	if userID != "" {
		uid = &userID
	}
	return MemorySettings{
		ID:                     uuid.New().String(),
		TenantID:               tenantID,
		UserID:                 uid,
		AutoExtractEnabled:     true,
		RecallEnabled:          true,
		SensitiveFilterEnabled: true,
		AuditEnabled:           true,
		HybridSearchEnabled:    true,
		HybridSearchMode:       "keyword_vector",
	}
}

// GetMemorySettings returns user override first, then tenant setting, then defaults.
func (s *MemoryService) GetMemorySettings(tenantID, userID string) MemorySettings {
	if database.DB == nil {
		return DefaultMemorySettings(tenantID, userID)
	}
	var settings MemorySettings
	if userID != "" {
		err := database.DB.Get(&settings, `
			SELECT id, tenant_id, user_id, auto_extract_enabled, recall_enabled, sensitive_filter_enabled,
			       audit_enabled, hybrid_search_enabled, hybrid_search_mode, created_at, updated_at
			FROM memory_settings
			WHERE tenant_id = ? AND user_id = ?
			LIMIT 1`, tenantID, userID)
		if err == nil {
			return settings
		}
		if err != sql.ErrNoRows {
			log.Printf("GetMemorySettings user override failed: %v", err)
		}
	}

	err := database.DB.Get(&settings, `
		SELECT id, tenant_id, user_id, auto_extract_enabled, recall_enabled, sensitive_filter_enabled,
		       audit_enabled, hybrid_search_enabled, hybrid_search_mode, created_at, updated_at
		FROM memory_settings
		WHERE tenant_id = ? AND user_id IS NULL
		LIMIT 1`, tenantID)
	if err == nil {
		return settings
	}
	if err != sql.ErrNoRows {
		log.Printf("GetMemorySettings tenant setting failed: %v", err)
	}
	return DefaultMemorySettings(tenantID, userID)
}

// SaveMemorySettings upserts tenant/user memory governance settings.
func (s *MemoryService) SaveMemorySettings(settings MemorySettings) error {
	if database.DB == nil {
		return nil
	}
	if settings.ID == "" {
		settings.ID = uuid.New().String()
	}
	if settings.HybridSearchMode == "" {
		settings.HybridSearchMode = "keyword_vector"
	}
	_, err := database.DB.NamedExec(`
		INSERT INTO memory_settings
		(id, tenant_id, user_id, auto_extract_enabled, recall_enabled, sensitive_filter_enabled,
		 audit_enabled, hybrid_search_enabled, hybrid_search_mode, created_at, updated_at)
		VALUES (:id, :tenant_id, :user_id, :auto_extract_enabled, :recall_enabled, :sensitive_filter_enabled,
		 :audit_enabled, :hybrid_search_enabled, :hybrid_search_mode, NOW(), NOW())
		ON DUPLICATE KEY UPDATE
			auto_extract_enabled = :auto_extract_enabled,
			recall_enabled = :recall_enabled,
			sensitive_filter_enabled = :sensitive_filter_enabled,
			audit_enabled = :audit_enabled,
			hybrid_search_enabled = :hybrid_search_enabled,
			hybrid_search_mode = :hybrid_search_mode,
			updated_at = NOW()`, &settings)
	return err
}

// ListMemoryAuditLogs returns recent memory governance audit entries.
func (s *MemoryService) ListMemoryAuditLogs(tenantID string, limit int) ([]MemoryAuditLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var logs []MemoryAuditLog
	err := database.DB.Select(&logs, `
		SELECT id, tenant_id, user_id, memory_id, action, source, original_preview, sanitized_preview, reason, metadata, created_at
		FROM memory_audit_logs
		WHERE tenant_id = ?
		ORDER BY created_at DESC
		LIMIT ?`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	for i := range logs {
		if len(logs[i].MetadataJSON) > 0 {
			_ = json.Unmarshal(logs[i].MetadataJSON, &logs[i].Metadata)
		}
	}
	return logs, nil
}

// SaveMemoryAudit writes a memory audit log if auditing is enabled.
func (s *MemoryService) SaveMemoryAudit(logEntry MemoryAuditLog) {
	if database.DB == nil || logEntry.TenantID == "" || logEntry.UserID == "" || logEntry.Action == "" {
		return
	}
	settings := s.GetMemorySettings(logEntry.TenantID, logEntry.UserID)
	if !settings.AuditEnabled {
		return
	}
	if logEntry.ID == "" {
		logEntry.ID = uuid.New().String()
	}
	if logEntry.Source == "" {
		logEntry.Source = "unknown"
	}
	logEntry.OriginalPreview = truncateRunes(logEntry.OriginalPreview, 512)
	logEntry.SanitizedPreview = truncateRunes(logEntry.SanitizedPreview, 512)
	if logEntry.Metadata != nil {
		logEntry.MetadataJSON, _ = json.Marshal(logEntry.Metadata)
	}
	if len(logEntry.MetadataJSON) == 0 {
		logEntry.MetadataJSON = []byte("{}")
	}
	_, err := database.DB.NamedExec(`
		INSERT INTO memory_audit_logs
		(id, tenant_id, user_id, memory_id, action, source, original_preview, sanitized_preview, reason, metadata, created_at)
		VALUES (:id, :tenant_id, :user_id, :memory_id, :action, :source, :original_preview, :sanitized_preview, :reason, :metadata, NOW())`, &logEntry)
	if err != nil {
		log.Printf("SaveMemoryAudit failed: %v", err)
	}
}

func truncateRunes(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit])
}
