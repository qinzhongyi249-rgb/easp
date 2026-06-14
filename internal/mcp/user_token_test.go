package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/easp-platform/easp/internal/models"
)

func TestBuildConnectorRuntimeAuthUsesUserSSOToken(t *testing.T) {
	mode := "user_token"
	header := "X-User-Token"
	prefix := "Token"
	connector := models.Connector{
		ID:                   "conn-1",
		CredentialMode:       &mode,
		UserTokenHeader:      &header,
		UserTokenPrefix:      &prefix,
		UserTokenRequiredSSO: true,
	}

	ctx := WithUserSSOToken(context.Background(), "biz-token-123")
	runtime, err := BuildConnectorRuntimeAuth(ctx, connector)
	if err != nil {
		t.Fatalf("BuildConnectorRuntimeAuth returned error: %v", err)
	}
	if runtime.AuthType != nil || runtime.AuthConfig != nil {
		t.Fatalf("user_token mode should not use static auth config")
	}
	if runtime.Headers == nil || *runtime.Headers != `{"X-User-Token":"Token biz-token-123"}` {
		t.Fatalf("unexpected runtime headers: %#v", runtime.Headers)
	}
}

func TestBuildConnectorRuntimeAuthRequiresUserSSOToken(t *testing.T) {
	mode := "user_token"
	connector := models.Connector{
		ID:                   "conn-1",
		CredentialMode:       &mode,
		UserTokenRequiredSSO: true,
	}

	_, err := BuildConnectorRuntimeAuth(context.Background(), connector)
	if err == nil {
		t.Fatal("expected missing SSO token error")
	}
	if !strings.Contains(err.Error(), "SSO") {
		t.Fatalf("expected explicit SSO error, got: %v", err)
	}
}
