package handlers

import (
	"errors"
	"testing"

	"github.com/easp-platform/easp/internal/models"
)

func TestRegisterIdentityRequiresAccount(t *testing.T) {
	req := RegisterRequest{TenantID: "tenant-1", Password: "password"}
	if err := req.NormalizeAndValidateIdentity(); err == nil {
		t.Fatalf("registration without account should be rejected")
	}
}

func TestRegisterIdentityAllowsAccountAndNormalizesContacts(t *testing.T) {
	req := RegisterRequest{TenantID: "tenant-1", Account: " User001 ", Email: " User@Example.COM ", Phone: " 138 0013-8000 ", Password: "password"}
	if err := req.NormalizeAndValidateIdentity(); err != nil {
		t.Fatalf("account registration should be allowed: %v", err)
	}
	if req.Account != "user001" {
		t.Fatalf("account should be normalized, got %q", req.Account)
	}
	if req.Email != "user@example.com" {
		t.Fatalf("email should be normalized, got %q", req.Email)
	}
	if req.Phone != "13800138000" {
		t.Fatalf("phone should be normalized, got %q", req.Phone)
	}
}

func TestLoginIdentifierUsesAccount(t *testing.T) {
	accountReq := LoginRequest{Account: " User001 ", Password: "password"}
	if got := accountReq.NormalizedIdentifier(); got != "user001" {
		t.Fatalf("expected normalized account identifier, got %q", got)
	}

	legacyEmailReq := LoginRequest{Email: "User@Example.COM", Password: "password"}
	if got := legacyEmailReq.NormalizedIdentifier(); got != "user@example.com" {
		t.Fatalf("expected legacy email as account identifier, got %q", got)
	}

	legacyPhoneReq := LoginRequest{Phone: " 138 0013-8000 ", Password: "password"}
	if got := legacyPhoneReq.NormalizedIdentifier(); got != "138 0013-8000" {
		t.Fatalf("legacy phone is treated as account text, got %q", got)
	}
}

func TestSelectLoginUserAllowsSingleTenantWithoutTenantID(t *testing.T) {
	users := []models.User{
		{ID: "u1", TenantID: "tenant-a", Account: "single"},
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
		{ID: "u1", TenantID: "tenant-a", Account: "same"},
		{ID: "u2", TenantID: "tenant-b", Account: "same"},
	}
	_, err := SelectLoginUser(users, "")
	if !errors.Is(err, ErrTenantRequired) {
		t.Fatalf("expected ErrTenantRequired for ambiguous cross-tenant login, got %v", err)
	}
}

func TestSelectLoginUserUsesSpecifiedTenant(t *testing.T) {
	users := []models.User{
		{ID: "u1", TenantID: "tenant-a", Account: "same"},
		{ID: "u2", TenantID: "tenant-b", Account: "same"},
	}
	user, err := SelectLoginUser(users, "tenant-b")
	if err != nil {
		t.Fatalf("specified tenant should resolve user: %v", err)
	}
	if user.ID != "u2" {
		t.Fatalf("expected tenant-b user, got %+v", user)
	}
}
