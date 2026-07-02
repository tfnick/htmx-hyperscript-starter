package authz

const (
	RolePlatformAdmin = "platform_admin"
	RoleOrgAdmin      = "org_admin"
	RoleUser          = "user"
)

const (
	DataScopePlatform     = "platform"
	DataScopeOrganization = "organization"
	DataScopeSelf         = "self"
)

const (
	PermissionPlatformOperate = "platform:operate"

	PermissionOrdersReadSelf = "orders:read:self"
	PermissionOrdersReadOrg  = "orders:read:org"
	PermissionOrdersReadAll  = "orders:read:all"

	PermissionUsersReadOrg = "users:read:org"
	PermissionUsersReadAll = "users:read:all"

	PermissionSupportReadOrg = "support:read:org"
	PermissionSupportReadAll = "support:read:all"

	PermissionProductsManage      = "products:manage"
	PermissionSchedulerManage     = "scheduler:manage"
	PermissionEventsReadAll       = "events:read:all"
	PermissionMessagesReadAll     = "messages:read:all"
	PermissionDictionaryManage    = "dictionary:manage"
	PermissionParametersManage    = "parameters:manage"
	PermissionCacheManage         = "cache:manage"
	PermissionMonitorManage       = "monitor:manage"
	PermissionNotificationsManage = "notifications:manage"
	PermissionVariablesManage     = "variables:manage"
	PermissionSettingsManage      = "settings:manage"
	PermissionKBManage            = "kb:manage"
	PermissionExperimentsUse      = "experiments:use"
)

func NormalizeRole(role string, isAdmin bool) string {
	switch role {
	case RolePlatformAdmin, RoleOrgAdmin, RoleUser:
		return role
	default:
		if isAdmin {
			return RolePlatformAdmin
		}
		return RoleUser
	}
}

func DataScopeForRole(role string) string {
	switch NormalizeRole(role, false) {
	case RolePlatformAdmin:
		return DataScopePlatform
	case RoleOrgAdmin:
		return DataScopeOrganization
	default:
		return DataScopeSelf
	}
}

func PermissionsForRole(role string) []string {
	switch NormalizeRole(role, false) {
	case RolePlatformAdmin:
		return []string{
			PermissionPlatformOperate,
			PermissionOrdersReadSelf,
			PermissionOrdersReadOrg,
			PermissionOrdersReadAll,
			PermissionUsersReadOrg,
			PermissionUsersReadAll,
			PermissionSupportReadOrg,
			PermissionSupportReadAll,
			PermissionProductsManage,
			PermissionSchedulerManage,
			PermissionEventsReadAll,
			PermissionMessagesReadAll,
			PermissionDictionaryManage,
			PermissionParametersManage,
			PermissionCacheManage,
			PermissionMonitorManage,
			PermissionNotificationsManage,
			PermissionVariablesManage,
			PermissionSettingsManage,
			PermissionKBManage,
			PermissionExperimentsUse,
		}
	case RoleOrgAdmin:
		return []string{
			PermissionOrdersReadSelf,
			PermissionOrdersReadOrg,
			PermissionUsersReadOrg,
			PermissionSupportReadOrg,
			PermissionExperimentsUse,
		}
	default:
		return []string{
			PermissionOrdersReadSelf,
			PermissionExperimentsUse,
		}
	}
}

func HasPermission(permissions []string, permission string) bool {
	for _, candidate := range permissions {
		if candidate == permission {
			return true
		}
	}
	return false
}

func HasRole(role string, allowed ...string) bool {
	normalized := NormalizeRole(role, false)
	for _, candidate := range allowed {
		if normalized == candidate {
			return true
		}
	}
	return false
}
