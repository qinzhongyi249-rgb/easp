package auth

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

type embedExternalTokenEntry struct {
	Token     string
	ExpiresAt time.Time
}

var embedExternalTokens sync.Map // map[string]embedExternalTokenEntry

// StoreEmbedExternalUserToken 暂存嵌入式助手当前外部业务系统用户 token，只返回不可反推的引用。
func StoreEmbedExternalUserToken(token string, expiresAt time.Time) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	if expiresAt.IsZero() || expiresAt.Before(time.Now()) {
		expiresAt = time.Now().Add(2 * time.Hour)
	}
	refBytes := make([]byte, 24)
	if _, err := rand.Read(refBytes); err != nil {
		return ""
	}
	ref := "exttok_" + hex.EncodeToString(refBytes)
	embedExternalTokens.Store(ref, embedExternalTokenEntry{Token: token, ExpiresAt: expiresAt})
	return ref
}

// GetEmbedExternalUserToken 通过引用读取外部业务系统用户 token。过期引用会自动清理。
func GetEmbedExternalUserToken(ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", false
	}
	value, ok := embedExternalTokens.Load(ref)
	if !ok {
		return "", false
	}
	entry, ok := value.(embedExternalTokenEntry)
	if !ok || strings.TrimSpace(entry.Token) == "" || (!entry.ExpiresAt.IsZero() && time.Now().After(entry.ExpiresAt)) {
		embedExternalTokens.Delete(ref)
		return "", false
	}
	return entry.Token, true
}
