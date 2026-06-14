package repositories

import (
	"testing"
)

func TestMemoryPoolCountQueryIncludesMemoryEntries(t *testing.T) {
	query := memoryPoolCountQuery()

	if !containsAll(query, "FROM memory_entries", "WHERE pool_id = ?") {
		t.Fatalf("memory pool count query must include memory_entries, got: %s", query)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !stringsContains(s, part) {
			return false
		}
	}
	return true
}

func stringsContains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return substr == ""
}
