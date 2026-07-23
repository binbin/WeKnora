package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// OrgUnitService manages the tenant-scoped administrative hierarchy and
// resolves ancestor-readable / self-writable visibility for knowledge.
type OrgUnitService interface {
	Create(ctx context.Context, tenantID uint64, req *types.CreateOrgUnitRequest) (*types.OrgUnit, error)
	Get(ctx context.Context, tenantID uint64, id string) (*types.OrgUnit, error)
	Update(ctx context.Context, tenantID uint64, id string, req *types.UpdateOrgUnitRequest) (*types.OrgUnit, error)
	Delete(ctx context.Context, tenantID uint64, id string) error
	ListFlat(ctx context.Context, tenantID uint64) ([]*types.OrgUnit, error)
	ListTree(ctx context.Context, tenantID uint64) ([]*types.OrgUnit, error)
	// ListPlatformTree returns the platform catalog (tenant_id=0) plus
	// legacy in-tenant org trees so system admins see the full forest.
	ListPlatformTree(ctx context.Context) ([]*types.OrgUnit, error)
	ListPlatformFlat(ctx context.Context) ([]*types.OrgUnit, error)
	Move(ctx context.Context, tenantID uint64, id string, newParentID string) (*types.OrgUnit, error)

	AddMember(ctx context.Context, tenantID uint64, orgUnitID string, userID string, isPrimary bool) (*types.OrgUnitMember, error)
	// TransferMember moves the user to toOrgUnitID within the tenant,
	// replacing any existing membership (single-membership model).
	TransferMember(
		ctx context.Context,
		tenantID uint64,
		userID string,
		toOrgUnitID string,
	) (*types.OrgUnitMember, error)
	RemoveMember(ctx context.Context, tenantID uint64, orgUnitID string, userID string) error
	ListMembers(ctx context.Context, tenantID uint64, orgUnitID string) ([]*types.OrgUnitMember, error)
	ListUserMemberships(ctx context.Context, tenantID uint64, userID string) ([]*types.OrgUnitMember, error)
	SetPrimary(ctx context.Context, tenantID uint64, userID string, orgUnitID string) error

	// ResolveActiveOrgUnit picks the caller's active unit from an
	// explicit header value or primary membership. Returns "" when the
	// tenant has no hierarchy or the user has no membership (legacy mode).
	ResolveActiveOrgUnit(ctx context.Context, tenantID uint64, userID string, requestedID string) (string, error)

	// ResolveVisibility returns readable (self+ancestors) and writable
	// (self) OrgUnit IDs for the active unit.
	ResolveVisibility(ctx context.Context, tenantID uint64, orgUnitID string) (*types.OrgUnitVisibility, error)

	// CanReadKB reports whether the active OrgUnit may read a KB bound
	// to kbOrgUnitID. Unbound KBs (empty kbOrgUnitID) stay tenant-wide
	// readable. Ancestor KBs are readable only when shareWithDescendants
	// is true (下级引用上级需显式勾选「共享给下级机构」).
	CanReadKB(
		ctx context.Context,
		tenantID uint64,
		activeOrgUnitID string,
		kbOrgUnitID string,
		shareWithDescendants bool,
	) (bool, error)

	// CanWriteKB reports whether the active OrgUnit may mutate a KB.
	CanWriteKB(ctx context.Context, tenantID uint64, activeOrgUnitID string, kbOrgUnitID string) (bool, error)

	// ListAncestorIDs returns [self, parent, ..., root] for path closure.
	ListAncestorIDs(ctx context.Context, tenantID uint64, orgUnitID string) ([]string, error)

	// HasHierarchy is true when the tenant has at least one live OrgUnit.
	HasHierarchy(ctx context.Context, tenantID uint64) (bool, error)

	// ListInviteableOrgUnits returns units the actor may assign when
	// inviting a member. Contributor/viewer: own unit + siblings (平级)
	// + descendants (下级). Admin/Owner: descendants only — 同级不可
	// 再任命管理员（或历史所有者）。
	ListInviteableOrgUnits(
		ctx context.Context,
		tenantID uint64,
		actorOrgUnitID string,
		role types.TenantRole,
	) ([]*types.OrgUnit, error)

	// CanInviteToOrgUnit reports whether targetOrgUnitID is within the
	// inviteable scope for actorOrgUnitID and the granted tenant role.
	CanInviteToOrgUnit(
		ctx context.Context,
		tenantID uint64,
		actorOrgUnitID string,
		targetOrgUnitID string,
		role types.TenantRole,
	) (bool, error)

	// AssertCanManageTenantMember enforces org-scoped admin limits:
	// manage peer non-admins or subordinates; never peer admins/owners;
	// never promote a peer to admin/owner. Unscoped actors (system admin,
	// cross-tenant superuser, tenant owner) and tenants without hierarchy
	// bypass. newRole may be empty when the operation is remove-only.
	AssertCanManageTenantMember(
		ctx context.Context,
		tenantID uint64,
		targetUserID string,
		targetRole types.TenantRole,
		newRole types.TenantRole,
	) error
}

