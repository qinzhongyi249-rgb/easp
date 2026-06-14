package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"unicode"
)

const RedactedPlaceholder = "[REDACTED]"

// SensitiveFinding describes a sensitive fragment detected before persistence.
type SensitiveFinding struct {
	Type   string `json:"type"`
	Action string `json:"action"` // redacted/blocked
}

type sensitivePattern struct {
	typ   string
	re    *regexp.Regexp
	block bool
}

var sensitivePatterns = []sensitivePattern{
	{typ: "private_key", re: regexp.MustCompile(`(?is)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`), block: true},
	{typ: "jwt", re: regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`)},
	{typ: "bearer_token", re: regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]{12,}`)},
	{typ: "api_key", re: regexp.MustCompile(`(?i)\b(api[_-]?key|access[_-]?token|refresh[_-]?token|secret|token)\s*[:=]\s*['"]?[^\s,'"，。；;]{6,}`)},
	{typ: "password", re: regexp.MustCompile(`(?i)\b(password|passwd|pwd|密码)\s*[:=：]\s*['"]?[^\s,'"，。；;]{4,}`)},
	{typ: "db_dsn", re: regexp.MustCompile(`(?i)\b(mysql|postgres|postgresql|mongodb|redis)://[^\s]+`)},
	{typ: "sk_key", re: regexp.MustCompile(`\b(sk|pk|ak)-[A-Za-z0-9_-]{10,}\b`)},
}

// SanitizeForPersistence redacts or blocks sensitive content before it is written to memory/audit stores.
// It must not be used on live tool-call inputs; only persistence/display/recall paths call this.
func SanitizeForPersistence(input string) (string, []SensitiveFinding, bool) {
	clean := input
	findings := make([]SensitiveFinding, 0)
	blocked := false

	for _, p := range sensitivePatterns {
		if !p.re.MatchString(clean) {
			continue
		}
		action := "redacted"
		if p.block {
			action = "blocked"
			blocked = true
		}
		findings = append(findings, SensitiveFinding{Type: p.typ, Action: action})
		if p.block {
			continue
		}
		clean = p.re.ReplaceAllString(clean, RedactedPlaceholder)
	}

	if blocked {
		return "", findings, true
	}
	return clean, findings, false
}

// NormalizeMemoryContent creates a stable comparable representation for exact de-duplication.
func NormalizeMemoryContent(content string) string {
	content = strings.ToLower(strings.TrimSpace(content))
	var b strings.Builder
	for _, r := range content {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// MemoryContentHash returns a stable sha256 hash for normalized memory content.
func MemoryContentHash(content string) string {
	sum := sha256.Sum256([]byte(NormalizeMemoryContent(content)))
	return hex.EncodeToString(sum[:])
}
