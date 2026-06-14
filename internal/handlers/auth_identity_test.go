package handlers

import (
	"errors"
	"testing"

	"github.com/easp-platform/easp/internal/models"
)

func TestRegisterIdentityRequiresEmailOrPhone(t *testing.T) {
	req := RegisterRequest{TenantID: "tenant-1", Password: "password"}
	if err := req.NormalizeAndValidateIdentity(); err == nil {
		t.Fatalf("registration without email and phone should be rejected")
	}
}

func TestRegisterIdentityAllowsPhoneOnlyAndNormalizesPhone(t *testing.T) {
	req := RegisterRequest{TenantID: "tenant-1", Phone: " 138 0013-8000 ", Password: "password"}
	if err := req.NormalizeAndValidateIdentity(); err != nil {
		t.Fatalf("phone-only registration should be allowed: %v", err)
	}
	if req.Phone != "13800138000" {
		t.Fatalf("phone should be normalized, got %q", req.Phone)
	}
}

func TestLoginIdentifierSupportsEmailAndPhone(t *testing.T) {
	emailReq := LoginRequest{Email: "User@Example.COM", Password: "password"}
	if got := emailReq.NormalizedIdentifier(); got != "user@example.com" {
		t.Fatalf("expected normalized email identifier, got %q", got)
	}

	phoneReq := LoginRequest{Phone: " 138 0013-8000 ", Password: "password"}
	if got := phoneReq.NormalizedIdentifier(); got != "13800138000" {
		t.Fatalf("expected normalized phone identifier, got %q", got)
	}
}

func TestSelectLoginUserAllowsSingleTenantWithoutTenantID(t *testing.T) {
	users := []models.User{
		{ID: "u1", TenantID: "tenant-a", Email: "single@example.com"},
	}
	user, err := SelectLoginUser(users, "")
	if err != nil {
		t.Fatalf("single-tenant login without tenant id should be allowed: %v", err)
	}
	if user.ID != "u1" {
		t.Fatalf("expected single matched user, got %+v", user)
	}
}

func TestSelectLoginUserRequiresTenantWhenIdentifierMatchesMultipleTenants(t *testing.T) {
	users := []models.User{
		{ID: "u1", TenantID: "tenant-a", Email: "same@example.com"},
		{ID: "u2", TenantID: "tenant-b", Email: "same@example.com"},
	}
	_, err := SelectLoginUser(users, "")
	if !errors.Is(err, ErrTenantRequired) {
		t.Fatalf("expected ErrTenantRequired for ambiguous cross-tenant login, got %v", err)
	}
}

func TestSelectLoginUserUsesSpecifiedTenant(t *testing.T) {
	users := []models.User{
		{ID: "u1", TenantID: "tenant-a", Email: "same@example.com"},
		{ID: "u2", TenantID: "tenant-b", Email: "same@example.com"},
	}
	user, err := SelectLoginUser(users, "tenant-b")
	if err != nil {
		t.Fatalf("specified tenant should resolve user: %v", err)
	}
	if user.ID != "u2" {
		t.Fatalf("expected tenant-b user, got %+v", user)
	}
}