// OrgUnitRepository persists OrgUnits and memberships.
type OrgUnitRepository interface {
	Create(ctx context.Context, unit *types.OrgUnit) error
	GetByID(ctx context.Context, tenantID uint64, id string) (*types.OrgUnit, error)
	// GetByIDGlobal looks up an OrgUnit by id without tenant scope
	// (platform catalog + legacy in-tenant trees).
	GetByIDGlobal(ctx context.Context, id string) (*types.OrgUnit, error)
	Update(ctx context.Context, unit *types.OrgUnit) error
	Delete(ctx context.Context, tenantID uint64, id string) error
	ListByTenant(ctx context.Context, tenantID uint64) ([]*types.OrgUnit, error)
	// ListAll returns every live OrgUnit across tenants (platform catalog
	// and legacy in-tenant trees). Used by system-admin platform views.
	ListAll(ctx context.Context) ([]*types.OrgUnit, error)
	ListRoots(ctx context.Context, tenantID uint64) ([]*types.OrgUnit, error)
	CountByTenant(ctx context.Context, tenantID uint64) (int64, error)
	CountChildren(ctx context.Context, tenantID uint64, parentID string) (int64, error)
	ListByPathPrefix(ctx context.Context, tenantID uint64, pathPrefix string) ([]*types.OrgUnit, error)
	UpdateSubtreePaths(ctx context.Context, tenantID uint64, oldPrefix string, newPrefix string, depthDelta int) error

	AddMember(ctx context.Context, member *types.OrgUnitMember) error
	RemoveMember(ctx context.Context, orgUnitID string, userID string) error
	ListMembers(ctx context.Context, orgUnitID string) ([]*types.OrgUnitMember, error)
	ListMembersByOrgUnitIDs(ctx context.Context, orgUnitIDs []string) ([]*types.OrgUnitMember, error)
	ListUserMemberships(ctx context.Context, tenantID uint64, userID string) ([]*types.OrgUnitMember, error)
	ListUserMembershipsByUser(ctx context.Context, userID string) ([]*types.OrgUnitMember, error)
	GetMember(ctx context.Context, orgUnitID string, userID string) (*types.OrgUnitMember, error)
	ClearPrimary(ctx context.Context, tenantID uint64, userID string) error
	SetPrimary(ctx context.Context, tenantID uint64, userID string, orgUnitID string) error
	// RemoveMembersByTenantUser deletes all org_unit_members rows for
	// the user in the tenant (0 or 1 row after unique constraint).
	RemoveMembersByTenantUser(ctx context.Context, tenantID uint64, userID string) error
	// TransferMember atomically moves the user to toOrgUnitID within
	// tenantID (delete any existing memberships, then insert).
	TransferMember(
		ctx context.Context,
		member *types.OrgUnitMember,
	) error
}

// OrgUnitWorkspaceRepository persists root-OrgUnit → Tenant bindings.
type OrgUnitWorkspaceRepository interface {
	GetByRootOrgUnitID(ctx context.Context, rootOrgUnitID string) (*types.OrgUnitWorkspace, error)
	GetByTenantID(ctx context.Context, tenantID uint64) (*types.OrgUnitWorkspace, error)
	Create(ctx context.Context, binding *types.OrgUnitWorkspace) error
}

// OrgWorkspaceService lazily provisions business Tenants for platform
// root OrgUnits and keeps org members enrolled in that workspace.
type OrgWorkspaceService interface {
	// EnsureWorkspaceForUser resolves the caller's primary (or first)
	// OrgUnit membership, walks to the root org, creates「{name}的空间」
	// if missing, enrolls the user (and syncs sibling/descendant
	// members), and returns the workspace tenant id. Returns 0 when the
	// user has no OrgUnit binding.
	EnsureWorkspaceForUser(ctx context.Context, userID string) (uint64, error)

	// EnsureFirstPlatformWorkspace returns the workspace for the
	// earliest platform root OrgUnit (created_at ASC), creating it if
	// needed. Returns 0 when the platform catalog has no roots.
	EnsureFirstPlatformWorkspace(ctx context.Context) (uint64, error)
}
