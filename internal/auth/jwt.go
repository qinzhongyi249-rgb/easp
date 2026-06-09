package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWT配置
var (
	JWTSecret     = []byte("easp-jwt-secret-key-2026")
	AccessTokenTTL  = 2 * time.Hour    // Access Token有效期2小时
	RefreshTokenTTL = 7 * 24 * time.Hour // Refresh Token有效期7天
)

// AdminRoleIDs 动态存储所有admin角色的ID集合（系统级+租户级管理员）
var AdminRoleIDs = map[string]bool{
	"sys-admin": true,
}

// IsAdminRole 判断角色ID是否为管理员角色
func IsAdminRole(roleID string) bool {
	return AdminRoleIDs[roleID]
}

// AddAdminRole 注册一个管理员角色ID
func AddAdminRole(roleID string) {
	AdminRoleIDs[roleID] = true
}

// Claims JWT声明
type Claims struct {
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id"`
	Email    string `json:"email"`
	RoleIDs  string `json:"role_ids"` // JSON格式的角色ID列表
	jwt.RegisteredClaims
}

// TokenPair Token对
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
}

// GenerateTokenPair 生成Token对
func GenerateTokenPair(userID, tenantID, email, roleIDs string) (*TokenPair, error) {
	now := time.Now()

	// Access Token
	accessClaims := &Claims{
		UserID:   userID,
		TenantID: tenantID,
		Email:    email,
		RoleIDs:  roleIDs,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "easp-platform",
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenStr, err := accessToken.SignedString(JWTSecret)
	if err != nil {
		return nil, err
	}

	// Refresh Token
	refreshClaims := &Claims{
		UserID:   userID,
		TenantID: tenantID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(RefreshTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "easp-platform",
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenStr, err := refreshToken.SignedString(JWTSecret)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessTokenStr,
		RefreshToken: refreshTokenStr,
		ExpiresAt:    now.Add(AccessTokenTTL).Unix(),
	}, nil
}

// ParseToken 解析Token
func ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return JWTSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// RefreshAccessToken 刷新Access Token
func RefreshAccessToken(refreshTokenStr string) (*TokenPair, error) {
	claims, err := ParseToken(refreshTokenStr)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	// 生成新的Token对
	return GenerateTokenPair(claims.UserID, claims.TenantID, claims.Email, claims.RoleIDs)
}
