package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// EmbedClaims 嵌入式助手短 Token 声明
type EmbedClaims struct {
	TokenType        string   `json:"token_type"`
	TenantID         string   `json:"tenant_id"`
	UserID           string   `json:"user_id"`
	Email            string   `json:"email"`
	ExternalSystem   string   `json:"external_system"`
	ExternalUserID   string   `json:"external_user_id"`
	AppID            string   `json:"app_id"`
	ExternalTokenRef string   `json:"external_token_ref,omitempty"`
	Scopes           []string `json:"scopes"`
	jwt.RegisteredClaims
}

// GenerateEmbedToken 生成嵌入式 AI 助手短 Token。
func GenerateEmbedToken(tenantID, userID, email, externalSystem, externalUserID, appID string, scopes []string, ttlSeconds int) (string, int64, error) {
	return GenerateEmbedTokenWithExternalTokenRef(tenantID, userID, email, externalSystem, externalUserID, appID, "", scopes, ttlSeconds)
}

// GenerateEmbedTokenWithExternalTokenRef 生成带外部业务 token 引用的嵌入式助手短 Token。
func GenerateEmbedTokenWithExternalTokenRef(tenantID, userID, email, externalSystem, externalUserID, appID, externalTokenRef string, scopes []string, ttlSeconds int) (string, int64, error) {
	if ttlSeconds <= 0 {
		ttlSeconds = 7200
	}
	now := time.Now()
	expiresAt := now.Add(time.Duration(ttlSeconds) * time.Second)
	claims := &EmbedClaims{
		TokenType:        "embed",
		TenantID:         tenantID,
		UserID:           userID,
		Email:            email,
		ExternalSystem:   externalSystem,
		ExternalUserID:   externalUserID,
		AppID:            appID,
		ExternalTokenRef: externalTokenRef,
		Scopes:           scopes,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "easp-platform",
			Subject:   userID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(JWTSecret)
	if err != nil {
		return "", 0, err
	}
	return tokenStr, expiresAt.Unix(), nil
}

// ParseEmbedToken 解析嵌入式 AI 助手短 Token。
func ParseEmbedToken(tokenStr string) (*EmbedClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &EmbedClaims{}, func(token *jwt.Token) (interface{}, error) {
		return JWTSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*EmbedClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid embed token")
	}
	if claims.TokenType != "embed" {
		return nil, errors.New("invalid token type")
	}
	return claims, nil
}
