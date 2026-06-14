package memory

import (
	"strings"
	"testing"
)

func TestSanitizeForPersistenceRedactsSecrets(t *testing.T) {
	input := "调用接口使用 Authorization: Bearer sk-1234567890abcdef，password=super-secret"

	clean, findings, blocked := SanitizeForPersistence(input)

	if blocked {
		t.Fatalf("expected redaction, got blocked")
	}
	if len(findings) < 2 {
		t.Fatalf("expected at least 2 findings, got %d", len(findings))
	}
	if strings.Contains(clean, "sk-1234567890abcdef") || strings.Contains(clean, "super-secret") {
		t.Fatalf("secret leaked after sanitization: %s", clean)
	}
	if !strings.Contains(clean, RedactedPlaceholder) {
		t.Fatalf("expected redacted placeholder in %q", clean)
	}
}

func TestSanitizeForPersistenceBlocksPrivateKey(t *testing.T) {
	input := "-----BEGIN PRIVATE KEY-----\nabc123\n-----END PRIVATE KEY-----"

	clean, findings, blocked := SanitizeForPersistence(input)

	if !blocked {
		t.Fatalf("expected private key content to be blocked, clean=%q findings=%v", clean, findings)
	}
	if clean != "" {
		t.Fatalf("blocked content should not be persisted, got %q", clean)
	}
}

func TestNormalizeMemoryContentAndHashAreStable(t *testing.T) {
	a := " 用户 偏好：简洁回答！！ "
	b := "用户偏好:简洁回答"

	if NormalizeMemoryContent(a) != NormalizeMemoryContent(b) {
		t.Fatalf("expected normalized content to match: %q vs %q", NormalizeMemoryContent(a), NormalizeMemoryContent(b))
	}
	if MemoryContentHash(a) != MemoryContentHash(b) {
		t.Fatalf("expected content hashes to match")
	}
}

func TestDefaultMemorySettings(t *testing.T) {
	settings := DefaultMemorySettings("tenant-1", "user-1")

	if !settings.AutoExtractEnabled || !settings.RecallEnabled || !settings.SensitiveFilterEnabled || !settings.AuditEnabled {
		t.Fatalf("default memory settings should enable governance controls: %+v", settings)
	}
	if settings.HybridSearchMode != "keyword_vector" {
		t.Fatalf("expected keyword_vector hybrid mode, got %q", settings.HybridSearchMode)
	}
}
