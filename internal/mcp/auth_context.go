package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/easp-platform/easp/internal/models"
)

type ssoTokenContextKey struct{}

// WithUserSSOToken stores the current login user's upstream SSO token in context for connector credential_mode=user_token.
func WithUserSSOToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, ssoTokenContextKey{}, token)
}

// UserSSOTokenFromContext reads the current login user's upstream SSO token from context.
func UserSSOTokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(ssoTokenContextKey{}).(string)
	return token, ok && strings.TrimSpace(token) != ""
}

// BuildConnectorRuntimeAuth returns a copy of connector whose auth/header fields are resolved for the current request.
func BuildConnectorRuntimeAuth(ctx context.Context, connector models.Connector) (models.Connector, error) {
	mode := strings.TrimSpace(valueOrDefault(connector.CredentialMode, "static"))
	if mode == "" {
		mode = "static"
	}

	switch mode {
	case "none":
		connector.AuthType = nil
		connector.AuthConfig = nil
		return connector, nil
	case "static":
		return connector, nil
	case "user_token":
		token, ok := UserSSOTokenFromContext(ctx)
		if !ok {
			return connector, fmt.Errorf("连接器要求透传当前 SSO 登录用户 Token，但当前请求没有可用 SSO Token")
		}
		header := strings.TrimSpace(valueOrDefault(connector.UserTokenHeader, "Authorization"))
		if header == "" {
			header = "Authorization"
		}
		prefix := strings.TrimSpace(valueOrDefault(connector.UserTokenPrefix, "Bearer"))
		value := token
		if prefix != "" {
			value = prefix + " " + token
		}
		runtimeHeaders := map[string]string{}
		if connector.Headers != nil && strings.TrimSpace(*connector.Headers) != "" {
			_ = json.Unmarshal([]byte(*connector.Headers), &runtimeHeaders)
		}
		runtimeHeaders[header] = value
		headerBytes, err := json.Marshal(runtimeHeaders)
		if err != nil {
			return connector, fmt.Errorf("构建用户 Token 请求头失败: %w", err)
		}
		headers := string(headerBytes)
		connector.Headers = &headers
		connector.AuthType = nil
		connector.AuthConfig = nil
		return connector, nil
	default:
		return connector, fmt.Errorf("不支持的连接器凭据模式: %s", mode)
	}
}

func valueOrDefault(value *string, def string) string {
	if value == nil {
		return def
	}
	return *value
}
