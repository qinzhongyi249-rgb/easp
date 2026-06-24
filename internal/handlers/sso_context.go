package handlers

import (
	"context"

	"github.com/easp-platform/easp/internal/mcp"
	"github.com/easp-platform/easp/internal/sso"
)

func contextWithUserSSOToken(ctx context.Context, tenantID, userID string) context.Context {
	if token, ok := mcp.UserSSOTokenFromContext(ctx); ok && token != "" {
		return ctx
	}
	if tenantID == "" || userID == "" {
		return ctx
	}
	token, err := sso.GetUserToken(tenantID, userID)
	if err != nil || token == "" {
		return ctx
	}
	return mcp.WithUserSSOToken(ctx, token)
}
