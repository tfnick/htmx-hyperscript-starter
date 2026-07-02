package authz

import "testing"

func TestNormalizeRolePreservesIsAdminCompatibility(t *testing.T) {
	if got := NormalizeRole("", true); got != RolePlatformAdmin {
		t.Fatalf("expected legacy admin to map to platform admin, got %q", got)
	}
	if got := NormalizeRole("", false); got != RoleUser {
		t.Fatalf("expected empty regular role to map to user, got %q", got)
	}
	if got := NormalizeRole(RoleOrgAdmin, false); got != RoleOrgAdmin {
		t.Fatalf("expected org admin role to be preserved, got %q", got)
	}
}

func TestRolePermissionAndDataScopeMatrix(t *testing.T) {
	if got := DataScopeForRole(RolePlatformAdmin); got != DataScopePlatform {
		t.Fatalf("expected platform scope, got %q", got)
	}
	if got := DataScopeForRole(RoleOrgAdmin); got != DataScopeOrganization {
		t.Fatalf("expected organization scope, got %q", got)
	}
	if got := DataScopeForRole(RoleUser); got != DataScopeSelf {
		t.Fatalf("expected self scope, got %q", got)
	}

	if !HasPermission(PermissionsForRole(RolePlatformAdmin), PermissionSettingsManage) {
		t.Fatalf("expected platform admin settings permission")
	}
	if !HasPermission(PermissionsForRole(RolePlatformAdmin), PermissionCacheManage) {
		t.Fatalf("expected platform admin cache permission")
	}
	if !HasPermission(PermissionsForRole(RolePlatformAdmin), PermissionMonitorManage) {
		t.Fatalf("expected platform admin monitor permission")
	}
	if HasPermission(PermissionsForRole(RoleOrgAdmin), PermissionSettingsManage) {
		t.Fatalf("did not expect org admin settings permission in MVP")
	}
	if !HasPermission(PermissionsForRole(RoleUser), PermissionOrdersReadSelf) {
		t.Fatalf("expected user self order permission")
	}
}
