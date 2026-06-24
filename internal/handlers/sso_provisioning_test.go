package handlers

import (
	"errors"
	"testing"
)

func TestSSOProvisioningRejectsUnknownUserWhenAutoCreateDisabled(t *testing.T) {
	_, err := resolveSSOProvisionedUser(nil, false, func() (*ssoProvisionedUser, error) {
		t.Fatalf("create callback must not be called when auto_create_user=false")
		return nil, nil
	})

	if !errors.Is(err, ErrSSOUserNotProvisioned) {
		t.Fatalf("expected ErrSSOUserNotProvisioned when SSO user is not pre-provisioned and auto_create_user=false, got %v", err)
	}
}

func TestSSOProvisioningCreatesUnknownUserWhenAutoCreateEnabled(t *testing.T) {
	created := &ssoProvisionedUser{ID: "user-1"}
	user, err := resolveSSOProvisionedUser(nil, true, func() (*ssoProvisionedUser, error) {
		return created, nil
	})

	if err != nil {
		t.Fatalf("auto_create_user=true should create unknown SSO user: %v", err)
	}
	if user.ID != "user-1" {
		t.Fatalf("expected created user, got %#v", user)
	}
}

func TestTenantSSOConfigDefaultRoleIDsJSONRoundTrip(t *testing.T) {
	encoded := mustEncodeRoleIDs([]string{"role-a", "role-b"})
	decoded := decodeRoleIDs(encoded)

	if len(decoded) != 2 || decoded[0] != "role-a" || decoded[1] != "role-b" {
		t.Fatalf("expected role IDs round-trip, got %#v", decoded)
	}
}
